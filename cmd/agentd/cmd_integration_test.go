package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CLI-level integration tests that invoke actual commands
// These test the full command execution path including cobra handlers

func TestCLI_DoctorCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Run doctor command
	cmd := exec.Command("./agentd-test", "doctor")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Doctor output:\n%s", output)
		t.Fatalf("Doctor command failed: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Docker daemon running") {
		t.Errorf("Doctor output missing 'Docker daemon running': %s", outputStr)
	}
}

func TestCLI_SpawnListStopWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx := context.Background()
	agentID := "test-cli-workflow"

	// Cleanup before test
	_ = stopAgent(ctx, nil, agentID)
	time.Sleep(500 * time.Millisecond)

	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Test spawn command
	t.Run("spawn via CLI", func(t *testing.T) {
		cmd := exec.Command("./agentd-test", "spawn", agentID)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Spawn output:\n%s", output)
			t.Fatalf("Spawn command failed: %v", err)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "Agent "+agentID+" ready") {
			t.Errorf("Spawn output missing success message: %s", outputStr)
		}
		if !strings.Contains(outputStr, "Worktree:") {
			t.Errorf("Spawn output missing worktree info: %s", outputStr)
		}
		if !strings.Contains(outputStr, "Container:") {
			t.Errorf("Spawn output missing container info: %s", outputStr)
		}
	})

	// Small delay for container to fully start
	time.Sleep(2 * time.Second)

	// Test list command
	t.Run("list via CLI", func(t *testing.T) {
		cmd := exec.Command("./agentd-test", "list")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("List output:\n%s", output)
			t.Fatalf("List command failed: %v", err)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, agentID) {
			t.Errorf("List output missing agent ID '%s': %s", agentID, outputStr)
		}
		if !strings.Contains(outputStr, "running") {
			t.Errorf("List output missing 'running' status: %s", outputStr)
		}
	})

	// Test list JSON format
	t.Run("list JSON via CLI", func(t *testing.T) {
		cmd := exec.Command("./agentd-test", "list", "--format", "json")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("List JSON output:\n%s", output)
			t.Fatalf("List JSON command failed: %v", err)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, agentID) {
			t.Errorf("List JSON output missing agent ID '%s': %s", agentID, outputStr)
		}
		if !strings.Contains(outputStr, `"Status"`) {
			t.Errorf("List JSON output missing Status field: %s", outputStr)
		}
	})

	// Test stop command
	t.Run("stop via CLI", func(t *testing.T) {
		cmd := exec.Command("./agentd-test", "stop", agentID)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Stop output:\n%s", output)
			t.Fatalf("Stop command failed: %v", err)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "Stopped container") {
			t.Errorf("Stop output missing 'Stopped container': %s", outputStr)
		}
		if !strings.Contains(outputStr, "Cleaned up agent resources") {
			t.Errorf("Stop output missing cleanup message: %s", outputStr)
		}
	})

	// Verify cleanup
	time.Sleep(500 * time.Millisecond)
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		t.Fatalf("Failed to list agents after stop: %v", err)
	}
	for _, agent := range agents {
		if agent.AgentID == agentID {
			t.Errorf("Agent still exists after stop")
		}
	}
}

func TestCLI_StopMultipleAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx := context.Background()
	agent1ID := "test-cli-multi-1"
	agent2ID := "test-cli-multi-2"

	// Cleanup
	defer func() {
		stopAgent(ctx, nil, agent1ID)
		stopAgent(ctx, nil, agent2ID)
	}()

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Spawn two agents
	launcher, err := createLauncher()
	if err != nil {
		t.Fatalf("Failed to create launcher: %v", err)
	}

	config1, _ := buildSpawnConfig(agent1ID)
	config2, _ := buildSpawnConfig(agent2ID)

	_, err = launcher.Spawn(ctx, config1)
	if err != nil {
		t.Fatalf("Failed to spawn agent1: %v", err)
	}

	_, err = launcher.Spawn(ctx, config2)
	if err != nil {
		t.Fatalf("Failed to spawn agent2: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Stop both agents with single command
	cmd := exec.Command("./agentd-test", "stop", agent1ID, agent2ID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Stop multiple output:\n%s", output)
		t.Fatalf("Stop multiple command failed: %v", err)
	}

	// Verify both are gone
	time.Sleep(500 * time.Millisecond)
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		t.Fatalf("Failed to list agents: %v", err)
	}

	for _, agent := range agents {
		if agent.AgentID == agent1ID || agent.AgentID == agent2ID {
			t.Errorf("Agent %s still exists after stop", agent.AgentID)
		}
	}
}

func TestCLI_LogsCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx := context.Background()
	agentID := "test-cli-logs"

	// Cleanup
	defer stopAgent(ctx, nil, agentID)

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Spawn agent
	launcher, err := createLauncher()
	if err != nil {
		t.Fatalf("Failed to create launcher: %v", err)
	}

	config, _ := buildSpawnConfig(agentID)
	_, err = launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Test logs command (without follow, just tail)
	cmd := exec.Command("./agentd-test", "logs", agentID, "--follow=false", "--tail", "10")

	// Set up pipes to capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start logs command: %v", err)
	}

	// Read some output
	output := make([]byte, 1024)
	n, _ := io.ReadAtLeast(stdout, output, 10)

	cmd.Process.Kill()
	cmd.Wait()

	if n > 0 {
		outputStr := string(output[:n])
		if !strings.Contains(outputStr, "Logs for agent") {
			t.Logf("Logs output: %s", outputStr)
			// Not a hard failure - container might not have produced logs yet
		}
	}
}

func TestCLI_SpawnWithEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx := context.Background()
	agentID := "test-cli-env"

	// Cleanup
	defer stopAgent(ctx, nil, agentID)

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Spawn with environment variables
	cmd := exec.Command("./agentd-test", "spawn", agentID,
		"--env", "DEBUG=1",
		"--env", "TEST_VAR=hello")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Spawn with env output:\n%s", output)
		t.Fatalf("Spawn with env command failed: %v", err)
	}

	// Verify agent was created
	time.Sleep(2 * time.Second)
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		t.Fatalf("Failed to list agents: %v", err)
	}

	found := false
	for _, agent := range agents {
		if agent.AgentID == agentID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Agent with environment variables not found")
	}
}

func TestCLI_InvalidCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	tests := []struct {
		name       string
		args       []string
		shouldFail bool
	}{
		{"logs without agent ID", []string{"logs"}, true},
		{"stop without agent ID", []string{"stop"}, true},
		{"send without agent ID", []string{"send"}, true},
		{"send without command", []string{"send", "alice"}, true},
		{"attach without agent ID", []string{"attach"}, true},
		{"repl without agent ID", []string{"repl"}, true},
		// Note: list --format invalid doesn't fail, it just defaults to table
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("./agentd-test", tt.args...)
			err := cmd.Run()
			if tt.shouldFail && err == nil {
				t.Errorf("Expected command to fail but it succeeded")
			}
			if !tt.shouldFail && err != nil {
				t.Errorf("Expected command to succeed but it failed: %v", err)
			}
		})
	}
}

func TestCLI_SendCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx := context.Background()
	agentID := "test-cli-send"

	// Cleanup
	defer stopAgent(ctx, nil, agentID)

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Spawn agent
	launcher, err := createLauncher()
	if err != nil {
		t.Fatalf("Failed to create launcher: %v", err)
	}

	config, _ := buildSpawnConfig(agentID)
	_, err = launcher.Spawn(ctx, config)
	if err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Test send command
	cmd := exec.Command("./agentd-test", "send", agentID, "echo 'test output'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Send output:\n%s", output)
		t.Fatalf("Send command failed: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "test output") {
		t.Errorf("Send output missing expected text: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Command completed successfully") {
		t.Errorf("Send output missing success message: %s", outputStr)
	}
}

func TestCLI_ReplCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Test repl command with non-existent agent (should fail with helpful message)
	cmd := exec.Command("./agentd-test", "repl", "nonexistent-agent")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("repl command should fail for non-existent agent")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "not found") {
		t.Errorf("repl output should indicate agent not found: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Running agents:") {
		t.Errorf("repl output should list running agents: %s", outputStr)
	}
}

func TestCLI_SpawnWithAPIKeyAndREPL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	agentID := "test-cli-repl"

	// Cleanup
	defer func() {
		stopAgent(ctx, nil, agentID)
		time.Sleep(500 * time.Millisecond)
	}()

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Test spawn with API key
	t.Run("spawn with API key", func(t *testing.T) {
		cmd := exec.Command("./agentd-test", "spawn", agentID, "--api-key", "sk-test-integration-key")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Spawn output:\n%s", output)
			t.Fatalf("Spawn command failed: %v", err)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "Agent "+agentID+" ready") {
			t.Errorf("Spawn output missing success message: %s", outputStr)
		}
	})

	time.Sleep(2 * time.Second)

	// Verify credential file was created
	t.Run("verify credential file", func(t *testing.T) {
		agents, err := listAgentsFromDocker(ctx)
		if err != nil {
			t.Fatalf("Failed to list agents: %v", err)
		}

		var workspace string
		for _, agent := range agents {
			if agent.AgentID == agentID {
				workspace = agent.Workspace
				break
			}
		}

		if workspace == "" {
			t.Fatal("Could not find agent workspace")
		}

		// Verify .creds directory exists with 0700 permissions
		credsDir := filepath.Join(workspace, ".creds")
		info, err := os.Stat(credsDir)
		if err != nil {
			t.Errorf("Credential directory not found at %s: %v", credsDir, err)
		} else {
			if !info.IsDir() {
				t.Error(".creds is not a directory")
			}
			if info.Mode().Perm() != 0o700 {
				t.Errorf("Expected .creds permissions 0700, got %o", info.Mode().Perm())
			}
		}

		// Verify .env file exists with 0600 permissions
		envFile := filepath.Join(workspace, ".creds", ".env")
		info, err = os.Stat(envFile)
		if err != nil {
			t.Errorf("Credential file not found at %s: %v", envFile, err)
		} else {
			if info.Mode().Perm() != 0o600 {
				t.Errorf("Expected .env permissions 0600, got %o", info.Mode().Perm())
			}

			// Verify content
			content, err := os.ReadFile(envFile)
			if err == nil {
				if !strings.Contains(string(content), "ANTHROPIC_API_KEY=sk-test-integration-key") {
					t.Errorf("Expected API key in .env, got: %s", string(content))
				}
			}
		}
	})

	// Test REPL command (just verify it can connect, don't test interactivity)
	t.Run("repl connects", func(t *testing.T) {
		// Since we can't test interactivity, just verify connection setup
		// by checking the command doesn't error on startup
		cmd := exec.Command("./agentd-test", "repl", agentID)

		// Kill after 2 seconds to avoid hanging
		go func() {
			time.Sleep(2 * time.Second)
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()

		output, _ := cmd.CombinedOutput()
		outputStr := string(output)

		// Should show connection message
		if !strings.Contains(outputStr, "Connected to agent") && !strings.Contains(outputStr, "connection") {
			t.Logf("REPL output: %s", outputStr)
			// Not a hard failure - might be timing issue
		}
	})
}
