# Phase 3 Manual Communication Test

This test validates ACPBridge implementation by manually verifying each step of CLI agent communication.

## Prerequisites

```bash
export DOCKER_HOST="unix:///Users/clint/.colima/default/docker.sock"
export ANTHROPIC_API_KEY="sk-test-dummy-key-for-testing"
```

## Test Steps

### 1. Spawn Agent via CLI

```bash
bin/agentd spawn test-agent
```

Expected output:
- ✓ Agent test-agent ready
- Container ID displayed
- Worktree created

Verify:
```bash
bin/agentd list | grep test-agent
docker ps | grep test-agent
```

### 2. Check Agent Container Labels

```bash
docker inspect $(docker ps -q --filter "label=agent-id=test-agent") \
  --format '{{json .Config.Labels}}' | jq
```

Expected labels:
```json
{
  "ourocodus.agent": "true",
  "agent-id": "test-agent",
  "ourocodus.session-id": "session-...",
  ...
}
```

### 3. Test Direct ACP Communication (Container Exec)

This tests the ACPBridge's Docker exec attachment directly:

```bash
# Attach to agent container's stdin/stdout
CONTAINER_ID=$(docker ps -q --filter "label=agent-id=test-agent")

# Send ACP JSON-RPC message
echo '{"jsonrpc":"2.0","id":"test-1","method":"sendMessage","params":{"content":"echo Hello"}}' | \
  docker exec -i "$CONTAINER_ID" cat
```

Expected: JSON-RPC response from agent

### 4. Test via Relay (Integration Test)

Use PWA or write a small Go test client to:
1. Connect WebSocket to `ws://localhost:8080/ws`
2. Create session: `{"version":"1.0","type":"session:create"}`
3. Attach agent: `{"version":"1.0","type":"agent:attach","userSessionId":"<id>","agentId":"test-agent"}`
4. Send message: `{"version":"1.0","type":"agent:message","userSessionId":"<id>","agentId":"test-agent","content":"echo test"}`
5. Receive response: `{"version":"1.0","type":"agent:response",...}`
6. Detach: `{"version":"1.0","type":"agent:detach","userSessionId":"<id>","agentId":"test-agent"}`

### 5. Verify Agent Survives Detach

```bash
bin/agentd list | grep test-agent  # Should still show running
```

### 6. Cleanup

```bash
bin/agentd stop test-agent
```

## Automated Test Alternative

For automated testing, use Go test client instead of websocat:

```go
// test/integration/cli_agent_communication_test.go
func TestCLIAgentCommunication(t *testing.T) {
    // Connect WebSocket
    // Create session
    // Attach CLI agent
    // Send agent:message
    // Verify agent:response
    // Detach agent
    // Verify agent still running
}
```

## Expected Results

Phase 3 acceptance criteria:
- ✅ findAgentContainerID discovers agent via Docker labels
- ✅ NewACPBridge creates Docker exec attachment
- ✅ ACPBridge demultiplexes stdout/stderr
- ✅ ACPBridge handles NDJSON framing
- ✅ handleAgentMessage routes to CLI agents via ACPClient interface
- ✅ Responses return via agent:response
- ✅ Agent detaches cleanly (stays running)
