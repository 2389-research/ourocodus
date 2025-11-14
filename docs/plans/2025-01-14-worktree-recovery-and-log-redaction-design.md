# Worktree Recovery and Log Redaction Design

**Date:** 2025-01-14
**Issues:** #192 (Agent spawn fails when stale worktree entry exists), #186 (Redact sensitive data from WebSocket message logs)
**Milestone:** 9 (PWA Polish & Features)

## Overview

This design addresses two relay service improvements: automatic recovery from stale Git worktree entries and sanitization of WebSocket message logs to prevent PII exposure.

## Problem Statement

### Stale Worktree Entries (#192)

When users manually delete worktree directories without running `git worktree remove`, Git retains stale registrations. Subsequent agent spawns fail with:

```
fatal: '/path/to/workspaces/agent-echo' is a missing but already registered worktree;
use 'add -f' to override, or 'prune' or 'remove' to clear
```

The relay crashes with `exit status 128`, forcing users to manually run `git worktree prune`.

### Sensitive Data in Logs (#186)

The relay logs full WebSocket message content at `pkg/relay/server.go:584`:

```go
s.logger.Printf("[RELAY] Received message: %s", string(message))
```

This exposes PII (names, emails, addresses) and credentials (API keys, tokens), creating GDPR/CCPA compliance risks.

## Design Solution

### Worktree Error Detection and Recovery

**Detection:** Parse git worktree errors for exit status 128 and the specific text "already registered".

**Recovery flow:**
1. Attempt `git worktree add workspaces/agent-<role>`
2. On exit 128, check error message for "already registered"
3. If matched, run `git worktree prune`
4. Retry `git worktree add` once
5. Report success or final error

**Error handling:**
- Exit 128 for other reasons (permissions, disk space): report original error without pruning
- Prune succeeds but retry fails: report as fatal error with context
- Permission errors during prune: report as configuration issue

### Log Redaction

**Implementation:** Extract and log only the message type field.

**Before:**
```go
s.logger.Printf("[RELAY] Received message: %s", string(message))
```

**After:**
```go
messageType := extractMessageType(message)
s.logger.Printf("[RELAY] Received message type=%s size=%d bytes", messageType, len(message))
```

**Helper function:**
```go
func extractMessageType(data []byte) string {
    var msg struct {
        Type string `json:"type"`
    }
    if err := json.Unmarshal(data, &msg); err != nil {
        return "unknown"
    }
    if msg.Type == "" {
        return "unknown"
    }
    return msg.Type
}
```

**Fallback behavior:**
- Invalid JSON: log `"Received message (invalid JSON, %d bytes)"`
- Missing type field: return `"unknown"`
- Never log raw message content

## Testing Strategy

### Worktree Recovery Tests

**Unit tests:**
- Mock git command returning exit 128 with "already registered" → verify prune called, retry attempted
- Mock git command returning exit 128 for other reasons → verify prune not called, original error returned
- Mock prune succeeds, retry fails → verify descriptive error returned

**Integration test:**
- Create actual stale worktree entry
- Attempt agent spawn
- Verify automatic recovery and successful spawn

**Edge cases:**
- Permission errors during prune
- Disk full during retry
- Multiple stale entries

### Log Redaction Tests

**Unit tests for `extractMessageType()`:**
- Valid JSON with type field → extracts type correctly
- Valid JSON without type field → returns "unknown"
- Invalid JSON → returns "unknown", doesn't panic
- Empty message → handles gracefully
- Type field with special characters → sanitizes output

**Security verification:**
- Code review confirms no test logs full message content
- Manual testing verifies logs contain only type and size

## Implementation Notes

**Location:** `pkg/relay/` (container spawning code and WebSocket message handling)

**Files to modify:**
- Find worktree creation code (likely in container spawn logic)
- `pkg/relay/server.go:584` for log statement

**Backwards compatibility:** No breaking changes. Error messages improve, logs become safer.

**Performance impact:** Negligible. JSON parsing for type extraction adds microseconds per message.

## Success Criteria

1. Agent spawning succeeds after manual worktree directory deletion without user intervention
2. WebSocket message logs show only message type and size, never full content
3. All tests pass with coverage for error cases
4. No breaking changes to existing functionality
