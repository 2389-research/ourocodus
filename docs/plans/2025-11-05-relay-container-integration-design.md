# Relay Container Integration Design

**Date:** 2025-11-05
**Issues:** #107 (Relay Integration), #108 (E2E Tests)
**Status:** Approved via Zen Consensus (9/10, 9/10, 8/10 confidence)
**Dependencies:** #105 (ContainerLauncher), #106 (Worktree Manager), #109 (Credentials) - COMPLETED

## Executive Summary

This design integrates the AgentContainerLauncher into the Relay server using a LauncherFactory pattern, enabling containerized agent execution with full isolation, credential mounting, and git worktree management. The approach was validated by multi-model consensus with high confidence scores.

## Goals

1. Replace direct agent spawning in Relay with containerized execution via AgentContainerLauncher
2. Maintain existing UserSession/AgentSession semantics and WebSocket protocol
3. Enable comprehensive E2E testing of containerized agent lifecycle
4. Provide clean abstractions for multiple launcher types (container, process, mock)
5. Ensure robust error handling and resource cleanup

## Architecture

### Component Structure

```
pkg/agent/
  ├── launcher.go          (existing AgentLauncher interface)
  ├── factory.go           (NEW: LauncherFactory interface + DefaultFactory)
  └── container/
      └── launcher.go      (existing AgentContainerLauncher)

pkg/relay/
  ├── server.go           (inject LauncherFactory)
  └── session/
      └── manager.go      (use factory to create launchers)

tests/e2e/
  ├── container_spawn_test.go        (NEW)
  ├── container_lifecycle_test.go    (NEW)
  ├── container_credentials_test.go  (NEW)
  ├── container_worktree_test.go     (NEW)
  ├── container_concurrent_test.go   (NEW)
  └── helpers/
      └── docker.go                  (NEW)
```

### LauncherFactory Pattern

**Why Factory Pattern?**
- Decouples launcher creation from usage
- Enables different launcher types per agent requirements
- Simplifies testing with MockLauncherFactory
- Future-proofs for process-based or remote launchers
- Consensus validated as appropriate (not over-engineering)

**LauncherFactory Interface:**
```go
type LauncherFactory interface {
    CreateLauncher(agentType string, config LauncherConfig) (AgentLauncher, error)
}

type LauncherConfig struct {
    DockerClient   *docker.Client
    WorktreeManager *worktree.Manager
    CredMounter    *container.CredentialMounter
    BaseWorkspace  string
    ImageName      string
    ResourceLimits ResourceLimits
}

type DefaultLauncherFactory struct {
    dockerClient    *docker.Client
    worktreeManager *worktree.Manager
    credMounter     *container.CredentialMounter
    config          LauncherFactoryConfig
}
```

**Implementation:**
- DefaultFactory creates AgentContainerLauncher instances
- MockFactory returns mock launchers for testing
- Future factories can support process-based or remote execution

### SessionManager Integration

**State Changes:**
```go
type Manager struct {
    // Existing fields
    store          SessionStore
    clientFactory  ClientFactory
    // ...

    // NEW fields
    launcherFactory AgentLauncherFactory
    launchers       map[string]agent.AgentLauncher  // agentID → launcher
    handles         map[string]agent.AgentHandle     // agentID → handle
    launchersMu     sync.RWMutex                     // protects launchers/handles maps
}
```

**SpawnAgent() Flow:**
```go
func (m *Manager) SpawnAgent(ctx context.Context, sessionID, agentID, workspace string) error {
    // 1. Validate inputs (existing logic)
    // 2. Get UserSession (existing logic)

    // 3. Create launcher via factory (NEW)
    launcher, err := m.launcherFactory.CreateLauncher("container", LauncherConfig{
        AgentID:   agentID,
        Workspace: workspace,
        // ... config from session
    })
    if err != nil {
        return fmt.Errorf("failed to create launcher: %w", err)
    }

    // 4. Spawn agent container (NEW)
    spawnConfig := &agent.SpawnConfig{
        AgentID:     agentID,
        ImageName:   m.config.AgentImage,
        Command:     m.config.AgentCommand,
        GitSSHKey:   session.GitSSHKey,
        GitHubToken: session.GitHubToken,
        Workspace:   workspace,
    }

    handle, err := launcher.Spawn(ctx, spawnConfig)
    if err != nil {
        return fmt.Errorf("failed to spawn agent: %w", err)
    }

    // 5. Store launcher and handle (NEW)
    m.launchersMu.Lock()
    m.launchers[agentID] = launcher
    m.handles[agentID] = handle
    m.launchersMu.Unlock()

    // 6. Add to session (existing logic)
    // 7. Publish events (existing logic)

    return nil
}
```

**TerminateAgent() Updates:**
```go
func (m *Manager) TerminateAgent(ctx context.Context, sessionID, agentID string) error {
    // 1. Validate and get session (existing)

    // 2. Stop container (NEW)
    m.launchersMu.RLock()
    launcher := m.launchers[agentID]
    handle := m.handles[agentID]
    m.launchersMu.RUnlock()

    if launcher != nil && handle != nil {
        if err := launcher.Stop(ctx, handle); err != nil {
            m.logger.Printf("WARN: Failed to stop container: %v", err)
            // Continue cleanup despite error
        }
    }

    // 3. Remove from maps (NEW)
    m.launchersMu.Lock()
    delete(m.launchers, agentID)
    delete(m.handles, agentID)
    m.launchersMu.Unlock()

    // 4. Remove from session (existing)
    // 5. Publish events (existing)

    return nil
}
```

## Data Flow

### agent:spawn Message Flow

```
1. PWA → Relay: {"type": "agent:spawn", "agentId": "coder-123", "workspace": "/workspace/myrepo"}
   ↓
2. Relay.handleAgentSpawn()
   - Parse and validate message
   - Call SessionManager.SpawnAgent()
   ↓
3. SessionManager.SpawnAgent()
   - Validate workspace path
   - Get UserSession
   - Create launcher via factory
   ↓
4. LauncherFactory.CreateLauncher()
   - Return AgentContainerLauncher instance
   ↓
5. AgentContainerLauncher.Spawn()
   ├─ WorktreeManager.Create() → isolated git worktree + branch
   ├─ CredentialMounter.Mount() → read-only credential files
   └─ ContainerSession.Start() → Docker container with mounts
   ↓
6. SessionManager stores handle
   - Add to launchers/handles maps
   - Add agent to UserSession
   ↓
7. Relay → PWA: {"type": "agent:ready", "agentId": "coder-123"}
```

### agent:terminate Message Flow

```
1. PWA → Relay: {"type": "agent:terminate", "agentId": "coder-123"}
   ↓
2. SessionManager.TerminateAgent()
   - Get launcher and handle from maps
   - Call launcher.Stop(handle)
   ↓
3. AgentContainerLauncher.Stop()
   ├─ ContainerSession.Stop() → graceful container shutdown
   ├─ CredentialMounter.Unmount() → remove credential files
   └─ WorktreeManager.Remove() → clean up worktree
   ↓
4. SessionManager cleanup
   - Remove from launchers/handles maps
   - Remove from UserSession
   ↓
5. Relay → PWA: {"type": "agent:terminated", "agentId": "coder-123"}
```

## Error Handling

### Error Categories

1. **Container Spawn Failures**
   - **Scenarios:** Docker daemon unreachable, image not found, resource limits
   - **Code:** `AGENT_SPAWN_FAILED`
   - **Recovery:** Clean up partial resources (worktree, credentials), return error
   - **Retryable:** Yes (after fixing Docker/config)

2. **Credential Mounting Errors**
   - **Scenarios:** Missing credentials, invalid paths, permission denied
   - **Code:** `CREDENTIAL_ERROR`
   - **Recovery:** Validate before spawn, return specific missing items
   - **Retryable:** Yes (after providing credentials)

3. **Worktree Creation Conflicts**
   - **Scenarios:** Branch exists, git errors, disk full
   - **Code:** `WORKSPACE_ERROR`
   - **Recovery:** Generate unique branch name with timestamp retry
   - **Retryable:** Yes (automatic retry with new branch name)

4. **Orphaned Containers**
   - **Scenarios:** Relay crash, process kill, system reboot
   - **Detection:** Container labels (`ourocodus.agent=true`, `ourocodus.session-id=<id>`)
   - **Recovery:** On startup, list labeled containers and stop/remove
   - **Logging:** Log all orphan cleanups for debugging

5. **Resource Exhaustion**
   - **Scenarios:** Too many containers, disk full, memory exceeded
   - **Code:** `RESOURCE_LIMIT`
   - **Recovery:** Implement max concurrent agents, clean up idle containers
   - **Retryable:** Yes (after cleanup or resource increase)

6. **Concurrent Spawn/Stop Race**
   - **Scenarios:** Stop() called during Spawn()
   - **Protection:** Per-agent mutex, context cancellation
   - **Recovery:** Graceful abort, ensure cleanup runs
   - **Code:** `OPERATION_CONFLICT`

### AgentSession State Machine

```
SPAWNING → RUNNING (spawn success)
SPAWNING → FAILED (spawn error, cleanup triggered)
RUNNING → STOPPING (stop requested)
RUNNING → FAILED (runtime error, cleanup triggered)
STOPPING → STOPPED (cleanup complete)
FAILED → cleanup resources
STOPPED → cleanup resources (idempotent)
```

**Cleanup Guarantees:**
- Always run in defer to handle panics
- Idempotent (safe to call multiple times)
- Log all cleanup operations
- Never leave orphaned resources

## E2E Test Suite

### Test Organization (Scenario-Based)

```
tests/e2e/
  ├── container_spawn_test.go
  ├── container_lifecycle_test.go
  ├── container_credentials_test.go
  ├── container_worktree_test.go
  ├── container_concurrent_test.go
  └── helpers/
      └── docker.go
```

### Test Scenarios

**1. container_spawn_test.go**
- `TestContainerSpawn_EchoAgent` - Basic container spawn with echo-agent
- `TestContainerSpawn_ClaudeCodeAgent` - Real Claude Code agent via ACP
- `TestContainerSpawn_InvalidImage` - Error handling for missing images

**2. container_lifecycle_test.go**
- `TestContainerLifecycle_Stop` - Spawn, interact, stop, verify cleanup
- `TestContainerLifecycle_Attach` - Attach to existing container
- `TestContainerLifecycle_CrashRecovery` - Orphan container cleanup

**3. container_credentials_test.go**
- `TestCredentials_GitHubCLI` - `gh auth status` inside container
- `TestCredentials_GitSSHKey` - Git operations with SSH key
- `TestCredentials_AnthropicAPIKey` - Claude API calls work
- `TestCredentials_ReadOnlyMount` - Credentials immutable

**4. container_worktree_test.go**
- `TestWorktree_IsolatedBranches` - Each agent gets unique worktree/branch
- `TestWorktree_CommitPropagation` - Commits visible in worktree
- `TestWorktree_CleanupOnStop` - Worktree removed on stop

**5. container_concurrent_test.go**
- `TestConcurrent_MultipleAgents` - 3+ agents in one UserSession
- `TestConcurrent_RaceConditions` - Concurrent spawn/stop operations

### Test Helpers (docker.go)

```go
// WaitForContainer polls until container is running
func WaitForContainer(ctx context.Context, containerID string, timeout time.Duration) error

// VerifyContainerCleanup ensures container is removed
func VerifyContainerCleanup(ctx context.Context, containerID string) error

// GetContainerLogs fetches logs for debugging
func GetContainerLogs(ctx context.Context, containerID string) (string, error)

// InspectContainer gets container state/config
func InspectContainer(ctx context.Context, containerID string) (*ContainerInfo, error)

// ListAgentContainers finds all ourocodus agent containers
func ListAgentContainers(ctx context.Context) ([]string, error)
```

### Test Principles

- Use `//go:build integration` tag (Docker required)
- Each test file independent and runnable alone
- Cleanup in `defer` to prevent container leaks
- Use testdata/ for test agent binaries
- Leverage existing helpers/ for relay/WebSocket
- Add detailed logging for debugging failures

## Implementation Checklist

### Phase 1: LauncherFactory Infrastructure
- [ ] Create `pkg/agent/factory.go` with interfaces
- [ ] Implement `DefaultLauncherFactory`
- [ ] Implement `MockLauncherFactory` for tests
- [ ] Add LauncherConfig types and validation
- [ ] Write unit tests for factory implementations

### Phase 2: SessionManager Integration
- [ ] Add `launcherFactory` field to Manager struct
- [ ] Add `launchers` and `handles` maps with mutex
- [ ] Update `NewManager()` constructor signature
- [ ] Modify `SpawnAgent()` to use factory and launcher
- [ ] Modify `TerminateAgent()` to stop containers
- [ ] Add cleanup on session close
- [ ] Update existing unit tests with mock factory

### Phase 3: Relay Server Updates
- [ ] Initialize Docker client in cmd/relay/main.go
- [ ] Initialize worktree manager
- [ ] Initialize credential mounter
- [ ] Create DefaultLauncherFactory with dependencies
- [ ] Pass factory to SessionManager constructor
- [ ] Update integration tests

### Phase 4: Error Handling & Edge Cases
- [ ] Implement orphan container detection on startup
- [ ] Add cleanup for partial spawn failures
- [ ] Add per-agent spawn/stop locking
- [ ] Implement resource limit enforcement
- [ ] Add comprehensive error codes and messages
- [ ] Test all error scenarios

### Phase 5: E2E Test Suite
- [ ] Create tests/e2e/helpers/docker.go
- [ ] Implement container_spawn_test.go
- [ ] Implement container_lifecycle_test.go
- [ ] Implement container_credentials_test.go
- [ ] Implement container_worktree_test.go
- [ ] Implement container_concurrent_test.go
- [ ] Add CI integration for E2E tests
- [ ] Document E2E test usage

### Phase 6: Documentation & Cleanup
- [ ] Update docs/ARCHITECTURE.md with factory pattern
- [ ] Document LauncherFactory usage
- [ ] Add troubleshooting guide for containers
- [ ] Remove old non-containerized code paths
- [ ] Update README with container requirements

## Consensus Validation Results

**Models Consulted:** 3 (o3-mini:for, o3-mini:against, gpt-5-mini:neutral)
**Confidence Scores:** 9/10, 9/10, 8/10
**Verdict:** **APPROVED - Proceed with implementation**

### Key Consensus Points

**Strong Agreement:**
- LauncherFactory pattern is technically sound
- Dependency injection enhances testability
- Future extensibility justified despite slight complexity
- Aligns with industry best practices
- Clean separation of concerns

**Implementation Guardrails (from neutral model):**
- Define stable, versioned LauncherConfig API upfront
- Make lifecycle semantics explicit (Start/Stop/Wait/Status)
- Handle orphaned containers from crashes/restarts
- Add observability (logs/metrics) early
- Comprehensive E2E testing of failure scenarios

**Risk Mitigation:**
- Initial abstraction overhead minimal
- Learning curve for factory pattern manageable
- Benefits in maintainability outweigh complexity cost

## Open Questions

1. **Container Resource Limits:** What default CPU/memory limits for agent containers?
   - **Recommendation:** Start with 2 CPU cores, 4GB memory, make configurable

2. **Credential Storage:** Where to store temporary credential files?
   - **Recommendation:** `/tmp/ourocodus-creds/` with restrictive permissions

3. **Orphan Container Age:** How old before cleaning up orphaned containers?
   - **Recommendation:** > 1 hour old, configurable via flag

4. **Max Concurrent Agents:** Per-session or global limit?
   - **Recommendation:** Per-session: 10, Global: 100, both configurable

5. **E2E Test CI Integration:** Run on every PR or only on main?
   - **Recommendation:** On main and on-demand (manual trigger), skip for small PRs

## References

- Issue #105: ContainerLauncher Implementation
- Issue #106: Worktree Manager Implementation
- Issue #107: Update Relay to Use ContainerLauncher
- Issue #108: E2E Tests for Containerized Agents
- Issue #109: Configure Agent Credentials & Environment
- docs/TERMINOLOGY.md: Core entity definitions
- docs/ARCHITECTURE.md: System architecture overview
- pkg/containersession/: Container session management
- pkg/worktree/: Git worktree management
