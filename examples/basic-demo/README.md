# Basic Demo - Relay and Agent Interaction

## Purpose

This example demonstrates the fundamental interaction between the Ourocodus relay server and an agent. It showcases:

- **Session lifecycle**: Creating, using, and terminating user sessions
- **Agent spawning**: Automatic agent creation on demand
- **Message routing**: Client → Relay → Agent → Client communication flow
- **Error handling**: Clear error semantics with recoverability
- **WebSocket communication**: Real-time bidirectional messaging

This is the best starting point for understanding how Ourocodus works.

## Prerequisites

1. **Built binaries**: Run `make build` from the repository root
2. **Go installed**: Required to run the demo script
3. **Available port**: The relay server uses port 8080

## What This Demo Does

The script automatically:

1. Starts a relay server with an echo-agent configuration
2. Connects to the relay via WebSocket
3. Creates a user session
4. Spawns an agent for that session
5. Sends a test message through the relay to the agent
6. Receives the agent's response
7. Demonstrates error handling and recovery
8. Cleans up all resources

## Running the Demo

From the repository root:

```bash
# Build the binaries first
make build

# Run the demo
cd examples/basic-demo
go run main.go
```

Or using the Makefile shortcut (if available):

```bash
make demo
```

## Expected Output

You should see output similar to:

```
🎬 Ourocodus Demo - PR #27 Features Showcase
==

🚀 Starting relay server...
🔌 Connecting to relay...
✅ Connected! Server ID: <server-id>

📋 Demo Scenarios:
1️⃣  Session Lifecycle & Agent Communication
2️⃣  Clear Error Semantics with Recoverability

━━━ Scenario 1: Session Lifecycle & Agent Communication ━━━
Full session flow: create → spawn → message → response

→ Creating session...
✅ Session created: <session-id>

→ Spawning agent...
✅ Agent spawned: <agent-id>

→ Sending message to agent...
✅ Agent response: <message-content>

━━━ Scenario 2: Clear Error Semantics with Recoverability ━━━
Testing error handling and recovery...

→ Testing invalid session handling...
✅ Error correctly reported: session not found

→ Testing recovery after error...
✅ System recovered, new session works

🎉 Demo complete! All PR #27 features working as expected.
```

## Key Concepts Demonstrated

### 1. Session Lifecycle

- **CreateSession**: Initiates a new user session with the relay
- **State Management**: Relay tracks session state and associated agents
- **Cleanup**: Proper termination of sessions and agents

### 2. Agent Spawning

- **On-Demand Creation**: Agents are spawned automatically when needed
- **Configuration**: The relay is configured with an agent binary path
- **Process Management**: Relay manages agent lifecycle and communication

### 3. Message Routing

```
Client → WebSocket → Relay → Agent Process → Relay → WebSocket → Client
```

- Messages are JSON-formatted
- Each message has a type and payload
- Relay handles routing based on session and agent IDs

### 4. Error Handling

- Invalid sessions return clear error messages
- System remains operational after errors
- Graceful degradation and recovery

## Understanding the Code

### Main Flow

1. **Setup** (`main()`):
   - Locates binaries using `findRepoRoot()`
   - Starts relay server as subprocess
   - Configures environment variables

2. **Connection** (`dialRelay()`):
   - Establishes WebSocket connection
   - Receives server handshake
   - Validates connection

3. **Scenarios** (`demoSessionLifecycle()`, `demoErrorSemantics()`):
   - Demonstrates specific features
   - Shows expected behavior
   - Validates responses

### Important Functions

- `findRepoRoot()`: Locates the git repository root
- `dialRelay()`: Establishes WebSocket connection
- `createSession()`: Creates a new user session
- `spawnAgent()`: Requests agent creation
- `sendMessage()`: Sends message through relay

## Troubleshooting

### Error: "Relay binary not found"

**Cause**: Binaries not built

**Solution**:
```bash
make build
```

### Error: "Failed to connect: connection refused"

**Cause**: Relay server didn't start or port 8080 is in use

**Solution**:
- Check if another process is using port 8080: `lsof -i :8080`
- Wait a few seconds for relay to fully start
- Check relay logs for startup errors

### Error: "Failed to start relay"

**Cause**: Binary permissions or missing dependencies

**Solution**:
- Ensure binaries are executable: `chmod +x bin/relay bin/echo-agent`
- Check that all Go dependencies are available: `go mod tidy`

### Demo hangs or times out

**Cause**: Relay or agent not responding

**Solution**:
- Increase timeout values in the script
- Check system resources (memory, CPU)
- Review relay logs for bottlenecks

## Next Steps

After understanding this basic demo:

1. **Interactive REPL** (`examples/interactive-repl/`): Manually control the relay and send custom messages
2. **Performance Testing** (`examples/performance-testing/`): See how the system handles load
3. **Smoke Tests** (`examples/smoke-tests/`): Validate specific functionality areas

## Related Documentation

- [Architecture Overview](../../docs/architecture.md) - System design and components
- [Protocol Documentation](../../docs/protocol.md) - WebSocket message format
- [Developer Guide](../../CONTRIBUTING.md) - Contributing to Ourocodus

## Notes

- This demo uses a mock API key (`demo-key`) which is sufficient for local testing
- The echo-agent is a simple agent that echoes received messages
- The relay automatically cleans up when the demo exits (via defer statements)
- Each run creates new sessions and agents, nothing persists between runs
