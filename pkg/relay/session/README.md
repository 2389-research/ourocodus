# Session Management Package

This package provides in-memory session management for the relay, implementing the lifecycle defined in [SESSION_LIFECYCLE.md](../../../docs/SESSION_LIFECYCLE.md).

## Overview

The session package manages two-level session hierarchy:

1. **UserSession** - Container for WebSocket connection and 0-N agents
2. **AgentSession** - Individual ACP process with its own state machine

**Key principle:** UserSession is a container, not tied to a single agent. Agents can be spawned/terminated independently without affecting the UserSession.

## Architecture

```text
UserSession (ACTIVE/TERMINATED)
├── WebSocket Connection
├── AgentSession "auth" (SPAWNING/ACTIVE/FAILED/TERMINATED)
├── AgentSession "db" (SPAWNING/ACTIVE/FAILED/TERMINATED)
└── AgentSession "tests" (SPAWNING/ACTIVE/FAILED/TERMINATED)
```

### Components

1. **Manager** - Orchestrates session lifecycle, composes all dependencies
2. **Store** - Thread-safe in-memory storage for sessions
3. **UserSession** - Container for WebSocket and agents
4. **AgentSession** - Individual ACP process with state machine
5. **Cleaner** - Pluggable cleanup strategy for workspace directories
6. **ClientFactory** - Creates ACP clients for agent processes

## State Machines

### UserSession States

```text
ACTIVE → TERMINATED
```

- **ACTIVE**: Session created, WebSocket connected, can spawn 0-N agents
- **TERMINATED**: All agents terminated, WebSocket closed, session removed

### AgentSession States

```text
SPAWNING → ACTIVE/FAILED → TERMINATED
```

- **SPAWNING**: ACP process being spawned, workspace being created
- **ACTIVE**: ACP process running, accepting messages
- **FAILED**: Spawn failed or process crashed
- **TERMINATED**: Agent gracefully stopped and cleaned up

## Usage

### Creating a Manager

```go
import (
    "github.com/2389-research/ourocodus/pkg/relay/session"
    "github.com/2389-research/ourocodus/pkg/acp"
)

// Setup dependencies
store := session.NewMemoryStore()
idGen := myIDGenerator{}  // implements session.IDGenerator
clock := myClock{}        // implements session.Clock
cleaner := myCleaner{}    // implements session.Cleaner
logger := myLogger{}      // implements session.Logger
clientFactory := myFactory{}  // implements session.ClientFactory
baseWorkspaceDir := "./workspaces"

// Create manager
manager := session.NewManager(
    store,
    idGen,
    clock,
    cleaner,
    logger,
    clientFactory,
    baseWorkspaceDir,
)
```

### Session Lifecycle

```go
ctx := context.Background()

// 1. Create UserSession (ACTIVE state)
sess, err := manager.CreateUserSession(ctx, websocketConn)
if err != nil {
    // Handle error
}
sessionID := sess.ID

// 2. Spawn agent (creates AgentSession in SPAWNING → ACTIVE)
err = manager.SpawnAgent(ctx, sessionID, "auth", "./workspaces/auth")
if err != nil {
    // Handle spawn failure, session remains ACTIVE
}

// 3. Send message to agent
err = manager.SendMessageToAgent(ctx, sessionID, "auth", "Implement authentication")
if err != nil {
    // Handle message failure
}

// 4. Spawn more agents (independent lifecycles)
manager.SpawnAgent(ctx, sessionID, "db", "./workspaces/db")
manager.SpawnAgent(ctx, sessionID, "tests", "./workspaces/tests")

// 5. Terminate specific agent (others unaffected)
err = manager.TerminateAgent(ctx, sessionID, "auth")

// 6. Terminate entire session (all agents terminated)
err = manager.TerminateUserSession(ctx, sessionID)
```

### Querying Sessions

```go
// Get session by ID
session, err := manager.GetUserSession(sessionID)

// List all agents in a session
agents, err := manager.ListAgents(sessionID)

// Get specific agent
agent, err := manager.GetAgent(sessionID, "auth")

// Get conversation history
history, err := manager.GetAgentHistory(sessionID, "auth")

// Count sessions
count := manager.CountUserSessions()

// List all sessions
sessions := manager.ListUserSessions()
```

## Design Principles

### 1. Two-Level Hierarchy

- **UserSession** = WebSocket + 0-N agents
- **AgentSession** = ACP process + state + history
- Independent agent lifecycles (failure isolation)

### 2. Separation of Concerns

- **UserSession**: WebSocket lifecycle, agent container
- **AgentSession**: ACP process lifecycle, message history
- **Manager**: Orchestration, dependency injection
- **Store**: Thread-safe storage

### 3. Dependency Injection

All collaborators injected through constructor:

```go
func NewManager(
    store Store,
    idGen IDGenerator,
    clock Clock,
    cleaner Cleaner,
    logger Logger,
    clientFactory ClientFactory,
    baseWorkspaceDir string,
) *Manager
```

Tests can supply fakes/mocks for deterministic behavior.

### 4. Interface Boundaries

Depend on contracts, not implementations:

```go
type Store interface {
    AddUserSession(session *UserSession)
    GetUserSession(id string) (*UserSession, error)
    RemoveUserSession(id string) error
    // ...
}

type ClientFactory interface {
    NewClient(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error)
}
```

Future phases can swap implementations without changing callers.

### 5. Runtime Context and Container Integration

Each ACP client receives runtime context containing session/agent metadata and optional container information:

```go
type AgentRuntimeContext struct {
    SessionID   string  // User session identifier
    AgentID     string  // Agent role (e.g., "coder", "reviewer")
    Workspace   string  // Absolute path to workspace on host
    ContainerID string  // Docker container ID (optional, for container mode)
}

func (c *AgentRuntimeContext) HasContainer() bool {
    return c != nil && c.ContainerID != ""
}
```

**ACP Launcher Selection:**

The `ACPClientFactory` automatically selects the appropriate launcher based on environment and runtime context:

- **Host Mode** (default): `HostProcessLauncher` spawns ACP as host process via `os/exec`
  - Selected when: `OUROCODUS_ACP_RUNTIME` unset or set to `"host"`
  - Behavior: Direct process spawn, standard workspace access

- **Container Mode**: `ContainerExecProcessLauncher` runs ACP inside agent containers via `docker exec`
  - Selected when: `OUROCODUS_ACP_RUNTIME=container` AND runtime has container ID
  - Behavior: Exec into existing container, workspace path rewriting for container mounts
  - Requires: `ContainerExecService` (typically `containersession.Manager`)

**Workspace Path Rewriting:**

In container mode, workspace arguments are automatically rewritten:
- Host path: `/Users/dev/workspaces/session-123`
- Container path: `/workspace` (standard mount point)
- Handled by: `rewriteWorkspaceArg()` in `container_exec_process_launcher.go`

**Example Configuration:**

```bash
# Host mode (default)
export ANTHROPIC_API_KEY=sk-...
./relay

# Container mode
export ANTHROPIC_API_KEY=sk-...
export OUROCODUS_ACP_RUNTIME=container
./relay
```

See [../../../docs/ACP.md](../../../docs/ACP.md) for complete launcher selection documentation.

## Thread Safety

### Manager Operations

All Manager methods are thread-safe. Concurrent calls to `CreateUserSession`, `SpawnAgent`, etc. are safe.

### Session Mutations

- UserSession uses `sync.RWMutex` for agents map and state
- AgentSession uses `sync.RWMutex` for internal state
- All mutations go through Manager methods that acquire appropriate locks

### Store

MemoryStore uses `sync.RWMutex` for thread-safe access to session map.

**Verified with:** `go test -race ./pkg/relay/session/...`

## Error Handling

The package defines sentinel errors for common failures:

```go
var (
    ErrSessionNotFound = errors.New("session not found")
    ErrAgentNotFound   = errors.New("agent not found")
)
```

Use `errors.Is()` to check for specific errors:

```go
err := manager.GetAgent(sessionID, "auth")
if errors.Is(err, session.ErrSessionNotFound) {
    // Session doesn't exist
} else if errors.Is(err, session.ErrAgentNotFound) {
    // Agent role not found in session
}
```

## Testing

### Running Tests

```bash
# Unit tests
go test ./pkg/relay/session/...

# With race detector
go test -race ./pkg/relay/session/...

# Verbose output
go test -v ./pkg/relay/session/...

# Coverage
go test -cover ./pkg/relay/session/...
```

### Test Coverage

- **Manager**: Session lifecycle, agent lifecycle, error handling, concurrency
- **Store**: CRUD operations, thread safety
- **Models**: State transitions, validation
- **Mocks**: All dependencies have test mocks (ClientFactory, Clock, Cleaner, Logger)

## Files

```text
pkg/relay/session/
├── README.md              # This file
├── models.go              # UserSession, AgentSession, states
├── manager.go             # Public API with dependency injection
├── store.go               # Store interface
├── memory_store.go        # In-memory Store implementation
├── client_factory.go      # ACP client factory
├── cleaner.go             # Workspace cleanup
├── errors.go              # Sentinel errors
├── manager_test.go        # Manager tests
├── models_test.go         # Model tests
└── *_test.go              # Additional test files
```

## Phase 1 Limitations

- **No persistence**: Sessions lost on relay restart
- **No reconnection**: WebSocket disconnect terminates session
- **In-memory only**: Not suitable for horizontal scaling
- **Manual cleanup**: Workspaces persist after session termination

## Future Enhancements

Later phases will add:

- Persistent storage (SQLite/PostgreSQL event store)
- Session reconnection support
- Automatic workspace cleanup
- Metrics and observability hooks
- Session recovery after relay restart

## Dependencies

- **Standard library** for core logic
- **pkg/acp** for ACP client integration
- **gorilla/websocket** (via relay server, not direct dependency)

## References

- [SESSION_LIFECYCLE.md](../../../docs/SESSION_LIFECYCLE.md) - Detailed state machine spec
- [docs/ARCHITECTURE.md](../../../docs/ARCHITECTURE.md) - Overall system architecture
- [docs/ERROR_HANDLING.md](../../../docs/ERROR_HANDLING.md) - Error handling strategy
