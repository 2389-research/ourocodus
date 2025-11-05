# ContainerSession Package

Container session management using Docker SDK for isolated execution environments.

## Overview

The `containersession` package provides a high-level API for managing Docker container lifecycle, workspace directories, and I/O streams with built-in support for container reuse and session attachment. It enables isolated execution environments for agents by managing Docker containers with persistent workspaces.

## Features

- **Container Lifecycle Management**: Create, start, stop, and attach to Docker containers
- **Intelligent Container Reuse**: Automatically reconnect to existing containers based on session ID
- **Workspace Isolation**: Each session gets a dedicated workspace directory mounted at `/workspace`
- **Security Validation**: Path validation to prevent directory traversal attacks
- **Cross-Process Attachment**: Reconnect to containers from different processes
- **Thread-Safe Operations**: All operations are safe for concurrent use
- **Structured Error Handling**: Well-defined error types for common failure cases
- **Configurable Timeouts**: Customize graceful shutdown timeouts
- **Verbose Logging**: Optional debug-level logging for troubleshooting

## Quick Start

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/docker/docker/client"
)

func main() {
	ctx := context.Background()

	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer dockerClient.Close()

	// Create Manager with dependencies
	idGen := &containersession.UUIDGenerator{}
	clock := &containersession.SystemClock{}
	logger := log.New(os.Stdout, "[containersession] ", log.LstdFlags)

	manager := containersession.NewManager(
		dockerClient,
		idGen,
		clock,
		logger,
		"./workspaces", // Base directory for workspaces
	)

	// Create and start a session
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"/bin/bash"})
	if err != nil {
		log.Fatal(err)
	}

	err = manager.StartContainerSession(ctx, session.ID())
	if err != nil {
		log.Fatal(err)
	}

	// Use the workspace
	workspacePath := session.WorkspacePath()
	log.Printf("Session workspace: %s", workspacePath)

	// Stop the session
	err = manager.StopContainerSession(ctx, session.ID())
	if err != nil {
		log.Fatal(err)
	}
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Manager                              │
│                                                               │
│  - Session tracking (map[sessionID]*ContainerSession)        │
│  - Docker API coordination                                   │
│  - Workspace management                                      │
│  - Thread-safe operations (RWMutex)                          │
└────────────┬──────────────────────────────────┬─────────────┘
             │                                   │
             │ manages                           │ uses
             ▼                                   ▼
┌────────────────────────┐          ┌─────────────────────────┐
│   ContainerSession     │          │     Docker Client       │
│                        │          │                         │
│  State: PENDING        │◄─────────│  - ContainerCreate      │
│         RUNNING        │  updates │  - ContainerStart       │
│         STOPPED        │          │  - ContainerStop        │
│         FAILED         │          │  - ContainerAttach      │
│                        │          │  - ContainerInspect     │
│  - Session ID          │          │  - ContainerList        │
│  - Container ID        │          └─────────────────────────┘
│  - Workspace Path      │
│  - Labels              │
│  - Timestamps          │
└────────┬───────────────┘
         │
         │ mounted at /workspace
         ▼
┌────────────────────────┐
│   Workspace Directory  │
│                        │
│  $baseDir/$sessionID/  │
│  ├── input.txt         │
│  ├── output.txt        │
│  └── ...               │
│                        │
│  Permissions: 0700     │
│  (owner-only access)   │
└────────────────────────┘
```

### State Transitions

```
PENDING ──StartContainerSession──> RUNNING ──StopContainerSession──> STOPPED
   │                                   │
   └──────────────(error)──────────────┴─────────────────────────────> FAILED
```

## Core Concepts

### Session Lifecycle

1. **PENDING**: Container created but not started
2. **RUNNING**: Container is actively running
3. **STOPPED**: Container stopped gracefully
4. **FAILED**: Container or operation failed

### Container Reuse

`CreateContainerSession` automatically checks for existing containers with the same session ID before creating new ones. This enables resilient reconnection after process restarts or network failures.

**Reuse behavior by container state:**

- **Running**: Reattaches I/O streams without restarting
- **Stopped/Exited**: Starts the container then attaches
- **Created**: Starts the container then attaches
- **Dead/Removing**: Removes the bad container and returns error (retry to create new)
- **Paused**: Returns error (unpause not yet supported)

**Note**: `CreateContainerSession` generates a new session ID each time, so automatic reuse only occurs when the generated ID matches an existing container (rare). For intentional reuse across processes, use `AttachContainerSession` instead.

### Explicit Session Attachment

`AttachContainerSession` allows explicitly reconnecting to a running container by session ID. This is useful for:

- Reconnecting after process restart
- Attaching from a different process
- Monitoring or debugging existing sessions

**Requirements:**
- Container must exist (returns `ErrSessionNotFound` if not)
- Container must be in "running" state (returns error otherwise)
- Container must have `/workspace` mount (returns error if missing)

**Example:**

```go
// Process A creates a session
session, _ := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"/bin/bash"})
sessionID := session.ID()
_ = manager.StartContainerSession(ctx, sessionID)

// Store sessionID persistently (database, file, etc.)
saveSessionID(sessionID)

// Process B reconnects to the same session
sessionID := loadSessionID()
manager2, _ := NewManager(...) // New manager instance
reattached, _ := manager2.AttachContainerSession(ctx, sessionID)
// reattached is connected to the same running container
```

### Label Conventions

Containers are labeled with:
- `com.ourocodus.containersession.id`: Unique session ID
- `com.ourocodus.containersession.created`: RFC3339 timestamp
- `com.ourocodus.containersession.managed-by`: "ourocodus-containersession"

These labels enable discovery and management of containers across restarts.

### Workspace Security

Workspace paths are validated to prevent directory traversal attacks. All workspace directories are created with `0700` permissions (owner-only access). Path validation follows defense-in-depth principles:

1. Prefix check with separator to prevent directory name bypass
2. `filepath.Rel` to detect `..` traversal sequences
3. Absolute path validation

When reusing containers, workspace paths are extracted from existing volume mounts and validated to ensure they're under the configured base directory.

## Configuration

### Timeout Handling

Configure graceful shutdown timeout for containers:

```go
manager := containersession.NewManager(dockerClient, idGen, clock, logger, "./workspaces")
manager.SetStopTimeout(60) // 60 seconds (default: 30)
```

### Verbose Logging

Enable debug-level logging for troubleshooting:

```go
// Wrap logger with leveled logging
leveledLogger := containersession.NewLeveledLogger(logger, containersession.LogLevelDebug)
manager := containersession.NewManager(dockerClient, idGen, clock, leveledLogger, "./workspaces")
```

**Log Levels:**
- `LogLevelError`: Errors only
- `LogLevelInfo`: Errors + informational messages (default)
- `LogLevelDebug`: Errors + info + debug messages (verbose)

## API Reference

### Manager

#### `NewManager(dockerClient, idGen, clock, logger, baseWorkspaceDir) *Manager`

Creates a container session manager with injected dependencies.

**Parameters:**
- `dockerClient`: Docker SDK client (required, non-nil)
- `idGen`: ID generator for session IDs (required, non-nil)
- `clock`: Time source (required, non-nil)
- `logger`: Logger implementation (required, non-nil)
- `baseWorkspaceDir`: Base directory for workspaces (empty = "./workspaces")

**Panics** if any dependency is nil.

#### `CreateContainerSession(ctx, imageName, cmd) (*ContainerSession, error)`

Creates a new container session with workspace and Docker container. Automatically checks for existing containers with the same session ID.

**Returns:**
- `*ContainerSession`: Session in PENDING state
- `error`: Non-nil if creation fails

#### `StartContainerSession(ctx, sessionID) error`

Starts a container and attaches I/O streams. Session must be in PENDING state.

**Returns:**
- `error`: Non-nil if start fails or session not found

#### `StopContainerSession(ctx, sessionID) error`

Stops a running container gracefully with configured timeout. Idempotent - safe to call multiple times.

**Returns:**
- `error`: Non-nil if stop fails (ignores if already stopped)

#### `AttachContainerSession(ctx, sessionID) (*ContainerSession, error)`

Explicitly attaches to a running container by session ID. Container must be running.

**Returns:**
- `*ContainerSession`: Attached session
- `error`: Non-nil if container not found, not running, or attach fails

#### `GetContainerSession(sessionID) *ContainerSession`

Retrieves a session by ID. Returns `nil` if not found.

#### `ListContainerSessions() []*ContainerSession`

Returns all tracked sessions.

#### `SetStopTimeout(seconds int)`

Configures graceful shutdown timeout (default: 30 seconds).

### ContainerSession

#### `ID() string`

Returns the unique session ID.

#### `ContainerID() string`

Returns the Docker container ID.

#### `WorkspacePath() string`

Returns the absolute path to the workspace directory.

#### `State() State`

Returns the current session state (PENDING, RUNNING, STOPPED, FAILED).

#### `CreatedAt() time.Time`

Returns the session creation timestamp.

#### `StartedAt() time.Time`

Returns the session start timestamp (zero if not started).

#### `StoppedAt() time.Time`

Returns the session stop timestamp (zero if not stopped).

## Error Handling

The package uses structured errors for common failure cases:

- **`ErrSessionNotFound`**: Session ID not found in manager or Docker
- **`ErrSessionAlreadyExists`**: Session ID already exists in manager (race condition)
- **`ErrInvalidState`**: Operation not allowed in current session state
- **`ErrInvalidWorkspacePath`**: Workspace path validation failed

Docker API errors are wrapped with context using `fmt.Errorf` with `%w`.

**Example:**

```go
session, err := manager.AttachContainerSession(ctx, sessionID)
if err != nil {
	if errors.Is(err, containersession.ErrSessionNotFound) {
		log.Printf("Session %s not found, creating new one", sessionID)
	} else {
		log.Fatalf("Failed to attach: %v", err)
	}
}
```

## Thread Safety

All Manager methods are thread-safe and can be called concurrently. ContainerSession methods use internal locking to ensure safe concurrent access.

The in-memory session map uses `RWMutex` for efficient concurrent reads with safe writes during session creation and cleanup.

## Performance Considerations

### Label-based Discovery

- Uses Docker API filters for efficient container lookup
- Lists containers with `All=true` to include stopped containers
- Single API call per discovery operation

### Memory Usage

- In-memory session map grows with active sessions
- Stopped sessions remain in map (call `StopContainerSession` to cleanup)
- Each session holds references to workspace path and labels

### Concurrency

- Manager uses `RWMutex` for optimal read concurrency
- ContainerSession uses `Mutex` for state protection
- No blocking operations under locks (fast critical sections)

## Troubleshooting

### Docker Socket Permission Denied

**Symptom**: `permission denied while trying to connect to the Docker daemon socket`

**Solution**:
```bash
# Add user to docker group (Linux)
sudo usermod -aG docker $USER
newgrp docker

# Or use sudo (not recommended for production)
sudo chown $USER /var/run/docker.sock
```

For Colima on macOS, ensure the socket is accessible:
```bash
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
```

### Workspace Permission Issues

**Symptom**: Cannot read/write files in workspace

**Solution**:
- Workspaces are created with `0700` permissions (owner-only)
- Ensure process user matches workspace owner
- Check parent directory permissions

### Container Won't Stop

**Symptom**: `StopContainerSession` hangs or times out

**Solution**:
- Increase stop timeout: `manager.SetStopTimeout(60)`
- Check container logs: `docker logs <container-id>`
- Inspect container state: `docker inspect <container-id>`
- Force remove if stuck: `docker rm -f <container-id>`

### Container Reuse Confusion

**Symptom**: Unexpected container reuse behavior

**Solution**:
- `CreateContainerSession` auto-reuses by session ID (rare unless IDs collide)
- Use `AttachContainerSession` for intentional cross-process reuse
- List containers with labels: `docker ps -a --filter "label=com.ourocodus.containersession.managed-by"`

### Path Traversal Errors

**Symptom**: `ErrInvalidWorkspacePath` when creating sessions

**Solution**:
- Ensure `baseWorkspaceDir` doesn't contain `..` sequences
- Avoid system directories: `/etc`, `/sys`, `/proc`, `/root`, etc.
- Use relative paths like `./workspaces` or absolute paths under project directory

### Debug Logging

Enable verbose logging to troubleshoot issues:

```go
leveledLogger := containersession.NewLeveledLogger(
	log.New(os.Stdout, "[containersession] ", log.LstdFlags),
	containersession.LogLevelDebug,
)
manager := containersession.NewManager(dockerClient, idGen, clock, leveledLogger, "./workspaces")
```

This logs:
- Docker API calls
- State transitions
- I/O attachment
- Workspace operations

## Examples

See the [examples/containersession/](../../examples/containersession/) directory for complete working examples:

- **basic/**: Simple session lifecycle
- **echo-agent/**: Interactive agent with I/O
- **multi/**: Concurrent sessions with shared workspace

## Testing

Run unit tests:
```bash
go test ./pkg/containersession -v
```

Run integration tests (requires Docker):
```bash
go test ./pkg/containersession -tags=integration -v
```

## License

See the LICENSE file in the root of this repository.
