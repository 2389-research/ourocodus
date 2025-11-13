# Context-Aware ACP Cleanup Design

**Date:** 2025-11-13
**Issues:** #222, #212, #210
**Type:** Critical Bug Fixes

## Problem Statement

Three critical bugs prevent reliable cleanup of ACP clients and container sessions:

1. **Issue #222**: `transport.Close()` blocks forever after `Kill()` when a process enters an uninterruptible state
2. **Issue #212**: Goroutine leaks occur when `acpClient.Close()` times out because the timeout wrapper cannot kill the underlying operation
3. **Issue #210**: Concurrent termination calls trigger double-stop races in the session manager, causing duplicate Docker stop commands

These bugs cause resource exhaustion (goroutines, file descriptors), hung shutdowns, and Docker API errors.

## Solution Architecture

### Three-Layer Context-Aware Design

Context flows from session manager through ACP client to transport, giving each layer proper timeout control without goroutine leaks.

**Layer 1: Transport** (`pkg/acp/transport.go`)
- Add `context.Context` parameter to `Close()` method
- Add second timeout after `Kill()` to prevent indefinite blocking
- Return error if process fails to exit within bounds

**Layer 2: ACP Client** (`pkg/acp/client.go`)
- Change `Close()` signature to accept `context.Context`
- Remove goroutine wrapper pattern
- Pass context through to transport cleanup
- Wait for stderr goroutine with context timeout

**Layer 3: Session Manager** (`pkg/relay/session/manager.go`)
- Use atomic take-and-delete pattern to prevent double-stop races
- Pass context to all `Close()` calls
- Eliminate goroutine leaks when cleanup times out

## Implementation Details

### Fix #222: Transport Blocking After Kill

**Current Code:**
```go
case <-time.After(5 * time.Second):
    _ = t.cmd.Process.Kill()
    waitErr = <-done  // BLOCKS FOREVER
```

**Fixed Code:**
```go
case <-time.After(5 * time.Second):
    _ = t.cmd.Process.Kill()

    select {
    case err := <-done:
        waitErr = err
    case <-time.After(2 * time.Second):
        waitErr = fmt.Errorf("process %d did not exit after kill", t.cmd.Process.Pid)
    case <-ctx.Done():
        waitErr = fmt.Errorf("close cancelled by context: %w", ctx.Err())
    }
```

### Fix #212: Goroutine Leaks from Client Close

**Current Code:**
```go
done := make(chan error, 1)
go func() {
    done <- acpClient.Close()  // If Close blocks, goroutine leaks
}()

select {
case err := <-done:
    // Happy path
case <-ctx.Done():
    // Timeout - but goroutine still running!
}
```

**Fixed Code:**
```go
// No goroutine wrapper needed - Close respects context
if err := acpClient.Close(ctx); err != nil {
    m.logger.Printf("Error closing ACP client: %v", err)
}
```

### Fix #210: Double-Stop Race in Session Manager

**Current Code:**
```go
// Read under RLock
m.launchersMu.RLock()
launcher := m.launchers[key]
handle := m.handles[key]
m.launchersMu.RUnlock()

// GAP: Another goroutine can read same launcher/handle here

if launcher != nil && handle != nil {
    launcher.Stop(ctx, handle)  // Both goroutines call Stop()
}

// Delete under Lock (too late!)
m.launchersMu.Lock()
delete(m.launchers, key)
delete(m.handles, key)
m.launchersMu.Unlock()
```

**Fixed Code:**
```go
// Atomic take-and-delete under Lock
m.launchersMu.Lock()
launcher := m.launchers[key]
handle := m.handles[key]
delete(m.launchers, key)
delete(m.handles, key)
m.launchersMu.Unlock()

// Now safe - only one goroutine has these pointers
if launcher != nil && handle != nil {
    launcher.Stop(ctx, handle)
}
```

## API Changes

### Breaking Changes

**Transport Interface:**
```go
// Before
Close() error

// After
Close(ctx context.Context) error
```

**ACP Client:**
```go
// Before
Close() error
CloseWithContext(ctx context.Context) error

// After
Close(ctx context.Context) error
```

### Migration Path

All call sites must pass a context:

```go
// Before
err := acpClient.Close()

// After
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
err := acpClient.Close(ctx)
```

## Testing Strategy

### Unit Tests

**Transport Layer** (`pkg/acp/transport_test.go`):
- `TestHostProcessTransportCloseTimeout` - verify timeout after kill works
- `TestHostProcessTransportCloseContextCancelled` - verify context cancellation
- `TestHostProcessTransportCloseHappyPath` - verify normal close unchanged

**Client Layer** (`pkg/acp/client_test.go`):
- `TestClientCloseNoGoroutineLeak` - verify no leaks using `runtime.NumGoroutine()`
- `TestClientCloseContextTimeout` - verify Close respects deadline
- `TestClientCloseLogStderrCleanup` - verify stderr goroutine cleanup

**Session Manager Layer** (`pkg/relay/session/manager_test.go`):
- `TestConcurrentTerminateAgentNoDoubleStop` - verify atomic take-and-delete
- `TestTerminateUserSessionContextPropagation` - verify context flows to agents
- `TestTerminateAgentWithAcpClientTimeout` - verify no leaks on timeout

### Integration Tests

Run all existing tests with `-race` flag to verify no data races.

## Implementation Order

Bottom-up approach builds each layer on the previous:

1. **Transport Layer** - Add context to Close(), fix blocking after Kill
2. **Client Layer** - Make Close() context-aware, remove wrapper goroutines
3. **Session Manager Layer** - Fix double-stop race, update all call sites
4. **Tests** - Add comprehensive test coverage for all three layers
5. **Verification** - Run full test suite with race detector

## Files Modified

1. `pkg/acp/transport.go` - Context-aware Close()
2. `pkg/acp/client.go` - Context-aware Close(), remove CloseWithContext()
3. `pkg/relay/session/manager.go` - Atomic take-and-delete, update Close() calls
4. `pkg/acp/transport_test.go` - Transport tests
5. `pkg/acp/client_test.go` - Client tests
6. `pkg/relay/session/manager_test.go` - Session manager tests
7. All other files calling `Close()` - Update to pass context (estimate 5-10 files)

## Success Criteria

- No goroutine leaks when Close() times out
- No indefinite blocking after Kill() signal
- No double-stop races in session manager
- All existing tests pass with `-race` flag
- New tests verify fixes under timeout conditions

## Risks and Mitigations

**Risk**: Breaking API changes require updating many call sites
**Mitigation**: Compiler will catch all call sites that need updates

**Risk**: Context timeout values might be too short or too long
**Mitigation**: Use existing timeout values as baseline, adjust based on testing

**Risk**: Tests might not catch all goroutine leaks
**Mitigation**: Use `runtime.NumGoroutine()` and `goleak` package for leak detection
