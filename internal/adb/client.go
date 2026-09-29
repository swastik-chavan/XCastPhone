package adb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Client wraps ADB CLI commands.
type Client struct {
	adbPath string
	verbose bool
}

// NewClient finds or verifies the ADB executable on the host system.
func NewClient(verbose bool) (*Client, error) {
	path, err := findADB()
	if err != nil {
		return nil, fmt.Errorf("adb not found: %w\nPlease ensure Android Platform Tools / ADB are installed", err)
	}

	client := &Client{
		adbPath: path,
		verbose: verbose,
	}

	// Ensure adb server is running
	_ = client.StartServer()

	return client, nil
}

// Path returns the path to the adb binary being used.
func (c *Client) Path() string {
	return c.adbPath
}

// StartServer starts the background adb daemon.
func (c *Client) StartServer() error {
	cmd := exec.Command(c.adbPath, "start-server")
	if c.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// Run executes an adb command with arguments and returns stdout as a string.
func (c *Client) Run(args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(c.adbPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("adb %s failed: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// RunDevice executes an adb command targeting a specific device serial.
func (c *Client) RunDevice(serial string, args ...string) (string, error) {
	deviceArgs := append([]string{"-s", serial}, args...)
	return c.Run(deviceArgs...)
}

// Shell runs a shell command on the specified device.
func (c *Client) Shell(serial string, shellCmd string) (string, error) {
	return c.RunDevice(serial, "shell", shellCmd)
}

// ExecOutStream runs an exec-out command and returns an io.ReadCloser streaming binary stdout.
// This is critical for screenrecord streaming without line-ending transformations.
func (c *Client) ExecOutStream(ctx context.Context, serial string, args ...string) (io.ReadCloser, *exec.Cmd, error) {
	cmdArgs := append([]string{"-s", serial, "exec-out"}, args...)
	cmd := exec.CommandContext(ctx, c.adbPath, cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	if c.verbose {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = io.Discard
	}

	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return nil, nil, fmt.Errorf("failed to start adb exec-out: %w", err)
	}

	return stdout, cmd, nil
}

// Pair pairs a device via wireless debugging pairing code.
func (c *Client) Pair(address, code string) (string, error) {
	return c.Run("pair", address, code)
}

// Connect connects to a wireless debugging device.
func (c *Client) Connect(address string) (string, error) {
	return c.Run("connect", address)
}

// Disconnect disconnects a wireless debugging device.
func (c *Client) Disconnect(address string) (string, error) {
	return c.Run("disconnect", address)
}

// MDNSServices lists active mDNS discovered services.
func (c *Client) MDNSServices() (string, error) {
	return c.Run("mdns", "services")
}

// findADB searches for ADB in PATH, environment variables, and standard install locations.
func findADB() (string, error) {
	// 1. Check if adb is in PATH
	if p, err := exec.LookPath("adb"); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("adb.exe"); err == nil {
			return p, nil
		}
	}

	// 2. Check ANDROID_HOME / ANDROID_SDK_ROOT
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		if val := os.Getenv(env); val != "" {
			candidate := filepath.Join(val, "platform-tools", "adb")
			if runtime.GOOS == "windows" {
				candidate += ".exe"
			}
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	// 3. Platform-specific standard locations
	var candidates []string
	home, _ := os.UserHomeDir()

	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			candidates = append(candidates,
				filepath.Join(localAppData, "Android", "Sdk", "platform-tools", "adb.exe"),
			)
		}
		if home != "" {
			candidates = append(candidates,
				filepath.Join(home, "AppData", "Local", "Android", "Sdk", "platform-tools", "adb.exe"),
				filepath.Join(home, ".xcast", "bin", "adb.exe"),
			)
		}
		candidates = append(candidates,
			`C:\Program Files (x86)\Android\android-sdk\platform-tools\adb.exe`,
			`C:\Android\platform-tools\adb.exe`,
		)
	} else if runtime.GOOS == "darwin" {
		if home != "" {
			candidates = append(candidates,
				filepath.Join(home, "Library", "Android", "sdk", "platform-tools", "adb"),
				filepath.Join(home, ".xcast", "bin", "adb"),
			)
		}
		candidates = append(candidates,
			"/usr/local/bin/adb",
			"/opt/homebrew/bin/adb",
		)
	} else { // Linux
		if home != "" {
			candidates = append(candidates,
				filepath.Join(home, "Android", "Sdk", "platform-tools", "adb"),
				filepath.Join(home, ".xcast", "bin", "adb"),
			)
		}
		candidates = append(candidates,
			"/usr/bin/adb",
			"/usr/local/bin/adb",
		)
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	return "", fmt.Errorf("adb executable could not be located in PATH or standard Android SDK directories")
}
