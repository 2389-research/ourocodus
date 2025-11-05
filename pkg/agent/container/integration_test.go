//go:build integration

package container_test

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

type testIDGenerator struct{}

func (g *testIDGenerator) Generate() string {
	return uuid.New().String()
}

type testClock struct{}

func (c *testClock) Now() time.Time {
	return time.Now()
}

type testLogger struct {
	t *testing.T
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "agent-container-integration-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Configure git for tests
	cmd = exec.Command("git", "config", "user.name", "Integration Test")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.email", "integration@test.example.com")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Create initial commit
	readmePath := filepath.Join(tmpDir, "README.md")
	err = os.WriteFile(readmePath, []byte("# Integration Test Repo\n"), 0o644)
	require.NoError(t, err)

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	return tmpDir
}

// setupDockerClient creates a Docker client
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
				dockerClient.Close()
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
			dockerClient.Close()
		}
	}

	t.Fatalf("Cannot connect to Docker - tried Colima (%s) and Docker Desktop", colimaSocket)
	return nil
}

// Test full Spawn and Stop lifecycle
func TestIntegration_SpawnAndStop(t *testing.T) {
	// Setup
	repoDir := setupTestRepo(t)
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	workspacesDir := filepath.Join(repoDir, "workspaces")
	credentialsDir := filepath.Join(repoDir, "credentials")

	// Create managers
	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	containerMgr := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		workspacesDir,
	)

	credMounter := container.NewAgentCredentialMounter(credentialsDir)

	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		workspacesDir,
	)

	ctx := context.Background()

	// Spawn agent
	sshKey := []byte("test-ssh-key-data")
	githubToken := []byte("test-github-token-data")

	handle, err := launcher.Spawn(ctx, container.SpawnConfig{
		AgentID:     "test-agent-1",
		ImageName:   "ubuntu:latest",
		Command:     []string{"sleep", "30"},
		GitSSHKey:   sshKey,
		GitHubToken: githubToken,
	})
	require.NoError(t, err)
	require.NotNil(t, handle)

	// Verify handle properties
	assert.Equal(t, "test-agent-1", handle.AgentID())
	assert.NotEmpty(t, handle.ContainerID())
	assert.Contains(t, handle.WorkspacePath(), "agent-test-agent-1")
	assert.Contains(t, handle.BranchName(), "agent-test-agent-1")
	assert.Contains(t, handle.CredentialsPath(), "agent-test-agent-1")
	assert.Equal(t, containersession.StateRunning, handle.State())

	// Verify worktree exists
	_, err = os.Stat(handle.WorkspacePath())
	assert.NoError(t, err, "Worktree directory should exist")

	// Verify credentials exist
	_, err = os.Stat(handle.CredentialsPath())
	assert.NoError(t, err, "Credentials directory should exist")

	// Verify SSH key file
	sshKeyPath := filepath.Join(handle.CredentialsPath(), "id_ed25519")
	content, err := os.ReadFile(sshKeyPath)
	require.NoError(t, err)
	assert.Equal(t, sshKey, content)

	// Verify GitHub token file
	tokenPath := filepath.Join(handle.CredentialsPath(), "github-token")
	content, err = os.ReadFile(tokenPath)
	require.NoError(t, err)
	assert.Equal(t, githubToken, content)

	// Stop agent
	err = launcher.Stop(ctx, "test-agent-1")
	require.NoError(t, err)

	// Verify worktree is removed
	_, err = os.Stat(handle.WorkspacePath())
	assert.True(t, os.IsNotExist(err), "Worktree should be removed")

	// Verify credentials are removed
	_, err = os.Stat(handle.CredentialsPath())
	assert.True(t, os.IsNotExist(err), "Credentials should be removed")

	// Verify handle is removed from launcher
	retrievedHandle := launcher.GetHandle("test-agent-1")
	assert.Nil(t, retrievedHandle, "Handle should be removed")
}

// Test spawning multiple agents concurrently
func TestIntegration_ConcurrentAgents(t *testing.T) {
	// Setup
	repoDir := setupTestRepo(t)
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	workspacesDir := filepath.Join(repoDir, "workspaces")
	credentialsDir := filepath.Join(repoDir, "credentials")

	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	containerMgr := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		workspacesDir,
	)

	credMounter := container.NewAgentCredentialMounter(credentialsDir)

	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		workspacesDir,
	)

	ctx := context.Background()

	// Spawn two agents concurrently
	agent1, err := launcher.Spawn(ctx, container.SpawnConfig{
		AgentID:   "agent-1",
		ImageName: "ubuntu:latest",
		Command:   []string{"sleep", "30"},
	})
	require.NoError(t, err)

	agent2, err := launcher.Spawn(ctx, container.SpawnConfig{
		AgentID:   "agent-2",
		ImageName: "ubuntu:latest",
		Command:   []string{"sleep", "30"},
	})
	require.NoError(t, err)

	// Verify both are running
	assert.Equal(t, containersession.StateRunning, agent1.State())
	assert.Equal(t, containersession.StateRunning, agent2.State())

	// Verify isolation - different paths
	assert.NotEqual(t, agent1.WorkspacePath(), agent2.WorkspacePath())
	assert.NotEqual(t, agent1.BranchName(), agent2.BranchName())
	assert.NotEqual(t, agent1.CredentialsPath(), agent2.CredentialsPath())

	// Verify both worktrees exist
	_, err = os.Stat(agent1.WorkspacePath())
	assert.NoError(t, err)
	_, err = os.Stat(agent2.WorkspacePath())
	assert.NoError(t, err)

	// List handles
	handles := launcher.ListHandles()
	assert.Len(t, handles, 2)

	// Stop both
	err = launcher.Stop(ctx, "agent-1")
	require.NoError(t, err)
	err = launcher.Stop(ctx, "agent-2")
	require.NoError(t, err)

	// Verify cleanup
	handles = launcher.ListHandles()
	assert.Empty(t, handles)
}

// Test validation errors
func TestIntegration_ValidationErrors(t *testing.T) {
	repoDir := setupTestRepo(t)
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	workspacesDir := filepath.Join(repoDir, "workspaces")
	credentialsDir := filepath.Join(repoDir, "credentials")

	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	containerMgr := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		workspacesDir,
	)

	credMounter := container.NewAgentCredentialMounter(credentialsDir)

	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		workspacesDir,
	)

	ctx := context.Background()

	t.Run("EmptyAgentID", func(t *testing.T) {
		handle, err := launcher.Spawn(ctx, container.SpawnConfig{
			AgentID:   "",
			ImageName: "ubuntu:latest",
			Command:   []string{"sleep", "10"},
		})
		assert.Error(t, err)
		assert.Nil(t, handle)
		assert.Equal(t, container.ErrInvalidAgentID, err)
	})

	t.Run("EmptyImageName", func(t *testing.T) {
		handle, err := launcher.Spawn(ctx, container.SpawnConfig{
			AgentID:   "test-agent",
			ImageName: "",
			Command:   []string{"sleep", "10"},
		})
		assert.Error(t, err)
		assert.Nil(t, handle)
		assert.Equal(t, container.ErrInvalidImageName, err)
	})

	t.Run("EmptyCommand", func(t *testing.T) {
		handle, err := launcher.Spawn(ctx, container.SpawnConfig{
			AgentID:   "test-agent",
			ImageName: "ubuntu:latest",
			Command:   []string{},
		})
		assert.Error(t, err)
		assert.Nil(t, handle)
		assert.Equal(t, container.ErrInvalidCommand, err)
	})

	t.Run("DuplicateAgentID", func(t *testing.T) {
		// Spawn first agent
		handle1, err := launcher.Spawn(ctx, container.SpawnConfig{
			AgentID:   "duplicate-agent",
			ImageName: "ubuntu:latest",
			Command:   []string{"sleep", "30"},
		})
		require.NoError(t, err)
		defer launcher.Stop(ctx, "duplicate-agent")

		// Try to spawn second agent with same ID
		handle2, err := launcher.Spawn(ctx, container.SpawnConfig{
			AgentID:   "duplicate-agent",
			ImageName: "ubuntu:latest",
			Command:   []string{"sleep", "30"},
		})
		assert.Error(t, err)
		assert.Nil(t, handle2)
		assert.Equal(t, container.ErrAgentAlreadyExists, err)

		// First agent should still be fine
		assert.NotNil(t, handle1)
		assert.Equal(t, containersession.StateRunning, handle1.State())
	})
}

// Test idempotent Stop
func TestIntegration_IdempotentStop(t *testing.T) {
	repoDir := setupTestRepo(t)
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	workspacesDir := filepath.Join(repoDir, "workspaces")
	credentialsDir := filepath.Join(repoDir, "credentials")

	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	containerMgr := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		workspacesDir,
	)

	credMounter := container.NewAgentCredentialMounter(credentialsDir)

	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		workspacesDir,
	)

	ctx := context.Background()

	// Spawn agent
	handle, err := launcher.Spawn(ctx, container.SpawnConfig{
		AgentID:   "idempotent-test",
		ImageName: "ubuntu:latest",
		Command:   []string{"sleep", "30"},
	})
	require.NoError(t, err)

	workspacePath := handle.WorkspacePath()

	// Stop once
	err = launcher.Stop(ctx, "idempotent-test")
	require.NoError(t, err)

	// Stop again - should not error (idempotent)
	err = launcher.Stop(ctx, "idempotent-test")
	assert.NoError(t, err, "Stop should be idempotent")

	// Stop third time
	err = launcher.Stop(ctx, "idempotent-test")
	assert.NoError(t, err, "Stop should be idempotent")

	// Verify still doesn't exist
	_, err = os.Stat(workspacePath)
	assert.True(t, os.IsNotExist(err))
}

// Test panics for nil dependencies
func TestNewAgentContainerLauncher_Panics(t *testing.T) {
	repoDir := setupTestRepo(t)
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	containerMgr := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		log.Default(),
		"./workspaces",
	)

	credMounter := container.NewAgentCredentialMounter("./credentials")

	t.Run("NilContainerMgr", func(t *testing.T) {
		assert.Panics(t, func() {
			container.NewAgentContainerLauncher(nil, worktreeMgr, credMounter, "./workspaces")
		})
	})

	t.Run("NilWorktreeMgr", func(t *testing.T) {
		assert.Panics(t, func() {
			container.NewAgentContainerLauncher(containerMgr, nil, credMounter, "./workspaces")
		})
	})

	t.Run("NilCredMounter", func(t *testing.T) {
		assert.Panics(t, func() {
			container.NewAgentContainerLauncher(containerMgr, worktreeMgr, nil, "./workspaces")
		})
	})
}
