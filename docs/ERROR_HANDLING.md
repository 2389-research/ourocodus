# Error Handling Strategy - Phase 1

## Overview

Phase 1 error handling is **fail-fast** with minimal recovery. The goal is to surface problems quickly so we can validate assumptions and understand failure modes.

**Philosophy:** Let it crash, log everything, fix the root cause.

## Error Categories

### 1. Fatal Errors (System Shutdown Required)

These errors indicate fundamental problems that prevent the system from functioning. The relay should shut down gracefully.

**Examples:**
- Failed to bind to port (already in use)
- ANTHROPIC_API_KEY missing or invalid
- Unable to create agent worktree directory
- ACP binary not found in PATH

**Handling:**
```go
log.Fatal("FATAL: %v", err)
os.Exit(1)
```

**User Impact:** System doesn't start. Clear error message shown.

---

### 2. Session Errors (Terminate Session, Keep System Running)

These errors affect a single session but other sessions can continue.

**Examples:**
- ACP process spawn failure (bad workspace path)
- ACP process crash during operation
- WebSocket client disconnection
- Invalid JSON-RPC from ACP process

**Handling:**
```go
log.Error("Session %s: %v", sessionID, err)
session.Close()
// Notify client via WebSocket error message
ws.WriteJSON(ErrorMessage{Type: "error", Error: err.Error()})
ws.Close()
```

**User Impact:** Single session fails, user sees error in PWA, other sessions unaffected.

---

### 3. Message Errors (Log and Continue)

These errors affect a single message but the session continues.

**Examples:**
- Malformed WebSocket message from client
- Empty message content
- Message too large (>1MB)
- Rate limit exceeded

**Handling:**
```go
log.Warn("Session %s: Invalid message: %v", sessionID, err)
ws.WriteJSON(ErrorMessage{Type: "error", Error: "Invalid message format"})
// Keep session alive, wait for next message
```

**User Impact:** Single message fails, user sees error bubble in chat, can retry.

---

### 4. Transient Errors (Retry with Backoff)

These errors may resolve themselves with retry.

**Examples:**
- Anthropic API rate limit (429)
- Anthropic API server error (500, 502, 503)
- Network timeout reading from ACP stdout
- Git lock file contention (.git/index.lock)

**Handling:**
```go
for attempt := 1; attempt <= 3; attempt++ {
    err := operation()
    if err == nil {
        return nil
    }
    if isTransient(err) {
        backoff := time.Duration(attempt) * time.Second
        log.Warn("Retry %d/%d after %v: %v", attempt, 3, backoff, err)
        time.Sleep(backoff)
        continue
    }
    return err // Non-transient, fail immediately
}
return fmt.Errorf("operation failed after 3 retries")
```

**User Impact:** Brief delay, operation succeeds automatically.

---

## Error Response Format

### WebSocket Error Messages

All error messages sent to PWA follow this format:

```json
{
  "type": "error",
  "sessionId": "uuid-here",
  "agentId": "auth",
  "error": {
    "code": "ACP_PROCESS_CRASHED",
    "message": "Agent process exited unexpectedly",
    "details": "exit status 1: stderr output here",
    "recoverable": false,
    "timestamp": "2025-10-22T12:34:56Z"
  }
}
```

**Error Codes:**
- `INVALID_MESSAGE` - Malformed WebSocket message
- `ACP_SPAWN_FAILED` - Failed to start ACP process
- `ACP_PROCESS_CRASHED` - ACP process exited during operation
- `ACP_TIMEOUT` - No response from ACP within timeout
- `API_KEY_INVALID` - Anthropic API key rejected
- `WORKTREE_ERROR` - Git worktree operation failed
- `RATE_LIMIT` - Too many messages too quickly
- `MESSAGE_TOO_LARGE` - Message exceeds size limit
- `SESSION_NOT_FOUND` - Session ID doesn't exist

---

## Logging Strategy

### Log Levels

**FATAL:** System cannot continue, about to exit
- Missing API key
- Port bind failure
- ACP binary not found

**ERROR:** Session-level failure, session terminated
- ACP process crash
- Worktree creation failed
- JSON-RPC parse error

**WARN:** Message-level failure, session continues
- Invalid message format
- Rate limit warning
- Retry attempt

**INFO:** Normal operations
- Session created
- Agent spawned
- Message relayed
- Session closed

**DEBUG:** Detailed flow (disabled in production)
- Raw JSON-RPC messages
- WebSocket frame details
- Goroutine lifecycle

### Log Format

**Structured JSON logs for Phase 1:**

```json
{
  "level": "ERROR",
  "timestamp": "2025-10-22T12:34:56Z",
  "component": "relay",
  "sessionId": "uuid-here",
  "agentId": "auth",
  "message": "ACP process crashed",
  "error": "exit status 1",
  "stderr": "Error: ANTHROPIC_API_KEY not found"
}
```

**Why JSON:** Easy to parse, grep, and ship to log aggregators later.

---

## Phase 1 Limitations

### No Retry for User Messages

If a user message fails to send to ACP, we **don't** automatically retry. User must resend.

**Rationale:** Avoids duplicate operations (agent may have received message before crash).

**Future:** Phase 2 adds message acknowledgement and idempotency.

---

### No Circuit Breaker

If Anthropic API is down, we'll keep trying (with backoff) indefinitely.

**Rationale:** Simple for Phase 1, user can refresh page to abort.

**Future:** Phase 2 adds circuit breaker after N failures.

---

### No Dead Letter Queue

Failed messages are logged but not stored for retry.

**Rationale:** No persistence in Phase 1.

**Future:** Phase 4 adds event store with replay capability.

---

## Error Recovery Playbook

### Scenario 1: ACP Process Crashes Mid-Conversation

**Detection:** Process exit detected, session.cmd.Wait() returns

**Response:**
1. Log error with session ID, agent ID, exit code, stderr
2. Send error message to PWA client
3. Close WebSocket connection
4. Clean up session from memory
5. User must create new session to resume

**No Attempt To:**
- Restart process automatically (may repeat crash)
- Preserve conversation history (not persisted)

---

### Scenario 2: WebSocket Client Disconnects

**Detection:** `ws.ReadMessage()` returns error

**Response:**
1. Log disconnection (INFO level)
2. Terminate ACP process gracefully (SIGTERM)
3. Wait up to 5 seconds for process to exit
4. Force kill if still running (SIGKILL)
5. Clean up session from memory

**No Attempt To:**
- Keep ACP process running for reconnect (no session persistence)

---

### Scenario 3: Git Worktree Lock Contention

**Detection:** Git command fails with "index.lock" in error message

**Response:**
1. Retry with exponential backoff (1s, 2s, 4s)
2. After 3 attempts, treat as session error
3. Log error, notify client, terminate session

**If Persistent:** Add worktree locking mechanism (out of scope for Phase 1)

---

### Scenario 4: Anthropic API Key Invalid

**Detection:** ACP process stderr contains "authentication" or "API key"

**Response:**
1. Log as ERROR (likely configuration issue)
2. Treat as fatal if no sessions have succeeded yet
3. Treat as session error if other sessions working
4. Clear error message to user: "Check ANTHROPIC_API_KEY"

**No Attempt To:**
- Validate key before spawning (adds latency)
- Prompt user for key (not in Phase 1)

---

## Testing Error Scenarios

### Manual Testing Checklist

Before Issue #13, validate error handling:

- [ ] Start relay without ANTHROPIC_API_KEY → Fatal error, clean message
- [ ] Start relay on port already in use → Fatal error, clean message
- [ ] Send malformed JSON over WebSocket → Error message, session continues
- [ ] Send empty message → Error message, session continues
- [ ] Kill ACP process during conversation → Session error, WebSocket closes
- [ ] Disconnect WebSocket → ACP process terminates
- [ ] Create 2 sessions, crash one → Other session unaffected
- [ ] Git worktree already exists → Session error, clean message
- [ ] Send 100 messages in 1 second → Rate limit error (if implemented)

### Error Message Validation

For each error code:
- [ ] Message is clear and actionable
- [ ] No stack traces or internal details exposed
- [ ] Includes session ID for debugging
- [ ] `recoverable` flag is correct

---

## Monitoring and Observability

### Phase 1 Minimal Observability

**What to log:**
- Session lifecycle (created, closed)
- Agent spawn success/failure
- Message count per session
- Error rate by error code
- ACP process crashes with stderr

**What NOT to log:**
- Message content (privacy concern)
- API key (security concern)
- Full JSON-RPC payloads (noise)

**Log to:** stdout (JSON format)

**Future:** Ship logs to Loki/Elasticsearch, add metrics to Prometheus

---

## Open Questions

### Must Answer During Implementation

1. **ACP Process Timeout:** How long to wait for ACP response?
   - Proposed: 5 minutes (allows for package installs)
   - Test with real operations to validate

2. **WebSocket Ping/Pong:** How to detect zombie connections?
   - Proposed: 30 second ping interval
   - Close connection after 90 seconds without pong

3. **Rate Limiting:** Prevent abuse?
   - Proposed: 10 messages per second per session
   - Enforce with token bucket

4. **Max Message Size:** Prevent memory issues?
   - Proposed: 1MB per message
   - Reject larger messages with clear error

---

## Migration to Phase 2

When adding NATS (Phase 2), error handling changes:

**New Patterns:**
- Message acknowledgement (at-least-once delivery)
- Dead letter queue for failed messages
- Circuit breaker for failing agents
- Health checks and automatic recovery

**Keep From Phase 1:**
- Structured logging format
- Error code taxonomy
- Fatal vs recoverable distinction

---

## Decision Log

| Decision | Rationale | Phase 1? |
|----------|-----------|----------|
| Fail-fast, no auto-recovery | Validate assumptions before adding complexity | ✅ Yes |
| JSON structured logs | Easy parsing, future aggregation | ✅ Yes |
| 3 retry attempts with backoff | Balance between resilience and quick failure | ✅ Yes |
| No message persistence | Avoids complexity, acceptable for POC | ✅ Yes |
| Terminate ACP on WebSocket disconnect | No reconnection in Phase 1 | ✅ Yes |
| Fatal error exits entire system | Clear signal something is wrong | ✅ Yes |

---

## Summary

**Phase 1 Error Handling:** Simple, explicit, fail-fast

**Categories:** Fatal (exit) → Session (close) → Message (log) → Transient (retry)

**Strategy:** Log everything, surface errors quickly, let user retry

**Future:** Add persistence, circuit breakers, auto-recovery in later phases
