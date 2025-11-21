#!/bin/bash
# Agent Adoption Demo - Phase 1/2 Showcase
# Demonstrates discovery, leases, and heartbeat monitoring

set -e

# Ensure DOCKER_HOST is set for Colima
export DOCKER_HOST=${DOCKER_HOST:-unix:///Users/clint/.colima/default/docker.sock}

# Colors for narration
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

narrate() {
    echo ""
    echo -e "${CYAN}===> $1${NC}"
    echo ""
    sleep 2
}

success() {
    echo -e "${GREEN}✓ $1${NC}"
}

info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

# Setup
clear
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "          Agent Adoption Demo - Phase 1/2"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Demonstrates:"
echo "    🔍 Agent discovery (Phase 1)"
echo "    🔐 Lease-based adoption (Phase 1)"
echo "    💓 Heartbeat monitoring (Phase 2)"
echo "    🧹 Automatic cleanup (Phase 2)"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
sleep 3

# Clean slate
narrate "Cleaning any previous demo state..."
bin/agentd stop demo-alice demo-bob 2>/dev/null || true
rm -rf ~/.agentd/session/*.lease 2>/dev/null || true
sleep 1

# Act 1: Start Infrastructure (1m)
narrate "Act 1: Start Infrastructure"

echo "Checking NATS server..."
if docker ps | grep -q ourocodus-nats; then
    success "NATS is running"
else
    info "Starting NATS server..."
    make nats-start > /dev/null 2>&1
    success "NATS server started"
fi
sleep 2

# Note about relay
echo ""
info "Note: Full attach/detach requires relay (PWA WebSocket server)"
info "This demo focuses on discovery and lease visibility from CLI"
sleep 2

# Act 2: Spawn Agents (1m)
narrate "Act 2: Spawn CLI Agents"

echo "$ agentd spawn demo-alice"
bin/agentd spawn demo-alice
sleep 2

echo ""
echo "$ agentd spawn demo-bob"
bin/agentd spawn demo-bob
sleep 2

# Act 3: Discover Agents (2m)
narrate "Act 3: Discover Agents with Adoption Status"

echo "$ agentd discover"
echo ""
bin/agentd discover
sleep 3

echo ""
success "Both agents discovered!"
info "Status shows 'discovered' - not yet attached to any session"
sleep 3

# Act 4: Show Lease Directory (1m)
narrate "Act 4: Lease System Ready for Adoption"

echo "Lease directory structure:"
echo "$ tree ~/.agentd/session/"
if command -v tree &> /dev/null; then
    tree ~/.agentd/session/ 2>/dev/null || ls -la ~/.agentd/session/ 2>/dev/null || echo "No leases yet (agents not attached)"
else
    ls -la ~/.agentd/session/ 2>/dev/null || echo "No leases yet (agents not attached)"
fi
sleep 3

echo ""
info "Leases will be created when PWA attaches to agents"
info "Each lease provides:"
echo "  - Exclusive attachment (one session at a time)"
echo "  - 5-minute TTL with automatic renewal"
echo "  - Atomic acquisition using O_EXCL"
sleep 3

# Act 5: Show Docker Labels (1m)
narrate "Act 5: Agent Labels for Discovery"

echo "Docker labels enable discovery:"
echo "$ docker inspect demo-alice | jq '.[0].Config.Labels'"
docker inspect demo-alice 2>/dev/null | jq '.[0].Config.Labels | with_entries(select(.key | startswith("ourocodus") or .key == "agent-id"))' || echo "Labels not found"
sleep 3

echo ""
success "spawn-source=cli label identifies CLI-spawned agents"
info "PWA can discover and adopt these agents"
sleep 2

# Act 6: Heartbeat Monitoring (1m)
narrate "Act 6: Heartbeat Monitoring (Phase 2)"

echo "Agents publish heartbeats to NATS every 30 seconds"
info "Subscribe to see heartbeats:"
echo ""
echo "  $ nats sub 'agent.heartbeat.*'"
echo ""
sleep 2

info "Heartbeats enable:"
echo "  - Liveness detection"
echo "  - Automatic lease renewal"
echo "  - Orphaned agent cleanup (if heartbeat stops)"
sleep 3

# Act 7: Discovery with JSON (30s)
narrate "Act 7: JSON Output for Automation"

echo "$ agentd discover --format json | jq"
echo ""
bin/agentd discover --format json | jq
sleep 3

echo ""
success "JSON output enables scripting and CI/CD integration"
sleep 2

# Act 8: Cleanup (1m)
narrate "Act 8: Graceful Cleanup"

echo "$ agentd stop demo-alice"
bin/agentd stop demo-alice
sleep 2

echo ""
echo "$ agentd stop demo-bob"
bin/agentd stop demo-bob
sleep 2

echo ""
echo "$ agentd discover"
bin/agentd discover
sleep 2

# Closing
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "${GREEN}                    Demo Complete! ✓${NC}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  What You've Seen:"
echo ""
echo "    🔍 Agent Discovery - CLI agents visible to PWA"
echo "    🔐 Lease System - Ready for atomic attachment"
echo "    💓 Heartbeat Monitoring - Automatic liveness tracking"
echo "    📊 JSON Output - Automation-friendly"
echo ""
echo "  Next Steps:"
echo ""
echo "    • Start relay: bin/relay"
echo "    • Open PWA and discover agents"
echo "    • Attach from PWA (acquires lease)"
echo "    • Watch: agentd discover --watch"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
