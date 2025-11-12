# Interactive REPL - Manual Testing and Exploration

## Purpose

This example provides an interactive Read-Eval-Print Loop (REPL) for manually testing and exploring the Ourocodus relay system. It's perfect for:

- **Interactive testing**: Send custom messages and observe behavior
- **Protocol exploration**: Understand the WebSocket message protocol
- **Debugging**: Test specific scenarios and edge cases
- **Learning**: Experiment with different commands and see immediate results

Unlike the automated basic demo, this gives you full control over every interaction.

## Prerequisites

1. **Built binaries**: Run `make build` from the repository root
2. **Go installed**: Required to run the REPL script
3. **Available port**: The relay server uses port 8080
4. **Basic understanding**: Familiarity with the basic demo is recommended

## What This REPL Provides

An interactive command-line interface with commands for:

- Session management (create, list, terminate)
- Agent management (spawn, list, status)
- Message sending and receiving
- Status inspection
- System exploration

All while keeping the relay server running and maintaining connection state.

## Running the REPL

From the repository root:

```bash
# Build the binaries first
make build

# Run the REPL
cd examples/interactive-repl
go run main.go
```

Or using the Makefile shortcut (if available):

```bash
make interactive
```

## Available Commands

### Session Commands

```
create-session
  Create a new user session
  Returns: Session ID

list-sessions
  List all active sessions
  Returns: Array of session IDs

terminate-session <session-id>
  Terminate a specific session
  Args: <session-id> - The session to terminate
```

### Agent Commands

```
spawn-agent <session-id>
  Spawn an agent for a session
  Args: <session-id> - The session ID
  Returns: Agent ID

list-agents
  List all spawned agents
  Returns: Array of agent IDs and their sessions

agent-status <agent-id>
  Get status of a specific agent
  Args: <agent-id> - The agent to check
  Returns: Agent status information
```

### Messaging Commands

```
send <session-id> <message>
  Send a message to an agent
  Args: <session-id> - Target session
       <message> - Message content (quoted if spaces)
  Returns: Agent's response

broadcast <message>
  Send message to all active sessions
  Args: <message> - Message content
  Returns: All agents' responses
```

### System Commands

```
status
  Display system status
  Returns: Server info, active sessions, agents

help
  Show this command list

quit / exit
  Exit the REPL and shutdown relay server
```

## Example Session

```
🎮 Ourocodus Interactive Demo - REPL
====================================

🚀 Starting relay server...
✅ Relay started
🔌 Connecting to relay...
✅ Connected! Server ID: srv_abc123

Type 'help' for available commands, 'quit' to exit

> help
Available commands:
  create-session              - Create a new user session
  list-sessions              - List all active sessions
  ...

> create-session
✅ Session created: session_xyz789

> spawn-agent session_xyz789
✅ Agent spawned: agent_def456

> send session_xyz789 "Hello, agent!"
📤 Sending to session_xyz789: Hello, agent!
📥 Response from agent_def456: Hello, agent!

> status
═══ System Status ═══
Server ID: srv_abc123
Active Sessions: 1
  - session_xyz789
Active Agents: 1
  - agent_def456 (session: session_xyz789)
Connection: ✅ Healthy

> quit
👋 Shutting down...
🛑 Stopping relay server...
✅ Cleanup complete
```

## Understanding the Code

### Main Components

1. **Server Management**:
   - Starts relay as subprocess
   - Configures echo-agent binary
   - Handles graceful shutdown

2. **Connection Handling**:
   - Maintains WebSocket connection
   - Receives handshake
   - Keeps connection alive

3. **REPL Loop**:
   - Reads user input
   - Parses commands
   - Executes actions
   - Displays results

4. **State Tracking**:
   - Tracks active sessions
   - Maps agents to sessions
   - Monitors connection health

### Key Structures

```go
type replState struct {
    conn          *websocket.Conn
    userSessionID string
    agents        map[string]bool  // Track spawned agents
}
```

## Use Cases

### 1. Protocol Exploration

Learn how messages flow through the system:

```
> create-session
> spawn-agent <session-id>
> send <session-id> "test message"
# Observe the request/response cycle
```

### 2. Edge Case Testing

Test boundary conditions:

```
> send invalid-session "test"
# See error handling

> create-session
# Don't spawn agent yet
> send <session-id> "test"
# See what happens without agent
```

### 3. Multi-Session Testing

Test concurrent sessions:

```
> create-session
# Note session-1
> create-session
# Note session-2
> spawn-agent <session-1>
> spawn-agent <session-2>
> send <session-1> "to first"
> send <session-2> "to second"
> list-sessions
> list-agents
```

### 4. Debugging Specific Scenarios

Reproduce reported issues:

```
# Create specific conditions
> create-session
> spawn-agent <session-id>
> send <session-id> "trigger-bug"
# Observe behavior
```

## Troubleshooting

### REPL won't start

**Cause**: Binaries not built or relay can't start

**Solution**:
```bash
make build
# Check if port 8080 is available
lsof -i :8080
```

### Commands not working

**Cause**: Wrong syntax or invalid state

**Solution**:
- Type `help` to see command syntax
- Use `status` to check current state
- Ensure you create sessions before spawning agents
- Ensure agents exist before sending messages

### Connection lost

**Cause**: Relay crashed or network issue

**Solution**:
- Check relay logs (stdout from REPL)
- Restart the REPL
- Check system resources

### Agent not responding

**Cause**: Agent crashed or message format wrong

**Solution**:
- Use `agent-status <agent-id>` to check agent
- Try `list-agents` to see all agents
- Check relay logs for agent errors

## Tips for Effective Use

1. **Use tab completion** (if your terminal supports it)
2. **Start simple**: Create session → Spawn agent → Send message
3. **Check status often**: Use `status` to understand current state
4. **Keep notes**: Track session/agent IDs for complex tests
5. **Experiment freely**: The REPL is safe for experimentation

## Next Steps

After mastering the REPL:

1. **Performance Testing** (`examples/performance-testing/`): See how the system handles load
2. **Smoke Tests** (`examples/smoke-tests/`): Validate specific functionality
3. **Source Code**: Read `main.go` to understand implementation details

## Related Documentation

- [Basic Demo](../basic-demo/README.md) - Start here if you haven't already
- [WebSocket Protocol](../../docs/architecture/WEBSOCKET_API.md) - WebSocket message formats
- [Architecture Overview](../../docs/architecture/ARCHITECTURE.md) - System design

## Notes

- The REPL maintains a single WebSocket connection throughout the session
- All state is tracked locally in the `replState` structure
- The relay server is automatically stopped when you exit the REPL
- Session and agent IDs are generated by the relay server
- Use Ctrl+C to force quit if needed (though `quit` is cleaner)
