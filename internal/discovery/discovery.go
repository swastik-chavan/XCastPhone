package discovery

import (
	"context"
	"fmt"
	"net"
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
// It strictly differentiates between ADB pairing ports and connection ports,
// and tolerates temporary network transitions without aborting.
func DiscoverAndPair(ctx context.Context, client *adb.Client, session *pairing.Session, timeout time.Duration, verbose bool) (*adb.Device, error) {
	// 1. Fast check: is an authorized device already connected?
	if dev, err := client.GetFirstAuthorizedDevice(); err == nil && dev != nil {
		fmt.Printf("[ADB] Existing authorized device found: %s\n", dev.DisplayName())
		return dev, nil
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1000 * time.Millisecond)
	defer ticker.Stop()

	var (
		paired               = false
		pairedIP             = ""
		loggedWaitingConnect = false
		lastPairAttempt      = make(map[string]time.Time)
		lastConnectAttempt   = make(map[string]time.Time)
		nudgedNetworkWarning = false
		startTime            = time.Now()
	)

	fmt.Println("[PAIR] Waiting for Android QR pairing...")

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("pairing and connection timed out (no authorized device within %v)", timeout)
			}

			// Stage A: Check if any device has become authorized in ADB
			if dev, err := client.GetFirstAuthorizedDevice(); err == nil && dev != nil {
				fmt.Printf("[ADB] Device authorized: %s\n", dev.DisplayName())
				return dev, nil
			}

			// Stage B: Discover active mDNS services from ADB
			mdnsOut, err := client.MDNSServices()
			if err != nil {
				if verbose {
					fmt.Printf("[DISCOVERY] mDNS poll error: %v (retrying)\n", err)
				}
				continue
			}

			services := ParseMDNSServices(mdnsOut)
			var pairingServices []MDNSService
			var connectServices []MDNSService

			for _, svc := range services {
				if svc.Service == "_adb-tls-pairing._tcp" {
					pairingServices = append(pairingServices, svc)
				} else if svc.Service == "_adb-tls-connect._tcp" {
					connectServices = append(connectServices, svc)
				}
			}

			// Stage C: Handle Pairing if not yet paired
			if !paired {
				for _, pSvc := range pairingServices {
					host, _, hErr := net.SplitHostPort(pSvc.Address)
					if hErr != nil {
						parts := strings.Split(pSvc.Address, ":")
						host = parts[0]
					}

					// Rate-limit pairing attempts to once every 2 seconds per address
					if time.Since(lastPairAttempt[pSvc.Address]) >= 2*time.Second {
						lastPairAttempt[pSvc.Address] = time.Now()
						fmt.Printf("[PAIR] Pairing service detected: %s\n", pSvc.Address)
						fmt.Println("[PAIR] Running ADB pairing...")

						pairOut, pairErr := client.Pair(pSvc.Address, session.PairingCode)
						outLower := strings.ToLower(pairOut)

						if pairErr == nil && (strings.Contains(outLower, "successfully paired") || strings.Contains(outLower, "paired to")) {
							fmt.Println("[PAIR] Pairing successful.")
							paired = true
							pairedIP = host
							break
						} else {
							if verbose {
								fmt.Printf("[PAIR] Pairing attempt output: %s\n", strings.TrimSpace(pairOut))
								if pairErr != nil {
									fmt.Printf("[PAIR] Pairing attempt error: %v\n", pairErr)
								}
							}
						}
					}
				}
			}

			// Stage D: Handle Connection
			if paired {
				if !loggedWaitingConnect {
					fmt.Println("[DISCOVERY] Waiting for ADB connection service...")
					loggedWaitingConnect = true
				}

				for _, cSvc := range connectServices {
					host, _, hErr := net.SplitHostPort(cSvc.Address)
					if hErr != nil {
						parts := strings.Split(cSvc.Address, ":")
						host = parts[0]
					}

					// If we know the paired IP, ensure connect service matches that IP
					if pairedIP != "" && host != pairedIP {
						if verbose {
							fmt.Printf("[DISCOVERY] Skipping unrelated connect service %s (waiting for %s)\n", cSvc.Address, pairedIP)
						}
						continue
					}

					// Rate-limit connection attempts to once every 2 seconds per address
					if time.Since(lastConnectAttempt[cSvc.Address]) >= 2*time.Second {
						lastConnectAttempt[cSvc.Address] = time.Now()
						fmt.Printf("[DISCOVERY] Device service detected: %s\n", cSvc.Address)
						fmt.Printf("[ADB] Connecting to %s...\n", cSvc.Address)

						connOut, connErr := client.Connect(cSvc.Address)
						if verbose {
							if connErr != nil {
								fmt.Printf("[ADB] Connect error: %v\n", connErr)
							} else {
								fmt.Printf("[ADB] Connect output: %s\n", strings.TrimSpace(connOut))
							}
						}

						// Allow brief delay for ADB daemon TLS handshake
						time.Sleep(500 * time.Millisecond)

						// Check if device is now authorized
						if dev, err := client.GetFirstAuthorizedDevice(); err == nil && dev != nil {
							fmt.Println("[ADB] Device authorized.")
							return dev, nil
						}
					}
				}
			} else {
				// Not yet paired in this session, but connect services exist.
				// Check periodically (every 5 seconds) if this device was already paired previously.
				for _, cSvc := range connectServices {
					if time.Since(lastConnectAttempt[cSvc.Address]) >= 5*time.Second {
						lastConnectAttempt[cSvc.Address] = time.Now()
						if verbose {
							fmt.Printf("[DISCOVERY] Connect service %s seen; checking if device is pre-paired...\n", cSvc.Address)
						}

						connOut, _ := client.Connect(cSvc.Address)
						if dev, err := client.GetFirstAuthorizedDevice(); err == nil && dev != nil {
							fmt.Println("[ADB] Device authorized.")
							return dev, nil
						}

						if verbose && strings.Contains(strings.ToLower(connOut), "unauthorized") {
							fmt.Printf("[ADB] Device at %s requires QR pairing\n", cSvc.Address)
						}
					}
				}
			}

			// Helpful troubleshooting guidance if no services are discovered for > 15 seconds
			if !nudgedNetworkWarning && time.Since(startTime) > 15*time.Second && len(pairingServices) == 0 && len(connectServices) == 0 {
				nudgedNetworkWarning = true
				fmt.Println("\n[i] Troubleshooting Tip:")
				fmt.Println("    1. Ensure phone and PC are connected to the same Wi-Fi network.")
				fmt.Println("    2. On Android: Settings > Developer options > Wireless debugging must be ON.")
				fmt.Println("    3. Tap 'Pair device with QR code' in Wireless debugging to scan the QR code.")
				fmt.Println("       (Do NOT scan with standard camera app - standard camera will misidentify it as Wi-Fi network)")
				fmt.Println("    4. If using guest or corporate Wi-Fi with client isolation, use phone hotspot.")
				fmt.Println()
			}
		}
	}
}

