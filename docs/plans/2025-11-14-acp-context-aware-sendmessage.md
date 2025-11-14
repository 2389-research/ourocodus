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
    // 1. Operation Lock - Serializes entire SendMessage calls
    opMu      sync.Mutex

    // 2. Write Lock - Protects ID increment and Write only
    writeMu   sync.Mutex    // Renamed from reqMu
    nextID    int

    // 3. Closed Flag Lock
    closedMu  sync.RWMutex
    closed    bool

    // Response demultiplexing
    pendingMu sync.Mutex
    pending   map[int]chan responseResult
    inFlight  sync.WaitGroup
    done      chan struct{}
}

type responseResult struct {
    msg *AgentMessage
    err error
}
```

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

**SendMessage:** opMu → pendingMu (brief) → writeMu (brief) → wait
**readLoop:** pendingMu only
**Close:** Never takes opMu; briefly takes writeMu

This ordering prevents deadlocks because:
- Close never competes for opMu
- readLoop only touches pendingMu
- No circular dependencies

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
            select { case ch <- rr: default: }  // Non-blocking
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

    // 3. Prevent new writes
    c.writeMu.Lock()
    c.writeMu.Unlock()

    // 4. Wake waiting operations
    select { case <-c.done: default: close(c.done) }

    // 5. Wait for in-flight (bounded)
    drained := make(chan struct{})
    go func() { c.inFlight.Wait(); close(drained) }()
    select {
    case <-drained:
    case <-ctx.Done():
        c.logger.Printf("[WARN] in-flight not drained before deadline")
    }

    // 6. Close transport
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
1. Update all `SendMessage()` calls to pass context
2. Update test mocks
3. Update integration tests

### Phase 6: Testing
1. Add new unit tests
2. Run with race detector
3. Verify leak tests pass

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
