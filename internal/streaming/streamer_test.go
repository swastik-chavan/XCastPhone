package streaming

import (
	"testing"
)

func TestCalculateOptimalResolution(t *testing.T) {
	tests := []struct {
		name     string
		nativeW  int
		nativeH  int
		maxDim   int
		expectW  int
		expectH  int
	}{
		{
			name:     "Within limit",
			nativeW:  1080,
			nativeH:  2400,
			maxDim:   2400,
			expectW:  1080,
			expectH:  2400,
		},
		{
			name:     "Scaled down 2K",
			nativeW:  1440,
			nativeH:  3200,
			maxDim:   2400,
			expectW:  1080,
			expectH:  2400,
		},
		{
			name:     "Ensure even numbers",
			nativeW:  1081,
			nativeH:  2341,
			maxDim:   2400,
			expectW:  1080,
			expectH:  2340,
		},
	}

	for _, tc := range tests {
		w, h := calculateOptimalResolution(tc.nativeW, tc.nativeH, tc.maxDim)
		if w != tc.expectW || h != tc.expectH {
			t.Errorf("%s: got (%d, %d), want (%d, %d)", tc.name, w, h, tc.expectW, tc.expectH)
		}
		// Codec even dimension check
		if w%2 != 0 || h%2 != 0 {
			t.Errorf("%s: dimensions must be even numbers, got %dx%d", tc.name, w, h)
		}
	}
}
