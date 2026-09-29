package adb

import (
	"testing"
)

func TestDeviceProperties(t *testing.T) {
	dev := &Device{
		Serial:       "192.168.1.50:41235",
		State:        "device",
		Manufacturer: "google",
		Model:        "Pixel 8",
		Width:        1080,
		Height:       2400,
	}

	if dev.DisplayName() != "Google Pixel 8" {
		t.Errorf("expected 'Google Pixel 8', got: '%s'", dev.DisplayName())
	}

	if dev.ResolutionString() != "1080x2400" {
		t.Errorf("expected '1080x2400', got: '%s'", dev.ResolutionString())
	}

	expectedRatio := 1080.0 / 2400.0
	if dev.AspectRatio() != expectedRatio {
		t.Errorf("expected aspect ratio %f, got: %f", expectedRatio, dev.AspectRatio())
	}
}

func TestParseResolution(t *testing.T) {
	tests := []struct {
		input  string
		wantW  int
		wantH  int
	}{
		{"Physical size: 1080x2400", 1080, 2400},
		{"Physical size: 1440x3120\nOverride size: 1080x2340", 1080, 2340},
		{"Unknown response", 0, 0},
	}

	for _, tc := range tests {
		w, h := parseResolution(tc.input)
		if w != tc.wantW || h != tc.wantH {
			t.Errorf("parseResolution(%q) = (%d, %d), want (%d, %d)", tc.input, w, h, tc.wantW, tc.wantH)
		}
	}
}
