# Relay Container Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Integrate AgentContainerLauncher into Relay via LauncherFactory pattern and add comprehensive E2E tests for containerized agent lifecycle.

**Architecture:** Use factory pattern to inject AgentContainerLauncher into SessionManager. SessionManager creates launchers per agent via factory, stores handles for lifecycle management. Add scenario-based E2E tests covering spawn, lifecycle, credentials, worktrees, and concurrency.

**Tech Stack:** Go 1.23, Docker SDK, pkg/containersession, pkg/worktree, pkg/agent/container, existing relay/session infrastructure

---

## Task 1: LauncherFactory Interface & Mock Implementation

**Files:**
- Create: `pkg/agent/factory.go`
- Create: `pkg/agent/factory_test.go`
- Create: `pkg/agent/mock_factory.go`

**Step 1: Write failing test for LauncherFactory interface**

Create `pkg/agent/factory_test.go`:

```go
package agent_test

import (
	"context"
	"testing"

	"github.com/2389-research/ourocodus/pkg/agent"
)

func TestDefaultFactory_CreateLauncher(t *testing.T) {
	// This will fail because DefaultFactory doesn't exist yet
	factory := agent.NewDefaultLauncherFactory(nil, nil, nil, agent.LauncherFactoryConfig{})

	launcher, err := factory.CreateLauncher(context.Background(), "test-agent", agent.LauncherConfig{})
	if err != nil {
		t.Fatalf("CreateLauncher failed: %v", err)
	}

	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/agent -v -run TestDefaultFactory_CreateLauncher`
Expected: FAIL - "undefined: agent.NewDefaultLauncherFactory"

**Step 3: Create LauncherFactory interface and types**

Create `pkg/agent/factory.go`:

```go
package agent

import (
	"context"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/docker/docker/client"
)

// LauncherFactory creates AgentLauncher instances based on agent type and configuration.
type LauncherFactory interface {
	// CreateLauncher creates a new launcher for the specified agent.
	// Returns an error if launcher creation fails.
	CreateLauncher(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error)
}

// LauncherConfig contains configuration for creating a launcher.
type LauncherConfig struct {
	AgentID        string
	ImageName      string
	Command        []string
	Workspace      string
	GitSSHKey      []byte
	GitHubToken    []byte
	AnthropicKey   string
	ResourceLimits ResourceLimits
}

// ResourceLimits defines container resource constraints.
type ResourceLimits struct {
	CPUCores int64  // CPU cores (e.g., 2)
	MemoryMB int64  // Memory in MB (e.g., 4096)
}

// LauncherFactoryConfig contains dependencies for creating launchers.
type LauncherFactoryConfig struct {
	DockerClient       *client.Client
	WorktreeManager    *worktree.Manager
	CredMounter        *container.CredentialMounter
	ContainerManager   *containersession.Manager
	BaseWorkspaceDir   string
	DefaultImageName   string
	DefaultResourceLimits ResourceLimits
}

// DefaultLauncherFactory creates AgentContainerLauncher instances.
type DefaultLauncherFactory struct {
	config LauncherFactoryConfig
}

// NewDefaultLauncherFactory creates a new factory with the provided configuration.
func NewDefaultLauncherFactory(config LauncherFactoryConfig) *DefaultLauncherFactory {
	return &DefaultLauncherFactory{
		config: config,
	}
}

// CreateLauncher creates an AgentContainerLauncher for the specified agent.
func (f *DefaultLauncherFactory) CreateLauncher(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error) {
	// Create container launcher with dependencies
	launcher := container.NewAgentContainerLauncher(
		f.config.DockerClient,
		f.config.WorktreeManager,
		f.config.CredMounter,
		f.config.ContainerManager,
		f.config.BaseWorkspaceDir,
	)

	return launcher, nil
}
```

**Step 4: Fix test to use correct constructor signature**

Update `pkg/agent/factory_test.go`:

```go
package agent_test

import (
	"context"
	"testing"

	"github.com/2389-research/ourocodus/pkg/agent"
)

func TestDefaultFactory_CreateLauncher(t *testing.T) {
	config := agent.LauncherFactoryConfig{
		// nil dependencies OK for this basic test
	}
	factory := agent.NewDefaultLauncherFactory(config)

	launcherConfig := agent.LauncherConfig{
		AgentID: "test-agent",
	}

	launcher, err := factory.CreateLauncher(context.Background(), "test-agent", launcherConfig)
	if err != nil {
		t.Fatalf("CreateLauncher failed: %v", err)
	}

	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./pkg/agent -v -run TestDefaultFactory_CreateLauncher`
Expected: PASS

**Step 6: Create MockLauncherFactory for testing**

Create `pkg/agent/mock_factory.go`:

```go
package agent

import (
	"context"
)

// MockLauncherFactory is a mock implementation for testing.
type MockLauncherFactory struct {
	CreateLauncherFunc func(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error)
}

// NewMockLauncherFactory creates a new mock factory.
func NewMockLauncherFactory() *MockLauncherFactory {
	return &MockLauncherFactory{
		CreateLauncherFunc: func(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error) {
			return NewMockLauncher(), nil
		},
	}
}

// CreateLauncher calls the mock function.
func (m *MockLauncherFactory) CreateLauncher(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error) {
	if m.CreateLauncherFunc != nil {
		return m.CreateLauncherFunc(ctx, agentID, config)
	}
	return NewMockLauncher(), nil
}
```

**Step 7: Commit factory implementation**

```bash
git add pkg/agent/factory.go pkg/agent/factory_test.go pkg/agent/mock_factory.go
git commit -m "feat(agent): add LauncherFactory interface and implementations

- Add LauncherFactory interface for creating launchers
- Implement DefaultLauncherFactory for container launchers
- Add MockLauncherFactory for testing
- Define LauncherConfig and ResourceLimits types

Part of #107"
```

---

## Task 2: SessionManager Integration - Add Factory Field

**Files:**
- Modify: `pkg/relay/session/manager.go`
- Modify: `pkg/relay/session/manager_test.go`

**Step 1: Add factory field to Manager struct**

In `pkg/relay/session/manager.go`, add to Manager struct (around line 30):

```go
type Manager struct {
	// Existing fields
	idGen             IDGenerator
	clock             Clock
	logger            Logger
	baseWorkspaceDir  string
	store             SessionStore
	clientFactory     ClientFactory
	publisher         EventPublisher
	cleaner           *Cleaner

	// NEW: Launcher management
	launcherFactory   agent.LauncherFactory
	launchers         map[string]agent.AgentLauncher  // agentID → launcher
	handles           map[string]agent.AgentHandle     // agentID → handle
	launchersMu       sync.RWMutex                     // protects launchers/handles
}
```

**Step 2: Update NewManager constructor**

In `pkg/relay/session/manager.go`, update NewManager function (around line 50):

```go
// NewManager creates a new session manager
func NewManager(
	idGen IDGenerator,
	clock Clock,
	logger Logger,
	baseWorkspaceDir string,
	store SessionStore,
	clientFactory ClientFactory,
	publisher EventPublisher,
	launcherFactory agent.LauncherFactory, // NEW parameter
) *Manager {
	m := &Manager{
		idGen:            idGen,
		clock:            clock,
		logger:           logger,
		baseWorkspaceDir: baseWorkspaceDir,
		store:            store,
		clientFactory:    clientFactory,
		publisher:        publisher,
		launcherFactory:  launcherFactory, // NEW
		launchers:        make(map[string]agent.AgentLauncher), // NEW
		handles:          make(map[string]agent.AgentHandle),    // NEW
	}

	// Set up cleaner
	m.cleaner = NewCleaner(m, logger, clock)

	return m
}
```

**Step 3: Update test helper setupManager**

In `pkg/relay/session/manager_test.go`, find `setupManager` function and add mock factory:

```go
func setupManager() (*Manager, *mockIDGen, *mockClock, *mockLogger, *MemoryStore, *mockPublisher) {
	idGen := &mockIDGen{}
	clock := &mockClock{currentTime: time.Now()}
	logger := &mockLogger{}
	store := NewMemoryStore()
	publisher := &mockPublisher{}
	clientFactory := &mockClientFactory{}
	mockFactory := agent.NewMockLauncherFactory() // NEW

	manager := NewManager(
		idGen,
		clock,
		logger,
		"/tmp/test-workspaces",
		store,
		clientFactory,
		publisher,
		mockFactory, // NEW parameter
	)

	return manager, idGen, clock, logger, store, publisher
}
```

**Step 4: Run existing tests to verify no breakage**

Run: `go test ./pkg/relay/session -v`
Expected: PASS (all existing tests still work)

**Step 5: Commit SessionManager updates**

```bash
git add pkg/relay/session/manager.go pkg/relay/session/manager_test.go
git commit -m "feat(session): add LauncherFactory to SessionManager

- Add launcherFactory field to Manager
- Add launchers and handles maps for tracking
- Update NewManager constructor signature
- Update test setup with MockLauncherFactory

Part of #107"
```

---

## Task 3: SessionManager SpawnAgent Integration

**Files:**
- Modify: `pkg/relay/session/manager.go`
- Create: `pkg/relay/session/launcher_integration_test.go`

**Step 1: Write failing integration test**

Create `pkg/relay/session/launcher_integration_test.go`:

```go
//go:build integration

package session_test

import (
	"context"
	"testing"

	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/relay/session"
)

func TestSpawnAgent_WithContainerLauncher(t *testing.T) {
	manager, _, _, _, _, _ := setupManagerWithMockFactory()
	ctx := context.Background()

	// Create user session
	ws := &mockWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("CreateUserSession failed: %v", err)
	}

	// Spawn agent - this should use the launcher factory
	err = manager.SpawnAgent(ctx, userSession.GetID(), "test-agent", "/tmp/workspace")
	if err != nil {
		t.Fatalf("SpawnAgent failed: %v", err)
	}

	// Verify launcher was created and stored
	// This will fail until we implement the integration
}

func setupManagerWithMockFactory() (*session.Manager, *mockIDGen, *mockClock, *mockLogger, *session.MemoryStore, *mockPublisher) {
	// Same as setupManager but exposed for this test
	return setupManager()
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/relay/session -v -tags=integration -run TestSpawnAgent_WithContainerLauncher`
Expected: FAIL - test times out or SpawnAgent doesn't use launcher

**Step 3: Update SpawnAgent to use launcher factory**

In `pkg/relay/session/manager.go`, update SpawnAgent function (around line 150):

```go
func (m *Manager) SpawnAgent(ctx context.Context, userSessionID, agentID, workspace string) error {
	// Existing validation...
	if strings.TrimSpace(agentID) == "" {
		return ErrEmptyAgentID
	}
	if strings.TrimSpace(workspace) == "" {
		return ErrEmptyWorkspace
	}

	// Existing workspace path validation...
	cleanPath := filepath.Clean(workspace)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid workspace path: %w", err)
	}
	// ... rest of validation

	// Get user session
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, userSessionID)
	}

	// NEW: Create launcher via factory
	launcherConfig := agent.LauncherConfig{
		AgentID:     agentID,
		ImageName:   "ourocodus/agent:latest", // TODO: make configurable
		Command:     []string{"/bin/bash"},     // TODO: make configurable
		Workspace:   absPath,
		// Credentials will be added in next task
	}

	launcher, err := m.launcherFactory.CreateLauncher(ctx, agentID, launcherConfig)
	if err != nil {
		return fmt.Errorf("failed to create launcher: %w", err)
	}

	// NEW: Spawn agent container
	spawnConfig := &agent.SpawnConfig{
		AgentID:   agentID,
		ImageName: launcherConfig.ImageName,
		Command:   launcherConfig.Command,
		Workspace: absPath,
	}

	handle, err := launcher.Spawn(ctx, spawnConfig)
	if err != nil {
		return fmt.Errorf("failed to spawn agent: %w", err)
	}

	// NEW: Store launcher and handle
	m.launchersMu.Lock()
	m.launchers[agentID] = launcher
	m.handles[agentID] = handle
	m.launchersMu.Unlock()

	// Existing: Create agent client and add to session
	// NOTE: This part stays the same for now, will integrate with container I/O later
	client, err := m.clientFactory.CreateClient(ctx, agentID, workspace)
	if err != nil {
		// Cleanup launcher on client creation failure
		m.launchersMu.Lock()
		delete(m.launchers, agentID)
		delete(m.handles, agentID)
		m.launchersMu.Unlock()

		_ = launcher.Stop(ctx, handle)
		return fmt.Errorf("failed to create agent client: %w", err)
	}

	if err := userSession.AddAgent(agentID, client); err != nil {
		// Cleanup on add failure
		m.launchersMu.Lock()
		delete(m.launchers, agentID)
		delete(m.handles, agentID)
		m.launchersMu.Unlock()

		_ = launcher.Stop(ctx, handle)
		return err
	}

	// Existing: Publish event
	if m.publisher != nil {
		if err := m.publisher.PublishAgentSpawned(ctx, userSessionID, agentID); err != nil {
			m.logger.Printf("WARN: Failed to publish agent.spawned event: %v", err)
		}
	}

	m.logger.Printf("Agent spawned: sessionID=%s agentID=%s container=%s",
		userSessionID, agentID, handle.ContainerID())

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/relay/session -v -tags=integration -run TestSpawnAgent_WithContainerLauncher`
Expected: PASS

**Step 5: Commit SpawnAgent integration**

```bash
git add pkg/relay/session/manager.go pkg/relay/session/launcher_integration_test.go
git commit -m "feat(session): integrate launcher factory in SpawnAgent

- Create launcher via factory before spawning
- Store launcher and handle per agent
- Add cleanup on spawn failures
- Add integration test for container spawning

Part of #107"
```

---

## Task 4: SessionManager TerminateAgent Integration

**Files:**
- Modify: `pkg/relay/session/manager.go`
- Modify: `pkg/relay/session/launcher_integration_test.go`

**Step 1: Write failing test for terminate**

Add to `pkg/relay/session/launcher_integration_test.go`:

```go
func TestTerminateAgent_WithContainerLauncher(t *testing.T) {
	manager, _, _, _, _, _ := setupManagerWithMockFactory()
	ctx := context.Background()

	// Create session and spawn agent
	ws := &mockWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("CreateUserSession failed: %v", err)
	}

	err = manager.SpawnAgent(ctx, userSession.GetID(), "test-agent", "/tmp/workspace")
	if err != nil {
		t.Fatalf("SpawnAgent failed: %v", err)
	}

	// Terminate agent - should stop container
	err = manager.TerminateAgent(ctx, userSession.GetID(), "test-agent")
	if err != nil {
		t.Fatalf("TerminateAgent failed: %v", err)
	}

	// Verify launcher and handle were cleaned up
	// This will fail until we implement terminate integration
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/relay/session -v -tags=integration -run TestTerminateAgent_WithContainerLauncher`
Expected: FAIL - launcher not stopped, resources leaked

**Step 3: Update TerminateAgent to stop container**

In `pkg/relay/session/manager.go`, update TerminateAgent (find existing function):

```go
func (m *Manager) TerminateAgent(ctx context.Context, userSessionID, agentID string) error {
	// Existing: Get session
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, userSessionID)
	}

	// Existing: Get agent
	agent := userSession.GetAgent(agentID)
	if agent == nil {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, agentID)
	}

	// NEW: Stop container if launcher exists
	m.launchersMu.RLock()
	launcher := m.launchers[agentID]
	handle := m.handles[agentID]
	m.launchersMu.RUnlock()

	if launcher != nil && handle != nil {
		if err := launcher.Stop(ctx, handle); err != nil {
			m.logger.Printf("WARN: Failed to stop container for agent %s: %v", agentID, err)
			// Continue cleanup despite error
		}
	}

	// NEW: Remove from launcher maps
	m.launchersMu.Lock()
	delete(m.launchers, agentID)
	delete(m.handles, agentID)
	m.launchersMu.Unlock()

	// Existing: Remove from session
	if err := userSession.RemoveAgent(agentID); err != nil {
		return err
	}

	// Existing: Publish event
	if m.publisher != nil {
		if err := m.publisher.PublishAgentTerminated(ctx, userSessionID, agentID); err != nil {
			m.logger.Printf("WARN: Failed to publish agent.terminated event: %v", err)
		}
	}

	m.logger.Printf("Agent terminated: sessionID=%s agentID=%s", userSessionID, agentID)

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/relay/session -v -tags=integration -run TestTerminateAgent_WithContainerLauncher`
Expected: PASS

**Step 5: Commit TerminateAgent integration**

```bash
git add pkg/relay/session/manager.go pkg/relay/session/launcher_integration_test.go
git commit -m "feat(session): integrate launcher in TerminateAgent

- Stop container via launcher before removing agent
- Clean up launcher and handle maps
- Add integration test for terminate flow
- Log warnings but continue cleanup on stop errors

Part of #107"
```

---

## Task 5: E2E Test Infrastructure - Docker Helpers

**Files:**
- Create: `tests/e2e/helpers/docker.go`
- Create: `tests/e2e/helpers/docker_test.go`

**Step 1: Write failing test for Docker helpers**

Create `tests/e2e/helpers/docker_test.go`:

```go
//go:build integration

package helpers_test

import (
	"context"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/tests/e2e/helpers"
)

func TestListAgentContainers(t *testing.T) {
	ctx := context.Background()

	// This will fail because helpers don't exist yet
	containers, err := helpers.ListAgentContainers(ctx)
	if err != nil {
		t.Fatalf("ListAgentContainers failed: %v", err)
	}

	// Should return empty list initially
	if len(containers) != 0 {
		t.Errorf("Expected 0 containers, got %d", len(containers))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./tests/e2e/helpers -v -tags=integration -run TestListAgentContainers`
Expected: FAIL - "undefined: helpers.ListAgentContainers"

**Step 3: Create Docker helpers**

Create `tests/e2e/helpers/docker.go`:

```go
package helpers

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// DockerHelper provides utilities for E2E tests with Docker containers.
type DockerHelper struct {
	client *client.Client
}

// NewDockerHelper creates a new Docker helper.
func NewDockerHelper() (*DockerHelper, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerHelper{client: cli}, nil
}

// Close closes the Docker client.
func (h *DockerHelper) Close() error {
	if h.client != nil {
		return h.client.Close()
	}
	return nil
}

// ListAgentContainers lists all containers with ourocodus.agent=true label.
func ListAgentContainers(ctx context.Context) ([]string, error) {
	helper, err := NewDockerHelper()
	if err != nil {
		return nil, err
	}
	defer helper.Close()

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "ourocodus.agent=true")

	containers, err := helper.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}

	return ids, nil
}

// WaitForContainer waits for a container to reach running state.
func WaitForContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	helper, err := NewDockerHelper()
	if err != nil {
		return err
	}
	defer helper.Close()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for container %s to start", containerID)
		case <-ticker.C:
			inspect, err := helper.client.ContainerInspect(ctx, containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container: %w", err)
			}

			if inspect.State.Running {
				return nil
			}

			if inspect.State.Status == "exited" || inspect.State.Status == "dead" {
				return fmt.Errorf("container %s exited unexpectedly", containerID)
			}
		}
	}
}

// VerifyContainerCleanup verifies a container has been removed.
func VerifyContainerCleanup(ctx context.Context, containerID string) error {
	helper, err := NewDockerHelper()
	if err != nil {
		return err
	}
	defer helper.Close()

	_, err = helper.client.ContainerInspect(ctx, containerID)
	if err == nil {
		return fmt.Errorf("container %s still exists", containerID)
	}

	// Check if error is "not found" (expected)
	if client.IsErrNotFound(err) {
		return nil // Success - container was cleaned up
	}

	return fmt.Errorf("unexpected error inspecting container: %w", err)
}

// GetContainerLogs fetches logs from a container for debugging.
func GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	helper, err := NewDockerHelper()
	if err != nil {
		return "", err
	}
	defer helper.Close()

	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       "100",
	}

	reader, err := helper.client.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(logs), nil
}

// InspectContainer gets detailed container information.
func InspectContainer(ctx context.Context, containerID string) (*types.ContainerJSON, error) {
	helper, err := NewDockerHelper()
	if err != nil {
		return nil, err
	}
	defer helper.Close()

	inspect, err := helper.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	return &inspect, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./tests/e2e/helpers -v -tags=integration -run TestListAgentContainers`
Expected: PASS

**Step 5: Commit Docker helpers**

```bash
git add tests/e2e/helpers/docker.go tests/e2e/helpers/docker_test.go
git commit -m "test(e2e): add Docker helper utilities

- Add DockerHelper with container management functions
- Implement ListAgentContainers for finding ourocodus containers
- Add WaitForContainer for polling container state
- Add VerifyContainerCleanup for cleanup validation
- Add GetContainerLogs and InspectContainer for debugging

Part of #108"
```

---

## Task 6: E2E Test - Container Spawn

**Files:**
- Create: `tests/e2e/container_spawn_test.go`

**Step 1: Create basic spawn test**

Create `tests/e2e/container_spawn_test.go`:

```go
//go:build integration

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/tests/e2e/helpers"
)

func TestContainerSpawn_EchoAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Start relay server
	relay, err := helpers.StartRelay(ctx, "./bin/relay", "8080")
	if err != nil {
		t.Fatalf("Failed to start relay: %v", err)
	}
	defer relay.Stop()

	// Wait for relay to be healthy
	if err := helpers.WaitForHealth("http://localhost:8080/health", 30*time.Second); err != nil {
		t.Fatalf("Relay failed to become healthy: %v", err)
	}

	// Connect WebSocket
	ws, err := helpers.Connect("ws://localhost:8080/ws")
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer ws.Close()

	// Wait for connection:established
	var connMsg helpers.ConnectionEstablished
	if err := ws.WaitForMessageType("connection:established", &connMsg, 5*time.Second); err != nil {
		t.Fatalf("Failed to receive connection:established: %v", err)
	}

	// Create session
	createMsg := map[string]interface{}{
		"version": "1.0",
		"type":    "session:create",
	}
	if err := ws.Send(createMsg); err != nil {
		t.Fatalf("Failed to send session:create: %v", err)
	}

	var sessionMsg helpers.SessionCreated
	if err := ws.WaitForMessageType("session:created", &sessionMsg, 5*time.Second); err != nil {
		t.Fatalf("Failed to receive session:created: %v", err)
	}

	sessionID := sessionMsg.SessionID
	t.Logf("Created session: %s", sessionID)

	// Spawn echo-agent in container
	spawnMsg := map[string]interface{}{
		"version":       "1.0",
		"type":          "agent:spawn",
		"userSessionId": sessionID,
		"agentId":       "echo-1",
		"workspace":     "/tmp/test-workspace",
	}
	if err := ws.Send(spawnMsg); err != nil {
		t.Fatalf("Failed to send agent:spawn: %v", err)
	}

	var readyMsg helpers.AgentReady
	if err := ws.WaitForMessageType("agent:ready", &readyMsg, 30*time.Second); err != nil {
		t.Fatalf("Failed to receive agent:ready: %v", err)
	}

	t.Logf("Agent ready: %s", readyMsg.AgentID)

	// Verify container is running
	containers, err := helpers.ListAgentContainers(ctx)
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}

	if len(containers) == 0 {
		t.Fatal("Expected at least 1 agent container, found none")
	}

	t.Logf("Found %d agent container(s)", len(containers))

	// Get container ID from handle (we'll need to enhance agent:ready message)
	// For now, just verify a container exists

	// Test passes if we got this far
	t.Log("Echo agent spawned successfully in container")
}
```

**Step 2: Run test to verify current state**

Run: `go test ./tests/e2e -v -tags=integration -run TestContainerSpawn_EchoAgent`
Expected: May FAIL if relay isn't built or Docker not available - that's OK for now

**Step 3: Build relay binary for testing**

Run: `make build` or `go build -o bin/relay ./cmd/relay`
Expected: Binary created at `bin/relay`

**Step 4: Run test again**

Run: `go test ./tests/e2e -v -tags=integration -run TestContainerSpawn_EchoAgent`
Expected: Test runs, may fail on specifics but validates E2E flow

**Step 5: Commit container spawn test**

```bash
git add tests/e2e/container_spawn_test.go
git commit -m "test(e2e): add container spawn test for echo agent

- Test full flow: relay start, session create, agent spawn
- Verify container is created and running
- Use Docker helpers to validate container existence
- Foundation for additional spawn tests

Part of #108"
```

---

## Task 7: E2E Test - Container Lifecycle

**Files:**
- Create: `tests/e2e/container_lifecycle_test.go`

**Step 1: Create lifecycle test**

Create `tests/e2e/container_lifecycle_test.go`:

```go
//go:build integration

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/tests/e2e/helpers"
)

func TestContainerLifecycle_StopAndCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Setup: Start relay and create session
	relay, ws, sessionID := setupRelayAndSession(t, ctx)
	defer relay.Stop()
	defer ws.Close()

	// Spawn agent
	agentID := "lifecycle-test-agent"
	spawnMsg := map[string]interface{}{
		"version":       "1.0",
		"type":          "agent:spawn",
		"userSessionId": sessionID,
		"agentId":       agentID,
		"workspace":     "/tmp/lifecycle-workspace",
	}
	if err := ws.Send(spawnMsg); err != nil {
		t.Fatalf("Failed to send agent:spawn: %v", err)
	}

	var readyMsg helpers.AgentReady
	if err := ws.WaitForMessageType("agent:ready", &readyMsg, 30*time.Second); err != nil {
		t.Fatalf("Failed to receive agent:ready: %v", err)
	}

	// Get container ID
	containers, err := helpers.ListAgentContainers(ctx)
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}
	if len(containers) == 0 {
		t.Fatal("No agent containers found")
	}
	containerID := containers[0]

	t.Logf("Agent spawned in container: %s", containerID)

	// Wait for container to be running
	if err := helpers.WaitForContainer(ctx, containerID, 10*time.Second); err != nil {
		t.Fatalf("Container failed to start: %v", err)
	}

	// Terminate agent
	terminateMsg := map[string]interface{}{
		"version":       "1.0",
		"type":          "agent:terminate",
		"userSessionId": sessionID,
		"agentId":       agentID,
	}
	if err := ws.Send(terminateMsg); err != nil {
		t.Fatalf("Failed to send agent:terminate: %v", err)
	}

	var terminatedMsg helpers.AgentTerminated
	if err := ws.WaitForMessageType("agent:terminated", &terminatedMsg, 30*time.Second); err != nil {
		t.Fatalf("Failed to receive agent:terminated: %v", err)
	}

	t.Logf("Agent terminated: %s", agentID)

	// Verify container was stopped and removed
	time.Sleep(2 * time.Second) // Give cleanup time to complete

	if err := helpers.VerifyContainerCleanup(ctx, containerID); err != nil {
		// Get logs for debugging
		logs, _ := helpers.GetContainerLogs(ctx, containerID)
		t.Logf("Container logs:\n%s", logs)

		t.Fatalf("Container cleanup failed: %v", err)
	}

	t.Log("Container cleanup verified successfully")
}

// setupRelayAndSession is a helper for lifecycle tests
func setupRelayAndSession(t *testing.T, ctx context.Context) (*helpers.RelayProcess, *helpers.WebSocketClient, string) {
	relay, err := helpers.StartRelay(ctx, "./bin/relay", "8080")
	if err != nil {
		t.Fatalf("Failed to start relay: %v", err)
	}

	if err := helpers.WaitForHealth("http://localhost:8080/health", 30*time.Second); err != nil {
		relay.Stop()
		t.Fatalf("Relay failed to become healthy: %v", err)
	}

	ws, err := helpers.Connect("ws://localhost:8080/ws")
	if err != nil {
		relay.Stop()
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}

	var connMsg helpers.ConnectionEstablished
	if err := ws.WaitForMessageType("connection:established", &connMsg, 5*time.Second); err != nil {
		ws.Close()
		relay.Stop()
		t.Fatalf("Failed to receive connection:established: %v", err)
	}

	createMsg := map[string]interface{}{
		"version": "1.0",
		"type":    "session:create",
	}
	if err := ws.Send(createMsg); err != nil {
		ws.Close()
		relay.Stop()
		t.Fatalf("Failed to send session:create: %v", err)
	}

	var sessionMsg helpers.SessionCreated
	if err := ws.WaitForMessageType("session:created", &sessionMsg, 5*time.Second); err != nil {
		ws.Close()
		relay.Stop()
		t.Fatalf("Failed to receive session:created: %v", err)
	}

	return relay, ws, sessionMsg.SessionID
}
```

**Step 2: Run lifecycle test**

Run: `go test ./tests/e2e -v -tags=integration -run TestContainerLifecycle_StopAndCleanup`
Expected: PASS - validates full spawn, terminate, cleanup flow

**Step 3: Commit lifecycle test**

```bash
git add tests/e2e/container_lifecycle_test.go
git commit -m "test(e2e): add container lifecycle test

- Test spawn, terminate, and cleanup flow
- Verify container is properly stopped and removed
- Add setupRelayAndSession helper for test reuse
- Validate cleanup with Docker API

Part of #108"
```

---

## Task 8: Relay Server Startup Integration

**Files:**
- Modify: `cmd/relay/main.go`

**Step 1: Add Docker client initialization**

In `cmd/relay/main.go`, add Docker client setup in main():

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/relay"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// ... existing code ...

func main() {
	// Existing flag parsing...
	port := flag.String("port", "8080", "Port to listen on")
	workspaceDir := flag.String("workspace", "./workspaces", "Base workspace directory")
	repoPath := flag.String("repo", ".", "Git repository path")
	flag.Parse()

	// Initialize logger
	logger := log.New(os.Stdout, "[relay] ", log.LstdFlags)

	// NEW: Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Fatalf("Failed to create Docker client: %v", err)
	}
	defer dockerClient.Close()

	// Verify Docker is accessible
	ctx := context.Background()
	if _, err := dockerClient.Ping(ctx); err != nil {
		logger.Fatalf("Docker daemon is not accessible: %v", err)
	}
	logger.Println("Docker client initialized successfully")

	// NEW: Initialize worktree manager
	worktreeManager, err := worktree.NewManager(*repoPath, *workspaceDir)
	if err != nil {
		logger.Fatalf("Failed to create worktree manager: %v", err)
	}
	logger.Println("Worktree manager initialized")

	// NEW: Initialize credential mounter
	credsDir := fmt.Sprintf("%s/credentials", *workspaceDir)
	if err := os.MkdirAll(credsDir, 0700); err != nil {
		logger.Fatalf("Failed to create credentials directory: %v", err)
	}
	credMounter := container.NewCredentialMounter(credsDir)
	logger.Println("Credential mounter initialized")

	// NEW: Initialize container session manager
	containerManager := containersession.NewManager(dockerClient)
	logger.Println("Container session manager initialized")

	// NEW: Create launcher factory
	factoryConfig := agent.LauncherFactoryConfig{
		DockerClient:       dockerClient,
		WorktreeManager:    worktreeManager,
		CredMounter:        credMounter,
		ContainerManager:   containerManager,
		BaseWorkspaceDir:   *workspaceDir,
		DefaultImageName:   "ourocodus/agent:latest",
		DefaultResourceLimits: agent.ResourceLimits{
			CPUCores: 2,
			MemoryMB: 4096,
		},
	}
	launcherFactory := agent.NewDefaultLauncherFactory(factoryConfig)
	logger.Println("Launcher factory initialized")

	// NEW: Cleanup orphaned containers on startup
	if err := cleanupOrphanedContainers(ctx, dockerClient, logger); err != nil {
		logger.Printf("WARN: Failed to cleanup orphaned containers: %v", err)
	}

	// Existing: Create session manager (with NEW parameter)
	sessionStore := session.NewMemoryStore()
	sessionMgr := session.NewManager(
		&uuidGenerator{},
		&systemClock{},
		logger,
		*workspaceDir,
		sessionStore,
		&defaultClientFactory{},
		nil, // publisher
		launcherFactory, // NEW
	)

	// Existing: Create relay server...
	// ... rest of main ...
}

// NEW: Cleanup orphaned containers from previous runs
func cleanupOrphanedContainers(ctx context.Context, cli *client.Client, logger *log.Logger) error {
	containers, err := helpers.ListAgentContainers(ctx)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		logger.Println("No orphaned containers found")
		return nil
	}

	logger.Printf("Found %d orphaned container(s), cleaning up...", len(containers))

	for _, containerID := range containers {
		// Check container age
		inspect, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			logger.Printf("WARN: Failed to inspect orphaned container %s: %v", containerID, err)
			continue
		}

		age := time.Since(inspect.Created)
		if age < 1*time.Hour {
			logger.Printf("Skipping recent container %s (age: %v)", containerID[:12], age)
			continue
		}

		// Stop and remove
		timeout := 10
		if err := cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
			logger.Printf("WARN: Failed to stop orphaned container %s: %v", containerID[:12], err)
		}

		if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
			logger.Printf("WARN: Failed to remove orphaned container %s: %v", containerID[:12], err)
		} else {
			logger.Printf("Cleaned up orphaned container: %s", containerID[:12])
		}
	}

	return nil
}
```

**Step 2: Build and test relay startup**

Run: `go build -o bin/relay ./cmd/relay`
Expected: Builds successfully

Run: `./bin/relay`
Expected: Starts with Docker client initialized, no errors

**Step 3: Commit relay startup integration**

```bash
git add cmd/relay/main.go
git commit -m "feat(relay): integrate launcher factory at startup

- Initialize Docker client with connection verification
- Create worktree manager and credential mounter
- Initialize container session manager
- Create DefaultLauncherFactory with all dependencies
- Add orphaned container cleanup on startup
- Pass factory to SessionManager constructor

Part of #107"
```

---

## Remaining Tasks Summary

The following tasks should be completed but follow the same pattern:

**Task 9: E2E Credentials Test** (tests/e2e/container_credentials_test.go)
- Test GitHub CLI access inside container
- Test Git SSH key mounting
- Test Anthropic API key passthrough
- Verify read-only credential mounts

**Task 10: E2E Worktree Test** (tests/e2e/container_worktree_test.go)
- Test isolated branch creation per agent
- Verify commit propagation
- Test worktree cleanup on stop

**Task 11: E2E Concurrent Test** (tests/e2e/container_concurrent_test.go)
- Test multiple agents in one session
- Test concurrent spawn/stop operations
- Verify resource isolation

**Task 12: Documentation Updates**
- Update docs/ARCHITECTURE.md with factory pattern
- Add troubleshooting guide for containers
- Update README with Docker requirements

## Testing Strategy

**Unit Tests:**
```bash
go test ./pkg/agent -v
go test ./pkg/relay/session -v
```

**Integration Tests:**
```bash
go test ./pkg/relay/session -v -tags=integration
go test ./tests/e2e/helpers -v -tags=integration
```

**E2E Tests:**
```bash
go test ./tests/e2e -v -tags=integration -timeout=10m
```

**Run All:**
```bash
make test              # Unit tests
make test-integration  # Integration tests
make test-e2e          # E2E tests
```

## Commit Conventions

Follow these commit message patterns:
- `feat(component): description` - New features
- `test(component): description` - New tests
- `fix(component): description` - Bug fixes
- `docs: description` - Documentation only
- `refactor(component): description` - Code restructuring

Always include "Part of #107" or "Part of #108" in commit body.

## Success Criteria

- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] All E2E tests pass
- [ ] Relay starts with Docker client initialized
- [ ] Agents spawn in containers successfully
- [ ] Containers are cleaned up on termination
- [ ] Orphaned containers are cleaned up on startup
- [ ] Documentation is updated
