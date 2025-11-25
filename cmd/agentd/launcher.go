package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/containersession/helpers"
	"github.com/2389-research/ourocodus/pkg/worktree"
)

// createLauncher instantiates AgentContainerLauncher with all dependencies
func createLauncher() (*container.AgentContainerLauncher, error) {
	// Get current working directory for repository path
	repoPath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Build absolute path for worktrees base directory
	worktreesBaseDir := fmt.Sprintf("%s/.agentd/worktrees", repoPath)

	// Create Docker client with Colima fallback
	dockerClient, err := helpers.CreateDockerClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Create container session manager with standard helpers
	containerMgr := containersession.NewManager(
		dockerClient,
		&helpers.UUIDGenerator{},
		&helpers.SystemClock{},
		&helpers.StdLogger{Logger: log.Default()},
		worktreesBaseDir,
	)

	// Create worktree manager
	worktreeMgr, err := worktree.NewAgentWorktreeManager(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree manager: %w", err)
	}

	// Create credential mounter (absolute path)
	credMounter := container.NewAgentCredentialMounter(fmt.Sprintf("%s/.agentd/credentials", repoPath))

	// Create launcher
	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		worktreesBaseDir,
	)

	return launcher, nil
}
