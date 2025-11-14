# Relay Server Security Hardening

**Date:** 2025-11-14
**Status:** Implementation
**Issues:** #219 (Error Message Sanitization), #215 (WebSocket DoS Protection)
**Milestone:** 10
**Priority:** P0 (High - Security Critical)
**Effort:** Medium (4-6 hours total)

## Summary

Implement comprehensive security hardening for the relay WebSocket server:
1. **#219** - Sanitize error messages to prevent information disclosure
2. **#215** - Harden WebSocket server against DoS attacks

## Problem Analysis

### Problem 1: Error Message Information Disclosure (#219)

**Severity:** MEDIUM (Security - Information Disclosure)
**File:** `pkg/relay/server.go`
**Lines:** 61-66, 209-214, 288-294, 358-362

**Current vulnerabilities:**
- Raw error strings expose internal details (stack traces, file paths, function names)
- Docker errors leak build instructions ("make agent-image", image names)
- Environment info disclosed (workspace paths, configuration)
- Aids reconnaissance for attackers
- Confuses end users with technical details they can't action

**Example of leaked information:**
```json
{
  "type": "error",
  "code": "AGENT_SPAWN_FAILED",
  "message": "Failed to spawn agent: container image 'ourocodus-agent:latest' not found. Build it with 'make agent-image' in /Users/dev/ourocodus"
}
```

### Problem 2: WebSocket DoS Vulnerabilities (#215)

**Severity:** HIGH (Security + Availability)
**File:** `pkg/relay/server.go`, `cmd/relay/main.go`
**Lines:** 561-565 (upgrade), 582-599 (read loop)

**Current vulnerabilities:**

1. **No Read Limits** - Can read unlimited message size → OOM
2. **No Read Deadlines** - Idle connections never timeout → FD exhaustion
3. **No Origin Checks** - Cross-Site WebSocket Hijacking (CSWSH) attacks possible
4. **No Rate Limiting** - Unlimited connections from single IP, unlimited messages per connection

**Attack scenarios:**
- Attacker sends 1GB message → OOM crash
- Attacker opens 10,000 idle connections → FD exhaustion
- Attacker from evil.com hijacks user's WebSocket session
- Attacker floods with connections or messages → service degradation

## Root Cause

### #219 - Error Sanitization
- No error message sanitization layer between internal errors and client responses
- Direct use of `err.Error()` in client-facing messages
- Missing error code mapping for stable programmatic handling

### #215 - WebSocket Hardening
- WebSocket upgrader uses default permissive settings
- No connection-level protections configured
- Missing rate limiting infrastructure
- No liveness checks (ping/pong)

## Proposed Solution

### Solution 1: Error Message Sanitization (#219)

**Approach:** Add error sanitization helper that maps internal errors to stable, user-friendly messages.

```go
// errorSanitizer.go - New file in pkg/relay/
package relay

// sanitizeError converts internal errors to user-safe messages
func sanitizeError(err error) string {
    switch {
    case errors.Is(err, container.ErrContainerSetupFailed):
        return "Agent container unavailable. Please ensure the system is properly configured."

    case errors.Is(err, container.ErrImageNotFound):
        return "Agent container unavailable. Please contact your system administrator."

    case errors.Is(err, context.DeadlineExceeded):
        return "Operation timed out. Please try again."

    case errors.Is(err, ErrSessionNotFound):
        return "Session not found. Please create a new session."

    case errors.Is(err, ErrAgentNotReady):
        return "Agent is starting. Please wait a moment and try again."

    case errors.Is(err, ErrInvalidMessage):
        return "Invalid message format. Please check your request."

    case errors.Is(err, ErrSessionAlreadyExists):
        return "A session with this ID already exists."

    default:
        // Generic fallback - never expose raw error
        return "An internal error occurred. Please contact support if this persists."
    }
}
```

**Update call sites:**

1. **Validation errors** (lines 61-66):
```go
validationErr = ValidationError{
    Code:        "INVALID_MESSAGE",
    Message:     sanitizeError(err),  // Instead of err.Error()
    Recoverable: true,
}
```

2. **Session creation errors** (lines 209-214):
```go
if err != nil {
    s.logger.Printf("[ERROR] Failed to create session: %+v", err)  // Log full error
    return sendError(conn, "SESSION_CREATION_FAILED", sanitizeError(err), true)
}
```

3. **Agent spawn errors** (lines 288-294):
```go
if err != nil {
    s.logger.Printf("[ERROR] Agent spawn failed: %+v", err)  // Log full error
    return sendError(conn, "AGENT_SPAWN_FAILED", sanitizeError(err), true)
}
```

4. **Agent message routing errors** (lines 358-362):
```go
if err := agent.SendMessage(ctx, msg); err != nil {
    s.logger.Printf("[ERROR] Failed to route message: %+v", err)  // Log full error
    return sendError(conn, "MESSAGE_ROUTING_FAILED", sanitizeError(err), true)
}
```

**Benefits:**
- No internal details leaked to clients
- Server-side logs retain full error context
- Stable error codes for programmatic handling
- User-friendly, actionable messages

### Solution 2: WebSocket DoS Protection (#215)

**Phase 1: Basic Protections (P0)**

1. **Configure upgrader with origin checks:**
```go
// In NewGorillaUpgrader or wherever upgrader is created
upgrader := &websocket.Upgrader{
    CheckOrigin: checkOriginFunc,  // Provided by caller
    ReadBufferSize:  4096,   // Limit buffer sizes
    WriteBufferSize: 4096,
}
```

2. **Set connection limits immediately after upgrade:**
```go
// In handleClientConnection, after successful upgrade
conn.SetReadLimit(1024 * 1024)  // 1MB max message size
conn.SetReadDeadline(time.Now().Add(60 * time.Second))  // Initial 60s deadline
```

**Phase 2: Liveness Checks (P1)**

3. **Add pong handler to extend deadline:**
```go
conn.SetPongHandler(func(string) error {
    conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    return nil
})
```

4. **Send periodic pings:**
```go
// Start ping goroutine
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := conn.WriteControl(websocket.PingMessage, []byte{},
                time.Now().Add(10*time.Second)); err != nil {
                return  // Connection dead, exit goroutine
            }
        case <-ctx.Done():
            return
        }
    }
}()
```

**Phase 3: Rate Limiting (P2 - Future Work)**

Defer to future PR:
- IP-based connection rate limiting
- Per-connection message rate limiting
- Consider using golang.org/x/time/rate or similar

**Configuration:**
```go
const (
    maxMessageSize   = 1024 * 1024   // 1MB
    readDeadline     = 60 * time.Second
    writeDeadline    = 10 * time.Second
    pingInterval     = 30 * time.Second
)
```

## Implementation Steps

### Phase 1: Error Sanitization (#219)

1. Create `pkg/relay/error_sanitizer.go` with `sanitizeError()` function
2. Add error mapping for all known error types
3. Update `handleConnect` to use sanitized errors (lines 209-214)
4. Update `handleAgentRequest` to use sanitized errors (lines 288-294)
5. Update `handleAgentMessage` to use sanitized errors (lines 358-362)
6. Update `validateMessage` to use sanitized errors (lines 61-66)
7. Ensure all server-side logging includes full error details

### Phase 2: WebSocket Hardening (#215)

**P0 - Basic Protections:**
1. Update `NewGorillaUpgrader` to accept CheckOrigin function
2. Set ReadBufferSize and WriteBufferSize to 4096
3. Add `SetReadLimit(1MB)` after WebSocket upgrade
4. Add initial `SetReadDeadline(60s)` after upgrade

**P1 - Liveness Checks:**
5. Add `SetPongHandler` to extend deadline on activity
6. Create ping goroutine with 30s ticker
7. Ensure ping goroutine cleanup on connection close

**P2 - Rate Limiting (Future):**
8. Document rate limiting as follow-up work
9. Add TODO comments for future rate limiting implementation

### Phase 3: Testing

**Error Sanitization Tests:**
```go
func TestErrorSanitization(t *testing.T) {
    tests := []struct {
        name             string
        internalError    error
        shouldContain    string
        shouldNotContain []string
    }{
        {
            name:          "container setup failure",
            internalError: container.ErrContainerSetupFailed,
            shouldContain: "Agent container unavailable",
            shouldNotContain: []string{"ourocodus-agent", "make agent-image", "/Users", "Docker", "ErrContainerSetupFailed"},
        },
        {
            name:          "timeout",
            internalError: context.DeadlineExceeded,
            shouldContain: "timed out",
            shouldNotContain: []string{"context", "deadline", "DeadlineExceeded"},
        },
        {
            name:          "generic error",
            internalError: fmt.Errorf("internal database connection failed at host 192.168.1.100"),
            shouldContain: "internal error occurred",
            shouldNotContain: []string{"database", "192.168.1.100", "host"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            sanitized := sanitizeError(tt.internalError)
            assert.Contains(t, sanitized, tt.shouldContain)
            for _, shouldNot := range tt.shouldNotContain {
                assert.NotContains(t, sanitized, shouldNot)
            }
        })
    }
}
```

**WebSocket Hardening Tests:**
```go
func TestWebSocketReadLimit(t *testing.T) {
    // Setup: Create test server with read limit
    // Action: Send message larger than 1MB
    // Assert: Connection closed, no OOM
}

func TestWebSocketIdleTimeout(t *testing.T) {
    // Setup: Create test server
    // Action: Connect, wait 61s without activity
    // Assert: Connection closed by server
}

func TestWebSocketOriginValidation(t *testing.T) {
    // Setup: Create test server with origin check
    // Action: Connect from disallowed origin
    // Assert: Upgrade rejected with 403
}

func TestWebSocketPingPong(t *testing.T) {
    // Setup: Create test server
    // Action: Respond to pings with pongs
    // Assert: Connection stays alive beyond initial deadline
}
```

## Files to Modify

1. **pkg/relay/error_sanitizer.go** (NEW) - Error sanitization logic
2. **pkg/relay/server.go** - Update error handling call sites
3. **pkg/relay/websocket.go** - WebSocket hardening configuration
4. **cmd/relay/main.go** - Configure CheckOrigin function
5. **pkg/relay/error_sanitizer_test.go** (NEW) - Error sanitization tests
6. **pkg/relay/server_test.go** - WebSocket hardening tests

## Impact

### #219 - Error Sanitization
- **Before**: Internal details leaked (paths, build commands, stack traces)
- **After**: User-friendly messages, full details in server logs only
- **Backward Compatibility**: Full (API unchanged, messages improved)

### #215 - WebSocket Hardening
- **Before**: Vulnerable to OOM, FD exhaustion, CSWSH, flooding
- **After**: Protected by size limits, timeouts, origin checks, liveness
- **Backward Compatibility**: Full (valid clients unaffected)

## Risk Assessment

- **Risk Level:** LOW-MEDIUM
- **Security Impact:** HIGH (prevents info disclosure and DoS)
- **Breaking Changes:** None (hardening only)
- **Performance Impact:** Negligible (standard WebSocket overhead)

## Success Criteria

- ✅ No internal details in client error messages
- ✅ Full error context retained in server logs
- ✅ WebSocket read limit enforced (1MB)
- ✅ Idle connections timeout (60s)
- ✅ Invalid origins rejected
- ✅ Pings sent every 30s, pongs extend deadline
- ✅ All tests pass
- ✅ No regression in valid client behavior

## References

- Issue #219: Sanitize error messages returned to clients
- Issue #215: Harden WebSocket server against DoS attacks
- OWASP Error Handling: Don't leak implementation details
- OWASP WebSocket Security: https://owasp.org/www-community/vulnerabilities/WebSocket_security
- Industry precedent: Slack RTM, K8s exec proxy, GitHub API
