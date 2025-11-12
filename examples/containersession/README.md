# Container Session Examples

Examples demonstrating the `pkg/containersession` package for managing isolated Docker container execution environments.

## Available Examples

### [echo-agent/](echo-agent/) - JSON-RPC ACP Agent

The echo-agent example demonstrates running an ACP-compatible agent inside a container with JSON-RPC communication over stdin/stdout.

**This is the only valid pattern for the relay system** - agents must communicate via JSON-RPC over stdio to work with the relay.

**What it demonstrates:**
- Deploy script to workspace
- Start container with custom command
- JSON-RPC communication pattern (stdin/stdout)
- Agent processes messages and responds

**Run it:**
```bash
go run examples/containersession/echo-agent/main.go
```

## Why JSON-RPC Stdio?

The relay system uses the ACP (Agent Client Protocol) which requires:
- **JSON-RPC 2.0** over stdin/stdout
- **Method:** `agent/sendMessage`
- **Bidirectional streaming**

Any example that doesn't implement this pattern (simple file I/O, sleep containers, etc.) won't work with the relay system.

## Prerequisites

All examples require:

- **Docker**: Docker Desktop or Colima must be running
  - Test: `docker ps` should work
  - Colima users: `colima start`
  - Docker Desktop users: Ensure it's running

- **Go**: Version 1.24 or later
  - Test: `go version`

- **Ubuntu Image**: Will be pulled automatically if not present
  - Pre-pull: `docker pull ubuntu:latest`

## Package Documentation

For complete API documentation, see:
- [pkg/containersession/doc.go](../../pkg/containersession/doc.go) - Package overview
- [pkg/containersession/](../../pkg/containersession/) - Source code
- [pkg/containersession/integration_test.go](../../pkg/containersession/integration_test.go) - Integration tests

## Core Concepts

### Manager

The Manager coordinates container session lifecycle with dependency injection:

```go
manager := containersession.NewManager(
    dockerClient,    // Docker SDK client
    idGenerator,     // UUID generator
    clock,           // Time provider
    logger,          // Logger implementation
    baseWorkspaceDir // Base directory for workspaces
)
```

### Container Session

A session represents one isolated Docker container with its own workspace:

```go
// Create (PENDING state)
session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "60"})

// Start (RUNNING state)
err = manager.StartContainerSession(ctx, session.ID())

// Stop (STOPPED state)
err = manager.StopContainerSession(ctx, session.ID())
```

### Workspace Directories

Each session gets an isolated workspace directory:

```
<baseWorkspaceDir>/<session-id>/
```

- **Host side**: Files accessible at `session.WorkspacePath()`
- **Container side**: Mounted at `/workspace`
- **Persistence**: Survives container restarts
- **Security**: Path validated to prevent traversal attacks

### Session States

```
PENDING  → CreateContainerSession()
    ↓
RUNNING  → StartContainerSession()
    ↓
STOPPED  → StopContainerSession()
```

**FAILED** state occurs on errors.

## Testing

### Unit Tests

```bash
# Run package unit tests
go test ./pkg/containersession
```

### Integration Tests

Integration tests require Docker and use the `-tags=integration` flag:

```bash
# Run integration tests
go test -tags=integration ./pkg/containersession -v

# Run specific test
go test -tags=integration ./pkg/containersession -run TestIntegration_CreateStartStop -v
```

See [pkg/containersession/integration_test.go](../../pkg/containersession/integration_test.go) for available tests.

## Platform Support

### Supported Platforms

**macOS and Linux:**
- Docker Desktop
- Colima (macOS)
- Docker Engine (Linux)

**Windows:**
- Requires manual `DOCKER_HOST` configuration
- Docker Desktop with WSL2 backend (recommended)
- See Windows setup instructions below

### Connection Order

The helper function (`pkg/containersession/helpers.CreateDockerClient`) attempts connection in this order:
1. **DOCKER_HOST environment variable** - Honors standard Docker configuration
2. **macOS Colima** (macOS only) - Automatic fallback to `~/.colima/default/docker.sock`
3. **Standard Unix socket** - Falls back to `/var/run/docker.sock`

### Windows Setup

Configure `DOCKER_HOST` before running examples:

**Docker Desktop with Named Pipe:**
```powershell
# PowerShell
$env:DOCKER_HOST="npipe:////./pipe/docker_engine"

# Or Command Prompt
set DOCKER_HOST=npipe:////./pipe/docker_engine
```

**Docker Desktop with TCP (if exposed):**
```powershell
$env:DOCKER_HOST="tcp://localhost:2375"
```

**WSL2 (Recommended):**
Run examples inside WSL2 where Unix socket behavior matches macOS/Linux:
```bash
# Inside WSL2
go run examples/containersession/echo-agent/main.go
```

## Troubleshooting

### "cannot connect to Docker"

**Problem**: Docker client can't reach Docker daemon

**Solutions**:
- Ensure Docker Desktop is running or Colima is started
- Test: `docker ps` should work
- Colima: `colima start`
- Check DOCKER_HOST: `echo $DOCKER_HOST` (Unix) or `echo %DOCKER_HOST%` (Windows)
- Windows: Verify `DOCKER_HOST` is set correctly (see Platform Support above)

### "permission denied" on workspace

**Problem**: Can't write to workspace directory

**Solutions**:
- Workspace created with 0700 (owner-only) permissions
- Ensure you have write access to the base directory
- Check disk space: `df -h`

### Container won't stop

**Problem**: StopContainerSession hangs

**Solutions**:
- Increase timeout in context
- Check container logs: `docker logs <container-id>`
- Force remove: `docker rm -f <container-id>`

### Image pull errors

**Problem**: Can't pull Ubuntu image

**Solutions**:
- Check internet connection
- Pre-pull: `docker pull ubuntu:latest`
- Use alternative registry if needed

## Docker Cleanup

To clean up containers from examples:

```bash
# Stop all containersession containers
docker stop $(docker ps -q -f label=com.ourocodus.containersession.managed-by)

# Remove all containersession containers
docker rm $(docker ps -aq -f label=com.ourocodus.containersession.managed-by)

# Remove workspace directories
rm -rf workspaces/
```

## Next Steps

1. **Read the echo-agent example** to understand JSON-RPC ACP pattern
2. **Run the example** to see it in action
3. **Read the source code** in `pkg/containersession/`
4. **Write your own** ACP-compatible agent
5. **Contribute** improvements or new examples

## Related Documentation

- [CONTRIBUTING.md](../../CONTRIBUTING.md) - How to contribute
- [CLAUDE.md](../../CLAUDE.md) - Development tools guide
- [README.md](../../README.md) - Project overview
- [docs/history/2025-Q4-milestone1-2/CONTAINERIZED_ACP.md](../../docs/history/2025-Q4-milestone1-2/CONTAINERIZED_ACP.md) - Containerized ACP implementation (archived)

## Questions?

- Check [pkg/containersession/doc.go](../../pkg/containersession/doc.go) for detailed package docs
- Review [integration tests](../../pkg/containersession/integration_test.go) for usage patterns
- Open an issue on GitHub for bugs or questions
