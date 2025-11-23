//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/2389-research/ourocodus/tests/e2e/helpers"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Test helpers for container session manager
type testIDGenerator struct{}

func (g *testIDGenerator) Generate() string {
	return uuid.New().String()
}

type testClock struct{}

func (c *testClock) Now() time.Time {
	return time.Now()
}

// setupDockerClient creates a Docker client, trying Colima first then Docker Desktop
func setupDockerClient(t *testing.T) *client.Client {
	t.Helper()

	// Try Colima socket first
	colimaSocket := filepath.Join(os.Getenv("HOME"), ".colima", "default", "docker.sock")
	if _, err := os.Stat(colimaSocket); err == nil {
		if err := os.Setenv("DOCKER_HOST", "unix://"+colimaSocket); err == nil {
			dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if _, err := dockerClient.Ping(ctx); err == nil {
					t.Logf("Using Colima at %s", colimaSocket)
					return dockerClient
				}
				_ = dockerClient.Close()
			}
		}
	}

	// Try Docker Desktop
	if err := os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock"); err == nil {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := dockerClient.Ping(ctx); err == nil {
				t.Log("Using Docker Desktop")
				return dockerClient
			}
			_ = dockerClient.Close()
		}
	}

	t.Skipf("Cannot connect to Docker - tried Colima (%s) and Docker Desktop. Skipping test.", colimaSocket)
	return nil
}

// pullImageIfNeeded pulls a Docker image if it's not already present
func pullImageIfNeeded(ctx context.Context, dockerClient *client.Client, imageName string) error {
	// Check if image exists
	_, _, err := dockerClient.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		// Image exists
		return nil
	}

	// Pull the image
	reader, err := dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Consume the pull output
	_, _ = io.Copy(io.Discard, reader)

	return nil
}

// TestContainerSpawn_EchoAgent tests basic container spawning with an echo agent.
// This validates the full spawn flow through AgentContainerLauncher:
//   - Container is created and running
//   - Container has proper labels
//   - Workspace is mounted correctly
//   - Container can be cleaned up
func TestContainerSpawn_EchoAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Setup test workspace
	projectRoot, err := helpers.FindProjectRoot()
	require.NoError(t, err, "Failed to find project root")

	testWorkspaceBase := filepath.Join(projectRoot, "tmp", "e2e-test-containers")
	require.NoError(t, os.MkdirAll(testWorkspaceBase, 0o755), "Failed to create test workspace")
	defer func() {
		_ = os.RemoveAll(testWorkspaceBase)
	}()

	// Initialize Docker client - try Colima first, then Docker Desktop
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

	// Initialize container session manager with required dependencies
	idGen := &testIDGenerator{}
	clock := &testClock{}
	logger := log.New(os.Stdout, "[e2e-test] ", log.LstdFlags)
	containerMgr := containersession.NewManager(dockerClient, idGen, clock, logger, testWorkspaceBase)

	// Create AgentContainerLauncher
	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		testWorkspaceBase,
	)

	// Generate unique agent ID for test isolation
	agentID := fmt.Sprintf("echo-agent-%d", time.Now().Unix())
	t.Logf("Testing with agent ID: %s", agentID)

	// Use alpine image with a simple sleep command
	// Alpine is small and widely available
	spawnConfig := container.SpawnConfig{
		AgentID:   agentID,
		ImageName: "alpine:latest",
		Command:   []string{"sleep", "300"}, // Keep container running for 5 minutes
		Env:       []string{"TEST_VAR=e2e-test"},
	}

	// Spawn the container
	t.Log("Spawning agent container...")
	handle, err := launcher.Spawn(ctx, spawnConfig)
	require.NoError(t, err, "Failed to spawn agent container")
	require.NotNil(t, handle, "Expected non-nil handle")

	// Ensure cleanup happens even if test fails
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		t.Log("Cleaning up agent container...")
		if err := launcher.Stop(cleanupCtx, agentID); err != nil {
			t.Logf("Warning: Failed to stop agent: %v", err)
		}
	}()

	// Get container ID from handle
	containerID := handle.ContainerID()
	require.NotEmpty(t, containerID, "Container ID should not be empty")
	t.Logf("Container ID: %s", containerID)

	// Wait for container to be running
	t.Log("Waiting for container to be running...")
	err = helpers.WaitForContainer(ctx, containerID, 30*time.Second)
	require.NoError(t, err, "Container failed to start")
	t.Log("Container is running")

	// Verify container exists in agent container list
	t.Log("Verifying container is in agent container list...")
	containers, err := helpers.ListAgentContainers(ctx)
	require.NoError(t, err, "Failed to list agent containers")
	require.Greater(t, len(containers), 0, "Expected at least 1 agent container")

	foundContainer := false
	for _, cid := range containers {
		if cid == containerID {
			foundContainer = true
			break
		}
	}
	require.True(t, foundContainer, "Container %s not found in agent container list", containerID)
	t.Log("Container found in agent container list")

	// Inspect container to verify configuration
	t.Log("Inspecting container configuration...")
	inspect, err := helpers.InspectContainer(ctx, containerID)
	require.NoError(t, err, "Failed to inspect container")

	// Verify labels
	require.NotNil(t, inspect.Config.Labels, "Container labels should not be nil")
	require.Equal(t, "true", inspect.Config.Labels["ourocodus.agent"], "Expected ourocodus.agent label to be true")
	require.Equal(t, agentID, inspect.Config.Labels["ourocodus.agent/agent-id"], "Expected ourocodus.agent/agent-id label to match")
	t.Log("Container labels verified")

	// Verify workspace mount
	require.NotNil(t, inspect.Mounts, "Container mounts should not be nil")
	foundWorkspaceMount := false
	for _, mount := range inspect.Mounts {
		if mount.Destination == "/workspace" {
			foundWorkspaceMount = true
			require.NotEmpty(t, mount.Source, "Workspace mount source should not be empty")
			t.Logf("Workspace mounted from: %s", mount.Source)
			break
		}
	}
	require.True(t, foundWorkspaceMount, "Expected /workspace mount to exist")
	t.Log("Workspace mount verified")

	// Verify container is actually running
	require.True(t, inspect.State.Running, "Container should be in running state")
	require.Equal(t, "running", inspect.State.Status, "Container status should be 'running'")
	t.Log("Container state verified")

	t.Log("Echo agent spawned successfully - all verifications passed")
}
