package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"xcastphone/internal/adb"
	"xcastphone/internal/pairing"
)

// MDNSService represents an ADB mDNS service entry.
type MDNSService struct {
	Name    string
	Service string // "_adb-tls-pairing._tcp" or "_adb-tls-connect._tcp"
	Address string // "ip:port"
}

// ParseMDNSServices parses the stdout of `adb mdns services`.
func ParseMDNSServices(output string) []MDNSService {
	var services []MDNSService
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of discovered") || strings.HasPrefix(line, "*") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			services = append(services, MDNSService{
				Name:    fields[0],
				Service: fields[1],
				Address: fields[2],
			})
		}
	}

	return services
}

// DiscoverAndPair manages the device discovery, pairing, and connection lifecycle.
func DiscoverAndPair(ctx context.Context, client *adb.Client, session *pairing.Session, timeout time.Duration) (*adb.Device, error) {
	// 1. Fast check: is an authorized device already connected?
	if dev, err := client.GetFirstAuthorizedDevice(); err == nil && dev != nil {
		return dev, nil
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	pairingAttempted := make(map[string]bool)
	connectAttempted := make(map[string]bool)
	nudgedNetworkWarning := false
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timed out waiting for Android device (no device connected within %v)", timeout)
			}

			// Check if any device is now authorized and ready
			if dev, err := client.GetFirstAuthorizedDevice(); err == nil && dev != nil {
				return dev, nil
			}

			// Check mDNS services
			mdnsOut, err := client.MDNSServices()
			if err == nil {
				services := ParseMDNSServices(mdnsOut)

				// Look for pairing services first
				for _, svc := range services {
					if svc.Service == "_adb-tls-pairing._tcp" && !pairingAttempted[svc.Address] {
						pairingAttempted[svc.Address] = true
						fmt.Printf("\n[+] Found pairing service at %s. Authenticating...\n", svc.Address)

						pairOut, pairErr := client.Pair(svc.Address, session.PairingCode)
						if pairErr == nil && strings.Contains(strings.ToLower(pairOut), "successfully paired") {
							fmt.Printf("[+] Successfully paired with %s!\n", svc.Address)
						}
					}
				}

				// Look for connect services
				for _, svc := range services {
					if svc.Service == "_adb-tls-connect._tcp" && !connectAttempted[svc.Address] {
						connectAttempted[svc.Address] = true
						fmt.Printf("[+] Found connect service at %s. Connecting...\n", svc.Address)

						_, _ = client.Connect(svc.Address)
					}
				}
			}

			// Network warning if waiting > 15s without any mDNS discovery
			if !nudgedNetworkWarning && time.Since(startTime) > 18*time.Second && len(pairingAttempted) == 0 {
				nudgedNetworkWarning = true
				fmt.Println("\n[i] Tip: If your phone is scanning but not connecting:")
				fmt.Println("    1. Ensure phone and PC are on the same Wi-Fi network.")
				fmt.Println("    2. Ensure Wireless Debugging is toggled ON in Developer Options.")
				fmt.Println("    3. If on public/guest Wi-Fi with client isolation, use phone hotspot.")
			}
		}
	}
}
