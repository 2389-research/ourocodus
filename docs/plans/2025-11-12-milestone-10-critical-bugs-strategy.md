# Milestone 10 Critical Bug Fixes - Implementation Strategy

**Date:** 2025-11-12
**Milestone:** Critical Bug Fixes - Code Review Nov 2025
**Total Issues:** 30 (7 critical remaining after PRs 240-241)

## Status

**Resolved by PR #240 and #241:**
- #233: NATS metrics compile error (functions exist in errors.go)
- #230: NATS Drain() goroutine leak
- #234: NATS Drain/Close race condition
- #221: ACP logStderr goroutine leak
- #223: ExecAttachment Close() goroutine leak
- #224: handleContainerOutput goroutine leaks
- #212: ACP goroutine leak (symptom resolved by fixing root causes)

**Remaining:** 7 critical bugs require fixes before production deployment.

## Implementation Approach

**Strategy:** Severity-based fixes with pattern analysis.

**Rationale:** Production blockers demand immediate resolution. Security issues take precedence over stability issues, which take precedence over resource leaks.

## Bug Patterns

### Pattern 1: Security Issues (2 bugs)
Attackers exploit these vulnerabilities. Fix these first.

- **#232:** Directory traversal in worktree manager
- **#217:** Credential leakage in NATS URL logs

### Pattern 2: Stability Issues (2 bugs)
Runtime failures occur without these fixes.

- **#213:** WebSocket concurrent write race (causes panics)
- **#211:** ACP client Close hangs indefinitely

### Pattern 3: Resource Leaks (3 bugs)
Long-running processes exhaust resources over time.

- **#225:** Container attach responses never close
- **#214:** WebSocket sessions persist after disconnect
- **#216:** Shutdown leaves resources uncleaned

## Fix Sequence

### Batch 1: Security (P0)

**#232: Directory Traversal Vulnerability**
- **Files:** `pkg/worktree/*.go`
- **Impact:** Attackers access files outside worktree boundaries
- **Fix:** Validate and sanitize all paths before operations
- **Effort:** Medium (2-3 hours)

**#217: Credential Leakage**
- **Files:** `cmd/relay/main.go`, NATS connection setup
- **Impact:** Logs expose NATS credentials
- **Fix:** Redact credentials from URLs before logging
- **Effort:** Low (1 hour)

### Batch 2: Stability (P1)

**#213: WebSocket Write Race**
- **Files:** `pkg/pwa/websocket.go`
- **Impact:** Concurrent writes cause panics
- **Fix:** Add mutex-protected write wrapper (design exists)
- **Effort:** Medium (2-3 hours)

**#211: ACP Client Close Timeout**
- **Files:** `pkg/acp/client.go`
- **Impact:** Shutdown hangs when Close blocks
- **Fix:** Add context and timeout to Close operations
- **Effort:** Medium (2-3 hours)

### Batch 3: Resource Cleanup (P2)

**#225: Container Attach Response Leaks**
- **Files:** `pkg/containersession/*.go`
- **Impact:** File descriptors and memory leak over time
- **Fix:** Close all responses in cleanup paths
- **Effort:** Low-Medium (1-2 hours)

**#214: WebSocket Session Cleanup**
- **Files:** `pkg/pwa/websocket.go`
- **Impact:** Sessions accumulate after client disconnects
- **Fix:** Hook cleanup into WebSocket close handler
- **Effort:** Low (1 hour)

**#216: Shutdown Cleanup Bug**
- **Files:** `cmd/relay/main.go`
- **Impact:** Resources remain allocated after shutdown
- **Fix:** Correct cleanup order and error handling
- **Effort:** Low (1 hour)

## Testing Strategy

Each fix requires:
1. Unit tests demonstrating the bug
2. Fix implementation
3. Verification tests pass with `-race` flag
4. Integration tests for critical paths
5. Manual verification where applicable

## Delivery Plan

**Phase 1: Security (Target: Day 1)**
- Fix #232 and #217
- Create PR for security review
- Block other work until merged

**Phase 2: Stability (Target: Day 2)**
- Fix #213 and #211
- Create PR, run full test suite
- Verify no regressions in shutdown paths

**Phase 3: Cleanup (Target: Day 3)**
- Fix #225, #214, #216
- Create PR with leak tests
- Run extended reliability tests

## Success Criteria

All seven bugs are fixed when:
- Code compiles without errors
- All tests pass with race detector
- Manual verification confirms fixes
- Code review approves changes
- PRs merge to main branch

## Dependencies

- PR #240: Must merge before Batch 2 work
- PR #241: Must merge before Batch 2 work
- WebSocket design doc: Reference for #213 implementation

## Risk Mitigation

**Risk:** Security fixes introduce regressions
**Mitigation:** Comprehensive test coverage, staged rollout

**Risk:** Timeout changes affect shutdown behavior
**Mitigation:** Test with various timeout scenarios

**Risk:** WebSocket mutex impacts performance
**Mitigation:** Benchmark before/after, monitor latency
