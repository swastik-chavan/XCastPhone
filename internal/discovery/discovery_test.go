package discovery

import (
	"testing"
)

func TestParseMDNSServices(t *testing.T) {
	rawOutput := `List of discovered mdns services
adb-pair-1234	_adb-tls-pairing._tcp	192.168.1.150:37123
adb-conn-5678	_adb-tls-connect._tcp	192.168.1.150:41982
`
	services := ParseMDNSServices(rawOutput)

	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	if services[0].Service != "_adb-tls-pairing._tcp" || services[0].Address != "192.168.1.150:37123" {
		t.Errorf("service 0 mismatch: %+v", services[0])
	}

	if services[1].Service != "_adb-tls-connect._tcp" || services[1].Address != "192.168.1.150:41982" {
		t.Errorf("service 1 mismatch: %+v", services[1])
	}
}
