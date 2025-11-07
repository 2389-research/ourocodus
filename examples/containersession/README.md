# Container Session Examples

Comprehensive examples demonstrating the `pkg/containersession` package capabilities for managing isolated Docker container execution environments.

## Available Examples

### [basic/](basic/) - Single Session Lifecycle

The simplest container session example showing the complete lifecycle:
- Create Manager and container session
- Start the container
- Write/read data to/from workspace
- Stop and cleanup

**Best for**: Learning the basic API and understanding session states.

```bash
go run examples/containersession/basic/main.go
```

### [echo-agent/](echo-agent/) - Interactive Agent with I/O

Demonstrates running an interactive agent inside a container with bidirectional I/O:
- Deploy script to workspace
- Start container with custom command
- Bidirectional stdin/stdout communication pattern
- Agent processes messages and responds

**Best for**: Understanding how to run agents that need input/output interaction.

```bash
go run examples/containersession/echo-agent/main.go
```

### [multi/](multi/) - Concurrent Sessions with Coordination

Shows multiple container sessions running concurrently and coordinating via shared workspace:
- Producer session creates tasks
- Consumer session processes tasks
- Monitor session tracks progress
- File-based coordination pattern

**Best for**: Learning how multiple isolated containers can work together.

```bash
go run examples/containersession/multi/main.go
```

## Prerequisites

All examples require:

- **Docker**: Docker Desktop or Colima must be running
  - Test: `docker ps` should work
  - Colima users: `colima start`
  - Docker Desktop users: Ensure it's running

- **Go**: Version 1.23 or later
  - Test: `go version`

- **Ubuntu Image**: Will be pulled automatically if not present
  - Pre-pull: `docker pull ubuntu:latest`

## Quick Start

```bash
# From project root
cd examples/containersession

# Run basic example
go run basic/main.go

# Run echo-agent example
go run echo-agent/main.go

# Run multi-session example
go run multi/main.go
```

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

## Common Patterns

### 1. Basic Usage

```go
session, _ := manager.CreateContainerSession(ctx, "ubuntu:latest", cmd)
manager.StartContainerSession(ctx, session.ID())
// ... use session ...
manager.StopContainerSession(ctx, session.ID())
```

### 2. Deploying Scripts

```go
// Copy script to workspace
scriptPath := filepath.Join(session.WorkspacePath(), "script.sh")
os.WriteFile(scriptPath, scriptData, 0755)

// Run script in container
session, _ := manager.CreateContainerSession(ctx, "ubuntu:latest",
    []string{"/bin/bash", "/workspace/script.sh"})
```

### 3. Container Reuse

```go
// First process
session1, _ := manager.CreateContainerSession(ctx, "ubuntu:latest", cmd)
sessionID := session1.ID()

// Process restarts...

// Second process reuses container
session2, _ := manager.AttachContainerSession(ctx, sessionID)
// session2 connects to same running container
```

### 4. Concurrent Sessions

```go
var wg sync.WaitGroup

for i := 0; i < 3; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        session, _ := manager.CreateContainerSession(ctx, image, cmd)
        manager.StartContainerSession(ctx, session.ID())
        // ... do work ...
        manager.StopContainerSession(ctx, session.ID())
    }()
}

wg.Wait()
```

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

### Windows Setup

The Docker client setup code in examples assumes Unix-like systems with Unix socket paths. For Windows, you need to configure `DOCKER_HOST` before running examples:

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
go run examples/containersession/basic/main.go
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

1. **Read the examples** in order: basic → echo-agent → multi
2. **Run each example** to see them in action
3. **Read the source code** in `pkg/containersession/`
4. **Write your own** container session application
5. **Contribute** improvements or new examples

## Related Documentation

- [CONTRIBUTING.md](../../CONTRIBUTING.md) - How to contribute
- [CLAUDE.md](../../CLAUDE.md) - Development tools guide
- [README.md](../../README.md) - Project overview

## Questions?

- Check [pkg/containersession/doc.go](../../pkg/containersession/doc.go) for detailed package docs
- Review [integration tests](../../pkg/containersession/integration_test.go) for usage patterns
- Open an issue on GitHub for bugs or questions
