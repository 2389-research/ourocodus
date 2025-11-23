package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/2389-research/ourocodus/pkg/labels"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

// ValidateNonEmpty validates that a string is not empty or whitespace-only.
// Returns the provided error if validation fails, nil otherwise.
func ValidateNonEmpty(value string, err error) error {
	if strings.TrimSpace(value) == "" {
		return err
	}
	return nil
}

// ValidateWorkspacePath validates and constrains a workspace path under a base directory.
// It performs path traversal prevention and ensures the path is within the allowed base.
// Returns the absolute path if valid, error otherwise.
func ValidateWorkspacePath(workspace, baseWorkspaceDir string) (string, error) {
	// Validate and constrain workspace path under base directory
	cleanPath := filepath.Clean(workspace)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid workspace path: %w", err)
	}

	baseAbs, err := filepath.Abs(baseWorkspaceDir)
	if err != nil {
		return "", fmt.Errorf("invalid base workspace directory: %w", err)
	}

	// Defense-in-depth: Check prefix with separator to prevent directory name bypass
	if absPath != baseAbs && !strings.HasPrefix(absPath, baseAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace path must be under base directory %s", baseWorkspaceDir)
	}

	// Use filepath.Rel to prevent directory traversal with ".."
	relPath, err := filepath.Rel(baseAbs, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == ".." || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("workspace path must be under base directory %s", baseWorkspaceDir)
	}

	return absPath, nil
}

// CloseACPClientSafely safely closes an ACP client with double-close protection.
// This helper extracts the client from the agent session atomically and closes it outside the lock.
// It transitions the agent to the AgentTerminated state.
// CRITICAL: Accepts context to allow proper cancellation handling (fixes Issue #212).
func CloseACPClientSafely(agent *AgentSession, ctx context.Context, logger Logger, userSessionID, agentID string) error {
	// Close ACP client (with double-close protection)
	agent.mu.Lock()
	acpClient := agent.acpClient
	if acpClient != nil {
		agent.acpClient = nil // Clear before Close to prevent double-close
	}
	agent.setAgentState(AgentTerminated)
	agent.mu.Unlock()

	// Close outside the lock with context (no goroutine wrapper - fixes Issue #212)
	if acpClient != nil {
		if err := acpClient.Close(ctx); err != nil {
			logger.Printf("Error closing ACP client: session=%s agentID=%s error=%v", userSessionID, agentID, err)
			return err
		}
	}
	return nil
}

// StopContainerAndCleanupLauncher stops a container and removes it from the launcher maps.
// Uses atomic take-and-delete pattern to prevent double-stop race (Issue #210).
// Returns true if the operation failed (for error tracking).
func StopContainerAndCleanupLauncher(
	ctx context.Context,
	m *Manager,
	userSessionID, agentID string,
	logger Logger,
) bool {
	if !m.isContainerModeEnabled() {
		return false
	}

	key := launcherKey(userSessionID, agentID)

	// Mutex-protected take-and-delete pattern to prevent double-stop race (Issue #210)
	m.launchersMu.Lock()
	launcher := m.launchers[key]
	handle := m.handles[key]
	delete(m.launchers, key) // Delete BEFORE releasing lock
	delete(m.handles, key)
	m.launchersMu.Unlock()

	// Now safe - only one goroutine has these pointers
	failed := false
	if launcher != nil && handle != nil {
		if err := launcher.Stop(ctx, handle); err != nil {
			logger.Printf("WARN: Failed to stop container for agent %s: %v", agentID, err)
			failed = true
			// Continue cleanup despite error
		}
	}

	return failed
}

// StopCLISpawnedContainer stops a CLI-spawned container and cleans up all associated resources.
// This is used for attached agents that weren't spawned by the relay (no launcher entry).
// Cleanup includes: Docker container, git worktree, branch, lease file, and token file.
// Returns true if the operation failed (for error tracking).
func StopCLISpawnedContainer(
	ctx context.Context,
	agentID string,
	logger Logger,
) bool {
	failed := false

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Printf("WARN: Failed to create Docker client for stopping CLI agent %s: %v", agentID, err)
		return true
	}
	defer func() { _ = cli.Close() }()

	// Find container by agent-id label
	containerID, workspacePath, err := FindAgentContainerIDForTesting(ctx, agentID)
	if err != nil {
		logger.Printf("WARN: Failed to find CLI agent container %s: %v", agentID, err)
		return true
	}

	if containerID == "" {
		// Container not found - may have already been stopped
		logger.Printf("CLI agent container %s not found (may be already stopped)", agentID)
		// Continue with cleanup of other resources even if container is gone
	} else {
		// Stop the container with grace period (same as agentd stop)
		timeout := 30
		if err := cli.ContainerStop(ctx, containerID, container.StopOptions{
			Timeout: &timeout,
		}); err != nil {
			// Use errdefs for robust error classification (ignore NotFound for idempotence)
			//nolint:staticcheck // errdefs is Docker SDK's official error handling
			if !errdefs.IsNotFound(err) {
				logger.Printf("WARN: Failed to stop CLI agent container %s: %v", agentID, err)
				failed = true
			}
		}

		// Remove the container to clean up artifacts (idempotent)
		if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			// Use errdefs for robust error classification (ignore NotFound for idempotence)
			//nolint:staticcheck // errdefs is Docker SDK's official error handling
			if !errdefs.IsNotFound(err) {
				logger.Printf("WARN: Failed to remove CLI agent container %s: %v", agentID, err)
				failed = true
			}
		}

		logger.Printf("Stopped and removed CLI agent container: agent=%s container=%s", agentID, formatContainerID(containerID))
	}

	// Clean up worktree if workspace path is known
	if workspacePath != "" {
		if err := cleanupWorktree(ctx, workspacePath, logger); err != nil {
			logger.Printf("WARN: Failed to cleanup worktree for agent %s: %v", agentID, err)
			// Don't mark as failed - worktree cleanup is best-effort
		}
	}

	// Release lease file (.agentd/session/{agent-id}.lease)
	if err := ReleaseLease(agentID); err != nil {
		logger.Printf("WARN: Failed to release lease for agent %s: %v", agentID, err)
		// Don't mark as failed - lease cleanup is best-effort
	} else {
		logger.Printf("Released lease for agent %s", agentID)
	}

	// Delete attach token file (.agentd/session/{agent-id}.token)
	if err := deleteAttachToken(agentID); err != nil {
		logger.Printf("WARN: Failed to delete attach token for agent %s: %v", agentID, err)
		// Don't mark as failed - token cleanup is best-effort
	} else {
		logger.Printf("Deleted attach token for agent %s", agentID)
	}

	return failed
}

// formatContainerID truncates container ID to first 12 characters for logging
func formatContainerID(containerID string) string {
	if len(containerID) > 12 {
		return containerID[:12]
	}
	return containerID
}

// FindAgentContainerIDForTesting discovers a CLI-spawned agent container by agent ID using Docker labels.
// This function is exported for testing purposes and used by integration tests.
//
// It queries the Docker daemon for containers with the label "ourocodus.agent/agent-id"
// matching the provided agentID, and extracts the container ID and workspace path.
//
// Returns:
//   - containerID: The Docker container ID
//   - workspace: The workspace path extracted from container labels
//   - error: If container not found, not running, or Docker API errors
func FindAgentContainerIDForTesting(ctx context.Context, agentID string) (containerID, workspace string, err error) {
	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", "", fmt.Errorf("failed to create docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	// Build filter using centralized label builder to ensure consistency
	filterArgs := labels.FindAgentFilter(agentID)

	// List containers with the agent label
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
		All:     false, // Only running containers
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return "", "", fmt.Errorf("no running container found for agent ID: %s", agentID)
	}

	if len(containers) > 1 {
		return "", "", fmt.Errorf("multiple containers found for agent ID %s (found %d)", agentID, len(containers))
	}

	// Extract container ID and workspace from labels using centralized constants
	ctr := containers[0]
	containerID = ctr.ID
	workspace = ctr.Labels[labels.Workspace]

	if workspace == "" {
		// Defensive: format container ID safely for error message
		shortID := containerID
		if len(containerID) > 12 {
			shortID = containerID[:12]
		}
		return "", "", fmt.Errorf("container %s missing workspace label", shortID)
	}

	return containerID, workspace, nil
}

// cleanupWorktree removes a git worktree and its associated branch.
// This is best-effort cleanup that logs warnings but doesn't fail the overall termination.
func cleanupWorktree(ctx context.Context, workspacePath string, logger Logger) error {
	// Get repository root from the worktree
	root, err := getRepoRoot(ctx, workspacePath)
	if err != nil {
		logger.Printf("WARN: Failed to get repo root for %s: %v", workspacePath, err)
		return err
	}

	// Get the branch name before removing the worktree
	branchName, err := getWorktreeBranch(ctx, root, workspacePath)
	if err != nil {
		logger.Printf("WARN: Failed to get worktree branch for %s: %v", workspacePath, err)
		// Continue with worktree removal even if we can't get the branch
	}

	// Remove the worktree (use -C to specify repository root)
	// #nosec G204 - root and workspacePath are validated via getRepoRoot and Docker labels
	cmd := exec.CommandContext(ctx, "git", "-C", root, "worktree", "remove", workspacePath, "--force")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %v: %s", err, strings.TrimSpace(string(out)))
	}

	logger.Printf("Removed worktree: %s", workspacePath)

	// Delete the branch if we found one
	if branchName != "" {
		// #nosec G204 - root is validated via getRepoRoot, branchName from git output
		cmd := exec.CommandContext(ctx, "git", "-C", root, "branch", "-D", branchName)
		if err := cmd.Run(); err != nil {
			logger.Printf("WARN: Failed to delete branch %s: %v", branchName, err)
			// Don't return error - branch deletion is best-effort
		} else {
			logger.Printf("Deleted branch: %s", branchName)
		}
	}

	return nil
}

// getRepoRoot finds the repository root for a given worktree path.
func getRepoRoot(ctx context.Context, worktreePath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve repo root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getWorktreeBranch extracts the branch name for a given worktree path.
// repoRoot is the repository root directory (from getRepoRoot).
func getWorktreeBranch(ctx context.Context, repoRoot, workspacePath string) (string, error) {
	// Run git worktree list --porcelain from the repository root
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Parse the porcelain output to find the branch for this workspace
	return parseBranchFromWorktreeList(string(output), workspacePath)
}

// parseBranchFromWorktreeList parses git worktree list --porcelain output to find the branch for a workspace.
// Format:
//
//	worktree /path/to/worktree
//	HEAD <commit-hash>
//	branch refs/heads/branch-name
//	<blank line>
func parseBranchFromWorktreeList(output, workspacePath string) (string, error) {
	lines := strings.Split(output, "\n")
	foundWorktree := false

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Look for the worktree line matching our path
		if strings.HasPrefix(line, "worktree ") {
			worktreePath := strings.TrimPrefix(line, "worktree ")
			if worktreePath == workspacePath {
				foundWorktree = true
			} else {
				foundWorktree = false
			}
			continue
		}

		// If we found the matching worktree, look for the branch line
		if foundWorktree && strings.HasPrefix(line, "branch ") {
			branchRef := strings.TrimPrefix(line, "branch ")
			// Extract branch name from refs/heads/branch-name
			branchName := strings.TrimPrefix(branchRef, "refs/heads/")
			return branchName, nil
		}
	}

	return "", fmt.Errorf("worktree not found or has no branch: %s", workspacePath)
}

// deleteAttachToken removes the attach token file for an agent.
// This is idempotent - returns nil if the file doesn't exist.
func deleteAttachToken(agentID string) error {
	// Validate agentID to prevent path traversal
	if err := validateAgentID(agentID); err != nil {
		return err
	}

	tokenPath := filepath.Join(".agentd/session", agentID+".token")
	if err := os.Remove(tokenPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, idempotent
		}
		return fmt.Errorf("failed to remove token file: %w", err)
	}
	return nil
}
