#!/usr/bin/env bash
# End-to-end integration test for Phase 1 Agent Adoption
# Tests: spawn → discover → attach → detach workflow

set -e  # Exit on error
set -o pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
AGENT_ID="e2e-test-agent-$(date +%s)"
RELAY_URL="ws://localhost:8080/ws"
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$TEST_DIR/../.." && pwd)"

# Test state
RELAY_PID=""
CONTAINER_ID=""
SESSION_ID=""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"

    # Stop agent if running
    if [ -n "$AGENT_ID" ]; then
        echo "Stopping agent: $AGENT_ID"
        "$ROOT_DIR/agentd" stop "$AGENT_ID" 2>/dev/null || true
    fi

    # Kill relay if running
    if [ -n "$RELAY_PID" ]; then
        echo "Stopping relay (PID: $RELAY_PID)"
        kill "$RELAY_PID" 2>/dev/null || true
        wait "$RELAY_PID" 2>/dev/null || true
    fi

    # Remove any leftover containers
    docker ps -a --filter "label=agent-id=$AGENT_ID" -q | xargs -r docker rm -f 2>/dev/null || true

    echo -e "${GREEN}Cleanup complete${NC}"
}

trap cleanup EXIT

# Helper functions
log_step() {
    echo -e "\n${BLUE}==>${NC} ${1}"
}

log_success() {
    echo -e "${GREEN}✓${NC} ${1}"
}

log_error() {
    echo -e "${RED}✗${NC} ${1}"
}

# Check if relay is running
check_relay() {
    if ! curl -s "http://localhost:8080/health" > /dev/null 2>&1; then
        return 1
    fi
    return 0
}

# Build binaries
build_binaries() {
    log_step "Building binaries..."
    cd "$ROOT_DIR"
    make build
    log_success "Binaries built"
}

# Setup Docker socket for Colima
setup_docker_socket() {
    # Check if Colima is running
    if command -v colima &> /dev/null && colima status &> /dev/null; then
        # Colima uses a different socket location
        export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
        log_step "Detected Colima, using socket: $DOCKER_HOST"
    fi
}

# Start relay server
start_relay() {
    log_step "Starting relay server..."
    cd "$ROOT_DIR"
    ./relay --port 8080 > /tmp/relay-e2e.log 2>&1 &
    RELAY_PID=$!

    # Wait for relay to be ready
    for i in {1..30}; do
        if check_relay; then
            log_success "Relay server started (PID: $RELAY_PID)"
            return 0
        fi
        sleep 0.5
    done

    log_error "Relay server failed to start"
    cat /tmp/relay-e2e.log
    exit 1
}

# Test 1: Spawn agent
test_spawn_agent() {
    log_step "Test 1: Spawn agent with spawn-source label"

    # Check if agent already exists (cleanup from previous run)
    if docker ps -a --filter "label=agent-id=$AGENT_ID" -q | grep -q .; then
        log_error "Agent $AGENT_ID already exists. Cleaning up..."
        docker ps -a --filter "label=agent-id=$AGENT_ID" -q | xargs docker rm -f
    fi

    # Spawn agent
    if ! ANTHROPIC_API_KEY="test-key" "$ROOT_DIR/agentd" spawn "$AGENT_ID"; then
        log_error "Failed to spawn agent"
        return 1
    fi

    # Verify container exists
    CONTAINER_ID=$(docker ps --filter "label=agent-id=$AGENT_ID" --format "{{.ID}}")
    if [ -z "$CONTAINER_ID" ]; then
        log_error "Agent container not found"
        return 1
    fi

    # Verify spawn-source label
    SPAWN_SOURCE=$(docker inspect "$CONTAINER_ID" --format '{{index .Config.Labels "ourocodus.agent/spawn-source"}}')
    if [ "$SPAWN_SOURCE" != "cli" ]; then
        log_error "Expected spawn-source=cli, got: $SPAWN_SOURCE"
        return 1
    fi

    log_success "Agent spawned successfully (Container: $CONTAINER_ID, spawn-source: $SPAWN_SOURCE)"
    return 0
}

# Test 2: Discover agents
test_discover_agents() {
    log_step "Test 2: Discover agents via WebSocket"

    # Create a WebSocket client script
    cat > /tmp/ws-discover.js << 'EOF'
const WebSocket = require('ws');

const ws = new WebSocket(process.argv[2]);

ws.on('open', () => {
    // Send agent:discover message
    ws.send(JSON.stringify({
        type: 'agent:discover'
    }));
});

ws.on('message', (data) => {
    const msg = JSON.parse(data);
    if (msg.type === 'agent:discovered') {
        console.log(JSON.stringify(msg, null, 2));
        process.exit(0);
    } else if (msg.type === 'error') {
        console.error('Error:', msg.message);
        process.exit(1);
    }
});

ws.on('error', (err) => {
    console.error('WebSocket error:', err.message);
    process.exit(1);
});

setTimeout(() => {
    console.error('Timeout waiting for agent:discovered');
    process.exit(1);
}, 5000);
EOF

    # Check if node is available
    if ! command -v node &> /dev/null; then
        log_error "Node.js not found (required for WebSocket testing)"
        log_error "Skipping discover test (functionality tested in unit tests)"
        return 0  # Don't fail the entire test suite
    fi

    # Install ws package if not present
    if ! node -e "require('ws')" 2>/dev/null; then
        log_error "ws package not found (run: npm install -g ws)"
        log_error "Skipping discover test (functionality tested in unit tests)"
        return 0  # Don't fail the entire test suite
    fi

    # Run discovery
    DISCOVERY_OUTPUT=$(node /tmp/ws-discover.js "$RELAY_URL" 2>&1)

    # Verify our agent is in the list
    if ! echo "$DISCOVERY_OUTPUT" | grep -q "$AGENT_ID"; then
        log_error "Agent not found in discovery response"
        echo "Discovery output:"
        echo "$DISCOVERY_OUTPUT"
        return 1
    fi

    # Verify status is detached
    if ! echo "$DISCOVERY_OUTPUT" | grep -q '"status": "detached"'; then
        log_error "Expected agent status to be 'detached'"
        return 1
    fi

    # Verify spawn source
    if ! echo "$DISCOVERY_OUTPUT" | grep -q '"spawnSource": "cli"'; then
        log_error "Expected spawnSource to be 'cli'"
        return 1
    fi

    log_success "Agent discovered successfully (status: detached, spawnSource: cli)"
    return 0
}

# Test 3: Attach agent (conflict scenario)
test_attach_conflict() {
    log_step "Test 3: Test attach conflict (simultaneous attach to different sessions)"

    # This test requires WebSocket support
    if ! command -v node &> /dev/null || ! node -e "require('ws')" 2>/dev/null; then
        log_error "Skipping conflict test (requires Node.js + ws package)"
        return 0
    fi

    # Create test script for simultaneous attaches
    cat > /tmp/ws-attach-conflict.js << 'EOF'
const WebSocket = require('ws');

const agentId = process.argv[2];
const relayUrl = process.argv[3];

async function attachAgent(sessionId) {
    return new Promise((resolve, reject) => {
        const ws = new WebSocket(relayUrl);

        ws.on('open', () => {
            ws.send(JSON.stringify({
                type: 'agent:attach',
                agentId: agentId,
                userSessionId: sessionId
            }));
        });

        ws.on('message', (data) => {
            const msg = JSON.parse(data);
            if (msg.type === 'agent:attached') {
                resolve({ success: true, sessionId });
            } else if (msg.type === 'error' && msg.code === 'AGENT_ALREADY_ATTACHED') {
                resolve({ success: false, reason: 'conflict', sessionId });
            } else if (msg.type === 'error') {
                reject(new Error(msg.message));
            }
        });

        ws.on('error', reject);

        setTimeout(() => reject(new Error('Timeout')), 5000);
    });
}

(async () => {
    try {
        // Try to attach to two different sessions simultaneously
        const [result1, result2] = await Promise.all([
            attachAgent('session-1'),
            attachAgent('session-2')
        ]);

        console.log(JSON.stringify({ result1, result2 }));

        // Exactly one should succeed
        const successCount = [result1.success, result2.success].filter(Boolean).length;
        process.exit(successCount === 1 ? 0 : 1);
    } catch (err) {
        console.error('Error:', err.message);
        process.exit(1);
    }
})();
EOF

    # Run conflict test
    CONFLICT_OUTPUT=$(node /tmp/ws-attach-conflict.js "$AGENT_ID" "$RELAY_URL" 2>&1)
    CONFLICT_EXIT=$?

    if [ $CONFLICT_EXIT -eq 0 ]; then
        log_success "Attach conflict handled correctly (one succeeded, one failed)"
        echo "Result: $CONFLICT_OUTPUT"
    else
        log_error "Attach conflict test failed"
        echo "Output: $CONFLICT_OUTPUT"
        return 1
    fi

    return 0
}

# Test 4: Idempotent attach
test_idempotent_attach() {
    log_step "Test 4: Test idempotent attach (same session, multiple times)"

    # This test requires WebSocket support
    if ! command -v node &> /dev/null || ! node -e "require('ws')" 2>/dev/null; then
        log_error "Skipping idempotent attach test (requires Node.js + ws package)"
        return 0
    fi

    # Create test script
    cat > /tmp/ws-idempotent-attach.js << 'EOF'
const WebSocket = require('ws');

const agentId = process.argv[2];
const relayUrl = process.argv[3];
const sessionId = 'idempotent-test-session';

function attachAgent() {
    return new Promise((resolve, reject) => {
        const ws = new WebSocket(relayUrl);

        ws.on('open', () => {
            ws.send(JSON.stringify({
                type: 'agent:attach',
                agentId: agentId,
                userSessionId: sessionId
            }));
        });

        ws.on('message', (data) => {
            const msg = JSON.parse(data);
            if (msg.type === 'agent:attached') {
                resolve({ success: true });
            } else if (msg.type === 'error') {
                resolve({ success: false, error: msg });
            }
        });

        ws.on('error', reject);
        setTimeout(() => reject(new Error('Timeout')), 5000);
    });
}

(async () => {
    try {
        // Attach three times to same session
        const results = await Promise.all([
            attachAgent(),
            attachAgent(),
            attachAgent()
        ]);

        console.log(JSON.stringify(results));

        // All should succeed (idempotent)
        const allSucceeded = results.every(r => r.success);
        process.exit(allSucceeded ? 0 : 1);
    } catch (err) {
        console.error('Error:', err.message);
        process.exit(1);
    }
})();
EOF

    # Run idempotent test
    IDEMPOTENT_OUTPUT=$(node /tmp/ws-idempotent-attach.js "$AGENT_ID" "$RELAY_URL" 2>&1)
    IDEMPOTENT_EXIT=$?

    if [ $IDEMPOTENT_EXIT -eq 0 ]; then
        log_success "Idempotent attach works correctly (all succeeded)"
    else
        log_error "Idempotent attach test failed"
        echo "Output: $IDEMPOTENT_OUTPUT"
        return 1
    fi

    return 0
}

# Test 5: Idempotent detach
test_idempotent_detach() {
    log_step "Test 5: Test idempotent detach"

    # This test requires WebSocket support
    if ! command -v node &> /dev/null || ! node -e "require('ws')" 2>/dev/null; then
        log_error "Skipping idempotent detach test (requires Node.js + ws package)"
        return 0
    fi

    # Create test script
    cat > /tmp/ws-idempotent-detach.js << 'EOF'
const WebSocket = require('ws');

const agentId = process.argv[2];
const relayUrl = process.argv[3];
const sessionId = 'detach-test-session';

function detachAgent() {
    return new Promise((resolve, reject) => {
        const ws = new WebSocket(relayUrl);

        ws.on('open', () => {
            ws.send(JSON.stringify({
                type: 'agent:detach',
                agentId: agentId,
                userSessionId: sessionId
            }));
        });

        ws.on('message', (data) => {
            const msg = JSON.parse(data);
            if (msg.type === 'agent:detached') {
                resolve({ success: true });
            } else if (msg.type === 'error') {
                resolve({ success: false, error: msg });
            }
        });

        ws.on('error', reject);
        setTimeout(() => reject(new Error('Timeout')), 5000);
    });
}

(async () => {
    try {
        // Detach three times (should all succeed via idempotence)
        const results = await Promise.all([
            detachAgent(),
            detachAgent(),
            detachAgent()
        ]);

        console.log(JSON.stringify(results));

        // All should succeed (idempotent)
        const allSucceeded = results.every(r => r.success);
        process.exit(allSucceeded ? 0 : 1);
    } catch (err) {
        console.error('Error:', err.message);
        process.exit(1);
    }
})();
EOF

    # Run idempotent detach test
    DETACH_OUTPUT=$(node /tmp/ws-idempotent-detach.js "$AGENT_ID" "$RELAY_URL" 2>&1)
    DETACH_EXIT=$?

    if [ $DETACH_EXIT -eq 0 ]; then
        log_success "Idempotent detach works correctly (all succeeded)"
    else
        log_error "Idempotent detach test failed"
        echo "Output: $DETACH_OUTPUT"
        return 1
    fi

    return 0
}

# Main test execution
main() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  Phase 1 Agent Adoption E2E Integration Test      ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════╝${NC}"

    # Setup Docker socket (Colima support)
    setup_docker_socket

    # Build
    build_binaries

    # Start relay
    start_relay

    # Run tests
    FAILED=0

    test_spawn_agent || FAILED=$((FAILED + 1))
    test_discover_agents || FAILED=$((FAILED + 1))
    test_attach_conflict || FAILED=$((FAILED + 1))
    test_idempotent_attach || FAILED=$((FAILED + 1))
    test_idempotent_detach || FAILED=$((FAILED + 1))

    # Summary
    echo -e "\n${BLUE}════════════════════════════════════════════════════${NC}"
    if [ $FAILED -eq 0 ]; then
        echo -e "${GREEN}✓ All tests passed!${NC}"
        return 0
    else
        echo -e "${RED}✗ $FAILED test(s) failed${NC}"
        return 1
    fi
}

# Run main
main
exit $?
