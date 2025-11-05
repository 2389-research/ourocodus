#!/bin/bash
# demo-nats-basic.sh - Basic NATS Publish/Subscribe Demo
# Demonstrates basic NATS messaging patterns

set -e

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

DEMO_NAME="nats-basic"
BINARY_NAME="demo-${DEMO_NAME}"

echo "==========================================="
echo "  NATS Basic Messaging Demo"
echo "  Publish/Subscribe Pattern"
echo "==========================================="
echo

# Check if NATS server is running
echo -e "${BLUE}Checking NATS server...${NC}"
if ! nc -z localhost 4222 2>/dev/null; then
    echo -e "${RED}✗ NATS server not running on localhost:4222${NC}"
    echo
    echo "To start NATS server:"
    echo "  docker run -d -p 4222:4222 nats:latest"
    echo "  OR"
    echo "  nats-server"
    exit 1
fi
echo -e "${GREEN}✓ NATS server is running${NC}"

# Check Go
echo -e "${BLUE}Checking Go...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go not found${NC}"
    echo "Please install Go 1.23+: https://go.dev/dl/"
    exit 1
fi
echo -e "${GREEN}✓ Go $(go version | awk '{print $3}')${NC}"

# Build demo binary
echo -e "${BLUE}Building demo binary...${NC}"
if go build -o "${BINARY_NAME}" demo-nats-basic.go; then
    echo -e "${GREEN}✓ Built ${BINARY_NAME}${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

echo
echo "==========================================="
echo "  Running Demo"
echo "==========================================="
echo

# Run the demo
./"${BINARY_NAME}"

echo
echo "==========================================="
echo "  Demo Complete!"
echo "==========================================="
echo
echo "What you saw:"
echo "  • Simple publish to NATS subject"
echo "  • Subscribe to receive messages"
echo "  • Message delivery with correlation IDs"
echo
echo "To run again: ./demo-nats-basic.sh"
echo
