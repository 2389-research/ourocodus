# Agent Session Adoption - Implementation Plan

**Parent Design**: [agent-session-adoption.md](./agent-session-adoption.md)
**Branch**: `feat/agent-session-adoption`
**Worktree**: `/Users/clint/.config/superpowers/worktrees/ourocodus/feat-agent-session-adoption`
**Timeline**: 3 weeks (Phase 1 + Phase 2)
**Date**: 2025-11-20

## Overview

This plan implements the Agent Session Adoption architecture in bite-sized tasks across 5 phases. Each task is self-contained and can be implemented, tested, and reviewed independently.

## Phase Breakdown

- **Phase 1** (Week 1-2): Docker Label Discovery - Basic discovery and attach/detach (no communication)
- **Phase 2** (Week 3): NATS Heartbeats - Liveness detection and automatic cleanup
- **Phase 3** (Week 4): ACP Communication Bridge - Full bidirectional PWA ↔ CLI agent communication
- **Phase 4** (Week 5): Security Hardening - Tokens, auth, audit logging, rate limiting
- **Phase 5** (Week 6+): Optional Registry - Performance optimization (defer until needed)

## Phase 1: Docker Label Discovery (Week 1-2)

**Goal**: Basic discovery and attach/detach without communication
**Milestone**: Can discover and attach to CLI agents, but can't send commands yet

### Task 1.1: Add spawn-source Label to agentd

**Estimated Time**: 2 hours

**Files to Modify**:
- `cmd/agentd/cmd_spawn.go`

**What to Do**:
1. Add `spawn-source: cli` label to Docker container creation
2. Locate the `SpawnConfig` initialization (around line 150-180)
3. Docker labels are set via `pkg/launcher`, but since we control `agentd`, we can pass custom labels through environment or config

**Code Changes**:
```go
// In cmd_spawn.go, after line 165 where SpawnConfig is built
// Add to the Docker labels (if pkg supports custom labels)
// OR add as environment variable that agents can self-report

// Option 1: If pkg/launcher supports custom labels
cfg := &launcher.SpawnConfig{
    // ... existing fields ...
    Labels: map[string]string{
        "ourocodus.agent/spawn-source": "cli",
    },
}

// Option 2: If pkg doesn't support custom labels yet
// Add environment variable that agent can use to set label on startup
cfg.Env = append(cfg.Env, "SPAWN_SOURCE=cli")
```

**Testing**:
```bash
# Spawn agent
agentd spawn test-label

# Verify label exists
docker inspect $(agentd list --format json | jq -r '.agents[0].containerId') | jq '.[0].Config.Labels'

# Should see: "ourocodus.agent/spawn-source": "cli"
```

**Acceptance Criteria**:
- [ ] CLI-spawned agents have `ourocodus.agent/spawn-source=cli` label
- [ ] Label is visible in `docker inspect`
- [ ] Existing spawn functionality unaffected

---

### Task 1.2: Create Lease Management Module

**Estimated Time**: 4 hours

**Files to Create**:
- `pkg/relay/session/lease.go`
- `pkg/relay/session/lease_test.go`

**What to Do**:
1. Implement `Lease` struct with JSON marshaling
2. Implement `AcquireLease()` with O_EXCL atomic creation
3. Implement `ReleaseLease()` to delete lease file
4. Implement `RenewLease()` to extend expiry
5. Implement `IsLeaseExpired()` to check TTL
6. Unit tests for all functions

**Code Structure**:
```go
// pkg/relay/session/lease.go
package session

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

const (
    LeaseDir         = ".agentd/session"
    LeaseTTL         = 5 * time.Minute
    HeartbeatInterval = 30 * time.Second
)

var (
    ErrAlreadyAttached = fmt.Errorf("agent already attached to another session")
    ErrLeaseNotFound   = fmt.Errorf("lease not found")
    ErrLeaseExpired    = fmt.Errorf("lease has expired")
)

type Lease struct {
    AgentID          string    `json:"agentId"`
    UserSessionID    string    `json:"userSessionId"`
    AttachedAt       time.Time `json:"attachedAt"`
    ExpiresAt        time.Time `json:"expiresAt"`
    HeartbeatInterval string   `json:"heartbeatInterval"`
}

// AcquireLease atomically creates a lease file for the given agent.
// Returns ErrAlreadyAttached if lease already exists.
func AcquireLease(agentID, userSessionID string) (*Lease, error) {
    // Ensure lease directory exists
    if err := os.MkdirAll(LeaseDir, 0700); err != nil {
        return nil, fmt.Errorf("failed to create lease directory: %w", err)
    }

    leasePath := filepath.Join(LeaseDir, agentID+".lease")

    // O_EXCL ensures atomic creation (fails if exists)
    f, err := os.OpenFile(leasePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
    if err != nil {
        if os.IsExist(err) {
            // Check if existing lease is expired
            if existing, err := ReadLease(agentID); err == nil {
                if IsLeaseExpired(existing) {
                    // Expired lease, force release and retry
                    _ = ReleaseLease(agentID)
                    return AcquireLease(agentID, userSessionID)
                }
                return nil, ErrAlreadyAttached
            }
            return nil, ErrAlreadyAttached
        }
        return nil, fmt.Errorf("failed to create lease file: %w", err)
    }
    defer f.Close()

    lease := &Lease{
        AgentID:           agentID,
        UserSessionID:     userSessionID,
        AttachedAt:        time.Now(),
        ExpiresAt:         time.Now().Add(LeaseTTL),
        HeartbeatInterval: HeartbeatInterval.String(),
    }

    if err := json.NewEncoder(f).Encode(lease); err != nil {
        _ = os.Remove(leasePath) // Cleanup on error
        return nil, fmt.Errorf("failed to write lease: %w", err)
    }

    return lease, nil
}

// ReleaseLease deletes the lease file for the given agent.
func ReleaseLease(agentID string) error {
    leasePath := filepath.Join(LeaseDir, agentID+".lease")
    if err := os.Remove(leasePath); err != nil {
        if os.IsNotExist(err) {
            return nil // Already released, idempotent
        }
        return fmt.Errorf("failed to remove lease: %w", err)
    }
    return nil
}

// ReadLease reads the lease file for the given agent.
func ReadLease(agentID string) (*Lease, error) {
    leasePath := filepath.Join(LeaseDir, agentID+".lease")
    data, err := os.ReadFile(leasePath)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, ErrLeaseNotFound
        }
        return nil, fmt.Errorf("failed to read lease: %w", err)
    }

    var lease Lease
    if err := json.Unmarshal(data, &lease); err != nil {
        return nil, fmt.Errorf("failed to parse lease: %w", err)
    }

    return &lease, nil
}

// RenewLease extends the expiry time of an existing lease.
func RenewLease(agentID string) error {
    lease, err := ReadLease(agentID)
    if err != nil {
        return err
    }

    // Extend expiry
    lease.ExpiresAt = time.Now().Add(LeaseTTL)

    leasePath := filepath.Join(LeaseDir, agentID+".lease")
    data, err := json.Marshal(lease)
    if err != nil {
        return fmt.Errorf("failed to marshal lease: %w", err)
    }

    if err := os.WriteFile(leasePath, data, 0600); err != nil {
        return fmt.Errorf("failed to write lease: %w", err)
    }

    return nil
}

// IsLeaseExpired checks if a lease has expired.
func IsLeaseExpired(lease *Lease) bool {
    return time.Now().After(lease.ExpiresAt)
}

// ListLeases returns all lease files in the lease directory.
func ListLeases() ([]*Lease, error) {
    entries, err := os.ReadDir(LeaseDir)
    if err != nil {
        if os.IsNotExist(err) {
            return []*Lease{}, nil // No leases yet
        }
        return nil, fmt.Errorf("failed to read lease directory: %w", err)
    }

    var leases []*Lease
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }

        // Extract agent ID from filename (remove .lease extension)
        agentID := entry.Name()
        if len(agentID) > 6 && agentID[len(agentID)-6:] == ".lease" {
            agentID = agentID[:len(agentID)-6]
        }

        lease, err := ReadLease(agentID)
        if err != nil {
            continue // Skip invalid leases
        }
        leases = append(leases, lease)
    }

    return leases, nil
}
```

**Testing**:
```go
// pkg/relay/session/lease_test.go
package session

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestAcquireLease(t *testing.T) {
    // Setup: temporary lease directory
    tmpDir := t.TempDir()
    originalLeaseDir := LeaseDir
    LeaseDir = filepath.Join(tmpDir, "session")
    defer func() { LeaseDir = originalLeaseDir }()

    agentID := "test-agent"
    sessionID := "sess-123"

    // Test: successful acquisition
    lease, err := AcquireLease(agentID, sessionID)
    if err != nil {
        t.Fatalf("AcquireLease failed: %v", err)
    }

    if lease.AgentID != agentID {
        t.Errorf("expected agentID %s, got %s", agentID, lease.AgentID)
    }
    if lease.UserSessionID != sessionID {
        t.Errorf("expected sessionID %s, got %s", sessionID, lease.UserSessionID)
    }

    // Test: conflict on second acquisition
    _, err = AcquireLease(agentID, "sess-456")
    if err != ErrAlreadyAttached {
        t.Errorf("expected ErrAlreadyAttached, got %v", err)
    }

    // Test: release and re-acquire
    if err := ReleaseLease(agentID); err != nil {
        t.Fatalf("ReleaseLease failed: %v", err)
    }

    lease, err = AcquireLease(agentID, "sess-789")
    if err != nil {
        t.Fatalf("AcquireLease after release failed: %v", err)
    }
    if lease.UserSessionID != "sess-789" {
        t.Errorf("expected new sessionID after release")
    }
}

func TestLeaseExpiry(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    originalLeaseDir := LeaseDir
    LeaseDir = filepath.Join(tmpDir, "session")
    defer func() { LeaseDir = originalLeaseDir }()

    agentID := "test-agent"
    lease, _ := AcquireLease(agentID, "sess-123")

    // Test: not expired initially
    if IsLeaseExpired(lease) {
        t.Error("lease should not be expired immediately after creation")
    }

    // Test: manually expire
    lease.ExpiresAt = time.Now().Add(-1 * time.Minute)
    if !IsLeaseExpired(lease) {
        t.Error("lease should be expired after manual expiry")
    }

    // Test: write expired lease and try to acquire
    leasePath := filepath.Join(LeaseDir, agentID+".lease")
    data, _ := json.Marshal(lease)
    os.WriteFile(leasePath, data, 0600)

    // Should succeed because expired lease is auto-released
    newLease, err := AcquireLease(agentID, "sess-456")
    if err != nil {
        t.Fatalf("AcquireLease should succeed on expired lease: %v", err)
    }
    if newLease.UserSessionID != "sess-456" {
        t.Error("expected new session to acquire expired lease")
    }
}

func TestRenewLease(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    originalLeaseDir := LeaseDir
    LeaseDir = filepath.Join(tmpDir, "session")
    defer func() { LeaseDir = originalLeaseDir }()

    agentID := "test-agent"
    lease, _ := AcquireLease(agentID, "sess-123")
    originalExpiry := lease.ExpiresAt

    // Wait a bit then renew
    time.Sleep(100 * time.Millisecond)
    if err := RenewLease(agentID); err != nil {
        t.Fatalf("RenewLease failed: %v", err)
    }

    // Read lease and verify expiry extended
    renewed, _ := ReadLease(agentID)
    if !renewed.ExpiresAt.After(originalExpiry) {
        t.Error("lease expiry should be extended after renewal")
    }
}
```

**Acceptance Criteria**:
- [ ] `AcquireLease()` creates lease file atomically with O_EXCL
- [ ] Second acquire attempt returns `ErrAlreadyAttached`
- [ ] Expired leases are auto-released on acquire attempt
- [ ] `ReleaseLease()` is idempotent (no error if already released)
- [ ] `RenewLease()` extends expiry by `LeaseTTL`
- [ ] All unit tests pass

---

### Task 1.3: Add Agent Discovery Message Handler

**Estimated Time**: 6 hours

**Files to Modify**:
- `pkg/relay/session/user_session.go` (add message handler)

**What to Do**:
1. Implement `agent:discover` WebSocket message handler
2. Query Docker for containers with `ourocodus.agent=true` label
3. Check for lease files to determine attached/detached status
4. Send `agent:discovered` response with agent list
5. Unit tests with mocked Docker client

**Code Structure**:
```go
// pkg/relay/session/user_session.go

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/filters"
    "github.com/docker/docker/client"
    "github.com/gorilla/websocket"
)

const (
    LabelNamespace = "ourocodus.agent"
    LabelAgentID   = "ourocodus.agent/agent-id"
    LabelSpawnSource = "ourocodus.agent/spawn-source"
)

type AgentStatus string

const (
    StatusDetached AgentStatus = "detached"
    StatusAttached AgentStatus = "attached"
)

type AgentInfo struct {
    AgentID      string      `json:"agentId"`
    ContainerID  string      `json:"containerId"`
    Workspace    string      `json:"workspace"`
    Status       AgentStatus `json:"status"`
    SpawnSource  string      `json:"spawnSource"`
    AttachedTo   string      `json:"attachedTo,omitempty"`
    CreatedAt    time.Time   `json:"createdAt"`
}

type DiscoverRequest struct {
    Type string `json:"type"` // "agent:discover"
}

type DiscoverResponse struct {
    Type   string      `json:"type"` // "agent:discovered"
    Agents []AgentInfo `json:"agents"`
}

// Add to UserSession.HandleMessage switch statement:
func (us *UserSession) HandleMessage(msg []byte) error {
    var msgType struct {
        Type string `json:"type"`
    }
    if err := json.Unmarshal(msg, &msgType); err != nil {
        return err
    }

    switch msgType.Type {
    case "agent:discover":
        return us.handleAgentDiscover(msg)
    // ... existing message types ...
    }
}

func (us *UserSession) handleAgentDiscover(msg []byte) error {
    agents, err := us.discoverAgents(context.Background())
    if err != nil {
        return fmt.Errorf("failed to discover agents: %w", err)
    }

    resp := DiscoverResponse{
        Type:   "agent:discovered",
        Agents: agents,
    }

    data, err := json.Marshal(resp)
    if err != nil {
        return err
    }

    return us.webSocket.WriteMessage(websocket.TextMessage, data)
}

func (us *UserSession) discoverAgents(ctx context.Context) ([]AgentInfo, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return nil, fmt.Errorf("failed to create Docker client: %w", err)
    }
    defer cli.Close()

    // Filter for agentd containers
    filterArgs := filters.NewArgs()
    filterArgs.Add("label", fmt.Sprintf("%s=true", LabelNamespace))

    containers, err := cli.ContainerList(ctx, container.ListOptions{
        All:     false, // Only running containers
        Filters: filterArgs,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to list containers: %w", err)
    }

    // Get all leases to determine attached status
    leases, err := session.ListLeases()
    if err != nil {
        return nil, fmt.Errorf("failed to list leases: %w", err)
    }

    leaseMap := make(map[string]*session.Lease)
    for _, lease := range leases {
        if !session.IsLeaseExpired(lease) {
            leaseMap[lease.AgentID] = lease
        }
    }

    agents := make([]AgentInfo, 0, len(containers))
    for _, c := range containers {
        agentID := c.Labels[LabelAgentID]
        if agentID == "" {
            continue // Skip containers without agent-id
        }

        // Extract workspace from mounts
        workspace := ""
        for _, mnt := range c.Mounts {
            if mnt.Destination == "/workspace" {
                workspace = mnt.Source
                break
            }
        }

        // Determine status from lease
        status := StatusDetached
        attachedTo := ""
        if lease, ok := leaseMap[agentID]; ok {
            status = StatusAttached
            attachedTo = lease.UserSessionID
        }

        agents = append(agents, AgentInfo{
            AgentID:     agentID,
            ContainerID: c.ID,
            Workspace:   workspace,
            Status:      status,
            SpawnSource: c.Labels[LabelSpawnSource],
            AttachedTo:  attachedTo,
            CreatedAt:   time.Unix(c.Created, 0),
        })
    }

    return agents, nil
}
```

**Testing**:
```bash
# Integration test (requires Docker)
# Spawn agent via CLI
agentd spawn test-agent

# Connect to relay WebSocket
wscat -c ws://localhost:8080/ws

# Send discover message
> {"type":"agent:discover"}

# Expected response:
< {
  "type": "agent:discovered",
  "agents": [
    {
      "agentId": "test-agent",
      "containerId": "abc123...",
      "workspace": "/path/to/.agentd/worktrees/agent-test-agent",
      "status": "detached",
      "spawnSource": "cli",
      "createdAt": "2025-11-20T10:00:00Z"
    }
  ]
}
```

**Acceptance Criteria**:
- [ ] `agent:discover` message returns all running agents
- [ ] Status correctly reflects attached/detached based on lease files
- [ ] Expired leases are filtered out (agent shows as detached)
- [ ] Response includes all required fields
- [ ] Handler gracefully handles Docker connection errors

---

### Task 1.4: Add Attach/Detach Message Handlers

**Estimated Time**: 6 hours

**Files to Modify**:
- `pkg/relay/session/user_session.go` (add message handlers)

**What to Do**:
1. Implement `agent:attach` WebSocket message handler
2. Implement `agent:detach` WebSocket message handler
3. Use lease module for atomic attach
4. Send appropriate error responses for conflicts
5. Integration tests

**Code Structure**:
```go
// Add to pkg/relay/session/user_session.go

type AttachRequest struct {
    Type    string `json:"type"`    // "agent:attach"
    AgentID string `json:"agentId"`
}

type AttachResponse struct {
    Type       string         `json:"type"`       // "agent:attached"
    AgentID    string         `json:"agentId"`
    Attached   bool           `json:"attached"`
    Lease      *Lease         `json:"lease,omitempty"`
    Error      string         `json:"error,omitempty"`
    AttachedTo string         `json:"attachedTo,omitempty"`
}

// Add to UserSession.HandleMessage switch statement:
case "agent:attach":
    return us.handleAgentAttach(msg)

func (us *UserSession) handleAgentAttach(msg []byte) error {
    var req AttachRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return err
    }

    if req.AgentID == "" {
        return us.sendAgentAttachError("", "agentId is required")
    }

    // Verify agent exists in Docker
    exists, err := us.agentExists(context.Background(), req.AgentID)
    if err != nil {
        return us.sendAgentAttachError(req.AgentID, fmt.Sprintf("Failed to verify agent: %v", err))
    }
    if !exists {
        return us.sendAgentAttachError(req.AgentID, "Agent not found")
    }

    // Try to acquire lease (us.ID contains authenticated UserSession ID)
    lease, err := AcquireLease(req.AgentID, us.ID)
    if err != nil {
        if err == ErrAlreadyAttached {
            // Get existing lease to see who has it
            existingLease, _ := ReadLease(req.AgentID)
            return us.sendAgentAttachConflict(req.AgentID, existingLease.UserSessionID)
        }
        return us.sendAgentAttachError(req.AgentID, fmt.Sprintf("Failed to attach agent: %v", err))
    }

    return us.sendAgentAttachSuccess(req.AgentID, lease)
}

func (us *UserSession) sendAgentAttachSuccess(agentID string, lease *Lease) error {
    resp := AttachResponse{
        Type:     "agent:attached",
        AgentID:  agentID,
        Attached: true,
        Lease:    lease,
    }
    data, _ := json.Marshal(resp)
    return us.webSocket.WriteMessage(websocket.TextMessage, data)
}

func (us *UserSession) sendAgentAttachError(agentID, errMsg string) error {
    resp := AttachResponse{
        Type:     "agent:attached",
        AgentID:  agentID,
        Attached: false,
        Error:    errMsg,
    }
    data, _ := json.Marshal(resp)
    return us.webSocket.WriteMessage(websocket.TextMessage, data)
}

func (us *UserSession) sendAgentAttachConflict(agentID, attachedTo string) error {
    resp := AttachResponse{
        Type:       "agent:attached",
        AgentID:    agentID,
        Attached:   false,
        Error:      "Agent already attached to another session",
        AttachedTo: attachedTo,
    }
    data, _ := json.Marshal(resp)
    return us.webSocket.WriteMessage(websocket.TextMessage, data)
}

type DetachRequest struct {
    Type    string `json:"type"`    // "agent:detach"
    AgentID string `json:"agentId"`
}

type DetachResponse struct {
    Type     string `json:"type"`     // "agent:detached"
    AgentID  string `json:"agentId"`
    Detached bool   `json:"detached"`
    Error    string `json:"error,omitempty"`
}

// Add to UserSession.HandleMessage switch statement:
case "agent:detach":
    return us.handleAgentDetach(msg)

func (us *UserSession) handleAgentDetach(msg []byte) error {
    var req DetachRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return err
    }

    if req.AgentID == "" {
        return us.sendAgentDetachError("", "agentId is required")
    }

    // Verify lease belongs to this user session
    lease, err := ReadLease(req.AgentID)
    if err != nil {
        if err == ErrLeaseNotFound {
            // Already detached, idempotent success
            return us.sendAgentDetachSuccess(req.AgentID)
        }
        return us.sendAgentDetachError(req.AgentID, fmt.Sprintf("Failed to read lease: %v", err))
    }

    if lease.UserSessionID != us.ID {
        return us.sendAgentDetachError(req.AgentID, "Cannot detach agent attached to another session")
    }

    // Release lease
    if err := ReleaseLease(req.AgentID); err != nil {
        return us.sendAgentDetachError(req.AgentID, fmt.Sprintf("Failed to detach agent: %v", err))
    }

    return us.sendAgentDetachSuccess(req.AgentID)
}

func (us *UserSession) sendAgentDetachSuccess(agentID string) error {
    resp := DetachResponse{
        Type:     "agent:detached",
        AgentID:  agentID,
        Detached: true,
    }
    data, _ := json.Marshal(resp)
    return us.webSocket.WriteMessage(websocket.TextMessage, data)
}

func (us *UserSession) sendAgentDetachError(agentID, errMsg string) error {
    resp := DetachResponse{
        Type:     "agent:detached",
        AgentID:  agentID,
        Detached: false,
        Error:    errMsg,
    }
    data, _ := json.Marshal(resp)
    return us.webSocket.WriteMessage(websocket.TextMessage, data)
}

func (s *Server) agentExists(ctx context.Context, agentID string) (bool, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return false, err
    }
    defer cli.Close()

    filterArgs := filters.NewArgs()
    filterArgs.Add("label", fmt.Sprintf("%s=true", LabelNamespace))
    filterArgs.Add("label", fmt.Sprintf("%s=%s", LabelAgentID, agentID))

    containers, err := cli.ContainerList(ctx, container.ListOptions{
        All:     false,
        Filters: filterArgs,
    })
    if err != nil {
        return false, err
    }

    return len(containers) > 0, nil
}
```

**Testing**:
```bash
# Spawn agent
agentd spawn test-agent

# Connect to WebSocket
wscat -c ws://localhost:8080/ws

# Attach via WebSocket
> {"type":"agent:attach","agentId":"test-agent"}

# Expected: {"type":"agent:attached","agentId":"test-agent","attached":true,"lease":{...}}

# Discover agents to verify status
> {"type":"agent:discover"}

# Expected: agent shows "status": "attached"

# Try to attach again from same session (idempotent)
> {"type":"agent:attach","agentId":"test-agent"}

# Expected: success (idempotent)

# Detach
> {"type":"agent:detach","agentId":"test-agent"}

# Expected: {"type":"agent:detached","agentId":"test-agent","detached":true}

# Discover agents to verify status changed back
> {"type":"agent:discover"}

# Expected: agent shows "status": "detached"
```

**Acceptance Criteria**:
- [ ] `agent:attach` message successfully acquires lease
- [ ] Simultaneous attach from two UserSessions returns conflict for second session
- [ ] `agent:detach` message releases lease and is idempotent
- [ ] Cannot detach agent attached to different session (returns error)
- [ ] Discovery reflects status change after attach/detach
- [ ] Non-existent agent returns error

---

### Task 1.5: Add UserSession.AttachAgent() Method

**Estimated Time**: 4 hours

**Files to Modify**:
- `pkg/relay/session/models.go`

**What to Do**:
1. Add `AttachAgent()` method to `UserSession`
2. Add `DetachAgent()` method to `UserSession`
3. Wire attach/detach endpoints to call these methods
4. Update `UserSession` to track attached agents from CLI

**Code Changes**:
```go
// In pkg/relay/session/models.go

// AttachAgent attaches a CLI-spawned agent to this UserSession.
// Returns error if agent is already attached to another session.
func (us *UserSession) AttachAgent(agentID string) (*AgentSession, error) {
    us.mu.Lock()
    defer us.mu.Unlock()

    // Check if already attached to this session
    if existing, ok := us.agents[agentID]; ok {
        return existing, nil // Idempotent
    }

    // Try to acquire lease
    lease, err := AcquireLease(agentID, us.ID)
    if err != nil {
        return nil, err
    }

    // Create AgentSession for this agent
    // Note: We don't have ACPClient yet (added in Phase 2)
    agent := &AgentSession{
        AgentID:    agentID,
        Workspace:  "", // TODO: Get from Docker container mounts
        createdAt:  lease.AttachedAt,
        state:      AgentActive,
        lastActive: time.Now(),
        history:    []Message{},
    }

    us.agents[agentID] = agent
    us.lastActive = time.Now()

    return agent, nil
}

// DetachAgent detaches a CLI-spawned agent from this UserSession.
// The agent continues running but is no longer associated with this session.
func (us *UserSession) DetachAgent(agentID string) error {
    us.mu.Lock()
    defer us.mu.Unlock()

    // Verify agent is attached to this session
    agent, ok := us.agents[agentID]
    if !ok {
        return nil // Already detached, idempotent
    }

    // Release lease
    if err := ReleaseLease(agentID); err != nil {
        return err
    }

    // Remove from session's agent map
    delete(us.agents, agentID)
    us.lastActive = time.Now()

    // Note: Don't terminate the agent, it continues running detached
    _ = agent // Silence unused warning

    return nil
}
```

**Wire to Message Handlers**:
```go
// In pkg/relay/session/user_session.go, modify handleAgentAttach:

func (us *UserSession) handleAgentAttach(msg []byte) error {
    var req AttachRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return err
    }

    // UserSession is already authenticated - us.ID contains user session ID
    // Call AttachAgent to add agent to this UserSession
    agent, err := us.AttachAgent(req.AgentID)
    if err != nil {
        return us.sendAgentAttachError(req.AgentID, err.Error())
    }

    // Send success response with lease info
    return us.sendAgentAttachSuccess(req.AgentID, agent)
}
```

**Acceptance Criteria**:
- [ ] `AttachAgent()` adds CLI agent to `UserSession.agents` map
- [ ] `DetachAgent()` removes agent from map but doesn't terminate container
- [ ] Methods are thread-safe (use `us.mu`)
- [ ] Idempotent (calling twice has no effect)

---

### Task 1.6: Integration Tests

**Estimated Time**: 4 hours

**Files to Create**:
- `scripts/test-agent-adoption.sh`

**What to Do**:
1. Write end-to-end test script
2. Test full workflow: spawn → discover → attach → detach
3. Test conflict scenarios (simultaneous attach)
4. Test idempotence (detach already-detached agent)

**Script Structure**:
```bash
#!/bin/bash
# scripts/test-agent-adoption.sh

set -e

RELAY_URL="http://localhost:8080"
AGENT_ID="test-adoption-$(date +%s)"

echo "=== Agent Adoption Integration Test ==="

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    agentd stop "$AGENT_ID" 2>/dev/null || true
}
trap cleanup EXIT

# 1. Spawn agent via CLI
echo "1. Spawning agent '$AGENT_ID' via CLI..."
agentd spawn "$AGENT_ID"
sleep 2

# 2. Discover agents
echo "2. Discovering agents..."
DISCOVER=$(curl -s "$RELAY_URL/api/agents/discover")
echo "$DISCOVER" | jq

# Verify agent is detached
STATUS=$(echo "$DISCOVER" | jq -r ".agents[] | select(.agentId == \"$AGENT_ID\") | .status")
if [ "$STATUS" != "detached" ]; then
    echo "FAIL: Expected status 'detached', got '$STATUS'"
    exit 1
fi
echo "✓ Agent is detached"

# 3. Attach agent
echo "3. Attaching agent..."
ATTACH=$(curl -s -X POST "$RELAY_URL/api/agents/attach" \
    -H "Content-Type: application/json" \
    -d "{\"agentId\": \"$AGENT_ID\"}")
echo "$ATTACH" | jq

ATTACHED=$(echo "$ATTACH" | jq -r '.attached')
if [ "$ATTACHED" != "true" ]; then
    echo "FAIL: Failed to attach agent"
    exit 1
fi
echo "✓ Agent attached successfully"

# 4. Verify status changed
echo "4. Verifying status changed to attached..."
DISCOVER=$(curl -s "$RELAY_URL/api/agents/discover")
STATUS=$(echo "$DISCOVER" | jq -r ".agents[] | select(.agentId == \"$AGENT_ID\") | .status")
if [ "$STATUS" != "attached" ]; then
    echo "FAIL: Expected status 'attached', got '$STATUS'"
    exit 1
fi
echo "✓ Status is attached"

# 5. Try to attach again (should conflict - simulated by timeout or manual second request)
echo "5. Testing conflict on second attach (skipped - requires concurrent requests)"
# In practice, test with two simultaneous curl commands

# 6. Detach agent
echo "6. Detaching agent..."
DETACH=$(curl -s -X POST "$RELAY_URL/api/agents/detach" \
    -H "Content-Type: application/json" \
    -d "{\"agentId\": \"$AGENT_ID\"}")
echo "$DETACH" | jq

DETACHED=$(echo "$DETACH" | jq -r '.detached')
if [ "$DETACHED" != "true" ]; then
    echo "FAIL: Failed to detach agent"
    exit 1
fi
echo "✓ Agent detached successfully"

# 7. Verify status changed back
echo "7. Verifying status changed back to detached..."
DISCOVER=$(curl -s "$RELAY_URL/api/agents/discover")
STATUS=$(echo "$DISCOVER" | jq -r ".agents[] | select(.agentId == \"$AGENT_ID\") | .status")
if [ "$STATUS" != "detached" ]; then
    echo "FAIL: Expected status 'detached', got '$STATUS'"
    exit 1
fi
echo "✓ Status is detached"

# 8. Verify agent is still running
echo "8. Verifying agent still running after detach..."
agentd list | grep "$AGENT_ID" || {
    echo "FAIL: Agent was terminated instead of detached"
    exit 1
}
echo "✓ Agent is still running"

echo ""
echo "✨ All tests passed!"
```

**Acceptance Criteria**:
- [ ] Script completes without errors
- [ ] Agent can be discovered after CLI spawn
- [ ] Agent can be attached and status reflects change
- [ ] Agent can be detached and status reflects change
- [ ] Agent continues running after detach

---

---

## Phase 2: NATS Heartbeats (Week 3)

**Goal**: Liveness detection and automatic lease cleanup
**Milestone**: Orphaned agents are automatically detected and cleaned up

### Task 2.1: Add Heartbeat Publisher to Agent

**Estimated Time**: 4 hours

**Files to Create**:
- `pkg/agent/heartbeat.go`
- `pkg/agent/heartbeat_test.go`

**Files to Modify**:
- `pkg/agent/agent.go` (start heartbeat goroutine)

**What to Do**:
1. Implement heartbeat publisher that connects to NATS
2. Publish on `agent.heartbeat.{agent-id}` every 30s
3. Start heartbeat goroutine on agent startup
4. Gracefully stop on shutdown

**Code Structure**:
```go
// pkg/agent/heartbeat.go
package agent

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/nats-io/nats.go"
)

const (
    HeartbeatInterval = 30 * time.Second
    HeartbeatSubject  = "agent.heartbeat.%s" // agent-id
)

type Heartbeat struct {
    AgentID   string    `json:"agentId"`
    Timestamp time.Time `json:"timestamp"`
    Status    string    `json:"status"`
}

type HeartbeatPublisher struct {
    agentID string
    nats    *nats.Conn
    cancel  context.CancelFunc
}

// NewHeartbeatPublisher creates a new heartbeat publisher.
func NewHeartbeatPublisher(agentID, natsURL string) (*HeartbeatPublisher, error) {
    nc, err := nats.Connect(natsURL)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to NATS: %w", err)
    }

    return &HeartbeatPublisher{
        agentID: agentID,
        nats:    nc,
    }, nil
}

// Start begins publishing heartbeats.
func (h *HeartbeatPublisher) Start(ctx context.Context) {
    ctx, cancel := context.WithCancel(ctx)
    h.cancel = cancel

    subject := fmt.Sprintf(HeartbeatSubject, h.agentID)
    ticker := time.NewTicker(HeartbeatInterval)
    defer ticker.Stop()

    // Send initial heartbeat immediately
    h.publish(subject)

    for {
        select {
        case <-ticker.C:
            h.publish(subject)
        case <-ctx.Done():
            return
        }
    }
}

func (h *HeartbeatPublisher) publish(subject string) {
    hb := Heartbeat{
        AgentID:   h.agentID,
        Timestamp: time.Now(),
        Status:    "active",
    }

    data, err := json.Marshal(hb)
    if err != nil {
        // Log error but don't crash
        fmt.Printf("Failed to marshal heartbeat: %v\n", err)
        return
    }

    if err := h.nats.Publish(subject, data); err != nil {
        // Log error but don't crash
        fmt.Printf("Failed to publish heartbeat: %v\n", err)
    }
}

// Stop halts heartbeat publishing.
func (h *HeartbeatPublisher) Stop() {
    if h.cancel != nil {
        h.cancel()
    }
    if h.nats != nil {
        h.nats.Close()
    }
}
```

**Wire to Agent**:
```go
// In pkg/agent/agent.go (or wherever agent main loop is)

func (a *Agent) Start(ctx context.Context) error {
    // ... existing agent startup ...

    // Start heartbeat publisher
    hb, err := NewHeartbeatPublisher(a.ID, a.config.NATSURL)
    if err != nil {
        return fmt.Errorf("failed to create heartbeat publisher: %w", err)
    }
    go hb.Start(ctx)
    defer hb.Stop()

    // ... rest of agent logic ...
}
```

**Testing**:
```bash
# Subscribe to heartbeat subject
nats sub "agent.heartbeat.test-agent"

# Spawn agent
agentd spawn test-agent

# Should see heartbeats every 30s:
# {"agentId":"test-agent","timestamp":"2025-11-20T10:30:00Z","status":"active"}
```

**Acceptance Criteria**:
- [ ] Heartbeat publishes to `agent.heartbeat.{agent-id}`
- [ ] First heartbeat sent immediately on start
- [ ] Subsequent heartbeats every 30 seconds
- [ ] Heartbeat stops when agent stops
- [ ] Failures to publish are logged but don't crash agent

---

### Task 2.2: Add Heartbeat Monitor to Relay

**Estimated Time**: 6 hours

**Files to Create**:
- `pkg/relay/session/heartbeat_monitor.go`
- `pkg/relay/session/heartbeat_monitor_test.go`

**Files to Modify**:
- `pkg/relay/relay.go` (start monitor on relay startup)

**What to Do**:
1. Implement heartbeat subscriber
2. Track last-seen timestamp for each agent
3. Renew lease on heartbeat if agent is attached
4. Background reaper for expired leases

**Code Structure**:
```go
// pkg/relay/session/heartbeat_monitor.go
package session

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "github.com/nats-io/nats.go"
)

type HeartbeatMonitor struct {
    nats      *nats.Conn
    sub       *nats.Subscription
    lastSeen  map[string]time.Time
    mu        sync.RWMutex
}

// NewHeartbeatMonitor creates a new heartbeat monitor.
func NewHeartbeatMonitor(natsURL string) (*HeartbeatMonitor, error) {
    nc, err := nats.Connect(natsURL)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to NATS: %w", err)
    }

    return &HeartbeatMonitor{
        nats:     nc,
        lastSeen: make(map[string]time.Time),
    }, nil
}

// Start begins monitoring heartbeats.
func (h *HeartbeatMonitor) Start(ctx context.Context) error {
    // Subscribe to all agent heartbeats
    sub, err := h.nats.Subscribe("agent.heartbeat.*", func(msg *nats.Msg) {
        var hb struct {
            AgentID   string    `json:"agentId"`
            Timestamp time.Time `json:"timestamp"`
            Status    string    `json:"status"`
        }

        if err := json.Unmarshal(msg.Data, &hb); err != nil {
            fmt.Printf("Failed to unmarshal heartbeat: %v\n", err)
            return
        }

        h.updateLastSeen(hb.AgentID, hb.Timestamp)
        h.renewLeaseIfAttached(hb.AgentID)
    })
    if err != nil {
        return fmt.Errorf("failed to subscribe to heartbeats: %w", err)
    }
    h.sub = sub

    // Start background reaper
    go h.reapExpiredLeases(ctx)

    return nil
}

func (h *HeartbeatMonitor) updateLastSeen(agentID string, timestamp time.Time) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.lastSeen[agentID] = timestamp
}

func (h *HeartbeatMonitor) renewLeaseIfAttached(agentID string) {
    // Check if lease exists
    _, err := ReadLease(agentID)
    if err != nil {
        return // No lease = agent is detached, nothing to renew
    }

    // Renew lease
    if err := RenewLease(agentID); err != nil {
        fmt.Printf("Failed to renew lease for agent %s: %v\n", agentID, err)
    }
}

func (h *HeartbeatMonitor) reapExpiredLeases(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            h.removeExpiredLeases()
        case <-ctx.Done():
            return
        }
    }
}

func (h *HeartbeatMonitor) removeExpiredLeases() {
    leases, err := ListLeases()
    if err != nil {
        fmt.Printf("Failed to list leases: %v\n", err)
        return
    }

    for _, lease := range leases {
        if IsLeaseExpired(lease) {
            fmt.Printf("Reaping expired lease for agent %s\n", lease.AgentID)
            if err := ReleaseLease(lease.AgentID); err != nil {
                fmt.Printf("Failed to release expired lease: %v\n", err)
            }
        }
    }
}

// Stop halts heartbeat monitoring.
func (h *HeartbeatMonitor) Stop() {
    if h.sub != nil {
        h.sub.Unsubscribe()
    }
    if h.nats != nil {
        h.nats.Close()
    }
}
```

**Wire to Relay**:
```go
// In pkg/relay/relay.go

func (r *Relay) Start(ctx context.Context) error {
    // ... existing relay startup ...

    // Start heartbeat monitor
    monitor, err := session.NewHeartbeatMonitor(r.config.NATSURL)
    if err != nil {
        return fmt.Errorf("failed to create heartbeat monitor: %w", err)
    }
    if err := monitor.Start(ctx); err != nil {
        return fmt.Errorf("failed to start heartbeat monitor: %w", err)
    }
    defer monitor.Stop()

    // ... rest of relay logic ...
}
```

**Testing**:
```bash
# Spawn and attach agent
agentd spawn test-agent
curl -X POST http://localhost:8080/api/agents/attach -d '{"agentId": "test-agent"}'

# Check lease file
cat .agentd/session/test-agent.lease | jq .expiresAt

# Wait 60 seconds (heartbeat should renew lease)
sleep 60

# Check lease file again (expiresAt should be extended)
cat .agentd/session/test-agent.lease | jq .expiresAt

# Kill agent
agentd stop test-agent

# Wait 6 minutes (lease should expire and be reaped)
sleep 360

# Verify lease file removed
ls .agentd/session/test-agent.lease
# Expected: file not found
```

**Acceptance Criteria**:
- [ ] Monitor subscribes to `agent.heartbeat.*`
- [ ] Lease is renewed on heartbeat if agent is attached
- [ ] Expired leases are reaped every minute
- [ ] Monitor gracefully stops on relay shutdown
- [ ] Orphaned agents (crashed, no heartbeat) have leases expire after 5 minutes

---

## Phase 3: ACP Communication Bridge (Week 4)

**Goal**: Full bidirectional communication between PWA and CLI agents
**Milestone**: PWA can control CLI agents just like PWA-spawned agents

### Task 3.1: Implement ACP Client Wrapper

**Estimated Time**: 6 hours

**Files to Create**:
- `pkg/relay/session/acp_bridge.go`
- `pkg/relay/session/acp_bridge_test.go`

**What to Do**:
1. Create ACPClient interface wrapper
2. Implement connection to agent via Docker exec or NATS
3. Implement Send() and Receive() methods
4. Handle connection lifecycle (connect, disconnect, reconnect)

**Code Structure**:
```go
// pkg/relay/session/acp_bridge.go
package session

import (
    "context"
    "encoding/json"
    "fmt"
    "io"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/client"
)

type ACPBridge struct {
    containerID string
    stdin       io.WriteCloser
    stdout      io.ReadCloser
    cancel      context.CancelFunc
}

// NewACPBridge creates a new ACP bridge to a containerized agent.
func NewACPBridge(ctx context.Context, containerID string) (*ACPBridge, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return nil, err
    }
    defer cli.Close()

    // Create exec instance for ACP communication
    execConfig := types.ExecConfig{
        Cmd:          []string{"/bin/acp-client"}, // or however agent exposes ACP
        AttachStdin:  true,
        AttachStdout: true,
        Tty:          false,
    }

    execResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create exec: %w", err)
    }

    attachResp, err := cli.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
    if err != nil {
        return nil, fmt.Errorf("failed to attach to exec: %w", err)
    }

    ctx, cancel := context.WithCancel(ctx)

    return &ACPBridge{
        containerID: containerID,
        stdin:       attachResp.Conn,
        stdout:      attachResp.Reader,
        cancel:      cancel,
    }, nil
}

// Send sends an ACP message to the agent.
func (b *ACPBridge) Send(payload json.RawMessage) error {
    _, err := b.stdin.Write(payload)
    return err
}

// Receive reads an ACP response from the agent.
func (b *ACPBridge) Receive() (json.RawMessage, error) {
    buf := make([]byte, 4096)
    n, err := b.stdout.Read(buf)
    if err != nil {
        return nil, err
    }
    return json.RawMessage(buf[:n]), nil
}

// Close closes the ACP bridge.
func (b *ACPBridge) Close() error {
    b.cancel()
    if err := b.stdin.Close(); err != nil {
        return err
    }
    return b.stdout.Close()
}
```

**Acceptance Criteria**:
- [ ] Can create ACP bridge to containerized agent
- [ ] Send() writes to agent stdin
- [ ] Receive() reads from agent stdout
- [ ] Close() cleans up resources

---

### Task 3.2: Wire ACP Bridge to AttachAgent()

**Estimated Time**: 4 hours

**Files to Modify**:
- `pkg/relay/session/models.go`

**What to Do**:
1. Modify `AttachAgent()` to create ACP bridge on attach
2. Store bridge in `AgentSession.acpClient`
3. Clean up bridge on detach

**Code Changes**:
```go
// In pkg/relay/session/models.go

func (us *UserSession) AttachAgent(agentID string) (*AgentSession, error) {
    us.mu.Lock()
    defer us.mu.Unlock()

    // Check if already attached
    if existing, ok := us.agents[agentID]; ok {
        return existing, nil
    }

    // Acquire lease (from Phase 1)
    lease, err := AcquireLease(agentID, us.ID)
    if err != nil {
        return nil, err
    }

    // Get container ID from Docker
    containerID, workspace, err := findAgentInfo(context.Background(), agentID)
    if err != nil {
        ReleaseLease(agentID)
        return nil, fmt.Errorf("failed to find agent: %w", err)
    }

    // Create ACP bridge
    acpBridge, err := NewACPBridge(context.Background(), containerID)
    if err != nil {
        ReleaseLease(agentID)
        return nil, fmt.Errorf("failed to create ACP bridge: %w", err)
    }

    agent := &AgentSession{
        AgentID:    agentID,
        Workspace:  workspace,
        createdAt:  lease.AttachedAt,
        state:      AgentActive,
        acpClient:  acpBridge, // Now implements ACPClient interface
        lastActive: time.Now(),
        history:    []Message{},
    }

    us.agents[agentID] = agent
    us.lastActive = time.Now()

    return agent, nil
}

func (us *UserSession) DetachAgent(agentID string) error {
    us.mu.Lock()
    defer us.mu.Unlock()

    agent, ok := us.agents[agentID]
    if !ok {
        return nil // Already detached
    }

    // Close ACP bridge
    if agent.acpClient != nil {
        _ = agent.acpClient.Close()
    }

    // Release lease
    if err := ReleaseLease(agentID); err != nil {
        return err
    }

    delete(us.agents, agentID)
    us.lastActive = time.Now()

    return nil
}
```

**Acceptance Criteria**:
- [ ] AttachAgent() creates ACP bridge and stores in AgentSession
- [ ] DetachAgent() closes ACP bridge before releasing lease
- [ ] Attach failure cleans up lease and bridge

---

### Task 3.3: Add WebSocket Message Types

**Estimated Time**: 4 hours

**Files to Create**:
- `pkg/relay/protocol/agent_messages.go`

**Files to Modify**:
- `pkg/relay/session/user_session.go` (add message handlers)

**What to Do**:
1. Define agent:command and agent:response message types
2. Add message handlers to UserSession
3. Route messages to/from ACP bridge

**Code Structure**:
```go
// pkg/relay/protocol/agent_messages.go
package protocol

type AgentCommand struct {
    Type      string          `json:"type"` // "agent:command"
    AgentID   string          `json:"agentId"`
    CommandID string          `json:"commandId"`
    Payload   json.RawMessage `json:"payload"`
}

type AgentResponse struct {
    Type      string          `json:"type"` // "agent:response"
    AgentID   string          `json:"agentId"`
    CommandID string          `json:"commandId"`
    Payload   json.RawMessage `json:"payload"`
    Error     string          `json:"error,omitempty"`
}

type AgentDetachRequest struct {
    Type    string `json:"type"` // "agent:detach"
    AgentID string `json:"agentId"`
}
```

**Message Handler**:
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
        return us.handleAgentDetachRequest(msg)
    // ... existing message types ...
    default:
        return fmt.Errorf("unknown message type: %s", msgType.Type)
    }
}

func (us *UserSession) handleAgentCommand(msg []byte) error {
    var cmd protocol.AgentCommand
    if err := json.Unmarshal(msg, &cmd); err != nil {
        return err
    }

    // Get agent session
    us.mu.RLock()
    agent, ok := us.agents[cmd.AgentID]
    us.mu.RUnlock()

    if !ok {
        return us.sendAgentError(cmd.AgentID, cmd.CommandID, "agent not attached")
    }

    // Forward to ACP client
    if err := agent.acpClient.Send(cmd.Payload); err != nil {
        return us.sendAgentError(cmd.AgentID, cmd.CommandID, err.Error())
    }

    // Read response (blocking - consider goroutine for async)
    resp, err := agent.acpClient.Receive()
    if err != nil {
        return us.sendAgentError(cmd.AgentID, cmd.CommandID, err.Error())
    }

    return us.sendAgentResponse(cmd.AgentID, cmd.CommandID, resp)
}

func (us *UserSession) sendAgentResponse(agentID, commandID string, payload json.RawMessage) error {
    resp := protocol.AgentResponse{
        Type:      "agent:response",
        AgentID:   agentID,
        CommandID: commandID,
        Payload:   payload,
    }

    data, err := json.Marshal(resp)
    if err != nil {
        return err
    }

    return us.webSocket.WriteMessage(websocket.TextMessage, data)
}

func (us *UserSession) sendAgentError(agentID, commandID, errMsg string) error {
    resp := protocol.AgentResponse{
        Type:      "agent:response",
        AgentID:   agentID,
        CommandID: commandID,
        Error:     errMsg,
    }

    data, err := json.Marshal(resp)
    if err != nil {
        return err
    }

    return us.webSocket.WriteMessage(websocket.TextMessage, data)
}

func (us *UserSession) handleAgentDetachRequest(msg []byte) error {
    var req protocol.AgentDetachRequest
    if err := json.Unmarshal(msg, &req); err != nil {
        return err
    }

    return us.DetachAgent(req.AgentID)
}
```

**Acceptance Criteria**:
- [ ] agent:command messages route to attached agent's ACP bridge
- [ ] agent:response messages return to PWA with matching commandID
- [ ] Errors are returned as agent:response with error field
- [ ] agent:detach messages trigger DetachAgent()

---

### Task 3.4: End-to-End Communication Tests

**Estimated Time**: 4 hours

**Files to Create**:
- `scripts/test-agent-communication.sh`

**What to Do**:
1. Spawn agent via CLI
2. Attach via API
3. Send command via WebSocket
4. Verify response received
5. Detach and verify agent still running

**Test Script**:
```bash
#!/bin/bash
# scripts/test-agent-communication.sh

set -e

RELAY_URL="http://localhost:8080"
WS_URL="ws://localhost:8080/ws"
AGENT_ID="test-comm-$(date +%s)"

echo "=== Agent Communication Integration Test ==="

cleanup() {
    echo "Cleaning up..."
    agentd stop "$AGENT_ID" 2>/dev/null || true
}
trap cleanup EXIT

# 1. Spawn agent
echo "1. Spawning agent..."
agentd spawn "$AGENT_ID"
sleep 2

# 2. Attach agent
echo "2. Attaching agent..."
curl -X POST "$RELAY_URL/api/agents/attach" \
    -d "{\"agentId\": \"$AGENT_ID\"}" | jq

# 3. Send command via WebSocket (using wscat or similar)
echo "3. Sending command to agent..."
# This requires WebSocket client - wscat, websocat, or custom script
echo '{"type":"agent:command","agentId":"'$AGENT_ID'","commandId":"cmd-1","payload":"ls"}' | \
    websocat "$WS_URL" --one-message

# Expected response:
# {"type":"agent:response","agentId":"test-comm-...","commandId":"cmd-1","payload":"..."}

echo "✨ Communication test passed!"
```

**Acceptance Criteria**:
- [ ] Commands sent via WebSocket reach agent
- [ ] Responses return via WebSocket with correct commandID
- [ ] Multiple commands can be sent sequentially
- [ ] Detaching stops communication but doesn't kill agent

---

## Phase 4: Security Hardening (Week 5)

**Goal**: Production-ready security and auth
**Milestone**: Production-ready with full security controls

### Task 4.1: Generate Attach Tokens

**Estimated Time**: 3 hours

**Files to Modify**:
- `cmd/agentd/cmd_spawn.go`

**What to Do**:
1. Generate cryptographically secure token on spawn
2. Write token to `.agentd/session/{agent-id}.token`
3. Display token to user

**Code Changes**:
```go
// In cmd/agentd/cmd_spawn.go

func generateAttachToken(agentID string) (string, error) {
    // Generate 32 random bytes
    tokenBytes := make([]byte, 32)
    if _, err := rand.Read(tokenBytes); err != nil {
        return "", fmt.Errorf("failed to generate token: %w", err)
    }

    tokenStr := base64.URLEncoding.EncodeToString(tokenBytes)

    // Ensure session directory exists
    sessionDir := ".agentd/session"
    if err := os.MkdirAll(sessionDir, 0700); err != nil {
        return "", fmt.Errorf("failed to create session directory: %w", err)
    }

    // Write token to file
    tokenPath := filepath.Join(sessionDir, agentID+".token")
    if err := os.WriteFile(tokenPath, []byte(tokenStr), 0600); err != nil {
        return "", fmt.Errorf("failed to write token: %w", err)
    }

    return tokenStr, nil
}

func runSpawn(cmd *cobra.Command, args []string) error {
    // ... existing spawn logic ...

    // Generate attach token
    token, err := generateAttachToken(agentID)
    if err != nil {
        return err
    }

    fmt.Println()
    _, _ = color.New(color.FgCyan, color.Bold).Println("🔑 Attach Token:")
    fmt.Printf("  %s\n", token)
    fmt.Println()
    _, _ = color.New(color.FgHiBlack).Println("  Use this token when attaching from PWA:")
    fmt.Printf("  POST /api/agents/attach -d '{\"agentId\": \"%s\", \"token\": \"%s\"}'\n", agentID, token)
    fmt.Println()

    return nil
}
```

**Acceptance Criteria**:
- [ ] Token is 32 random bytes (256-bit)
- [ ] Token file has 0600 permissions
- [ ] Token is displayed to user after spawn
- [ ] Token persists across agent restarts

---

### Task 4.2: Add Token Verification to Attach

**Estimated Time**: 3 hours

**Files to Modify**:
- `pkg/relay/api/handlers_agents.go`

**What to Do**:
1. Update AttachRequest to include token field
2. Verify token before acquiring lease
3. Return 403 if token invalid

**Code Changes**:
```go
// In pkg/relay/api/handlers_agents.go

type AttachRequest struct {
    AgentID string `json:"agentId"`
    Token   string `json:"token"` // Required
}

func (s *Server) HandleAgentAttach(w http.ResponseWriter, r *http.Request) {
    var req AttachRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if req.AgentID == "" || req.Token == "" {
        http.Error(w, "agentId and token are required", http.StatusBadRequest)
        return
    }

    // Verify token BEFORE acquiring lease
    if !verifyAgentToken(req.AgentID, req.Token) {
        http.Error(w, "Invalid attach token", http.StatusForbidden)
        return
    }

    // ... proceed with attach (existing logic) ...
}

func verifyAgentToken(agentID, providedToken string) bool {
    tokenPath := filepath.Join(".agentd/session", agentID+".token")
    expectedToken, err := os.ReadFile(tokenPath)
    if err != nil {
        return false
    }

    // Constant-time comparison to prevent timing attacks
    return subtle.ConstantTimeCompare([]byte(providedToken), expectedToken) == 1
}
```

**Acceptance Criteria**:
- [ ] Attach without token returns 400
- [ ] Attach with invalid token returns 403
- [ ] Attach with valid token succeeds
- [ ] Token comparison is constant-time (prevents timing attacks)

---

### Task 4.3: Add Audit Logging

**Estimated Time**: 3 hours

**Files to Create**:
- `pkg/relay/audit/logger.go`

**Files to Modify**:
- `pkg/relay/session/models.go` (add audit calls)

**What to Do**:
1. Implement structured audit logger
2. Log attach/detach operations with user/agent IDs
3. Log auth failures

**Code Structure**:
```go
// pkg/relay/audit/logger.go
package audit

import (
    "encoding/json"
    "log"
    "time"
)

type Event struct {
    Timestamp time.Time         `json:"timestamp"`
    Type      string            `json:"type"` // "agent:attach", "agent:detach", "auth:failure"
    UserID    string            `json:"userId"`
    AgentID   string            `json:"agentId,omitempty"`
    Success   bool              `json:"success"`
    Error     string            `json:"error,omitempty"`
    Metadata  map[string]string `json:"metadata,omitempty"`
}

func Log(event Event) {
    event.Timestamp = time.Now()
    data, _ := json.Marshal(event)
    log.Printf("[AUDIT] %s", data)
}
```

**Usage**:
```go
// In pkg/relay/session/models.go

func (us *UserSession) AttachAgent(agentID string) (*AgentSession, error) {
    audit.Log(audit.Event{
        Type:    "agent:attach",
        UserID:  us.ID,
        AgentID: agentID,
        Success: false, // Will update on success
    })

    // ... attach logic ...

    audit.Log(audit.Event{
        Type:    "agent:attach",
        UserID:  us.ID,
        AgentID: agentID,
        Success: true,
    })

    return agent, nil
}
```

**Acceptance Criteria**:
- [ ] All attach/detach operations logged
- [ ] Auth failures logged with reason
- [ ] Logs are structured JSON
- [ ] Logs include timestamp, user ID, agent ID, success/failure

---

### Task 4.4: Add Rate Limiting

**Estimated Time**: 3 hours

**Files to Create**:
- `pkg/relay/api/middleware.go`

**Files to Modify**:
- `pkg/relay/api/router.go` (apply middleware)

**What to Do**:
1. Implement rate limiting middleware
2. Apply to agent endpoints
3. Return 429 if rate exceeded

**Code Structure**:
```go
// pkg/relay/api/middleware.go
package api

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.Mutex
    rate     rate.Limit
    burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     r,
        burst:    b,
    }
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.limiters[key]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[key] = limiter
    }

    return limiter
}

func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Rate limit by IP or user ID
        key := r.RemoteAddr

        limiter := rl.getLimiter(key)
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }

        next(w, r)
    }
}
```

**Apply to Routes**:
```go
// In pkg/relay/api/router.go

func (s *Server) setupRoutes() {
    // Create rate limiter: 10 requests per second with burst of 20
    rateLimiter := NewRateLimiter(rate.Every(time.Second/10), 20)

    // Apply to agent endpoints
    s.mux.HandleFunc("/api/agents/discover", rateLimiter.Middleware(s.HandleAgentDiscover))
    s.mux.HandleFunc("/api/agents/attach", rateLimiter.Middleware(s.HandleAgentAttach))
    s.mux.HandleFunc("/api/agents/detach", rateLimiter.Middleware(s.HandleAgentDetach))
}
```

**Acceptance Criteria**:
- [ ] Rate limiter tracks requests per IP/user
- [ ] Returns 429 when rate exceeded
- [ ] Burst requests allowed up to burst limit
- [ ] Limits reset over time

---

## Summary

This implementation plan now covers 4 complete phases plus an optional 5th phase:

**Phase 1 (Week 1-2)**: Discovery and attach/detach infrastructure
**Phase 2 (Week 3)**: Heartbeats and automatic cleanup
**Phase 3 (Week 4)**: Full bidirectional PWA ↔ CLI agent communication
**Phase 4 (Week 5)**: Production security (tokens, auth, audit, rate limiting)
**Phase 5 (Week 6+)**: Optional performance optimization with in-memory registry

**Total Timeline**: 5 weeks for production-ready implementation, 6+ weeks if registry optimization needed
