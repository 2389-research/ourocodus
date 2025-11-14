# NATS Client JS() Method Fixes

**Date:** 2025-11-14
**Status:** Design
**Issues:** #236, #237
**Milestone:** 10
**Priority:** P2 (Medium)
**Effort:** Low (20 minutes total)

## Summary

Fix two related issues in the NATS client `JS()` method:
1. **#236** - JS() can initialize JetStream after Close() is called
2. **#237** - jsErr read without lock (potential data race)

## Problems

### Problem 1: JS() Ignores Closed State (#236)

**File:** `pkg/nats/client.go:316-335`
**Severity:** MEDIUM

The `JS()` method can initialize JetStream after `Close()` has been called, which is inconsistent with other client methods that check the closed state.

**Current Code:**
```go
func (c *client) JS() (JSClient, error) {
    c.jsOnce.Do(func() {
        c.jsMu.Lock()
        defer c.jsMu.Unlock()

        js, err := c.conn.JetStream()
        if err != nil {
            c.jsErr = fmt.Errorf("create jetstream context: %w", err)
            c.health.recordError(c.jsErr)
            return
        }

        c.js = newJSClient(c, js)
    })

    return c.js, c.jsErr  // No closed-state check
}
```

**Issue:** If `JS()` is called after `Close()`, it will still initialize JetStream and return a client that may attempt to use a closed connection.

### Problem 2: jsErr Data Race (#237)

**File:** `pkg/nats/client.go:334`
**Severity:** MEDIUM (low probability)

The `jsErr` field is read at line 335 without holding any lock, after the `sync.Once` has completed. While `sync.Once` provides memory barriers for the initialization, subsequent reads without synchronization could theoretically race with other operations.

**Additionally:** The `jsMu` lock inside the `sync.Once.Do()` is redundant - `sync.Once` already provides the necessary synchronization.

## Root Cause

1. **#236**: Missing closed-state validation before JetStream initialization
2. **#237**: Redundant lock (`jsMu`) and lack of protection for return value reads

## Proposed Solution

### Fix #236: Add Closed-State Check

Add a closed-state check before the `sync.Once.Do()`:

```go
func (c *client) JS() (JSClient, error) {
    // Check closed state before initialization
    c.mu.RLock()
    closed := c.closed
    c.mu.RUnlock()

    if closed {
        return nil, ErrClientClosed
    }

    c.jsOnce.Do(func() {
        // ... initialization
    })

    return c.js, c.jsErr
}
```

### Fix #237: Remove Redundant Lock

Remove the `jsMu` lock from inside the `sync.Once.Do()` since `sync.Once` already provides memory barriers:

```go
func (c *client) JS() (JSClient, error) {
    c.mu.RLock()
    closed := c.closed
    c.mu.RUnlock()

    if closed {
        return nil, ErrClientClosed
    }

    c.jsOnce.Do(func() {
        // Create JetStream context (no jsMu lock needed)
        js, err := c.conn.JetStream()
        if err != nil {
            c.jsErr = fmt.Errorf("create jetstream context: %w", err)
            c.health.recordError(c.jsErr)
            return
        }

        c.js = newJSClient(c, js)
    })

    return c.js, c.jsErr
}
```

**Note:** We can also remove the `jsMu` field entirely from the struct if it's not used elsewhere.

## Implementation Steps

### Phase 1: Add Closed-State Check
1. Add `c.mu.RLock()` / `c.mu.RUnlock()` to read `closed` field
2. Check if `closed` is true, return `ErrClientClosed` if so
3. Place check before `c.jsOnce.Do()`

### Phase 2: Remove Redundant Lock
1. Remove `c.jsMu.Lock()` and `c.jsMu.Unlock()` from inside `sync.Once.Do()`
2. Verify `jsMu` is not used elsewhere in the file
3. If unused, remove `jsMu sync.Mutex` field from struct definition

## Testing Strategy

### Unit Tests

Existing tests should continue to pass. Key scenarios to verify:

1. **Test: JS() after Close()** - Verify `ErrClientClosed` is returned
2. **Test: Concurrent JS() calls** - Verify only one initialization occurs
3. **Test: Race detector** - Verify no data races with `-race` flag

### Race Detector

```bash
go test -race ./pkg/nats/...
```

## Impact

### #236 - Closed-State Check
- **Before**: JS() could initialize after Close(), leading to use of closed connection
- **After**: JS() returns `ErrClientClosed` if called after Close()
- **Consistency**: Matches behavior of other client methods

### #237 - Remove Redundant Lock
- **Before**: Redundant lock inside sync.Once, potential race on return
- **After**: Relies on sync.Once memory barriers, cleaner code
- **Performance**: Negligible improvement (one less lock acquisition)

## Risk Assessment

- **Risk Level**: LOW
- **Backward Compatibility**: Full (API unchanged, behavior improved)
- **Breaking Changes**: None
- **Performance Impact**: Negligible improvement

## Success Criteria

- ✅ JS() returns `ErrClientClosed` when called after Close()
- ✅ No data races detected by `go test -race`
- ✅ All existing tests pass
- ✅ No performance regression

## References

- Issue #236: NATS Client JS() ignores closed state
- Issue #237: NATS Client jsErr data race
- Go sync.Once documentation: https://pkg.go.dev/sync#Once
