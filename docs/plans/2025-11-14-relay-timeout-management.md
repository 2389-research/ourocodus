# Relay Timeout Management Improvements

**Date:** 2025-11-14
**Status:** Design
**Issues:** #220, #218
**Milestone:** 10
**Priority:** P1 (High)
**Effort:** Medium (2-3 hours total)

## Summary

Fix timeout management issues in the relay service:
1. **#220** - Implement separate shutdown timeouts for each subsystem
2. **#218** - Add startup timeout bounds for Docker operations

## Problems

### Problem 1: Shared Shutdown Context (#220)

**File:** `cmd/relay/main.go:129-171`
**Severity:** MEDIUM

Single 10-second context shared across all shutdown phases:

```go
// Current problematic code
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Phase 1: HTTP shutdown (may use 2s)
httpServer.Shutdown(shutdownCtx)

// Phase 2: Session termination (may use remaining 8s)
for _, id := range sessionIDs {
    sessionManager.TerminateUserSession(shutdownCtx, id)  // Context may be exhausted!
}

// Phase 3: NATS drain (may never run)
natsConn.Drain()  // Context already exceeded
```

**Issues:**
- Sequential operations share time budget
- Later phases timeout before executing
- Misleading error messages (which phase failed?)
- Unpredictable behavior based on session count

### Problem 2: Unbounded Docker Operations (#218)

**File:** `cmd/relay/main.go:209-211, 301-349`
**Severity:** HIGH

Docker operations use `context.Background()` with no timeouts:

```go
// Startup: Docker ping can hang forever
if _, err := dockerClient.Ping(ctx); err != nil {
    log.Fatalf("[DOCKER] Failed to ping Docker daemon: %v", err)
}

// Cleanup: Container operations can hang
containers, err := dockerClient.ContainerList(ctx, ...)  // No timeout
dockerClient.ContainerStop(ctx, ...)                      // No timeout
dockerClient.ContainerRemove(ctx, ...)                    // No timeout
```

**Issues:**
- Startup can hang indefinitely if Docker daemon slow/unreachable
- Kubernetes liveness probe kills hung pod
- Manual intervention required
- Entire deployment blocked by one slow Docker daemon

## Root Cause

Both issues stem from improper context/timeout management:
- #220: Sharing context across sequential operations
- #218: Not using timeouts for external I/O operations

## Proposed Solution

### Fix #220: Per-Subsystem Shutdown Timeouts

Replace shared context with dedicated contexts for each shutdown phase:

```go
// Phase 1: HTTP shutdown (10s)
httpCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
if err := httpServer.Shutdown(httpCtx); err != nil {
    log.Printf("[SHUTDOWN] HTTP server shutdown error: %v", err)
}
cancel()

// Phase 2: Session termination (2min total, allows graceful per-session cleanup)
sessionsCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
for _, id := range sessionIDs {
    if err := sessionManager.TerminateUserSession(sessionsCtx, id); err != nil {
        log.Printf("[SHUTDOWN] Failed to terminate session %s: %v", id, err)
    }
}
cancel()

// Phase 3: NATS drain (10s)
if natsConn != nil {
    natsCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    if err := natsConn.Drain(natsCtx); err != nil {
        log.Printf("[SHUTDOWN] NATS drain error: %v", err)
    }
    cancel()
}
```

### Fix #218: Add Docker Operation Timeouts

Add per-operation timeouts for all Docker calls:

**1. Docker Ping (startup):**
```go
pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

if _, err := dockerClient.Ping(pingCtx); err != nil {
    log.Fatalf("[DOCKER] Failed to ping Docker daemon (timeout: 5s): %v", err)
}
```

**2. Container Cleanup:**
```go
func cleanupOrphanedContainers(ctx context.Context, client *client.Client) error {
    // List: 10s timeout
    listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    containers, err := client.ContainerList(listCtx, ...)
    if err != nil {
        return fmt.Errorf("list containers: %w", err)
    }

    for _, container := range containers {
        // Stop: 30s timeout (allows graceful shutdown)
        stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
        if err := client.ContainerStop(stopCtx, container.ID, container.StopOptions{Timeout: intPtr(10)}); err != nil {
            log.Printf("[CLEANUP] Failed to stop container %s: %v", container.ID, err)
        }
        cancel()

        // Remove: 10s timeout
        removeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
        if err := client.ContainerRemove(removeCtx, container.ID, ...); err != nil {
            log.Printf("[CLEANUP] Failed to remove container %s: %v", container.ID, err)
        }
        cancel()
    }

    return nil
}
```

## Implementation Steps

### Phase 1: Add Docker Operation Timeouts (#218)
1. Add timeout to Docker Ping at startup (line ~209-211)
2. Update `cleanupOrphanedContainers` to add timeouts:
   - ContainerList: 10s timeout
   - ContainerStop: 30s timeout (with 10s stop signal timeout)
   - ContainerRemove: 10s timeout
3. Update error messages to include timeout values
4. Add helper function `intPtr` if needed

### Phase 2: Refactor Shutdown Timeouts (#220)
1. Replace shared shutdown context with per-phase contexts
2. HTTP shutdown: 10s timeout
3. Session termination: 2min timeout
4. NATS drain: 10s timeout (use context-aware Drain)
5. Add phase-specific logging
6. Ensure each phase's cancel() is called

### Phase 3: Testing
1. Run existing tests to ensure no regressions
2. Manual testing:
   - Start relay with slow Docker daemon
   - Shutdown relay with active sessions
   - Verify timeout behavior

## Testing Strategy

### Manual Testing

**Test 1: Docker Ping Timeout**
```bash
# Simulate slow Docker daemon (block Docker socket)
# Expected: Relay fails startup after 5s with clear timeout message
```

**Test 2: Shutdown Phase Timeouts**
```bash
# Start relay with active sessions
# Send SIGTERM
# Expected: Each phase gets dedicated timeout, clear logging per phase
```

**Test 3: Container Cleanup Timeout**
```bash
# Create orphaned containers
# Start relay
# Expected: Cleanup operations timeout appropriately, don't hang
```

### Integration Tests

Existing tests should continue to pass:
```bash
go test ./cmd/relay/...
```

## Recommended Timeouts

| Operation | Timeout | Rationale |
|-----------|---------|-----------|
| **Docker Ping** | 5s | Quick health check at startup |
| **Container List** | 10s | Local operation, should be fast |
| **Container Stop** | 30s | Allow graceful container shutdown (10s signal timeout) |
| **Container Remove** | 10s | Force remove if needed |
| **HTTP Shutdown** | 10s | Stop accepting new connections, close idle |
| **Session Termination** | 2min | Allow ACP client Close + container cleanup (30-60s per session) |
| **NATS Drain** | 10s | Flush pending messages |

## Impact

### #218 - Docker Timeouts
- **Before**: Startup can hang indefinitely, manual intervention required
- **After**: Clear 5s timeout, predictable failure mode
- **Fix Type**: Defensive bounds on external I/O

### #220 - Shutdown Timeouts
- **Before**: Shared context exhausted by early phases, later phases fail
- **After**: Each phase gets dedicated time budget, predictable behavior
- **Benefit**: Graceful shutdown even with slow sessions

## Risk Assessment

- **Risk Level**: LOW-MEDIUM
- **Backward Compatibility**: Full (internal change only, log format may change)
- **Breaking Changes**: None (timeout behavior improvement)
- **Performance Impact**: Positive (faster failure detection)

## Success Criteria

- ✅ Docker Ping times out after 5s on startup
- ✅ Container cleanup operations timeout appropriately
- ✅ HTTP shutdown gets full 10s budget
- ✅ Session termination gets full 2min budget
- ✅ NATS drain gets full 10s budget
- ✅ Clear logging indicates which phase/operation failed
- ✅ All existing tests pass
- ✅ No indefinite hangs during startup or shutdown

## References

- Issue #220: Implement separate shutdown timeouts for each subsystem
- Issue #218: Add startup timeout bounds for Docker operations
- Go pattern: context.WithTimeout for bounded operations
- Industry practice: K8s gives 30s default grace period with phases
