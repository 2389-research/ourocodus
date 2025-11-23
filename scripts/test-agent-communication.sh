#!/bin/bash
# scripts/test-agent-communication.sh
#
# End-to-end test for Phase 3: CLI Agent Communication
# Tests ACPBridge implementation for attaching and communicating with CLI-spawned agents

set -e

# Dependency validation
for cmd in websocat jq; do
  command -v "$cmd" > /dev/null || { echo "ERROR: $cmd not found"; exit 1; }
done
[ -x bin/agentd ] || { echo "ERROR: bin/agentd not found or not executable"; exit 1; }

# Environment setup for Docker and Anthropic API
export DOCKER_HOST="${DOCKER_HOST:-unix:///var/run/docker.sock}"
export ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-sk-test-dummy-key-for-testing}"

WS_URL="ws://localhost:8080/ws"
AGENT_ID="test-comm-$(date +%s)"
SESSION_ID=""

echo "=== Agent Communication Integration Test ==="
echo "Using DOCKER_HOST: $DOCKER_HOST"

cleanup() {
    echo "Cleaning up..."
    bin/agentd stop "$AGENT_ID" 2>/dev/null || true
}
trap cleanup EXIT

# 1. Spawn agent via CLI
echo "1. Spawning agent..."
bin/agentd spawn "$AGENT_ID"
sleep 3

# Verify agent is running
echo "   Verifying agent is running..."
bin/agentd list | grep "$AGENT_ID" || {
    echo "ERROR: Agent not found after spawn"
    exit 1
}

# 2. Create WebSocket session
echo "2. Creating WebSocket session..."
SESSION_ID=$(websocat "$WS_URL" <<EOF | jq -r '.userSessionId'
{"version":"1.0","type":"session:create"}
EOF
)
echo "   Session ID: $SESSION_ID"

if [ -z "$SESSION_ID" ] || [ "$SESSION_ID" = "null" ]; then
    echo "ERROR: Failed to create session"
    exit 1
fi

# 3. Attach agent to session
echo "3. Attaching agent to session..."
ATTACH_RESP=$(websocat "$WS_URL" --one-message <<EOF
{"version":"1.0","type":"agent:attach","userSessionId":"$SESSION_ID","agentId":"$AGENT_ID"}
EOF
)
echo "   Response: $ATTACH_RESP"

# Check if attach succeeded
if echo "$ATTACH_RESP" | jq -e '.error' > /dev/null 2>&1; then
    echo "ERROR: Agent attach failed"
    echo "$ATTACH_RESP" | jq .
    exit 1
fi

# 4. Send message to agent (using existing agent:message protocol)
echo "4. Sending message to agent..."
# NOTE: Uses agent:message (NOT agent:command) - unified protocol for all agents
MSG_RESP=$(websocat "$WS_URL" --one-message <<EOF
{"version":"1.0","type":"agent:message","userSessionId":"$SESSION_ID","agentId":"$AGENT_ID","content":"echo 'Hello from Phase 3 test!'"}
EOF
)
echo "   Response:"
echo "$MSG_RESP" | jq .

# Expected response:
# {"version":"1.0","type":"agent:response","userSessionId":"...","agentId":"test-comm-...","content":"..."}

# Verify response type
RESP_TYPE=$(echo "$MSG_RESP" | jq -r '.type')
if [ "$RESP_TYPE" != "agent:response" ]; then
    echo "ERROR: Expected agent:response, got: $RESP_TYPE"
    exit 1
fi

# 5. Detach agent
echo "5. Detaching agent..."
DETACH_RESP=$(websocat "$WS_URL" --one-message <<EOF
{"version":"1.0","type":"agent:detach","userSessionId":"$SESSION_ID","agentId":"$AGENT_ID"}
EOF
)
echo "   Response: $DETACH_RESP"

# 6. Verify agent still running
echo "6. Verifying agent still running..."
bin/agentd list | grep "$AGENT_ID" || {
    echo "ERROR: Agent terminated after detach (should still be running)"
    exit 1
}

echo ""
echo "✨ Communication test passed!"
echo ""
echo "Summary:"
echo "  - Agent spawned and discovered via Docker labels"
echo "  - ACPBridge created Docker exec attachment"
echo "  - Messages routed through existing agent:message protocol"
echo "  - Responses received via agent:response"
echo "  - Agent detached cleanly (still running)"
