# Implementation Plan: Container Session Package (Issue #101)

## Overview

Implementation of `pkg/containersession` - a core package providing container session management using Docker SDK for isolated agent execution environments.

**Issue:** #101 - Phase 1: Core Container Session Package
**Milestone:** M1 - Container Session Foundation
**Branch:** `feature/container-session-core-101`

## Design Principles

Based on analysis of `pkg/relay/session/manager.go`, we follow these established patterns:

1. **Dependency Injection** - Constructor injection for all dependencies
2. **Interface-based Design** - All dependencies are interfaces for testability
3. **Panic on nil dependencies** - Constructor panics for required deps (programmer error)
4. **Error Handling** - Proper error wrapping with `fmt.Errorf` and `%w` verb
5. **Concurrency Safety** - Mutex locks for state management
6. **Structured Logging** - Contextual logging (session ID, container ID, etc.)
7. **Idempotency** - Operations handle already-stopped scenarios gracefully
8. **Path Validation** - Strict workspace path validation (defense-in-depth)
9. **TOCTOU Prevention** - Check-lock-check pattern for race condition prevention
10. **Event Publishing** - Optional publisher for lifecycle events (NATS integration)

## Package Architecture

```
pkg/containersession/
├── doc.go              # Package documentation
├── interfaces.go       # All abstractions (DockerClient, IDGenerator, etc.)
├── errors.go           # Package-level sentinel errors
├── manager.go          # Core Manager struct with Docker SDK
├── session.go          # ContainerSession state model
├── labels.go           # Label building/discovery helpers
├── workspace.go        # Workspace directory management
├── manager_test.go     # Manager tests with mocks
└── session_test.go     # Session model tests
```

## Implementation Phases

### Phase 1: Foundation Types (interfaces.go, errors.go)

**File:** `pkg/containersession/interfaces.go`

```go
package containersession

import (
    "context"
    "io"
    "time"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/filters"
)

// DockerClient abstracts Docker SDK operations for testability
type DockerClient interface {
    ContainerCreate(ctx context.Context, config *container.Config,
        hostConfig *container.HostConfig, networkingConfig *container.NetworkingConfig,
        containerName string) (container.CreateResponse, error)
    ContainerStart(ctx context.Context, containerID string,
        options container.StartOptions) error
    ContainerStop(ctx context.Context, containerID string,
        options container.StopOptions) error
    ContainerAttach(ctx context.Context, containerID string,
        options container.AttachOptions) (types.HijackedResponse, error)
    ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
    ContainerRemove(ctx context.Context, containerID string,
        options container.RemoveOptions) error
}

// IDGenerator abstracts unique ID generation
type IDGenerator interface {
    Generate() string
}

// Clock abstracts time operations for deterministic testing
type Clock interface {
    Now() time.Time
}

// Logger abstracts logging operations
type Logger interface {
    Printf(format string, v ...interface{})
}
```

**File:** `pkg/containersession/errors.go`

```go
package containersession

import "errors"

var (
    // ErrSessionNotFound indicates the requested session does not exist
    ErrSessionNotFound = errors.New("container session not found")

    // ErrSessionAlreadyExists indicates a session with the given ID already exists
    ErrSessionAlreadyExists = errors.New("container session already exists")

    // ErrInvalidSessionID indicates the session ID is empty or invalid
    ErrInvalidSessionID = errors.New("invalid session ID")

    // ErrInvalidWorkspacePath indicates the workspace path is invalid or unsafe
    ErrInvalidWorkspacePath = errors.New("invalid workspace path")

    // ErrContainerNotFound indicates the container does not exist
    ErrContainerNotFound = errors.New("container not found")

    // ErrContainerAlreadyRunning indicates the container is already running
    ErrContainerAlreadyRunning = errors.New("container already running")

    // ErrInvalidState indicates the operation cannot be performed in current state
    ErrInvalidState = errors.New("invalid state for operation")
)
```

### Phase 2: Session State Model (session.go)

**File:** `pkg/containersession/session.go`

```go
package containersession

import (
    "sync"
    "time"
)

// SessionState represents the lifecycle state of a container session
type SessionState string

const (
    StatePending   SessionState = "PENDING"   // Created but not started
    StateRunning   SessionState = "RUNNING"   // Container is running
    StateStopped   SessionState = "STOPPED"   // Container stopped gracefully
    StateFailed    SessionState = "FAILED"    // Container failed or error occurred
)

// ContainerSession represents a single container session with lifecycle management
type ContainerSession struct {
    id            string
    containerID   string
    workspacePath string
    labels        map[string]string
    state         SessionState
    createdAt     time.Time
    startedAt     *time.Time
    stoppedAt     *time.Time
    errorMsg      string

    // Synchronization
    mu sync.RWMutex
}

// NewContainerSession creates a new container session in PENDING state
func NewContainerSession(id, workspacePath string, labels map[string]string, createdAt time.Time) *ContainerSession {
    return &ContainerSession{
        id:            id,
        workspacePath: workspacePath,
        labels:        labels,
        state:         StatePending,
        createdAt:     createdAt,
    }
}

// ID returns the session ID (thread-safe, immutable)
func (s *ContainerSession) ID() string {
    return s.id
}

// ContainerID returns the Docker container ID
func (s *ContainerSession) ContainerID() string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.containerID
}

// SetContainerID sets the Docker container ID (internal use)
func (s *ContainerSession) SetContainerID(id string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.containerID = id
}

// State returns the current session state
func (s *ContainerSession) State() SessionState {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.state
}

// SetState transitions the session to a new state
func (s *ContainerSession) SetState(state SessionState) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.state = state
}

// WorkspacePath returns the workspace directory path
func (s *ContainerSession) WorkspacePath() string {
    return s.workspacePath
}

// Labels returns a copy of the session labels
func (s *ContainerSession) Labels() map[string]string {
    s.mu.RLock()
    defer s.mu.RUnlock()

    labelsCopy := make(map[string]string, len(s.labels))
    for k, v := range s.labels {
        labelsCopy[k] = v
    }
    return labelsCopy
}

// SetError records an error message and transitions to FAILED state
func (s *ContainerSession) SetError(err string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.errorMsg = err
    s.state = StateFailed
}

// Error returns the error message (if any)
func (s *ContainerSession) Error() string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.errorMsg
}

// MarkStarted records the start time and transitions to RUNNING
func (s *ContainerSession) MarkStarted(t time.Time) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.startedAt = &t
    s.state = StateRunning
}

// MarkStopped records the stop time and transitions to STOPPED
func (s *ContainerSession) MarkStopped(t time.Time) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.stoppedAt = &t
    s.state = StateStopped
}
```

### Phase 3: Label Helpers (labels.go)

**File:** `pkg/containersession/labels.go`

```go
package containersession

import (
    "fmt"
    "time"

    "github.com/docker/docker/api/types/filters"
)

const (
    // LabelPrefix is the namespace for all containersession labels
    LabelPrefix = "com.ourocodus.containersession"

    // LabelSessionID identifies the session ID
    LabelSessionID = "com.ourocodus.containersession.id"

    // LabelCreatedAt stores the creation timestamp
    LabelCreatedAt = "com.ourocodus.containersession.created"

    // LabelManagedBy identifies the manager
    LabelManagedBy = "com.ourocodus.containersession.managed-by"
)

// BuildLabels creates standard labels for a container session
func BuildLabels(sessionID string, timestamp time.Time) map[string]string {
    return map[string]string{
        LabelSessionID:  sessionID,
        LabelCreatedAt:  timestamp.Format(time.RFC3339),
        LabelManagedBy:  "ourocodus-containersession",
    }
}

// BuildLabelFilters creates Docker API filters for finding sessions
func BuildLabelFilters(sessionID string) filters.Args {
    f := filters.NewArgs()
    if sessionID != "" {
        f.Add("label", fmt.Sprintf("%s=%s", LabelSessionID, sessionID))
    } else {
        // Find all containers managed by us
        f.Add("label", fmt.Sprintf("%s=ourocodus-containersession", LabelManagedBy))
    }
    return f
}
```

### Phase 4: Workspace Management (workspace.go)

**File:** `pkg/containersession/workspace.go`

```go
package containersession

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// PrepareWorkspace creates and validates a workspace directory for a session
// Follows strict path validation pattern from pkg/relay/session/manager.go:154-174
func PrepareWorkspace(basePath, sessionID string) (string, error) {
    // Build workspace path
    workspacePath := filepath.Join(basePath, sessionID)
    cleanPath := filepath.Clean(workspacePath)

    // Get absolute paths for validation
    absPath, err := filepath.Abs(cleanPath)
    if err != nil {
        return "", fmt.Errorf("%w: cannot resolve absolute path: %v", ErrInvalidWorkspacePath, err)
    }

    baseAbs, err := filepath.Abs(basePath)
    if err != nil {
        return "", fmt.Errorf("%w: cannot resolve base path: %v", ErrInvalidWorkspacePath, err)
    }

    // Defense-in-depth: Check prefix with separator to prevent directory name bypass
    if absPath != baseAbs && !strings.HasPrefix(absPath, baseAbs+string(os.PathSeparator)) {
        return "", fmt.Errorf("%w: workspace path must be under base directory %s", ErrInvalidWorkspacePath, basePath)
    }

    // Use filepath.Rel to prevent directory traversal with ".."
    relPath, err := filepath.Rel(baseAbs, absPath)
    if err != nil || strings.HasPrefix(relPath, "..") || relPath == ".." || filepath.IsAbs(relPath) {
        return "", fmt.Errorf("%w: workspace path must be under base directory %s", ErrInvalidWorkspacePath, basePath)
    }

    // Create directory with strict permissions (owner-only access)
    err = os.MkdirAll(absPath, 0o700)
    if err != nil {
        return "", fmt.Errorf("failed to create workspace directory: %w", err)
    }

    return absPath, nil
}

// CleanupWorkspace removes a workspace directory
// Idempotent - does not fail if directory doesn't exist
func CleanupWorkspace(path string, logger Logger) error {
    err := os.RemoveAll(path)
    if err != nil && !os.IsNotExist(err) {
        logger.Printf("WARN: Failed to cleanup workspace %s: %v", path, err)
        return fmt.Errorf("failed to cleanup workspace: %w", err)
    }
    return nil
}
```

### Phase 5: Manager Implementation (manager.go)

**File:** `pkg/containersession/manager.go`

```go
package containersession

import (
    "context"
    "fmt"
    "io"
    "sync"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/pkg/stdcopy"
)

// Manager coordinates container session lifecycle with dependency injection
type Manager struct {
    dockerClient     DockerClient
    idGen            IDGenerator
    clock            Clock
    logger           Logger
    baseWorkspaceDir string

    // In-memory session tracking
    sessions map[string]*ContainerSession
    mu       sync.RWMutex
}

// NewManager creates a container session manager with injected dependencies
//
// All dependencies are required and must be non-nil. This constructor
// panics on nil collaborators because missing dependencies indicate programmer
// configuration bugs, not runtime failures.
//
// baseWorkspaceDir specifies the base directory under which all workspace paths
// must be constrained. If empty, defaults to "./workspaces".
func NewManager(dockerClient DockerClient, idGen IDGenerator, clock Clock, logger Logger, baseWorkspaceDir string) *Manager {
    if dockerClient == nil {
        panic("dockerClient cannot be nil")
    }
    if idGen == nil {
        panic("idGen cannot be nil")
    }
    if clock == nil {
        panic("clock cannot be nil")
    }
    if logger == nil {
        panic("logger cannot be nil")
    }

    if baseWorkspaceDir == "" {
        baseWorkspaceDir = "./workspaces"
    }

    return &Manager{
        dockerClient:     dockerClient,
        idGen:            idGen,
        clock:            clock,
        logger:           logger,
        baseWorkspaceDir: baseWorkspaceDir,
        sessions:         make(map[string]*ContainerSession),
    }
}

// CreateSession creates a new container session with workspace and Docker container
func (m *Manager) CreateSession(ctx context.Context, imageName string, cmd []string) (*ContainerSession, error) {
    // Generate unique session ID
    sessionID := m.idGen.Generate()
    now := m.clock.Now()

    // Build labels
    labels := BuildLabels(sessionID, now)

    // Prepare workspace directory
    workspacePath, err := PrepareWorkspace(m.baseWorkspaceDir, sessionID)
    if err != nil {
        return nil, fmt.Errorf("failed to prepare workspace: %w", err)
    }

    // Create session in PENDING state
    session := NewContainerSession(sessionID, workspacePath, labels, now)

    // Store session (with TOCTOU prevention)
    m.mu.Lock()
    if _, exists := m.sessions[sessionID]; exists {
        m.mu.Unlock()
        return nil, fmt.Errorf("%w: %s", ErrSessionAlreadyExists, sessionID)
    }
    m.sessions[sessionID] = session
    m.mu.Unlock()

    // Create Docker container
    containerConfig := &container.Config{
        Image:  imageName,
        Cmd:    cmd,
        Labels: labels,
    }

    hostConfig := &container.HostConfig{
        Binds: []string{
            fmt.Sprintf("%s:/workspace", workspacePath),
        },
    }

    resp, err := m.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, "")
    if err != nil {
        // Remove session from map on failure
        m.mu.Lock()
        delete(m.sessions, sessionID)
        m.mu.Unlock()

        session.SetError(err.Error())
        m.logger.Printf("Container creation failed: session=%s error=%v", sessionID, err)
        return nil, fmt.Errorf("failed to create container: %w", err)
    }

    session.SetContainerID(resp.ID)
    m.logger.Printf("Container session created: id=%s container=%s state=PENDING", sessionID, resp.ID)

    return session, nil
}

// StartSession starts a container and attaches I/O streams
func (m *Manager) StartSession(ctx context.Context, sessionID string) error {
    session := m.GetSession(sessionID)
    if session == nil {
        return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
    }

    // Validate state
    if session.State() != StatePending {
        return fmt.Errorf("%w: cannot start session in state %s", ErrInvalidState, session.State())
    }

    containerID := session.ContainerID()
    if containerID == "" {
        return fmt.Errorf("session has no container ID")
    }

    // Start container
    err := m.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
    if err != nil {
        session.SetError(err.Error())
        m.logger.Printf("Container start failed: session=%s container=%s error=%v", sessionID, containerID, err)
        return fmt.Errorf("failed to start container: %w", err)
    }

    // Attach to container I/O
    attachResp, err := m.dockerClient.ContainerAttach(ctx, containerID, container.AttachOptions{
        Stream: true,
        Stdin:  false,
        Stdout: true,
        Stderr: true,
        Logs:   true,
    })
    if err != nil {
        m.logger.Printf("Container attach failed: session=%s container=%s error=%v", sessionID, containerID, err)
        // Continue even if attach fails - container is still running
    } else {
        // Start goroutines to demux stdout/stderr
        go m.handleContainerOutput(sessionID, containerID, attachResp.Reader)
    }

    session.MarkStarted(m.clock.Now())
    m.logger.Printf("Container session started: id=%s container=%s state=RUNNING", sessionID, containerID)

    return nil
}

// handleContainerOutput demultiplexes Docker container output streams
func (m *Manager) handleContainerOutput(sessionID, containerID string, reader io.Reader) {
    defer func() {
        if r := recover(); r != nil {
            m.logger.Printf("Panic in output handler: session=%s container=%s panic=%v", sessionID, containerID, r)
        }
    }()

    // Use stdcopy to demux stdout/stderr
    // For MVP, we log output - future versions can stream to clients
    _, err := stdcopy.StdCopy(
        &logWriter{logger: m.logger, prefix: fmt.Sprintf("[%s:stdout]", sessionID)},
        &logWriter{logger: m.logger, prefix: fmt.Sprintf("[%s:stderr]", sessionID)},
        reader,
    )

    if err != nil && err != io.EOF {
        m.logger.Printf("Output stream error: session=%s error=%v", sessionID, err)
    }
}

// logWriter adapts Logger to io.Writer interface
type logWriter struct {
    logger Logger
    prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
    w.logger.Printf("%s %s", w.prefix, string(p))
    return len(p), nil
}

// StopSession stops a running container gracefully
func (m *Manager) StopSession(ctx context.Context, sessionID string) error {
    session := m.GetSession(sessionID)
    if session == nil {
        // Idempotent - already removed
        m.logger.Printf("Session not found during stop: %s (already removed?)", sessionID)
        return nil
    }

    // Idempotent - already stopped
    state := session.State()
    if state == StateStopped || state == StateFailed {
        m.logger.Printf("Session already stopped: id=%s state=%s", sessionID, state)
        return nil
    }

    containerID := session.ContainerID()
    if containerID == "" {
        m.logger.Printf("Session has no container: id=%s", sessionID)
        return nil
    }

    // Stop container with timeout (graceful shutdown)
    timeout := 30 // seconds
    err := m.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{
        Timeout: &timeout,
    })
    if err != nil {
        session.SetError(err.Error())
        m.logger.Printf("Container stop failed: session=%s container=%s error=%v", sessionID, containerID, err)
        return fmt.Errorf("failed to stop container: %w", err)
    }

    session.MarkStopped(m.clock.Now())
    m.logger.Printf("Container session stopped: id=%s container=%s state=STOPPED", sessionID, containerID)

    return nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) *ContainerSession {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.sessions[sessionID]
}

// ListSessions returns all tracked sessions
func (m *Manager) ListSessions() []*ContainerSession {
    m.mu.RLock()
    defer m.mu.RUnlock()

    sessions := make([]*ContainerSession, 0, len(m.sessions))
    for _, session := range m.sessions {
        sessions = append(sessions, session)
    }
    return sessions
}
```

### Phase 6: Package Documentation (doc.go)

**File:** `pkg/containersession/doc.go`

```go
/*
Package containersession provides container session management using Docker SDK.

This package enables isolated execution environments for agents by managing
Docker container lifecycle, workspace directories, and I/O streams.

# Basic Usage

	// Create manager with dependencies
	dockerClient := client.NewClientWithOpts(client.FromEnv)
	idGen := &UUIDGenerator{}
	clock := &SystemClock{}
	logger := log.New(os.Stdout, "[containersession] ", log.LstdFlags)

	manager := containersession.NewManager(dockerClient, idGen, clock, logger, "./workspaces")

	// Create and start a session
	session, err := manager.CreateSession(ctx, "ubuntu:latest", []string{"/bin/bash"})
	if err != nil {
	    log.Fatal(err)
	}

	err = manager.StartSession(ctx, session.ID())
	if err != nil {
	    log.Fatal(err)
	}

	// Stop the session
	err = manager.StopSession(ctx, session.ID())

# Thread Safety

All Manager methods are thread-safe and can be called concurrently.
ContainerSession methods use internal locking to ensure safe concurrent access.

# Label Conventions

Containers are labeled with:
  - com.ourocodus.containersession.id: Unique session ID
  - com.ourocodus.containersession.created: RFC3339 timestamp
  - com.ourocodus.containersession.managed-by: "ourocodus-containersession"

These labels enable discovery and management of containers across restarts.

# Workspace Security

Workspace paths are validated to prevent directory traversal attacks.
All workspace directories are created with 0700 permissions (owner-only access).
Path validation follows defense-in-depth principles from pkg/relay/session.
*/
package containersession
```

## Testing Strategy

### Unit Tests (80%+ coverage target)

**File:** `pkg/containersession/manager_test.go`

Mock implementations needed:
- `mockDockerClient` - implements DockerClient interface
- `mockIDGenerator` - implements IDGenerator interface
- `mockClock` - implements Clock interface
- `mockLogger` - implements Logger interface

Test cases:
1. `TestNewManager` - Constructor validation and defaults
2. `TestCreateSession` - Happy path and error cases
3. `TestStartSession` - Container start and I/O attachment
4. `TestStopSession` - Graceful shutdown and idempotency
5. `TestGetSession` - Session retrieval
6. `TestListSessions` - Session listing
7. `TestConcurrency` - Concurrent operations

**File:** `pkg/containersession/workspace_test.go`

Test cases:
1. `TestPrepareWorkspace` - Valid paths
2. `TestPrepareWorkspaceTraversal` - Path traversal prevention
3. `TestCleanupWorkspace` - Idempotent cleanup

## Dependencies to Add

```bash
go get github.com/docker/docker@latest
go get github.com/docker/docker/api/types/container@latest
go get github.com/docker/docker/api/types/filters@latest
go get github.com/docker/docker/client@latest
go get github.com/docker/docker/pkg/stdcopy@latest
```

## Pre-commit Checklist

Before committing, run:

```bash
make fmt        # Format with gofumpt
go vet ./...    # Basic static analysis
make lint       # golangci-lint
make test       # Run tests
make build      # Verify build
```

## Success Criteria

- [ ] All files created in pkg/containersession/
- [ ] Unit tests pass with 80%+ coverage
- [ ] No linting errors
- [ ] All code formatted with gofumpt
- [ ] Package documentation complete
- [ ] Security: Workspace path validation implemented
- [ ] Thread safety: All operations properly synchronized
- [ ] Idempotency: Stop operations handle already-stopped state

## Future Enhancements (Out of Scope for Phase 1)

- Phase 2: Container reuse and attach (Issue #102)
- Phase 3: Integration tests with real Docker (Issue #103)
- Phase 4: Production polish, error handling improvements (Issue #104)
- Event publishing to NATS (future milestone)
- Container resource limits (CPU, memory)
- Container health checks
- Container cleanup on manager shutdown
