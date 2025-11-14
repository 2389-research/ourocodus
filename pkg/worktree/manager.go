package worktree

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AgentWorktreeManager manages git worktrees for AgentSession workspace isolation.
//
// Each AgentSession requires an isolated workspace to avoid conflicts with other
// concurrent agents. AgentWorktreeManager creates git worktrees with unique branches,
// providing filesystem and git isolation.
//
// Thread-safety: AgentWorktreeManager is safe for concurrent use via internal mutex
// serialization. Methods may block if another goroutine holds the lock.
type AgentWorktreeManager struct {
	repoPath string
	mu       sync.Mutex // Serializes all mutating operations
}

// NewAgentWorktreeManager creates a new worktree manager for the given repository.
//
// The repoPath must be a valid git repository with a working tree.
// Returns an error if repoPath is empty or not a valid git repository.
func NewAgentWorktreeManager(repoPath string) (*AgentWorktreeManager, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repoPath cannot be empty")
	}

	// Verify this is a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("not a valid git repository: %w", err)
	}

	return &AgentWorktreeManager{
		repoPath: repoPath,
	}, nil
}

// validateAgentID validates that an agent ID is safe for use in filesystem paths.
// Returns ErrInvalidAgentID if the agentID contains path separators or traversal sequences.
func validateAgentID(agentID string) error {
	if agentID == "" {
		return ErrInvalidAgentID
	}

	// Check for path separators and traversal sequences
	if strings.Contains(agentID, "/") || strings.Contains(agentID, "\\") ||
		strings.Contains(agentID, "..") || strings.Contains(agentID, string(filepath.Separator)) {
		return fmt.Errorf("%w: agent ID must not contain path separators or traversal sequences", ErrInvalidAgentID)
	}

	return nil
}

// validatePath validates that a path is safe and within expected boundaries.
// For baseDir: must be absolute path.
// For worktreePath: must be absolute and not contain traversal sequences.
func validatePath(path string, mustBeAbsolute bool) error {
	if path == "" {
		return ErrInvalidPath
	}

	// Clean the path to resolve any . or .. components
	cleanPath := filepath.Clean(path)

	// Check if path must be absolute
	if mustBeAbsolute && !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("%w: path must be absolute", ErrInvalidPath)
	}

	// Check for path traversal attempts
	// After cleaning, if the path still contains ".." it's trying to traverse
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("%w: path contains traversal sequences", ErrInvalidPath)
	}

	return nil
}

// validateWorktreePath validates that a worktree path is safe for removal.
// Ensures the path is absolute and doesn't contain traversal sequences.
func validateWorktreePath(worktreePath string) error {
	if err := validatePath(worktreePath, true); err != nil {
		return err
	}

	// Additional check: worktree paths should contain "agent-" to prevent
	// accidental removal of non-worktree directories
	if !strings.Contains(filepath.Base(worktreePath), "agent-") {
		return fmt.Errorf("%w: worktree path must contain 'agent-' prefix", ErrInvalidPath)
	}

	return nil
}

// AgentWorktree represents an isolated git worktree for an AgentSession.
type AgentWorktree struct {
	path       string
	branchName string
	createdAt  time.Time
}

// Path returns the filesystem path to the worktree directory.
func (w *AgentWorktree) Path() string {
	return w.path
}

// BranchName returns the git branch name for this worktree.
func (w *AgentWorktree) BranchName() string {
	return w.branchName
}

// CreatedAt returns when the worktree was created.
func (w *AgentWorktree) CreatedAt() time.Time {
	return w.createdAt
}

// Create creates a new git worktree for an AgentSession.
//
// Parameters:
//   - ctx: Context for cancellation
//   - agentID: Unique identifier for the AgentSession (e.g., "coder", "reviewer", "agent-abc123")
//   - baseDir: Base directory where worktree will be created (e.g., "/workspaces")
//
// The worktree will be created at: {baseDir}/agent-{agentID}/
// A new branch will be created: agent-{agentID}-{timestamp}
//
// Returns:
//   - AgentWorktree: Information about the created worktree
//   - error: Non-nil if creation fails
//
// Example:
//
//	wt, err := manager.Create(ctx, "coder-abc123", "/workspaces")
//	// Creates: /workspaces/agent-coder-abc123/
//	// Branch: agent-coder-abc123-20250105-143022
func (m *AgentWorktreeManager) Create(ctx context.Context, agentID, baseDir string) (*AgentWorktree, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate agentID for path traversal attempts
	if err := validateAgentID(agentID); err != nil {
		return nil, err
	}

	// Validate baseDir is absolute and safe
	if err := validatePath(baseDir, true); err != nil {
		return nil, fmt.Errorf("invalid baseDir: %w", err)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Generate unique branch name with timestamp
	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("agent-%s-%s", agentID, timestamp)

	// Create worktree path - safe because agentID is validated
	worktreePath := filepath.Join(baseDir, fmt.Sprintf("agent-%s", agentID))

	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	// Clean up any stale worktree directory/registration before creating
	if err := m.cleanupStaleWorktree(ctx, worktreePath); err != nil {
		return nil, fmt.Errorf("failed to cleanup stale worktree: %w", err)
	}

	// Create worktree with new branch
	// git worktree add -b <branch> <path>
	// #nosec G204 -- git command with validated arguments
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, worktreePath)
	cmd.Dir = m.repoPath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w (stderr: %s)", err, stderr.String())
	}

	return &AgentWorktree{
		path:       worktreePath,
		branchName: branchName,
		createdAt:  time.Now(),
	}, nil
}

// cleanupStaleWorktree removes any stale worktree directory/registration at the given path.
// This is called before creating a new worktree to handle interrupted/failed previous runs.
// It's idempotent and safe to call even if no stale worktree exists.
func (m *AgentWorktreeManager) cleanupStaleWorktree(ctx context.Context, worktreePath string) error {
	// Validate worktree path for safety
	if err := validateWorktreePath(worktreePath); err != nil {
		return err
	}

	// Check if directory exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		// No stale directory, nothing to clean up
		return nil
	}

	// Directory exists - try to remove it from git's worktree tracking
	// This uses the existing Remove method which handles all cleanup
	if err := m.Remove(ctx, worktreePath); err != nil {
		// If Remove fails, try manual cleanup
		// This can happen if git doesn't know about the directory
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("failed to remove stale directory: %w", err)
		}
	}

	// Verify directory was actually removed (defensive check)
	// Even if Remove() succeeded, ensure the directory is gone
	if _, err := os.Stat(worktreePath); err == nil {
		// Directory still exists, force removal
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("failed to remove stale directory after cleanup: %w", err)
		}
	}

	// Prune stale worktree references from git after cleanup
	// This runs after attempting to remove the directory, regardless of whether Remove() succeeded or failed
	// Best effort - we don't fail the operation if prune fails since the directory is already cleaned
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = m.repoPath
	_ = cmd.Run()

	return nil
}

// Remove removes a git worktree and its associated branch.
//
// Parameters:
//   - ctx: Context for cancellation
//   - worktreePath: Filesystem path to the worktree to remove
//
// This will:
//  1. Remove the worktree from git's tracking
//  2. Delete the worktree directory from filesystem
//  3. Delete the associated branch (if it exists)
//
// Returns an error if removal fails. Idempotent - safe to call multiple times.
func (m *AgentWorktreeManager) Remove(ctx context.Context, worktreePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate worktree path for safety
	if err := validateWorktreePath(worktreePath); err != nil {
		return err
	}

	// Check context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Find the branch name before removing
	branchName := m.findBranchForWorktree(ctx, worktreePath)

	// Remove worktree and directory
	if err := m.removeWorktreeAndDir(ctx, worktreePath); err != nil {
		return err
	}

	// Clean up branch if found
	m.removeBranch(ctx, branchName)

	return nil
}

// findBranchForWorktree finds the git branch associated with a worktree path.
// Returns empty string if no branch is found.
func (m *AgentWorktreeManager) findBranchForWorktree(ctx context.Context, worktreePath string) string {
	// Resolve symlinks for path comparison (macOS /tmp -> /private/tmp)
	resolvedWorktreePath, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		resolvedWorktreePath = worktreePath
	}

	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoPath
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse worktree list to find branch
	lines := strings.Split(string(output), "\n")
	var currentPath string
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			// Compare paths with symlink resolution
			resolvedCurrentPath, _ := filepath.EvalSymlinks(currentPath)
			if resolvedCurrentPath == "" {
				resolvedCurrentPath = currentPath
			}
			if resolvedCurrentPath == resolvedWorktreePath || currentPath == worktreePath {
				return strings.TrimPrefix(line, "branch refs/heads/")
			}
		}
	}
	return ""
}

// removeWorktreeAndDir removes the worktree from git tracking and ensures the directory is deleted.
func (m *AgentWorktreeManager) removeWorktreeAndDir(ctx context.Context, worktreePath string) error {
	// Remove worktree from git tracking
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", worktreePath, "--force")
	cmd.Dir = m.repoPath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if worktree doesn't exist (idempotent)
		if !strings.Contains(stderr.String(), "not a working tree") &&
			!strings.Contains(stderr.String(), "does not exist") {
			return fmt.Errorf("failed to remove worktree: %w (stderr: %s)", err, stderr.String())
		}
	}

	// Verify the directory was actually removed
	// If it still exists, remove it manually (happens when git doesn't know about the worktree)
	if _, err := os.Stat(worktreePath); err == nil {
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("failed to remove worktree directory: %w", err)
		}
	}

	return nil
}

// removeBranch removes a git branch. Best effort - logs errors but does not fail.
func (m *AgentWorktreeManager) removeBranch(ctx context.Context, branchName string) {
	if branchName == "" {
		return
	}

	// Prune worktree references first
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = m.repoPath
	if err := cmd.Run(); err != nil {
		log.Printf("[WARN] Failed to prune worktree references: %v", err)
	}

	// Delete the branch
	// #nosec G204 -- git command with controlled arguments
	cmd = exec.CommandContext(ctx, "git", "branch", "-D", branchName)
	cmd.Dir = m.repoPath
	if err := cmd.Run(); err != nil {
		log.Printf("[WARN] Failed to delete branch %s: %v", branchName, err)
	}
}

// List returns all worktrees managed by this repository.
//
// This includes all worktrees created for AgentSessions (branches starting with "agent-").
// The main repository worktree is excluded from the results.
//
// Returns:
//   - []AgentWorktree: List of all agent worktrees
//   - error: Non-nil if listing fails
func (m *AgentWorktreeManager) List(ctx context.Context) ([]*AgentWorktree, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// List worktrees in porcelain format for easier parsing
	// git worktree list --porcelain
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return parseWorktreeList(string(output)), nil
}

// parseWorktreeList parses git worktree list --porcelain output.
func parseWorktreeList(output string) []*AgentWorktree {
	var result []*AgentWorktree
	lines := strings.Split(output, "\n")

	var currentPath string
	var currentBranch string

	saveWorktree := func() {
		if currentPath != "" && currentBranch != "" && strings.HasPrefix(currentBranch, "agent-") {
			result = append(result, &AgentWorktree{
				path:       currentPath,
				branchName: currentBranch,
				createdAt:  time.Time{}, // Not available from git
			})
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "worktree "):
			saveWorktree() // Save previous entry
			currentPath = strings.TrimPrefix(line, "worktree ")
			currentBranch = ""
		case strings.HasPrefix(line, "branch "):
			fullBranch := strings.TrimPrefix(line, "branch ")
			currentBranch = strings.TrimPrefix(fullBranch, "refs/heads/")
		case line == "":
			saveWorktree() // End of entry
			currentPath = ""
			currentBranch = ""
		}
	}

	// Handle last entry
	saveWorktree()

	return result
}
