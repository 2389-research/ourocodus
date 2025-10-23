# Session Management Package

This package provides in-memory session management for the relay, implementing the lifecycle defined in [SESSION_LIFECYCLE.md](../../../docs/SESSION_LIFECYCLE.md).

## Overview

The session package manages the lifecycle of WebSocket sessions between PWA clients and ACP agents. Each session represents one user conversation with one AI agent, progressing through defined states from creation to cleanup.

## Architecture

The package follows strict separation of concerns with dependency injection:

```
┌─────────────────┐
│    Manager      │ ← Public API
│  (coordinator)  │
└────────┬────────┘
         │
    ┌────┴────┬──────────┬──────────┐
    │         │          │          │
┌───▼────┐ ┌─▼────┐ ┌───▼──────┐ ┌──▼──────┐
│  Store │ │State │ │  Cleaner │ │ IDGen   │
│(memory)│ │Machine│ │  (hooks) │ │ Clock   │
└────────┘ └──────┘ └──────────┘ └─────────┘
```

### Components

1. **Manager** - Orchestrates session lifecycle, composes all dependencies
2. **Store** - Thread-safe in-memory storage for sessions
3. **StateMachine** - Pure functions for state transitions
4. **Session** - Immutable domain model with controlled mutation
5. **Cleaner** - Pluggable cleanup strategy (no-op in Phase 1)

## State Machine

Sessions progress through these states:

```
CREATED → SPAWNING → ACTIVE → TERMINATING → CLEANED
```

Valid transitions:
- `CREATED + SPAWN → SPAWNING`
- `CREATED + TERMINATE → TERMINATING` (early cancel)
- `SPAWNING + ACTIVATE → ACTIVE`
- `SPAWNING + TERMINATE → TERMINATING` (spawn failure)
- `ACTIVE + TERMINATE → TERMINATING`
- `TERMINATING + CLEAN → CLEANED`
- `TERMINATING + TERMINATE → TERMINATING` (idempotent)

Invalid transitions return `TransitionError`.

## Usage

### Creating a Manager

```go
import (
    "github.com/2389-research/ourocodus/pkg/relay/session"
)

// Setup dependencies
store := session.NewMemoryStore()
idGen := myIDGenerator{}  // implements session.IDGenerator
clock := myClock{}        // implements session.Clock
cleaner := session.NewNoOpCleaner()
logger := myLogger{}      // implements session.Logger

// Create manager
manager := session.NewManager(store, idGen, clock, cleaner, logger)
```

### Session Lifecycle

```go
ctx := context.Background()

// 1. Create session (CREATED state)
sess, err := manager.Create(ctx, "auth", websocketConn)
if err != nil {
    // Handle duplicate role or validation error
}

// 2. Begin spawning ACP process (CREATED → SPAWNING)
err = manager.BeginSpawn(ctx, sess.GetID())

// 3. Attach ACP client after spawn succeeds (SPAWNING → ACTIVE)
err = manager.AttachAgent(ctx, sess.GetID(), "/path/to/worktree", acpClient)

// 4. Track activity
manager.RecordHeartbeat(ctx, sess.GetID())
manager.IncrementMessageCount(ctx, sess.GetID())

// 5. Begin termination (ACTIVE → TERMINATING)
err = manager.MarkTerminating(ctx, sess.GetID(), "user requested")

// 6. Complete cleanup (TERMINATING → CLEANED, removes from store)
err = manager.CompleteCleanup(ctx, sess.GetID())
```

### Querying Sessions

```go
// Get by ID
session := manager.Get(sessionID)

// Get by agent role
session := manager.GetByRole("auth")

// List all sessions
allSessions := manager.List(nil)

// List by state
activeState := session.StateActive
activeSessions := manager.List(&session.SessionFilter{
    State: &activeState,
})

// List by agent role
authID := "auth"
authSessions := manager.List(&session.SessionFilter{
    AgentID: &authID,
})

// Count sessions
count := manager.Count()
```

## Design Principles

### 1. Separation of Concerns

- **Data containers** (Session, Handle) - Hold state, no behavior
- **State transitions** (StateMachine) - Pure logic, no side effects
- **Coordination** (Manager) - Orchestrates dependencies, handles concurrency

### 2. Pure Logic First

State transitions are implemented as pure functions:

```go
func NextState(current SessionState, event Event) (SessionState, error)
```

No side effects, fully deterministic, trivial to test.

### 3. Dependency Injection

All collaborators injected through constructor:

```go
func NewManager(
    store Store,
    idGen IDGenerator,
    clock Clock,
    cleaner Cleaner,
    logger Logger,
) *Manager
```

Tests can supply fakes/mocks for deterministic behavior.

### 4. Interface Boundaries

Depend on contracts, not implementations:

```go
type Store interface {
    Create(session *Session) error
    Get(id string) *Session
    // ...
}
```

Future phases can swap implementations without changing callers.

## Thread Safety

### Manager Operations

All Manager methods are thread-safe. Concurrent calls to `Create`, `Get`, `List`, etc. are safe.

### Session Mutations

Session fields are protected by internal mutex (`sync.RWMutex`). All state changes flow through Manager methods that acquire appropriate locks.

### Store

MemoryStore uses `sync.RWMutex` for thread-safe access to session maps.

**Verified with:** `go test -race ./pkg/relay/session/...`

## Testing

### Running Tests

```bash
# Unit tests
go test ./pkg/relay/session/...

# With race detector
go test -race ./pkg/relay/session/...

# Verbose output
go test -v ./pkg/relay/session/...
```

### Test Coverage

- **State machine**: Table-driven tests for all valid/invalid transitions
- **Manager**: Creation, lookup, lifecycle, cleanup, concurrency
- **Store**: CRUD operations, filtering, thread safety
- **Mocks**: All dependencies have test mocks (IDGenerator, Clock, Cleaner, Logger)

## Future Enhancements

Issue #7 will extend this package with:
- Real cleanup logic (close WebSocket, terminate ACP, remove worktree)
- Integration with ACP client from `pkg/acp`
- Process management hooks

Later phases will add:
- Persistent storage (SQLite/PostgreSQL)
- Session reconnection support
- Metrics and observability hooks

## Dependencies

- Standard library only
- No external dependencies for core logic
- `pkg/acp` integration comes in Issue #7

## Files

```
pkg/relay/session/
├── README.md              # This file
├── models.go              # Session, Handle, SessionState
├── state_machine.go       # Pure transition functions
├── store_memory.go        # In-memory Store implementation
├── manager.go             # Public API with DI
├── cleaner.go             # NoOpCleaner for Phase 1
├── state_machine_test.go  # State machine tests
└── manager_test.go        # Manager + integration tests
```

## References

- [SESSION_LIFECYCLE.md](../../../docs/SESSION_LIFECYCLE.md) - State machine spec
- [Issue #6](../../../docs/issues/06-relay-session-management.md) - Implementation brief
- [Issue #7](../../../docs/issues/07-relay-acp-integration.md) - Next steps
