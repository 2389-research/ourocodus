# NATS Shutdown Hygiene: Context Propagation and Bounded Drain

**Date:** 2025-11-07
**Milestone:** Milestone 2 - Container Runtime Integration
**Issues:** #94, #95, #96

## Overview

This design addresses three interconnected issues in the NATS subscription lifecycle that affect shutdown reliability and goroutine management. The changes improve container runtime integration by ensuring clean shutdown behavior when agent containers are stopped.

## Problems

### Issue #96: Unused stopCh Field
- `stopCh chan struct{}` is created and closed but never read from
- Dead code that adds complexity without value
- Located in `pkg/nats/subscription.go:22`

### Issue #94: Non-Cancellable Message Handlers
- Handlers receive `context.Background()` instead of cancellable context
- Cannot detect when subscription is stopping
- Leads to goroutine leaks during shutdown
- Located in `pkg/nats/subscription.go:82`

### Issue #95: Unbounded Drain Operation
- `Subscription.Stop()` calls `Drain()` without timeout
- Can hang indefinitely if NATS server is slow/unresponsive
- No way for caller to enforce shutdown deadline
- Located in `pkg/nats/subscription.go:105`

## Goals

1. **Clean shutdown**: Handlers detect cancellation and exit gracefully
2. **Bounded operations**: Stop() respects caller's timeout
3. **No goroutine leaks**: All handlers terminate when subscription stops
4. **Backwards compatible**: Existing handlers continue to work
5. **Simple code**: Remove unused complexity

## Solution

### Approach: Sequential Implementation

Three commits in logical order:
1. **Remove stopCh** - Cleanup dead code
2. **Context propagation** - Enable handler cancellation
3. **Bounded drain** - Add timeout enforcement

### Architecture Changes

#### Current State
```
start(ctx) → [ignores ctx]
  ↓
messageHandler()
  ↓
context.Background() → handler(ctx, msg)
  ↓
Stop(ctx) → Drain() [unbounded]
```

#### Target State
```
start(ctx) → WithCancel(ctx) → stored lifecycle context
  ↓
messageHandler()
  ↓
lifecycle context → handler(ctx, msg)
  ↓
Stop(ctx) → cancel() + Drain() [with timeout]
```

## Detailed Design

### Commit 1: Remove Unused stopCh (#96)

**Changes:**
```go
// pkg/nats/subscription.go

type Subscription struct {
    // ... existing fields ...
-   stopCh chan struct{}  // Line 22 - REMOVE
}

func newSubscription(...) *Subscription {
    return &Subscription{
        // ... existing fields ...
-       stopCh: make(chan struct{}),  // Line 33 - REMOVE
    }
}

func (s *Subscription) Stop(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.closed {
        return ErrSubscriptionClosed
    }

    s.closed = true
-   close(s.stopCh)  // Line 101 - REMOVE

    // ... rest of Stop() ...
}
```

**Rationale:**
- `stopCh` is never read from (no `<-s.stopCh` anywhere)
- Only operations: create in constructor, close in Stop()
- Classic dead code - remove it

**Testing:**
- All existing tests should pass
- No new tests needed - proves it's unused

**Risk:** None - purely dead code removal

---

### Commit 2: Propagate Cancellable Context (#94)

**Changes:**
```go
// pkg/nats/subscription.go

type Subscription struct {
    client  *client
    subject string
    handler MsgHandler
    opts    *subOptions

    natsSub *nats.Subscription

+   ctx    context.Context      // Lifecycle context
+   cancel context.CancelFunc   // Cancellation function

    mu     sync.RWMutex
    closed bool
}

func (s *Subscription) start(ctx context.Context) error {
+   // Create cancellable context for subscription lifetime
+   s.ctx, s.cancel = context.WithCancel(ctx)

    s.mu.Lock()
    defer s.mu.Unlock()

    // ... existing start logic ...
}

func (s *Subscription) messageHandler(msg *nats.Msg) {
    start := time.Now()

    wrappedMsg := wrapNatsMessage(msg, s.client.config.CorrelationHeader)

-   // Create context for handler
-   ctx := context.Background()
+   // Use subscription lifecycle context
+   ctx := s.ctx

    // Call user handler
    err := s.handler(ctx, wrappedMsg)

    s.client.metrics.recordMessageReceived(s.subject, time.Since(start), err)
}

func (s *Subscription) Stop(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.closed {
        return ErrSubscriptionClosed
    }

    s.closed = true
+
+   // Cancel subscription context to signal handlers
+   if s.cancel != nil {
+       s.cancel()
+   }

    if s.natsSub != nil {
        if err := s.natsSub.Drain(); err != nil {
            return fmt.Errorf("drain subscription: %w", err)
        }
    }

    return nil
}
```

**Rationale:**
- Handlers receive cancellable context from subscription lifecycle
- When Stop() is called, context is cancelled
- Handlers can detect cancellation via `ctx.Done()` or `ctx.Err()`
- Enables graceful handler shutdown

**Handler compatibility:**
- Handlers that ignore context: Continue to work (no change)
- Handlers that respect context: Can now detect cancellation
- Fully backwards compatible

**Testing:**
```go
TestSubscription_HandlerCancellation(t *testing.T) {
    // Handler blocks on channel
    blocked := make(chan struct{})
    handler := func(ctx context.Context, msg Message) error {
        <-blocked // Block until cancelled
        return ctx.Err()
    }

    // Start subscription
    sub := client.Subscribe("test", handler)

    // Trigger message (handler blocks)
    // ...

    // Cancel parent context
    cancel()

    // Assert handler exits
    // Assert ctx.Err() == context.Canceled
}

TestSubscription_StopCancelsHandlers(t *testing.T) {
    // Multiple in-flight messages
    // Call Stop()
    // Assert all handlers receive cancelled context
}
```

**Risk:** Low
- Additive change - doesn't break existing code
- Handlers that ignore context are unaffected

---

### Commit 3: Add Context Deadline to Drain (#95)

**Changes:**
```go
// pkg/nats/subscription.go

func (s *Subscription) Stop(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.closed {
        return ErrSubscriptionClosed
    }

    s.closed = true

    // Cancel subscription context to signal handlers
    if s.cancel != nil {
        s.cancel()
    }

    if s.natsSub != nil {
-       // Unsubscribe and drain
-       if err := s.natsSub.Drain(); err != nil {
-           return fmt.Errorf("drain subscription: %w", err)
-       }
+       // Drain with context deadline
+       drainDone := make(chan error, 1)
+       go func() {
+           drainDone <- s.natsSub.Drain()
+       }()
+
+       select {
+       case err := <-drainDone:
+           if err != nil {
+               return fmt.Errorf("drain subscription: %w", err)
+           }
+       case <-ctx.Done():
+           // Drain exceeded deadline
+           return fmt.Errorf("drain timeout: %w", ctx.Err())
+       }
    }

    return nil
}
```

**Rationale:**
- Stop() now respects the caller's context deadline
- Drain operation is bounded by timeout
- Prevents indefinite hangs during shutdown
- Caller can enforce shutdown deadline

**Usage pattern:**
```go
// Graceful shutdown with 5s timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := sub.Stop(ctx); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // Force cleanup if needed
    }
}
```

**Testing:**
```go
TestSubscription_StopWithTimeout(t *testing.T) {
    // Mock NATS subscription with slow Drain()
    slowDrain := &mockNatsSub{
        DrainFunc: func() error {
            time.Sleep(10 * time.Second)
            return nil
        },
    }

    // Call Stop with 1s timeout
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    start := time.Now()
    err := sub.Stop(ctx)
    elapsed := time.Since(start)

    // Assert Stop returned within timeout
    assert.True(t, elapsed < 2*time.Second)
    assert.ErrorIs(t, err, context.DeadlineExceeded)
}

TestSubscription_StopDrainSuccess(t *testing.T) {
    // Mock NATS subscription with fast Drain()
    fastDrain := &mockNatsSub{
        DrainFunc: func() error {
            return nil
        },
    }

    // Call Stop with generous timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := sub.Stop(ctx)

    // Assert success
    assert.NoError(t, err)
}
```

**Risk:** Low
- Existing callers pass context (already required by signature)
- Behavior change: Stop() can now return context.DeadlineExceeded
- Improvement: Prevents hangs, makes shutdown predictable

## Implementation Plan

### Phase 1: Commit 1 - Remove stopCh
1. Remove field from struct
2. Remove initialization
3. Remove close() call
4. Run tests (all should pass)

### Phase 2: Commit 2 - Context Propagation
1. Add ctx and cancel fields to struct
2. Update start() to create cancellable context
3. Update messageHandler to use stored context
4. Update Stop() to call cancel()
5. Add handler cancellation tests
6. Run full test suite

### Phase 3: Commit 3 - Bounded Drain
1. Wrap Drain() in goroutine with channel
2. Add select with context deadline
3. Handle timeout error
4. Add timeout tests
5. Run full test suite

### Phase 4: Integration Validation
1. Run all pkg/nats tests
2. Run relay integration tests
3. Manual test: Start subscription, send messages, call Stop()
4. Verify clean shutdown in logs

## Testing Strategy

### Unit Tests
- `TestSubscription_RemoveStopCh`: Verify removal doesn't break anything (implicit - existing tests)
- `TestSubscription_HandlerCancellation`: Handler detects context cancellation
- `TestSubscription_StopCancelsHandlers`: Stop() cancels in-flight handlers
- `TestSubscription_StopWithTimeout`: Stop() respects context deadline
- `TestSubscription_StopDrainSuccess`: Drain completes before timeout

### Integration Tests
- Existing pkg/nats test suite (regression check)
- Relay NATS integration tests (if they exist)

### Manual Testing
```bash
# Run NATS server
nats-server

# Run example with subscription
go run examples/nats-basic/main.go

# Trigger shutdown (Ctrl+C)
# Verify clean exit in logs
```

## Success Criteria

- [ ] All three commits pass tests independently
- [ ] No goroutine leaks detected (use `goleak`)
- [ ] Stop() returns within timeout even with slow drain
- [ ] Handlers can detect cancellation via context
- [ ] Existing tests pass (backwards compatibility)
- [ ] CI green on all platforms

## Risks and Mitigations

### Risk: Handler expects context.Background() semantics
**Likelihood:** Low
**Impact:** Low
**Mitigation:** Context cancellation is additive - handlers that ignore context are unaffected

### Risk: Timeout too short for production drain
**Likelihood:** Medium
**Impact:** Low
**Mitigation:** Caller controls timeout, can adjust based on environment

### Risk: Drain() goroutine leak if timeout occurs
**Likelihood:** Low
**Impact:** Low
**Mitigation:** Buffered channel prevents goroutine block; NATS library should eventually complete

## Future Enhancements

1. **Drain timeout configuration**: Add subscription option for default drain timeout
2. **Graceful handler shutdown**: Add handler pre-shutdown callback
3. **Metrics**: Track drain duration and timeout occurrences
4. **Force-close option**: Add ForceStop() that skips drain

## Related Work

- Issue #93: Prometheus metrics normalization (separate PR)
- Issue #97: NATS error wrapper nil checks (separate PR)
- Milestone 2: Container Runtime Integration

## References

- Go context patterns: https://go.dev/blog/context
- NATS drain semantics: https://docs.nats.io/using-nats/developer/connecting/draining
- Goroutine lifecycle: https://go.dev/doc/effective_go#goroutines
