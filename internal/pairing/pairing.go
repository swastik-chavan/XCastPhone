package pairing

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

// RenderTerminalQR renders the QR code for terminal display and checks terminal dimensions.
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
