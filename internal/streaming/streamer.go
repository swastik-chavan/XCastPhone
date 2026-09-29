package streaming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"xcastphone/internal/adb"
	"xcastphone/internal/decoder"
)

var (
	ErrScreenOff          = errors.New("android phone screen turned off")
	ErrDeviceDisconnected = errors.New("android device disconnected")
	ErrSessionClosed      = errors.New("casting session closed")
)

// StreamConfig specifies parameters for the screen capture stream.
type StreamConfig struct {
	Device       *adb.Device
	Bitrate      int // in bps, e.g. 8000000 (8 Mbps)
	MaxDimension int // e.g. 1920, 2400
	TargetFPS    int // e.g. 60
	Verbose      bool
}

// StreamManager coordinates continuous screen capture and pipe forwarding.
type StreamManager struct {
	client    *adb.Client
	config    StreamConfig
	inspector *decoder.StreamInspector
	width     int
	height    int
}

// NewStreamManager creates a manager for screen recording and streaming.
func NewStreamManager(client *adb.Client, cfg StreamConfig, inspector *decoder.StreamInspector) *StreamManager {
	if cfg.Bitrate <= 0 {
		cfg.Bitrate = 8000000 // 8 Mbps default for crisp 60fps
	}
	if cfg.TargetFPS <= 0 {
		cfg.TargetFPS = 60
	}
	if cfg.MaxDimension <= 0 {
		cfg.MaxDimension = 2400
	}

	w, h := calculateOptimalResolution(cfg.Device.Width, cfg.Device.Height, cfg.MaxDimension)

	return &StreamManager{
		client:    client,
		config:    cfg,
		inspector: inspector,
		width:     w,
		height:    h,
	}
}

// Width returns the active streaming width.
func (sm *StreamManager) Width() int {
	return sm.width
}

// Height returns the active streaming height.
func (sm *StreamManager) Height() int {
	return sm.height
}

// StartStreaming initiates the video streaming loop, forwarding frames to dstWriter.
// It automatically renews the screenrecord command upon normal 180s expiry,
// and terminates immediately if the phone screen is turned off or device disconnects.
func (sm *StreamManager) StartStreaming(ctx context.Context, dstWriter io.WriteCloser) error {
	defer dstWriter.Close()

	// Screen-off background monitor
	screenOffChan := make(chan error, 1)
	monitorCtx, cancelMonitor := context.WithCancel(ctx)
	defer cancelMonitor()

	go sm.monitorScreenState(monitorCtx, screenOffChan)

	consecutiveErrors := 0

	for {
		select {
		case <-ctx.Done():
			return ErrSessionClosed
		case err := <-screenOffChan:
			return err
		default:
		}

		// Check screen power before starting each screenrecord cycle
		state, pErr := sm.client.CheckScreenState(sm.config.Device.Serial)
		if pErr == nil && !state.IsOn {
			return fmt.Errorf("%w (%s)", ErrScreenOff, state.Details)
		}

		// Run adb exec-out screenrecord
		sizeArg := fmt.Sprintf("%dx%d", sm.width, sm.height)
		bitrateArg := fmt.Sprintf("%d", sm.config.Bitrate)

		args := []string{
			"screenrecord",
			"--output-format=h264",
			"--size", sizeArg,
			"--bit-rate", bitrateArg,
			"--time-limit", "180",
			"-",
		}

		streamCtx, cancelStream := context.WithCancel(ctx)

		stdoutPipe, cmd, err := sm.client.ExecOutStream(streamCtx, sm.config.Device.Serial, args...)
		if err != nil {
			cancelStream()
			consecutiveErrors++
			if consecutiveErrors > 3 {
				return fmt.Errorf("failed to start screen capture: %w", err)
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		consecutiveErrors = 0

		// Forward stdout to dstWriter via inspector
		pipeErrChan := make(chan error, 1)
		go func() {
			pipeErrChan <- sm.inspector.PipeAndInspect(stdoutPipe, dstWriter)
		}()

		// Wait for either:
		// 1. Context cancelled (user abort)
		// 2. Screen off detected by monitor
		// 3. Pipe reached EOF or error (e.g., screenrecord 180s finished or crashed)
		var loopErr error
		select {
		case <-ctx.Done():
			loopErr = ErrSessionClosed
		case err := <-screenOffChan:
			loopErr = err
		case pErr := <-pipeErrChan:
			loopErr = pErr
		}

		// Clean up stream process
		cancelStream()
		_ = stdoutPipe.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()

		if errors.Is(loopErr, ErrScreenOff) {
			return loopErr
		}
		if errors.Is(loopErr, ErrSessionClosed) {
			return nil
		}

		// If the stream ended, check whether it was because the phone screen was turned off
		state, pErr = sm.client.CheckScreenState(sm.config.Device.Serial)
		if pErr == nil && !state.IsOn {
			return fmt.Errorf("%w: display turned off during casting", ErrScreenOff)
		}

		// If pipe error indicates broken pipe to player window, user closed window
		if isPipeClosedError(loopErr) {
			return ErrSessionClosed
		}

		// Otherwise, loop seamlessly continues for next 180s block
		time.Sleep(100 * time.Millisecond)
	}
}

// monitorScreenState periodically checks dumpsys power and display state.
func (sm *StreamManager) monitorScreenState(ctx context.Context, errChan chan<- error) {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state, err := sm.client.CheckScreenState(sm.config.Device.Serial)
			if err != nil {
				// Device might have disconnected
				devices, dErr := sm.client.ListDevices()
				if dErr == nil {
					found := false
					for _, d := range devices {
						if d.Serial == sm.config.Device.Serial && d.State == "device" {
							found = true
							break
						}
					}
					if !found {
						select {
						case errChan <- ErrDeviceDisconnected:
						default:
						}
						return
					}
				}
				continue
			}

			if !state.IsOn {
				select {
				case errChan <- fmt.Errorf("%w (%s)", ErrScreenOff, state.Details):
				default:
				}
				return
			}
		}
	}
}

func calculateOptimalResolution(nativeW, nativeH, maxDim int) (int, int) {
	if nativeW <= 0 || nativeH <= 0 {
		return 1080, 1920
	}

	w, h := nativeW, nativeH
	// Scale down proportionally if larger than maxDim
	if h > maxDim {
		w = int(float64(w) * float64(maxDim) / float64(h))
		h = maxDim
	}

	// Codecs require dimensions to be even numbers
	w = (w / 2) * 2
	h = (h / 2) * 2

	if w < 320 {
		w = 320
	}
	if h < 640 {
		h = 640
	}

	return w, h
}

func isPipeClosedError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, exec.ErrNotFound) ||
		len(s) > 0 && (s == "broken pipe" || s == "file already closed")
}
