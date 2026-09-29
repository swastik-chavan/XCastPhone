package pairing

import (
	"image"
	_ "image/png"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	session, err := NewSession(60 * time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	if !strings.HasPrefix(session.ServiceName, "xcast-") {
		t.Errorf("expected service name prefix 'xcast-', got: %s", session.ServiceName)
	}

	if len(session.PairingCode) != 6 {
		t.Errorf("expected 6-digit pairing code, got length %d (%s)", len(session.PairingCode), session.PairingCode)
	}

	expectedPrefix := "WIFI:T:ADB;S:"
	if !strings.HasPrefix(session.Payload, expectedPrefix) {
		t.Errorf("payload should start with '%s', got: %s", expectedPrefix, session.Payload)
	}

	if !strings.HasSuffix(session.Payload, ";;") {
		t.Errorf("payload should end with ';;', got: %s", session.Payload)
	}

	if session.IsExpired() {
		t.Errorf("newly created session should not be expired")
	}
}

func TestRenderTerminalQR(t *testing.T) {
	session, err := NewSession(60 * time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	qr, err := session.RenderTerminalQR()
	if err != nil {
		t.Fatalf("failed to render QR: %v", err)
	}

	if len(qr) == 0 {
		t.Errorf("rendered QR should not be empty")
	}

	// Should contain unicode block characters
	if !strings.Contains(qr, "█") && !strings.Contains(qr, "▀") && !strings.Contains(qr, "▄") {
		t.Errorf("expected unicode half-blocks in rendered QR, got:\n%s", qr)
	}
}

func TestGenerateQRImage(t *testing.T) {
	session, err := NewSession(60 * time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	targetDim := 800
	img, size, err := session.GenerateQRImage(targetDim)
	if err != nil {
		t.Fatalf("unexpected error generating QR image: %v", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// 1. Must be mathematically square (1:1 aspect ratio)
	if w != h {
		t.Fatalf("QR image is not square: %dx%d", w, h)
	}
	if w != size {
		t.Fatalf("image width %d does not match returned size %d", w, size)
	}

	// 2. Must be close to target resolution (integer-scaled)
	if w < 600 || w > 1000 {
		t.Errorf("unexpected image size: %d (expected ~800)", w)
	}

	// 3. Crisp pixels: only pure black or pure white, no semi-transparent or gray anti-aliased pixels
	hasBlack := false
	hasWhite := false
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// 16-bit values from RGBA(): pure black is 0, pure white is 0xffff
			isBlack := (r == 0 && g == 0 && b == 0 && a == 0xffff)
			isWhite := (r == 0xffff && g == 0xffff && b == 0xffff && a == 0xffff)
			if !isBlack && !isWhite {
				t.Fatalf("non-binary pixel found at (%d, %d): r=%d g=%d b=%d a=%d (must be pure black or white)", x, y, r, g, b, a)
			}
			if isBlack {
				hasBlack = true
			}
			if isWhite {
				hasWhite = true
			}
		}
	}

	if !hasBlack || !hasWhite {
		t.Fatalf("image must contain both black modules and white background (hasBlack=%v, hasWhite=%v)", hasBlack, hasWhite)
	}

	// 4. Quiet zone: outer border (pixel 0,0) must be white
	r0, g0, b0, a0 := img.At(0, 0).RGBA()
	if !(r0 == 0xffff && g0 == 0xffff && b0 == 0xffff && a0 == 0xffff) {
		t.Errorf("quiet zone top-left corner is not white: r=%d g=%d b=%d a=%d", r0, g0, b0, a0)
	}
}

func TestCreateTempQRImageFile(t *testing.T) {
	session, err := NewSession(60 * time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating session: %v", err)
	}

	filePath, err := session.CreateTempQRImageFile(800)
	if err != nil {
		t.Fatalf("failed to create temp QR image file: %v", err)
	}
	defer func() {
		_ = os.Remove(filePath)
	}()

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("temp QR file does not exist: %v", err)
	}

	if info.Size() == 0 {
		t.Fatalf("temp QR file is empty")
	}

	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open temp QR file: %v", err)
	}
	defer f.Close()

	imgConfig, format, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("failed to decode temp PNG config: %v", err)
	}

	if format != "png" {
		t.Errorf("expected PNG format, got: %s", format)
	}

	if imgConfig.Width != imgConfig.Height {
		t.Errorf("PNG image is not square: %dx%d", imgConfig.Width, imgConfig.Height)
	}
}

