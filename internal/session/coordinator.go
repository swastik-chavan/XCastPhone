package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"xcastphone/internal/adb"
	"xcastphone/internal/decoder"
	"xcastphone/internal/discovery"
	"xcastphone/internal/display"
	"xcastphone/internal/pairing"
	"xcastphone/internal/streaming"
)

// Config configures a casting session.
type Config struct {
	Verbose      bool
	Debug        bool
	TestStream   bool
	Bitrate      int // in bps
	MaxDimension int
	TargetFPS    int
	PairTimeout  time.Duration
}

// Coordinator manages the lifecycle of an XCastPhone session from pairing to casting and termination.
type Coordinator struct {
	config Config
}

// NewCoordinator creates a new session coordinator.
func NewCoordinator(cfg Config) *Coordinator {
	if cfg.PairTimeout <= 0 {
		cfg.PairTimeout = 90 * time.Second
	}
	if cfg.TargetFPS <= 0 {
		cfg.TargetFPS = 60
	}
	if cfg.Bitrate <= 0 {
		cfg.Bitrate = 8000000 // 8 Mbps
	}
	if cfg.MaxDimension <= 0 {
		cfg.MaxDimension = 2400
	}

	return &Coordinator{
		config: cfg,
	}
}

// Run executes the complete XCastPhone workflow.
func (c *Coordinator) Run(ctx context.Context) error {
	// 1. Initialize ADB client
	client, err := adb.NewClient(c.config.Verbose)
	if err != nil {
		return fmt.Errorf("ADB initialization failed: %w", err)
	}

	// 2. Setup graceful signal handling for Ctrl+C
	sigCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// 3. Fast check: is an authorized device already connected?
	dev, err := client.GetFirstAuthorizedDevice()
	if err != nil {
		return fmt.Errorf("failed checking connected devices: %w", err)
	}

	if dev == nil {
		// No active device found: initiate QR pairing flow
		fmt.Println("==========================================")
		fmt.Println("       XCastPhone Wireless Pairing        ")
		fmt.Println("==========================================")
		fmt.Println()
		fmt.Println("Scan this QR code using:")
		fmt.Println()
		fmt.Println("Android Settings")
		fmt.Println("→ Developer options")
		fmt.Println("→ Wireless debugging")
		fmt.Println("→ Pair device with QR code")
		fmt.Println()
		fmt.Println("[QR WINDOW]")
		fmt.Println()

		pairSession, err := pairing.NewSession(c.config.PairTimeout)
		if err != nil {
			return fmt.Errorf("failed to generate pairing session: %w", err)
		}

		// Generate exact square integer-scaled QR image
		qrPath, err := pairSession.CreateTempQRImageFile(800)
		if err != nil {
			return fmt.Errorf("failed to generate QR code image: %w", err)
		}

		// Launch native QR window
		qrWin, err := display.LaunchQRWindow(sigCtx, qrPath, c.config.Verbose)
		if err != nil {
			_ = os.Remove(qrPath)
			return fmt.Errorf("failed to launch QR window: %w", err)
		}
		defer qrWin.Close()

		// If user closes the QR window manually, cancel pairing context
		pairCtx, cancelPair := context.WithCancel(sigCtx)
		defer cancelPair()

		go func() {
			select {
			case <-qrWin.ExitChan:
				cancelPair()
			case <-pairCtx.Done():
			}
		}()

		// Discover and pair
		dev, err = discovery.DiscoverAndPair(pairCtx, client, pairSession, c.config.PairTimeout, c.config.Verbose)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Println("\nPairing aborted by user.")
				return nil
			}
			return fmt.Errorf("pairing failed: %w", err)
		}

		// Close QR window immediately after successful pairing & authorization
		_ = qrWin.Close()
	}

	// 4. Authorized device confirmed & refreshed
	activeDev, err := client.EnsureAuthorizedDevice(dev)
	if err != nil {
		fmt.Printf("[STREAM][ERROR] Device verification failed: %v\n", err)
		return fmt.Errorf("device verification failed: %w", err)
	}
	dev = activeDev
	_ = client.PopulateDeviceInfo(dev)

	fmt.Println()
	fmt.Println("XCastPhone")
	fmt.Println()
	fmt.Printf("Device:     %s\n", dev.DisplayName())
	fmt.Printf("Resolution: %s\n", dev.ResolutionString())
	fmt.Printf("FPS:        %d\n", c.config.TargetFPS)
	fmt.Println("Status:     Connected")
	fmt.Println()

	// 5. Setup streaming pipeline
	inspector := decoder.NewStreamInspector()
	defer inspector.Close()

	streamCfg := streaming.StreamConfig{
		Device:       dev,
		Bitrate:      c.config.Bitrate,
		MaxDimension: c.config.MaxDimension,
		TargetFPS:    c.config.TargetFPS,
		Verbose:      c.config.Verbose || c.config.Debug,
	}

	streamMgr := streaming.NewStreamManager(client, streamCfg, inspector)

	// If TestStream flag is set, run 3-second headless diagnostic stream
	if c.config.TestStream {
		fmt.Println("[DIAG] Running 3-second stream diagnostic test (headless)...")
		stats, dErr := streamMgr.DiagnosticCapture(sigCtx, 3*time.Second)
		if dErr != nil {
			fmt.Printf("[DIAG][ERROR] %v\n", dErr)
			return dErr
		}
		if stats.TotalBytes == 0 {
			fmt.Println("[DIAG][ERROR] Case A: Android screen capture failed to produce bytes.")
			return errors.New("stream produced 0 bytes")
		} else if stats.TotalFrames == 0 {
			fmt.Println("[DIAG][ERROR] Case B: Transport received data, but no H.264 video frames were decoded.")
			return errors.New("no frames decoded")
		}
		fmt.Printf("[DIAG] Case C: SUCCESS! Received %d bytes, %d video frames (FPS: %.1f)\n",
			stats.TotalBytes, stats.TotalFrames, stats.CurrentFPS)
		fmt.Println("[DIAG] Video capture and transport pipeline is verified working.")
		return nil
	}

	fmt.Println("[STREAM] Initializing...")

	// 6. Ensure video renderer is available
	_, _, err = display.FindRenderer()
	if err != nil {
		fmt.Printf("[WINDOW][ERROR] Video renderer missing: %v\n", err)
		return fmt.Errorf("video renderer missing: %w\nPlease install mpv or run install.ps1 / install.sh", err)
	}
	fmt.Println("[STREAM] Renderer initialized.")
	fmt.Println("[STREAM] Decoder initialized.")

	// 7. Launch floating portrait mirror window
	winCfg := display.WindowConfig{
		Title:       fmt.Sprintf("XCastPhone - %s", dev.DisplayName()),
		Width:       streamMgr.Width(),
		Height:      streamMgr.Height(),
		AspectRatio: dev.AspectRatio(),
		TargetFPS:   c.config.TargetFPS,
		Verbose:     c.config.Verbose || c.config.Debug,
	}

	winSession, winStdin, err := display.LaunchWindow(sigCtx, winCfg)
	if err != nil {
		fmt.Printf("[WINDOW][ERROR] Failed to launch mirror window: %v\n", err)
		return fmt.Errorf("failed to launch mirror window: %w", err)
	}
	defer winSession.Close()
	fmt.Println("[STREAM] Mirror window created.")

	// 8. Run streaming loop
	streamCtx, cancelStream := context.WithCancel(sigCtx)
	defer cancelStream()

	streamErrChan := make(chan error, 1)
	go func() {
		streamErrChan <- streamMgr.StartStreaming(streamCtx, winStdin)
	}()

	// In debug mode, print real-time stream stats periodically
	if c.config.Debug || c.config.Verbose {
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-streamCtx.Done():
					return
				case <-ticker.C:
					stats := inspector.Stats()
					if stats.TotalBytes > 0 {
						fmt.Printf("[STREAM] Bytes received: %d, Frames: %d (%.1f FPS, %.2f Mbps)\n",
							stats.TotalBytes, stats.TotalFrames, stats.CurrentFPS, stats.CurrentBps/1e6)
					}
				}
			}
		}()
	}

	// 9. Wait for termination triggers
	var terminationReason string

	select {
	case <-sigCtx.Done():
		terminationReason = "user requested termination (Ctrl+C)"
	case wErr := <-winSession.ExitChan:
		if winSession.Stderr != nil && winSession.Stderr.Len() > 0 {
			errText := strings.TrimSpace(winSession.Stderr.String())
			if errText != "" && !strings.Contains(errText, "Exiting...") {
				fmt.Printf("[WINDOW][ERROR] %s\n", errText)
			}
		}
		if wErr != nil {
			fmt.Printf("[PROCESS] Mirror window exited: %v\n", wErr)
			terminationReason = fmt.Sprintf("mirror window closed (%v)", wErr)
		} else {
			fmt.Println("[PROCESS] Mirror window closed by user")
			terminationReason = "mirror window closed by user"
		}
	case sErr := <-streamErrChan:
		if errors.Is(sErr, streaming.ErrScreenOff) {
			terminationReason = "phone screen turned off"
		} else if errors.Is(sErr, streaming.ErrDeviceDisconnected) {
			terminationReason = "android device disconnected"
		} else if sErr != nil && !errors.Is(sErr, streaming.ErrSessionClosed) {
			fmt.Printf("[STREAM][ERROR] %v\n", sErr)
			terminationReason = fmt.Sprintf("stream error: %v", sErr)
		} else {
			terminationReason = "session ended"
		}
	}

	// 10. Clean teardown
	cancelStream()
	_ = winSession.Close()

	fmt.Println()
	fmt.Printf("Session terminated (%s).\n", terminationReason)

	return nil
}

