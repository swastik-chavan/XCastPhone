package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
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
		fmt.Println("XCastPhone")
		fmt.Println()
		fmt.Println("Waiting for Android device...")
		fmt.Println()

		pairSession, err := pairing.NewSession(c.config.PairTimeout)
		if err != nil {
			return fmt.Errorf("failed to generate pairing session: %w", err)
		}

		qrString, err := pairSession.RenderTerminalQR()
		if err != nil {
			return fmt.Errorf("failed to render QR code: %w", err)
		}

		fmt.Print(qrString)
		fmt.Println()
		fmt.Println("Scan the QR code using Android Wireless Debugging.")
		fmt.Println("(Settings > Developer options > Wireless debugging > Pair device with QR code)")
		fmt.Println()
		fmt.Println("Waiting for connection...")

		// Discover and pair
		dev, err = discovery.DiscoverAndPair(sigCtx, client, pairSession, c.config.PairTimeout)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Println("\nPairing aborted by user.")
				return nil
			}
			return fmt.Errorf("pairing failed: %w", err)
		}
	}

	// 4. Authorized device confirmed
	_ = client.PopulateDeviceInfo(dev)

	fmt.Println()
	fmt.Println("XCastPhone")
	fmt.Println()
	fmt.Printf("Device:     %s\n", dev.DisplayName())
	fmt.Printf("Resolution: %s\n", dev.ResolutionString())
	fmt.Printf("FPS:        %d\n", c.config.TargetFPS)
	fmt.Println("Status:     Connected")
	fmt.Println()
	fmt.Println("Casting... (Press Ctrl+C to terminate)")

	// 5. Ensure video renderer is available
	_, _, err = display.FindRenderer()
	if err != nil {
		return fmt.Errorf("video renderer missing: %w\nPlease install mpv or run install.ps1 / install.sh", err)
	}

	// 6. Setup streaming pipeline
	inspector := decoder.NewStreamInspector()
	defer inspector.Close()

	streamCfg := streaming.StreamConfig{
		Device:       dev,
		Bitrate:      c.config.Bitrate,
		MaxDimension: c.config.MaxDimension,
		TargetFPS:    c.config.TargetFPS,
		Verbose:      c.config.Verbose,
	}

	streamMgr := streaming.NewStreamManager(client, streamCfg, inspector)

	// 7. Launch floating portrait mirror window
	winCfg := display.WindowConfig{
		Title:       fmt.Sprintf("XCastPhone - %s", dev.DisplayName()),
		Width:       streamMgr.Width(),
		Height:      streamMgr.Height(),
		AspectRatio: dev.AspectRatio(),
		TargetFPS:   c.config.TargetFPS,
		Verbose:     c.config.Verbose,
	}

	winSession, winStdin, err := display.LaunchWindow(sigCtx, winCfg)
	if err != nil {
		return fmt.Errorf("failed to launch mirror window: %w", err)
	}
	defer winSession.Close()

	// 8. Run streaming loop
	streamCtx, cancelStream := context.WithCancel(sigCtx)
	defer cancelStream()

	streamErrChan := make(chan error, 1)
	go func() {
		streamErrChan <- streamMgr.StartStreaming(streamCtx, winStdin)
	}()

	// 9. Wait for termination triggers:
	// - Ctrl+C / SIGTERM
	// - Phone screen off / disconnect
	// - Window closed by user
	var terminationReason string

	select {
	case <-sigCtx.Done():
		terminationReason = "user requested termination"
	case wErr := <-winSession.ExitChan:
		if wErr != nil {
			terminationReason = "mirror window closed"
		} else {
			terminationReason = "mirror window closed"
		}
	case sErr := <-streamErrChan:
		if errors.Is(sErr, streaming.ErrScreenOff) {
			terminationReason = "phone screen turned off"
		} else if errors.Is(sErr, streaming.ErrDeviceDisconnected) {
			terminationReason = "android device disconnected"
		} else if sErr != nil && !errors.Is(sErr, streaming.ErrSessionClosed) {
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
