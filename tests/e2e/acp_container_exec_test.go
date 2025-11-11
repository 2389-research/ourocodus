//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
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
					t.Log("Using Colima Docker")
					return dockerClient
				}
			}
		}
	}

	// Fall back to default Docker (Desktop)
	_ = os.Unsetenv("DOCKER_HOST")
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "Failed to create Docker client")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = dockerClient.Ping(ctx)
	require.NoError(t, err, "Docker daemon is not accessible")

	t.Log("Using Docker Desktop")
	return dockerClient
}

// TestContainerExecProcessLauncher_SmokeTest verifies that ACP can run commands inside containers
func TestContainerExecProcessLauncher_SmokeTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Setup Docker client
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	// Create test workspace
	workspaceDir := t.TempDir()
	testFile := filepath.Join(workspaceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello from host\n"), 0644))

	// Create container session manager
	containerManager := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		workspaceDir,
	)

	// Pull test image (alpine is small and fast)
	imageName := "alpine:latest"
	t.Logf("Pulling image: %s", imageName)
	reader, err := dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	require.NoError(t, err)
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader) // Wait for pull to complete
	require.NoError(t, err)

	// Create and start test container
	containerID := fmt.Sprintf("acp-test-%s", uuid.New().String()[:8])
	t.Logf("Creating container: %s", containerID)

	createResp, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: imageName,
			Cmd:   []string{"sleep", "120"}, // Keep container alive
		},
		&container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:/workspace", workspaceDir)},
		},
		nil,
		nil,
		containerID,
	)
	require.NoError(t, err)
	actualContainerID := createResp.ID

	// Ensure cleanup
	defer func() {
		t.Logf("Cleaning up container: %s", containerID)
		_ = dockerClient.ContainerRemove(ctx, actualContainerID, container.RemoveOptions{Force: true})
	}()

	// Start container
	require.NoError(t, dockerClient.ContainerStart(ctx, actualContainerID, container.StartOptions{}))

	// Wait for container to be ready with proper readiness check
	{
		ctxReady, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		for {
			inspect, err := dockerClient.ContainerInspect(ctxReady, actualContainerID)
			require.NoError(t, err)
			if inspect.State.Running {
				break
			}
			select {
			case <-ctxReady.Done():
				t.Fatal("Timeout waiting for container to be ready")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}

	// Create ContainerExecProcessLauncher
	launcher := session.NewContainerExecProcessLauncher(containerManager, actualContainerID).
		WithWorkspacePath("/workspace")

	t.Log("Testing command execution inside container via ProcessLauncher")

	// Test 1: Simple command execution
	launchCfg := acp.ProcessLaunchConfig{
		Workspace:   workspaceDir,
		CommandPath: "/bin/sh",
		CommandArgs: []string{"-c", "echo 'hello from container' > /workspace/output.txt"},
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	transport, err := launcher.Start(ctx, launchCfg)
	require.NoError(t, err, "Failed to start process in container")
	require.NotNil(t, transport)

	// Close transport
	require.NoError(t, transport.Close())

	// Verify the command executed successfully by checking the output file
	outputFile := filepath.Join(workspaceDir, "output.txt")
	require.FileExists(t, outputFile)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	require.Equal(t, "hello from container\n", string(content))

	t.Log("✓ Container exec smoke test passed")
}

// TestContainerExecProcessLauncher_WithEchoAgent verifies echo-agent can run inside container
func TestContainerExecProcessLauncher_WithEchoAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if echo-agent binary exists
	echoAgentPath := os.Getenv("OUROCODUS_ACP_BINARY")
	if echoAgentPath == "" {
		echoAgentPath = "./bin/echo-agent"
	}
	if _, err := os.Stat(echoAgentPath); os.IsNotExist(err) {
		t.Skipf("echo-agent not found at %s, run 'make build' first", echoAgentPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Setup Docker client
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	// Create test workspace and copy echo-agent
	workspaceDir := t.TempDir()
	containerEchoAgent := filepath.Join(workspaceDir, "echo-agent")

	// Copy echo-agent to workspace
	src, err := os.ReadFile(echoAgentPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(containerEchoAgent, src, 0755))

	// Create container session manager
	containerManager := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		workspaceDir,
	)

	// Pull test image
	imageName := "alpine:latest"
	t.Logf("Pulling image: %s", imageName)
	reader, err := dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	require.NoError(t, err)
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	require.NoError(t, err)

	// Create and start container
	containerID := fmt.Sprintf("acp-echo-test-%s", uuid.New().String()[:8])
	t.Logf("Creating container: %s", containerID)

	createResp, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: imageName,
			Cmd:   []string{"sleep", "120"},
		},
		&container.HostConfig{
			Binds: []string{fmt.Sprintf("%s:/workspace", workspaceDir)},
		},
		nil,
		nil,
		containerID,
	)
	require.NoError(t, err)
	actualContainerID := createResp.ID

	defer func() {
		t.Logf("Cleaning up container: %s", containerID)
		_ = dockerClient.ContainerRemove(ctx, actualContainerID, container.RemoveOptions{Force: true})
	}()

	require.NoError(t, dockerClient.ContainerStart(ctx, actualContainerID, container.StartOptions{}))

	// Wait for container to be ready with proper readiness check
	{
		ctxReady, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		for {
			inspect, err := dockerClient.ContainerInspect(ctxReady, actualContainerID)
			require.NoError(t, err)
			if inspect.State.Running {
				break
			}
			select {
			case <-ctxReady.Done():
				t.Fatal("Timeout waiting for container to be ready")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}

	// Create launcher with echo-agent
	launcher := session.NewContainerExecProcessLauncher(containerManager, actualContainerID).
		WithWorkspacePath("/workspace")

	t.Log("Testing echo-agent execution inside container")

	launchCfg := acp.ProcessLaunchConfig{
		Workspace:   workspaceDir,
		CommandPath: "/workspace/echo-agent",
		CommandArgs: []string{},
		APIKey:      "test-api-key",
		Env:         map[string]string{},
	}

	transport, err := launcher.Start(ctx, launchCfg)
	require.NoError(t, err, "Failed to start echo-agent in container")
	require.NotNil(t, transport)
	defer transport.Close()

	// The echo-agent should start successfully
	// (We don't test full protocol interaction here, just that it starts)
	t.Log("✓ Echo-agent container exec smoke test passed")
}

type testLogger struct {
	t *testing.T
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}
