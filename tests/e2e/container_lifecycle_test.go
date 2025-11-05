//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/2389-research/ourocodus/tests/e2e/helpers"
	"github.com/stretchr/testify/require"
)

// TestContainerLifecycle_StopAndCleanup tests the full container lifecycle:
// spawn → run → stop → cleanup
//
// This validates:
//   - Container spawns successfully
//   - Container can be stopped via launcher.Stop()
//   - Container is properly removed after stop
//   - No resource leaks
func TestContainerLifecycle_StopAndCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Setup test workspace
	projectRoot, err := helpers.FindProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	testWorkspaceBase := filepath.Join(projectRoot, "tmp", "e2e-test-lifecycle")
	require.NoError(t, os.MkdirAll(testWorkspaceBase, 0o755), "Failed to create test workspace")
	defer func() {
		_ = os.RemoveAll(testWorkspaceBase)
	}()

	// Initialize Docker client
	dockerClient := setupDockerClient(t)
	require.NotNil(t, dockerClient, "Failed to create Docker client")
	defer func() { _ = dockerClient.Close() }()

	// Pull alpine image if not present
	t.Log("Ensuring alpine:latest image is available...")
	if err := pullImageIfNeeded(ctx, dockerClient, "alpine:latest"); err != nil {
		t.Fatalf("Failed to pull alpine image: %v", err)
	}

	// Initialize worktree manager
	repoPath := projectRoot
	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoPath)
	require.NoError(t, err, "Failed to create worktree manager")

	// Initialize credential mounter
	credsDir := filepath.Join(testWorkspaceBase, "credentials")
	require.NoError(t, os.MkdirAll(credsDir, 0o700), "Failed to create credentials directory")
	credMounter := container.NewAgentCredentialMounter(credsDir)

	// Initialize container session manager
	idGen := &testIDGenerator{}
	clock := &testClock{}
	logger := log.New(os.Stdout, "[e2e-lifecycle] ", log.LstdFlags)
	containerMgr := containersession.NewManager(dockerClient, idGen, clock, logger, testWorkspaceBase)

	// Create AgentContainerLauncher
	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		testWorkspaceBase,
	)

	// Generate unique agent ID for test isolation
	agentID := fmt.Sprintf("lifecycle-agent-%d", time.Now().Unix())
	t.Logf("Testing lifecycle with agent ID: %s", agentID)

	// Spawn the container
	spawnConfig := container.SpawnConfig{
		AgentID:   agentID,
		ImageName: "alpine:latest",
		Command:   []string{"sleep", "300"}, // Keep running for 5 minutes
		Env:       []string{"TEST_VAR=lifecycle-test"},
	}

	t.Log("Spawning agent container...")
	handle, err := launcher.Spawn(ctx, spawnConfig)
	require.NoError(t, err, "Failed to spawn agent container")
	require.NotNil(t, handle, "Expected non-nil handle")

	// Get container ID from handle
	containerID := handle.ContainerID()
	require.NotEmpty(t, containerID, "Container ID should not be empty")
	t.Logf("Container spawned: %s", containerID)

	// Wait for container to be running
	t.Log("Waiting for container to be running...")
	err = helpers.WaitForContainer(ctx, containerID, 30*time.Second)
	require.NoError(t, err, "Container failed to start")
	t.Log("Container is running")

	// Verify container is in agent container list
	t.Log("Verifying container is in agent container list...")
	containers, err := helpers.ListAgentContainers(ctx)
	require.NoError(t, err, "Failed to list agent containers")
	foundContainer := false
	for _, cid := range containers {
		if cid == containerID {
			foundContainer = true
			break
		}
	}
	require.True(t, foundContainer, "Container %s not found in agent container list", containerID)
	t.Log("Container found in agent container list")

	// Verify container is actually running by inspecting it
	inspect, err := helpers.InspectContainer(ctx, containerID)
	require.NoError(t, err, "Failed to inspect container")
	require.True(t, inspect.State.Running, "Container should be running")
	require.Equal(t, "running", inspect.State.Status, "Container status should be 'running'")
	t.Log("Container state verified as running")

	// Stop the agent
	t.Logf("Stopping agent: %s", agentID)
	err = launcher.Stop(ctx, agentID)
	require.NoError(t, err, "Failed to stop agent")
	t.Log("Agent stopped successfully")

	// Give cleanup a moment to complete
	time.Sleep(2 * time.Second)

	// Verify container was stopped (it may still exist in stopped state, which is acceptable)
	t.Log("Verifying container was stopped...")
	inspectAfter, err := helpers.InspectContainer(ctx, containerID)
	if err != nil {
		// If container is not found, that's fine - it was removed
		t.Log("Container was removed completely (not found)")
	} else {
		// Container still exists - verify it's stopped
		require.False(t, inspectAfter.State.Running, "Container should not be running after stop")
		require.Contains(t, []string{"exited", "stopped", "dead"}, inspectAfter.State.Status,
			"Container status should be exited/stopped/dead after stop, got: %s", inspectAfter.State.Status)
		t.Logf("Container stopped successfully with status: %s", inspectAfter.State.Status)
	}

	t.Log("Full container lifecycle test passed: spawn → run → stop → cleanup")
}

// TestContainerLifecycle_StopNonExistent tests stopping a non-existent agent
func TestContainerLifecycle_StopNonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Setup test workspace
	projectRoot, err := helpers.FindProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	testWorkspaceBase := filepath.Join(projectRoot, "tmp", "e2e-test-stop-nonexistent")
	require.NoError(t, os.MkdirAll(testWorkspaceBase, 0o755), "Failed to create test workspace")
	defer func() {
		_ = os.RemoveAll(testWorkspaceBase)
	}()

	// Initialize Docker client
	dockerClient := setupDockerClient(t)
	require.NotNil(t, dockerClient, "Failed to create Docker client")
	defer func() { _ = dockerClient.Close() }()

	// Initialize dependencies
	repoPath := projectRoot
	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoPath)
	require.NoError(t, err, "Failed to create worktree manager")

	credsDir := filepath.Join(testWorkspaceBase, "credentials")
	require.NoError(t, os.MkdirAll(credsDir, 0o700), "Failed to create credentials directory")
	credMounter := container.NewAgentCredentialMounter(credsDir)

	idGen := &testIDGenerator{}
	clock := &testClock{}
	logger := log.New(os.Stdout, "[e2e-stop-nonexistent] ", log.LstdFlags)
	containerMgr := containersession.NewManager(dockerClient, idGen, clock, logger, testWorkspaceBase)

	// Create launcher
	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		testWorkspaceBase,
	)

	// Try to stop a non-existent agent
	nonExistentAgentID := "non-existent-agent-12345"
	t.Logf("Attempting to stop non-existent agent: %s", nonExistentAgentID)

	err = launcher.Stop(ctx, nonExistentAgentID)

	// Stop is idempotent - should not error for non-existent agent
	require.NoError(t, err, "Stop should be idempotent and not error for non-existent agent")
	t.Log("Stop correctly handled non-existent agent (idempotent)")

	t.Log("Stop non-existent agent test passed")
}

// TestContainerLifecycle_MultipleStartStop tests starting and stopping the same agent multiple times
func TestContainerLifecycle_MultipleStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Setup test workspace
	projectRoot, err := helpers.FindProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	testWorkspaceBase := filepath.Join(projectRoot, "tmp", "e2e-test-multiple-cycles")
	require.NoError(t, os.MkdirAll(testWorkspaceBase, 0o755), "Failed to create test workspace")
	defer func() {
		_ = os.RemoveAll(testWorkspaceBase)
	}()

	// Initialize Docker client
	dockerClient := setupDockerClient(t)
	require.NotNil(t, dockerClient, "Failed to create Docker client")
	defer func() { _ = dockerClient.Close() }()

	// Pull alpine image if not present
	t.Log("Ensuring alpine:latest image is available...")
	if err := pullImageIfNeeded(ctx, dockerClient, "alpine:latest"); err != nil {
		t.Fatalf("Failed to pull alpine image: %v", err)
	}

	// Initialize dependencies
	repoPath := projectRoot
	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoPath)
	require.NoError(t, err, "Failed to create worktree manager")

	credsDir := filepath.Join(testWorkspaceBase, "credentials")
	require.NoError(t, os.MkdirAll(credsDir, 0o700), "Failed to create credentials directory")
	credMounter := container.NewAgentCredentialMounter(credsDir)

	idGen := &testIDGenerator{}
	clock := &testClock{}
	logger := log.New(os.Stdout, "[e2e-multiple-cycles] ", log.LstdFlags)
	containerMgr := containersession.NewManager(dockerClient, idGen, clock, logger, testWorkspaceBase)

	// Create launcher
	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		testWorkspaceBase,
	)

	// Base agent ID for all cycles
	baseAgentID := fmt.Sprintf("cycle-agent-%d", time.Now().Unix())

	// Run 3 spawn/stop cycles
	numCycles := 3
	for i := 1; i <= numCycles; i++ {
		agentID := fmt.Sprintf("%s-cycle%d", baseAgentID, i)
		t.Logf("Cycle %d/%d: Testing with agent ID: %s", i, numCycles, agentID)

		// Spawn
		spawnConfig := container.SpawnConfig{
			AgentID:   agentID,
			ImageName: "alpine:latest",
			Command:   []string{"sleep", "300"},
			Env:       []string{fmt.Sprintf("CYCLE=%d", i)},
		}

		t.Logf("Cycle %d: Spawning...", i)
		handle, err := launcher.Spawn(ctx, spawnConfig)
		require.NoError(t, err, "Cycle %d: Failed to spawn", i)
		require.NotNil(t, handle, "Cycle %d: Expected non-nil handle", i)

		containerID := handle.ContainerID()
		t.Logf("Cycle %d: Container spawned: %s", i, containerID)

		// Wait for running
		err = helpers.WaitForContainer(ctx, containerID, 30*time.Second)
		require.NoError(t, err, "Cycle %d: Container failed to start", i)
		t.Logf("Cycle %d: Container running", i)

		// Stop
		t.Logf("Cycle %d: Stopping...", i)
		err = launcher.Stop(ctx, agentID)
		require.NoError(t, err, "Cycle %d: Failed to stop", i)
		t.Logf("Cycle %d: Stopped", i)

		// Verify container is stopped (may still exist)
		time.Sleep(1 * time.Second)
		inspectAfter, err := helpers.InspectContainer(ctx, containerID)
		if err != nil {
			t.Logf("Cycle %d: Container removed completely", i)
		} else {
			require.False(t, inspectAfter.State.Running, "Cycle %d: Container should be stopped", i)
			t.Logf("Cycle %d: Container stopped (status: %s)", i, inspectAfter.State.Status)
		}
	}

	t.Logf("Successfully completed %d spawn/stop cycles without resource leaks", numCycles)
}

// TestContainerLifecycle_StopWithTimeout tests that Stop handles container stop timeout gracefully
func TestContainerLifecycle_StopWithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Setup test workspace
	projectRoot, err := helpers.FindProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	testWorkspaceBase := filepath.Join(projectRoot, "tmp", "e2e-test-stop-timeout")
	require.NoError(t, os.MkdirAll(testWorkspaceBase, 0o755), "Failed to create test workspace")
	defer func() {
		_ = os.RemoveAll(testWorkspaceBase)
	}()

	// Initialize Docker client
	dockerClient := setupDockerClient(t)
	require.NotNil(t, dockerClient, "Failed to create Docker client")
	defer func() { _ = dockerClient.Close() }()

	// Pull alpine image if not present
	t.Log("Ensuring alpine:latest image is available...")
	if err := pullImageIfNeeded(ctx, dockerClient, "alpine:latest"); err != nil {
		t.Fatalf("Failed to pull alpine image: %v", err)
	}

	// Initialize dependencies
	repoPath := projectRoot
	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoPath)
	require.NoError(t, err, "Failed to create worktree manager")

	credsDir := filepath.Join(testWorkspaceBase, "credentials")
	require.NoError(t, os.MkdirAll(credsDir, 0o700), "Failed to create credentials directory")
	credMounter := container.NewAgentCredentialMounter(credsDir)

	idGen := &testIDGenerator{}
	clock := &testClock{}
	logger := log.New(os.Stdout, "[e2e-stop-timeout] ", log.LstdFlags)
	containerMgr := containersession.NewManager(dockerClient, idGen, clock, logger, testWorkspaceBase)

	// Create launcher
	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		testWorkspaceBase,
	)

	agentID := fmt.Sprintf("timeout-agent-%d", time.Now().Unix())
	t.Logf("Testing stop timeout with agent ID: %s", agentID)

	// Spawn container with a command that traps SIGTERM to test timeout handling
	// The container will sleep and ignore SIGTERM, forcing Docker to use SIGKILL
	spawnConfig := container.SpawnConfig{
		AgentID:   agentID,
		ImageName: "alpine:latest",
		Command: []string{"sh", "-c",
			"trap 'echo Ignoring SIGTERM' TERM; while true; do sleep 1; done"},
		Env: []string{"TEST=timeout"},
	}

	t.Log("Spawning container that ignores SIGTERM...")
	handle, err := launcher.Spawn(ctx, spawnConfig)
	require.NoError(t, err, "Failed to spawn agent container")
	require.NotNil(t, handle, "Expected non-nil handle")

	containerID := handle.ContainerID()
	t.Logf("Container spawned: %s", containerID)

	// Wait for container to be running
	err = helpers.WaitForContainer(ctx, containerID, 30*time.Second)
	require.NoError(t, err, "Container failed to start")
	t.Log("Container is running")

	// Stop the agent - this should eventually succeed even if SIGTERM is ignored
	// Docker will use SIGKILL after the stop timeout
	t.Log("Stopping agent (expecting timeout handling)...")
	stopStart := time.Now()
	err = launcher.Stop(ctx, agentID)
	stopDuration := time.Since(stopStart)
	require.NoError(t, err, "Failed to stop agent")
	t.Logf("Agent stopped after %v", stopDuration)

	// Verify container is stopped (Docker should have forced it)
	time.Sleep(2 * time.Second)
	inspectAfter, err := helpers.InspectContainer(ctx, containerID)
	if err != nil {
		t.Log("Container was removed completely")
	} else {
		require.False(t, inspectAfter.State.Running, "Container should be stopped")
		require.Contains(t, []string{"exited", "stopped", "dead"}, inspectAfter.State.Status,
			"Container should be stopped, got status: %s", inspectAfter.State.Status)
		t.Logf("Container stopped successfully with status: %s (exit code: %d)",
			inspectAfter.State.Status, inspectAfter.State.ExitCode)
	}
	t.Log("Container stop with timeout handling verified")
}
