package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
func CloseACPClientSafely(agent *AgentSession, logger Logger, userSessionID, agentID string) error {
	// Close ACP client (with double-close protection)
	agent.mu.Lock()
	acpClient := agent.acpClient
	if acpClient != nil {
		agent.acpClient = nil // Clear before Close to prevent double-close
	}
	agent.setAgentState(AgentTerminated)
	agent.mu.Unlock()

	// Close outside the lock
	if acpClient != nil {
		if err := acpClient.Close(); err != nil {
			logger.Printf("Error closing ACP client: session=%s agentID=%s error=%v", userSessionID, agentID, err)
			return err
		}
	}
	return nil
}

// StopContainerAndCleanupLauncher stops a container and removes it from the launcher maps.
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
	m.launchersMu.RLock()
	launcher := m.launchers[key]
	handle := m.handles[key]
	m.launchersMu.RUnlock()

	failed := false
	if launcher != nil && handle != nil {
		if err := launcher.Stop(ctx, handle); err != nil {
			logger.Printf("WARN: Failed to stop container for agent %s: %v", agentID, err)
			failed = true
			// Continue cleanup despite error
		}
	}

	// Remove from launcher maps
	m.launchersMu.Lock()
	delete(m.launchers, key)
	delete(m.handles, key)
	m.launchersMu.Unlock()

	return failed
}

// ACPClientHandle represents a handle to an ACP client for safe cleanup.
type ACPClientHandle struct {
	client        ACPClient
	logger        Logger
	userSessionID string
	agentID       string
}

// Close safely closes the ACP client and logs any errors.
func (h *ACPClientHandle) Close() error {
	if h.client == nil {
		return nil
	}
	if err := h.client.Close(); err != nil {
		h.logger.Printf("Error closing ACP client: session=%s agentID=%s error=%v",
			h.userSessionID, h.agentID, err)
		return err
	}
	return nil
}

// ExtractACPClient extracts the ACP client from an agent session with double-close protection.
// Returns a handle that can be safely closed outside the lock.
func ExtractACPClient(agent *AgentSession, logger Logger, userSessionID, agentID string) *ACPClientHandle {
	agent.mu.Lock()
	acpClient := agent.acpClient
	if acpClient != nil {
		agent.acpClient = nil // Clear before Close to prevent double-close
	}
	agent.setAgentState(AgentTerminated)
	agent.mu.Unlock()

	return &ACPClientHandle{
		client:        acpClient,
		logger:        logger,
		userSessionID: userSessionID,
		agentID:       agentID,
	}
}
