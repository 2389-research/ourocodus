//go:build integration

package containersession_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// Test helpers

type testIDGenerator struct{}

func (g *testIDGenerator) Generate() string {
	return uuid.New().String()
}

type fixedIDGenerator struct {
	id string
}

func (g *fixedIDGenerator) Generate() string {
	return g.id
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

// cleanupContainers stops and removes containers for given session IDs
func cleanupContainers(t *testing.T, dockerClient *client.Client, sessionIDs []string) {
	t.Helper()
	ctx := context.Background()

	for _, sessionID := range sessionIDs {
		// Find containers with this session ID
		filterArgs := containersession.BuildLabelFilters(sessionID)

		containers, err := dockerClient.ContainerList(ctx, container.ListOptions{
			All:     true,
			Filters: filterArgs,
		})
		if err != nil {
			t.Logf("Warning: Failed to list containers for cleanup: %v", err)
			continue
		}

		for _, c := range containers {
			// Stop with short timeout
			timeout := 2
			if err := dockerClient.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
				t.Logf("Warning: Failed to stop container %s: %v", c.ID[:12], err)
			}

			// Remove
			if err := dockerClient.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
				t.Logf("Warning: Failed to remove container %s: %v", c.ID[:12], err)
			} else {
				t.Logf("Cleaned up container %s for session %s", c.ID[:12], sessionID)
			}
		}
	}
}

// cleanupWorkspace removes workspace directory
func cleanupWorkspace(t *testing.T, workspacePath string) {
	t.Helper()
	if err := os.RemoveAll(workspacePath); err != nil {
		t.Logf("Warning: Failed to cleanup workspace %s: %v", workspacePath, err)
	}
}

// TestIntegration_CreateStartStop tests the basic container lifecycle
func TestIntegration_CreateStartStop(t *testing.T) {
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	ctx := context.Background()
	baseWorkspace := filepath.Join(os.TempDir(), "containersession-test", t.Name())
	defer os.RemoveAll(baseWorkspace)

	manager := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	// Create session
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "30"})
	require.NoError(t, err)
	require.NotNil(t, session)
	defer cleanupContainers(t, dockerClient, []string{session.ID()})

	assert.Equal(t, containersession.StatePending, session.State())
	assert.NotEmpty(t, session.ID())
	assert.NotEmpty(t, session.ContainerID())

	// Start session
	err = manager.StartContainerSession(ctx, session.ID())
	require.NoError(t, err)
	assert.Equal(t, containersession.StateRunning, session.State())

	// Stop session with timeout (must be longer than Docker's 30s stop grace period)
	stopCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	err = manager.StopContainerSession(stopCtx, session.ID())
	require.NoError(t, err)
	assert.Equal(t, containersession.StateStopped, session.State())
}

// TestIntegration_ContainerReuse_Running tests reusing a running container
func TestIntegration_ContainerReuse_Running(t *testing.T) {
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	ctx := context.Background()
	baseWorkspace := filepath.Join(os.TempDir(), "containersession-test", t.Name())
	defer os.RemoveAll(baseWorkspace)

	sessionID := uuid.New().String()
	defer cleanupContainers(t, dockerClient, []string{sessionID})

	// Manager 1: Create and start
	manager1 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	session1, err := manager1.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "60"})
	require.NoError(t, err)
	err = manager1.StartContainerSession(ctx, session1.ID())
	require.NoError(t, err)

	originalContainerID := session1.ContainerID()

	// Manager 2: Reuse running container
	manager2 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	session2, err := manager2.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "60"})
	require.NoError(t, err)

	// Should reuse the same container
	assert.Equal(t, originalContainerID, session2.ContainerID(), "Container should be reused")
	assert.Equal(t, containersession.StateRunning, session2.State())

	// Cleanup
	err = manager2.StopContainerSession(ctx, session2.ID())
	require.NoError(t, err)
}

// TestIntegration_ContainerReuse_Stopped tests restarting a stopped container
func TestIntegration_ContainerReuse_Stopped(t *testing.T) {
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	ctx := context.Background()
	baseWorkspace := filepath.Join(os.TempDir(), "containersession-test", t.Name())
	defer os.RemoveAll(baseWorkspace)

	sessionID := uuid.New().String()
	defer cleanupContainers(t, dockerClient, []string{sessionID})

	// Manager 1: Create, start, and stop
	manager1 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	session1, err := manager1.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "60"})
	require.NoError(t, err)
	err = manager1.StartContainerSession(ctx, session1.ID())
	require.NoError(t, err)
	err = manager1.StopContainerSession(ctx, session1.ID())
	require.NoError(t, err)

	originalContainerID := session1.ContainerID()

	// Manager 2: Reuse stopped container
	manager2 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	session2, err := manager2.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "60"})
	require.NoError(t, err)

	// Should restart the same container
	assert.Equal(t, originalContainerID, session2.ContainerID(), "Container should be reused and restarted")
	assert.Equal(t, containersession.StateRunning, session2.State())

	// Cleanup
	err = manager2.StopContainerSession(ctx, session2.ID())
	require.NoError(t, err)
}

// TestIntegration_CrossProcessAttach tests explicit attachment from different manager
func TestIntegration_CrossProcessAttach(t *testing.T) {
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	ctx := context.Background()
	baseWorkspace := filepath.Join(os.TempDir(), "containersession-test", t.Name())
	defer os.RemoveAll(baseWorkspace)

	sessionID := uuid.New().String()
	defer cleanupContainers(t, dockerClient, []string{sessionID})

	// Manager A: Create and start
	managerA := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	sessionA, err := managerA.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "60"})
	require.NoError(t, err)
	err = managerA.StartContainerSession(ctx, sessionA.ID())
	require.NoError(t, err)

	// Manager B: Attach to existing session
	managerB := containersession.NewManager(
		dockerClient,
		&testIDGenerator{}, // Different ID generator
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	sessionB, err := managerB.AttachContainerSession(ctx, sessionA.ID())
	require.NoError(t, err)

	// Should attach to the same container
	assert.Equal(t, sessionA.ContainerID(), sessionB.ContainerID())
	assert.Equal(t, sessionA.ID(), sessionB.ID())
	assert.Equal(t, containersession.StateRunning, sessionB.State())

	// Cleanup
	err = managerB.StopContainerSession(ctx, sessionB.ID())
	require.NoError(t, err)
}

// TestIntegration_ConcurrentSessions tests running multiple sessions simultaneously
func TestIntegration_ConcurrentSessions(t *testing.T) {
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	ctx := context.Background()
	baseWorkspace := filepath.Join(os.TempDir(), "containersession-test", t.Name())
	defer os.RemoveAll(baseWorkspace)

	manager := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	const numSessions = 3
	sessions := make([]*containersession.ContainerSession, numSessions)
	sessionIDs := make([]string, numSessions)
	
	// Use errgroup to properly handle errors from goroutines
	g, gctx := errgroup.WithContext(ctx)

	// Create and start sessions concurrently
	for i := 0; i < numSessions; i++ {
		index := i // Capture loop variable
		g.Go(func() error {
			session, err := manager.CreateContainerSession(gctx, "ubuntu:latest", []string{"sleep", "30"})
			if err != nil {
				return fmt.Errorf("failed to create session %d: %w", index, err)
			}

			err = manager.StartContainerSession(gctx, session.ID())
			if err != nil {
				return fmt.Errorf("failed to start session %d: %w", index, err)
			}

			sessions[index] = session
			sessionIDs[index] = session.ID()
			t.Logf("Session %d started: %s", index, session.ID())
			return nil
		})
	}

	// Wait for all goroutines and check for errors
	require.NoError(t, g.Wait(), "All sessions should be created and started successfully")
	defer cleanupContainers(t, dockerClient, sessionIDs)

	// Verify all sessions are running
	for i, session := range sessions {
		assert.NotNil(t, session, "Session %d should exist", i)
		assert.Equal(t, containersession.StateRunning, session.State(), "Session %d should be running", i)
	}

	// Stop all sessions
	for i, session := range sessions {
		err := manager.StopContainerSession(ctx, session.ID())
		assert.NoError(t, err, "Session %d should stop cleanly", i)
	}
}

// TestIntegration_WorkspacePersistence tests data persistence across container lifecycle
func TestIntegration_WorkspacePersistence(t *testing.T) {
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	ctx := context.Background()
	baseWorkspace := filepath.Join(os.TempDir(), "containersession-test", t.Name())
	defer os.RemoveAll(baseWorkspace)

	sessionID := uuid.New().String()
	defer cleanupContainers(t, dockerClient, []string{sessionID})

	manager := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	// Session 1: Create, start, write file, stop
	session1, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "30"})
	require.NoError(t, err)
	err = manager.StartContainerSession(ctx, session1.ID())
	require.NoError(t, err)

	testFile := filepath.Join(session1.WorkspacePath(), "test-data.txt")
	testContent := "persistent data test"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	err = manager.StopContainerSession(ctx, session1.ID())
	require.NoError(t, err)

	// Session 2: Reuse container and verify file exists
	manager2 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	session2, err := manager2.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "30"})
	require.NoError(t, err)

	// Verify workspace path is the same
	assert.Equal(t, session1.WorkspacePath(), session2.WorkspacePath())

	// Verify file still exists with correct content
	content, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content), "Workspace data should persist")

	// Cleanup
	err = manager2.StopContainerSession(ctx, session2.ID())
	require.NoError(t, err)
}

// TestIntegration_InvalidWorkspacePath tests security validation of workspace paths
func TestIntegration_InvalidWorkspacePath(t *testing.T) {
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	ctx := context.Background()
	baseWorkspace := filepath.Join(os.TempDir(), "containersession-test", t.Name())
	defer os.RemoveAll(baseWorkspace)

	sessionID := uuid.New().String()
	defer cleanupContainers(t, dockerClient, []string{sessionID})

	manager := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&testClock{},
		&testLogger{t: t},
		baseWorkspace,
	)

	// Create and start session normally
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "30"})
	require.NoError(t, err)
	err = manager.StartContainerSession(ctx, session.ID())
	require.NoError(t, err)

	// Verify workspace path is under base directory
	assert.Contains(t, session.WorkspacePath(), baseWorkspace,
		"Workspace path should be under base directory")

	// Verify no directory traversal
	assert.NotContains(t, session.WorkspacePath(), "..",
		"Workspace path should not contain directory traversal")

	// Cleanup
	err = manager.StopContainerSession(ctx, session.ID())
	require.NoError(t, err)
}
