# NATS Pending Limits Configuration Design

**Date:** 2025-11-03
**Issue:** #91 - Make NATS pending limits configurable
**Status:** Approved

## Problem

The `SetPendingLimits(-1, -1)` call in `pkg/nats/subscription.go:59` disables all buffering limits for NATS subscriptions. This allows unbounded memory growth when subscribers cannot keep up with message rates, creating a classic "slow consumer" problem.

From NATS documentation:
> "Any negative value means that the given metric is not limited. However, be careful - buffered messages can grow without bound if limits are disabled."

## Solution

Make pending limits configurable through the existing functional options pattern, defaulting to NATS-recommended safe values.

## Design

### NATS Default Values

NATS recommends these default pending limits (proven safe in production):
- **Messages**: 512 * 1024 = 524,288 messages
- **Bytes**: 64 * 1024 * 1024 = 67,108,864 bytes (64 MB)

These defaults protect against slow consumers while allowing reasonable message bursts.

### Data Structure Changes

Add two fields to `subOptions` in `pkg/nats/options.go`:

```go
type subOptions struct {
    queueGroup        string
    maxInflight       int
    pendingLimitMsgs  int  // Default: 512*1024 (NATS recommended)
    pendingLimitBytes int  // Default: 64*1024*1024 (NATS recommended)
}
```

Update defaults:

```go
func defaultSubOptions() *subOptions {
    return &subOptions{
        queueGroup:        "", // Empty = no queue group
        maxInflight:       1,
        pendingLimitMsgs:  512 * 1024,      // 524,288 messages
        pendingLimitBytes: 64 * 1024 * 1024, // 67,108,864 bytes (64 MB)
    }
}
```

### Functional Options API

Add two new option constructors in `pkg/nats/options.go`:

```go
// WithPendingLimits sets custom pending message and byte limits for subscriptions.
//
// The pending limits control how many messages and bytes can be buffered by the
// NATS client when the subscriber cannot keep up with the message rate. When either
// limit is exceeded, the subscription will be considered a "slow consumer" and may
// be dropped by the server.
//
// Use -1 for either parameter to disable that specific limit (not recommended).
//
// Default values (if not specified):
//   - Messages: 524,288 (512 * 1024)
//   - Bytes: 67,108,864 (64 MB)
//
// Example:
//   // High-throughput subscription with 1M message buffer and 128MB byte buffer
//   sub, err := client.Subscribe(ctx, "orders", handler,
//       nats.WithPendingLimits(1_000_000, 128*1024*1024))
func WithPendingLimits(msgs, bytes int) SubOption {
    return func(opts *subOptions) {
        opts.pendingLimitMsgs = msgs
        opts.pendingLimitBytes = bytes
    }
}

// WithUnlimitedPending disables all pending limits for the subscription.
//
// WARNING: This allows unbounded memory growth if the subscriber cannot keep up
// with the message rate. Only use this if you have external backpressure mechanisms
// in place (e.g., bounded channels, rate limiting, or guaranteed fast processing).
//
// This is equivalent to calling WithPendingLimits(-1, -1).
//
// Example:
//   // Only use when you control message rate externally
//   sub, err := client.Subscribe(ctx, "logs", handler,
//       nats.WithUnlimitedPending())
func WithUnlimitedPending() SubOption {
    return func(opts *subOptions) {
        opts.pendingLimitMsgs = -1
        opts.pendingLimitBytes = -1
    }
}
```

### Usage Examples

**Safe defaults (recommended):**
```go
sub, err := client.Subscribe(ctx, "subject", handler)
// Uses: 524,288 messages, 64MB bytes
```

**Custom limits for high-throughput:**
```go
sub, err := client.Subscribe(ctx, "orders", handler,
    nats.WithPendingLimits(1_000_000, 128*1024*1024))
// Uses: 1M messages, 128MB bytes
```

**Disable limits (dangerous - explicit opt-in):**
```go
sub, err := client.Subscribe(ctx, "logs", handler,
    nats.WithUnlimitedPending())
// Uses: unlimited (careful!)
```

### Implementation Changes

**Update `pkg/nats/subscription.go:59`:**

```go
// Before:
if err := s.natsSub.SetPendingLimits(-1, -1); err != nil {

// After:
// Set pending limits from options (defaults: 512K msgs, 64MB bytes)
if err := s.natsSub.SetPendingLimits(s.opts.pendingLimitMsgs, s.opts.pendingLimitBytes); err != nil {
```

Complete implementation in context:

```go
func (s *Subscription) start(_ context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.closed {
        return ErrSubscriptionClosed
    }

    // Create NATS subscription
    var err error
    if s.opts.queueGroup != "" {
        s.natsSub, err = s.client.conn.QueueSubscribe(s.subject, s.opts.queueGroup, s.messageHandler)
    } else {
        s.natsSub, err = s.client.conn.Subscribe(s.subject, s.messageHandler)
    }

    if err != nil {
        return fmt.Errorf("subscribe to %q: %w", s.subject, err)
    }

    // Set pending limits from options (defaults: 512K msgs, 64MB bytes)
    if err := s.natsSub.SetPendingLimits(s.opts.pendingLimitMsgs, s.opts.pendingLimitBytes); err != nil {
        _ = s.natsSub.Unsubscribe()
        return fmt.Errorf("set pending limits: %w", err)
    }

    return nil
}
```

## Testing Strategy

Add unit tests to `pkg/nats/options_test.go`:

```go
func TestDefaultSubOptions_PendingLimits(t *testing.T) {
    opts := defaultSubOptions()

    // Verify NATS default values
    assert.Equal(t, 512*1024, opts.pendingLimitMsgs, "default message limit should be NATS default")
    assert.Equal(t, 64*1024*1024, opts.pendingLimitBytes, "default byte limit should be NATS default")
}

func TestWithPendingLimits(t *testing.T) {
    opts := defaultSubOptions()

    // Apply custom limits
    WithPendingLimits(1000, 5*1024*1024)(opts)

    assert.Equal(t, 1000, opts.pendingLimitMsgs)
    assert.Equal(t, 5*1024*1024, opts.pendingLimitBytes)
}

func TestWithUnlimitedPending(t *testing.T) {
    opts := defaultSubOptions()

    // Apply unlimited
    WithUnlimitedPending()(opts)

    assert.Equal(t, -1, opts.pendingLimitMsgs, "unlimited should use -1")
    assert.Equal(t, -1, opts.pendingLimitBytes, "unlimited should use -1")
}

func TestWithPendingLimits_NegativeValues(t *testing.T) {
    opts := defaultSubOptions()

    // Can explicitly set -1 via WithPendingLimits
    WithPendingLimits(-1, -1)(opts)

    assert.Equal(t, -1, opts.pendingLimitMsgs)
    assert.Equal(t, -1, opts.pendingLimitBytes)
}
```

**Test coverage:**
- ✅ Default values match NATS recommendations
- ✅ Custom limits via `WithPendingLimits()`
- ✅ Unlimited via `WithUnlimitedPending()`
- ✅ Explicit -1 values work

## Backward Compatibility

**Breaking Change:** This changes the default behavior from unlimited buffering to limited buffering.

**Rationale:**
- The current unlimited default is unsafe and can cause production issues
- NATS documentation explicitly warns against unlimited buffering
- Safe defaults are more important than backward compatibility
- Users who need unlimited buffering must explicitly opt-in with `WithUnlimitedPending()`

**Migration path:**
Existing code that relies on unlimited buffering must add:
```go
sub, err := client.Subscribe(ctx, subject, handler, nats.WithUnlimitedPending())
```

## Impact

**Security/Stability:**
- Prevents unbounded memory growth in production
- Protects against slow consumer problems
- Fast failure detection (better than silent memory exhaustion)

**Performance:**
- No performance impact for fast subscribers
- Slow subscribers will now fail fast instead of consuming all memory
- Encourages proper backpressure handling in application code

**Operations:**
- Defaults work for 99% of use cases
- Clear error messages when limits are hit
- Easy to tune for specific high-throughput scenarios

## Implementation Files

- `pkg/nats/options.go` - Add fields, defaults, and option constructors
- `pkg/nats/subscription.go` - Use configured limits instead of -1, -1
- `pkg/nats/options_test.go` - Unit tests for new functionality
