package containersession

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrepareWorkspace creates and validates a workspace directory for a session
// Follows strict path validation pattern from pkg/relay/session/manager.go:154-174
func PrepareWorkspace(basePath, sessionID string) (string, error) {
	// Build workspace path
	workspacePath := filepath.Join(basePath, sessionID)
	cleanPath := filepath.Clean(workspacePath)

	// Get absolute paths for validation
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve absolute path: %v", ErrInvalidWorkspacePath, err)
	}

	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve base path: %v", ErrInvalidWorkspacePath, err)
	}

	// Defense-in-depth: Check prefix with separator to prevent directory name bypass
	if absPath != baseAbs && !strings.HasPrefix(absPath, baseAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: workspace path must be under base directory %s", ErrInvalidWorkspacePath, basePath)
	}

	// Use filepath.Rel to prevent directory traversal with ".."
	relPath, err := filepath.Rel(baseAbs, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == ".." || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%w: workspace path must be under base directory %s", ErrInvalidWorkspacePath, basePath)
	}

	// Create directory with strict permissions (owner-only access)
	err = os.MkdirAll(absPath, 0o700)
	if err != nil {
		return "", fmt.Errorf("failed to create workspace directory: %w", err)
	}

	return absPath, nil
}

// ValidateWorkspacePath verifies that a workspace path is under the base directory.
// This function is used to validate workspace paths extracted from container mounts
// to prevent directory traversal attacks when reusing containers.
//
// Security: A malicious actor could create a container with our session labels but
// mount an arbitrary host path at /workspace. This validation ensures we only accept
// paths that are descendants of baseWorkspaceDir.
func ValidateWorkspacePath(baseWorkspaceDir, workspacePath string) error {
	// Get absolute paths for validation
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve workspace path: %v", ErrInvalidWorkspacePath, err)
	}

	baseAbs, err := filepath.Abs(baseWorkspaceDir)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve base path: %v", ErrInvalidWorkspacePath, err)
	}

	// Defense-in-depth: Check prefix with separator to prevent directory name bypass
	if absPath != baseAbs && !strings.HasPrefix(absPath, baseAbs+string(os.PathSeparator)) {
		return fmt.Errorf("%w: workspace path must be under base directory %s", ErrInvalidWorkspacePath, baseWorkspaceDir)
	}

	// Use filepath.Rel to prevent directory traversal with ".."
	relPath, err := filepath.Rel(baseAbs, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == ".." || filepath.IsAbs(relPath) {
		return fmt.Errorf("%w: workspace path must be under base directory %s", ErrInvalidWorkspacePath, baseWorkspaceDir)
	}

	return nil
}

// CleanupWorkspace removes a workspace directory
// Idempotent - does not fail if directory doesn't exist
func CleanupWorkspace(path string, logger Logger) error {
	err := os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		logger.Printf("WARN: Failed to cleanup workspace %s: %v", path, err)
		return fmt.Errorf("failed to cleanup workspace: %w", err)
	}
	return nil
}
