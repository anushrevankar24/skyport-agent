#!/bin/bash
# SkyPort Agent Installer for Linux and macOS
# https://github.com/anushrevankar24/skyport-agent

set -e

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="skyport"
GITHUB_REPO="anushrevankar24/skyport-agent"
VERSION="latest"

echo "Installing SkyPort Agent..."
echo ""

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Normalize OS names
case $OS in
    darwin*) OS="darwin" ;;
    linux*) OS="linux" ;;
    *)
        echo "Error: Unsupported operating system: $OS"
        echo ""
        echo "Supported systems:"
        echo "  - Linux (amd64, arm64)"
        echo "  - macOS (amd64, arm64)"
        echo ""
        echo "For Windows, use the PowerShell installer:"
        echo "  irm https://raw.githubusercontent.com/$GITHUB_REPO/main/install.ps1 | iex"
        echo ""
        echo "Or visit https://github.com/$GITHUB_REPO/releases for manual installation"
        exit 1
        ;;
esac

# Normalize architecture names
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64) ARCH="arm64" ;;
    armv7l) ARCH="arm" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        echo ""
        echo "Supported architectures:"
        echo "  - amd64 (x86_64)"
        echo "  - arm64 (aarch64)"
        echo "  - arm (armv7l)"
        echo ""
        echo "Please visit https://github.com/$GITHUB_REPO/releases for manual installation"
        exit 1
        ;;
esac

echo "Detected system: $OS-$ARCH"

# Build download URL
DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/latest/download/skyport-$OS-$ARCH"

echo "Downloading from GitHub releases..."
echo ""

# Download the binary
if ! curl -fsSL "$DOWNLOAD_URL" -o "/tmp/$BINARY_NAME"; then
    echo "Error: Download failed!"
    echo ""
    echo "Please check:"
    echo "  1. Your internet connection"
    echo "  2. The release exists at: $DOWNLOAD_URL"
    echo "  3. Visit https://github.com/$GITHUB_REPO/releases for manual download"
    echo ""
    echo "If this is a new release, it may take a few minutes to become available."
    exit 1
fi

# Make it executable
chmod +x "/tmp/$BINARY_NAME"

# Install to system
echo "Installing to $INSTALL_DIR..."

# Create install directory if it doesn't exist
if [ ! -d "$INSTALL_DIR" ]; then
    echo ""
    echo "Creating $INSTALL_DIR directory..."
    if ! sudo mkdir -p "$INSTALL_DIR"; then
        echo ""
        echo "Error: Could not create $INSTALL_DIR"
        echo ""
        echo "Alternative: Install to user directory"
        echo "  mkdir -p ~/.local/bin"
        echo "  mv /tmp/$BINARY_NAME ~/.local/bin/$BINARY_NAME"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        exit 1
    fi
fi

if [ -w "$INSTALL_DIR" ]; then
    mv "/tmp/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    echo "Installed without sudo (directory is writable)"
else
    echo ""
    echo "Administrator privileges required for installation"
    if ! sudo mv "/tmp/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"; then
        echo ""
        echo "Error: Installation failed!"
        echo ""
        echo "Alternative: Install to user directory"
        echo "  mkdir -p ~/.local/bin"
        echo "  mv /tmp/$BINARY_NAME ~/.local/bin/$BINARY_NAME"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        exit 1
    fi
fi

echo ""
echo "SkyPort Agent installed successfully!"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Get started:"
echo "   skyport login"
echo ""
echo "View available commands:"
echo "   skyport --help"
echo ""
echo "Check installation:"
echo "   skyport --version"
echo ""
echo "To uninstall:"
echo "   skyport uninstall"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

