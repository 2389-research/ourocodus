package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
)

// Integration tests that exercise full command flows
// These require Docker and git to be available

func TestIntegration_SpawnListStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Ensure DOCKER_HOST is set for Colima
	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx := context.Background()
	agentID := "test-integration"

	// Cleanup any previous test runs
	_ = stopAgent(ctx, nil, agentID)
	time.Sleep(1 * time.Second)

	// Test spawn
	t.Run("spawn", func(t *testing.T) {
		launcher, err := createLauncher()
		if err != nil {
			t.Fatalf("Failed to create launcher: %v", err)
		}

		config, err := buildSpawnConfig(agentID)
		if err != nil {
			t.Fatalf("Failed to build spawn config: %v", err)
		}

		handle, err := launcher.Spawn(ctx, config)
		if err != nil {
			t.Fatalf("Spawn failed: %v", err)
		}

		// Verify handle
		if handle.AgentID() != agentID {
			t.Errorf("Handle AgentID = %q, want %q", handle.AgentID(), agentID)
		}

		if handle.ContainerID() == "" {
			t.Error("Handle ContainerID is empty")
		}

		if handle.WorkspacePath() == "" {
			t.Error("Handle WorkspacePath is empty")
		}
	})

	// Small delay for container to fully start
	time.Sleep(2 * time.Second)

	// Test list
	t.Run("list", func(t *testing.T) {
		agents, err := listAgentsFromDocker(ctx)
		if err != nil {
			t.Fatalf("listAgentsFromDocker failed: %v", err)
		}

		// Find our test agent
		found := false
		for _, agent := range agents {
			if agent.AgentID == agentID {
				found = true
				if agent.Status != "running" {
					t.Errorf("Agent status = %q, want running", agent.Status)
				}
				if agent.ContainerID == "" {
					t.Error("Agent ContainerID is empty")
				}
				if agent.Workspace == "" {
					t.Error("Agent Workspace is empty")
				}
				break
			}
		}

		if !found {
			t.Errorf("Agent %q not found in list", agentID)
		}
	})

	// Test that worktree exists
	t.Run("worktree exists", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "git", "worktree", "list")
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("git worktree list failed: %v", err)
		}

		if !strings.Contains(string(output), agentID) {
			t.Errorf("Worktree for agent %q not found in git worktree list", agentID)
		}
	})

	// Test stop
	t.Run("stop", func(t *testing.T) {
		err := stopAgent(ctx, nil, agentID)
		if err != nil {
			t.Fatalf("stopAgent failed: %v", err)
		}

		// Give Docker a moment to fully remove the container
		time.Sleep(500 * time.Millisecond)

		// Verify container is gone by checking if we can inspect it
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			t.Fatalf("Failed to create Docker client: %v", err)
		}
		defer cli.Close()

		// Try to inspect the container - should fail
		agents, err := listAgentsFromDocker(ctx)
		if err != nil {
			t.Fatalf("listAgentsFromDocker failed: %v", err)
		}

		// Agent should not be in the list
		for _, agent := range agents {
			if agent.AgentID == agentID {
				t.Errorf("Agent %q still in list after stop", agentID)
			}
		}

		// Verify worktree is gone
		cmd := exec.CommandContext(ctx, "git", "worktree", "list")
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("git worktree list failed: %v", err)
		}

		if strings.Contains(string(output), agentID) {
			t.Errorf("Worktree for agent %q still exists after stop", agentID)
		}

		// Verify branch is gone
		cmd = exec.CommandContext(ctx, "git", "branch")
		output, err = cmd.Output()
		if err != nil {
			t.Fatalf("git branch failed: %v", err)
		}

		if strings.Contains(string(output), "agent-"+agentID) {
			t.Errorf("Branch for agent %q still exists after stop", agentID)
		}
	})
}

func TestIntegration_StopIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx := context.Background()
	agentID := "test-idempotent"

	// Call stop on non-existent agent - should succeed
	err := stopAgent(ctx, nil, agentID)
	if err != nil {
		t.Errorf("stopAgent on non-existent agent failed: %v", err)
	}

	// Call stop again - should still succeed (idempotent)
	err = stopAgent(ctx, nil, agentID)
	if err != nil {
		t.Errorf("stopAgent second call failed: %v", err)
	}
}

func TestIntegration_MultipleAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx := context.Background()
	agent1ID := "test-multi-1"
	agent2ID := "test-multi-2"

	// Cleanup
	defer func() {
		stopAgent(ctx, nil, agent1ID)
		stopAgent(ctx, nil, agent2ID)
	}()

	// Spawn two agents
	launcher, err := createLauncher()
	if err != nil {
		t.Fatalf("Failed to create launcher: %v", err)
	}

	config1, _ := buildSpawnConfig(agent1ID)
	config2, _ := buildSpawnConfig(agent2ID)

	handle1, err := launcher.Spawn(ctx, config1)
	if err != nil {
		t.Fatalf("Spawn agent1 failed: %v", err)
	}

	handle2, err := launcher.Spawn(ctx, config2)
	if err != nil {
		t.Fatalf("Spawn agent2 failed: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Verify both are listed
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		t.Fatalf("listAgentsFromDocker failed: %v", err)
	}

	found1, found2 := false, false
	for _, agent := range agents {
		if agent.AgentID == agent1ID {
			found1 = true
		}
		if agent.AgentID == agent2ID {
			found2 = true
		}
	}

	if !found1 {
		t.Errorf("Agent1 %q not found in list", agent1ID)
	}
	if !found2 {
		t.Errorf("Agent2 %q not found in list", agent2ID)
	}

	// Verify they have different container IDs
	if handle1.ContainerID() == handle2.ContainerID() {
		t.Error("Agents have same container ID")
	}

	// Verify they have different workspace paths
	if handle1.WorkspacePath() == handle2.WorkspacePath() {
		t.Error("Agents have same workspace path")
	}

	// Stop agent1, verify agent2 still running
	err = stopAgent(ctx, nil, agent1ID)
	if err != nil {
		t.Fatalf("stopAgent agent1 failed: %v", err)
	}

	time.Sleep(1 * time.Second)

	agents, err = listAgentsFromDocker(ctx)
	if err != nil {
		t.Fatalf("listAgentsFromDocker failed: %v", err)
	}

	found1, found2 = false, false
	for _, agent := range agents {
		if agent.AgentID == agent1ID {
			found1 = true
		}
		if agent.AgentID == agent2ID {
			found2 = true
		}
	}

	if found1 {
		t.Error("Agent1 still in list after stop")
	}
	if !found2 {
		t.Error("Agent2 not in list after stopping agent1")
	}
}
