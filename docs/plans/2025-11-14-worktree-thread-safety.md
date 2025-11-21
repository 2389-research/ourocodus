# Worktree Manager Thread-Safety and Cleanup Fixes

**Date:** 2025-11-14
**Status:** Design
**Issues:** #235, #239
**Milestone:** 10
**Priority:** P1 (High)
**Effort:** Low (45 minutes)

## Summary

Fix two related worktree manager issues:
1. **#235** - Thread-safety violation risking git repository corruption under concurrent access
2. **#239** - Silent branch cleanup failures leaving orphaned `agent-*` branches

## Problems

### Problem 1: Thread-Safety Violation (#235)

**File:** `pkg/worktree/manager.go:20-21`
**Severity:** HIGH

The `AgentWorktreeManager` struct has documentation claiming thread-safe operations, but the implementation lacks any synchronization primitives. Concurrent calls to `Create()` or `Remove()` can corrupt the git repository state.

### Problem 2: Silent Branch Cleanup Failures (#239)

**File:** `pkg/worktree/manager.go:278-294`
**Severity:** MEDIUM

Best-effort branch cleanup in the `Remove()` method silently ignores errors, leaving orphaned `agent-*` branches in the repository. Over time, these accumulate and clutter the repository, making it harder to track active vs abandoned branches.

### Current Vulnerable Code

```go
// AgentWorktreeManager manages git worktrees for agent workspaces.
// Thread-safe for concurrent operations.  ← CLAIM
type AgentWorktreeManager struct {
    baseRepo   string
    worktreeTool WorktreeTool
    // NO MUTEX HERE ← VIOLATION
}

func (m *AgentWorktreeManager) Create(ctx context.Context, branchName, worktreePath string) error {
    // CONCURRENT CALLS CAN RACE:
    // - Git worktree add operations
    // - Branch creation
    // - Filesystem operations
}

func (m *AgentWorktreeManager) Remove(ctx context.Context, worktreePath string) error {
    // CONCURRENT CALLS CAN RACE:
    // - Git worktree remove operations
    // - Filesystem cleanup
}
```

### Race Scenarios

**Scenario 1: Concurrent Create Operations**
- Thread A: Creates worktree for `feature/login`
- Thread B: Creates worktree for `feature/signup`
- Both call `git worktree add` simultaneously
- Git's lock file mechanism may fail or corrupt internal state

**Scenario 2: Concurrent Create + Remove**
- Thread A: Creates worktree at `/tmp/workspace-1`
- Thread B: Removes worktree at `/tmp/workspace-1` (reusing path)
- Race between creation and cleanup
- Filesystem operations interleave unpredictably

**Scenario 3: Git Repository Corruption**
- Git worktree metadata stored in `.git/worktrees/`
- Concurrent modifications without coordination
- Can corrupt git index, config, or worktree metadata

## Root Cause

The manager was designed with the assumption that callers would serialize access, or relied on external synchronization. However:

1. The type is documented as "thread-safe for concurrent operations"
2. The relay server may spawn multiple agents concurrently
3. No external coordination exists in call sites

## Proposed Solution

Add a `sync.Mutex` to serialize all mutating operations:

```go
// AgentWorktreeManager manages git worktrees for agent workspaces.
// Thread-safe for concurrent operations.
type AgentWorktreeManager struct {
    baseRepo     string
    worktreeTool WorktreeTool
    mu           sync.Mutex  // ← ADD THIS
}

func (m *AgentWorktreeManager) Create(ctx context.Context, branchName, worktreePath string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Safe: serialized access to git operations
    return m.worktreeTool.Add(ctx, worktreePath, branchName)
}

func (m *AgentWorktreeManager) Remove(ctx context.Context, worktreePath string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Safe: serialized access to git operations
    return m.worktreeTool.Remove(ctx, worktreePath)
}
```

### Why Mutex (not RWMutex)?

- Both `Create` and `Remove` are mutating operations
- No read-only operations exist
- Simple mutex is sufficient and clearer

### Coarse vs Fine-Grained Locking

**Decision: Coarse-grained (method-level) locking**

**Rationale:**
1. Git operations are already I/O bound
2. Lock contention is low (typical workload: 1-10 concurrent agents)
3. Simpler implementation and reasoning
4. Matches claimed "thread-safe for concurrent operations" semantics

**Alternative (Fine-grained) rejected:**
- Lock per worktree path
- Higher complexity
- Overkill for current scale
- Can revisit if profiling shows contention

## Implementation Steps

### Phase 1: Add Mutex Field and Logger
```go
// In pkg/worktree/manager.go
type AgentWorktreeManager struct {
    baseRepo     string
    worktreeTool WorktreeTool
    mu           sync.Mutex
}
```

### Phase 2: Protect Create Method
```go
func (m *AgentWorktreeManager) Create(ctx context.Context, branchName, worktreePath string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Existing implementation remains unchanged
    if err := m.worktreeTool.Add(ctx, worktreePath, branchName); err != nil {
        return fmt.Errorf("failed to add worktree: %w", err)
    }

    return nil
}
```

### Phase 3: Protect Remove Method
```go
func (m *AgentWorktreeManager) Remove(ctx context.Context, worktreePath string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Existing implementation remains unchanged
    if err := m.worktreeTool.Remove(ctx, worktreePath); err != nil {
        return fmt.Errorf("failed to remove worktree: %w", err)
    }

    return nil
}
```

### Phase 4: Document Lock Semantics
```go
// AgentWorktreeManager manages git worktrees for agent workspaces.
// Thread-safe for concurrent operations via internal mutex serialization.
// Methods may block if another goroutine holds the lock.
type AgentWorktreeManager struct {
    baseRepo     string
    worktreeTool WorktreeTool
    mu           sync.Mutex // Serializes all mutating operations
}
```

## Testing Strategy

### Unit Tests

**Test 1: Concurrent Create Operations**
```go
func TestAgentWorktreeManager_ConcurrentCreate(t *testing.T) {
    manager := NewAgentWorktreeManager("/tmp/repo", mockTool)

    var wg sync.WaitGroup
    errors := make(chan error, 10)

    // Spawn 10 concurrent Create operations
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            branch := fmt.Sprintf("feature/test-%d", n)
            path := fmt.Sprintf("/tmp/workspace-%d", n)
            if err := manager.Create(context.Background(), branch, path); err != nil {
                errors <- err
            }
        }(i)
    }

    wg.Wait()
    close(errors)

    // Assert: All operations succeeded (no corruption)
    for err := range errors {
        t.Errorf("Concurrent create failed: %v", err)
    }
}
```

**Test 2: Concurrent Create + Remove**
```go
func TestAgentWorktreeManager_ConcurrentCreateRemove(t *testing.T) {
    manager := NewAgentWorktreeManager("/tmp/repo", mockTool)

    // Create 5 worktrees
    paths := []string{}
    for i := 0; i < 5; i++ {
        path := fmt.Sprintf("/tmp/workspace-%d", i)
        branch := fmt.Sprintf("feature/test-%d", i)
        require.NoError(t, manager.Create(context.Background(), branch, path))
        paths = append(paths, path)
    }

    var wg sync.WaitGroup

    // Concurrently remove existing and create new
    for i := 0; i < 5; i++ {
        wg.Add(2)

        go func(idx int) {
            defer wg.Done()
            _ = manager.Remove(context.Background(), paths[idx])
        }(i)

        go func(idx int) {
            defer wg.Done()
            newPath := fmt.Sprintf("/tmp/workspace-new-%d", idx)
            newBranch := fmt.Sprintf("feature/new-%d", idx)
            _ = manager.Create(context.Background(), newBranch, newPath)
        }(i)
    }

    wg.Wait()

    // Assert: No panics, no corrupted state
}
```

**Test 3: Race Detector**
```go
// Run with: go test -race ./pkg/worktree/...
func TestAgentWorktreeManager_RaceDetection(t *testing.T) {
    manager := NewAgentWorktreeManager("/tmp/repo", mockTool)

    done := make(chan bool)

    // Writer goroutine
    go func() {
        for i := 0; i < 100; i++ {
            path := fmt.Sprintf("/tmp/workspace-%d", i%10)
            branch := fmt.Sprintf("feature/test-%d", i%10)
            _ = manager.Create(context.Background(), branch, path)
        }
        done <- true
    }()

    // Another writer goroutine
    go func() {
        for i := 0; i < 100; i++ {
            path := fmt.Sprintf("/tmp/workspace-%d", i%10)
            _ = manager.Remove(context.Background(), path)
        }
        done <- true
    }()

    <-done
    <-done

    // Race detector will report if mutex is missing
}
```

### Integration Tests

**Test 4: Real Git Operations**
```go
func TestAgentWorktreeManager_RealGitConcurrency(t *testing.T) {
    // Setup: Real git repository
    tmpDir := t.TempDir()
    initGitRepo(t, tmpDir)

    tool := &RealWorktreeTool{}
    manager := NewAgentWorktreeManager(tmpDir, tool)

    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()

            branch := fmt.Sprintf("feature/test-%d", n)
            path := filepath.Join(tmpDir, "worktrees", fmt.Sprintf("wt-%d", n))

            // Create
            err := manager.Create(context.Background(), branch, path)
            require.NoError(t, err)

            // Verify worktree exists
            _, err = os.Stat(path)
            require.NoError(t, err)

            // Remove
            err = manager.Remove(context.Background(), path)
            require.NoError(t, err)
        }(i)
    }

    wg.Wait()

    // Verify git repository is not corrupted
    verifyGitRepoIntegrity(t, tmpDir)
}
```

## Call Site Analysis

### Current Callers

**1. Relay Server - Session Manager**
```go
// pkg/relay/session/manager.go
func (m *Manager) SpawnAgent(ctx context.Context, sessionID, workspace string) error {
    // May be called concurrently for different sessions
    if err := m.worktreeManager.Create(ctx, branchName, worktreePath); err != nil {
        return err
    }
}
```

**Impact:** Multiple users spawning agents simultaneously → concurrent Create calls → race

**2. Container Session Cleanup**
```go
// pkg/containersession/session.go
func (s *Session) Cleanup(ctx context.Context) error {
    // May be called concurrently during shutdown
    if err := s.worktreeManager.Remove(ctx, s.worktreePath); err != nil {
        return err
    }
}
```

**Impact:** Multiple sessions terminating simultaneously → concurrent Remove calls → race

### No Changes Required

The mutex addition is transparent to callers. All existing code continues to work correctly, but now with proper synchronization.

## Performance Impact

### Expected Impact: Negligible

**Benchmark Assumptions:**
- Git worktree operations: 50-200ms (I/O bound)
- Mutex acquisition: <1µs (negligible)
- Lock contention: Low (1-10 concurrent agents typical)

**Worst Case:**
- 10 concurrent Create operations
- Serial execution: 10 * 100ms = 1000ms
- Without mutex (racey): undefined behavior, corruption risk
- With mutex: Correct, predictable behavior

**Trade-off:** Accept minor serialization delay to prevent corruption

### Future Optimization

If profiling shows mutex contention is a bottleneck:
1. Per-worktree locking (path-based key)
2. Lock-free data structures
3. Queue-based coordinator

**Current decision:** Premature optimization. Start simple.

## Rollout Plan

### Phase 1: Implementation (LOW RISK)
1. Add mutex field to struct
2. Add Lock/Unlock to Create and Remove
3. Update documentation

### Phase 2: Testing (MEDIUM EFFORT)
1. Unit tests with race detector
2. Integration tests with real git
3. Concurrent stress tests

### Phase 3: Deployment (NO RISK)
- Drop-in replacement
- No API changes
- No configuration changes
- Backwards compatible

## Success Criteria

- ✅ No data races detected by `go test -race`
- ✅ Concurrent operations complete without errors
- ✅ Git repository integrity maintained under stress
- ✅ Existing tests continue to pass
- ✅ No performance regression >10ms per operation

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Deadlock if lock held across blocking call | LOW | HIGH | Use `defer unlock()`, no nested locks |
| Performance degradation | LOW | LOW | Benchmark before/after, optimize if needed |
| Missed locking in future code | MEDIUM | HIGH | Document locking policy, code review |

## References

- Issue #235: Worktree manager thread-safety violation
- Related: Issue #239 (cleanup errors) may also need mutex
- Go sync.Mutex documentation: https://pkg.go.dev/sync#Mutex
- Git worktree internals: https://git-scm.com/docs/git-worktree
