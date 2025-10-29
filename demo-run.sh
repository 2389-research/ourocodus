#!/bin/bash
# demo-run.sh - Automated WebSocket reconnection demonstration
# This script runs the demo while you focus on explaining to your VP

set -e

# Color codes for better visibility
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to show pause points with clear instructions
pause_for_talk() {
    echo
    echo -e "${YELLOW}⏸  PAUSE: $1${NC}"
    echo -e "${BLUE}(Press ENTER when ready to continue)${NC}"
    read -r
    echo
}

# Function to display status
show_status() {
    echo -e "${GREEN}▶ $1${NC}"
}

# Function to display action
show_action() {
    echo -e "${BLUE}⚡ $1${NC}"
}

# Function to display problem
show_problem() {
    echo -e "${RED}✗ $1${NC}"
}

# Cleanup function
cleanup() {
    echo
    show_status "Cleaning up..."
    if [ -n "$SERVER_PID" ] && ps -p $SERVER_PID > /dev/null 2>&1; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
    echo "Done!"
}

trap cleanup EXIT

# Start of demo
clear
echo "=========================================="
echo "  Ourocodus WebSocket Reconnection Demo"
echo "=========================================="
echo
pause_for_talk "INTRO: Explain that you built automatic reconnection into the PWA"

# Phase 1: Start the server and show baseline
show_status "Phase 1: Starting the relay server..."
./bin/relay > /tmp/relay.log 2>&1 &
SERVER_PID=$!
echo "Server PID: $SERVER_PID"

# Wait for server to be ready
sleep 2

if ! ps -p $SERVER_PID > /dev/null; then
    echo -e "${RED}ERROR: Server failed to start${NC}"
    cat /tmp/relay.log
    exit 1
fi

show_status "Server running on http://localhost:8080"
echo
pause_for_talk "Open browser to http://localhost:8080 and click 'New Project'. Show the connection status (green)"

# Phase 2: Simulate network failure
show_action "Phase 2: Simulating network failure..."
echo
show_problem "Killing the server to simulate network outage..."
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null || true
SERVER_PID=""

show_problem "Server is DOWN - simulating network failure"
echo
pause_for_talk "Point to browser: Notice 'Disconnected' status and 'Reconnecting in X seconds' message"

# Phase 3: Recovery - restart server
show_action "Phase 3: Bringing server back online..."
echo
show_status "Restarting relay server (simulating network recovery)..."
./bin/relay > /tmp/relay.log 2>&1 &
SERVER_PID=$!

sleep 2

if ! ps -p $SERVER_PID > /dev/null; then
    echo -e "${RED}ERROR: Server failed to restart${NC}"
    cat /tmp/relay.log
    exit 1
fi

show_status "Server is BACK ONLINE"
echo
pause_for_talk "Watch browser: Connection automatically restored. Status turns green. No page reload needed!"

# Phase 4: Show resilience
show_action "Phase 4: Demonstrating resilience with multiple failures..."
echo
show_status "The app handles multiple connection drops gracefully"
echo

for i in {1..3}; do
    echo "--- Failure cycle $i/3 ---"

    # Kill server
    show_problem "Dropping connection..."
    kill $SERVER_PID
    wait $SERVER_PID 2>/dev/null || true
    sleep 3

    # Restart server
    show_status "Recovering connection..."
    ./bin/relay > /tmp/relay.log 2>&1 &
    SERVER_PID=$!
    sleep 2

    if ! ps -p $SERVER_PID > /dev/null; then
        echo -e "${RED}ERROR: Server failed to restart on cycle $i${NC}"
        exit 1
    fi

    echo "✓ Cycle $i complete"
    echo
done

pause_for_talk "Browser automatically reconnected after every failure. No user intervention needed!"

# Summary
echo
echo "=========================================="
echo "           Demo Complete!"
echo "=========================================="
echo
echo "Key Points Demonstrated:"
echo "  ✓ Automatic reconnection with exponential backoff"
echo "  ✓ Visual connection status for users"
echo "  ✓ No page reload or manual refresh needed"
echo "  ✓ Handles multiple failures gracefully"
echo "  ✓ Session state maintained across reconnections"
echo
echo "Server is still running at http://localhost:8080"
echo "Press ENTER to stop the server and exit"
read -r

# Cleanup handled by trap
