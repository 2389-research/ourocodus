# Phase 3 Demo: ACP Bridge for CLI Agent Adoption

This demo showcases the Phase 3 functionality that enables bidirectional communication with CLI-spawned agents via Docker container attachment.

## What This Demo Shows

The demo exercises all the new Phase 3 code:

1. **Docker Label-Based Discovery** - Uses `FindAgentContainerIDForTesting()` to discover CLI-spawned agent containers by their `ourocodus.agent/agent-id` label

2. **ACPBridge Creation** - Demonstrates creating an `ACPBridge` that attaches to a running container's stdin/stdout

3. **Bidirectional Communication** - Sends JSON-RPC messages to the agent and receives responses via the bridge

4. **Clean Resource Management** - Shows proper cleanup with context timeouts and graceful shutdown

## Architecture Demonstrated

```
┌─────────────────────────────────────────────────────────────┐
│                        Demo Program                          │
│  (cmd/demo-phase3/main.go)                                  │
└───────┬─────────────────────────────────────────────────────┘
        │
        │ 1. Spawns agent
        ├──────────────> agentd spawn demo-phase3-agent
        │                Creates Docker container with labels:
        │                • ourocodus.agent/agent-id=demo-phase3-agent
        │                • ourocodus.agent/workspace=/workspace
        │
        │ 2. Discovers container
        ├──────────────> FindAgentContainerIDForTesting()
        │                Queries Docker API for container with matching label
        │                Returns: containerID, workspace
        │
        │ 3. Creates bridge
        ├──────────────> NewACPBridge(ctx, containerID, agentID)
        │                • Attaches to container stdin/stdout
        │                • Sets up JSON-RPC message protocol
        │                • Spawns reader/writer goroutines
        │
        │ 4. Sends messages
        ├──────────────> bridge.SendMessage(ctx, "echo 'Hello'")
        │                • Serializes to JSON-RPC request
        │                • Writes to container stdin
        │                • Waits for response on stdout
        │                • Deserializes JSON-RPC response
        │                Returns: response payload
        │
        │ 5. Cleanup
        ├──────────────> bridge.Close(ctx)
        │                • Gracefully shuts down goroutines
        │                • Closes Docker connection
        │
        └──────────────> agentd terminate demo-phase3-agent
                         Stops and removes container
```

## Prerequisites

1. **Docker daemon running** - The demo uses Docker to manage agent containers
   ```bash
   docker info  # Should succeed
   ```

2. **agentd binary built** - Build the ourocodus binaries first
   ```bash
   make build
   ```

3. **ANTHROPIC_API_KEY** (optional) - Set for real agent interaction, or demo will use a dummy key
   ```bash
   export ANTHROPIC_API_KEY="your-key-here"
   # OR let demo use sk-test-dummy (agent won't work but infrastructure will)
   ```

## Running the Demo

### Option 1: Run directly with Go
```bash
go run cmd/demo-phase3/main.go
```

### Option 2: Build and run
```bash
go build -o bin/demo-phase3 ./cmd/demo-phase3
./bin/demo-phase3
```

## Expected Output

```
╔════════════════════════════════════════════════════════════╗
║  Phase 3 Demo: ACP Bridge for CLI Agent Adoption          ║
║  Demonstrates Docker-based agent discovery and ACP comms   ║
╚════════════════════════════════════════════════════════════╝

📋 Step 1: Checking prerequisites...
   ✓ Docker daemon accessible
   ✓ agentd binary found

🚀 Step 2: Spawning CLI agent (ID: demo-phase3-agent)...
   ✓ Agent spawned successfully

🔍 Step 3: Discovering agent container via Docker labels...
   ✓ Found container: 8f3a2b1c4d5e
   ✓ Workspace: /workspace

🌉 Step 4: Creating ACP Bridge...
   ✓ Bridge established

💬 Step 5: Testing ACP communication...

   📤 Sending: "Hello! Can you confirm you're receiving this message?"
   📥 Response type: text
   📝 Content: Yes, I'm receiving your message! I'm a Claude Code agent...

   📤 Sending: "What files are in your current workspace?"
   📥 Response type: text
   📝 Content: Let me check the workspace... [agent response with file listing]

   📤 Sending: "Write a simple hello world function in Go."
   📥 Response type: text
   📝 Content: Here's a simple hello world function... [agent response with code]

✅ All messages sent and received successfully!

🎉 Key achievements demonstrated:
   • Docker label-based agent discovery
   • ACPBridge creation and attachment
   • Bidirectional JSON-RPC communication
   • Clean shutdown and resource cleanup

🔌 Closing ACP Bridge...
   ✓ Bridge closed cleanly

🧹 Cleanup: Terminating demo agent...
   ✓ Agent terminated

✅ Demo completed successfully!
```

## Code Walkthrough

### Step 1: Prerequisites Check
```go
// Verifies Docker is accessible and agentd binary exists
func checkPrerequisites() error {
    cmd := exec.Command("docker", "info")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("Docker daemon not accessible: %w", err)
    }
    // ...
}
```

### Step 2: Spawn Agent
```go
// Uses agentd CLI to spawn a containerized agent
func spawnAgent(ctx context.Context, agentID string) error {
    cmd := exec.CommandContext(ctx, "./bin/agentd", "spawn", agentID)
    return cmd.Run()
}
```

### Step 3: Discover Container
```go
// Uses new Phase 3 discovery function
containerID, workspace, err := session.FindAgentContainerIDForTesting(ctx, demoAgentID)
// Queries Docker: containers with label ourocodus.agent/agent-id=demo-phase3-agent
```

### Step 4: Create Bridge
```go
// Uses new Phase 3 ACPBridge
bridge, err := session.NewACPBridge(ctx, containerID, demoAgentID)
// Attaches to container stdin/stdout
// Sets up JSON-RPC protocol
// Spawns reader/writer goroutines
```

### Step 5: Send Messages
```go
// Send message to the Claude Code agent through bridge
response, err := bridge.SendMessage(ctx, "Hello! Can you help me with some code?")
// Serializes to JSON-RPC: {"jsonrpc":"2.0","id":"req-1","method":"agent/sendMessage","params":{"content":"Hello! Can you help me with some code?"}}
// Writes to container stdin
// Agent processes the message and responds
// Reads JSON-RPC response from stdout
// Deserializes and returns agent's response
```

### Step 6: Cleanup
```go
// Graceful shutdown with context timeout
closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
bridge.Close(closeCtx)
// Signals goroutines to exit
// Waits for all goroutines with WaitGroup
// Closes Docker connection
```

## Troubleshooting

### "Docker daemon not accessible"
- Ensure Docker is running: `docker info`
- Check DOCKER_HOST environment variable if using Colima/remote Docker

### "bin/agentd not found"
- Build the project first: `make build`

### "failed to spawn agent"
- Check Docker has resources available
- Try: `docker ps` to see existing containers
- Try: `./bin/agentd list` to see existing agents

### "no running container found for agent ID"
- Agent may have failed to start
- Check: `docker ps -a` for exited containers
- Check: `docker logs <container-id>` for error messages

### "timeout waiting for response"
- Agent may not be responding (e.g., with dummy API key)
- Increase timeout in demo code
- Check agent logs: `docker logs <container-id>`

## What's Next?

This demo validates the Phase 3 infrastructure. Future phases will build on this:

- **Phase 4**: Security hardening (input validation, sandboxing)
- **Phase 5**: WebSocket integration (relay can communicate with adopted agents)
- **Phase 6**: Multi-agent orchestration

## Related Files

- `pkg/relay/session/acp_bridge.go` - ACPBridge implementation
- `pkg/relay/session/helpers.go` - FindAgentContainerIDForTesting
- `pkg/relay/session/acp_bridge_integration_test.go` - Integration tests
- `cmd/agentd/spawn.go` - Agent spawning logic
- `cmd/agentd/terminate.go` - Agent cleanup logic
