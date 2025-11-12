# ContainerSession Library: Design and Integration Plan

**Date:** 2025-11-03
**Purpose:** Build core container session management component in ourocodus

## Executive Summary

Both PackNplay and ACP-Relay contain a **session-based container management** component. We're building this component directly in ourocodus as `pkg/containersession`. This uses the Docker SDK directly (not CLI) to avoid TTY issues, provides container reuse, I/O streaming, and label-based discovery.

**Benefits:**
- ✅ **No TTY issues** - Docker SDK API, not exec.Command
- ✅ **Observable** - Container labels built-in
- ✅ **Testable** - Isolated unit tests
- ✅ **Maintainable** - Single source of truth for Docker patterns
- ✅ **Can extract later** - Start in-repo, can move to separate library if needed

---

## Package Structure: `pkg/containersession`

### Package Layout

```
pkg/containersession/
├── manager.go              # Manager, CreateSession, StopSession, ListSessions
├── session.go              # Session struct and methods
├── config.go               # Config, SessionConfig, Volume types
├── labels.go               # Label building and filtering helpers
├── streams.go              # I/O demuxing (stdout/stderr separation)
├── errors.go               # Error types and helpers
├── manager_test.go         # Manager tests
├── session_test.go         # Session tests
├── integration_test.go     # E2E tests (requires Docker)
└── doc.go                  # Package documentation

examples/containersession/  # Example programs
├── basic/
│   └── main.go            # Simple single-session example
├── multi/
│   └── main.go            # Multiple concurrent sessions
└── echo-agent/
    └── main.go            # Echo agent example
```

### Core API

```go
package containersession

import (
    "context"
    "io"
    "github.com/docker/docker/client"
)

// Manager handles Docker container sessions
type Manager struct {
    dockerClient *client.Client
    config       Config
}

// Config for the container manager
type Config struct {
    DockerHost      string // e.g., "unix:///var/run/docker.sock"
    ManagedBy       string // Label value (e.g., "ourocodus")
    WorkspaceBase   string // Base directory for workspaces
    AutoRemove      bool   // Remove containers on stop
    DefaultImage    string // Fallback image
}

// SessionConfig describes what to run
type SessionConfig struct {
    SessionID    string            // Unique identifier
    Image        string            // Docker image
    Command      []string          // Command to run
    Env          map[string]string // Environment variables
    Volumes      []Volume          // Volume mounts
    WorkingDir   string            // Container working directory
    MemoryLimit  int64             // Memory limit in bytes
    CPULimit     float64           // CPU limit (1.0 = 1 core)
    NetworkMode  string            // e.g., "bridge", "host"
    Labels       map[string]string // Additional custom labels
}

// Volume represents a host:container mount
type Volume struct {
    HostPath      string
    ContainerPath string
    ReadOnly      bool
}

// Session represents a running container session
type Session struct {
    ID          string
    ContainerID string
    Stdin       io.WriteCloser
    Stdout      io.ReadCloser
    Stderr      io.ReadCloser
    Workspace   string // Host path to workspace
}

// SessionInfo for listing
type SessionInfo struct {
    SessionID   string
    ContainerID string
    State       string
    CreatedAt   int64
}

// API Methods
func NewManager(config Config) (*Manager, error)
func (m *Manager) CreateSession(ctx context.Context, cfg SessionConfig) (*Session, error)
func (m *Manager) AttachSession(ctx context.Context, sessionID string) (*Session, error)
func (m *Manager) StopSession(ctx context.Context, sessionID string) error
func (m *Manager) ListSessions(ctx context.Context) ([]SessionInfo, error)
func (m *Manager) Close() error
```

### Key Features

**Container Reuse:**
```go
// CreateSession checks for existing containers with matching session ID
// If found and running, reattaches instead of creating new container
existingID, err := m.findContainer(ctx, cfg.SessionID)
if existingID != "" {
    return m.attachToExisting(ctx, existingID, cfg.SessionID)
}
```

**I/O Streaming with Demux:**
```go
// Docker multiplexes stdout/stderr in a single stream
// Must use stdcopy.StdCopy to separate them
attachResp, err := m.dockerClient.ContainerAttach(ctx, containerID, container.AttachOptions{
    Stream: true,
    Stdin:  true,
    Stdout: true,
    Stderr: true,
})

// Demux into separate streams
stdoutReader, stderrReader := demuxStreams(attachResp.Reader)
```

**Label-Based Discovery:**
```go
// All containers get standard labels
labels := map[string]string{
    "managed-by": m.config.ManagedBy,
    "session-id": cfg.SessionID,
    "created-at": time.Now().UTC().Format(time.RFC3339),
}

// Plus custom labels from SessionConfig
for k, v := range cfg.Labels {
    labels[k] = v
}

// Queryable via Docker API
docker ps --filter label=managed-by=ourocodus --filter label=session-id=abc123
```

**No TTY Issues:**
```go
// Docker SDK API - no exec.Command, no stdin inheritance
&container.Config{
    Tty:       false, // NO TTY!
    OpenStdin: true,  // But stdin is available
    // ...
}
```

---

## Build Plan: 4 Phases

### Phase 1: Core Package

**Goal:** Minimal working package with basic container management

**Tasks:**
- [ ] Create `pkg/containersession/` package structure
- [ ] Implement `Manager` struct and `NewManager()`
- [ ] Implement `CreateSession()` with Docker SDK
  - Container creation via `ContainerCreate()`
  - Container start via `ContainerStart()`
  - I/O attachment via `ContainerAttach()`
  - Stream demuxing with `stdcopy.StdCopy()`
- [ ] Implement `StopSession()` with graceful shutdown
- [ ] Implement `ListSessions()` with label filtering
- [ ] Implement label building and discovery
- [ ] Implement workspace directory management
- [ ] Write unit tests (mock Docker client)
- [ ] Write basic README with examples

**Deliverables:**
- Working package that can create/stop/list sessions
- Unit tests (80%+ coverage)
- Basic documentation

### Phase 2: Container Reuse & Attach

**Goal:** Support reusing existing containers and attaching to them

**Tasks:**
- [ ] Implement `findContainer()` for label-based lookup
- [ ] Add reuse logic to `CreateSession()`
- [ ] Implement `AttachSession()` for reconnecting
- [ ] Add tests for reuse scenarios
- [ ] Add tests for attach scenarios
- [ ] Document reuse behavior

**Deliverables:**
- Container reuse working
- Attach to existing sessions working
- Tests for both features

### Phase 3: Examples & Integration Tests

**Goal:** Comprehensive examples and E2E validation

**Tasks:**
- [ ] Create `examples/basic/` - single session
- [ ] Create `examples/multi/` - concurrent sessions
- [ ] Create `examples/echo-agent/` - real agent example
- [ ] Write integration tests (requires Docker)
- [ ] Add CI workflow with Docker
- [ ] Document testing approach

**Deliverables:**
- 3 working examples
- Integration test suite
- CI pipeline running tests

### Phase 4: Production Polish

**Goal:** Production-ready library with error handling and docs

**Tasks:**
- [ ] Implement structured errors
- [ ] Add resource cleanup on errors
- [ ] Add timeout handling
- [ ] Add verbose logging option
- [ ] Complete README with all features
- [ ] Add godoc comments to all exports
- [ ] Add architecture diagram to README
- [ ] Add troubleshooting section

**Deliverables:**
- Production-quality error handling
- Complete documentation
- Ready for use in ourocodus

---

## Integration into Ourocodus

### New Package: `pkg/agent/container/`

Replace `pkg/agent/packnplay/` with `pkg/agent/container/` that wraps containersession.

**Structure:**
```
pkg/agent/container/
├── launcher.go         # ContainerLauncher implements agent.Launcher
├── launcher_test.go    # Unit tests with mock Manager
├── handle.go           # ContainerHandle implements agent.AgentHandle
├── integration_test.go # E2E tests (requires Docker)
└── doc.go              # Package documentation
```

### ContainerLauncher Implementation

```go
// pkg/agent/container/launcher.go
package container

import (
    "context"
    "fmt"

    "github.com/2389-research/containersession"
    "github.com/2389-research/ourocodus/pkg/agent"
    "github.com/oklog/ulid/v2"
)

type ContainerLauncher struct {
    manager   *containersession.Manager
    projectPath string
    defaultImage string
}

type LauncherOption func(*ContainerLauncher) error

func WithProjectPath(path string) LauncherOption {
    return func(l *ContainerLauncher) error {
        l.projectPath = path
        return nil
    }
}

func WithDefaultImage(image string) LauncherOption {
    return func(l *ContainerLauncher) error {
        l.defaultImage = image
        return nil
    }
}

func WithDockerHost(host string) LauncherOption {
    return func(l *ContainerLauncher) error {
        // Create manager with custom Docker host
        // ...
        return nil
    }
}

func NewLauncher(opts ...LauncherOption) (*ContainerLauncher, error) {
    l := &ContainerLauncher{
        defaultImage: "ubuntu:22.04",
    }

    // Apply options
    for _, opt := range opts {
        if err := opt(l); err != nil {
            return nil, err
        }
    }

    // Create containersession Manager
    manager, err := containersession.NewManager(containersession.Config{
        DockerHost:    "unix:///var/run/docker.sock",
        ManagedBy:     "ourocodus",
        WorkspaceBase: "/tmp/ourocodus-workspaces",
        AutoRemove:    true,
        DefaultImage:  l.defaultImage,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create container manager: %w", err)
    }

    l.manager = manager
    return l, nil
}

// Spawn implements agent.Launcher
func (l *ContainerLauncher) Spawn(ctx context.Context, cfg *agent.SpawnConfig) (agent.AgentHandle, error) {
    // Generate unique session ID
    sessionID := ulid.Make().String()

    // Convert agent.SpawnConfig → containersession.SessionConfig
    sessionCfg := containersession.SessionConfig{
        SessionID:   sessionID,
        Image:       cfg.Image,
        Command:     cfg.Command,
        Env:         cfg.Environment,
        WorkingDir:  "/workspace",
        MemoryLimit: 512 * 1024 * 1024, // 512MB
        CPULimit:    1.0,
        Labels: map[string]string{
            "role": cfg.Role,
        },
    }

    // Add volumes from cfg.Workspace
    if cfg.Workspace != "" {
        sessionCfg.Volumes = []containersession.Volume{
            {
                HostPath:      cfg.Workspace,
                ContainerPath: "/workspace",
                ReadOnly:      false,
            },
        }
    }

    // Create session via containersession
    session, err := l.manager.CreateSession(ctx, sessionCfg)
    if err != nil {
        return nil, fmt.Errorf("failed to create container session: %w", err)
    }

    // Wrap in ContainerHandle
    return &ContainerHandle{
        id:          sessionID,
        containerID: session.ContainerID,
        workspace:   session.Workspace,
        stdin:       session.Stdin,
        stdout:      session.Stdout,
        stderr:      session.Stderr,
        role:        cfg.Role,
    }, nil
}

// Attach implements agent.Launcher
func (l *ContainerLauncher) Attach(ctx context.Context, id string) (agent.AgentHandle, error) {
    session, err := l.manager.AttachSession(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to attach to session: %w", err)
    }

    return &ContainerHandle{
        id:          id,
        containerID: session.ContainerID,
        workspace:   session.Workspace,
        stdin:       session.Stdin,
        stdout:      session.Stdout,
        stderr:      session.Stderr,
    }, nil
}

// Stop implements agent.Launcher
func (l *ContainerLauncher) Stop(ctx context.Context, handle agent.AgentHandle) error {
    h, ok := handle.(*ContainerHandle)
    if !ok {
        return fmt.Errorf("invalid handle type")
    }

    return l.manager.StopSession(ctx, h.id)
}
```

### ContainerHandle Implementation

```go
// pkg/agent/container/handle.go
package container

import (
    "context"
    "io"
)

type ContainerHandle struct {
    id          string
    containerID string
    workspace   string
    stdin       io.WriteCloser
    stdout      io.ReadCloser
    stderr      io.ReadCloser
    role        string
}

func (h *ContainerHandle) ID() string              { return h.id }
func (h *ContainerHandle) ContainerID() string     { return h.containerID }
func (h *ContainerHandle) Workspace() string       { return h.workspace }
func (h *ContainerHandle) Stdin() io.WriteCloser   { return h.stdin }
func (h *ContainerHandle) Stdout() io.ReadCloser   { return h.stdout }
func (h *ContainerHandle) Stderr() io.ReadCloser   { return h.stderr }

func (h *ContainerHandle) Wait(ctx context.Context) error {
    // Wait for stdout/stderr to close (indicates container exit)
    buf := make([]byte, 1)
    for {
        _, err := h.stdout.Read(buf)
        if err != nil {
            return nil // EOF or error = container stopped
        }
    }
}

func (h *ContainerHandle) Close() error {
    // Close all streams
    var firstErr error
    if err := h.stdin.Close(); err != nil && firstErr == nil {
        firstErr = err
    }
    if err := h.stdout.Close(); err != nil && firstErr == nil {
        firstErr = err
    }
    if err := h.stderr.Close(); err != nil && firstErr == nil {
        firstErr = err
    }
    return firstErr
}
```

### Git Worktree Management

**Option 1: Keep in ourocodus (Recommended)**

Create `pkg/worktree/manager.go` in ourocodus that handles git worktree creation/deletion:

```go
// pkg/worktree/manager.go
package worktree

import (
    "fmt"
    "os/exec"
    "path/filepath"
)

type Manager struct {
    projectPath string
}

func NewManager(projectPath string) *Manager {
    return &Manager{projectPath: projectPath}
}

func (m *Manager) Create(name string) (string, error) {
    workspacePath := filepath.Join(m.projectPath, ".worktrees", name)

    cmd := exec.Command("git", "worktree", "add", "-b", name, workspacePath, "HEAD")
    cmd.Dir = m.projectPath
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("failed to create worktree: %w", err)
    }

    return workspacePath, nil
}

func (m *Manager) Remove(name, path string) error {
    // Remove worktree
    cmd := exec.Command("git", "worktree", "remove", "-f", path)
    cmd.Dir = m.projectPath
    _ = cmd.Run() // Tolerate errors

    // Delete branch
    cmd = exec.Command("git", "branch", "-D", name)
    cmd.Dir = m.projectPath
    _ = cmd.Run() // Tolerate errors

    return nil
}
```

Then `ContainerLauncher.Spawn()` becomes:

```go
func (l *ContainerLauncher) Spawn(ctx context.Context, cfg *agent.SpawnConfig) (agent.AgentHandle, error) {
    sessionID := ulid.Make().String()

    // Create git worktree if needed
    var workspace string
    if cfg.Workspace == "" {
        worktreeName := fmt.Sprintf("agent-%s", sessionID)
        ws, err := l.worktreeManager.Create(worktreeName)
        if err != nil {
            return nil, fmt.Errorf("failed to create worktree: %w", err)
        }
        workspace = ws
    } else {
        workspace = cfg.Workspace
    }

    // Create container with workspace mounted
    // ...
}
```

**Option 2: Add to containersession**

Add git worktree support as an optional feature in containersession if it's generally useful.

---

## Updated Milestone Plans

### Current: Milestone 2 - PWA + NATS Foundation

**Before (PackNplay Integration Track):**
- Issue #83: PacknplayLauncher Implementation
- Issue #84: Configure Agent Credentials & Environment
- Issue #85: Update Relay to Use AgentLauncher
- Issue #86: E2E Tests for Containerized Agents
- Issue #87: Documentation - Packnplay Architecture Integration

**After (ContainerSession Integration Track):**

#### Issue #83: ContainerLauncher Implementation (Updated)

**Goal:** Implement AgentLauncher interface using containersession library

**Dependencies:**
- containersession library (Phases 1-4)

**Tasks:**
- [ ] Add `github.com/2389-research/containersession` dependency
- [ ] Create `pkg/agent/container/launcher.go`
- [ ] Implement `Spawn()` using `containersession.Manager`
- [ ] Implement `Stop()` using `containersession.Manager.StopSession()`
- [ ] Implement `Attach()` using `containersession.Manager.AttachSession()`
- [ ] Create `ContainerHandle` that implements `AgentHandle`
- [ ] Create `pkg/worktree/manager.go` for git worktree management
- [ ] Integrate worktree creation in `Spawn()`
- [ ] Add error handling
- [ ] Write unit tests (mock containersession.Manager)

**Acceptance Criteria:**
- [ ] Can spawn containerized agent with `Spawn()`
- [ ] Can stop running agent with `Stop()`
- [ ] Can attach to existing container with `Attach()`
- [ ] Worktrees are created automatically
- [ ] Errors are properly wrapped and returned
- [ ] Unit tests pass

#### Issue #84: Configure Agent Credentials & Environment (Mostly Unchanged)

**Goal:** Set up credential mounting and environment variables

**Tasks:**
- [ ] Configure GitHub CLI credential mounting (via volumes)
- [ ] Configure AWS credential support (via volumes)
- [ ] Configure Git credential helper mounting
- [ ] Set up `ANTHROPIC_API_KEY` passthrough
- [ ] Add credential configuration to `SpawnConfig`
- [ ] Document credential setup in README
- [ ] Test credential access from inside container

**Changes:**
- Uses `containersession.Volume` for credential mounts instead of Packnplay config
- No built-in credential helpers, but explicit volume mounts


#### Issue #85: Update Relay to Use AgentLauncher (Minimal Changes)

**Goal:** Replace current agent spawning with `ContainerLauncher`

**Tasks:**
- [ ] Add `AgentLauncher` field to Relay struct
- [ ] Inject `ContainerLauncher` at Relay startup
- [ ] Update `agent:spawn` handler to use `launcher.Spawn()`
- [ ] Update `agent:terminate` handler to use `launcher.Stop()`
- [ ] Remove old direct `exec.Command()` agent spawning code
- [ ] Update spawn progress reporting
- [ ] Update Relay tests to use mock AgentLauncher

**Changes:**
- Use `container.NewLauncher()` instead of `packnplay.NewLauncher()`
- API is nearly identical, minimal changes needed


#### Issue #86: E2E Tests for Containerized Agents (Mostly Unchanged)

**Goal:** Create comprehensive E2E tests for containerized agent lifecycle

**Tasks:**
- [ ] Add E2E test: Spawn containerized echo-agent
- [ ] Add E2E test: Spawn containerized Claude Code agent
- [ ] Add E2E test: Multiple concurrent containerized agents
- [ ] Add E2E test: Container cleanup on agent termination
- [ ] Add E2E test: Worktree creation and isolation
- [ ] Add E2E test: Credential mounting and access
- [ ] Document container cleanup requirements

**Changes:**
- Tests use `container.Launcher` instead of `packnplay.Launcher`
- Can also add tests directly in containersession library


#### Issue #87: Documentation - Container Runtime Architecture (Updated)

**Goal:** Document the containersession integration architecture

**Tasks:**
- [ ] Create `docs/AGENT_RUNTIME.md` explaining the abstraction
- [ ] Document how containersession is integrated
- [ ] Update `docs/ARCHITECTURE.md` with agent runtime layer
- [ ] Add diagram: PWA → Relay → ContainerLauncher → containersession → Docker
- [ ] Document credential configuration requirements
- [ ] Update README with containerization details
- [ ] Add troubleshooting section for container issues
- [ ] Document future: how to swap for K8s

**Changes:**
- Focus on containersession library instead of Packnplay
- Explain separation of concerns (git worktrees vs container management)


### New Issue: Build ContainerSession Library

#### Issue #NEW: Build ContainerSession Library

**Goal:** Create standalone container session management library

**Priority:** HIGH (blocking for #83)

**Milestone:** Milestone 2: PWA + NATS Foundation

**Labels:** `type:library`, `priority:high`, `component:agent`, `track:container-integration`

**Tasks:**
- [ ] Phase 1: Core Library (Manager, CreateSession, StopSession, ListSessions)
- [ ] Phase 2: Container Reuse & Attach
- [ ] Phase 3: Examples & Integration Tests
- [ ] Phase 4: Production Polish (errors, docs, CI)
- [ ] Create repository `github.com/2389-research/containersession`
- [ ] Set up CI with Docker
- [ ] Write comprehensive README
- [ ] Tag v0.1.0 release

**Acceptance Criteria:**
- [ ] Library can create/stop/list container sessions
- [ ] Container reuse works (find existing by session ID)
- [ ] Attach to existing sessions works
- [ ] I/O streaming works (demuxed stdout/stderr)
- [ ] Unit tests pass (80%+ coverage)
- [ ] Integration tests pass (requires Docker)
- [ ] Examples run successfully
- [ ] Documentation complete

**Dependencies:**
- None (standalone)

**Blocks:**
- #83 (ContainerLauncher Implementation)


---

## Implementation Path

### Step 1: Build containersession package
- Create `pkg/containersession/` package structure
- Implement Phases 1-4
- Test and document

### Step 2: Integrate into agent layer
- Implement `pkg/agent/container/launcher.go`
- Implement `pkg/worktree/manager.go` (if needed)
- Write tests
- Update container-race demo

### Step 3: Update Relay
- Replace old agent spawning with ContainerLauncher
- Update tests
- Verify E2E tests pass

### Step 4: Documentation & Polish
- Update architecture docs
- Add troubleshooting guides
- Complete E2E test suite
- Production testing

---

## Comparison: Before vs After

### Before (PackNplay Fork Approach)

**Stack:**
```
ourocodus
  └─> PacknplayLauncher
      └─> PackNplay (fork)
          └─> Docker CLI (exec.Command)
              └─> Worktree management
```

**Issues:**
- TTY errors (Docker CLI)
- Fork maintenance burden
- Tight coupling to git worktrees
- Can't reuse for acp-relay

### After (ContainerSession Approach)

**Stack:**
```
ourocodus
  ├─> ContainerLauncher
  │   └─> containersession (library)
  │       └─> Docker SDK (client.Client)
  └─> worktree.Manager
      └─> git worktree commands
```

**Benefits:**
- ✅ No TTY issues (Docker SDK)
- ✅ No fork maintenance
- ✅ Separation of concerns (containers vs worktrees)
- ✅ Reusable across projects (acp-relay, etc.)
- ✅ Better testability
- ✅ Single source of truth for Docker patterns

---

## Dependencies

### pkg/containersession:
- `github.com/docker/docker` (Docker SDK) - Already in ourocodus!
- `github.com/docker/docker/pkg/stdcopy` - Stream demuxing
- Standard library only

### pkg/agent/container:
- `github.com/2389-research/ourocodus/pkg/containersession` (local package)
- `github.com/oklog/ulid/v2` (already using)
- Everything else already in place

---

## Risk Analysis

### Technical Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Docker SDK API changes | High | Low | Pin to stable version, test matrix |
| Stream handling bugs | Medium | Medium | Comprehensive I/O tests |
| Resource leaks | High | Low | Proper cleanup in Close/Stop methods |
| Container reuse edge cases | Medium | Medium | Extensive integration tests |

### Implementation Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Complexity underestimated | Medium | Medium | Incremental releases, MVP first |
| Integration issues in ourocodus | Low | Low | API designed to match existing Launcher interface |
| Docker availability in CI | Low | Medium | Use Docker-in-Docker or hosted runners |

### Maintenance Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Package becomes stale | Medium | Low | In-repo, regularly tested via agent launcher |
| Breaking API changes | Medium | Low | Clear interface boundaries, extensive tests |
| Documentation gaps | Medium | Medium | Examples + comprehensive README |

---

## Success Criteria

### pkg/containersession package:
- [ ] ✅ Can create/stop/list container sessions
- [ ] ✅ Container reuse works
- [ ] ✅ I/O streaming works (separate stdout/stderr)
- [ ] ✅ Unit tests ≥80% coverage
- [ ] ✅ Integration tests pass with Docker
- [ ] ✅ 3 working examples
- [ ] ✅ Complete documentation

### ourocodus Integration:
- [ ] ✅ ContainerLauncher implements agent.Launcher
- [ ] ✅ Worktree manager handles git worktrees
- [ ] ✅ Container-race demo works
- [ ] ✅ Can spawn/stop/attach agents
- [ ] ✅ Credentials mount correctly
- [ ] ✅ E2E tests pass
- [ ] ✅ No PackNplay fork dependency

### Documentation:
- [ ] ✅ Architecture docs updated
- [ ] ✅ Agent runtime explained
- [ ] ✅ Troubleshooting guide exists
- [ ] ✅ Migration from Packnplay documented

---

## Open Questions

1. **Should containersession handle git worktrees?**
   - **Decision:** No. Keep worktrees in ourocodus (`pkg/worktree`). Git worktrees are project-specific, containers are general-purpose.

2. **Should we support non-Docker runtimes (Podman, K8s)?**
   - **Decision:** Design for it (interface abstraction), implement Docker first, add others later if needed.

3. **How to handle credential mounting?**
   - **Decision:** Use `Volume` mounts. Document common credential paths. Let caller handle specifics.

4. **What about the PackNplay fork work?**
   - **Decision:** Archive it. The investigation was valuable (taught us Docker SDK is the right approach), but we don't need the fork anymore.

---

## Next Steps

**Immediate:**
1. Review and approve this plan
2. Clean up existing packnplay-related issues and milestones
3. Create new milestone: "Container Session Management"
4. Create new milestone: "Container Integration"

**Phase 1: Build containersession**
- Create `pkg/containersession/` package structure
- Implement core container management (Phases 1-4)
- Write tests and documentation

**Phase 2: Integration**
- Implement `pkg/agent/container/` wrapper
- Implement `pkg/worktree/` if needed
- Update milestone issues
- Update container-race demo

**Phase 3: Production**
- E2E testing
- Documentation updates
- Production validation
