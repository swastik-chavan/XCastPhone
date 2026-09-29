# XCastPhone

<div align="center">

**Terminal-First Android Screen-Casting**

*Cast your Android screen onto your PC with zero phone apps, zero accounts, and zero cloud services.*

```text
Install once  →  xcast  →  Scan QR code  →  Floating portrait mirror
```

</div>

---

## What is XCastPhone?

**XCastPhone** is a minimalist, terminal-first Android screen-casting utility for Windows, macOS, and Linux written entirely in Go.

With XCastPhone, you run a single command in your terminal:

```bash
xcast
```

A temporary QR code appears in your terminal. You scan it using Android's native **Wireless Debugging** menu in Developer Options. Once authorized, a native, borderless floating portrait window opens displaying your Android phone's screen live at up to 60 FPS in crisp 1080p/2K resolution.

**No Android app or APK is ever installed on your phone.** The phone is purely the streaming source; your PC handles the pairing, discovery, H.264 decoding, and floating mirror display.

---

## Why It Exists

Most screen-mirroring and casting solutions fall into two problematic categories:
1. **Bloated Commercial Utilities**: Require user accounts, paid licenses, cloud relays, invasive companion APKs, or bundled adware.
2. **Heavyweight Desktop Suites**: Demand gigabytes of dependencies, Electron runtimes, Node.js, or complex manual configuration just to mirror a phone screen.

XCastPhone strips away everything except what matters: **high-quality, low-latency, real-time screen mirroring in a lightweight, floating portrait window directly from your terminal.**

---

## Features

- **No Android APK / Companion App**: Operates entirely over Android's native ADB and Wireless Debugging protocols.
- **Terminal QR Pairing**: Pair securely by scanning a temporary, high-contrast QR code rendered directly in your terminal.
- **Floating Portrait Window**: Borderless, movable, resizable mirror window that preserves your device's natural aspect ratio without fake phone bezels or distracting UI elements.
- **High Performance & Low Latency**: Hardware-accelerated H.264 video decoding targeting 60 FPS and 1080p/2K resolution.
- **Screen-Off Termination**: Automatically detects when the phone's physical display turns off or sleeps and cleanly terminates the mirror session.
- **Zero Cloud / Local-Only**: All communication is strictly local between your computer and phone over Wi-Fi (or USB).
- **Single Native Executable**: Built with Go for fast startup, minimal memory footprint, and simple distribution.

---

## Core Architecture & Workflow

```text
┌────────────────┐
│  Install Once  │
└───────┬────────┘
        ▼
   ┌─────────┐
   │  xcast  │
   └────┬────┘
        ▼
┌──────────────────────────────────────┐
│ Temporary QR Generated in Terminal   │
└──────────────────┬───────────────────┘
                   ▼
┌──────────────────────────────────────┐
│ Phone Scans QR (Wireless Debugging)  │
└──────────────────┬───────────────────┘
                   ▼
┌──────────────────────────────────────┐
│ PC Handshakes & Authorizes via mDNS  │
└──────────────────┬───────────────────┘
                   ▼
┌──────────────────────────────────────┐
│ Native H.264 Video Stream Begins     │
└──────────────────┬───────────────────┘
                   ▼
┌──────────────────────────────────────┐
│ Floating Portrait Mirror Window      │
│ (60 FPS, Low Latency, Move/Resize)   │
└──────────────────┬───────────────────┘
                   ▼
┌──────────────────────────────────────┐
│ Clean Teardown on Screen-Off/Ctrl+C  │
└──────────────────────────────────────┘
```

---

## Requirements

### PC Requirements
- **Operating System**:
  - Windows 10 / 11 (64-bit or ARM64)
  - Linux (Ubuntu, Debian, Fedora, Arch, etc.)
  - macOS (Intel or Apple Silicon)
- **Android Platform Tools (ADB)**: Standard ADB CLI tool. (The installer configures this automatically if missing).
- **Video Renderer**: `mpv` (recommended for ultra-low-latency playback) or `ffplay`. (Portable `mpv` is set up automatically by the Windows installer).

### Android Requirements
- **Android Version**: Android 11+ (API 30 or higher) for QR-based Wireless Debugging.
  - *Android 5.0 - 10:* Can be cast directly over USB debugging or standard wireless TCP (`adb tcpip 5555`).
- **Network**: The computer and Android device must be connected to the same local Wi-Fi network (or computer mobile hotspot).
- **Developer Options**: Must be enabled with **Wireless Debugging** turned on.

---

## Installation

### Windows (PowerShell)

Run in PowerShell:

```powershell
iwr -useb https://raw.githubusercontent.com/<OWNER>/xcastphone/main/install.ps1 | iex
```

*Note: Replace `<OWNER>` with your GitHub organization or username.*

The Windows installer:
1. Detects your CPU architecture (x64 / ARM64).
2. Installs `xcast.exe` to `~/.xcast/bin`.
3. Sets up portable ADB and portable hardware-accelerated `mpv` if not already installed.
4. Adds `~/.xcast/bin` to your User `PATH` (no administrator privileges required).

### Linux & macOS

Run in terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/<OWNER>/xcastphone/main/install.sh | sh
```

The installer detects your OS and architecture, installs `xcast` to `~/.xcast/bin`, and adds it to your shell configuration (`~/.bashrc` / `~/.zshrc`).

---

## First-Time Setup on Android

Before your first wireless casting session, ensure Wireless Debugging is enabled on your phone:

1. Open **Settings** on your Android phone.
2. Tap **About Phone** and tap **Build Number** 7 times to enable Developer Options.
3. Go to **Settings > System > Developer options**.
4. Enable **Developer options** and toggle **Wireless debugging** to **ON**.
5. When prompted, check *"Always allow on this network"* and confirm.

---

## Usage

Simply run:

```bash
xcast
```

### 1. Terminal Pairing Screen
```text
XCastPhone

Waiting for Android device...

  ██████████████  ██    ██████████████
  ██          ██  ██    ██          ██
  ██  ██████  ██  ██    ██  ██████  ██
  ...
  ██████████████  ██    ██████████████

Scan the QR code using Android Wireless Debugging.
(Settings > Developer options > Wireless debugging > Pair device with QR code)

Waiting for connection...
```

### 2. Scan the QR Code
On your Android phone:
- Go to **Developer options > Wireless debugging**.
- Tap **"Pair device with QR code"**.
- Point your phone's camera at the terminal screen.

### 3. Live Casting
The mirror window immediately launches on your desktop:

```text
XCastPhone

Device:     Google Pixel 8
Resolution: 1080x2400
FPS:        60
Status:     Connected

Casting... (Press Ctrl+C to terminate)
```

---

## CLI Options

```bash
xcast [options]
```

| Flag | Default | Description |
|---|---|---|
| `-help` | `false` | Display help and usage instructions |
| `-version` | `false` | Display XCastPhone version |
| `-verbose` | `false` | Enable verbose debug logging |
| `-fps <n>` | `60` | Target frame rate (e.g. `30`, `60`) |
| `-bitrate <n>` | `8` | Target streaming bitrate in Mbps (e.g. `8`, `12`, `16`) |
| `-max-size <n>` | `2400` | Maximum screen dimension in pixels |
| `-timeout <n>` | `90` | Pairing discovery timeout in seconds |

Examples:

```bash
# Cast with ultra-high bitrate on fast Wi-Fi 6 networks
xcast -bitrate 16 -fps 60

# Limit resolution for older devices or congested Wi-Fi
xcast -max-size 1280 -bitrate 4
```

---

## Terminating a Session

To end a casting session cleanly:
- Press **Ctrl+C** in the terminal running `xcast`, OR
- Close the floating mirror window, OR
- **Turn off your phone's screen**: XCastPhone continuously monitors the device's display and power subsystems. As soon as the phone display goes to sleep or is locked, the stream stops and the mirror window closes automatically.

When terminated:
```text
Session terminated (phone screen turned off).
```
All temporary pairing keys, decoder processes, and background pipes are cleanly freed.

---

## Performance Considerations

- **Latency Targets**: When connected over 5 GHz Wi-Fi or USB, typical latency ranges between 35ms - 70ms. 2.4 GHz Wi-Fi may introduce jitter under interference.
- **Hardware Acceleration**: XCastPhone utilizes Android's hardware `MediaCodec` video encoder on device and desktop GPU decoding via `mpv`/libplacebo.
- **Adaptive Sizing**: For high-density screens (e.g. 1440x3120+), XCastPhone automatically calculates proportional, even-dimension resolutions that prevent encoder strain while preserving exact aspect ratios.

---

## Troubleshooting

### Phone says "Searching for pairing devices..." but doesn't connect
1. **Client Isolation**: Guest or corporate Wi-Fi networks often block multicast DNS (mDNS) traffic between wireless devices. Connect both PC and phone to a home network or enable a mobile hotspot on your phone and connect your PC to it.
2. **Same Wi-Fi Network**: Ensure both phone and PC are connected to the same subnet (e.g. both on the same router).
3. **Firewall**: Ensure your PC's firewall allows local ADB network traffic.

### Device already paired previously
If your device was previously paired or is connected via USB, running `xcast` bypasses the QR code and immediately launches the mirror window.

---

## Security & Privacy

- **Single-Use Temporary Credentials**: The QR code contains ephemeral pairing tokens that expire automatically after the session handshake.
- **Encrypted TLS Handshake**: Android Wireless Debugging uses TLS encryption with 2048-bit RSA keys to prevent eavesdropping on local networks.
- **Zero Cloud / Telemetry**: No screen data, identifiers, or logs ever leave your local machine.

---

## Repository Structure

```text
xcastphone/
├── cmd/
│   └── xcast/
│       └── main.go          # CLI entry point and argument parsing
├── internal/
│   ├── adb/
│   │   ├── client.go        # ADB command execution & discovery
│   │   ├── device.go        # Device enumeration and resolution parsing
│   │   └── power.go         # Screen state & display power monitoring
│   ├── pairing/
│   │   ├── pairing.go       # Session token generation & terminal QR rendering
│   │   └── pairing_test.go  # QR & token unit tests
│   ├── discovery/
│   │   ├── discovery.go     # mDNS pairing handshake & auto-connect
│   │   └── discovery_test.go# Service parser unit tests
│   ├── streaming/
│   │   ├── streamer.go      # Screen capture pipeline & continuous coordination
│   │   └── streamer_test.go # Dimension calculation unit tests
│   ├── decoder/
│   │   ├── decoder.go       # H.264 NAL parser & real-time metric tracking
│   │   └── decoder_test.go  # H.264 stream unit tests
│   ├── display/
│   │   └── window.go        # Native borderless floating portrait window
│   └── session/
│       └── coordinator.go   # End-to-end lifecycle orchestrator
├── scripts/
│   ├── install.sh           # Linux & macOS installer
│   └── install.ps1          # Windows PowerShell installer
├── install.sh               # Root installer shortcut
├── install.ps1              # Root installer shortcut
├── assets/                  # Architecture assets & icons
├── go.mod
├── go.sum
├── LICENSE                  # MIT License
└── README.md
```

---

## Development & Building

### Prerequisites
- Go 1.22+
- Platform Tools (ADB)

### Build Locally
```bash
# Clone the repository
git clone https://github.com/<OWNER>/xcastphone.git
cd xcastphone

# Run all unit tests
go test -v ./...

# Build the binary
go build -o xcast.exe ./cmd/xcast
```

---

## License

This project is licensed under the [MIT License](LICENSE).
