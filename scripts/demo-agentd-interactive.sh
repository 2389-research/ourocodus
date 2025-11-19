#!/bin/bash
# agentd Interactive Demo - Step through at your own pace
# Press ENTER to advance to the next step

set -e

# Ensure DOCKER_HOST is set for Colima
export DOCKER_HOST=${DOCKER_HOST:-unix:///Users/clint/.colima/default/docker.sock}

# Colors for narration
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

narrate() {
    echo ""
    echo -e "${CYAN}===> $1${NC}"
    echo ""
}

wait_for_enter() {
    echo ""
    echo -e "${YELLOW}Press ENTER to continue...${NC}"
    read -r
}

run_command() {
    echo -e "${GREEN}\$ $1${NC}"
    eval "$1"
}

# Setup
clear
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "          agentd - Interactive Multi-Agent Demo"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Three-Layer Architecture:"
echo "    🌳 Git worktrees isolate code"
echo "    📦 Docker containers isolate processes"
echo "    🔑 Credentials isolate access"
echo ""
echo "  This demo lets you step through each command at your own pace."
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
wait_for_enter

# Clean slate
narrate "Cleaning any previous demo state..."
run_command "bin/agentd stop alice bob 2>/dev/null || true"
wait_for_enter

# Act 1: Environment Validation
narrate "Act 1: Validate Environment"
run_command "bin/agentd doctor"
wait_for_enter

# Act 2: Spawn Agents
narrate "Act 2: Spawn Isolated Agents"
echo ""
narrate "First, let's spawn alice..."
run_command "bin/agentd spawn alice"
wait_for_enter

narrate "Now spawn bob in a separate isolated environment..."
run_command "bin/agentd spawn bob"
wait_for_enter

# Act 3: Show Isolation
narrate "Act 3: Demonstrate Isolation"
echo ""
narrate "List running agents..."
run_command "bin/agentd list"
echo ""
echo -e "${CYAN}Note: WORKSPACE column shows the path INSIDE the container (/workspace)${NC}"
echo -e "${CYAN}The actual git worktree paths on the host are shown below:${NC}"
wait_for_enter

narrate "Each agent has its own isolated git worktree on the HOST:"
run_command "git worktree list | grep -E 'alice|bob' || git worktree list"
echo ""
echo -e "${CYAN}These host paths are mounted INTO the containers at /workspace${NC}"
wait_for_enter

narrate "Containers are running independently:"
run_command "docker ps --filter label=ourocodus.agent=true --format 'table {{.Names}}\t{{.Status}}'"
echo ""
echo -e "${CYAN}Each container has the host worktree mounted at /workspace (inside container)${NC}"
wait_for_enter

narrate "Let's peek inside alice's container to see the workspace:"
ALICE_CONTAINER=$(docker ps --filter "label=agent-id=alice" --format "{{.ID}}")
if [ -n "$ALICE_CONTAINER" ]; then
    run_command "docker exec $ALICE_CONTAINER ls -la /workspace | head -10"
    echo ""
    echo -e "${CYAN}This is the INSIDE view - the container sees /workspace${NC}"
    echo -e "${CYAN}But it's actually the host worktree mounted as a volume${NC}"
fi
wait_for_enter

# Act 4: Cleanup
narrate "Act 4: Graceful Cleanup"
echo ""
narrate "Stop alice while bob keeps running..."
run_command "bin/agentd stop alice"
wait_for_enter

narrate "Bob continues running after alice is stopped:"
run_command "bin/agentd list"
wait_for_enter

narrate "Now stop bob..."
run_command "bin/agentd stop bob"
wait_for_enter

narrate "Verify all agents are stopped:"
run_command "bin/agentd list"
wait_for_enter

# Closing
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${GREEN}                    Demo Complete! ✓${NC}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Key takeaway: Path mapping"
echo ""
echo "    HOST: .agentd/worktrees/agent-alice"
echo "           ↓ (mounted as Docker volume)"
echo "    CONTAINER: /workspace"
echo ""
echo "  The 'list' command shows container-internal paths (/workspace)"
echo "  The 'git worktree list' shows host paths (where code actually lives)"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
