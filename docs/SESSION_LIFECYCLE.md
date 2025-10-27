# Session Lifecycle - Phase 1 (PR7 Implementation)

## Overview

**UserSession** is a container for 0-N agent processes. Each agent is independent and has its own lifecycle.

- **UserSession**: WebSocket connection container with 2 states (ACTIVE, TERMINATED)
- **AgentSession**: Individual ACP process with 4 states (SPAWNING, ACTIVE, FAILED, TERMINATED)

**Key Design Principles:**
- UserSession can have 0 to N agents
- Agents can be spawned/terminated independently
- Agent failure doesn't terminate the UserSession
- Roles are dynamic (user-specified, not hardcoded)

**State Storage:** In-memory only (no persistence across relay restarts)

---

## Session Architecture

```
UserSession (ID: uuid)
├── State: ACTIVE | TERMINATED
├── WebSocket Connection (to PWA)
├── Created At: time.Time
├── Last Active: time.Time
└── Agents: map[role]AgentSession
    ├── AgentSession (role: "auth")
    │   ├── State: SPAWNING | ACTIVE | FAILED | TERMINATED
    │   ├── Workspace: string (git worktree path)
    │   ├── ACPClient: *acp.Client
    │   ├── Created At: time.Time
    │   ├── Last Active: time.Time
    │   └── Error: string (if FAILED)
    │
    ├── AgentSession (role: "db")
    │   └── ...
    │
    └── AgentSession (role: "tests")
        └── ...
```

---

## UserSession States

```
   ┌─────────┐
   │ ACTIVE  │  Session created, WebSocket connected
   └────┬────┘
        │ Can spawn agents
        │ Can send messages
        │ Can terminate agents
        │
        v
   ┌─────────────┐
   │ TERMINATED  │  Session stopped, resources cleaned
   └─────────────┘
```

**ACTIVE:**
- WebSocket connection established
- Can spawn 0-N agents
- Agents can be added/removed dynamically
- Session stays ACTIVE even if all agents fail

**TERMINATED:**
- All agents terminated
- WebSocket closed
- Session removed from store
- Resources freed

---

## AgentSession States

```
   ┌──────────┐
   │ SPAWNING │  Worktree + ACP process being created
   └────┬─────┘
        │
        ├──> Success
        │         v
        │    ┌────────┐
        │    │ ACTIVE │  Process running, accepting messages
        │    └───┬────┘
        │        │
        │        v
        │    ┌─────────────┐
        │    │ TERMINATED  │  Gracefully stopped
        │    └─────────────┘
        │
        └──> Failure
                  v
             ┌────────┐
             │ FAILED │  Spawn/connect failed
             └───┬────┘
                 │
                 v
             ┌─────────────┐
             │ TERMINATED  │  Cleaned up
             └─────────────┘
```

**SPAWNING:**
- Workspace directory being created
- ACP process being spawned
- May transition to ACTIVE or FAILED
- Session remains ACTIVE during spawn

**ACTIVE:**
- ACP process running (has ACPClient)
- Can send/receive messages
- Workspace ready for git operations

**FAILED:**
- Spawn failed or process crashed
- Error message recorded
- UserSession stays ACTIVE
- Can retry with new agent

**TERMINATED:**
- Agent stopped gracefully
- ACP process terminated
- Resources cleaned
- Agent removed from session

---

## UserSession Creation Flow

### 1. Create UserSession

```go
manager.CreateUserSession(ctx, websocketConn)
```

**What happens:**
1. Generate unique session ID
2. Create UserSession with ACTIVE state
3. Attach WebSocket connection
4. Store in session manager
5. Return UserSession

**Result:** Empty session with 0 agents, ready to spawn agents

### 2. WebSocket Notification

```json
{
  "type": "session:created",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-10-24T12:34:56Z"
}
```

---

## Agent Spawn Flow

### 1. Spawn Agent Request

```go
manager.SpawnAgent(ctx, sessionID, role, workspace)
```

**Validation:**
- Session must exist
- Session must be ACTIVE
- Role must not already exist
- Role and workspace must be non-empty

### 2. Agent Creation (SPAWNING)

**What happens:**
1. Create AgentSession in SPAWNING state
2. Create workspace directory (0o750 permissions)
3. Spawn ACP client process
4. Transition to ACTIVE on success
5. Transition to FAILED on error

### 3. Agent Ready

```json
{
  "type": "agent:ready",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "role": "auth",
  "workspace": "/path/to/agent/auth"
}
```

### 4. Spawn Failure (Independent)

**If agent fails to spawn:**
- Agent transitions to FAILED state
- Error message recorded in agent
- **UserSession remains ACTIVE**
- Other agents unaffected
- Can retry or spawn different agent

---

## Active Session Operations

### Send Message to Agent

```json
// PWA → Relay
{
  "type": "agent:message",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "role": "auth",
  "content": "Create a user authentication module"
}
```

**Validation:**
- Session must exist and be ACTIVE
- Agent must exist for given role
- Agent must be in ACTIVE state

**Response:**
```json
// Relay → PWA
{
  "type": "agent:response",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "role": "auth",
  "content": "I've created auth.go with JWT implementation...",
  "timestamp": "2025-10-24T12:34:57Z"
}
```

### List Agents

```go
agents, err := manager.ListAgents(sessionID)
```

Returns all agents for the session with their current states.

### Get Specific Agent

```go
agent, err := manager.GetAgent(sessionID, role)
```

Returns single agent by role.

---

## Agent Termination Flow

### 1. Terminate Agent Request

```go
manager.TerminateAgent(ctx, sessionID, role)
```

**What happens:**
1. Find agent by role
2. Close ACP client connection
3. Set agent state to TERMINATED
4. Remove agent from session
5. **UserSession remains ACTIVE**

**Idempotent:** Safe to call multiple times

### 2. Agent Terminated Notification

```json
{
  "type": "agent:terminated",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "role": "auth",
  "reason": "user requested"
}
```

### 3. Other Agents Unaffected

**Independent lifecycles:**
- Terminating "auth" doesn't affect "db" agent
- Session continues with remaining agents
- Can spawn new agents after termination

---

## UserSession Termination Flow

### 1. Terminate Session Request

```go
manager.TerminateUserSession(ctx, sessionID)
```

**What happens:**
1. Mark session as TERMINATED
2. Terminate all active agents in parallel (with timeout)
3. Close WebSocket connection
4. Run cleaner (workspace cleanup)
5. Remove session from store

**Parallel Termination:**
- All agents closed concurrently
- 5-second timeout per agent
- Continues even if some agents fail to close

### 2. Session Terminated Notification

```json
{
  "type": "session:terminated",
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "reason": "user requested",
  "agentsTerminated": 3
}
```

### 3. Cleanup

**What gets cleaned:**
- All ACP processes terminated
- WebSocket connection closed
- Session removed from memory

**What persists:**
- Git worktrees (for inspection)
- Committed work

---

## Error Handling

### Agent Spawn Failure

**Scenario:** Workspace creation fails

**Behavior:**
- Agent enters FAILED state
- Error message recorded
- Session remains ACTIVE
- User can retry or spawn different agent

### Agent Process Crash

**Scenario:** ACP process exits unexpectedly

**Behavior:**
- Agent marked as FAILED
- Error logged
- Session remains ACTIVE
- Other agents unaffected

### Session Termination During Spawn

**Scenario:** User terminates session while agent is SPAWNING

**Behavior:**
- Spawn continues or is cancelled
- Agent transitions to TERMINATED
- Session cleanup proceeds normally

---

## Concurrency and Thread Safety

### Session Manager

```go
type Manager struct {
    store         Store  // Thread-safe operations
    clientFactory ClientFactory
    // ... other dependencies
}
```

**Thread-safe operations:**
- CreateUserSession
- SpawnAgent
- TerminateAgent
- TerminateUserSession
- Get/List/Count

### UserSession

```go
type UserSession struct {
    // Immutable
    ID        string
    createdAt time.Time

    // Mutable (protected by mu)
    state      UserSessionState
    agents     map[string]*AgentSession
    lastActive time.Time

    mu sync.RWMutex
}
```

### AgentSession

```go
type AgentSession struct {
    // Immutable
    Role      string
    Workspace string
    createdAt time.Time

    // Mutable (protected by mu)
    state      AgentState
    acpClient  ACPClient
    lastActive time.Time
    errorMsg   string

    mu sync.RWMutex
}
```

---

## Known Limitations (POC Trade-offs)

### No Session Persistence

**Issue:** Relay restart = all sessions lost

**Impact:** User must recreate sessions

**Mitigation:** Acceptable for Phase 1 POC

**Future:** Phase 4 adds SQLite event store

---

### No Session Reconnection

**Issue:** WebSocket disconnect = session terminates

**Impact:** Browser refresh loses session

**Mitigation:** Worktrees persist for inspection

**Future:** Phase 2 adds session resumption

---

### Agent Lifecycle Complexity

**Issue:** Managing multiple independent agent lifecycles

**Current:** Each agent has own state machine

**Complexity:** Session can have mix of SPAWNING/ACTIVE/FAILED agents

**Benefit:** Isolation - one agent failure doesn't affect others

---

## Testing Scenarios

### Happy Path
- [ ] Create session → Verify ACTIVE state
- [ ] Spawn agent → Verify agent ACTIVE
- [ ] Send message → Verify response
- [ ] Terminate agent → Verify agent removed, session ACTIVE
- [ ] Terminate session → Verify all cleaned

### Multiple Agents
- [ ] Spawn 3 agents → All ACTIVE
- [ ] Terminate 1 agent → Others unaffected
- [ ] Send messages to remaining agents
- [ ] Terminate session → All agents cleaned

### Failure Scenarios
- [ ] Spawn agent with invalid workspace → Agent FAILED, session ACTIVE
- [ ] Terminate agent during spawn → Graceful handling
- [ ] Terminate session with SPAWNING agents → All cleaned
- [ ] Agent process crashes → Session stays ACTIVE

### Edge Cases
- [ ] Spawn duplicate role → Error returned
- [ ] Send message to non-existent agent → Error
- [ ] Terminate already-terminated agent → Idempotent
- [ ] Create session after session terminated → New session

---

## Summary

**Architecture:**
- UserSession (container) → 0-N AgentSessions
- Independent agent lifecycles
- Dynamic roles (not hardcoded)

**States:**
- UserSession: ACTIVE → TERMINATED
- AgentSession: SPAWNING → ACTIVE/FAILED → TERMINATED

**Key Benefits:**
- Agent isolation (failures don't cascade)
- Flexibility (variable agent count)
- Simplicity (only 2 states for UserSession)

**Storage:** In-memory (no persistence)

**Future:** Add persistence, reconnection, monitoring in later phases
