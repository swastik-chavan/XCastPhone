package decoder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// NALUnitType represents standard H.264 NAL unit types.
type NALUnitType byte

const (
	NALTypeNonIDR NALUnitType = 1
	NALTypeIDR    NALUnitType = 5
	NALTypeSEI    NALUnitType = 6
	NALTypeSPS    NALUnitType = 7
	NALTypePPS    NALUnitType = 8
	NALTypeAUD    NALUnitType = 9
)

// StreamStats tracks real-time video stream performance metrics.
type StreamStats struct {
	TotalBytes   uint64
	TotalFrames  uint64
	KeyFrames    uint64
	CurrentBps   float64
	CurrentFPS   float64
	LastFrameAt  time.Time
	FirstFrameAt time.Time
}

// StreamInspector validates incoming H.264 bitstream and tracks real-time performance.
type StreamInspector struct {
	mu              sync.RWMutex
	totalBytes      atomic.Uint64
	totalFrames     atomic.Uint64
	keyFrames       atomic.Uint64
	fpsCounter      atomic.Uint64
	byteCounter     atomic.Uint64
	lastFrameAt     time.Time
	firstFrameAt    time.Time
	currentBps      float64
	currentFPS      float64
	stopChan        chan struct{}
	firstFrameChan  chan struct{}
	firstFrameOnce  sync.Once
}

// NewStreamInspector creates a stream inspector for validating H.264 frames and collecting stats.
func NewStreamInspector() *StreamInspector {
	si := &StreamInspector{
		stopChan:       make(chan struct{}),
		firstFrameChan: make(chan struct{}),
	}

	// Metrics calculation loop (every 1 second)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-si.stopChan:
				return
			case <-ticker.C:
				frames := si.fpsCounter.Swap(0)
				bytesCount := si.byteCounter.Swap(0)

				si.mu.Lock()
				si.currentFPS = float64(frames)
				si.currentBps = float64(bytesCount) * 8.0 // bits per second
				si.mu.Unlock()
			}
		}
	}()

	return si
}

// Close stops the metric ticker.
func (si *StreamInspector) Close() {
	select {
	case <-si.stopChan:
	default:
		close(si.stopChan)
	}
}

// FirstFrameChan returns a channel that is closed when the first valid video frame/header arrives.
func (si *StreamInspector) FirstFrameChan() <-chan struct{} {
	return si.firstFrameChan
}

// FirstFrameReceived returns true if at least one video frame has been processed.
func (si *StreamInspector) FirstFrameReceived() bool {
	select {
	case <-si.firstFrameChan:
		return true
	default:
		return false
	}
}

// Stats returns a snapshot of current stream metrics.
func (si *StreamInspector) Stats() StreamStats {
	si.mu.RLock()
	defer si.mu.RUnlock()

	return StreamStats{
		TotalBytes:   si.totalBytes.Load(),
		TotalFrames:  si.totalFrames.Load(),
		KeyFrames:    si.keyFrames.Load(),
		CurrentBps:   si.currentBps,
		CurrentFPS:   si.currentFPS,
		LastFrameAt:  si.lastFrameAt,
		FirstFrameAt: si.firstFrameAt,
	}
}

// PipeAndInspect forwards the raw H.264 stream from src to dst while inspecting NAL units.
func (si *StreamInspector) PipeAndInspect(src io.Reader, dst io.Writer) error {
	buf := make([]byte, 64*1024)

	for {
		n, err := src.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			si.totalBytes.Add(uint64(n))
			si.byteCounter.Add(uint64(n))

			// Scan for H.264 NAL start codes (0x000001 or 0x00000001)
			now := time.Now()
			si.mu.Lock()
			if si.firstFrameAt.IsZero() {
				si.firstFrameAt = now
			}
			si.lastFrameAt = now
			si.mu.Unlock()

			si.detectNALUnits(chunk)

			// Forward to destination (the display window stdin pipe)
			if _, werr := dst.Write(chunk); werr != nil {
				return fmt.Errorf("failed writing to video pipe: %w", werr)
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				if si.totalBytes.Load() == 0 {
					return errors.New("stream closed without producing any video data (0 bytes received)")
				}
				return nil
			}
			return err
		}
	}
}

// detectNALUnits scans for Annex-B start prefixes and counts keyframes and slices.
func (si *StreamInspector) detectNALUnits(data []byte) {
	start3 := []byte{0x00, 0x00, 0x01}
	idx := 0

	for {
		pos := bytes.Index(data[idx:], start3)
		if pos == -1 {
			break
		}

		nalPos := idx + pos + 3
		if nalPos < len(data) {
			nalHeader := data[nalPos]
			nalType := NALUnitType(nalHeader & 0x1F)

			switch nalType {
			case NALTypeIDR:
				si.keyFrames.Add(1)
				si.totalFrames.Add(1)
				si.fpsCounter.Add(1)
				si.firstFrameOnce.Do(func() { close(si.firstFrameChan) })
			case NALTypeNonIDR:
				si.totalFrames.Add(1)
				si.fpsCounter.Add(1)
				si.firstFrameOnce.Do(func() { close(si.firstFrameChan) })
			case NALTypeSPS, NALTypePPS:
				si.firstFrameOnce.Do(func() { close(si.firstFrameChan) })
			}
		}

		idx = nalPos
	}
}

