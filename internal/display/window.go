package display

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

// RendererType specifies which multimedia backend is active.
type RendererType string

const (
	RendererMPV    RendererType = "mpv"
	RendererFFplay RendererType = "ffplay"
)

// WindowConfig configures the floating mirror window.
type WindowConfig struct {
	Title       string
	Width       int
	Height      int
	AspectRatio float64
	TargetFPS   int
	Verbose     bool
}

// WindowSession represents an active native mirror window.
type WindowSession struct {
	Renderer RendererType
	Cmd      *exec.Cmd
	Stdin    io.WriteCloser
	ExitChan chan error
}

// FindRenderer locates an available low-latency video renderer.
func FindRenderer() (string, RendererType, error) {
	home, _ := os.UserHomeDir()

	// 1. Check ~/.xcast/bin for mpv
	if home != "" {
		localMpv := filepath.Join(home, ".xcast", "bin", "mpv")
		if runtime.GOOS == "windows" {
			localMpv += ".exe"
		}
		if _, err := os.Stat(localMpv); err == nil {
			return localMpv, RendererMPV, nil
		}
	}

	// 2. Check PATH for mpv
	if p, err := exec.LookPath("mpv"); err == nil {
		return p, RendererMPV, nil
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("mpv.exe"); err == nil {
			return p, RendererMPV, nil
		}
	}

	// 3. Check ~/.xcast/bin for ffplay
	if home != "" {
		localFFplay := filepath.Join(home, ".xcast", "bin", "ffplay")
		if runtime.GOOS == "windows" {
			localFFplay += ".exe"
		}
		if _, err := os.Stat(localFFplay); err == nil {
			return localFFplay, RendererFFplay, nil
		}
	}

	// 4. Check PATH for ffplay
	if p, err := exec.LookPath("ffplay"); err == nil {
		return p, RendererFFplay, nil
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("ffplay.exe"); err == nil {
			return p, RendererFFplay, nil
		}
	}

	return "", "", fmt.Errorf("no supported video renderer found (mpv or ffplay)\nPlease install mpv or run the installer to set up the renderer")
}

// LaunchWindow creates and starts the floating portrait mirror window.
func LaunchWindow(ctx context.Context, cfg WindowConfig) (*WindowSession, io.WriteCloser, error) {
	rendererPath, rType, err := FindRenderer()
	if err != nil {
		return nil, nil, err
	}

	// Determine optimal initial window dimensions for portrait display
	initialHeight := 800
	initialWidth := int(float64(initialHeight) * cfg.AspectRatio)
	if initialWidth < 320 {
		initialWidth = 360
	}
	if initialWidth > 600 {
		initialWidth = 450
	}

	var args []string

	if rType == RendererMPV {
		args = []string{
			fmt.Sprintf("--title=%s", cfg.Title),
			"--no-border",         // Borderless floating window per requirement
			"--keepaspect=yes",    // Strictly preserve phone aspect ratio
			fmt.Sprintf("--autofit=%dx%d", initialWidth, initialHeight),
			"--profile=low-latency",
			"--untimed",
			"--video-sync=desync",
			"--cache=no",
			"--demuxer-lavf-probesize=32",
			"--demuxer-lavf-analyzeduration=0",
			"--no-osc",     // Clean screen without on-screen controller
			"--no-osd-bar", // No progress bar
			"--idle=no",
			"--window-scale=1.0",
		}
		if cfg.TargetFPS > 0 {
			args = append(args, fmt.Sprintf("--fps=%d", cfg.TargetFPS))
		}
		args = append(args, "-") // read h264 bitstream from stdin
	} else {
		// FFplay fallback
		args = []string{
			"-window_title", cfg.Title,
			"-noborder",
			"-probesize", "32",
			"-sync", "video",
			"-framerate", fmt.Sprintf("%d", cfg.TargetFPS),
			"-x", fmt.Sprintf("%d", initialWidth),
			"-y", fmt.Sprintf("%d", initialHeight),
			"-",
		}
	}

	cmd := exec.CommandContext(ctx, rendererPath, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open stdin pipe for renderer: %w", err)
	}

	if cfg.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, nil, fmt.Errorf("failed to launch renderer (%s): %w", rendererPath, err)
	}

	session := &WindowSession{
		Renderer: rType,
		Cmd:      cmd,
		Stdin:    stdin,
		ExitChan: make(chan error, 1),
	}

	go func() {
		session.ExitChan <- cmd.Wait()
	}()

	return session, stdin, nil
}

// Close gracefully terminates the display window.
func (w *WindowSession) Close() error {
	if w.Stdin != nil {
		_ = w.Stdin.Close()
	}
	if w.Cmd != nil && w.Cmd.Process != nil {
		_ = w.Cmd.Process.Kill()
	}
	return nil
}

// QRWindowSession represents the native window displaying the pairing QR code.
type QRWindowSession struct {
	Cmd       *exec.Cmd
	FilePath  string
	ExitChan  chan error
	closeOnce sync.Once
}

// Close gracefully terminates the QR window and removes the temporary QR image file.
func (q *QRWindowSession) Close() error {
	var err error
	q.closeOnce.Do(func() {
		if q.Cmd != nil && q.Cmd.Process != nil {
			_ = q.Cmd.Process.Kill()
		}
		if q.FilePath != "" {
			_ = os.Remove(q.FilePath)
		}
	})
	return err
}

// LaunchQRWindow opens a native desktop window displaying the square QR code for wireless debugging.
// It uses mpv if available, preserving 1:1 aspect ratio, integer scaling, and sharp rendering.
// If mpv/ffplay is not available, it falls back to opening the image using the OS desktop viewer.
func LaunchQRWindow(ctx context.Context, imgPath string, verbose bool) (*QRWindowSession, error) {
	rendererPath, rType, err := FindRenderer()
	if err == nil {
		var args []string
		if rType == RendererMPV {
			args = []string{
				"--title=XCastPhone - Wireless Pairing",
				"--keepaspect=yes",
				"--geometry=600x600",
				"--autofit=600x600",
				"--image-display-duration=inf",
				"--loop-file=inf",
				"--force-window=yes",
				"--no-osc",
				"--no-osd-bar",
				"--scale=nearest",
				"--background-color=#FFFFFF",
				imgPath,
			}
		} else { // RendererFFplay
			args = []string{
				"-window_title", "XCastPhone - Wireless Pairing",
				"-x", "600",
				"-y", "600",
				"-loop", "0",
				imgPath,
			}
		}

		cmd := exec.CommandContext(ctx, rendererPath, args...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard

		if err := cmd.Start(); err == nil {
			session := &QRWindowSession{
				Cmd:      cmd,
				FilePath: imgPath,
				ExitChan: make(chan error, 1),
			}
			go func() {
				session.ExitChan <- cmd.Wait()
			}()
			return session, nil
		}
	}

	// Desktop fallback per Part 4 if no renderer or launch failed
	var fallbackCmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		fallbackCmd = exec.CommandContext(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", imgPath)
	case "darwin":
		fallbackCmd = exec.CommandContext(ctx, "open", imgPath)
	default:
		fallbackCmd = exec.CommandContext(ctx, "xdg-open", imgPath)
	}

	if err := fallbackCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to open QR code window: %w", err)
	}

	session := &QRWindowSession{
		Cmd:      fallbackCmd,
		FilePath: imgPath,
		ExitChan: make(chan error, 1),
	}
	go func() {
		session.ExitChan <- fallbackCmd.Wait()
	}()

	return session, nil
}

