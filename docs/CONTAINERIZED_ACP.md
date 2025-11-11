# Containerized ACP Implementation

This document describes the implementation of running ACP (Agent Client Protocol) inside Docker containers, addressing issues #194 and #195.

## Overview

Previously, ACP processes ran on the host machine. Now, ACP runs inside agent Docker containers for better isolation and resource management.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Relay Server                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │         session.Manager                            │  │
│  │  ┌────────────────────────────────────────────┐   │  │
│  │  │   ACPClientFactory                          │   │  │
│  │  │   - Selects launcher based on runtime       │   │  │
│  │  │   - OUROCODUS_ACP_RUNTIME env var           │   │  │
│  │  └────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                        │
        ┌───────────────┴───────────────┐
        │                               │
        v                               v
┌──────────────────┐         ┌─────────────────────────┐
│ Host Mode        │         │ Container Mode          │
│ (default)        │         │ (OUROCODUS_ACP_RUNTIME= │
│                  │         │  container)             │
│ HostProcessLaunch│         │ ContainerExecProcess    │
│                  │         │ Launcher                │
│ Uses os/exec     │         │ Uses docker exec        │
└──────────────────┘         └─────────────────────────┘
                                       │
                                       v
                             ┌──────────────────────┐
                             │ Agent Container      │
                             │ ┌──────────────────┐ │
                             │ │ ACP Binary       │ │
                             │ │ (echo-agent)     │ │
                             │ │                  │ │
                             │ │ Stdio: JSON-RPC  │ │
                             │ └──────────────────┘ │
                             │                      │
                             │ /workspace (mounted) │
                             │ /root/.ssh (creds)   │
                             └──────────────────────┘
```

## Implementation Details

### Issue #194: Ship ACP Runtime in Docker Image

**Changes:**
1. **Makefile** - Added `acp-binary` target:
   - Builds `cmd/echo-agent` as `/bin/acp` (cross-compiled for linux/amd64)
   - `agent-image` target now depends on `acp-binary`

2. **Dockerfile.agent** - Updated to include ACP binary:
   - Copies `bin/acp` to `/usr/local/bin/acp`
   - Sets `ENTRYPOINT ["/usr/local/bin/acp"]`
   - Sets `CMD ["--workspace", "/workspace"]`
   - Documents required environment variables

**Environment Variables:**
- `ANTHROPIC_API_KEY` - Required for ACP authentication
- `/workspace` - Container working directory (mounted from host)
- Stdio - ACP communicates via JSON-RPC over stdin/stdout

**Building:**
```bash
make agent-image
```

**Manual Testing:**
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"agent/sendMessage","params":{"content":"test"}}' | \
  docker run --rm -i ourocodus/agent:latest
```

### Issue #195: Launch ACP via Container Exec

**Implementation Notes:**
The implementation was already complete! The following components existed:

1. **ContainerExecProcessLauncher** (`pkg/relay/session/container_exec_process_launcher.go`):
   - Implements `acp.ProcessLauncher` interface
   - Uses `containersession.Manager.ExecInContainer()` for docker exec
   - Rewrites workspace paths from host to container paths
   - Returns `containerExecTransport` wrapping exec stdio streams

2. **ACPClientFactory** (`pkg/relay/session/client_factory.go`):
   - `selectLauncher()` chooses between host and container modes
   - Based on `OUROCODUS_ACP_RUNTIME` environment variable
   - Validates container prerequisites (container ID, manager availability)

3. **AgentContainerHandle** (`pkg/agent/container/types.go`):
   - Provides `ContainerID()` method
   - session.Manager populates `AgentRuntimeContext.ContainerID` from handle

**Launcher Selection Logic:**
```go
mode := os.Getenv("OUROCODUS_ACP_RUNTIME")
switch mode {
case "host":    // Default - uses HostProcessLauncher
case "container": // Uses ContainerExecProcessLauncher (requires ContainerID)
}
```

## Usage

### Host Mode (Default)
```bash
# No configuration needed - default behavior
./bin/relay
```

### Container Mode
```bash
# Enable container mode
export OUROCODUS_ACP_RUNTIME=container

# Run relay with containerized agents
./bin/relay
```

## Testing

### Unit Tests
```bash
make test
```

### Container Exec Tests
```bash
go test -v ./pkg/relay/session -run TestContainerExec
```

### Manual Integration Test
```bash
# Build image with ACP
make agent-image

# Test ACP binary in container
echo '{"jsonrpc":"2.0","id":1,"method":"agent/sendMessage","params":{"content":"Hello"}}' | \
  docker run --rm -i ourocodus/agent:latest

# Expected output:
# {"id":1,"result":{"type":"text","content":"Echo: Hello"},"jsonrpc":"2.0"}
```

## Files Modified

- `Makefile` - Added `acp-binary` target, updated `agent-image` dependency
- `Dockerfile.agent` - Added ACP binary, updated entrypoint/cmd
- `docs/CONTAINERIZED_ACP.md` - This documentation

## Files Already Implemented (No Changes)

- `pkg/relay/session/container_exec_process_launcher.go` - Container launcher
- `pkg/relay/session/container_exec_process_launcher_test.go` - Tests
- `pkg/relay/session/client_factory.go` - Launcher selection logic
- `pkg/containersession/exec.go` - Docker exec implementation
- `pkg/agent/container/types.go` - AgentContainerHandle

## Future Work

- Replace `echo-agent` with actual `claude-code-acp` binary
- Add metrics for container exec performance
- Implement automatic fallback to host mode on container failures
- Add container resource limits configuration

## Related Issues

- #194 - Ship ACP runtime inside agent Docker image (COMPLETE)
- #195 - Launch ACP process via container exec (COMPLETE)
- #193 - Run ACP client inside spawned container (parent tracking issue)
