package pairing

import (
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
