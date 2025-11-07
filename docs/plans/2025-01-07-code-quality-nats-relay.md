# Code Quality Bundle: NATS & Relay Improvements

**Date:** 2025-01-07
**Milestone:** Milestone 2 - Container Runtime Integration
**Issues:** #93, #97, #159

## Overview

This bundle addresses three code quality issues that improve observability, defensive programming, and code cleanliness across the NATS client library and relay service.

## Problems

### Issue #93: Unbounded Prometheus Metric Cardinality

The `normalizeSubject()` function in `pkg/nats/metrics.go:247` returns subjects unchanged, creating unbounded metric cardinality. When subjects contain session or agent IDs (`sessions.abc123.events`), Prometheus creates new time series for each unique ID, causing:

- Excessive memory consumption
- Performance degradation
- Unusable metrics for aggregation

### Issue #97: Non-Idiomatic Error Wrappers

`WrapTransientError` and `WrapPermanentError` in `pkg/nats/errors.go:69-85` allocate error objects even when wrapping nil. Go idiom requires error wrappers to return nil when wrapping nil.

### Issue #159: Dead Path Validation Code

Lines 146-149 in `pkg/relay/session_adapter.go` contain ineffective path validation:

```go
relPath, err := filepath.Rel(cleanPath, absPath)
if err == nil && (relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator))) {
    return fmt.Errorf("workspace path escapes base directory")
}
```

This check fails to provide security:
- When `cleanPath` is relative (common case), `filepath.Rel` errors and skips the check
- When paths match, `relPath` becomes "." and doesn't trigger rejection
- Earlier checks (lines 132-137, 140-142) already prevent traversal

## Goals

1. **Bounded metrics**: Normalize subjects to prevent cardinality explosion
2. **Idiomatic errors**: Follow Go error wrapping conventions
3. **Clean code**: Remove dead validation logic
4. **Backward compatibility**: Preserve existing behavior

## Solution

### Architecture: Two-PR Strategy

**PR 1: NATS Improvements (#93 + #97)**
- Component: `pkg/nats`
- Files: `metrics.go`, `errors.go`
- Relationship: Both enhance NATS client robustness

**PR 2: Relay Cleanup (#159)**
- Component: `pkg/relay`
- File: `session_adapter.go`
- Independence: Can merge without blocking PR 1

### Positional Subject Normalization

Replace dynamic segments at known positions with wildcards:

```go
func normalizeSubject(subject string) string {
    tokens := strings.Split(subject, ".")
    if len(tokens) == 0 {
        return subject
    }

    switch tokens[0] {
    case "sessions":
        // sessions.<session-id>.{events|work|results|approvals}
        // Normalize to: sessions.*.{suffix}
        if len(tokens) >= 2 {
            tokens[1] = "*"
        }
    case "agents":
        // agents.<session-id>.<agent-id>.heartbeat
        // Normalize to: agents.*.*.{suffix}
        if len(tokens) >= 2 {
            tokens[1] = "*"
        }
        if len(tokens) >= 3 {
            tokens[2] = "*"
        }
    }

    return strings.Join(tokens, ".")
}
```

**Examples:**
- `sessions.abc123.events` → `sessions.*.events`
- `sessions.sess_456.work` → `sessions.*.work`
- `agents.sess_123.agent_789.heartbeat` → `agents.*.*.*`
- `test.static` → `test.static` (preserved)

### Nil-Safe Error Wrappers

Add nil checks to both wrapper functions:

```go
func WrapTransientError(op, subject string, err error) error {
    if err == nil {
        return nil
    }
    return &TransientError{
        Op:      op,
        Subject: subject,
        Err:     err,
    }
}

func WrapPermanentError(op, subject string, err error) error {
    if err == nil {
        return nil
    }
    return &PermanentError{
        Op:      op,
        Subject: subject,
        Err:     err,
    }
}
```

### Remove Dead Path Validation

Delete lines 146-149 from `pkg/relay/session_adapter.go`. Security remains enforced by:
- Lines 132-137: Block system directories (`/etc`, `/sys`, `/proc`)
- Lines 140-142: Catch ".." in cleaned path

## Implementation Plan

### PR 1: NATS Improvements

**Commit 1: Implement subject normalization (#93)**
- Add positional logic to `normalizeSubject()`
- Support `sessions.*` and `agents.*` patterns
- Preserve unknown prefixes unchanged

**Commit 2: Add normalization tests**
- Test cases: sessions, agents, unknown prefixes
- Verify metrics use normalized subjects
- Integration test with Prometheus output

**Commit 3: Add nil checks to error wrappers (#97)**
- Guard `WrapTransientError` with nil check
- Guard `WrapPermanentError` with nil check
- Add unit tests for nil-in → nil-out

### PR 2: Relay Cleanup

**Commit 1: Remove redundant path validation (#159)**
- Delete lines 146-149
- Verify existing tests pass
- Confirm security checks at lines 132-142 remain

## Testing Strategy

### Unit Tests

**Subject Normalization:**
```go
TestNormalizeSubject(t *testing.T) {
    cases := []struct{
        input    string
        expected string
    }{
        {"sessions.abc123.events", "sessions.*.events"},
        {"sessions.test.work", "sessions.*.work"},
        {"agents.s1.a2.heartbeat", "agents.*.*.*"},
        {"unknown.prefix", "unknown.prefix"},
    }
    // ...
}
```

**Error Wrappers:**
```go
TestErrorWrapperNilHandling(t *testing.T) {
    if err := WrapTransientError("test", "subject", nil); err != nil {
        t.Error("expected nil")
    }
    if err := WrapPermanentError("test", "subject", nil); err != nil {
        t.Error("expected nil")
    }
}
```

**Path Validation:**
- Run existing test suite
- All path traversal tests must pass
- Verify security checks remain effective

### Integration Tests

- Verify Prometheus metrics show bounded cardinality
- Confirm no performance regression in metrics collection
- Validate existing relay path security

## Success Criteria

- [ ] Prometheus metrics display `sessions.*` instead of `sessions.abc123`
- [ ] Error wrappers return nil for nil input
- [ ] No performance regression in metrics
- [ ] All existing tests pass
- [ ] CI green on all platforms

## Risks and Mitigations

### Risk: Normalization loses debugging information
**Likelihood:** Low
**Impact:** Low
**Mitigation:** Original subjects remain in logs; metrics aggregate correctly

### Risk: Existing code expects non-nil from error wrappers
**Likelihood:** Very Low
**Impact:** Low
**Mitigation:** Nil checks are defensive; Go standard library follows this pattern

### Risk: Removing path check exposes security hole
**Likelihood:** None
**Impact:** N/A
**Mitigation:** Earlier checks provide actual security; removed code never executed effectively

## Future Enhancements

1. **Configurable patterns**: Add option to define custom normalization rules
2. **Metrics dashboard**: Create Grafana dashboard using normalized subjects
3. **Pattern detection**: Log warning when unknown subject patterns appear

## References

- NATS subject naming: `docs/PROTOCOLS.md:137-151`
- Prometheus best practices: https://prometheus.io/docs/practices/naming/
- Go error handling: https://go.dev/blog/error-handling-and-go
