#!/bin/bash
# demo-run.sh - Automated Container Session demonstration
# This script runs the demo while you focus on explaining to your team

set -e

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Pause function matching your existing demo pattern
pause_for_talk() {
    echo
    echo -e "${YELLOW}⏸  PAUSE: $1${NC}"
    echo -e "${BLUE}(Press ENTER when ready to continue)${NC}"
    read -r
    echo
}

# Cleanup function
cleanup() {
    echo
    echo -e "${GREEN}▶ Cleaning up demo containers...${NC}"
    DEMO_CONTAINERS=$(docker ps -a --filter "label=com.ourocodus.containersession.managed-by" -q 2>/dev/null || true)
    if [ -n "$DEMO_CONTAINERS" ]; then
        docker rm -f $DEMO_CONTAINERS > /dev/null 2>&1 || true
    fi
    if [ -d "./demo-workspaces" ]; then
        rm -rf ./demo-workspaces
    fi
    echo "Done!"
}

trap cleanup EXIT

# Start
clear
echo "=========================================="
echo "  Container Session Demo - Phase 2"
echo "  Reuse & Attach"
echo "=========================================="
echo
pause_for_talk "INTRO: Explain that Phase 2 adds intelligent container reuse and cross-process attachment"

# Run the interactive demo
# The Go program handles all scenarios with colored output and pause points
./containersession-demo

echo
echo "Demo complete!"
echo

# Cleanup handled by trap
