#!/bin/bash
# demo-session-hierarchy-reset.sh - Cleanup for Session Hierarchy Demo

set -e

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "==========================================="
echo "  Session Hierarchy Demo - Cleanup"
echo "==========================================="
echo

echo -e "${YELLOW}Removing demo containers...${NC}"
DEMO_CONTAINERS=$(docker ps -a --filter "label=com.ourocodus.containersession.managed-by" -q 2>/dev/null || true)
if [ -n "$DEMO_CONTAINERS" ]; then
    docker rm -f $DEMO_CONTAINERS > /dev/null 2>&1 || true
    echo -e "${GREEN}✓ Removed containers${NC}"
else
    echo -e "${GREEN}✓ No containers to remove${NC}"
fi

echo -e "${YELLOW}Removing demo workspaces...${NC}"
if [ -d "./demo-workspaces" ]; then
    rm -rf ./demo-workspaces
    echo -e "${GREEN}✓ Removed workspaces${NC}"
else
    echo -e "${GREEN}✓ No workspaces to remove${NC}"
fi

echo -e "${YELLOW}Removing demo binary...${NC}"
if [ -f "./demo-session-hierarchy" ]; then
    rm -f ./demo-session-hierarchy
    echo -e "${GREEN}✓ Removed binary${NC}"
else
    echo -e "${GREEN}✓ No binary to remove${NC}"
fi

echo
echo "==========================================="
echo "  Cleanup Complete!"
echo "==========================================="
echo
