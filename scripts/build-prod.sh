#!/bin/bash

# SkyPort Agent Build Script
# Builds the agent binary with production URLs baked in

set -e

echo "Building SkyPort Agent..."
echo ""

# Load configuration from build-config.env
if [ -f "scripts/build-config-prod.env" ]; then
    echo "Loading build configuration from build-config-prod.env"
    source scripts/build-config-prod.env
else
    echo "Warning: build-config-prod.env not found, using defaults"
    SKYPORT_SERVER_URL="http://localhost:8080/api/v1"
    SKYPORT_WEB_URL="http://localhost:3000"
    SKYPORT_TUNNEL_DOMAIN="localhost:8080"
fi

echo "   Server URL: $SKYPORT_SERVER_URL"
echo "   Web URL:    $SKYPORT_WEB_URL"
echo "   Tunnel Domain: $SKYPORT_TUNNEL_DOMAIN"
echo "   Debug Mode: $DEBUG_MODE"
echo ""

# Build with ldflags to set the default URLs at compile time
echo "Compiling binary..."

go build \
    -ldflags="-X 'skyport-agent/internal/config.DefaultServerURL=$SKYPORT_SERVER_URL' \
              -X 'skyport-agent/internal/config.DefaultWebURL=$SKYPORT_WEB_URL' \
              -X 'skyport-agent/internal/config.DefaultTunnelDomain=$SKYPORT_TUNNEL_DOMAIN' \
              -X 'skyport-agent/internal/config.DebugMode=$DEBUG_MODE'" \
    -o skyport \
    cmd/skyport/main.go

if [ $? -eq 0 ]; then
    echo ""
    echo "Build successful!"
    echo "   Binary: ./skyport"
    echo ""
    echo "Binary is ready for distribution with:"
    echo "   Server: $SKYPORT_SERVER_URL"
    echo "   Web:    $SKYPORT_WEB_URL"
    echo "   Tunnel Domain: $SKYPORT_TUNNEL_DOMAIN"
    echo ""
    echo "Users can now run: ./skyport login"
    echo "   (No environment variables needed!)"
else
    echo ""
    echo "Build failed!"
    exit 1
fi

