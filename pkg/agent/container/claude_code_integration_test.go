//go:build integration

package container_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
)

func TestClaudeCodeContainer_Integration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test (set RUN_INTEGRATION_TESTS=true)")
	}

	// Skip if Docker is not available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Skipping: docker not available")
	}

	// Skip if no API key (optional - can test container creation without it)
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Log("ANTHROPIC_API_KEY not set - will test container structure only")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Setup test repository
	tmpDir := setupTestRepo(t)

	// Create Docker client
	dockerClient := setupDockerClient(t)
	defer dockerClient.Close()

	// Create managers
	worktreeMgr, err := worktree.NewAgentWorktreeManager(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create worktree manager: %v", err)
	}

	containerMgr := containersession.NewManager(
		dockerClient,
		&testIDGenerator{},
		&testClock{},
		&testLogger{t: t},
		filepath.Join(tmpDir, "workspaces"),
	)

	credMounter := container.NewAgentCredentialMounter(filepath.Join(tmpDir, "creds"))

	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		filepath.Join(tmpDir, "workspaces"),
	)

	// Create SpawnConfig with RuntimeHardening
	spawnConfig := container.SpawnConfig{
		AgentID:    "test-claude-code",
		ImageName:  "ourocodus/agent:latest",
		Command:    []string{"/usr/local/bin/claude-code-entry.sh"},
		Entrypoint: []string{"/usr/bin/tini", "--"},
		APIKey:     apiKey,
		RuntimeHardening: container.RuntimeHardening{
			ReadOnlyRootfs:  true,
			DropAllCaps:     true,
			NoNewPrivileges: true,
			MemoryLimitMB:   2048,
			CPULimit:        2.0,
			TmpfsSizeMB:     100,
		},
	}

	// Spawn Claude Code container
	handle, err := launcher.Spawn(ctx, spawnConfig)
	if err != nil {
		t.Fatalf("Failed to spawn container: %v", err)
	}
	defer func() {
		if err := launcher.Stop(context.Background(), "test-claude-code"); err != nil {
			t.Logf("Warning: failed to stop container: %v", err)
		}
	}()

	// Verify container is created
	if handle.ContainerID() == "" {
		t.Error("Expected container ID to be set")
	}

	t.Logf("Claude Code container created: %s", handle.ContainerID())
}
