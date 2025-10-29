#!/bin/bash
# demo-reset.sh - Reset demo environment to clean state
# Run this between demo runs or if something goes wrong

set -e

echo "=== Resetting Demo Environment ==="
echo

# Kill any running relay servers
echo "Stopping any running relay servers..."
pkill -f "bin/relay" 2>/dev/null || true
pkill -f "./bin/relay" 2>/dev/null || true
sleep 1

# Check and report on port usage
if lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo "WARNING: Port 8080 is still in use by:"
    lsof -Pi :8080 -sTCP:LISTEN
    echo
    echo "Attempting to kill processes on port 8080..."
    lsof -ti:8080 | xargs kill -9 2>/dev/null || true
    sleep 1
fi

# Clean up temp files
echo "Cleaning temporary files..."
rm -f /tmp/relay.log

# Verify clean state
if lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo
    echo "ERROR: Failed to free port 8080"
    echo "You may need to manually kill the process:"
    lsof -Pi :8080 -sTCP:LISTEN
    exit 1
fi

echo "✓ All processes stopped"
echo "✓ Port 8080 is free"
echo "✓ Temporary files cleaned"
echo
echo "=== Reset Complete ==="
echo
echo "Environment is ready for another demo run"
echo "Next step: ./demo-run.sh"
echo
