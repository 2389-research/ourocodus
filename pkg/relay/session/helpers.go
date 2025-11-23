package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/2389-research/ourocodus/pkg/labels"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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
