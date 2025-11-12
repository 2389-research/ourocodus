# Packnplay Integration Design

**Date:** October 30, 2025
**Author:** Architecture Team
**Status:** Approved
**Issues:** #81-#87

## Overview

Ourocodus orchestrates multiple AI coding agents (Claude Code, Codex) to build software collaboratively. Agents currently run as host processes. This design integrates Packnplay to containerize agents with automatic worktree management and credential mounting.

## Goals

1. **Containerize agents** - Isolate agents in Docker containers
2. **Automate worktrees** - Replace manual git worktree scripts with Packnplay's automation
3. **Manage credentials** - Mount GitHub, AWS, and API keys securely
4. **Preserve architecture** - Keep orchestration layer (PWA, Relay, NATS, Coordinator) unchanged
5. **Enable scaling** - Abstract agent runtime to support future Kubernetes migration

## Why Packnplay

Packnplay provides proven infrastructure code:

- Docker lifecycle management
- Automatic git worktree creation per container
- Credential mounting (GitHub CLI, AWS, Git, macOS keychain)
- Dev container support (`.devcontainer/devcontainer.json`)
- Port mapping and user detection

Building this ourselves would duplicate 4,000+ lines of tested Go code. Using Packnplay as a library accelerates development.

## Architecture

### Layered Design

```
┌──────────────────────────────────────────────────┐
│         ORCHESTRATION LAYER                       │
│   PWA + Relay + NATS + Coordinator                │
│   (Your unique IP - multi-agent coordination)    │
└────────────────────┬─────────────────────────────┘
                     │
                     │ Clean Interface
                     ▼
┌──────────────────────────────────────────────────┐
│         AGENT RUNTIME ABSTRACTION                 │
│   pkg/agent/launcher.go                           │
│   Interface: Spawn(), Attach(), Stop()            │
└────────────────────┬─────────────────────────────┘
                     │
                     │ Implementation
                     ▼
┌──────────────────────────────────────────────────┐
│         PACKNPLAY INTEGRATION                     │
│   pkg/agent/packnplay/launcher.go                │
│   Uses: github.com/obra/packnplay/pkg/*          │
└──────────────────────────────────────────────────┘
```

### Key Interfaces

**AgentLauncher** - Runtime-agnostic agent lifecycle
```go
type AgentLauncher interface {
    Spawn(ctx context.Context, role, workspace string, config SpawnConfig) (AgentHandle, error)
    Attach(ctx context.Context, handle AgentHandle) error
    Stop(ctx context.Context, handle AgentHandle) error
}
```

**AgentHandle** - Reference to running agent
```go
type AgentHandle interface {
    ID() string
    Workspace() string
    ContainerID() string
    SendMessage(msg []byte) error
    ReceiveMessage() ([]byte, error)
}
```

**SpawnConfig** - Agent configuration
```go
type SpawnConfig struct {
    Command         []string          // ["claude-code-acp"]
    Credentials     CredentialConfig  // GitHub, AWS, etc.
    Environment     map[string]string // ANTHROPIC_API_KEY, etc.
    Ports           []string          // ["8080:8080"]
    DevContainer    bool              // Use .devcontainer if present
}
```

## Integration Approach

### Use Packnplay as Library, Not CLI

Import Packnplay packages directly:

```go
import (
    "github.com/obra/packnplay/pkg/runner"
    "github.com/obra/packnplay/pkg/docker"
    "github.com/obra/packnplay/pkg/git"
)

func (l *PacknplayLauncher) Spawn(...) (AgentHandle, error) {
    config := &runner.RunConfig{
        Path:         workspace,
        Worktree:     role,
        Command:      []string{"claude-code-acp"},
        Credentials:  l.credentials,
        PublishPorts: l.ports,
    }

    return runner.Run(config)
}
```

Benefits:
- Full error handling control
- No CLI subprocess overhead
- Can customize behavior
- Tighter integration

### Abstraction Layer Benefits

The `AgentLauncher` interface decouples orchestration from runtime:

1. **Testing** - Mock the interface in unit tests
2. **Flexibility** - Swap implementations without changing orchestration code
3. **Future-proof** - Migrate to Kubernetes by implementing interface for K8s Pods

Example future implementation:
```go
type KubernetesLauncher struct {
    clientset *kubernetes.Clientset
}

func (l *KubernetesLauncher) Spawn(...) (AgentHandle, error) {
    // Create K8s Pod with agent container
    // Mount worktree via PersistentVolumeClaim
    // Return handle wrapping Pod
}
```

Coordinator code stays identical.

## Responsibility Division

| Component | Owner | Responsibility |
|-----------|-------|----------------|
| **Packnplay** | Infrastructure | Container lifecycle, credentials, worktrees, isolation |
| **Ourocodus** | Orchestration | Multi-agent coordination, messaging (NATS), workflows, approval gates, PWA |

Ourocodus builds the orchestration brain. Packnplay provides the infrastructure layer.

## Implementation Plan

### Phase 1: Foundation (Issues #81-#82)

**#81: Agent Abstraction Layer**
- Create `pkg/agent/launcher.go` with interfaces
- Define `AgentLauncher`, `AgentHandle`, `SpawnConfig`
- Write unit tests with mock implementations
- Time: 4-6 hours

**#82: Add Packnplay Dependency**
- Run `go get github.com/obra/packnplay@latest`
- Update `go.mod` and verify build
- Add license attribution
- Time: 1-2 hours

**Risk:** Low - no breaking changes

### Phase 2: Implementation (Issues #83-#84)

**#83: PacknplayLauncher Implementation**
- Create `pkg/agent/packnplay/launcher.go`
- Implement `Spawn()` using `runner.Run()`
- Implement `Stop()` using `docker.Client`
- Implement `Attach()` for reconnecting
- Create `PacknplayHandle` implementing `AgentHandle`
- Integration tests with real containers
- Time: 8-12 hours

**#84: Configure Credentials**
- Configure GitHub CLI credential mounting
- Configure AWS credential support
- Set up API key passthrough (ANTHROPIC_API_KEY)
- Document credential requirements
- Time: 4-6 hours

**Risk:** Medium - new containerization complexity

### Phase 3: Integration (Issue #85)

**#85: Update Relay to Use AgentLauncher**
- Inject `PacknplayLauncher` at Relay startup
- Update `agent:spawn` handler to call `launcher.Spawn()`
- Update `agent:terminate` handler to call `launcher.Stop()`
- Remove old `exec.Command()` code
- Update Relay tests with mock launcher
- Verify all existing E2E tests pass
- Time: 6-8 hours

**Risk:** High - changes critical path (requires thorough testing)

### Phase 4: Validation (Issues #86-#87)

**#86: E2E Tests for Containerized Agents**
- Test: Spawn containerized echo-agent
- Test: Spawn containerized Claude Code agent
- Test: Multiple concurrent agents
- Test: Container cleanup on termination
- Test: Worktree creation and isolation
- Test: Credential mounting
- Time: 6-8 hours

**#87: Documentation**
- Create `docs/AGENT_RUNTIME.md`
- Update `docs/ARCHITECTURE.md`
- Add architecture diagrams
- Document credential setup
- Document troubleshooting
- Document future K8s migration path
- Time: 4-6 hours

**Risk:** Low - validation only

**Total Time:** 33-48 hours

## Trade-offs & Decisions

### Decision: Library Integration vs CLI Wrapper

**Options:**
1. Import Packnplay as Go library
2. Shell out to `packnplay run` CLI

**Choice:** Library integration

**Rationale:**
- Full control over errors and lifecycle
- No subprocess overhead
- Can customize behavior
- Tighter type safety

### Decision: Abstraction Layer vs Direct Packnplay Usage

**Options:**
1. Create `AgentLauncher` interface with Packnplay implementation
2. Use Packnplay directly throughout codebase

**Choice:** Abstraction layer

**Rationale:**
- Enables testing with mocks
- Decouples orchestration from container runtime
- Allows future migration to Kubernetes
- Clean architectural boundaries

### Decision: What to Throw Away

**Throwing away:**
- `scripts/setup-worktrees.sh` (completed work, ~3 hours)
- Current `exec.Command()` agent spawning

**Rationale:**
- Packnplay's worktree management is superior
- Gain containerization + credentials + dev container support
- Net positive trade

## What Changes, What Stays

### Stays Unchanged

✅ **PWA** - All frontend work preserved
✅ **Relay** - WebSocket protocol unchanged (adds container support)
✅ **NATS** - Message bus plans unchanged
✅ **Coordinator** - Orchestration logic unchanged
✅ **Session Management** - Lifecycle unchanged from user perspective

### Changes

🔄 **Agent Spawning** - Relay uses `AgentLauncher` instead of `exec.Command()`
🔄 **Worktrees** - Created automatically by Packnplay per container
➕ **Credentials** - New: automatic mounting via Packnplay
➕ **Containers** - New: Docker isolation for agents
➕ **Dev Containers** - New: Support for `.devcontainer` configs

## Milestone Impact

**Milestone 2 (PWA + NATS Foundation):**
- Original: 20 issues
- Added: 7 Packnplay issues
- New total: 27 issues
- Added time: 33-48 hours

**Milestone 3 (NATS Integration):**
- No changes

**Milestone 4 (Autonomous Coordination):**
- Coordinator uses `AgentLauncher` to spawn agents
- No other changes

## Future Considerations

### Scaling to Kubernetes

The abstraction layer enables Kubernetes migration:

1. Implement `KubernetesLauncher` for `AgentLauncher` interface
2. Replace Packnplay injection with Kubernetes implementation
3. Orchestration layer remains identical

No rewrite required - just swap implementations.

### Multi-Runtime Support

Could support multiple runtimes simultaneously:

```go
type MultiRuntimeLauncher struct {
    local      *PacknplayLauncher
    kubernetes *KubernetesLauncher
}

func (m *MultiRuntimeLauncher) Spawn(...) (AgentHandle, error) {
    if m.shouldUseKubernetes(config) {
        return m.kubernetes.Spawn(...)
    }
    return m.local.Spawn(...)
}
```

### Container Registry

Future: Package agents in custom images

```dockerfile
FROM ubuntu:22.04
RUN npm install -g @zed-industries/claude-code-acp
COPY .devcontainer/Dockerfile /
```

Packnplay supports custom images via `DefaultImage` config.

## Success Criteria

Integration succeeds when:

1. ✅ Agents run in isolated Docker containers
2. ✅ Worktrees create automatically (one per agent/role)
3. ✅ Credentials mount correctly (GitHub, AWS, API keys)
4. ✅ WebSocket communication works through containers
5. ✅ All existing E2E tests pass
6. ✅ Multiple concurrent containerized agents work
7. ✅ Containers clean up on agent termination
8. ✅ Dev containers load when `.devcontainer` exists

## References

- **Packnplay Repository:** https://github.com/obra/packnplay
- **Issues:** #81 (Abstraction), #82 (Dependency), #83 (Implementation), #84 (Credentials), #85 (Relay), #86 (E2E Tests), #87 (Docs)
- **Milestone:** Milestone 2: PWA + NATS Foundation
- **Track:** `track:packnplay-integration`

## Appendix: Credential Flow

```
┌──────────────┐
│ Host Machine │
├──────────────┤
│ ~/.config/gh │ ──┐
│ ~/.aws/      │   │
│ ~/.gitconfig │   │ Read-only
│ Keychain     │   │ mounts
└──────────────┘   │
                   │
                   ▼
            ┌─────────────┐
            │  Container  │
            ├─────────────┤
            │ Packnplay   │
            │ mounts      │
            │ credentials │
            └─────────────┘
                   │
                   ▼
            ┌─────────────┐
            │ Agent       │
            │ (claude-    │
            │  code-acp)  │
            └─────────────┘
```

Credentials mount read-only. Agents access GitHub, AWS, Git without host pollution.

## Appendix: Dependency Graph

```
       #81 (Abstraction)     #82 (Dependency)
             │                      │
             └──────────┬───────────┘
                        │
                   #83 (Packnplay)
                        │
            ┌───────────┴───────────┐
            │                       │
       #84 (Creds)          #85 (Relay)
                                    │
                              #86 (E2E Tests)
                                    │
                              #87 (Docs)
```

Critical path: #81 → #82 → #83 → #85 → #86 → #87
