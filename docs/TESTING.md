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
  5. Bonus chaos: 100 fuzzed payloads exercise echo/validation paths.

## Integration Test Gaps (Future Work)

**Gap 1: WebSocket Server Integration**
- File: `pkg/relay/server.go`
- Missing: End-to-end WebSocket client → server → echo verification.
- Needed: Real WebSocket handshake test covering `connection:established`, validation failures, and echo loop.
- Issue: #XX (to be created)

**Gap 2: ACP Process Integration**
- File: `pkg/acp/client.go`
- Missing: Tests with real `claude-code-acp` process to validate JSON-RPC flow.
- Needed: Spawn process → send message → receive response (happy path + failure modes).
- Issue: #XX (to be created)

**Gap 3: Session Lifecycle Integration**
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
