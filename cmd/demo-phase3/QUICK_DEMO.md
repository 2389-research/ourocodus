# Quick Phase 3 Demo (Manual Setup)

This is a simplified demo for systems where `agentd` doesn't automatically detect Docker (e.g., Colima on macOS).

## Quick Demo (2 minutes)

### Terminal 1: Spawn an Agent Manually

```bash
# Set Docker socket for Colima (macOS)
export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock

# Spawn agent with Docker directly
docker run -d \
  --name demo-phase3-agent \
  --label "ourocodus.agent/agent-id=demo-phase3-agent" \
  --label "ourocodus.agent/workspace=/workspace" \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-sk-test-dummy}" \
  ourocodus/agent:latest

# Verify it's running
docker ps | grep demo-phase3-agent
```

### Terminal 2: Run the Minimal Demo

Create `demo-minimal.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/session"
)

func main() {
	agentID := "demo-phase3-agent"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("🔍 Step 1: Discovering agent container...")
	containerID, workspace, err := session.FindAgentContainerIDForTesting(ctx, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   ✓ Found: %s (workspace: %s)\n\n", containerID[:12], workspace)

	fmt.Println("🌉 Step 2: Creating ACP Bridge...")
	bridge, err := session.NewACPBridge(ctx, containerID, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed: %v\n", err)
		os.Exit(1)
	}
	defer bridge.Close(context.Background())
	fmt.Println("   ✓ Bridge established\n")

	fmt.Println("💬 Step 3: Sending test message...")
	response, err := bridge.SendMessage(ctx, "Hello! Can you confirm you're receiving this message?")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed: %v\n", err)
		os.Exit(1)
	}

	if respMap, ok := response.(map[string]interface{}); ok {
		fmt.Printf("   ✓ Response: %v\n", respMap)
	}

	fmt.Println("\n✅ Phase 3 Demo Complete!")
}
```

Run it:

```bash
go run demo-minimal.go
```

### Cleanup

```bash
docker stop demo-phase3-agent
docker rm demo-phase3-agent
```

## What This Demonstrates

✅ **Docker Label-Based Discovery** - `FindAgentContainerIDForTesting()` finds the container by label
✅ **ACPBridge Creation** - Successfully attaches to running container
✅ **Bidirectional Communication** - Sends JSON-RPC message and receives response
✅ **Clean Shutdown** - Properly closes bridge and releases resources

## Full Demo (once agentd Docker issue is fixed)

Once agentd respects DOCKER_HOST properly, you can run the full automated demo:

```bash
./bin/demo-phase3
```

See [README.md](README.md) for full details.

## Troubleshooting

**"no running container found"**: Ensure the container is running with correct labels
```bash
docker ps --filter "label=ourocodus.agent/agent-id=demo-phase3-agent"
```

**"failed to create docker client"**: Set DOCKER_HOST
```bash
export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock
```

**"failed to attach to container"**: Container may have exited
```bash
docker logs demo-phase3-agent
```
