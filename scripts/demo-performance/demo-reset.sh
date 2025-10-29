#!/bin/bash
set -e

# Demo Reset Script
# Purpose: Clean up after demo and return to clean state

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🧹 Demo Reset"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Kill any running visualization servers
echo -e "${YELLOW}🔍 Checking for running processes...${NC}"

if pgrep -f "viz-server" > /dev/null 2>&1; then
    echo "   Found running visualization server"
    pkill -f "viz-server" || true
    echo -e "${GREEN}   ✅ Stopped visualization server${NC}"
else
    echo "   No running processes found"
fi

# Clean up any ports
echo ""
echo -e "${YELLOW}🔌 Checking port 9090...${NC}"
PID=$(lsof -ti:9090 2>/dev/null || true)
if [ -n "$PID" ]; then
    echo "   Found process using port 9090 (PID: $PID)"
    kill -9 "$PID" 2>/dev/null || true
    echo -e "${GREEN}   ✅ Port 9090 released${NC}"
else
    echo "   Port 9090 is available"
fi

# Clean up any generated files (optional - keep binaries for next run)
echo ""
echo -e "${YELLOW}📁 Checking for temporary files...${NC}"
if [ -f "${SCRIPT_DIR}/.demo-data.json" ]; then
    rm "${SCRIPT_DIR}/.demo-data.json"
    echo -e "${GREEN}   ✅ Removed temporary data${NC}"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}✅ Reset complete!${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📝 Demo is ready to run again:"
echo "   ./demo-run.sh"
echo ""
