package pairing

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"golang.org/x/term"
)

// Session represents a temporary wireless debugging pairing session.
type Session struct {
	ServiceName string
	PairingCode string
	Payload     string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// NewSession generates a fresh temporary QR pairing session with random credentials.
func NewSession(lifetime time.Duration) (*Session, error) {
	// Generate random 8-character hex suffix for service name
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random service name: %w", err)
	}
	serviceName := fmt.Sprintf("xcast-%s", hex.EncodeToString(b))

	// Generate 6-digit pairing code (100000 - 999999)
	nBig, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return nil, fmt.Errorf("failed to generate random pairing code: %w", err)
	}
	code := fmt.Sprintf("%06d", nBig.Int64()+100000)

	// Standard Android Wireless Debugging QR format
	// WIFI:T:ADB;S:<service-name>;P:<pairing-code>;;
	payload := fmt.Sprintf("WIFI:T:ADB;S:%s;P:%s;;", serviceName, code)

	now := time.Now()
	return &Session{
		ServiceName: serviceName,
		PairingCode: code,
		Payload:     payload,
		CreatedAt:   now,
		ExpiresAt:   now.Add(lifetime),
	}, nil
}

// IsExpired checks if the temporary pairing session has timed out.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// GenerateQRImage generates an exact square, integer-scaled QR image without anti-aliasing or interpolation.
// It uses error-correction level Medium (M) per ADB wireless pairing specification.
// targetDimension is the approximate target size in pixels (e.g. 800); integer module scaling is strictly preserved.
func (s *Session) GenerateQRImage(targetDimension int) (image.Image, int, error) {
	if targetDimension <= 0 {
		targetDimension = 800
	}

	qr, err := qrcode.New(s.Payload, qrcode.Medium)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to generate QR code: %w", err)
	}

	bitmap := qr.Bitmap()
	numModules := len(bitmap)
	if numModules == 0 {
		return nil, 0, fmt.Errorf("generated QR bitmap is empty")
	}

	// Calculate integer module size in pixels so no fractional scaling or subpixel blur occurs
	modulePx := targetDimension / numModules
	if modulePx < 8 {
		modulePx = 8
	}

	imgSize := numModules * modulePx
	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))

	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	for y := 0; y < imgSize; y++ {
		moduleY := y / modulePx
		for x := 0; x < imgSize; x++ {
			moduleX := x / modulePx
			if bitmap[moduleY][moduleX] {
				img.SetRGBA(x, y, black)
			} else {
				img.SetRGBA(x, y, white)
			}
		}
	}

	return img, imgSize, nil
}

// CreateTempQRImageFile generates a mathematically square QR code and saves it to a temporary PNG file.
// The caller is responsible for deleting the file when no longer needed.
func (s *Session) CreateTempQRImageFile(targetDimension int) (string, error) {
	img, _, err := s.GenerateQRImage(targetDimension)
	if err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp("", "xcast-pairing-qr-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary QR file: %w", err)
	}
	defer tmpFile.Close()

	if err := png.Encode(tmpFile, img); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to encode QR image as PNG: %w", err)
	}

	return tmpFile.Name(), nil
}

// RenderTerminalQR renders the fallback QR code for terminal display.
func (s *Session) RenderTerminalQR() (string, error) {
	// Check terminal dimensions if stdout is a terminal
	if term.IsTerminal(int(os.Stdout.Fd())) {
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err == nil {
			if width < 50 || height < 20 {
				fmt.Println("[!] Notice: Terminal window appears small. Please enlarge terminal if QR is truncated.")
			}
		}
	}

	qr, err := qrcode.New(s.Payload, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Generate compact half-block string (renders 2 vertical pixels per line)
	// Inverting ensures black modules on white background for phone camera scannability
	rawSmall := qr.ToSmallString(true)

	var sb strings.Builder
	lines := strings.Split(strings.TrimRight(rawSmall, "\n"), "\n")

	for _, line := range lines {
		// Indent 2 spaces for visual padding
		sb.WriteString("  ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

