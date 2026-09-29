#!/usr/bin/env sh
# ==============================================================================
# XCastPhone Linux / macOS Shell Installer
# ==============================================================================
# Installation:
#   curl -fsSL https://raw.githubusercontent.com/swastik-chavan/XCastPhone/main/install.sh | sh
# ==============================================================================

set -e

# ------------------------------------------------------------------------------
# Repository Configuration
# ------------------------------------------------------------------------------
REPO_OWNER="swastik-chavan"
REPO_NAME="XCastPhone"
VERSION="latest"

INSTALL_DIR="$HOME/.xcast/bin"
mkdir -p "$INSTALL_DIR"

echo "=========================================="
echo "        XCastPhone Shell Installer        "
echo "=========================================="
echo ""

# 1. Detect OS and Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

echo "[+] Platform: $OS-$ARCH"

TARGET_BIN="$INSTALL_DIR/xcast"

# 2. Install xcast binary
if [ -f "./xcast" ]; then
    echo "[+] Installing local binary build..."
    cp "./xcast" "$TARGET_BIN"
    chmod +x "$TARGET_BIN"
elif command -v go >/dev/null 2>&1 && [ -f "./cmd/xcast/main.go" ]; then
    echo "[+] Building xcast from local source..."
    go build -o "$TARGET_BIN" ./cmd/xcast
    chmod +x "$TARGET_BIN"
else
    DOWNLOAD_URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/$VERSION/download/xcast-$OS-$ARCH"
    echo "[+] Downloading XCastPhone from $DOWNLOAD_URL..."
    if curl -fsSL -o "$TARGET_BIN" "$DOWNLOAD_URL"; then
        chmod +x "$TARGET_BIN"
    else
        echo "Remote download failed. You can build locally from source via 'go build -o xcast ./cmd/xcast'." >&2
    fi
fi

# 3. Check for ADB
if ! command -v adb >/dev/null 2>&1; then
    echo "[!] Warning: 'adb' was not found on your system."
    echo "    Please install Android Platform Tools:"
    if [ "$OS" = "darwin" ]; then
        echo "      brew install --cask android-platform-tools"
    elif command -v apt-get >/dev/null 2>&1; then
        echo "      sudo apt-get install adb"
    elif command -v dnf >/dev/null 2>&1; then
        echo "      sudo dnf install android-tools"
    elif command -v pacman >/dev/null 2>&1; then
        echo "      sudo pacman -S android-tools"
    fi
else
    echo "[+] ADB: found."
fi

# 4. Check for Video Renderer (mpv)
if ! command -v mpv >/dev/null 2>&1 && ! [ -f "$INSTALL_DIR/mpv" ]; then
    echo "[!] Notice: Video renderer 'mpv' was not detected."
    echo "    Please install mpv for hardware-accelerated low-latency playback:"
    if [ "$OS" = "darwin" ]; then
        echo "      brew install mpv"
    elif command -v apt-get >/dev/null 2>&1; then
        echo "      sudo apt-get install mpv"
    elif command -v dnf >/dev/null 2>&1; then
        echo "      sudo dnf install mpv"
    elif command -v pacman >/dev/null 2>&1; then
        echo "      sudo pacman -S mpv"
    fi
else
    echo "[+] Video renderer: found."
fi

# 5. Add to PATH
SHELL_RC=""
if [ -n "$ZSH_VERSION" ] || [ -f "$HOME/.zshrc" ]; then
    SHELL_RC="$HOME/.zshrc"
elif [ -f "$HOME/.bashrc" ]; then
    SHELL_RC="$HOME/.bashrc"
else
    SHELL_RC="$HOME/.profile"
fi

if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    if [ -f "$SHELL_RC" ] && ! grep -q "$INSTALL_DIR" "$SHELL_RC"; then
        echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$SHELL_RC"
        echo "[+] Added $INSTALL_DIR to $SHELL_RC"
    fi
fi

echo ""
echo "=========================================="
echo "    XCastPhone installed successfully!    "
echo "=========================================="
echo ""
echo "Binary location: $TARGET_BIN"
echo ""
echo "To start casting, open a new shell and run:"
echo "  xcast"
echo ""
