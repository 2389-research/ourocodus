package helpers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WorktreeInfo holds information about a git worktree
type WorktreeInfo struct {
	Path         string
	CommitCount  int
	LatestCommit string
}

// CheckWorktreeExists verifies a worktree directory exists
func CheckWorktreeExists(worktreePath string) (bool, error) {
	info, err := os.Stat(worktreePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check worktree path: %w", err)
	}
	return info.IsDir(), nil
}

// GetWorktreeCommits returns the number of commits in a worktree since a given time
func GetWorktreeCommits(ctx context.Context, worktreePath string, since time.Time) (int, error) {
	// Check if worktree exists
	exists, err := CheckWorktreeExists(worktreePath)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, fmt.Errorf("worktree does not exist: %s", worktreePath)
	}

	// Get commit count since the specified time
	sinceStr := since.Format(time.RFC3339)
	// #nosec G204 -- worktreePath is constructed via filepath.Join from test-controlled values (projectRoot + "agent" + role)
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "rev-list", "--count", fmt.Sprintf("--since=%s", sinceStr), "HEAD")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to get commit count: %w\nOutput: %s", err, string(output))
	}

	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count); err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// GetLatestCommitMessage returns the latest commit message in a worktree
func GetLatestCommitMessage(ctx context.Context, worktreePath string) (string, error) {
	// Check if worktree exists
	exists, err := CheckWorktreeExists(worktreePath)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("worktree does not exist: %s", worktreePath)
	}

	// Get latest commit message
	// #nosec G204 -- worktreePath is constructed via filepath.Join from test-controlled values (projectRoot + "agent" + role)
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "log", "-1", "--pretty=%B")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get commit message: %w\nOutput: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// GetWorktreeInfo returns comprehensive information about a worktree
func GetWorktreeInfo(ctx context.Context, worktreePath string, since time.Time) (*WorktreeInfo, error) {
	commits, err := GetWorktreeCommits(ctx, worktreePath, since)
	if err != nil {
		return nil, err
	}

	var latestCommit string
	if commits > 0 {
		latestCommit, err = GetLatestCommitMessage(ctx, worktreePath)
		if err != nil {
			return nil, err
		}
	}

	return &WorktreeInfo{
		Path:         worktreePath,
		CommitCount:  commits,
		LatestCommit: latestCommit,
	}, nil
}

// WaitForWorktreeCommits polls a worktree until it has commits or times out
func WaitForWorktreeCommits(ctx context.Context, worktreePath string, since time.Time, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for commits in worktree %s: %w", worktreePath, ctx.Err())
		default:
		}

		commits, err := GetWorktreeCommits(ctx, worktreePath, since)
		if err != nil {
			lastErr = err
		} else if commits > 0 {
			return nil
		}

		// Wait before retrying
		select {
		case <-time.After(1 * time.Second):
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for commits in worktree %s: %w", worktreePath, ctx.Err())
		}
	}

	if lastErr != nil {
		return fmt.Errorf("timeout waiting for commits in worktree %s (last error: %w)", worktreePath, lastErr)
	}
	return fmt.Errorf("timeout waiting for commits in worktree: %s", worktreePath)
}

// VerifyAllWorktreesHaveCommits checks that all specified worktrees have commits since the test started
func VerifyAllWorktreesHaveCommits(ctx context.Context, worktreeBasePath string, agentNames []string, since time.Time, timeout time.Duration) error {
	// Give each agent a fair share of the timeout
	perAgentTimeout := timeout
	if len(agentNames) > 1 {
		perAgentTimeout = timeout / time.Duration(len(agentNames))
		// Ensure minimum timeout of 5 seconds per agent
		if perAgentTimeout < 5*time.Second {
			perAgentTimeout = 5 * time.Second
		}
	}

	for _, agentName := range agentNames {
		worktreePath := filepath.Join(worktreeBasePath, agentName)

		if err := WaitForWorktreeCommits(ctx, worktreePath, since, perAgentTimeout); err != nil {
			return fmt.Errorf("agent %s worktree verification failed: %w", agentName, err)
		}
	}

	return nil
}
