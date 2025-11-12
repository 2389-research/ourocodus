# Jitter Randomization Fix Design

**Date:** 2025-11-03
**Issue:** #92 - Implement proper jitter randomization in exponential backoff
**Status:** Approved

## Problem

The `randomFloat()` function in `pkg/nats/options.go:397` returns a hardcoded `0.5`, defeating the purpose of jitter in exponential backoff. All retry attempts use identical delays, causing thundering herd problems when multiple clients reconnect simultaneously.

## Solution

Inject a random source into the `exponentialBackoff` struct through an interface.

## Design

### Interface Definition

Define a `RandomSource` interface with a single method:

```go
// RandomSource provides random number generation for jitter calculations.
type RandomSource interface {
    Float64() float64
}
```

### Default Implementation

Use `math/rand/v2` for the default implementation:

```go
// defaultRandomSource uses the global math/rand/v2 random source.
type defaultRandomSource struct{}

func (defaultRandomSource) Float64() float64 {
    return rand.Float64()
}
```

The `math/rand/v2` package provides thread-safe per-goroutine random generators in Go 1.22+. No mutex or seeding required.

### Test Implementation

Provide a fixed random source for deterministic testing:

```go
// fixedRandomSource always returns the same value (for testing).
type fixedRandomSource struct {
    value float64
}

func (f fixedRandomSource) Float64() float64 {
    return f.value
}
```

### Struct Modification

Add the random source to `exponentialBackoff`:

```go
type exponentialBackoff struct {
    initial time.Duration
    max     time.Duration
    rand    RandomSource  // Injected random source
}
```

### Constructor Update

Modify `newExponentialBackoff` to accept an optional random source:

```go
func newExponentialBackoff(initial, max time.Duration, rand RandomSource) BackoffStrategy {
    if rand == nil {
        rand = defaultRandomSource{}
    }
    return &exponentialBackoff{
        initial: initial,
        max:     max,
        rand:    rand,
    }
}
```

### Next() Method

Update the jitter calculation to use the injected source:

```go
func (e *exponentialBackoff) Next(attempt int) time.Duration {
    // ... existing exponential calculation ...

    // Add jitter: 0-25% of backoff
    jitter := time.Duration(float64(backoff) * 0.25 * (0.5 + (0.5 * e.rand.Float64())))

    return backoff + jitter
}
```

### Cleanup

Remove the `randomFloat()` function entirely (lines 395-399).

## API Changes

Update `defaultClientConfig()` at line 76:

```go
// Before
RetryBackoff: newExponentialBackoff(200*time.Millisecond, 5*time.Second),

// After
RetryBackoff: newExponentialBackoff(200*time.Millisecond, 5*time.Second, nil),
```

The `nil` parameter uses the default random source.

## Testing Strategy

### Deterministic Test

Verify jitter calculation with fixed random values:

```go
func TestExponentialBackoff_DeterministicJitter(t *testing.T) {
    backoff := newExponentialBackoff(100*time.Millisecond, 5*time.Second, fixedRandomSource{0.5})
    duration := backoff.Next(1)
    // Verify exact duration with known random value
}
```

### Randomness Test

Verify jitter varies across multiple attempts:

```go
func TestExponentialBackoff_RandomJitter(t *testing.T) {
    backoff := newExponentialBackoff(100*time.Millisecond, 5*time.Second, nil)

    durations := make(map[time.Duration]bool)
    for i := 0; i < 100; i++ {
        backoff.Reset()
        durations[backoff.Next(1)] = true
    }

    assert.Greater(t, len(durations), 10, "jitter should produce varied delays")
}
```

### Bounds Test

Verify jitter stays within 0-25% range:

```go
func TestExponentialBackoff_JitterBounds(t *testing.T) {
    // Test with extreme random values: 0.0, 0.5, 1.0
    // Verify bounds hold for all values
}
```

## Impact

This fix prevents thundering herd problems by distributing reconnection attempts across time. Testing becomes deterministic, making CI/CD more reliable.

## Implementation Files

- `pkg/nats/options.go` - Interface, implementations, struct modifications
- `pkg/nats/options_test.go` - New test cases
