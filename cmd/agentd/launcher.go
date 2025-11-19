package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/docker/docker/client"
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

	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Create container session manager
	containerMgr := containersession.NewManager(
		dockerClient,
		&defaultIDGenerator{},
		&defaultClock{},
		&defaultLogger{},
		worktreesBaseDir, // base workspace dir (absolute)
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

// defaultIDGenerator implements containersession.IDGenerator
type defaultIDGenerator struct{}

func (g *defaultIDGenerator) Generate() string {
	// Generate session ID: session-<shortid>
	return fmt.Sprintf("session-%s", generateShortID())
}

// defaultClock implements containersession.Clock
type defaultClock struct{}

func (c *defaultClock) Now() time.Time {
	return time.Now()
}

// defaultLogger implements containersession.Logger
type defaultLogger struct{}

func (l *defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *defaultLogger) Println(v ...interface{}) {
	log.Println(v...)
}
