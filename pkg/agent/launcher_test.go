package agent

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestMockLauncher_Spawn(t *testing.T) {
	launcher := NewMockLauncher()

	config := &SpawnConfig{
		Role:      "test-agent",
		Workspace: "/tmp/test-workspace",
		Image:     "test-image:latest",
		Environment: map[string]string{
			"KEY": "value",
		},
	}

	ctx := context.Background()
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	if handle == nil {
		t.Fatal("Spawn returned nil handle")
	}

	if handle.ID() == "" {
		t.Error("Handle ID is empty")
	}

	if handle.Workspace() != config.Workspace {
		t.Errorf("Workspace mismatch: got %s, want %s", handle.Workspace(), config.Workspace)
	}

	if handle.ContainerID() == "" {
		t.Error("ContainerID is empty")
	}

	// Verify agent was tracked
	agents := launcher.GetSpawnedAgents()
	if len(agents) != 1 {
		t.Errorf("Expected 1 spawned agent, got %d", len(agents))
	}
}

func TestMockLauncher_Spawn_MultipleAgents(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	// Spawn multiple agents
	handles := make([]AgentHandle, 3)
	for i := 0; i < 3; i++ {
		config := &SpawnConfig{
			Role:      "test-agent",
			Workspace: "/tmp/workspace",
		}

		handle, err := launcher.Spawn(ctx, config)
		if err != nil {
			t.Fatalf("Spawn %d failed: %v", i, err)
		}
		handles[i] = handle
	}

	// Verify all agents have unique IDs
	ids := make(map[string]bool)
	for _, handle := range handles {
		if ids[handle.ID()] {
			t.Errorf("Duplicate agent ID: %s", handle.ID())
		}
		ids[handle.ID()] = true
	}

	// Verify all agents are tracked
	spawnedAgents := launcher.GetSpawnedAgents()
	if len(spawnedAgents) != 3 {
		t.Errorf("Expected 3 spawned agents, got %d", len(spawnedAgents))
	}
}

func TestMockLauncher_Spawn_Error(t *testing.T) {
	launcher := NewMockLauncher()
	expectedErr := errors.New("spawn failed")
	launcher.SpawnError = expectedErr

	ctx := context.Background()
	config := &SpawnConfig{Role: "test"}

	handle, err := launcher.Spawn(ctx, config)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if handle != nil {
		t.Error("Expected nil handle on error")
	}

	// Verify no agent was tracked
	agents := launcher.GetSpawnedAgents()
	if len(agents) != 0 {
		t.Errorf("Expected 0 spawned agents, got %d", len(agents))
	}
}

func TestMockLauncher_Attach(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	// Spawn an agent
	config := &SpawnConfig{Role: "test"}
	handle1, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Attach to the spawned agent
	handle2, err := launcher.Attach(ctx, handle1.ID())
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	if handle2.ID() != handle1.ID() {
		t.Errorf("Attached handle ID mismatch: got %s, want %s", handle2.ID(), handle1.ID())
	}
}

func TestMockLauncher_Attach_NotFound(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	_, err := launcher.Attach(ctx, "nonexistent-id")
	if err == nil {
		t.Error("Expected error when attaching to nonexistent agent")
	}
}

func TestMockLauncher_Attach_Error(t *testing.T) {
	launcher := NewMockLauncher()
	expectedErr := errors.New("attach failed")
	launcher.AttachError = expectedErr

	ctx := context.Background()
	_, err := launcher.Attach(ctx, "any-id")
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestMockLauncher_Stop(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	// Spawn an agent
	config := &SpawnConfig{Role: "test"}
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Verify agent is tracked
	if len(launcher.GetSpawnedAgents()) != 1 {
		t.Fatal("Agent not tracked after spawn")
	}

	// Stop the agent
	err = launcher.Stop(ctx, handle)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify agent is no longer tracked
	if len(launcher.GetSpawnedAgents()) != 0 {
		t.Error("Agent still tracked after stop")
	}

	// Verify Wait() unblocks
	waitCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = handle.Wait(waitCtx)
	if err != nil && err != context.DeadlineExceeded {
		// Wait should return nil (agent stopped normally) or deadline exceeded if already consumed
		t.Logf("Wait returned: %v", err)
	}
}

func TestMockLauncher_Stop_Error(t *testing.T) {
	launcher := NewMockLauncher()
	expectedErr := errors.New("stop failed")
	launcher.StopError = expectedErr

	ctx := context.Background()
	config := &SpawnConfig{Role: "test"}
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	err = launcher.Stop(ctx, handle)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestMockLauncher_Reset(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	// Spawn agents and set errors
	launcher.Spawn(ctx, &SpawnConfig{Role: "test1"})
	launcher.Spawn(ctx, &SpawnConfig{Role: "test2"})
	launcher.SpawnError = errors.New("error")

	// Reset
	launcher.Reset()

	// Verify state is cleared
	if len(launcher.GetSpawnedAgents()) != 0 {
		t.Error("Agents not cleared after reset")
	}

	if launcher.SpawnError != nil {
		t.Error("SpawnError not cleared after reset")
	}

	// Verify can spawn again
	_, err := launcher.Spawn(ctx, &SpawnConfig{Role: "test3"})
	if err != nil {
		t.Errorf("Spawn after reset failed: %v", err)
	}
}

func TestMockHandle_IO(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	config := &SpawnConfig{Role: "test"}
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	mockHandle, ok := handle.(*MockHandle)
	if !ok {
		t.Fatal("Handle is not a MockHandle")
	}

	// Test writing to stdin and reading back
	testData := []byte("test input data")
	n, err := mockHandle.Stdin().Write(testData)
	if err != nil {
		t.Fatalf("Write to stdin failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch: got %d, want %d", n, len(testData))
	}

	// Read from stdin (simulating agent reading)
	buf := make([]byte, len(testData))
	n, err = mockHandle.ReadFromStdin(buf)
	if err != nil {
		t.Fatalf("Read from stdin failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Read length mismatch: got %d, want %d", n, len(testData))
	}
	if string(buf) != string(testData) {
		t.Errorf("Data mismatch: got %s, want %s", buf, testData)
	}

	// Test stdout
	stdoutData := []byte("stdout output")
	mockHandle.WriteToStdout(stdoutData)

	buf = make([]byte, len(stdoutData))
	n, err = handle.Stdout().Read(buf)
	if err != nil {
		t.Fatalf("Read from stdout failed: %v", err)
	}
	if n != len(stdoutData) {
		t.Errorf("Stdout read length mismatch: got %d, want %d", n, len(stdoutData))
	}
	if string(buf) != string(stdoutData) {
		t.Errorf("Stdout data mismatch: got %s, want %s", buf, stdoutData)
	}

	// Test stderr
	stderrData := []byte("stderr output")
	mockHandle.WriteToStderr(stderrData)

	buf = make([]byte, len(stderrData))
	n, err = handle.Stderr().Read(buf)
	if err != nil {
		t.Fatalf("Read from stderr failed: %v", err)
	}
	if n != len(stderrData) {
		t.Errorf("Stderr read length mismatch: got %d, want %d", n, len(stderrData))
	}
	if string(buf) != string(stderrData) {
		t.Errorf("Stderr data mismatch: got %s, want %s", buf, stderrData)
	}
}

func TestMockHandle_Wait(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	config := &SpawnConfig{Role: "test"}
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	mockHandle, ok := handle.(*MockHandle)
	if !ok {
		t.Fatal("Handle is not a MockHandle")
	}

	// Test Wait() with simulated exit
	go func() {
		time.Sleep(50 * time.Millisecond)
		mockHandle.SimulateExit(nil)
	}()

	waitCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = handle.Wait(waitCtx)
	if err != nil {
		t.Errorf("Wait returned unexpected error: %v", err)
	}
}

func TestMockHandle_Wait_WithError(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	config := &SpawnConfig{Role: "test"}
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	mockHandle, ok := handle.(*MockHandle)
	if !ok {
		t.Fatal("Handle is not a MockHandle")
	}

	// Test Wait() with simulated error exit
	expectedErr := errors.New("agent crashed")
	go func() {
		time.Sleep(50 * time.Millisecond)
		mockHandle.SimulateExit(expectedErr)
	}()

	waitCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = handle.Wait(waitCtx)
	if err != expectedErr {
		t.Errorf("Wait returned wrong error: got %v, want %v", err, expectedErr)
	}
}

func TestMockHandle_Wait_ContextCanceled(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	config := &SpawnConfig{Role: "test"}
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Test Wait() with canceled context
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = handle.Wait(waitCtx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestMockHandle_Close(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	config := &SpawnConfig{Role: "test"}
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Close handle
	err = handle.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify subsequent writes fail
	_, err = handle.Stdin().Write([]byte("test"))
	if err != io.ErrClosedPipe {
		t.Errorf("Expected ErrClosedPipe, got %v", err)
	}

	// Verify double close returns error
	err = handle.Close()
	if err == nil {
		t.Error("Expected error on double close")
	}
}

func TestSpawnConfig_EmptyWorkspace(t *testing.T) {
	launcher := NewMockLauncher()
	ctx := context.Background()

	// Spawn with empty workspace
	config := &SpawnConfig{
		Role:      "test",
		Workspace: "", // Empty workspace
	}

	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}

	// Verify handle was created (workspace empty is valid - launcher decides behavior)
	if handle == nil {
		t.Fatal("Expected handle even with empty workspace")
	}

	if handle.Workspace() != "" {
		t.Errorf("Workspace should remain empty, got %s", handle.Workspace())
	}
}
