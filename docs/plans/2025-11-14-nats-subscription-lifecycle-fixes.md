# NATS Subscription Lifecycle Management Fixes

**Date:** 2025-11-14
**Status:** Design
**Issues:** #231, #238
**Milestone:** 10
**Priority:** P2 (Medium)
**Effort:** Low-Medium (1.5 hours total)

## Summary

Fix two related subscription lifecycle issues:
1. **#231** - Subscription Stop() goroutine leak on timeout
2. **#238** - Client doesn't track subscriptions for proper cleanup

## Problems

### Problem 1: Stop() Goroutine Leak (#231)

**File:** `pkg/nats/subscription.go:112-127`
**Severity:** MEDIUM

The `Stop()` method spawns a goroutine to drain the subscription, but if the context times out, the goroutine is abandoned and continues running until drain completes or fails.

**Current Code:**
```go
func (s *Subscription) Stop(ctx context.Context) error {
    drainDone := make(chan error, 1)
    go func() {
        drainDone <- s.natsSub.Drain()  // MAY NEVER COMPLETE
    }()

    select {
    case err := <-drainDone:
        return err
    case <-ctx.Done():
        _ = s.natsSub.Unsubscribe()
        return fmt.Errorf("drain timeout: %w", ctx.Err())
        // GOROUTINE STILL RUNNING, will eventually exit when Drain() completes
    }
}
```

**Issue:** While the goroutine will eventually exit, it's a resource leak during shutdown and poor practice.

### Problem 2: Subscriptions Not Tracked (#238)

**File:** `pkg/nats/client.go:233-257`
**Severity:** MEDIUM

The client creates subscriptions but doesn't track them. `Close()` relies solely on `c.conn.Close()` to clean up subscriptions, which may not give subscriptions a chance to drain gracefully.

**Current Code:**
```go
type client struct {
    conn    *nats.Conn
    // ... no subscription tracking
}

func (c *client) Subscribe(ctx context.Context, subject string, handler MsgHandler, opts ...SubOption) (*Subscription, error) {
    // Creates subscription but doesn't track it
    sub := newSubscription(c, subject, handler, ...)
    if err := sub.start(ctx); err != nil {
        return nil, err
    }
    return sub, nil  // Lost reference
}

func (c *client) Close() error {
    c.conn.Close()  // Abruptly closes all subscriptions
    return nil
}
```

**Issue:** Subscriptions don't get a chance to drain gracefully during shutdown, potentially losing in-flight messages.

## Root Cause

1. **#231**: The goroutine pattern doesn't account for early exit from the select
2. **#238**: No subscription registry, relying entirely on connection close for cleanup

## Proposed Solution

### Fix #231: Wait for Goroutine with Timeout

The goroutine will eventually exit when `Drain()` completes. The current implementation already forces an `Unsubscribe()` on timeout, which will cause `Drain()` to fail quickly. The "leak" is temporary and self-resolving.

**However**, we can improve clarity by documenting this behavior:

```go
func (s *Subscription) Stop(ctx context.Context) error {
    drainDone := make(chan error, 1)
    go func() {
        // This goroutine will exit when Drain() completes/fails
        // If context times out, the Unsubscribe() below causes Drain() to fail quickly
        drainDone <- s.natsSub.Drain()
    }()

    select {
    case err := <-drainDone:
        return err
    case <-ctx.Done():
        // Force unsubscribe to cause Drain() to fail quickly
        _ = s.natsSub.Unsubscribe()
        return fmt.Errorf("drain timeout: %w", ctx.Err())
        // Note: goroutine will exit shortly after Unsubscribe() causes Drain() to fail
    }
}
```

### Fix #238: Add Subscription Tracking

Add a subscription registry to track active subscriptions:

```go
type client struct {
    conn    *nats.Conn
    config  *ClientConfig
    // ... existing fields

    subsMu sync.Mutex
    subs   []*Subscription  // Track active subscriptions
}

func (c *client) Subscribe(ctx context.Context, subject string, handler MsgHandler, opts ...SubOption) (*Subscription, error) {
    // ... create subscription
    sub := newSubscription(c, subject, handler, applySubOptions(opts...))
    if err := sub.start(ctx); err != nil {
        return nil, err
    }

    // Track the subscription
    c.subsMu.Lock()
    c.subs = append(c.subs, sub)
    c.subsMu.Unlock()

    return sub, nil
}

// Queue subscriptions use the same Subscribe() method with WithQueueGroup() option
// Example: client.Subscribe(ctx, subject, handler, nats.WithQueueGroup(queueName))

func (c *client) Close() error {
    // Mark as closed
    c.mu.Lock()
    if c.closed {
        c.mu.Unlock()
        return nil
    }
    c.closed = true
    c.mu.Unlock()

    // Stop all tracked subscriptions with per-subscription timeout
    c.subsMu.Lock()
    subs := append([]*Subscription{}, c.subs...)  // Copy to avoid holding lock during Stop
    c.subsMu.Unlock()

    // Stop each subscription with its own timeout to prevent cascade failures
    for _, sub := range subs {
        // Skip subscriptions that are already stopped
        if !sub.IsValid() {
            continue
        }

        // Best effort - log but don't fail Close()
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        if err := sub.Stop(ctx); err != nil {
            log.Printf("[WARN] Failed to stop subscription: %v", err)
        }
        cancel()
    }

    // Close connection
    return c.conn.Close()
}
```

## Implementation Steps

### Phase 1: Document Goroutine Behavior (#231)
1. Add comment explaining goroutine lifecycle in `Stop()`
2. Clarify that `Unsubscribe()` causes `Drain()` to fail quickly
3. Note that the goroutine is short-lived after timeout

### Phase 2: Add Subscription Tracking (#238)
1. Add `subsMu sync.Mutex` and `subs []*Subscription` fields to `client` struct
2. Update `Subscribe()` to track subscriptions (queue subscriptions use same method with `WithQueueGroup()` option)
3. Add `log` import for warning messages

### Phase 3: Update Close() Method
1. Copy tracked subscriptions (avoid holding lock during Stop)
2. Skip subscriptions that are already stopped (using `IsValid()`)
3. Create per-subscription context with 5s timeout (prevents cascade failures)
4. Iterate and call `Stop()` on each subscription (best effort)
5. Call `cancel()` immediately after each `Stop()` to avoid context leaks
6. Log warnings for failures, don't block Close()

**Design Decision**: Each subscription gets its own 5-second timeout rather than sharing a single timeout across all subscriptions. This prevents one slow subscription from consuming all the timeout budget and causing cascade failures. The trade-off is that total Close() time could be `N * 5 seconds` in the worst case (if all N subscriptions timeout), but this is acceptable for graceful shutdown.

## Testing Strategy

### Unit Tests

**Test 1: Subscription Tracking**
```go
func TestClient_SubscriptionTracking(t *testing.T) {
    client := setupTestClient(t)

    // Subscribe to multiple subjects
    sub1, err := client.Subscribe(ctx, "test.1", handler)
    require.NoError(t, err)

    sub2, err := client.Subscribe(ctx, "test.2", handler, nats.WithQueueGroup("queue"))
    require.NoError(t, err)

    // Verify subscriptions are tracked
    client.subsMu.Lock()
    count := len(client.subs)
    client.subsMu.Unlock()

    assert.Equal(t, 2, count)
}
```

**Test 2: Close Drains Subscriptions**
```go
func TestClient_CloseGracefulDrain(t *testing.T) {
    client := setupTestClient(t)

    received := make(chan bool, 1)
    sub, _ := client.Subscribe("test", func(msg Message) {
        received <- true
    })

    // Publish message
    client.Publish("test", []byte("data"))

    // Give time for message to arrive
    time.Sleep(100 * time.Millisecond)

    // Close should drain subscriptions
    err := client.Close()
    assert.NoError(t, err)

    // Message should have been received during drain
    select {
    case <-received:
        // Success
    case <-time.After(time.Second):
        t.Fatal("Message not received during drain")
    }
}
```

### Race Detector

```bash
go test -race ./pkg/nats/...
```

## Impact

### #231 - Goroutine Leak
- **Before**: Goroutine continues running after timeout (self-resolving but unclear)
- **After**: Documented behavior, clarified that goroutine exits quickly after Unsubscribe()
- **Fix Type**: Documentation improvement (code already handles this correctly)

### #238 - Subscription Tracking
- **Before**: Subscriptions not tracked, no graceful drain on Close()
- **After**: Subscriptions tracked and drained with timeout on Close()
- **Benefit**: More graceful shutdown, better message delivery guarantees

## Risk Assessment

- **Risk Level**: LOW
- **Backward Compatibility**: Full (internal change only)
- **Breaking Changes**: None
- **Performance Impact**: Negligible (small slice operations during Subscribe/Close)

## Success Criteria

- ✅ Subscriptions tracked in client
- ✅ Close() attempts to drain subscriptions with timeout
- ✅ No data races detected by `go test -race`
- ✅ All existing tests pass
- ✅ Graceful shutdown demonstrated in tests

## References

- Issue #231: NATS Subscription Stop() goroutine leak
- Issue #238: NATS Client subscriptions not tracked
- NATS Drain documentation: https://docs.nats.io/using-nats/developer/sending/caches#drain
