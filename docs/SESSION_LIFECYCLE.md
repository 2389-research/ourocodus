# Session Lifecycle - Phase 1

## Overview

A **session** represents one user conversation with one AI agent. Each session has:
- Unique session ID (UUID)
- WebSocket connection (user ↔ relay)
- ACP process (relay ↔ agent)
- Git worktree (agent workspace)

**Lifecycle:** Create → Active → Terminate → Cleanup

**State Storage:** In-memory only (no persistence across relay restarts)

---

## Session States

```
   ┌─────────┐
   │ CREATED │  User clicks "Start" in PWA
   └────┬────┘
        │
        v
   ┌─────────┐
   │SPAWNING │  Relay spawning ACP process
   └────┬────┘
        │
        v
   ┌─────────┐
   │  ACTIVE │  Process running, accepting messages
   └────┬────┘
        │
        v
   ┌───────────┐
   │TERMINATING│  User clicked "Stop" or error occurred
   └────┬──────┘
        │
        v
   ┌─────────┐
   │ CLEANED │  Resources freed, session removed
   └─────────┘
```

### State Definitions

**CREATED:**
- Session ID generated
- WebSocket connection established
- Session added to relay's session map
- No ACP process yet

**SPAWNING:**
- Git worktree being created
- ACP process being spawned
- stdin/stdout pipes being set up
- May fail and transition to TERMINATING

**ACTIVE:**
- ACP process running (cmd.Process.Pid > 0)
- stdin/stdout pipes open
- Accepting messages from WebSocket
- Relaying responses back to WebSocket

**TERMINATING:**
- SIGTERM sent to ACP process
- Waiting for graceful shutdown (5 second timeout)
- stdin/stdout pipes being closed
- WebSocket connection being closed

**CLEANED:**
- Session removed from session map
- All resources freed
- Session ID now invalid
- No further operations possible

---

## Session Data Structure

```go
type Session struct {
    // Immutable fields (set at creation)
    ID      string // UUID v4
    AgentID string // Role identifier ("auth", "db", "tests")

    // Mutable fields (guarded by internal mutex)
    state        SessionState
    worktreeDir  string
    handle       *Handle
    createdAt    time.Time
    lastActive   time.Time
    messageCount int

    mu sync.RWMutex
}

type Handle struct {
    WebSocket  WebSocketConn // Connection to PWA
    ACPClient  ACPClient     // ACP process wrapper
    CancelFunc func()        // Context cancel for shutdown
}

type SessionState string

const (
    StateCreated     SessionState = "CREATED"
    StateSpawning    SessionState = "SPAWNING"
    StateActive      SessionState = "ACTIVE"
    StateTerminating SessionState = "TERMINATING"
    StateCleaned     SessionState = "CLEANED"
)
```

All mutation flows through the `Manager`. Callers observe session state using accessors such as `session.GetState()` and `session.GetHandle()`. For implementation details see `pkg/relay/session/README.md`.

---

## Session Creation Flow

### 1. PWA Requests New Session

```json
// WebSocket message from PWA to Relay
{
  "type": "session:create",
  "agentId": "auth"
}
```

### 2. Relay Creates Session via Manager

```go
// session.Manager.Create attaches the WebSocket handle and persists the session
sess, err := manager.Create(ctx, agentID, ws)
if err != nil {
    return fmt.Errorf("create session failed: %w", err)
}
```

The manager generates the session ID, attaches the WebSocket handle, and persists the session through the injected store.

### 3. Relay Sends Acknowledgement

```json
// WebSocket message from Relay to PWA
{
  "type": "session:created",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "agentId": "auth",
  "timestamp": "2025-10-22T12:34:56Z"
}
```

### 4. Relay Initiates Spawn via Manager

```go
// session.Manager.BeginSpawn transitions CREATED → SPAWNING
if err := manager.BeginSpawn(ctx, sess.GetID()); err != nil {
    return fmt.Errorf("begin spawn failed: %w", err)
}

go func(sessionID string) {
    worktreeDir, err := createWorktree(agentID)
    if err != nil {
        manager.MarkTerminating(ctx, sessionID, fmt.Sprintf("worktree failed: %v", err))
        manager.CompleteCleanup(ctx, sessionID)
        return
    }

    acpClient, err := acp.NewClient(worktreeDir, os.Getenv("ANTHROPIC_API_KEY"))
    if err != nil {
        manager.MarkTerminating(ctx, sessionID, fmt.Sprintf("acp spawn failed: %v", err))
        manager.CompleteCleanup(ctx, sessionID)
        return
    }

    // session.Manager.AttachAgent transitions SPAWNING → ACTIVE
    if err := manager.AttachAgent(ctx, sessionID, worktreeDir, acpClient); err != nil {
        manager.MarkTerminating(ctx, sessionID, fmt.Sprintf("attach failed: %v", err))
        manager.CompleteCleanup(ctx, sessionID)
        return
    }

    session := manager.Get(sessionID)
    handle := session.GetHandle()
    handle.WebSocket.WriteJSON(map[string]interface{}{
        "type":      "session:ready",
        "sessionId": sessionID,
    })

    go readACPOutput(ctx, session)
}(sess.GetID())
```

### 5. PWA Receives Ready Notification

```json
// WebSocket message from Relay to PWA
{
  "type": "session:ready",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Now the user can send messages.**

---

## Active Session Operations

### Sending User Message

```json
// PWA → Relay (WebSocket)
{
  "type": "agent:message",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "content": "Create a user authentication module"
}
```

```go
handle := session.GetHandle()
if session.GetState() != StateActive {
    return fmt.Errorf("session not active")
}
if handle == nil || handle.ACPClient == nil {
    return fmt.Errorf("session not ready")
}

agentResp, err := handle.ACPClient.SendMessage(message.Content)
if err != nil {
    manager.MarkTerminating(ctx, session.GetID(), fmt.Sprintf("send failed: %v", err))
    manager.CompleteCleanup(ctx, session.GetID())
    return
}

payload := map[string]interface{}{
    "type":      "agent:response",
    "sessionId": session.GetID(),
    "content":   agentResp.Content,
    "timestamp": time.Now(),
}

if err := handle.WebSocket.WriteJSON(payload); err != nil {
    manager.MarkTerminating(ctx, session.GetID(), fmt.Sprintf("write failed: %v", err))
    manager.CompleteCleanup(ctx, session.GetID())
    return
}

_ = manager.IncrementMessageCount(ctx, session.GetID())
_ = manager.RecordHeartbeat(ctx, session.GetID())
```

---

## Session Termination Flow

### Initiated by User

```json
// PWA → Relay (WebSocket)
{
  "type": "session:stop",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Initiated by Error

```go
if err := manager.MarkTerminating(ctx, session.GetID(), fmt.Sprintf("error: %v", err)); err != nil {
    logger.Printf("Failed to mark terminating: %v", err)
}
if err := manager.CompleteCleanup(ctx, session.GetID()); err != nil {
    logger.Printf("Cleanup failed: %v", err)
}
```

### Initiated by WebSocket Disconnect

```go
// Relay detects disconnect in ReadMessage loop
if err != nil {
    logger.Printf("WebSocket disconnected for session %s", sessionID)
    if err := manager.MarkTerminating(ctx, sessionID, "client disconnected"); err != nil {
        logger.Printf("Failed to mark terminating: %v", err)
    }
    if err := manager.CompleteCleanup(ctx, sessionID); err != nil {
        logger.Printf("Cleanup failed: %v", err)
    }
}
```

### Termination Implementation

```go
func terminateSession(ctx context.Context, manager *session.Manager, logger session.Logger, sessionID string, reason string) {
    if err := manager.MarkTerminating(ctx, sessionID, reason); err != nil {
        logger.Printf("MarkTerminating failed: %v", err)
    }

    if err := manager.CompleteCleanup(ctx, sessionID); err != nil {
        logger.Printf("CompleteCleanup failed: %v", err)
    }
}
```

---

## Session Cleanup

### What Gets Cleaned

**In Memory:**
- Session object removed from `relay.sessions` map
- WebSocket connection closed
- ACP client stdin/stdout closed

**Processes:**
- ACP process terminated (SIGTERM → wait 5s → SIGKILL if needed)

**Git Worktrees:**
- **NOT cleaned up in Phase 1** (worktrees persist for inspection)
- **Future:** Add `--cleanup` flag to remove worktrees on exit

### What Persists

**After session termination:**
- Git worktree directory (`agent/auth/`, `agent/db/`, etc.)
- All commits made by agent
- All files created by agent

**Rationale:** User may want to inspect agent's work after session ends.

**Manual Cleanup:**
```bash
# Remove all agent worktrees
./scripts/cleanup-worktrees.sh

# Or manually:
rm -rf agent/
git worktree prune
```

---

## Concurrency and Thread Safety

### Session Map

```go
// In relay.go
type Relay struct {
    sessions sync.Map  // map[string]*Session (thread-safe)
}
```

**Operations:**
- `sessions.Store(id, session)` - Add new session
- `sessions.Load(id)` - Get session by ID
- `sessions.Delete(id)` - Remove session
- `sessions.Range(func)` - Iterate all sessions

### Session State

```go
state := sess.GetState()
if state != session.StateActive {
    return errors.New("session not active")
}
```

**Protected by mutex:**
- State transitions
- ACPClient pointer assignment
- WorktreeDir assignment

**NOT protected (no concurrent writes):**
- ID, AgentID (immutable after creation)
- CreatedAt (written once)

---

## Session Limits (Phase 1)

### Hard Limits

**Max concurrent sessions:** 3 (one per agent)
- Enforced by agent ID uniqueness
- User cannot start second "auth" session while first is active

**Max session duration:** None (runs until user stops or error occurs)

**Max messages per session:** None

**Max message size:** 1MB (enforced at WebSocket layer)

### Soft Limits (Warnings)

**Session idle timeout:** None in Phase 1
- **Future:** Phase 2 adds 30 minute idle timeout

**Message rate:** None enforced
- **Future:** Add 10 messages/second rate limit

---

## Session Observability

### Logs

**Session Created:**
```json
{
  "level": "INFO",
  "timestamp": "2025-10-22T12:34:56Z",
  "event": "session.created",
  "sessionId": "uuid",
  "agentId": "auth"
}
```

**Session Ready:**
```json
{
  "level": "INFO",
  "event": "session.ready",
  "sessionId": "uuid",
  "agentId": "auth",
  "worktreeDir": "/path/to/agent/auth"
}
```

**Message Sent:**
```json
{
  "level": "INFO",
  "event": "session.message_sent",
  "sessionId": "uuid",
  "messageCount": 5
}
```

**Session Terminated:**
```json
{
  "level": "INFO",
  "event": "session.terminated",
  "sessionId": "uuid",
  "reason": "client disconnected",
  "duration": "15m32s",
  "messageCount": 12
}
```

### Metrics (Future)

**Phase 2 additions:**
- Active session count (gauge)
- Session duration histogram
- Messages per session histogram
- Session error rate by reason

---

## Known Issues and Limitations

### No Session Persistence

**Issue:** Relay restart = all sessions lost

**Impact:** User must recreate sessions after relay crashes

**Mitigation:** Phase 1 POC, acceptable for testing

**Future:** Phase 4 adds SQLite event store

---

### No Session Reconnection

**Issue:** WebSocket disconnect = session terminates

**Impact:** Browser refresh loses conversation history

**Mitigation:** User can restart session, worktree persists

**Future:** Phase 2 adds session resumption

---

### Race Condition: Terminate During Spawn

**Issue:** User can click "Stop" while agent is still spawning

**Scenario:**
1. User clicks "Start" (session enters SPAWNING)
2. User immediately clicks "Stop"
3. Terminate called before ACPClient assigned

**Mitigation:**
```go
func (s *Session) terminate(reason string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.ACPClient != nil {
        s.ACPClient.Close() // Safe, client exists
    } else if s.State == StateSpawning {
        // Mark for cancellation, spawn goroutine will check
        s.State = StateTerminating
    }
}
```

---

### Session ID Collision

**Issue:** UUID collision (astronomically unlikely but theoretically possible)

**Mitigation:** Check if ID exists before adding to map

```go
sessionID := uuid.New().String()
for {
    _, exists := relay.sessions.Load(sessionID)
    if !exists {
        break
    }
    sessionID = uuid.New().String() // Generate new ID
}
```

---

## Testing Session Lifecycle

### Manual Test Cases

- [ ] Create session → Verify ready notification
- [ ] Send message → Verify agent response
- [ ] Stop session → Verify clean termination
- [ ] Create session → Disconnect WebSocket → Verify ACP process killed
- [ ] Create 3 sessions (all agents) → Stop 1 → Verify others unaffected
- [ ] Try to create duplicate agent session → Verify rejected
- [ ] Stop session during spawn → Verify no zombie process
- [ ] Relay restart → Verify all sessions lost (expected)

### Automated Tests (Issue #13)

```go
func TestSessionLifecycle(t *testing.T) {
    relay := NewRelay()
    go relay.Start()
    defer relay.Stop()

    // Connect WebSocket
    ws := connectWebSocket(t, "ws://localhost:3000/ws")
    defer ws.Close()

    // Create session
    ws.WriteJSON(map[string]interface{}{
        "type": "session:create",
        "agentId": "auth",
    })

    // Wait for ready
    var msg map[string]interface{}
    ws.ReadJSON(&msg)
    assert.Equal(t, "session:created", msg["type"])
    sessionID := msg["sessionId"].(string)

    ws.ReadJSON(&msg)
    assert.Equal(t, "session:ready", msg["type"])

    // Send message
    ws.WriteJSON(map[string]interface{}{
        "type": "agent:message",
        "sessionId": sessionID,
        "content": "hello",
    })

    // Read response
    ws.ReadJSON(&msg)
    assert.Equal(t, "agent:response", msg["type"])

    // Stop session
    ws.WriteJSON(map[string]interface{}{
        "type": "session:stop",
        "sessionId": sessionID,
    })

    // Verify terminated
    ws.ReadJSON(&msg)
    assert.Equal(t, "session:terminated", msg["type"])
}
```

---

## Session Lifecycle Diagram

```
USER ACTION          RELAY STATE           ACP PROCESS        GIT WORKTREE
───────────────────────────────────────────────────────────────────────────

"Start Agent" ──────> CREATED
                      │
                      ├─> Create UUID
                      ├─> Store session
                      └─> Send "created"
                      │
                      v
                      SPAWNING ──────────> (spawning)      ───> Create worktree
                      │                                          on branch "auth"
                      │                     │
                      │                     v
                      │                   Running
                      │                   (pid 12345)
                      v
"Send message" ────> ACTIVE
                      │
                      ├─> Relay to stdin ─> Process
                      │                     │
                      │                     └─> Response
                      └─< Read from stdout <─
                      │
                      └─> Forward to WS


"Stop" ─────────────> TERMINATING
                      │
                      ├─> SIGTERM ────────> Process exits  ───> Files committed
                      │                     (graceful)          (by ACP)
                      │   (wait 5s)
                      │
                      ├─> SIGKILL ────────> Force quit
                      │   (if needed)       (if stuck)
                      │
                      └─> Close WS
                      │
                      v
                      CLEANED
                      │
                      └─> Remove from map
                                                             ───> Worktree remains
                                                                  (not deleted)
```

---

## Summary

**Session = User ↔ Agent Conversation**

**Lifecycle:** Create → Spawn → Active → Terminate → Cleanup

**Storage:** In-memory only (no persistence)

**Concurrency:** Thread-safe with sync.Map and mutexes

**Cleanup:** Processes terminated, worktrees preserved

**Future:** Add persistence, reconnection, timeouts in later phases
