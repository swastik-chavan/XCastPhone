package decoder

import (
	"bytes"
	"testing"
	"time"
)

func TestStreamInspector(t *testing.T) {
	inspector := NewStreamInspector()
	defer inspector.Close()

	// Construct mock H.264 stream with SPS (7), PPS (8), IDR (5), and NonIDR (1)
	mockStream := []byte{
		0x00, 0x00, 0x01, 0x67, 0x42, 0x00, 0x1f, // SPS
		0x00, 0x00, 0x01, 0x68, 0xce, 0x3c, 0x80, // PPS
		0x00, 0x00, 0x01, 0x65, 0x88, 0x84, 0x00, // IDR slice (keyframe)
		0x00, 0x00, 0x01, 0x61, 0x9a, 0x00, 0x12, // Non-IDR slice
	}

	src := bytes.NewReader(mockStream)
	var dst bytes.Buffer

	err := inspector.PipeAndInspect(src, &dst)
	if err != nil {
		t.Fatalf("unexpected pipe error: %v", err)
	}

	if dst.Len() != len(mockStream) {
		t.Errorf("expected %d bytes forwarded, got %d", len(mockStream), dst.Len())
	}

	stats := inspector.Stats()
	if stats.TotalBytes != uint64(len(mockStream)) {
		t.Errorf("expected stats.TotalBytes = %d, got %d", len(mockStream), stats.TotalBytes)
	}

	if stats.KeyFrames != 1 {
		t.Errorf("expected 1 keyframe detected, got %d", stats.KeyFrames)
	}

	if stats.TotalFrames != 2 {
		t.Errorf("expected 2 total frames detected, got %d", stats.TotalFrames)
	}
}

func TestStreamStatsTimestamps(t *testing.T) {
	inspector := NewStreamInspector()
	defer inspector.Close()

	buf := []byte{0x00, 0x00, 0x01, 0x65}
	_ = inspector.PipeAndInspect(bytes.NewReader(buf), ioDiscard{})

	stats := inspector.Stats()
	if stats.FirstFrameAt.IsZero() || stats.LastFrameAt.IsZero() {
		t.Errorf("frame timestamps should be populated")
	}
	if time.Since(stats.LastFrameAt) > 2*time.Second {
		t.Errorf("last frame timestamp should be recent")
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
