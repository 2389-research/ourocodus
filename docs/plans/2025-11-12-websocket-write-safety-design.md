# WebSocket Write Safety Design

**Date**: 2025-11-12
**Status**: Validated via Zen Consensus
**Related Issues**: #213 (CRITICAL), #214 (CRITICAL), #215 (HIGH)
**Milestone**: 10

## Executive Summary

This design addresses concurrent write race conditions in the WebSocket server (`pkg/relay/server.go`) by introducing a mutex-based synchronization wrapper. The Gorilla WebSocket library requires external synchronization for concurrent writers, which the current implementation lacks, leading to data races and potential frame corruption.

**Scope**: This document covers Issue #213 (Fix concurrent write race in WebSocket server). Issues #214 (session cleanup) and #215 (DoS hardening) will be addressed in subsequent PRs.

## Problem Statement

### Issue #213: Concurrent Write Race in WebSocket Server

**Severity**: CRITICAL
**Current Behavior**: Multiple goroutines can write to the WebSocket connection concurrently without coordination:
- Relay goroutine writes messages from NATS
- Session manager goroutine writes control frames
- Potential ping/pong handler writes

**Root Cause**: Gorilla WebSocket's `*websocket.Conn` is **not safe for concurrent writers**. The library documentation explicitly states:

> "Connections support one concurrent reader and one concurrent writer. Applications are responsible for ensuring that no more than one goroutine calls the write methods (NextWriter, SetWriteDeadline, WriteMessage, WriteJSON, EnableWriteCompression, SetCompressionLevel) concurrently."

**Impact**:
- Data races detected by Go race detector
- Corrupted WebSocket frames
- Potential connection drops
- Undefined behavior under load

**Evidence**: Go race detector reports concurrent writes to the same connection from multiple goroutines.

## Design Decision: Mutex-Wrapped Adapter

### Approach Selected

After evaluating two approaches (mutex wrapper vs. write pump pattern), we selected the **Mutex-Wrapped Adapter** approach for the following reasons:

1. **Simplicity**: Single mutex, clear synchronization point
2. **Maintainability**: Easy to understand and audit
3. **Low Risk**: Minimal changes to existing code structure
4. **Sufficient Performance**: Mutex overhead negligible for typical WebSocket traffic

### Rejected Alternative

**Write Pump Pattern**: A dedicated goroutine with a message queue was considered but rejected because:
- Higher complexity (goroutine lifecycle, queue management)
- More failure modes (queue full, goroutine crashes)
- Unnecessary for current load patterns
- Can be adopted later if profiling shows mutex contention

## Architecture

### Component: SafeWebSocketAdapter

Replace the current `SessionWebSocketAdapter` with a new `SafeWebSocketAdapter` that adds mutex protection to **all** write operations.

```go
// SafeWebSocketAdapter wraps a WebSocket connection with mutex protection
// for concurrent write safety. Gorilla WebSocket requires external
// synchronization for write operations.
type SafeWebSocketAdapter struct {
    conn    WebSocketConn
    writeMu sync.Mutex
}

func NewSafeWebSocketAdapter(conn WebSocketConn) *SafeWebSocketAdapter {
    return &SafeWebSocketAdapter{conn: conn}
}
```

### Critical Finding: Complete Method Coverage Required

**Zen Consensus Validation** (gpt-5-codex: 8/10, gpt-5: 8/10) identified a critical gap in the initial design:

**Original Design** (INCOMPLETE):
- Protected only: `WriteJSON`, `Close`

**Validated Design** (COMPLETE):
- Must protect ALL write methods:
  1. `WriteJSON(v interface{}) error`
  2. `WriteMessage(messageType int, data []byte) error`
  3. `WriteControl(messageType int, data []byte, deadline time.Time) error`
  4. `SetWriteDeadline(t time.Time) error`
  5. `Close() error`

**Rationale**: Any write-related method that touches the underlying connection must be synchronized. Missing even one method leaves a race condition window.

### Implementation

```go
// WriteJSON sends a JSON-encoded message.
// Thread-safe: serializes with other write operations.
func (a *SafeWebSocketAdapter) WriteJSON(v interface{}) error {
    a.writeMu.Lock()
    defer a.writeMu.Unlock()
    return a.conn.WriteJSON(v)
}

// WriteMessage sends a binary or text message.
// Thread-safe: serializes with other write operations.
func (a *SafeWebSocketAdapter) WriteMessage(messageType int, data []byte) error {
    a.writeMu.Lock()
    defer a.writeMu.Unlock()
    return a.conn.WriteMessage(messageType, data)
}

// WriteControl sends a control frame (ping, pong, close).
// Thread-safe: serializes with other write operations.
// Note: Has built-in write deadline parameter.
func (a *SafeWebSocketAdapter) WriteControl(messageType int, data []byte, deadline time.Time) error {
    a.writeMu.Lock()
    defer a.writeMu.Unlock()
    return a.conn.WriteControl(messageType, data, deadline)
}

// SetWriteDeadline sets the write deadline for the connection.
// Thread-safe: serializes with other write operations.
func (a *SafeWebSocketAdapter) SetWriteDeadline(t time.Time) error {
    a.writeMu.Lock()
    defer a.writeMu.Unlock()
    return a.conn.SetWriteDeadline(t)
}

// Close closes the WebSocket connection.
// Thread-safe: serializes with other write operations.
func (a *SafeWebSocketAdapter) Close() error {
    a.writeMu.Lock()
    defer a.writeMu.Unlock()
    return a.conn.Close()
}

// Read-only methods (no mutex required)
func (a *SafeWebSocketAdapter) ReadJSON(v interface{}) error {
    return a.conn.ReadJSON(v)
}

func (a *SafeWebSocketAdapter) SetReadDeadline(t time.Time) error {
    return a.conn.SetReadDeadline(t)
}

func (a *SafeWebSocketAdapter) SetPongHandler(h func(appData string) error) {
    a.conn.SetPongHandler(h)
}
```

## Special Concern: Ping/Pong Handlers

**Zen Consensus** raised an important question: **Do ping/pong handlers bypass the adapter?**

### Investigation Required

Before implementation, we must verify:

1. **Are ping/pong handlers configured in the current code?**
   - Check for `SetPingHandler()` or `SetPongHandler()` calls
   - Check if default handlers are active

2. **If configured, do they write directly to the connection?**
   - Default pong handler writes control frames
   - Custom handlers might write responses
   - These writes could bypass our mutex

3. **If they bypass the adapter, how do we protect them?**
   - Option A: Wrap handler functions to acquire mutex before writing
   - Option B: Disable automatic pong responses, handle manually
   - Option C: Document as unsafe pattern, remove handlers

### Default Behavior

Gorilla WebSocket's default ping/pong behavior:
- **Default ping handler**: Sends pong response automatically (uses `WriteControl`)
- **Default pong handler**: No-op
- **Risk**: If we don't pass adapter reference to handlers, they may hold raw connection reference

### Resolution Strategy

1. Audit current code for handler configuration
2. If handlers are used, ensure they use the adapter (not raw conn)
3. Add tests to verify handler writes are synchronized
4. Document handler usage requirements

## Testing Strategy

### Unit Tests

```go
func TestSafeWebSocketAdapter_ConcurrentWrites(t *testing.T) {
    // Test: Multiple goroutines writing simultaneously
    // Verify: No data races (run with -race flag)
    // Verify: All messages sent successfully
}

func TestSafeWebSocketAdapter_WriteJSON_Concurrent(t *testing.T) {
    // Test: Concurrent WriteJSON calls
    // Verify: Messages serialized correctly
}

func TestSafeWebSocketAdapter_WriteMessage_Concurrent(t *testing.T) {
    // Test: Concurrent WriteMessage calls
    // Verify: Frames not corrupted
}

func TestSafeWebSocketAdapter_WriteControl_Concurrent(t *testing.T) {
    // Test: Concurrent control frame writes
    // Verify: Control frames serialized
}

func TestSafeWebSocketAdapter_Mixed_Concurrent(t *testing.T) {
    // Test: Mix of WriteJSON, WriteMessage, WriteControl
    // Verify: All write types properly serialized
}

func TestSafeWebSocketAdapter_PingPong_NoBypass(t *testing.T) {
    // Test: Ping/pong handlers don't bypass mutex
    // Verify: Handler writes go through adapter
}
```

### Race Detection

```bash
# Must pass with race detector enabled
go test -race ./pkg/relay/...
```

### Integration Tests

- Send high-frequency messages from multiple NATS subjects
- Trigger session lifecycle events during active relay
- Monitor for connection drops or frame corruption
- Verify message ordering and completeness

## Error Handling

### Mutex Lock Failures

Mutexes don't fail under normal operation, but we must consider:

1. **Deadlock Prevention**: Use `defer unlock()` pattern consistently
2. **Panic Recovery**: Ensure defer runs even if underlying write panics
3. **Lock Visibility**: Document that all write methods must acquire mutex

### Connection Errors

SafeWebSocketAdapter propagates all connection errors unchanged:
- Network errors surface immediately
- Protocol violations bubble up
- No error swallowing or transformation

### Usage Requirements

**Documentation**: Add godoc comments explaining:
1. Adapter is required for concurrent write safety
2. All goroutines must use adapter (not raw conn)
3. Handlers must be configured with adapter reference

## Implementation Phases

### Phase 1: Investigation

**Goal**: Understand current write patterns and handler usage

**Tasks**:
1. Audit `pkg/relay/server.go` for all WebSocket write call sites
2. Check for ping/pong handler configuration
3. Verify no code holds direct reference to raw `*websocket.Conn`
4. List all WebSocket methods currently used in the codebase
5. Document findings in PR description

**Acceptance Criteria**:
- Complete list of write call sites
- Handler configuration status known
- No raw connection references outside adapter

### Phase 2: Implementation

**Goal**: Replace SessionWebSocketAdapter with SafeWebSocketAdapter

**Tasks**:
1. Create `SafeWebSocketAdapter` struct with mutex
2. Implement all required write methods (WriteJSON, WriteMessage, WriteControl, SetWriteDeadline, Close)
3. Implement pass-through read methods (ReadJSON, SetReadDeadline, SetPongHandler)
4. Replace `SessionWebSocketAdapter` with `SafeWebSocketAdapter` in server code
5. If handlers exist, ensure they use adapter reference
6. Add godoc comments documenting thread-safety guarantees

**Acceptance Criteria**:
- All write methods protected by mutex
- No direct access to raw connection
- Handlers (if any) use adapter
- Code compiles without errors

### Phase 3: Testing

**Goal**: Verify concurrent write safety

**Tasks**:
1. Add concurrent write unit tests
2. Add ping/pong handler tests (if handlers used)
3. Run full test suite with `-race` flag
4. Perform manual integration testing with high message volume
5. Verify no race conditions reported
6. Verify no connection drops or frame corruption

**Acceptance Criteria**:
- All unit tests pass
- `go test -race ./pkg/relay/...` passes
- Integration tests show stable connections
- No race conditions detected

## Rollout Strategy

### Three Sequential PRs

1. **PR #1: Fix Concurrent Write Race** (Issue #213)
   - This design document
   - SafeWebSocketAdapter implementation
   - Comprehensive tests
   - **Merge Requirement**: All tests pass with `-race` flag

2. **PR #2: Add Session Cleanup** (Issue #214)
   - Depends on: PR #1 merged
   - Adds cleanup logic on WebSocket disconnect
   - Uses thread-safe adapter from PR #1

3. **PR #3: DoS Hardening** (Issue #215)
   - Depends on: PR #1 merged (PR #2 optional)
   - Rate limiting, connection limits, message size limits
   - Builds on safe write foundation

### Risk Mitigation

- **Small PRs**: Each PR addresses one issue, easy to review
- **Sequential Merge**: No merge conflicts, clear dependencies
- **Incremental Testing**: Validate each fix independently
- **Rollback Safety**: Each PR can be reverted independently

## Success Criteria

### Functional Requirements

- [ ] No data races detected by Go race detector
- [ ] All WebSocket write methods properly synchronized
- [ ] Ping/pong handlers (if any) do not bypass synchronization
- [ ] Connection stability under concurrent load
- [ ] No frame corruption or protocol violations

### Non-Functional Requirements

- [ ] Performance: Mutex overhead < 1% of message latency
- [ ] Maintainability: Code is clear and well-documented
- [ ] Testability: Comprehensive unit and integration tests
- [ ] Documentation: Usage requirements clearly stated

## Open Questions

1. **Are ping/pong handlers currently configured?**
   - To be answered in Phase 1 investigation

2. **What is the typical message frequency?**
   - Informs whether mutex contention is a concern
   - To be measured during integration testing

3. **Are there other write paths we haven't identified?**
   - To be confirmed during code audit

## References

- **Gorilla WebSocket Documentation**: https://pkg.go.dev/github.com/gorilla/websocket
- **Go Race Detector**: https://go.dev/doc/articles/race_detector
- **Issue #213**: Fix concurrent write race in WebSocket server
- **Issue #214**: Add session cleanup on WebSocket disconnect
- **Issue #215**: Harden WebSocket server against DoS attacks
- **Milestone 10**: Current milestone containing these issues

## Validation

This design was validated using Zen Consensus with two models:

- **gpt-5-codex**: 8/10 confidence, confirmed approach sound, identified missing methods
- **gpt-5**: 8/10 confidence, confirmed approach sound, raised ping/pong handler concern

**Key Feedback**:
- Must protect ALL write methods, not just WriteJSON and Close
- Must investigate ping/pong handlers to ensure they don't bypass mutex
- Mutex approach is technically sound for this use case
- Sequential PR strategy reduces risk effectively

## Conclusion

The SafeWebSocketAdapter design provides a simple, maintainable solution to WebSocket concurrent write race conditions. By protecting all write methods with a mutex and carefully investigating ping/pong handler behavior, we ensure thread-safe WebSocket operations without introducing complex goroutine coordination.

The three-phase implementation approach (investigation, implementation, testing) and three-PR rollout strategy minimize risk while delivering incremental value.
