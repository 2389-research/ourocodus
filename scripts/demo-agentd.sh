#!/bin/bash
# agentd Demo Script - 4-Minute Showcase
# Demonstrates three-layer isolation architecture

set -e

# Ensure DOCKER_HOST is set for Colima
export DOCKER_HOST=${DOCKER_HOST:-unix:///Users/clint/.colima/default/docker.sock}

# Colors for narration
CYAN='\033[0;36m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

narrate() {
    echo ""
    echo -e "${CYAN}===> $1${NC}"
    echo ""
    sleep 2
}

# Setup
clear
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "          agentd - Multi-Agent Isolation Demo"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Three-Layer Architecture:"
echo "    🌳 Git worktrees isolate code"
echo "    📦 Docker containers isolate processes"
echo "    🔑 Credentials isolate access"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
sleep 3

# Clean slate
narrate "Cleaning any previous demo state..."
bin/agentd stop alice bob 2>/dev/null || true
sleep 1

# Act 1: Environment Validation (30s)
narrate "Act 1: Validate Environment"
bin/agentd doctor
sleep 2

# Act 2: Spawn Agents (1m)
narrate "Act 2: Spawn Isolated Agents"

echo "$ agentd spawn alice"
bin/agentd spawn alice
sleep 2

echo ""
echo "$ agentd spawn bob"
bin/agentd spawn bob
sleep 2

# Act 3: Show Isolation (1m30s)
narrate "Act 3: Demonstrate Isolation"

echo "$ agentd list"
bin/agentd list
sleep 3

narrate "Each agent has its own isolated workspace:"
echo "$ git worktree list"
git worktree list | grep -E "alice|bob" || git worktree list
sleep 3

narrate "Containers are running independently:"
echo "$ docker ps --filter label=ourocodus.agent=true --format 'table {{.Names}}\t{{.Status}}'"
docker ps --filter "label=ourocodus.agent=true" --format "table {{.Names}}\t{{.Status}}"
sleep 3

# Act 4: Cleanup (1m)
narrate "Act 4: Graceful Cleanup"

echo "$ agentd stop alice"
bin/agentd stop alice
sleep 2

echo ""
narrate "Bob continues running while alice is cleaned up:"
echo "$ agentd list"
bin/agentd list
sleep 2

echo ""
echo "$ agentd stop bob"
bin/agentd stop bob
sleep 2

echo ""
echo "$ agentd list"
bin/agentd list
sleep 2

# Closing
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${GREEN}                    Demo Complete! ✓${NC}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  agentd demonstrates three-layer isolation:"
echo ""
echo "    🌳 Git worktrees → Code isolation"
echo "    📦 Docker containers → Process isolation"
echo "    🔑 Credential volumes → Access isolation"
echo ""
echo "  Multiple agents work concurrently without conflicts."
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
