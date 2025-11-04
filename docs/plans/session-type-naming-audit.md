# Session Type Naming Audit - Critical Task

**Priority:** CRITICAL
**Date:** 2025-11-04
**Branch:** feature/container-session-core-101 (PR #140)

## Requirement

**User Requirement:** "ensure everywhere we talk about a session in our documentation we are extremely and always explicit about the type of session, this should carry over to our naming conventions in the code as well."

**Specific Mandate:** "sessionId is a no-go it should be a userSessionId or agentSessionId or containerSessionId etc..."

## Three Session Types

1. **UserSession** - One per WebSocket connection (pkg/relay/session)
2. **AgentSession** - One per agent instance within a UserSession (pkg/relay/session)
3. **ContainerSession** - One per Docker container backing an agent (pkg/containersession - Phase 2)

## Naming Rules

### ❌ NEVER USE (Ambiguous)
- `sessionID` or `session_id`
- `session` (bare variable name)
- `sess`
- `id` (context unclear)

### ✅ ALWAYS USE (Explicit)
- `userSessionID` or `user_session_id`
- `agentSessionID` or `agent_session_id` (if we add this concept later)
- `containerSessionID` or `container_session_id`
- `userSession` (variable holding UserSession)
- `agentSession` (variable holding AgentSession)
- `containerSession` (variable holding ContainerSession)

## Systematic Audit Plan

### 1. Function Parameters

**Search:** All function signatures with `sessionID` parameter

```bash
grep -rn "func.*sessionID" pkg/
grep -rn "sessionID string" pkg/
grep -rn "sessionID," pkg/
```

**Update:** Determine from context which session type, then rename

Examples:
- `SpawnAgent(ctx, sessionID, agentID, workspace)` → `SpawnAgent(ctx, userSessionID, agentID, workspace)`
- `GetAgent(sessionID, agentID)` → `GetAgent(userSessionID, agentID)`
- `CreateUserSession(ctx, ws)` returns string → return `userSessionID string`

### 2. Variable Names

**Search:** All variable declarations

```bash
grep -rn "session :=" pkg/
grep -rn "session =" pkg/
grep -rn "var session" pkg/
```

**Update:**
- `session := m.store.Get(sessionID)` → `userSession := m.store.Get(userSessionID)`
- `if session == nil` → `if userSession == nil`
- `session.GetAgent()` → `userSession.GetAgent()`

### 3. Struct Fields and JSON Tags

**Search:**
```bash
grep -rn '"sessionId"' pkg/
grep -rn '`json:"sessionId"`' pkg/
grep -rn 'SessionID string' pkg/
```

**Update:**
- `SessionID string \`json:"sessionId"\`` → `UserSessionID string \`json:"userSessionId"\``
- Consider if field actually refers to user session, agent session, or container session

### 4. Method Receivers

**Search:**
```bash
grep -rn "func (.*) .*sessionID" pkg/
```

**Update:** Method parameters from `sessionID` to `userSessionID` where applicable

### 5. Log Statements

**Search:**
```bash
grep -rn 'session=%s' pkg/
grep -rn 'sessionID' pkg/ | grep -i "printf\|logger"
```

**Update:**
- `"session=%s"` → `"userSession=%s"` or appropriate type
- `"Agent spawned: session=%s"` → `"Agent spawned: userSession=%s"`

### 6. Error Messages

**Search:**
```bash
grep -rn "session %s" pkg/
grep -rn "Session" pkg/ | grep -i "error\|errorf"
```

**Update:**
- `"session %s not found"` → `"user session %s not found"`
- `ErrSessionNotFound` → Consider renaming to `ErrUserSessionNotFound`

### 7. Comments and Documentation

**Search:**
```bash
grep -rn "// .*session" pkg/
grep -rn "session:" pkg/
```

**Update:** All comments to specify session type explicitly

### 8. Test Files

**Search:**
```bash
grep -rn "sessionID" pkg/relay/session/*_test.go
grep -rn '"session-' pkg/relay/session/*_test.go
```

**Update:**
- Test variables: `sessionID` → `userSessionID`
- Test fixture IDs: `"session-123"` → `"user-session-123"` (or keep if clear from context)

### 9. API Handlers and Server Code

**Search:**
```bash
grep -rn "sessionID" pkg/relay/server.go
grep -rn "sessionID" pkg/relay/handlers*.go
grep -rn "sessionID" pkg/relay/message.go
```

**Update:** All request/response handling with explicit session types

### 10. Documentation Examples

Already updated in previous commits, but verify:
- docs/ARCHITECTURE.md
- docs/SESSION_LIFECYCLE.md
- docs/ACP.md
- docs/PWA.md
- docs/prd/*.md

## Context-Specific Decisions

### Manager Methods (pkg/relay/session/manager.go)

These methods work with UserSession:
- `CreateUserSession(ctx, ws)` - Returns userSessionID ✅ (already clear)
- `SpawnAgent(ctx, sessionID, agentID, workspace)` - First param is userSessionID
- `GetAgent(sessionID, agentID)` - First param is userSessionID
- `TerminateAgent(ctx, sessionID, agentID)` - First param is userSessionID
- `TerminateUserSession(ctx, sessionID)` - Param is userSessionID

**Decision:** All `sessionID` parameters in Manager should be `userSessionID`

### Store Interface (pkg/relay/session/store.go)

```go
type Store interface {
    Get(sessionID string) *UserSession
    Add(session *UserSession)
    Remove(sessionID string)
    // ...
}
```

**Decision:**
- `Get(sessionID string)` → `Get(userSessionID string)`
- `Remove(sessionID string)` → `Remove(userSessionID string)`

### Event Publisher (pkg/relay/session/publisher.go)

```go
type Publisher interface {
    PublishAgentSpawned(ctx, sessionID, agentID, workspace string) error
    PublishAgentTerminated(ctx, sessionID, agentID string, exitCode int) error
}
```

**Decision:** All `sessionID` → `userSessionID` (these events are for agents in user sessions)

## Verification Commands

After updates, run these to verify:

```bash
# Should return NO results (except in comments explaining what NOT to do)
grep -r "\bsessionID\b" pkg/relay/session/ | grep -v "// " | grep -v "_test.go"
grep -r "\bsession_id\b" pkg/relay/session/ | grep -v "// "
grep -r "var session " pkg/relay/session/
grep -r "session :=" pkg/relay/session/
grep -r "session =" pkg/relay/session/ | grep -v "userSession ="

# These should have explicit types
grep -r "session\." pkg/relay/session/ | head -20  # Manual review

# JSON fields should be explicit
grep -r '"sessionId"' pkg/
```

## Files to Update (Priority Order)

### High Priority (Core Session Management)
1. ✅ pkg/relay/session/models.go - Partially done (AgentID complete, session naming needed)
2. ✅ pkg/relay/session/manager.go - Partially done (agentID complete, session naming needed)
3. [ ] pkg/relay/session/store.go - Interface and implementations
4. [ ] pkg/relay/session/store_memory.go - Implementation
5. [ ] pkg/relay/session/errors.go - Error constants and messages
6. [ ] pkg/relay/session/publisher.go - Event interface

### Medium Priority (Integration Points)
7. [ ] pkg/relay/server.go - HTTP/WebSocket handlers
8. [ ] pkg/relay/message.go - Message types and handling
9. [ ] pkg/relay/handlers*.go (if they exist)

### Medium Priority (Tests)
10. [ ] pkg/relay/session/manager_test.go
11. [ ] pkg/relay/session/models_test.go
12. [ ] pkg/relay/session/store_test.go
13. [ ] All other *_test.go files

### Low Priority (Supporting)
14. [ ] pkg/relay/session/README.md
15. [ ] pkg/relay/session/cleaner.go
16. [ ] Any other support files

## Expected Breaking Changes

1. **Function signatures** - All callers must update parameter names
2. **JSON message format** - PWA and any API clients must update field names
3. **Event topic names** - NATS topics may need updates (if they use sessionId)
4. **Log parsing** - Any log analysis tools expecting "session=" instead of "userSession="

## Sign-Off Criteria

✅ Ready when:
1. Zero occurrences of bare `sessionID` parameter names in core packages
2. Zero occurrences of bare `session` variable names (must be `userSession`, `agentSession`, etc.)
3. All JSON tags explicitly specify session type
4. All log statements use explicit session type names
5. All error messages specify which session type
6. All tests pass with new naming
7. Documentation examples match code
8. PR #140 description documents breaking changes

## Notes

- This is a BREAKING CHANGE that must be coordinated with PR #140 merge
- All callers of session management APIs will need updates
- PWA client must update JSON field names
- Consider this foundational for Phase 2 when ContainerSession is introduced
- Better to enforce this now than retrofit later
