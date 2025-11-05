#!/bin/bash
# demo-setup.sh - One-time setup for Session Hierarchy demo
# Run this before your first demo to ensure everything is ready

set -e

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "  Session Hierarchy Demo - Setup"
echo "=========================================="
echo

# Check Docker
echo -e "${BLUE}Checking Docker...${NC}"
if ! command -v docker &> /dev/null; then
    echo -e "${RED}✗ Docker not found${NC}"
    echo "Please install Docker Desktop: https://www.docker.com/products/docker-desktop"
    exit 1
fi

if ! docker info &> /dev/null; then
    echo -e "${RED}✗ Docker daemon not running${NC}"
    echo "Please start Docker Desktop"
    exit 1
fi

echo -e "${GREEN}✓ Docker is ready${NC}"

# Check Go
echo -e "${BLUE}Checking Go installation...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go not found${NC}"
    echo "Please install Go 1.23+: https://go.dev/dl/"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "${GREEN}✓ Go ${GO_VERSION} installed${NC}"

# Pull ubuntu image
echo -e "${BLUE}Pulling ubuntu:latest image (if not present)...${NC}"
if docker pull ubuntu:latest > /dev/null 2>&1; then
    echo -e "${GREEN}✓ ubuntu:latest image ready${NC}"
else
    echo -e "${YELLOW}⚠ Failed to pull ubuntu:latest, will try during demo${NC}"
fi

# Build demo binary
echo -e "${BLUE}Building demo binary...${NC}"
if go build -o sessions-demo main.go; then
    echo -e "${GREEN}✓ Demo binary built${NC}"
    ls -lh sessions-demo
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# Cleanup any previous demo containers
echo -e "${BLUE}Cleaning up any previous demo containers...${NC}"
DEMO_CONTAINERS=$(docker ps -a --filter "label=com.ourocodus.containersession.managed-by" -q 2>/dev/null || true)
if [ -n "$DEMO_CONTAINERS" ]; then
    docker rm -f $DEMO_CONTAINERS > /dev/null 2>&1 || true
    echo -e "${GREEN}✓ Cleaned up old containers${NC}"
else
    echo -e "${GREEN}✓ No old containers to clean${NC}"
fi

# Cleanup workspace
if [ -d "./demo-workspaces" ]; then
    rm -rf ./demo-workspaces
    echo -e "${GREEN}✓ Cleaned up old workspaces${NC}"
fi

echo
echo "=========================================="
echo "           Setup Complete!"
echo "=========================================="
echo
echo -e "${GREEN}You're ready to run the demo!${NC}"
echo
echo "Next step:"
echo -e "  ${BLUE}./demo-run.sh${NC}"
echo
