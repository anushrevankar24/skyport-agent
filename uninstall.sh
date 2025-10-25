#!/bin/bash
# SkyPort Agent Uninstaller for Linux and macOS

set -e

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "SkyPort Agent Uninstaller"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "💡 Tip: If skyport command is available, you can use:"
echo "   skyport uninstall"
echo ""
echo "This script is a fallback for manual uninstallation."
echo ""
read -p "Continue with manual uninstall? [y/N] " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Uninstall cancelled."
    exit 0
fi
echo ""

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="skyport"
SERVICE_NAME="skyport-agent"
CONFIG_DIR="$HOME/.skyport"

# Check if binary exists
if [ ! -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    echo "⚠️  SkyPort agent not found at $INSTALL_DIR/$BINARY_NAME"
    echo "   It may already be uninstalled or installed in a different location."
    echo ""
else
    echo "✓ Found SkyPort agent at $INSTALL_DIR/$BINARY_NAME"
fi

# Step 1: Check and stop/remove systemd service
echo ""
echo "Step 1: Checking for systemd service..."
if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    echo "   Found systemd service. Removing..."
    
    # Stop the service
    if sudo systemctl is-active --quiet $SERVICE_NAME; then
        echo "   Stopping service..."
        sudo systemctl stop $SERVICE_NAME
    fi
    
    # Disable the service
    if sudo systemctl is-enabled --quiet $SERVICE_NAME 2>/dev/null; then
        echo "   Disabling service..."
        sudo systemctl disable $SERVICE_NAME
    fi
    
    # Remove service file
    echo "   Removing service file..."
    sudo rm -f "/etc/systemd/system/$SERVICE_NAME.service"
    
    # Reload systemd
    echo "   Reloading systemd..."
    sudo systemctl daemon-reload
    
    echo "   ✓ Service removed successfully"
else
    echo "   ✓ No systemd service found"
fi

# Step 2: Remove binary
echo ""
echo "Step 2: Removing binary..."
if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    if [ -w "$INSTALL_DIR" ]; then
        rm -f "$INSTALL_DIR/$BINARY_NAME"
        echo "   ✓ Removed binary without sudo"
    else
        echo "   Administrator privileges required to remove binary"
        sudo rm -f "$INSTALL_DIR/$BINARY_NAME"
        echo "   ✓ Removed binary"
    fi
else
    echo "   ✓ Binary already removed"
fi

# Step 3: Remove configuration files
echo ""
echo "Step 3: Removing configuration files..."
if [ -d "$CONFIG_DIR" ]; then
    echo "   Found config directory at $CONFIG_DIR"
    echo "   This contains:"
    echo "     - User authentication data"
    echo "     - Tunnel configurations"
    echo ""
    read -p "   Do you want to remove it? [y/N] " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "$CONFIG_DIR"
        echo "   ✓ Configuration files removed"
    else
        echo "   ⊙ Configuration files kept"
    fi
else
    echo "   ✓ No configuration directory found"
fi

# Step 4: Clear keyring credentials
echo ""
echo "Step 4: Clearing keyring credentials..."
if command -v secret-tool &> /dev/null; then
    secret-tool clear service skyport-agent 2>/dev/null && echo "   ✓ Keyring credentials cleared" || echo "   ⊙ No keyring credentials found"
else
    echo "   ⊙ secret-tool not available, skipping keyring clear"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✓ SkyPort Agent uninstalled successfully!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Verify uninstallation
if command -v $BINARY_NAME &> /dev/null; then
    echo "⚠️  Warning: '$BINARY_NAME' command still available"
    echo "   It may be installed in another location."
    echo "   Run: which $BINARY_NAME"
else
    echo "✓ Verified: '$BINARY_NAME' command no longer available"
fi

echo ""
echo "Thank you for using SkyPort! 👋"
echo ""

