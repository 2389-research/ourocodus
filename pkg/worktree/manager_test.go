package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Configure git for tests
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Create initial commit (required for worktrees)
	dummyFile := filepath.Join(tmpDir, "README.md")
	err = os.WriteFile(dummyFile, []byte("# Test Repo\n"), 0o644)
	require.NoError(t, err)

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	return tmpDir
}

func TestNewAgentWorktreeManager(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repoDir := setupTestRepo(t)

		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)
		assert.NotNil(t, manager)
		assert.Equal(t, repoDir, manager.repoPath)
	})

	t.Run("EmptyPath", func(t *testing.T) {
		manager, err := NewAgentWorktreeManager("")
		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("NotAGitRepository", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "not-git-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		manager, err := NewAgentWorktreeManager(tmpDir)
		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "not a valid git repository")
	})
}

func TestAgentWorktreeManager_Create(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx := context.Background()
		baseDir := filepath.Join(repoDir, "workspaces")

		wt, err := manager.Create(ctx, "coder-123", baseDir)
		require.NoError(t, err)
		assert.NotNil(t, wt)

		// Verify worktree properties
		assert.Contains(t, wt.Path(), "agent-coder-123")
		assert.Contains(t, wt.BranchName(), "agent-coder-123")
		assert.False(t, wt.CreatedAt().IsZero())

		// Verify worktree directory exists
		_, err = os.Stat(wt.Path())
		assert.NoError(t, err)

		// Verify branch exists
		cmd := exec.Command("git", "branch", "--list", wt.BranchName())
		cmd.Dir = repoDir
		output, err := cmd.Output()
		require.NoError(t, err)
		assert.Contains(t, string(output), wt.BranchName())
	})

	t.Run("EmptyAgentID", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx := context.Background()
		baseDir := filepath.Join(repoDir, "workspaces")

		wt, err := manager.Create(ctx, "", baseDir)
		assert.Error(t, err)
		assert.Nil(t, wt)
		assert.Contains(t, err.Error(), "agentID")
	})

	t.Run("EmptyBaseDir", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx := context.Background()

		wt, err := manager.Create(ctx, "coder-123", "")
		assert.Error(t, err)
		assert.Nil(t, wt)
		assert.Contains(t, err.Error(), "baseDir")
	})

	t.Run("ContextCanceled", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		baseDir := filepath.Join(repoDir, "workspaces")
		wt, err := manager.Create(ctx, "coder-123", baseDir)
		assert.Error(t, err)
		assert.Nil(t, wt)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestAgentWorktreeManager_Remove(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx := context.Background()
		baseDir := filepath.Join(repoDir, "workspaces")

		// Create worktree
		wt, err := manager.Create(ctx, "coder-123", baseDir)
		require.NoError(t, err)

		worktreePath := wt.Path()
		branchName := wt.BranchName()

		// Verify worktree exists
		_, err = os.Stat(worktreePath)
		require.NoError(t, err)

		// Remove worktree
		err = manager.Remove(ctx, worktreePath)
		require.NoError(t, err)

		// Verify worktree directory is removed
		_, err = os.Stat(worktreePath)
		assert.True(t, os.IsNotExist(err), "worktree directory should be removed")

		// Verify branch is removed
		cmd := exec.Command("git", "branch", "--list", branchName)
		cmd.Dir = repoDir
		output, err := cmd.Output()
		require.NoError(t, err)
		assert.NotContains(t, string(output), branchName, "branch should be removed")
	})

	t.Run("EmptyPath", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx := context.Background()
		err = manager.Remove(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "worktreePath")
	})

	t.Run("NonexistentWorktree", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx := context.Background()
		nonexistentPath := filepath.Join(repoDir, "nonexistent")

		// Should not error for idempotent removal
		err = manager.Remove(ctx, nonexistentPath)
		assert.NoError(t, err, "removing nonexistent worktree should be idempotent")
	})

	t.Run("ContextCanceled", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		worktreePath := filepath.Join(repoDir, "some-path")
		err = manager.Remove(ctx, worktreePath)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestAgentWorktreeManager_List(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx := context.Background()
		baseDir := filepath.Join(repoDir, "workspaces")

		// Create multiple worktrees
		wt1, err := manager.Create(ctx, "coder-1", baseDir)
		require.NoError(t, err)

		// Give a small delay to ensure unique timestamps
		time.Sleep(10 * time.Millisecond)

		wt2, err := manager.Create(ctx, "reviewer-2", baseDir)
		require.NoError(t, err)

		// List worktrees
		worktrees, err := manager.List(ctx)
		require.NoError(t, err)

		// Should find both agent worktrees (not main worktree)
		assert.Len(t, worktrees, 2)

		// Verify worktrees are in the list
		// Note: Need to resolve symlinks for comparison (macOS /tmp -> /private/tmp)
		paths := make(map[string]bool)
		for _, wt := range worktrees {
			resolvedPath, _ := filepath.EvalSymlinks(wt.Path())
			if resolvedPath == "" {
				resolvedPath = wt.Path()
			}
			paths[resolvedPath] = true
		}

		resolved1, _ := filepath.EvalSymlinks(wt1.Path())
		if resolved1 == "" {
			resolved1 = wt1.Path()
		}
		resolved2, _ := filepath.EvalSymlinks(wt2.Path())
		if resolved2 == "" {
			resolved2 = wt2.Path()
		}

		assert.True(t, paths[resolved1], "wt1 path should be in list: %s (resolved: %s)", wt1.Path(), resolved1)
		assert.True(t, paths[resolved2], "wt2 path should be in list: %s (resolved: %s)", wt2.Path(), resolved2)
	})

	t.Run("EmptyList", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx := context.Background()
		worktrees, err := manager.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, worktrees)
	})

	t.Run("ContextCanceled", func(t *testing.T) {
		repoDir := setupTestRepo(t)
		manager, err := NewAgentWorktreeManager(repoDir)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		worktrees, err := manager.List(ctx)
		assert.Error(t, err)
		assert.Nil(t, worktrees)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestAgentWorktree_Accessors(t *testing.T) {
	wt := &AgentWorktree{
		path:       "/path/to/worktree",
		branchName: "agent-coder-123",
		createdAt:  time.Now(),
	}

	assert.Equal(t, "/path/to/worktree", wt.Path())
	assert.Equal(t, "agent-coder-123", wt.BranchName())
	assert.False(t, wt.CreatedAt().IsZero())
}
