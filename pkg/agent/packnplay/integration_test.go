//go:build integration

package packnplay

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// TestMain ensures Docker is available
func TestMain(m *testing.M) {
	ctx := context.Background()
	// Try to create a launcher to verify Docker is available
	l, err := NewLauncher(WithProjectPath("."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Docker not available: %v\n", err)
		fmt.Fprintf(os.Stderr, "Skipping integration tests\n")
		os.Exit(0)
	}
	_ = l.Close()

	// Run tests
	code := m.Run()

	// Cleanup any orphaned containers
	cleanupOrphanedContainers(ctx)

	os.Exit(code)
}

// cleanupOrphanedContainers removes any Packnplay containers left behind
func cleanupOrphanedContainers(ctx context.Context) {
	l, err := NewLauncher(WithProjectPath("."))
	if err != nil {
		return
	}
	defer l.Close()

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "managed-by=packnplay")

	containers, err := l.dockerClient.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return
	}

	for _, c := range containers {
		_ = l.dockerClient.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
	}
}

func TestIntegration_SpawnAndStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	l, err := NewLauncher(
		WithProjectPath("."),
		WithDefaultImage("busybox:latest"),
		WithVerbose(true),
	)
	if err != nil {
		t.Fatalf("failed to create launcher: %v", err)
	}
	defer l.Close()

	// Register cleanup
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		cleanupOrphanedContainers(cleanupCtx)
	})

	// Spawn a simple agent
	cfg := &agent.SpawnConfig{
		Role:    "test",
		Image:   "busybox:latest",
		Command: []string{"sleep", "30"},
	}

	handle, err := l.Spawn(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}

	// Verify handle fields
	if handle.ID() == "" {
		t.Error("expected non-empty agent ID")
	}
	if handle.ContainerID() == "" {
		t.Error("expected non-empty container ID")
	}
	if handle.Workspace() == "" {
		t.Error("expected non-empty workspace")
	}

	t.Logf("Spawned agent: ID=%s, Container=%s, Workspace=%s", handle.ID(), handle.ContainerID()[:12], handle.Workspace())

	// Stop the agent
	if err := l.Stop(ctx, handle); err != nil {
		t.Fatalf("failed to stop agent: %v", err)
	}

	t.Log("Agent stopped successfully")
}

func TestIntegration_BasicIO(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	l, err := NewLauncher(
		WithProjectPath("."),
		WithDefaultImage("busybox:latest"),
	)
	if err != nil {
		t.Fatalf("failed to create launcher: %v", err)
	}
	defer l.Close()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		cleanupOrphanedContainers(cleanupCtx)
	})

	// Spawn agent with echo command
	cfg := &agent.SpawnConfig{
		Role:    "echo-test",
		Image:   "busybox:latest",
		Command: []string{"sh", "-c", "echo 'Hello from container'; sleep 5"},
	}

	handle, err := l.Spawn(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}
	defer l.Stop(ctx, handle)

	// Read from stdout with timeout
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()

	stdoutBuf := make([]byte, 1024)
	readDone := make(chan struct{})
	var readErr error
	var n int

	go func() {
		n, readErr = handle.Stdout().Read(stdoutBuf)
		close(readDone)
	}()

	select {
	case <-readDone:
		if readErr != nil && readErr != io.EOF {
			t.Fatalf("failed to read stdout: %v", readErr)
		}
		output := string(stdoutBuf[:n])
		t.Logf("Received output: %q", output)
		if len(output) == 0 {
			t.Error("expected non-empty output")
		}
	case <-readCtx.Done():
		t.Fatal("timeout waiting for output")
	}
}

func TestIntegration_LiveStdinStdout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	l, err := NewLauncher(
		WithProjectPath("."),
		WithDefaultImage("busybox:latest"),
	)
	if err != nil {
		t.Fatalf("failed to create launcher: %v", err)
	}
	defer l.Close()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		cleanupOrphanedContainers(cleanupCtx)
	})

	// Spawn agent with cat (echoes stdin to stdout)
	cfg := &agent.SpawnConfig{
		Role:    "cat-test",
		Image:   "busybox:latest",
		Command: []string{"sh", "-c", "while read line; do echo \"GOT: $line\"; done"},
	}

	handle, err := l.Spawn(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}
	defer l.Stop(ctx, handle)

	// Write to stdin
	testInput := "test message\n"
	if _, err := handle.Stdin().Write([]byte(testInput)); err != nil {
		t.Fatalf("failed to write to stdin: %v", err)
	}

	// Read response from stdout
	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()

	output := make([]byte, 1024)
	readDone := make(chan int)
	var readErr error

	go func() {
		n, err := handle.Stdout().Read(output)
		readErr = err
		readDone <- n
	}()

	select {
	case n := <-readDone:
		if readErr != nil && readErr != io.EOF {
			t.Fatalf("failed to read stdout: %v", readErr)
		}
		got := string(output[:n])
		t.Logf("Received: %q", got)
		// Should contain our echoed input
		if len(got) == 0 {
			t.Error("expected non-empty response")
		}
	case <-readCtx.Done():
		t.Fatal("timeout waiting for response")
	}
}

func TestIntegration_AttachToExisting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	l, err := NewLauncher(
		WithProjectPath("."),
		WithDefaultImage("busybox:latest"),
	)
	if err != nil {
		t.Fatalf("failed to create launcher: %v", err)
	}
	defer l.Close()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		cleanupOrphanedContainers(cleanupCtx)
	})

	// Spawn an agent
	cfg := &agent.SpawnConfig{
		Role:    "attach-test",
		Image:   "busybox:latest",
		Command: []string{"sleep", "60"},
	}

	handle1, err := l.Spawn(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}
	defer l.Stop(ctx, handle1)

	agentID := handle1.ID()
	containerID := handle1.ContainerID()

	t.Logf("Spawned agent %s (container %s)", agentID, containerID[:12])

	// Create a new launcher instance (simulating process restart)
	l2, err := NewLauncher(
		WithProjectPath("."),
		WithDefaultImage("busybox:latest"),
	)
	if err != nil {
		t.Fatalf("failed to create second launcher: %v", err)
	}
	defer l2.Close()

	// Attach to the existing agent
	handle2, err := l2.Attach(ctx, agentID)
	if err != nil {
		t.Fatalf("failed to attach to agent: %v", err)
	}

	// Verify attached handle
	if handle2.ID() != agentID {
		t.Errorf("expected agent ID %s, got %s", agentID, handle2.ID())
	}
	if handle2.ContainerID() != containerID {
		t.Errorf("expected container ID %s, got %s", containerID, handle2.ContainerID())
	}

	t.Log("Successfully attached to existing agent")
}

func TestIntegration_FindRunningAgents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	l, err := NewLauncher(
		WithProjectPath("."),
		WithDefaultImage("busybox:latest"),
	)
	if err != nil {
		t.Fatalf("failed to create launcher: %v", err)
	}
	defer l.Close()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		cleanupOrphanedContainers(cleanupCtx)
	})

	// Initially, should find no agents
	agents, err := l.FindRunningAgents(ctx)
	if err != nil {
		t.Fatalf("failed to find running agents: %v", err)
	}
	initialCount := len(agents)

	// Spawn two agents
	cfg := &agent.SpawnConfig{
		Role:    "find-test",
		Image:   "busybox:latest",
		Command: []string{"sleep", "30"},
	}

	handle1, err := l.Spawn(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to spawn agent 1: %v", err)
	}
	defer l.Stop(ctx, handle1)

	handle2, err := l.Spawn(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to spawn agent 2: %v", err)
	}
	defer l.Stop(ctx, handle2)

	// Find running agents
	agents, err = l.FindRunningAgents(ctx)
	if err != nil {
		t.Fatalf("failed to find running agents: %v", err)
	}

	// Should find 2 more than initially
	if len(agents) != initialCount+2 {
		t.Errorf("expected %d agents, found %d", initialCount+2, len(agents))
	}

	// Verify our agent IDs are in the list
	found := make(map[string]bool)
	for _, id := range agents {
		found[id] = true
	}

	if !found[handle1.ID()] {
		t.Errorf("agent 1 (%s) not found in running agents", handle1.ID())
	}
	if !found[handle2.ID()] {
		t.Errorf("agent 2 (%s) not found in running agents", handle2.ID())
	}

	t.Logf("Successfully found %d running agents", len(agents))
}

func TestIntegration_WaitForExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	l, err := NewLauncher(
		WithProjectPath("."),
		WithDefaultImage("busybox:latest"),
	)
	if err != nil {
		t.Fatalf("failed to create launcher: %v", err)
	}
	defer l.Close()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		cleanupOrphanedContainers(cleanupCtx)
	})

	// Spawn agent that exits quickly with success
	cfg := &agent.SpawnConfig{
		Role:    "wait-test",
		Image:   "busybox:latest",
		Command: []string{"sh", "-c", "echo 'done'; exit 0"},
	}

	handle, err := l.Spawn(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to spawn agent: %v", err)
	}
	defer l.Stop(ctx, handle)

	// Wait for exit
	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer waitCancel()

	if err := handle.Wait(waitCtx); err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	t.Log("Agent exited successfully")
}
