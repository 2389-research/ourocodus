#!/bin/bash
# demo-container-reuse.sh - Container Reuse & Attach Demo
# Demonstrates Phase 2: Intelligent container discovery and cross-process attachment

set -e

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

DEMO_NAME="container-reuse"
BINARY_NAME="demo-${DEMO_NAME}"

echo "==========================================="
echo "  Container Reuse & Attach Demo"
echo "  Phase 2 Features"
echo "==========================================="
echo

# Check Docker/Colima
echo -e "${BLUE}Checking Docker/Colima...${NC}"

# Check if docker command exists
if ! command -v docker &> /dev/null; then
    echo -e "${RED}✗ Docker command not found${NC}"
    echo
    echo "Please install one of:"
    echo "  • Docker Desktop: https://www.docker.com/products/docker-desktop"
    echo "  • Colima: brew install colima"
    echo
    exit 1
fi

# Auto-detect Colima and set DOCKER_HOST if needed
USING_COLIMA=false
if docker context ls 2>/dev/null | grep -q 'colima \*'; then
    COLIMA_SOCKET=$(docker context inspect colima -f '{{.Endpoints.docker.Host}}' 2>/dev/null || echo "")
    if [ -n "$COLIMA_SOCKET" ]; then
        export DOCKER_HOST="$COLIMA_SOCKET"
        USING_COLIMA=true
        echo -e "${GREEN}✓ Detected Colima${NC}"
    fi
fi

# Test connection to Docker daemon
if ! docker info &> /dev/null; then
    echo -e "${RED}✗ Cannot connect to Docker daemon${NC}"
    echo
    if [ "$USING_COLIMA" = true ]; then
        echo "Colima detected but not running. Start it with:"
        echo "  colima start"
    else
        echo "Docker Desktop detected but not running. Start it with:"
        echo "  • Open Docker Desktop application"
        echo "  • Or use Colima: brew install colima && colima start"
    fi
    echo
    echo "Current DOCKER_HOST: ${DOCKER_HOST:-unix:///var/run/docker.sock}"
    echo
    exit 1
fi

if [ "$USING_COLIMA" = true ]; then
    echo -e "${GREEN}✓ Connected to Colima at $DOCKER_HOST${NC}"
else
    echo -e "${GREEN}✓ Connected to Docker Desktop${NC}"
fi

# Check Go
echo -e "${BLUE}Checking Go...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go not found${NC}"
    echo "Please install Go 1.23+: https://go.dev/dl/"
    exit 1
fi
echo -e "${GREEN}✓ Go $(go version | awk '{print $3}')${NC}"

# Pull ubuntu image
echo -e "${BLUE}Pulling ubuntu:latest...${NC}"
if docker pull ubuntu:latest > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Image ready${NC}"
else
    echo -e "${YELLOW}⚠ Failed to pull, will try during demo${NC}"
fi

# Build demo binary
echo -e "${BLUE}Building demo binary...${NC}"
if go build -o "${BINARY_NAME}" demo-container-reuse.go; then
    echo -e "${GREEN}✓ Built ${BINARY_NAME}${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# Cleanup any previous containers
echo -e "${BLUE}Cleaning up old containers...${NC}"
DEMO_CONTAINERS=$(docker ps -a --filter "label=com.ourocodus.containersession.managed-by" -q 2>/dev/null || true)
if [ -n "$DEMO_CONTAINERS" ]; then
    docker rm -f $DEMO_CONTAINERS > /dev/null 2>&1 || true
    echo -e "${GREEN}✓ Cleaned up${NC}"
else
    echo -e "${GREEN}✓ No cleanup needed${NC}"
fi

# Cleanup workspace
if [ -d "./demo-workspaces" ]; then
    rm -rf ./demo-workspaces
fi

echo
echo "==========================================="
echo "  Running Demo"
echo "==========================================="
echo

# Cleanup function
cleanup() {
    echo
    echo -e "${BLUE}Cleaning up...${NC}"
    DEMO_CONTAINERS=$(docker ps -a --filter "label=com.ourocodus.containersession.managed-by" -q 2>/dev/null || true)
    if [ -n "$DEMO_CONTAINERS" ]; then
        docker rm -f $DEMO_CONTAINERS > /dev/null 2>&1 || true
    fi
    if [ -d "./demo-workspaces" ]; then
        rm -rf ./demo-workspaces
    fi
    echo -e "${GREEN}✓ Cleanup complete${NC}"
}

trap cleanup EXIT

# Run the demo
./"${BINARY_NAME}"

echo
echo "==========================================="
echo "  Demo Complete!"
echo "==========================================="
echo
echo "To run again: ./demo-container-reuse.sh"
echo "To clean up:  ./demo-container-reuse-reset.sh"
echo
