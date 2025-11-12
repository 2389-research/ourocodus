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

### ACP Container Execution Testing

The ACP launcher selection and container execution system uses a three-tier testing approach to ensure reliability across different execution modes.

#### Testing Tiers

**1. Unit Tests (Fast, No Dependencies)**

Location: `pkg/relay/session/client_factory_test.go`

These tests validate individual functions in isolation with no external dependencies:

```bash
# Run unit tests for launcher selection
go test ./pkg/relay/session -run "TestGetRuntimeMode|TestValidateContainerPrerequisites|TestCreateHostLauncher|TestCreateContainerLauncher|TestSelectLauncher"
```

**Coverage:**
- `getRuntimeMode()` - Environment variable parsing and validation (4 test cases)
- `validateContainerPrerequisites()` - Prerequisite validation logic (4 test cases)
- `createHostLauncher()` - Host launcher factory (2 test cases)
- `createContainerLauncher()` - Container launcher factory (2 test cases)
- `selectLauncher()` - Integration of all components (6 test cases)

**Benefits:**
- Fast execution (milliseconds)
- No Docker required
- Deterministic results
- Easy to debug

**2. Integration Tests (Medium Speed, Mocked Dependencies)**

Location: `pkg/relay/session/client_factory_test.go`

These tests validate the full `NewClient()` flow with mocked container managers:

```bash
# Run integration tests for ACP client creation
go test ./pkg/relay/session -run "TestNewClient_Integration"
```

**Coverage:**
- Host mode client creation (success path)
- Container mode validation failures (missing prerequisites, container ID, manager)
- Runtime context validation (nil runtime, empty workspace)

**Benefits:**
- Validates full flow from client factory to launcher selection
- Tests error propagation through the stack
- No Docker required
- Runs in normal test suite

**3. Smoke Tests (Slow, Real Docker Required)**

Location: `tests/e2e/acp_container_exec_test.go`

These tests validate actual Docker container execution with real containers:

```bash
# Run smoke tests (requires Docker)
go test -tags=integration ./tests/e2e -run "TestContainerExecProcessLauncher"
```

**Coverage:**
- `TestContainerExecProcessLauncher_SmokeTest` - Basic command execution inside containers
- `TestContainerExecProcessLauncher_WithEchoAgent` - Echo-agent binary execution in containers

**Requirements:**
- Docker daemon running (Docker Desktop or Colima)
- Alpine image pullable from Docker Hub
- For echo-agent test: Built echo-agent binary at `./bin/echo-agent`

**Benefits:**
- Validates real Docker integration
- Catches Docker API compatibility issues
- Verifies workspace mounting
- Tests actual command execution

#### Running Tests by Category

```bash
# Fast: Unit tests only (no Docker)
go test ./pkg/relay/session -run "Test(GetRuntimeMode|ValidateContainerPrerequisites|CreateHostLauncher|CreateContainerLauncher|SelectLauncher)"

# Medium: Unit + Integration tests (no Docker)
go test ./pkg/relay/session

# Slow: All tests including smoke tests (requires Docker)
go test -tags=integration ./tests/e2e

# All ACP-related tests
make test  # Runs unit + integration (no Docker)
make test-integration  # Runs smoke tests (requires Docker)
```

#### Test Coverage Summary

| Component | Unit Tests | Integration Tests | Smoke Tests | Total Coverage |
|-----------|-----------|------------------|-------------|----------------|
| `getRuntimeMode()` | 4 | - | - | 100% |
| `validateContainerPrerequisites()` | 4 | - | - | 100% |
| `createHostLauncher()` | 2 | - | - | 100% |
| `createContainerLauncher()` | 2 | - | - | 100% |
| `selectLauncher()` | 6 | - | - | 83.4% |
| `NewClient()` | - | 5 | - | Key paths |
| Container exec | - | - | 2 | Real Docker |

#### Docker Environment Setup

The smoke tests automatically detect your Docker environment:

**Colima (preferred):**
```bash
# Start Colima
colima start

# Tests will automatically detect socket at:
# ~/.colima/default/docker.sock
```

**Docker Desktop:**
```bash
# Tests will use default socket at:
# /var/run/docker.sock
```

#### CI/CD Integration

**Standard CI (Unit + Integration):**
```yaml
- name: Run tests
  run: make test
```

**Extended CI (Include Smoke Tests):**
```yaml
- name: Setup Docker
  uses: docker/setup-buildx-action@v2

- name: Run integration tests
  run: go test -tags=integration ./tests/e2e
```

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
