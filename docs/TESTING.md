# Testing Strategy

## Current Coverage

### Unit Tests (Excellent)

- `pkg/relay/session/` — State machine, manager orchestration, and store behavior have full coverage.
- `pkg/relay/message.go` — Message validation paths are exercised.
- `pkg/acp/client.go` — Core client lifecycle, request/response handling, and logger integration covered with mocks.

### Smoke Testing

- `scripts/smoke-test.sh` — Launches the relay, drives a WebSocket session, and verifies:
  1. Handshake payload is returned.
  2. Echo responses include timestamps.
  3. Recoverable validation errors stay connected.
  4. Non-recoverable errors close the connection.
  5. Bonus chaos: configurable fuzzing (default 100 cases) now mixes oversized payloads, malformed JSON/UTF-8, duplicate-key echoes, binary frames, and type violations.
  6. Flags:
     - Via `smoke-test.sh`: `--fuzz N` (fuzz count), `--verbose` (debug output)
     - Direct Go program: `go run scripts/smoketest/relay -fuzz N -max-payload BYTES -seed VALUE`
  7. Any fuzz discrepancies are logged (⚠️) and summarized at the end instead of aborting mid-run.

## Testing Patterns

### Time Dependency Injection with Clock Interface

The session layer uses dependency injection for time operations to enable deterministic testing of time-sensitive code.

**Pattern:**

- `Clock` interface provides abstract time operations
- Manager receives Clock via constructor injection
- Returns `time.Time` directly (no adapter layer needed)
- Prevents direct `time.Now()` calls that would make tests non-deterministic

**Implementation:**

```go
// In pkg/relay/session/manager.go
type Clock interface {
    Now() time.Time
}

// Manager uses injected clock
type Manager struct {
    clock Clock
    // ... other fields
}

func NewManager(store Store, idGen IDGenerator, clock Clock, ...) *Manager {
    return &Manager{
        clock: clock,
        // ...
    }
}

// Usage in manager methods
now := m.clock.Now()
session := NewUserSession(sessionID, ws, now)
```

**Benefits:**

1. **Deterministic Tests** - Control exact timestamps in unit tests with mock clocks
2. **Race Condition Prevention** - No TOCTOU issues from multiple time.Now() calls
3. **Time Travel** - Fast-forward time in tests without waiting
4. **Simple Interface** - Direct time.Time return, no conversions needed

**Usage in Tests:**

```go
// Create a mock clock
mockClock := &MockClock{currentTime: time.Unix(1000, 0)}

// Inject into manager
manager := NewManager(store, idGen, mockClock, ...)

// Control time in tests
mockClock.SetTime(time.Unix(2000, 0)) // Set to specific time
```

**Important:** Always use the injected clock (`m.clock.Now()`) instead of calling `time.Now()` directly in session-related code.

## Integration Test Gaps (Future Work)

### Gap 1: WebSocket Server Integration

- File: `pkg/relay/server.go`
- Missing: End-to-end WebSocket client → server → echo verification.
- Needed: Real WebSocket handshake test covering `connection:established`, validation failures, and echo loop.
- Issue: #XX (to be created)

### Gap 2: ACP Process Integration

- File: `pkg/acp/client.go`
- Missing: Tests with real `claude-code-acp` process to validate JSON-RPC flow.
- Needed: Spawn process → send message → receive response (happy path + failure modes).
- Issue: #XX (to be created)

### Gap 3: Session Lifecycle Integration

- Files: `pkg/relay/server.go`, `pkg/relay/session/manager.go`
- Missing: Full lifecycle from HTTP upgrade → session creation → ACP spawn → cleanup.
- Needed: Run a full session including message relay, termination, and cleanup hooks.
- Issue: #XX (to be created)

## Phase 2 Test Strategy

Plan to add integration tests covering:

1. WebSocket connection handling.
2. Real ACP process communication (success and failure paths).
3. Full session lifecycle orchestration.
4. Error scenarios (process crash, disconnect, spawn failure, cleanup failure).
