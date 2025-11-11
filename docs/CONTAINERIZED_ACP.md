# Containerized ACP Implementation

This document describes the implementation of running ACP (Agent Client Protocol) inside Docker containers, addressing issues #194 and #195.

## Overview

Previously, ACP processes ran on the host machine. Now, ACP runs inside agent Docker containers for better isolation and resource management.

## Architecture

```text
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
│ HostProcessLaunch│         │ ContainerAttachProcess  │
│                  │         │ Launcher                │
│ Uses os/exec     │         │ Attaches to container   │
│                  │         │ stdio                   │
└──────────────────┘         └─────────────────────────┘
                                       │
                                       v
                             ┌──────────────────────┐
                             │ Agent Container      │
                             │ ┌──────────────────┐ │
                             │ │ ACP Binary       │ │
                             │ │ (echo-agent)     │ │
                             │ │ Runs as main     │ │
                             │ │ process          │ │
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

### Issue #195: Launch ACP via Container Attach

**Implementation Notes:**
ACP runs as the container's main process with direct stdio attachment:

1. **ContainerAttachProcessLauncher** (`pkg/relay/session/container_attach_process_launcher.go`):
   - Implements `acp.ProcessLauncher` interface
   - Attaches to container's main process stdio using `dockerClient.ContainerAttach()`
   - Demultiplexes Docker's stream format (stdout/stderr) using `stdcopy.StdCopy()`
   - Logs stderr line-by-line with container ID prefix
   - Returns `containerAttachTransport` wrapping hijacked connection

2. **ACPClientFactory** (`pkg/relay/session/client_factory.go`):
   - `selectLauncher()` chooses between host and container modes
   - Based on `OUROCODUS_ACP_RUNTIME` environment variable
   - Validates container prerequisites (container ID, Docker client availability)
   - Creates `ContainerAttachProcessLauncher` for container mode

3. **AgentContainerHandle** (`pkg/agent/container/types.go`):
   - Provides `ContainerID()` method
   - session.Manager populates `AgentRuntimeContext.ContainerID` from handle

**Launcher Selection Logic:**
```go
mode := os.Getenv("OUROCODUS_ACP_RUNTIME")
switch mode {
case "host":    // Default - uses HostProcessLauncher
case "container": // Uses ContainerAttachProcessLauncher (requires ContainerID)
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

### Container Attach Tests
```bash
go test -v ./pkg/relay/session -run TestContainerAttach
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
- `Dockerfile.agent` - Multi-stage build to compile ACP binary, updated entrypoint/cmd
- `pkg/relay/session/container_attach_process_launcher.go` - **NEW** - Container attach launcher
- `pkg/relay/session/client_factory.go` - Updated to create ContainerAttachProcessLauncher for container mode
- `pkg/runtime/mode.go` - **NEW** - Shared runtime mode checking functions (moved from pkg/relay/session)
- `pkg/relay/session/manager.go` - Added logging and uses runtime mode helper functions
- `pkg/agent/container/launcher.go` - Uses runtime mode helper to check for container attach mode
- `pkg/containersession/manager.go` - Added skip output logging support for attach mode
- `pkg/containersession/session.go` - Added skipOutputLogging field
- `pkg/containersession/config.go` - Added SkipOutputLogging configuration option
- `docs/CONTAINERIZED_ACP.md` - This documentation

## Future Work

- Replace `echo-agent` with actual `claude-code-acp` binary
- Add metrics for container attach performance
- Implement automatic fallback to host mode on container failures
- Add container resource limits configuration

## Related Issues

- #194 - Ship ACP runtime inside agent Docker image (COMPLETE)
- #195 - Launch ACP process via container attach (COMPLETE)
- #193 - Run ACP client inside spawned container (parent tracking issue)
