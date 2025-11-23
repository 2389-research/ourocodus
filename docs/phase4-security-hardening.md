# Phase 4: Security Hardening for Agent Adoption

This document describes the security features added in Phase 4 to secure the agent adoption flow.

## Overview

Phase 4 adds three layers of security to prevent unauthorized access to CLI-spawned agents:

1. **🔐 Token Authentication** - Cryptographic tokens required for agent attachment
2. **📝 Audit Logging** - Complete audit trail of all security events
3. **🚦 Rate Limiting** - Protection against brute-force and DoS attacks

## Features

### Token Authentication

**What it does:**
- Generates a 256-bit cryptographic token when an agent is spawned
- Requires this token for all agent attachment operations
- Uses constant-time comparison to prevent timing attacks

**How it works:**
```bash
# Spawn agent - token is generated automatically
$ bin/agentd spawn my-agent

🔐 Attach Token:
   hK8xMzQ5Nj...  # 44-character base64-encoded token
```

**Security properties:**
- ✅ 256-bit entropy (32 bytes from `crypto/rand`)
- ✅ Base64URL-encoded for safe transmission
- ✅ Stored with 0600 permissions in `.agentd/session/<agent-id>.token`
- ✅ Constant-time verification prevents timing side-channel attacks
- ✅ Three error types: missing token, invalid token, token file not found

**Implementation:**
- Token generation: `cmd/agentd/cmd_spawn.go`
- Token verification: `pkg/relay/session/token.go`
- Token validation: `pkg/relay/session/lease.go` (validateAgentID prevents path traversal)

### Audit Logging

**What it does:**
- Logs all security-sensitive operations in structured JSON format
- Tracks who attached/detached agents and when
- Records all authentication failures with detailed context

**Log format:**
```json
{
  "timestamp": "2025-11-23T02:00:00Z",
  "type": "agent:attach",
  "userId": "user-session-abc123",
  "agentId": "my-agent",
  "success": true
}
```

**Event types:**
- `agent:attach` - Agent attachment attempt (success or failure)
- `agent:detach` - Agent detachment (always successful if attached)
- Authentication failures include `error` field with reason

**Viewing logs:**
```bash
# View all audit events
bin/relay 2>&1 | grep AUDIT

# Pretty-print with jq
bin/relay 2>&1 | grep AUDIT | jq .

# Filter for specific agent
bin/relay 2>&1 | grep AUDIT | jq 'select(.agentId == "my-agent")'

# Filter for failures
bin/relay 2>&1 | grep AUDIT | jq 'select(.success == false)'
```

**Implementation:**
- Audit logger: `pkg/relay/audit/logger.go`
- Integration: `pkg/relay/session/models.go` (AttachAgent/DetachAgent)

### Rate Limiting

**What it does:**
- Limits attachment attempts per user session to prevent brute-force attacks
- Uses token bucket algorithm with configurable burst and refill rate
- Prevents both brute-force token guessing and DoS attacks

**Configuration:**
- **Burst capacity**: 10 requests (allows brief bursts of legitimate activity)
- **Refill rate**: 1 request/second (sustained rate limit)

**Behavior:**
```
User makes 15 rapid attach attempts:
  Requests 1-10:  Processed (consume 10 tokens from burst capacity)
  Request  11:    RATE_LIMIT_EXCEEDED (bucket empty, need to wait)
  Request  12-15: RATE_LIMIT_EXCEEDED (1 token refills per second)

After 5 seconds: 5 tokens available again
```

**Testing rate limits:**
See `scripts/demo-phase4-security.sh` for a demonstration script.

**Implementation:**
- Rate limiter: `pkg/relay/ratelimit/limiter.go`
- Integration: `pkg/relay/handlers_agent_adoption.go` (checks before attachment)

## Security Properties

| Property | Implementation | Prevents |
|----------|---------------|----------|
| **Authentication** | 256-bit cryptographic tokens | Unauthorized attachment to agents |
| **Timing Attack Protection** | Constant-time token comparison | Token guessing via timing side-channels |
| **Brute Force Protection** | Rate limiting (1 req/sec after burst) | Token guessing attacks |
| **DoS Protection** | Per-session rate limiting | Resource exhaustion attacks |
| **Audit Trail** | Structured JSON logging | Undetected security breaches |
| **Path Traversal Protection** | AgentID validation | Arbitrary file access via tokens |

## Demo

Run the comprehensive demo script:

```bash
./scripts/demo-phase4-security.sh
```

The demo walks through:
1. ✓ Spawning an agent with token generation
2. ✓ Verifying secure token storage
3. ✓ Testing valid and invalid token authentication
4. ✓ Viewing audit logs
5. ✓ Demonstrating rate limiting
6. ✓ Security properties summary

## Manual Testing

### Test Valid Token Authentication

```bash
# 1. Spawn agent
bin/agentd spawn test-agent
# Copy the displayed token

# 2. Start relay
bin/relay

# 3. Open browser to http://localhost:8080
# 4. Try to attach to agent "test-agent"
# 5. Paste the token
# Expected: Attachment succeeds
```

### Test Invalid Token

```bash
# Follow steps above but use a wrong token
# Expected: INVALID_TOKEN error
```

### Test Rate Limiting

```javascript
// In browser DevTools console:
for (let i = 0; i < 15; i++) {
  fetch('/api/attach', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
      type: 'agent:attach',
      agentId: 'test-agent',
      userSessionId: 'user-' + Date.now(),
      token: 'invalid-token-' + i
    })
  }).then(r => r.json())
    .then(d => console.log(`Attempt ${i+1}:`, d.code || 'success'));
}

// Expected:
// - First ~10 attempts: INVALID_TOKEN (authentication fails)
// - Next 5 attempts: RATE_LIMIT_EXCEEDED
```

## Token Lifecycle

### Token Generation

Tokens are automatically generated when you spawn an agent:

```bash
$ bin/agentd spawn my-agent
```

The token is:
- Displayed once in the terminal output (copy it!)
- Stored in `.agentd/session/my-agent.token` (file permissions: 0600)
- Never logged or stored in any other location

### Token Storage Location

```
.agentd/
└── session/
    └── <agent-id>.token   # 44-character base64-encoded token
```

### Token Lifecycle Events

| Event | When | Effect |
|-------|------|--------|
| **Generated** | Agent spawn | New token created and stored |
| **Verified** | Agent attach | Token compared with stored value |
| **Cleaned up** | Agent stop | Token file deleted |

### Viewing Token Status

```bash
# Check if token exists
ls -la .agentd/session/<agent-id>.token

# View token file permissions
stat .agentd/session/<agent-id>.token

# Read token (if needed for debugging)
cat .agentd/session/<agent-id>.token
```

⚠️ **Security Note**: Never log tokens or commit them to version control!

## Error Handling

### Token Errors

| Error Code | Meaning | Resolution |
|------------|---------|------------|
| `MISSING_TOKEN` | No token provided | Include token in attach request |
| `INVALID_TOKEN` | Wrong token | Use correct token from spawn output |
| `RATE_LIMIT_EXCEEDED` | Too many attempts | Wait before retrying (1 req/sec) |
| `AGENT_NOT_FOUND` | Agent doesn't exist | Verify agent ID and ensure it's spawned |

### Common Issues

**"Token file not found"**
- Agent may have been stopped and restarted (token is deleted on stop)
- Solution: Check if agent is still running, respawn if needed

**"Rate limit exceeded"**
- Too many attachment attempts in short time
- Solution: Wait 10 seconds and try again

**"Invalid token"**
- Token may have been copied incorrectly
- Solution: Copy full token from spawn output, including all characters

## Implementation Details

### Token Format

```
Format: Base64URL-encoded 256-bit random value
Example: hK8xMzQ5NjEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2
Length: 44 characters (32 bytes → 43 base64 chars + 1 padding)
```

### Token Comparison

Uses `crypto/subtle.ConstantTimeCompare()` to prevent timing attacks:

```go
// Constant-time comparison prevents attackers from guessing tokens
// by measuring response time differences
if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
    return ErrInvalidAttachToken
}
```

### Rate Limiter Algorithm

Token bucket with per-session tracking:

```
Capacity: 10 tokens
Refill:   1 token/second

Time  Tokens  Action
0s    10      Allow (9 remaining)
0s    9       Allow (8 remaining)
...
0s    1       Allow (0 remaining)
1s    0       Rate limited (wait for refill)
2s    1       Allow (refilled 1 token)
```

## Test Coverage

### Unit Tests

- ✅ Token generation (various scenarios)
- ✅ Token verification (valid, invalid, missing)
- ✅ Audit logging (all event types)
- ✅ Rate limiting (burst, refill, concurrent access)
- ✅ Path traversal protection (agent ID validation)

### Integration Tests

- ✅ Full attach flow with token
- ✅ Multiple users with independent rate limits
- ✅ Token cleanup on agent stop

Run tests:
```bash
make test
```

## Security Considerations

### Threats Mitigated

✅ **Unauthorized Agent Access**
- Attack: User tries to attach to agent without permission
- Mitigation: Token authentication required

✅ **Token Guessing**
- Attack: Brute-force token values
- Mitigation: Rate limiting (1 attempt/sec after burst)

✅ **Timing Attacks**
- Attack: Measure response times to guess tokens
- Mitigation: Constant-time comparison

✅ **DoS Attacks**
- Attack: Flood server with attachment requests
- Mitigation: Per-session rate limiting

✅ **Path Traversal**
- Attack: Use malicious agent IDs like `../../etc/passwd`
- Mitigation: Agent ID validation before file operations

### Best Practices

1. **Never log tokens** - Tokens should only appear in spawn output
2. **Copy tokens immediately** - They're displayed once and stored in a file
3. **Secure token storage** - `.agentd/session/` directory should have restricted permissions
4. **Monitor audit logs** - Watch for suspicious patterns (many failures, rate limiting)
5. **Clean up old tokens** - Delete `.agentd/session/*.token` files for stopped agents

## Future Enhancements

Potential improvements for future phases:

- Token rotation (manual or automatic)
- Token revocation without agent restart
- Multiple tokens per agent (for team access)
- Token expiration (TTL)
- Audit log API endpoint
- Real-time audit event streaming

## References

- PR #274: Phase 4 Security Hardening for Agent Adoption
- Related: Phase 3 ACP Bridge (foundation for agent attachment)
- Security review: `docs/PR274_REVIEW_TRIAGE.md`
