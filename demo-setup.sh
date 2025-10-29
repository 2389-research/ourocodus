#!/bin/bash
# demo-setup.sh - One-time setup for WebSocket reconnection demo
# Run this BEFORE your meeting to ensure everything is ready

set -e

echo "=== Ourocodus WebSocket Demo Setup ==="
echo

# Check if we're in the right directory
if [ ! -f "cmd/relay/main.go" ]; then
    echo "ERROR: Please run this script from the ourocodus root directory"
    exit 1
fi

# Check for required tools
echo "Checking prerequisites..."

if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed or not in PATH"
    exit 1
fi

if ! command -v lsof &> /dev/null; then
    echo "ERROR: lsof is not installed (needed for demo control)"
    exit 1
fi

echo "✓ All prerequisites found"
echo

# Build the relay server
echo "Building relay server..."
if ! go build -o bin/relay ./cmd/relay; then
    echo "ERROR: Failed to build relay server"
    exit 1
fi
echo "✓ Relay server built successfully"
echo

# Check if web directory exists
if [ ! -d "web" ]; then
    echo "ERROR: web directory not found"
    exit 1
fi

if [ ! -f "web/index.html" ] || [ ! -f "web/app.js" ]; then
    echo "ERROR: PWA files missing from web directory"
    exit 1
fi
echo "✓ PWA files verified"
echo

# Clean up any running instances
echo "Cleaning up any running instances..."
pkill -f "bin/relay" 2>/dev/null || true
sleep 1
echo "✓ Cleanup complete"
echo

# Verify port 8080 is available
if lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "WARNING: Port 8080 is in use by another process"
    echo "The demo may fail to start. Kill the process using port 8080:"
    lsof -Pi :8080 -sTCP:LISTEN
    exit 1
fi
echo "✓ Port 8080 is available"
echo

echo "=== Setup Complete ==="
echo
echo "You're ready to run the demo!"
echo "Next step: ./demo-run.sh"
echo
