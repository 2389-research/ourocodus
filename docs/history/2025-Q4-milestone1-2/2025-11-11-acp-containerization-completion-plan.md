# ACP Containerization Completion Plan

**Date:** 2025-11-11
**Status:** Execution Plan
**Parent Plan:** [2025-11-10-acp-containerization-plan.md](./2025-11-10-acp-containerization-plan.md)
**Estimated Effort:** 2-3 days

## Overview

This plan completes the remaining 35% of the ACP containerization work and fixes issues discovered during code review. The foundation is solid - we need to wire the container exec path into the runtime selection logic and add missing tests.

## Current State

**Completed:**
- ✅ Phase 1: Transport abstraction (90%)
- ✅ Phase 2: Runtime context plumbing (70%)
- ✅ Phase 3: Container exec implementation (40%)
- ❌ Phase 4: Not started

**Critical Gap:** Container exec code exists but is never invoked - no launcher selection logic.

---

## Workstream 1: Complete Phase 2 & 3 Integration (CRITICAL PATH)

### Task 1.1: Implement Launcher Selection in ACPClientFactory

**File:** `pkg/relay/session/client_factory.go`

**Current Problem:** Lines 44-67 always use `acp.NewClient()` which defaults to `HostProcessLauncher`.

**Solution:**

```go
// pkg/relay/session/client_factory.go

import (
    "os"
    "github.com/2389-research/ourocodus/pkg/containersession"
)

// ACPClientFactory implements ClientFactory with runtime-based launcher selection
type ACPClientFactory struct {
    apiKey              string
    acpBinaryPath       string
    containerSessionMgr containersession.Manager // NEW: inject container session manager
}

// NewACPClientFactory creates a new ACP client factory
// containerSessionMgr is optional - if nil, container mode is disabled
func NewACPClientFactory(containerSessionMgr containersession.Manager) (*ACPClientFactory, error) {
    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        return nil, ErrMissingAnthropicAPIKey
    }

    acpBinaryPath := os.Getenv("OUROCODUS_ACP_BINARY")

    return &ACPClientFactory{
        apiKey:              apiKey,
        acpBinaryPath:       acpBinaryPath,
        containerSessionMgr: containerSessionMgr,
    }, nil
}

// NewClient spawns a new ACP process using the appropriate launcher
func (f *ACPClientFactory) NewClient(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error) {
    if runtime == nil {
        return nil, fmt.Errorf("runtime context is required")
    }
    workspace := runtime.Workspace
    if workspace == "" {
        return nil, fmt.Errorf("workspace is required")
    }

    // Select launcher based on runtime context and feature flag
    launcher, err := f.selectLauncher(runtime)
    if err != nil {
        return nil, fmt.Errorf("failed to select launcher: %w", err)
    }

    // Build client options
    opts := []acp.ClientOption{
        acp.WithProcessLauncher(launcher),
    }

    if f.acpBinaryPath != "" {
        opts = append(opts, acp.WithCommand(f.acpBinaryPath))
    }

    client, err := acp.NewClient(workspace, f.apiKey, opts...)
    if err != nil {
        return nil, fmt.Errorf("failed to create ACP client: %w", err)
    }

    return &acpClientAdapter{client: client}, nil
}

// selectLauncher chooses between host and container execution
func (f *ACPClientFactory) selectLauncher(runtime *AgentRuntimeContext) (acp.ProcessLauncher, error) {
    acpRuntime := os.Getenv("OUROCODUS_ACP_RUNTIME")

    // Default to host if not specified or explicitly set to "host"
    if acpRuntime == "" || acpRuntime == "host" {
        return &acp.HostProcessLauncher{}, nil
    }

    // Container mode requested
    if acpRuntime == "container" {
        // Validate prerequisites
        if !runtime.HasContainer() {
            return nil, fmt.Errorf("container runtime requested but no container ID in runtime context")
        }
        if f.containerSessionMgr == nil {
            return nil, fmt.Errorf("container runtime requested but container session manager not available")
        }

        // Create container exec launcher
        launcher := NewContainerExecProcessLauncher(
            f.containerSessionMgr,
            runtime.ContainerID,
        )

        // Configure workspace path mapping if needed
        // Container workspace path should match the mount point
        launcher = launcher.WithWorkspacePath("/workspace")

        return launcher, nil
    }

    return nil, fmt.Errorf("invalid OUROCODUS_ACP_RUNTIME value: %q (must be 'host' or 'container')", acpRuntime)
}
```

**Testing:**

```go
// pkg/relay/session/client_factory_test.go (NEW)

func TestACPClientFactory_SelectLauncher_Host(t *testing.T) {
    t.Setenv("OUROCODUS_ACP_RUNTIME", "host")
    t.Setenv("ANTHROPIC_API_KEY", "test-key")

    factory, _ := NewACPClientFactory(nil)
    runtime := &AgentRuntimeContext{Workspace: "/tmp/test"}

    launcher, err := factory.selectLauncher(runtime)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }

    if _, ok := launcher.(*acp.HostProcessLauncher); !ok {
        t.Fatalf("expected HostProcessLauncher, got %T", launcher)
    }
}

func TestACPClientFactory_SelectLauncher_Container(t *testing.T) {
    t.Setenv("OUROCODUS_ACP_RUNTIME", "container")
    t.Setenv("ANTHROPIC_API_KEY", "test-key")

    mgr := &mockContainerSessionManager{}
    factory, _ := NewACPClientFactory(mgr)
    runtime := &AgentRuntimeContext{
        Workspace:   "/tmp/test",
        ContainerID: "container-123",
    }

    launcher, err := factory.selectLauncher(runtime)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }

    if _, ok := launcher.(*ContainerExecProcessLauncher); !ok {
        t.Fatalf("expected ContainerExecProcessLauncher, got %T", launcher)
    }
}

func TestACPClientFactory_SelectLauncher_ContainerMissingPrerequisites(t *testing.T) {
    t.Setenv("OUROCODUS_ACP_RUNTIME", "container")
    t.Setenv("ANTHROPIC_API_KEY", "test-key")

    factory, _ := NewACPClientFactory(nil) // No manager
    runtime := &AgentRuntimeContext{Workspace: "/tmp/test"}

    _, err := factory.selectLauncher(runtime)
    if err == nil {
        t.Fatal("expected error for missing container manager")
    }
}
```

**Dependencies Updated:**
- `cmd/relay/main.go` needs to pass containersession.Manager to NewACPClientFactory
- Update all factory constructors in tests

---

### Task 1.2: Wire Container ID from AgentHandle

**File:** `pkg/relay/session/manager.go`

**Current Issue:** Manager.SpawnAgent sets ContainerID in runtime context before retrieving from handle.

**Solution:**

```go
// pkg/relay/session/manager.go (around line 299-307)

func (m *Manager) SpawnAgent(ctx context.Context, userSessionID, agentID, workspace string) error {
    // ... existing validation ...

    // Create agent session in SPAWNING state
    agent := NewAgentSession(agentID, workspace, m.clock.Now())

    // Spawn container if launcher factory available
    var handle agent.AgentHandle
    var containerID string

    if m.launcherFactory != nil {
        launcher, err := m.launcherFactory.CreateLauncher(agentID)
        if err != nil {
            return fmt.Errorf("failed to create launcher: %w", err)
        }

        handle, err = launcher.Spawn(ctx, agentID, workspace)
        if err != nil {
            return fmt.Errorf("failed to spawn agent container: %w", err)
        }

        // Store handle and launcher
        key := launcherKey(userSessionID, agentID)
        m.launchersMu.Lock()
        m.launchers[key] = launcher
        m.handles[key] = handle
        m.launchersMu.Unlock()

        // Extract container ID from handle
        // AgentContainerHandle should expose ContainerID() method
        if containerHandle, ok := handle.(interface{ ContainerID() string }); ok {
            containerID = containerHandle.ContainerID()
        }
    }

    // Build runtime context with container ID if available
    runtime := &AgentRuntimeContext{
        SessionID:   userSessionID,
        AgentID:     agentID,
        Workspace:   workspace,
        ContainerID: containerID, // Now properly set from handle
    }

    // Spawn ACP client using selected launcher
    acpClient, err := m.clientFactory.NewClient(ctx, runtime)
    if err != nil {
        // Cleanup container if spawn failed
        if handle != nil {
            _ = handle.Terminate(ctx)
        }
        return fmt.Errorf("failed to spawn ACP client: %w", err)
    }

    // ... rest of method ...
}
```

**Prerequisite:** Ensure `pkg/agent/packnplay/handle.go` exposes `ContainerID()` method:

```go
// pkg/agent/packnplay/handle.go

// ContainerID returns the underlying container ID
func (h *AgentContainerHandle) ContainerID() string {
    return h.containerID
}
```

---

### Task 1.3: Make Workspace Path Configurable

**File:** `pkg/relay/session/container_exec_process_launcher.go`

**Current Issue:** Hard-coded `/workspace` on line 29.

**Solution:**

```go
// pkg/relay/session/container_exec_process_launcher.go

// NewContainerExecProcessLauncher constructs a container-based ProcessLauncher.
// Workspace path defaults to deriving from host workspace mapping.
func NewContainerExecProcessLauncher(service ContainerExecService, containerID string) *ContainerExecProcessLauncher {
    return &ContainerExecProcessLauncher{
        execService:   service,
        containerID:   containerID,
        workspacePath: "", // Will be derived from config
    }
}

// WithWorkspacePath overrides the default container workspace path.
func (l *ContainerExecProcessLauncher) WithWorkspacePath(path string) *ContainerExecProcessLauncher {
    l.workspacePath = path
    return l
}

// Start implements acp.ProcessLauncher.
func (l *ContainerExecProcessLauncher) Start(ctx context.Context, cfg acp.ProcessLaunchConfig) (acp.Transport, error) {
    // ... validation ...

    // Determine workspace path
    workspacePath := l.workspacePath
    if workspacePath == "" {
        // Default: assume standard container mount at /workspace
        // In production, this should be configurable via AgentRuntimeContext
        workspacePath = "/workspace"
    }

    execCfg := containersession.ExecConfig{
        Command:    command,
        Env:        env,
        WorkingDir: workspacePath,
    }

    // ... rest of method ...
}
```

**Better Long-term:** Add `ContainerWorkspacePath` to `AgentRuntimeContext` and derive from it.

---

## Workstream 2: Add Missing Tests

### Task 2.1: Unit Tests for Transport Abstraction

**File:** `pkg/acp/client_test.go` (extend existing)

```go
// pkg/acp/client_test.go

func TestNewClientFromTransport(t *testing.T) {
    transport := &fakeTransport{
        readBuf:  bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"result":{"content":"response"}}`+"\n"),
        writeBuf: &bytes.Buffer{},
    }

    client, err := NewClientFromTransport(transport, WithLogger(&testLogger{}))
    if err != nil {
        t.Fatalf("NewClientFromTransport failed: %v", err)
    }
    defer client.Close()

    resp, err := client.SendMessage("test")
    if err != nil {
        t.Fatalf("SendMessage failed: %v", err)
    }

    if resp.Content != "response" {
        t.Errorf("unexpected response: %v", resp)
    }
}

func TestClient_ConcurrentRequests_IDOrdering(t *testing.T) {
    // Test that concurrent SendMessage calls get sequential IDs
    // and responses match correctly

    transport := &syncedFakeTransport{
        responses: map[int]string{
            1: `{"jsonrpc":"2.0","id":1,"result":{"content":"resp1"}}`,
            2: `{"jsonrpc":"2.0","id":2,"result":{"content":"resp2"}}`,
            3: `{"jsonrpc":"2.0","id":3,"result":{"content":"resp3"}}`,
        },
    }

    client, _ := NewClientFromTransport(transport)
    defer client.Close()

    var wg sync.WaitGroup
    results := make([]string, 3)

    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            resp, err := client.SendMessage(fmt.Sprintf("msg%d", idx))
            if err != nil {
                t.Errorf("SendMessage failed: %v", err)
                return
            }
            results[idx] = resp.Content
        }(i)
    }

    wg.Wait()

    // Verify all responses received correctly despite concurrent calls
    expected := []string{"resp1", "resp2", "resp3"}
    if !reflect.DeepEqual(results, expected) {
        t.Errorf("ID ordering broken: got %v, want %v", results, expected)
    }
}

// fakeTransport implements Transport for testing
type fakeTransport struct {
    readBuf  *bytes.Buffer
    writeBuf *bytes.Buffer
    stderr   *bytes.Buffer
}

func (f *fakeTransport) Read(p []byte) (int, error)  { return f.readBuf.Read(p) }
func (f *fakeTransport) Write(p []byte) (int, error) { return f.writeBuf.Write(p) }
func (f *fakeTransport) Close() error                { return nil }
func (f *fakeTransport) Stderr() io.Reader           { return f.stderr }
```

---

### Task 2.2: Integration Test for Container Execution

**File:** `pkg/relay/session/launcher_integration_test.go` (NEW)

```go
//go:build integration

package session

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/2389-research/ourocodus/pkg/agent/packnplay"
    "github.com/2389-research/ourocodus/pkg/containersession"
    "github.com/docker/docker/client"
)

func TestACP_RunsInsideContainer(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Setup: Create real Docker client and container session manager
    dockerClient, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        t.Fatalf("failed to create docker client: %v", err)
    }

    csMgr := containersession.NewManager(/* ... */)

    // Create factory with container exec support
    t.Setenv("OUROCODUS_ACP_RUNTIME", "container")
    t.Setenv("ANTHROPIC_API_KEY", "test-key-integration")
    t.Setenv("OUROCODUS_ACP_BINARY", "/path/to/echo-agent") // Use echo agent for testing

    factory, err := NewACPClientFactory(csMgr)
    if err != nil {
        t.Fatalf("failed to create factory: %v", err)
    }

    // Spawn container using packnplay launcher
    launcherFactory := packnplay.NewLauncherFactory(/* ... */)
    launcher, _ := launcherFactory.CreateLauncher("test-agent")

    ctx := context.Background()
    handle, err := launcher.Spawn(ctx, "test-agent", "/tmp/test-workspace")
    if err != nil {
        t.Fatalf("failed to spawn container: %v", err)
    }
    defer handle.Terminate(ctx)

    // Get container ID
    containerHandle := handle.(*packnplay.AgentContainerHandle)
    containerID := containerHandle.ContainerID()

    // Create runtime context
    runtime := &AgentRuntimeContext{
        SessionID:   "test-session",
        AgentID:     "test-agent",
        Workspace:   "/tmp/test-workspace",
        ContainerID: containerID,
    }

    // Create ACP client (should use container exec)
    acpClient, err := factory.NewClient(ctx, runtime)
    if err != nil {
        t.Fatalf("failed to create ACP client: %v", err)
    }
    defer acpClient.Close()

    // Verify: Send message and get response
    resp, err := acpClient.SendMessage("test message")
    if err != nil {
        t.Fatalf("SendMessage failed: %v", err)
    }

    if resp == nil {
        t.Fatal("expected response, got nil")
    }

    // Verify: Check that ACP process is running inside container
    execResult, err := dockerClient.ContainerExecCreate(ctx, containerID, container.ExecOptions{
        Cmd: []string{"ps", "aux"},
    })
    if err != nil {
        t.Fatalf("failed to exec ps: %v", err)
    }

    // Attach and read output
    attachResp, _ := dockerClient.ContainerExecAttach(ctx, execResult.ID, container.ExecAttachOptions{})
    defer attachResp.Close()

    output, _ := io.ReadAll(attachResp.Reader)

    // Verify ACP binary is running (look for echo-agent or claude-code-acp in process list)
    if !bytes.Contains(output, []byte("echo-agent")) && !bytes.Contains(output, []byte("claude-code-acp")) {
        t.Errorf("ACP process not found inside container. Process list:\n%s", output)
    }

    t.Logf("✓ ACP process verified running inside container %s", containerID[:12])
}
```

**Run with:** `make docker-test` or `go test -tags=integration ./pkg/relay/session/`

---

## Workstream 3: Fix Code Quality Issues

### Task 3.1: Unify Logger Abstractions

**Problem:** Two separate logger interfaces exist:
- `pkg/acp/client.go:23` - `type Logger interface { Printf(...) }`
- `pkg/containersession/interfaces.go:44-97` - `LeveledLogger` with Error/Info/Debug

**Solution Option A** (Recommended): Use LeveledLogger everywhere

```go
// pkg/acp/logger.go (NEW)

package acp

import "github.com/2389-research/ourocodus/pkg/containersession"

// Logger is an alias to containersession.Logger for compatibility
type Logger = containersession.Logger

// noOpLogger remains for default case
type noOpLogger struct{}
func (noOpLogger) Printf(format string, v ...interface{}) {}
```

**Solution Option B:** Create shared logger package

```go
// pkg/logger/logger.go (NEW)

package logger

type Logger interface {
    Printf(format string, v ...interface{})
}

type LeveledLogger struct {
    logger Logger
    level  LogLevel
}

// ... implementation ...
```

Then import from both packages.

**Recommendation:** Option A - reuse existing containersession logger.

---

### Task 3.2: Fix Error Wrapping

**File:** `pkg/relay/session/container_exec_process_launcher.go:66`

**Current:**
```go
attachment, err := l.execService.ExecInContainer(ctx, l.containerID, execCfg)
if err != nil {
    return nil, err  // No context
}
```

**Fixed:**
```go
attachment, err := l.execService.ExecInContainer(ctx, l.containerID, execCfg)
if err != nil {
    return nil, fmt.Errorf("failed to exec ACP in container %s: %w", l.containerID, err)
}
```

**Apply similar fixes throughout the file for all error returns.**

---

### Task 3.3: Add Runtime Logging

**File:** `pkg/relay/session/client_factory.go`

Add logging when launcher is selected:

```go
func (f *ACPClientFactory) selectLauncher(runtime *AgentRuntimeContext) (acp.ProcessLauncher, error) {
    acpRuntime := os.Getenv("OUROCODUS_ACP_RUNTIME")

    if acpRuntime == "" || acpRuntime == "host" {
        log.Printf("[ACP] Using host process launcher for session=%s agent=%s",
            runtime.SessionID, runtime.AgentID)
        return &acp.HostProcessLauncher{}, nil
    }

    if acpRuntime == "container" {
        // ... validation ...

        log.Printf("[ACP] Using container exec launcher for session=%s agent=%s container=%s",
            runtime.SessionID, runtime.AgentID, runtime.ContainerID)

        launcher := NewContainerExecProcessLauncher(
            f.containerSessionMgr,
            runtime.ContainerID,
        )
        launcher = launcher.WithWorkspacePath("/workspace")

        return launcher, nil
    }

    return nil, fmt.Errorf("invalid OUROCODUS_ACP_RUNTIME value: %q", acpRuntime)
}
```

---

## Workstream 4: Documentation & Polish

### Task 4.1: Update ACP Documentation

**File:** `docs/ACP.md`

Add section:

```markdown
## Container Execution Mode

As of Milestone 3, ACP can run either on the host or inside agent containers.

### Configuration

Set the `OUROCODUS_ACP_RUNTIME` environment variable:

- `host` (default) - ACP runs as host process (original behavior)
- `container` - ACP runs inside spawned agent container via Docker exec

### Architecture

```
Host Mode:
  Session Manager → ACP Client → HostProcessLauncher → os/exec → ACP binary

Container Mode:
  Session Manager → ACP Client → ContainerExecProcessLauncher
    → containersession.ExecInContainer → docker exec → ACP binary (in container)
```

### Requirements for Container Mode

1. Agent container must be spawned via `agent.Launcher`
2. Container must have ACP binary available at expected path
3. Container must have network access for Anthropic API
4. `containersession.Manager` must be injected into `ACPClientFactory`

### Troubleshooting

**Issue:** "container runtime requested but no container ID"
- Ensure `agent.Launcher` spawned successfully before creating ACP client
- Check that `AgentHandle.ContainerID()` returns valid container ID

**Issue:** ACP fails to start in container
- Verify ACP binary exists in container: `docker exec <container> which claude-code-acp`
- Check container logs: `docker logs <container>`
- Verify API key propagated: `docker exec <container> env | grep ANTHROPIC_API_KEY`
```

---

### Task 4.2: Document Feature Flag

**File:** `.envrc.example`

Add:

```bash
# ACP Execution Mode
# Controls where claude-code-acp process runs
# Values: "host" (default) | "container"
# Set to "container" to run ACP inside agent containers (requires Milestone 3+)
export OUROCODUS_ACP_RUNTIME=host
```

**File:** `README.md` (Environment Variables section)

Add:

```markdown
| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OUROCODUS_ACP_RUNTIME` | No | `host` | ACP execution mode: `host` or `container` |
```

---

### Task 4.3: Update Plan Status

**File:** `docs/plans/2025-11-10-acp-containerization-plan.md`

Update checkboxes:

```markdown
## Next Steps Checklist

- [x] Phase 1 PR: transport abstraction + acp.Client refactor
- [x] Phase 2 PR: session runtime context + factory wiring
- [x] Phase 3 PRs:
  - [x] Docker exec helper in containersession
  - [x] ContainerExecProcessLauncher implementation
  - [x] Feature flag and launcher selection
  - [x] Integration test
- [x] Update docs + testing instructions (Phase 4)
```

---

## Execution Order & Dependencies

### Sprint 1: Critical Path (Day 1-2)

**Priority 1: Get it working**

1. Task 1.1 - Implement launcher selection (2-3 hours)
2. Task 1.2 - Wire container ID (1 hour)
3. Task 1.3 - Fix workspace path (30 min)
4. Task 3.2 - Fix error wrapping (30 min)
5. Manual smoke test with `OUROCODUS_ACP_RUNTIME=container`

### Sprint 2: Testing & Quality (Day 2-3)

**Priority 2: Validate and stabilize**

6. Task 2.1 - Unit tests for transport (2 hours)
7. Task 2.2 - Integration test (3-4 hours)
8. Task 3.1 - Unify loggers (1-2 hours)
9. Task 3.3 - Add runtime logging (1 hour)
10. Task 4.1 - Documentation (2 hours)
11. Task 4.2 - Environment variable docs (30 min)

### Sprint 3: Review & Ship (Day 3)

**Priority 3: Polish and merge**

12. Run full test suite (`make test`, `make lint`)
13. Run integration test with docker (`make docker-test`)
14. Code review with other LLM
15. Update task tracking and close issues

---

## Success Criteria

### Must Have (Blocking)

- [ ] `OUROCODUS_ACP_RUNTIME=container` launches ACP inside container
- [ ] Feature gracefully falls back to host mode if container unavailable
- [ ] Integration test passes showing ACP process inside container
- [ ] All existing tests still pass (no regressions)
- [ ] Code passes linting (`make lint`)

### Should Have (Strongly Recommended)

- [ ] Unit tests for transport abstraction
- [ ] Unit tests for launcher selection logic
- [ ] Error messages include context
- [ ] Logging clearly shows which mode is active
- [ ] Documentation updated

### Nice to Have (Future Work)

- [ ] Unified logger abstraction across packages
- [ ] Configurable container workspace path from runtime context
- [ ] Metrics/observability for container vs host execution
- [ ] Performance comparison test (host vs container latency)

---

## Risk Mitigation

### Risk 1: Docker SDK Compatibility Issues
**Likelihood:** Low
**Impact:** Medium
**Mitigation:** containersession.ExecInContainer already tested; if issues arise, pin Docker SDK version in go.mod

### Risk 2: Integration Test Flakiness
**Likelihood:** Medium
**Impact:** Low
**Mitigation:** Use proper cleanup in defer blocks; add retries for container startup; increase timeouts

### Risk 3: Breaking Existing Host Mode
**Likelihood:** Low
**Impact:** High
**Mitigation:** Default to host mode; ensure all existing tests still pass; feature flag allows rollback

### Risk 4: Container Missing ACP Binary
**Likelihood:** High (in dev)
**Impact:** High
**Mitigation:** Document requirement clearly; add integration test that verifies binary exists; provide clear error message

---

## Testing Strategy

### Unit Tests (Fast)
```bash
go test ./pkg/acp/
go test ./pkg/relay/session/
go test ./pkg/containersession/
```

### Integration Tests (Slow, requires Docker)
```bash
make docker-test
# or
go test -tags=integration ./pkg/relay/session/
```

### Manual Smoke Test
```bash
# Start relay with container mode
export OUROCODUS_ACP_RUNTIME=container
export ANTHROPIC_API_KEY=sk-ant-...
./bin/relay

# In another terminal, spawn agent
curl -X POST http://localhost:8080/session

# Check logs for "[ACP] Using container exec launcher"
# Check container: docker ps
# Verify process: docker exec <container> ps aux | grep claude-code-acp
```

---

## Rollout Plan

### Phase 1: Development (This Plan)
- Feature complete with tests
- Default: host mode
- Container mode: opt-in via env var

### Phase 2: Staging (Future)
- Deploy to staging with `OUROCODUS_ACP_RUNTIME=container`
- Monitor for 48 hours
- Compare latency metrics (host vs container)
- Fix any issues discovered

### Phase 3: Production (Future)
- Gradual rollout: 10% → 50% → 100%
- Keep host mode available as fallback
- After 1 week: make container default
- After 2 weeks: deprecate host mode

### Phase 4: Cleanup (Future)
- Remove host mode fallback
- Remove feature flag
- Update documentation to reflect container-only execution

---

## Notes for Implementation

- **Dependency Injection:** Pass `containersession.Manager` through constructor chain from `cmd/relay/main.go`
- **Backward Compatibility:** Ensure factory works without container manager (host mode only)
- **Error Messages:** Include session ID, agent ID, container ID in all errors for debuggability
- **Logging:** Use structured logging if available; include runtime mode in log prefix
- **Testing:** Prioritize integration test - it validates the entire chain end-to-end

---

## Questions & Decisions

**Q: Should we support mixed mode (some agents host, some container)?**
A: Yes, via per-agent configuration in future milestone. For now, global flag is sufficient.

**Q: What if container dies while ACP is running?**
A: ACP transport will receive EOF and close gracefully. Session manager already handles cleanup.

**Q: Performance impact of container exec vs host?**
A: Expect 5-10ms overhead for Docker exec startup. Once running, stdio performance is equivalent.

**Q: Do we need metrics?**
A: Not blocking for this milestone. Add in future observability work.

---

## Completion Checklist

Before marking this plan complete:

- [ ] All tasks 1.1-1.3 implemented (critical path)
- [ ] All tasks 2.1-2.2 implemented (testing)
- [ ] All tasks 3.1-3.3 implemented (quality)
- [ ] All tasks 4.1-4.2 implemented (docs)
- [ ] Integration test passes
- [ ] Manual smoke test successful
- [ ] Code review completed
- [ ] Original plan updated with status

**Estimated Completion Date:** 2025-11-13
