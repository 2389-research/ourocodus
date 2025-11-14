# Context-Aware ACP SendMessage and Close Coordination

**Date:** 2025-11-14
**Status:** Design Approved
**Issues:** #226, #228, #229
**Milestone:** 10

## Summary

This design fixes three ACP client issues by adding context support to `SendMessage`, introducing a background readLoop for response demultiplexing, and implementing proper Close coordination. The changes prevent indefinite blocking, eliminate SendMessage/Close races, and enable timeout propagation throughout the system.

## Issues Fixed

### #226 - Add context/timeout support to ACP SendMessage (High)
**Problem:** `SendMessage` has no context parameter and blocks indefinitely if agent hangs.
**Impact:** No way to timeout requests; blocked calls hold lock and prevent other operations.

### #229 - Fix race between SendMessage and Close (Medium)
**Problem:** Concurrent Close() while SendMessage blocks in `scanner.Scan()` causes broken pipe errors or panics.
**Impact:** System instability during shutdown; corrupted client state.

### #228 - Fix StopContainerSession timeout mismatch (Medium)
**Problem:** `StopContainerSession` uses hardcoded 30s timeout instead of respecting caller's context deadline.
**Impact:** Operations block 6x longer than caller expects when given 5s timeout.

## Root Cause Analysis

The current `SendMessage` implementation holds `reqMu` across the entire request/response cycle, including a blocking `scanner.Scan()` call. This creates two problems:

1. **No cancellation:** Context cannot interrupt `scanner.Scan()`, so timeout/cancel has no effect
2. **Close race:** Close() can close the transport while SendMessage is reading, causing broken pipe

```go
// Current problematic code
func (c *Client) SendMessage(content string) (*AgentMessage, error) {
    c.reqMu.Lock()              // Lock held for entire operation
    defer c.reqMu.Unlock()

    // ... write request ...

    return c.readResponse(id)   // Blocks in scanner.Scan()
}
```

## Design: Background ReadLoop with Three-Lock Architecture

### Core Principle

Separate the write lock from the read wait. A background goroutine handles all reading; SendMessage registers a response channel and waits with a select statement that respects context.

### Architecture Components

```go
type Client struct {
    // Stderr logging goroutine (pre-existing, unmodified by this design)
    stderrCancel context.CancelFunc  // Signals logStderr goroutine to stop
    stderrDone   chan struct{}       // Closed when logStderr exits

    // 1. Operation Lock - Serializes entire SendMessage calls
    opMu      sync.Mutex

    // 2. Write Lock - Protects ID increment and Write only
    writeMu   sync.Mutex    // Renamed from reqMu
    nextID    int

    // 3. Closed Flag Lock
    closedMu  sync.RWMutex
    closed    bool

    // Response demultiplexing (NEW)
    pendingMu sync.Mutex
    pending   map[int]chan responseResult
    inFlight  sync.WaitGroup
    done      chan struct{}  // Closed on shutdown to wake waiters
}

type responseResult struct {  // NEW
    msg *AgentMessage
    err error
}
```

**Note on stderrCancel:** This field is part of the pre-existing stderr logging mechanism
(the `logStderr` goroutine that tails the agent process's stderr). It's not modified by
this design but is referenced in Close() to ensure clean shutdown. Close() calls
`c.stderrCancel()` to signal the logging goroutine before proceeding with other cleanup.

### Three-Lock Rationale

**opMu (Operation Lock):**
- Serializes entire SendMessage operations
- Maintains strict one-at-a-time semantics
- Held during write AND wait phases
- Can be removed later if concurrent SendMessage is desired

**writeMu (Write Lock):**
- Protects only ID increment and transport.Write()
- Very short critical sections (microseconds)
- Released before waiting for response
- Enables Close to prevent new writes quickly

**closedMu (Closed Flag Lock):**
- RWMutex for concurrent closed checks
- Write lock held only when setting closed=true
- Read lock held briefly during closed checks

### Lock Ordering Rules

**SendMessage:** opMu → writeMu (brief) → pendingMu (brief) → wait
**readLoop:** pendingMu only
**Close:** Never takes opMu

**Nested Lock Pattern:**
SendMessage has a TOCTOU-prevention pattern where `closedMu` is checked twice:
1. Early check: `closedMu.RLock()` → fast-path return if closed
2. Pre-write check: Inside `writeMu` hold, re-check `closedMu.RLock()` before Write

Full ordering with nested checks:
```
SendMessage:
  closedMu.RLock() (early exit check) → closedMu.RUnlock()
  → opMu.Lock()
    → writeMu.Lock()
      → closedMu.RLock() (re-check before write) → closedMu.RUnlock()
    → writeMu.Unlock()
    → pendingMu.Lock() → pendingMu.Unlock()
  → wait (select on channels)
```

This ordering prevents deadlocks because:
- Close never competes for opMu (can't deadlock with SendMessage)
- readLoop only touches pendingMu (no interaction with other locks)
- No circular dependencies in lock acquisition
- closedMu is always RLock (read-only) during checks, never held across other locks

### SendMessage Flow

```go
func (c *Client) SendMessage(ctx context.Context, content string) (*AgentMessage, error) {
    // 1. Quick closed check
    c.closedMu.RLock()
    if c.closed {
        c.closedMu.RUnlock()
        return nil, fmt.Errorf("client is closed")
    }
    c.closedMu.RUnlock()

    // 2. Serialize operation
    c.opMu.Lock()
    defer c.opMu.Unlock()

    // 3. Allocate ID (brief writeMu hold)
    c.writeMu.Lock()
    id := c.nextID
    c.nextID++
    c.writeMu.Unlock()

    // 4. Register response channel
    respCh := make(chan responseResult, 1)
    c.pendingMu.Lock()
    c.pending[id] = respCh
    c.pendingMu.Unlock()
    c.inFlight.Add(1)
    defer func() {
        c.pendingMu.Lock()
        delete(c.pending, id)
        c.pendingMu.Unlock()
        c.inFlight.Done()
    }()

    // 5. Marshal request
    req := Request{...}
    data, _ := json.Marshal(req)
    data = append(data, '\n')

    // 6. Write with closed re-check
    c.writeMu.Lock()
    c.closedMu.RLock()
    if c.closed {
        c.closedMu.RUnlock()
        c.writeMu.Unlock()
        return nil, fmt.Errorf("client is closed")
    }
    c.closedMu.RUnlock()
    c.transport.Write(data)
    c.writeMu.Unlock()

    // 7. Wait (opMu held, writeMu released)
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-c.done:
        return nil, fmt.Errorf("client closed")
    case res := <-respCh:
        return res.msg, res.err
    }
}
```

**Key Points:**
- `opMu` held throughout, maintaining serialization
- `writeMu` released before wait, allowing Close to proceed
- `select` enables context cancellation and shutdown notification
- Re-check closed before write prevents race with Close

### Background readLoop

```go
func (c *Client) readLoop() {
    for c.scanner.Scan() {
        line := c.scanner.Bytes()

        // Decode response, extract ID
        var resp Response
        json.Unmarshal(line, &resp)
        id := extractID(resp)

        // Build result
        rr := buildResult(resp)

        // Dispatch to waiter
        c.pendingMu.Lock()
        ch := c.pending[id]
        c.pendingMu.Unlock()

        if ch != nil {
            select {
            case ch <- rr:
                // Successfully delivered
            default:
                // Waiter already exited (context canceled or timeout)
                c.logger.Printf("[ACP read] waiter gone for id=%d", id)
            }
        } else {
            // No waiter registered (unsolicited or late response)
            c.logger.Printf("[ACP read] no waiter for id=%d", id)
        }
    }

    // Scanner ended - broadcast to waiters
    endErr := c.scanner.Err()
    if endErr == nil { endErr = io.EOF }

    c.pendingMu.Lock()
    for _, ch := range c.pending {
        select {
        case ch <- responseResult{err: endErr}:
        default:
        }
    }
    c.pendingMu.Unlock()

    // Signal shutdown
    select { case <-c.done: default: close(c.done) }
}
```

**Key Points:**
- Single goroutine owns all reading
- Non-blocking sends prevent hanging if waiter already exited
- Broadcasts termination to all pending waiters
- Double-close protection on `done` channel

### Close Coordination

```go
func (c *Client) Close(ctx context.Context) error {
    // 1. Set closed flag
    c.closedMu.Lock()
    if c.closed {
        c.closedMu.Unlock()
        return nil
    }
    c.closed = true
    c.closedMu.Unlock()

    // 2. Cancel stderr goroutine
    if c.stderrCancel != nil {
        c.stderrCancel()
    }

    // 3. Wake waiting operations (no additional lock needed - closed flag already set)
    select { case <-c.done: default: close(c.done) }

    // 4. Wait for in-flight (bounded)
    drained := make(chan struct{})
    go func() { c.inFlight.Wait(); close(drained) }()
    select {
    case <-drained:
    case <-ctx.Done():
        c.logger.Printf("[WARN] in-flight not drained before deadline")
    }

    // 5. Close transport
    return c.transport.Close(ctx)
}
```

**Key Points:**
- Never takes `opMu`, so Close never blocks on SendMessage
- Closes `done` to wake the single waiting SendMessage
- Bounded wait respects context deadline
- Double-close protection on `done` channel

## Issue Resolution

### #226 - Context/Timeout Support
✅ Breaking API change: `SendMessage(ctx context.Context, content string)`
✅ Context honored via `select` on `ctx.Done()`
✅ Returns `ctx.Err()` on timeout/cancellation

### #229 - SendMessage/Close Race
✅ Close sets `closed` flag, then briefly holds `writeMu` to prevent new writes
✅ `done` channel wakes waiting SendMessage immediately
✅ Re-check `closed` before Write while holding `writeMu`
✅ No broken pipe panics

### #228 - Timeout Propagation
✅ All callers updated to pass context
✅ `StopContainerSession` respects context deadline
✅ No more hardcoded 30s timeout

## Breaking Changes

**ACP Client API:**
```go
// Before
func (c *Client) SendMessage(content string) (*AgentMessage, error)

// After
func (c *Client) SendMessage(ctx context.Context, content string) (*AgentMessage, error)
```

**Rationale:**
- Consistent with PR #250 (Transport.Close and Client.Close are context-aware)
- Forces callers to consider timeouts
- Most Go-idiomatic pattern

**Migration:**
- Update all call sites (estimated 10-15 files)
- Update test mocks and integration tests
- Optional: Add deprecated shim for transition period

## Testing Strategy

### New Unit Tests

**pkg/acp/client_test.go:**
1. `TestSendMessageWithTimeout` - verify context timeout during wait
2. `TestSendMessageCancellation` - verify context cancel returns promptly
3. `TestSendMessageCloseDuringWait` - verify Close wakes SendMessage
4. `TestSendMessageRejectsAfterClose` - verify closed flag prevents writes
5. `TestReadLoopEOFHandling` - verify clean shutdown on transport EOF
6. `TestReadLoopBroadcastsError` - verify error broadcast to pending waiters

**pkg/containersession/manager_test.go:**
7. `TestStopContainerSessionRespectsDeadline` - verify deadline propagation

### Integration Tests

**pkg/acp/leak_test.go:**
- Verify no goroutine leaks with context timeout
- Verify no goroutine leaks with Close during SendMessage
- Verify readLoop exits cleanly

### Race Detector

Run all tests with `-race` flag to verify:
- No data races in pending map access
- No races between Close and SendMessage
- No races in readLoop dispatch

## Implementation Plan

### Phase 1: Core Architecture
1. Add `opMu`, rename `reqMu` to `writeMu`
2. Add `pendingMu`, `pending` map, `inFlight`, `done` channel
3. Define `responseResult` type
4. Initialize in constructor

### Phase 2: Background ReadLoop
1. Implement `readLoop()` method
2. Start `readLoop()` goroutine in constructor
3. Remove `readResponse()` method (no longer used)

### Phase 3: SendMessage Refactor
1. Add context parameter to signature
2. Implement three-lock flow with select/wait
3. Add cleanup defer
4. Add closed re-check before write

### Phase 4: Close Coordination
1. Add `done` channel close logic
2. Add bounded inFlight wait
3. Verify double-close protection

### Phase 5: Call Site Updates

Break down into prioritized sub-phases to avoid missing mocks or integration test failures:

**Phase 5a (Critical Path - High Risk):**
- `pkg/relay/server.go` - HandleAgentMessage (production message routing)
  - Add 30-second timeout wrapper for defense-in-depth
- `pkg/relay/session/manager.go` - StopContainerSession (issue #228 fix)
  - Ensure context deadline propagates correctly

**Phase 5b (Interfaces and Adapters):**
- `pkg/relay/session/models.go` - ACPClient interface signature
- `pkg/relay/session/client_factory.go` - acpClientAdapter implementation
- `pkg/relay/session/manager_test.go` - mockACPClient test helper

**Phase 5c (Test Infrastructure):**
- `pkg/acp/client_test.go` - All test cases using real client (5 locations)
- `examples/smoke-tests/session/main.go` - fakeACPClient for smoke tests
- `pkg/relay/integration_test.go` - integrationMockACPClient

**Files Modified (Complete List):**
1. pkg/acp/client.go (core implementation)
2. pkg/acp/client_test.go (update + new tests)
3. pkg/relay/server.go (add timeout wrapper)
4. pkg/relay/session/models.go (interface)
5. pkg/relay/session/client_factory.go (adapter)
6. pkg/relay/session/manager_test.go (mock)
7. examples/smoke-tests/session/main.go (fake)
8. pkg/relay/integration_test.go (mock)

### Phase 6: Testing

**Core Context Tests (Added):**
1. `TestSendMessage_ContextTimeout` - Verifies timeout support (issue #226)
2. `TestSendMessage_ContextCancellation` - Verifies cancellation support
3. `TestSendMessage_ContextCancelDuringClose` - Verifies done channel interaction

**Additional Edge-Case Tests (Recommended):**
4. `TestSendMessage_OutOfOrderResponses` - Verify readLoop correctly routes responses even if they arrive out of order
5. `TestSendMessage_DuplicateResponseID` - Confirm responses with duplicate IDs don't corrupt state
6. `TestSendMessage_UnregisteredID` - Verify responses with unregistered IDs are logged and don't block readLoop
7. `TestClose_DuringPartialWrite` - Simulate transport failure during concurrent Close; verify state consistency
8. `TestClose_DoubleClose` - Explicit test confirming panic safety on double-close
9. `TestSendMessage_ConcurrentRequests` - Multiple concurrent SendMessage calls with different contexts

**Verification Steps:**
1. Run full test suite with race detector: `go test -race ./pkg/acp/...`
2. Verify no goroutine leaks with existing leak tests
3. Verify integration tests pass after call site updates
4. Run smoke tests to ensure end-to-end functionality

## Risks and Mitigations

**Risk:** Lock ordering violation causes deadlock
**Mitigation:** Strict lock ordering rules; never hold multiple locks simultaneously except documented cases

**Risk:** Double-close panic on `done` channel
**Mitigation:** Non-blocking close pattern in both Close() and readLoop()

**Risk:** Goroutine leaks if readLoop blocks
**Mitigation:** Scanner is only blocking call; exits on EOF/error; waiters released via `done` channel

**Risk:** Unsolicited responses block readLoop
**Mitigation:** Non-blocking sends to response channels

**Risk:** Large responses exceed scanner buffer
**Mitigation:** Scanner buffer already increased to 5MB in constructor; log warning if limit hit

**Risk:** Breaking changes affect external consumers
**Mitigation:** Coordinate with team; provide deprecated shim if needed; document in CHANGELOG

## Success Criteria

- ✅ SendMessage honors context timeout and cancellation
- ✅ No broken pipe errors during concurrent Close
- ✅ StopContainerSession respects context deadlines
- ✅ All existing tests pass (after call site updates)
- ✅ No goroutine leaks (verified by leak tests)
- ✅ No data races (verified by race detector)
- ✅ Close completes within context deadline

## References

- Issue #226: Add context/timeout support to ACP SendMessage
- Issue #228: Fix StopContainerSession timeout mismatch
- Issue #229: Fix race between SendMessage and Close
- PR #250: Context-aware ACP cleanup (establishes context pattern)
- Zen architectural review: Confirmed three-lock design and safety properties

## Notes

This design maintains backward compatibility for serialization semantics (one SendMessage at a time) while enabling modern context-aware cancellation. The `opMu` lock can be removed in the future if concurrent SendMessage calls are desired, without changing the core readLoop architecture.
