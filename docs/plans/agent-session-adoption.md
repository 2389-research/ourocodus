# Agent Session Adoption Architecture

**Status**: Design Phase
**Authors**: Claude Code with Zen Multi-Model Consensus
**Date**: 2025-11-20
**Related**: docs/agentd.md, pkg/relay/session/models.go

## Problem Statement

Developers need the ability to spawn agents programmatically via CLI (`agentd spawn`) and later "adopt" them into a PWA session for graphical interaction. This enables mobility scenarios:

- SSH into remote machine, spawn agents via CLI
- Open PWA locally (or via Tailscale) and attach to running agents
- Handoff between CLI and GUI workflows
- Persistent agents that survive UserSession lifecycles

### Current Limitation

Today, `AgentSession` objects exist only within a `UserSession` hierarchy on the web/PWA. CLI-spawned agents run independently in Docker but cannot be discovered or controlled from the PWA.

## Design Goals

1. **Unified Agent Catalog** - PWA can discover all agents (CLI + PWA spawned)
2. **Attach/Detach Semantics** - Agents can be "detached" (CLI-only) or "attached" (owned by UserSession)
3. **Developer Mobility** - Support SSH + PWA, local/remote handoff scenarios
4. **Eternal Agents** - Agents persist until explicitly stopped, independent of UserSession lifecycle
5. **Concurrency Safety** - Prevent two UserSessions from attaching to same agent
6. **Security** - Prevent agent hijacking across workspaces/users

## Architectural Approach

### Consensus Recommendation

After multi-model analysis (GPT-5, GPT-5 Codex, O3), we adopt a **"Simple First, Evolve Later"** approach:

- **Phase 1**: Docker labels + filesystem as source of truth (discovery MVP)
- **Phase 2**: NATS heartbeats for liveness detection
- **Phase 3**: ACP communication bridge (attach → communicate)
- **Phase 4**: Security hardening (workspace-scoped tokens, auth)
- **Phase 5**: Optional in-memory registry for performance (defer until needed)

### Why Not a Persistent Registry First?

**Trade-off Analysis**:
- **Against Persistent Registry (GPT-5 position)**: Creates "impedance mismatch" between eternal agents (survive restarts) and ephemeral in-memory state. Adds database dependency (SQLite/BoltDB) before validating core workflow.
- **For Persistent Registry (Codex position)**: Provides authoritative state, enables complex queries, better for multi-tenant scenarios.
- **Neutral (O3 position)**: Either works, depends on scale and operational requirements.

**Decision**: Start with Docker + FS as source of truth. We already have Docker label infrastructure from `agentd` CLI. Validate the workflow first, add registry if performance degrades.

## Phase 1: Docker Label Discovery (MVP)

### Source of Truth

**Docker Container Labels**:
```go
// Existing labels (from pkg/)
"ourocodus.agent": "true"
"ourocodus.agent/agent-id": "alice"

// New labels
"ourocodus.agent/spawn-source": "cli"  // or "pwa"
"ourocodus.agent/workspace": "/path/to/.agentd/worktrees/agent-alice"
```

**Filesystem State**:
```text
.agentd/
  worktrees/
    agent-alice/         # Agent workspace (git worktree)
  session/
    alice.lease          # Lease file (exists only when attached)
```

### Discovery Flow

```text
┌─────────┐                    ┌───────────┐                  ┌────────┐
│   PWA   │                    │   Relay   │                  │ Docker │
└─────────┘                    └───────────┘                  └────────┘
     │                              │                              │
     │  WS: agent:discover          │                              │
     ├─────────────────────────────>│                              │
     │                              │  ContainerList(label filter) │
     │                              ├─────────────────────────────>│
     │                              │<─────────────────────────────┤
     │                              │  [container list with labels]│
     │                              │                              │
     │  WS: agent:discovered        │                              │
     │<─────────────────────────────┤                              │
     │  [{id, workspace, status}]   │                              │
```

### Attach Flow with Lease

```text
┌─────────┐                    ┌───────────┐                  ┌────────────┐
│   PWA   │                    │   Relay   │                  │ Filesystem │
└─────────┘                    └───────────┘                  └────────────┘
     │                              │                              │
     │  WS: agent:attach            │                              │
     │  {agentId: "alice"}          │                              │
     ├─────────────────────────────>│                              │
     │                              │  Create lease (O_EXCL)       │
     │                              ├─────────────────────────────>│
     │                              │<─────────────────────────────┤
     │                              │  (success or EEXIST)         │
     │                              │                              │
     │  WS: agent:attached          │                              │
     │<─────────────────────────────┤                              │
     │  {attached: true}            │                              │
```

**Lease File Format** (`.agentd/session/alice.lease`):
```json
{
  "agentId": "alice",
  "userSessionId": "usr_abc123",
  "attachedAt": "2025-11-20T10:30:00Z",
  "expiresAt": "2025-11-20T10:35:00Z",
  "heartbeatInterval": "30s"
}
```

### Concurrency Guard

**Atomic Lease Creation**:
```go
// pkg/relay/session/lease.go
// Returns (*Lease, error) - lease contains the actual expiry time
// Lease directory can be configured with OUROCODUS_LEASE_DIR environment variable
func AcquireLease(agentID, userSessionID string) (*Lease, error) {
    leasePath := filepath.Join(LeaseDir, agentID+".lease")

    // O_EXCL ensures atomic creation (fails if exists)
    f, err := os.OpenFile(leasePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if err != nil {
        if os.IsExist(err) {
            // Check if existing lease is expired and can be reclaimed
            existing, readErr := ReadLease(agentID)
            if readErr == nil && IsLeaseExpired(existing) {
                // Retry with backoff
                // ... (retry logic omitted for brevity)
            }
            return nil, ErrAlreadyAttached
        }
        return nil, err
    }
    defer f.Close()

    lease := &Lease{
        AgentID:          agentID,
        UserSessionID:    userSessionID,
        AttachedAt:       time.Now(),
        ExpiresAt:        time.Now().Add(LeaseTTL),
        HeartbeatInterval: HeartbeatInterval,
    }

    if err := json.NewEncoder(f).Encode(lease); err != nil {
        return nil, err
    }

    return lease, nil
}
```

### WebSocket Message Types

#### agent:discover

**Request** (PWA → Relay):
```json
{
  "type": "agent:discover"
}
```

**Response** (Relay → PWA):
```json
{
  "type": "agent:discovered",
  "agents": [
    {
      "agentId": "alice",
      "containerId": "abc123def456",
      "workspace": "/path/to/.agentd/worktrees/agent-alice",
      "status": "detached",
      "spawnSource": "cli",
      "createdAt": "2025-11-20T10:00:00Z"
    },
    {
      "agentId": "bob",
      "containerId": "def456ghi789",
      "workspace": "/path/to/.agentd/worktrees/agent-bob",
      "status": "attached",
      "spawnSource": "pwa",
      "attachedTo": "usr_abc123",
      "createdAt": "2025-11-20T10:15:00Z"
    }
  ]
}
```

#### agent:attach

**Request** (PWA → Relay):
```json
{
  "type": "agent:attach",
  "agentId": "alice"
}
```

**Response** (Relay → PWA, Success):
```json
{
  "type": "agent:attached",
  "agentId": "alice",
  "attached": true,
  "lease": {
    "expiresAt": "2025-11-20T10:35:00Z",
    "heartbeatInterval": "30s"
  }
}
```

**Response** (Relay → PWA, Conflict):
```json
{
  "type": "agent:attached",
  "agentId": "alice",
  "attached": false,
  "error": "Agent already attached to another session",
  "attachedTo": "usr_xyz789"
}
```

#### agent:detach

**Request** (PWA → Relay):
```json
{
  "type": "agent:detach",
  "agentId": "alice"
}
```

**Response** (Relay → PWA):
```json
{
  "type": "agent:detached",
  "agentId": "alice",
  "detached": true
}
```

## Phase 2: NATS Heartbeats (Liveness Detection)

### Heartbeat Protocol

**Agent publishes**:
```text
Subject: agent.heartbeat.alice
Payload: {"agentId": "alice", "timestamp": "2025-11-20T10:30:15Z", "status": "active"}
Interval: 30s
```

**Relay subscribes**:
```go
// pkg/relay/session/heartbeat_monitor.go
func (r *Relay) monitorHeartbeats(ctx context.Context) {
    _, err := r.nats.Subscribe("agent.heartbeat.*", func(msg *nats.Msg) {
        var hb Heartbeat
        json.Unmarshal(msg.Data, &hb)

        // Update last-seen timestamp
        r.updateAgentLiveness(hb.AgentID, hb.Timestamp)

        // Renew lease if attached
        r.renewLeaseIfAttached(hb.AgentID)
    })
}
```

### Lease Expiry

**Problem**: Attached agent crashes, UserSession never explicitly detaches.

**Solution**: Lease TTL with heartbeat renewal:
```go
func (r *Relay) renewLeaseIfAttached(agentID string) error {
    leasePath := filepath.Join(".agentd/session", agentID+".lease")

    var lease Lease
    data, err := os.ReadFile(leasePath)
    if err != nil {
        return err // No lease = agent is detached
    }
    json.Unmarshal(data, &lease)

    // Extend expiry by 5 minutes
    lease.ExpiresAt = time.Now().Add(5 * time.Minute)

    updated, _ := json.Marshal(lease)
    return os.WriteFile(leasePath, updated, 0600)
}
```

**Background Reaper**:
```go
func (r *Relay) reapExpiredLeases(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    for {
        select {
        case <-ticker.C:
            r.removeExpiredLeases()
        case <-ctx.Done():
            return
        }
    }
}
```

## Phase 3: ACP Communication Bridge

### Goal

Enable PWA to send commands to and receive responses from attached agents via the existing ACP protocol.

### ACP Client Integration

**Problem**: When attaching a CLI-spawned agent, we need to establish bidirectional communication over ACP.

**Solution**: Create ACP client connection on attach.

```go
// In pkg/relay/session/models.go

func (us *UserSession) AttachAgent(agentID string) (*AgentSession, error) {
    // ... acquire lease (from Phase 1) ...

    // Connect to agent via ACP
    acpClient, err := us.connectToAgent(agentID)
    if err != nil {
        ReleaseLease(agentID) // Cleanup on failure
        return nil, fmt.Errorf("failed to connect to agent: %w", err)
    }

    agent := &AgentSession{
        AgentID:    agentID,
        Workspace:  workspace,
        createdAt:  lease.AttachedAt,
        state:      AgentActive,
        acpClient:  acpClient,
        lastActive: time.Now(),
        history:    []Message{},
    }

    us.agents[agentID] = agent
    return agent, nil
}

func (us *UserSession) connectToAgent(agentID string) (ACPClient, error) {
    // Get agent container ID from Docker
    containerID, err := findAgentContainerID(context.Background(), agentID)
    if err != nil {
        return nil, err
    }

    // Connect via docker exec or NATS ACP endpoint
    // Implementation depends on how agents expose ACP
    return NewACPClient(containerID)
}
```

### WebSocket Message Routing

**PWA → Agent**:
```text
┌─────────┐        ┌───────────┐        ┌────────────┐
│   PWA   │  WS    │   Relay   │  ACP   │ CLI Agent  │
└─────────┘        └───────────┘        └────────────┘
     │                   │                     │
     │  {type:"cmd",     │                     │
     │   agentId:"alice",│                     │
     │   payload:"..."}  │                     │
     ├──────────────────>│                     │
     │                   │  ACP Request        │
     │                   ├────────────────────>│
     │                   │                     │
     │                   │  ACP Response       │
     │                   │<────────────────────┤
     │  {type:"resp",    │                     │
     │   agentId:"alice",│                     │
     │   payload:"..."}  │                     │
     │<──────────────────┤                     │
```

### Agent Message Types

```go
// WebSocket message from PWA
type AgentCommand struct {
    Type      string          `json:"type"` // "agent:command"
    AgentID   string          `json:"agentId"`
    CommandID string          `json:"commandId"`
    Payload   json.RawMessage `json:"payload"`
}

// WebSocket message to PWA
type AgentResponse struct {
    Type      string          `json:"type"` // "agent:response"
    AgentID   string          `json:"agentId"`
    CommandID string          `json:"commandId"`
    Payload   json.RawMessage `json:"payload"`
}
```

### Relay Message Handler

```go
// In pkg/relay/session/user_session.go

func (us *UserSession) HandleMessage(msg []byte) error {
    var msgType struct {
        Type string `json:"type"`
    }
    if err := json.Unmarshal(msg, &msgType); err != nil {
        return err
    }

    switch msgType.Type {
    case "agent:command":
        return us.handleAgentCommand(msg)
    case "agent:detach":
        return us.handleAgentDetach(msg)
    // ... existing message types ...
    }
}

func (us *UserSession) handleAgentCommand(msg []byte) error {
    var cmd AgentCommand
    if err := json.Unmarshal(msg, &cmd); err != nil {
        return err
    }

    // Get agent session
    us.mu.RLock()
    agent, ok := us.agents[cmd.AgentID]
    us.mu.RUnlock()

    if !ok {
        return fmt.Errorf("agent %s not attached to this session", cmd.AgentID)
    }

    // Forward to ACP client
    resp, err := agent.acpClient.Send(cmd.Payload)
    if err != nil {
        return err
    }

    // Send response back to PWA
    return us.sendAgentResponse(cmd.AgentID, cmd.CommandID, resp)
}
```

### File Changes

**New Files**:
- `pkg/relay/session/acp_bridge.go` - ACP client wrapper for CLI agents

**Modified Files**:
- `pkg/relay/session/models.go` - Add ACP connection logic to AttachAgent()
- `pkg/relay/session/user_session.go` - Add agent:command message handler
- `pkg/relay/relay.go` - Wire up new message types

### Testing

```bash
# Spawn agent
agentd spawn test-agent

# Connect PWA via WebSocket
wscat -c ws://localhost:8080/ws

# Attach agent via WebSocket
> {"type":"agent:attach","agentId":"test-agent"}

# Should receive response:
< {"type":"agent:attached","agentId":"test-agent","attached":true,"lease":{...}}

# Send command to agent
> {"type":"agent:command","agentId":"test-agent","commandId":"cmd-1","payload":"ls"}

# Should receive response:
< {"type":"agent:response","agentId":"test-agent","commandId":"cmd-1","payload":"..."}
```

## Phase 4: Security Hardening

### Goal

Prevent unauthorized attachment to agents and secure the attach/detach workflow.

### Workspace-Scoped Tokens

**Token Generation** (on agent spawn):
```go
// In cmd/agentd/cmd_spawn.go

func generateAgentToken(agentID string) (string, error) {
    token := make([]byte, 32)
    if _, err := rand.Read(token); err != nil {
        return "", err
    }

    tokenStr := base64.URLEncoding.EncodeToString(token)

    // Write to .agentd/session/{agent-id}.token
    tokenPath := filepath.Join(".agentd/session", agentID+".token")
    if err := os.WriteFile(tokenPath, []byte(tokenStr), 0600); err != nil {
        return "", err
    }

    return tokenStr, nil
}
```

**Display token to user**:
```text
✨ Agent 'alice' spawned successfully

  Attach Token: tok_abc123def456...

  To attach from PWA via WebSocket:
    {"type": "agent:attach", "agentId": "alice", "token": "tok_abc123def456..."}
```

**Token Verification** (on attach):
```go
// In pkg/relay/session/user_session.go

type AgentAttachRequest struct {
    Type    string `json:"type"`    // "agent:attach"
    AgentID string `json:"agentId"`
    Token   string `json:"token"` // Required
}

func (us *UserSession) handleAgentAttach(msg []byte) error {
    var req AgentAttachRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return err
    }

    // Verify token
    if !verifyAgentToken(req.AgentID, req.Token) {
        return us.sendAgentAttachError(req.AgentID, "Invalid or missing attach token")
    }

    // ... proceed with attach ...
}

func verifyAgentToken(agentID, providedToken string) bool {
    tokenPath := filepath.Join(".agentd/session", agentID+".token")
    expectedToken, err := os.ReadFile(tokenPath)
    if err != nil {
        return false
    }

    return subtle.ConstantTimeCompare([]byte(providedToken), expectedToken) == 1
}
```

### User Authentication

**Integrate with existing WebSocket session**:
```go
// In pkg/relay/session/user_session.go

// User authentication happens at WebSocket connection time
// Each UserSession is already authenticated and tied to a specific user
// Agent attach/detach messages inherit the user context from the UserSession

func (us *UserSession) handleAgentAttach(msg []byte) error {
    // us.ID already contains the authenticated user session ID
    // No additional auth needed - WebSocket session provides authentication

    var req AgentAttachRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return err
    }

    // Verify token
    if !verifyAgentToken(req.AgentID, req.Token) {
        return us.sendAgentAttachError(req.AgentID, "Invalid or missing attach token")
    }

    // Attach agent to this authenticated user session
    agent, err := us.AttachAgent(req.AgentID)
    if err != nil {
        return us.sendAgentAttachError(req.AgentID, err.Error())
    }

    return us.sendAgentAttachSuccess(req.AgentID, agent)
}
```

### Audit Logging

```go
// Log all attach/detach operations
func (us *UserSession) AttachAgent(agentID string) (*AgentSession, error) {
    log.Printf("[AUDIT] User %s attaching to agent %s", us.ID, agentID)
    // ... attach logic ...
    log.Printf("[AUDIT] User %s successfully attached to agent %s", us.ID, agentID)
}
```

### Rate Limiting

```go
// Prevent attach spam at UserSession level
type UserSession struct {
    // ... existing fields ...
    agentOpLimiter *rate.Limiter
}

func NewUserSession(...) *UserSession {
    return &UserSession{
        // ... existing fields ...
        agentOpLimiter: rate.NewLimiter(rate.Every(time.Second), 10), // 10 ops/sec
    }
}

func (us *UserSession) handleAgentAttach(msg []byte) error {
    // Rate limit agent operations per UserSession
    if !us.agentOpLimiter.Allow() {
        return us.sendAgentAttachError("", "Rate limit exceeded - too many agent operations")
    }

    // ... rest of attach logic ...
}
```

### File Changes

**New Files**:
- `pkg/relay/audit/logger.go` - Structured audit logging

**Modified Files**:
- `cmd/agentd/cmd_spawn.go` - Generate and display attach token
- `pkg/relay/session/user_session.go` - Add token verification to message handlers, add rate limiting
- `pkg/relay/session/models.go` - Add audit logging to AttachAgent() and DetachAgent()

## Phase 5: Optional In-Memory Registry (Performance)

### When to Add Registry

**Triggers**:
- Agent count > 100 (Docker API calls become slow)
- Discovery endpoint latency > 500ms
- Need for complex queries (e.g., "all agents in workspace X")

### Registry as Cache

```go
// pkg/relay/registry/agent_registry.go
type AgentRegistry struct {
    agents map[string]*AgentDescriptor
    mu     sync.RWMutex
}

type AgentDescriptor struct {
    AgentID       string
    ContainerID   string
    Workspace     string
    Status        AgentStatus // detached | attached
    AttachedTo    string      // UserSession ID (if attached)
    SpawnSource   string      // cli | pwa
    LastHeartbeat time.Time
    CreatedAt     time.Time
}
```

**Rebuild from Docker on Startup**:
```go
func (r *AgentRegistry) Rebuild(ctx context.Context) error {
    cli, _ := client.NewClientWithOpts(client.FromEnv)
    containers, _ := cli.ContainerList(ctx, container.ListOptions{
        Filters: filters.NewArgs(
            filters.Arg("label", "ourocodus.agent=true"),
        ),
    })

    for _, c := range containers {
        r.Register(c.Labels["ourocodus.agent/agent-id"], c.ID, c.Labels)
    }
    return nil
}
```

**Note**: Docker labels remain source of truth. Registry is just a cache for fast reads.

## Security Considerations

### Phase 1 (MVP): Workspace Isolation

**Assumption**: Agents in different workspaces are implicitly isolated. User can only discover agents in workspaces they have filesystem access to.

**Limitation**: No explicit user authentication. Anyone with PWA access can attach to any agent.

### Phase 2+: Workspace-Scoped Tokens

**Token Generation** (on agent spawn):
```go
// Generate random token and write to .agentd/session/{agent-id}.token
token := generateSecureToken()
os.WriteFile(filepath.Join(".agentd/session", agentID+".token"), []byte(token), 0600)
```

**Attach Requires Token**:
```json
POST /api/agents/attach
{
  "agentId": "alice",
  "token": "tok_abc123def456..."
}
```

**Token Verification**:
```go
func (r *Relay) verifyAgentToken(agentID, providedToken string) bool {
    expectedToken, _ := os.ReadFile(filepath.Join(".agentd/session", agentID+".token"))
    return subtle.ConstantTimeCompare([]byte(providedToken), expectedToken) == 1
}
```

## File Changes Required

### New Files

- `pkg/relay/session/lease.go` - Lease management (acquire, release, renew, reap)
- `pkg/agent/heartbeat.go` - NATS heartbeat publisher
- `pkg/relay/session/heartbeat_monitor.go` - NATS heartbeat subscriber
- `docs/plans/agent-session-adoption.md` - This document

### Modified Files

- `cmd/agentd/cmd_spawn.go` - Add `spawn-source: cli` label
- `pkg/relay/session/models.go` - Add `AttachAgent()` and `DetachAgent()` methods
- `pkg/relay/session/user_session.go` - Add WebSocket message handlers for agent:discover, agent:attach, agent:detach
- `pkg/relay/relay.go` - Initialize heartbeat monitor on startup

## Testing Strategy

### Unit Tests

- `lease_test.go` - Atomic lease creation (O_EXCL semantics)
- `heartbeat_monitor_test.go` - Lease renewal on heartbeat
- `handlers_agents_test.go` - Discovery endpoint returns correct status

### Integration Tests

- Spawn agent via CLI, discover via PWA, verify `detached` status
- Attach agent, verify lease file created
- Simultaneous attach from two UserSessions, verify one fails with conflict
- Agent crashes, verify lease expires after 5 minutes

### End-to-End Tests

- `scripts/test-agent-adoption.sh` - Full workflow: spawn → discover → attach → communicate → detach

## Rollout Plan

### Phase 1: Docker Label Discovery (Week 1-2)
**Goal**: Basic discovery and attach/detach without communication

- Add spawn-source label to agentd
- Implement `/api/agents/discover` endpoint
- Implement lease-based attach/detach
- Integration tests
- **Milestone**: Can discover and attach to CLI agents, but can't send commands yet

### Phase 2: NATS Heartbeats (Week 3)
**Goal**: Liveness detection and automatic lease cleanup

- Add heartbeat publisher to agent
- Add heartbeat monitor to relay
- Implement lease renewal
- Background reaper for expired leases
- **Milestone**: Orphaned agents are automatically detected and cleaned up

### Phase 3: ACP Communication Bridge (Week 4)
**Goal**: Full bidirectional communication between PWA and CLI agents

- Implement ACP client wrapper for attached agents
- Add agent:command/agent:response WebSocket message types
- Wire up relay message routing
- End-to-end communication tests
- **Milestone**: PWA can control CLI agents just like PWA-spawned agents

### Phase 4: Security Hardening (Week 5)
**Goal**: Production-ready security and auth

- Add workspace-scoped attach tokens
- Implement token verification
- Add user authentication middleware
- Add audit logging for attach/detach
- Add rate limiting
- Security audit and penetration testing
- **Milestone**: Production-ready with full security controls

### Phase 5: Optional Registry (Week 6+, if needed)
**Goal**: Performance optimization for large deployments

- Implement in-memory registry
- Benchmark discovery latency improvement
- Add complex query support (e.g., filter by workspace, status)
- **Milestone**: Sub-100ms discovery for 100+ agents

## Monitoring and Observability

### Metrics

- `agent_discovery_latency_ms` - Time to scan Docker containers
- `lease_acquisition_conflicts_total` - Count of simultaneous attach attempts
- `lease_expiry_total` - Count of orphaned agents detected
- `heartbeat_missed_total` - Count of missed heartbeats per agent

### Logs

- Agent attach/detach events with UserSession ID
- Lease expiry with reason (crash vs explicit detach)
- Discovery endpoint calls with result count

## Alternatives Considered

### Alternative 1: Persistent Registry with SQLite

**Pros**:
- Fast queries, no Docker API calls
- Supports complex filters
- Better for multi-tenant scenarios

**Cons**:
- Adds database dependency
- Registry can drift from Docker reality
- Requires reconciliation loop

**Why rejected**: Premature complexity. Start simple, add if needed.

### Alternative 2: Workspace-Centric Federation

**Pros**:
- No central registry
- Natural isolation by workspace

**Cons**:
- Requires scanning filesystem on every discovery
- Slow for large workspace counts
- Hard to detect agent liveness

**Why rejected**: Performance concerns at scale.

### Alternative 3: NATS-Only Discovery

**Pros**:
- Real-time agent announcements
- No Docker API dependency

**Cons**:
- Agents must announce on startup
- Missed announcements = invisible agents
- Requires persistent NATS state

**Why rejected**: Fragile without fallback to Docker source of truth.

## References

- [agentd CLI Documentation](../agentd.md)
- [Session Models](../../pkg/relay/session/models.go)
- [Docker Labels Best Practices](https://docs.docker.com/config/labels-custom-metadata/)
- [NATS Heartbeat Pattern](https://docs.nats.io/nats-concepts/subjects)

## Appendix: Consensus Summary

### Model Perspectives

**GPT-5 (against persistent registry)**:
- Start with Docker + FS as source of truth
- Add registry only if performance degrades
- Timeline: 2-3 weeks MVP

**GPT-5 Codex (for persistent registry)**:
- Build robust registry from start
- Persistent state for complex queries
- Timeline: 2-3 sprints

**O3 (neutral)**:
- Either approach works
- Depends on scale and ops requirements
- Timeline: 4-6 weeks production-ready

**Agreement**:
- Heartbeats essential for liveness
- Concurrency guards needed
- Security/auth critical
- High user value

**Decision**: Adopt GPT-5's incremental approach. Start simple, evolve as needed.
