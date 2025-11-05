//go:build integration

package worktree_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// setupIntegrationRepo creates a temporary git repository for integration testing.
func setupIntegrationRepo(t *testing.T) string {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "worktree-integration-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Configure git for tests
	cmd = exec.Command("git", "config", "user.name", "Integration Test")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "config", "user.email", "integration@test.example.com")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	// Create initial commit (required for worktrees)
	dummyFile := filepath.Join(tmpDir, "README.md")
	err = os.WriteFile(dummyFile, []byte("# Integration Test Repo\n"), 0o644)
	require.NoError(t, err)

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	require.NoError(t, cmd.Run())

	return tmpDir
}

// TestIntegration_CreateAndRemoveWorktree tests the full lifecycle of a worktree.
func TestIntegration_CreateAndRemoveWorktree(t *testing.T) {
	repoDir := setupIntegrationRepo(t)
	manager, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	ctx := context.Background()
	baseDir := filepath.Join(repoDir, "workspaces")

	// Create worktree
	wt, err := manager.Create(ctx, "agent-test-1", baseDir)
	require.NoError(t, err)
	require.NotNil(t, wt)

	// Verify worktree directory exists
	_, err = os.Stat(wt.Path())
	assert.NoError(t, err, "Worktree directory should exist")

	// Verify README.md from main branch is present
	readmePath := filepath.Join(wt.Path(), "README.md")
	_, err = os.Stat(readmePath)
	assert.NoError(t, err, "README.md should exist in worktree")

	// Verify branch exists in git
	cmd := exec.Command("git", "branch", "--list", wt.BranchName())
	cmd.Dir = repoDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(output), wt.BranchName(), "Branch should exist")

	// Create a file in the worktree
	testFile := filepath.Join(wt.Path(), "test-file.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0o644)
	require.NoError(t, err)

	// Verify file is visible in worktree
	_, err = os.Stat(testFile)
	assert.NoError(t, err, "Test file should exist in worktree")

	// Remove worktree
	err = manager.Remove(ctx, wt.Path())
	require.NoError(t, err)

	// Verify worktree directory is removed
	_, err = os.Stat(wt.Path())
	assert.True(t, os.IsNotExist(err), "Worktree directory should be removed")

	// Verify branch is removed
	cmd = exec.Command("git", "branch", "--list", wt.BranchName())
	cmd.Dir = repoDir
	output, err = cmd.Output()
	require.NoError(t, err)
	assert.NotContains(t, string(output), wt.BranchName(), "Branch should be removed")
}

// TestIntegration_ConcurrentWorktrees tests creating multiple worktrees concurrently.
func TestIntegration_ConcurrentWorktrees(t *testing.T) {
	repoDir := setupIntegrationRepo(t)
	manager, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	ctx := context.Background()
	baseDir := filepath.Join(repoDir, "workspaces")

	const numWorktrees = 5
	worktrees := make([]*worktree.AgentWorktree, numWorktrees)

	// Use errgroup to properly handle errors from goroutines
	g, gctx := errgroup.WithContext(ctx)

	// Create worktrees concurrently
	for i := 0; i < numWorktrees; i++ {
		index := i // Capture loop variable
		g.Go(func() error {
			// Small sleep to ensure unique timestamps
			time.Sleep(time.Duration(index) * 10 * time.Millisecond)

			wt, err := manager.Create(gctx, "concurrent-agent-"+string(rune('A'+index)), baseDir)
			if err != nil {
				return err
			}
			worktrees[index] = wt
			t.Logf("Created worktree %d: %s (branch: %s)", index, wt.Path(), wt.BranchName())
			return nil
		})
	}

	// Wait for all goroutines and check for errors
	require.NoError(t, g.Wait(), "All worktrees should be created successfully")

	// Verify all worktrees exist
	for i, wt := range worktrees {
		assert.NotNil(t, wt, "Worktree %d should exist", i)
		_, err := os.Stat(wt.Path())
		assert.NoError(t, err, "Worktree %d directory should exist", i)
	}

	// Verify List() returns all worktrees
	list, err := manager.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, numWorktrees, "List should return all worktrees")

	// Cleanup: Remove all worktrees
	for i, wt := range worktrees {
		err := manager.Remove(ctx, wt.Path())
		assert.NoError(t, err, "Worktree %d should be removed cleanly", i)
	}

	// Verify all are removed
	list, err = manager.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list, "List should be empty after cleanup")
}

// TestIntegration_WorktreeIsolation tests that worktrees are isolated from each other.
func TestIntegration_WorktreeIsolation(t *testing.T) {
	repoDir := setupIntegrationRepo(t)
	manager, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	ctx := context.Background()
	baseDir := filepath.Join(repoDir, "workspaces")

	// Create two worktrees
	wt1, err := manager.Create(ctx, "agent-isolation-1", baseDir)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // Ensure unique timestamps

	wt2, err := manager.Create(ctx, "agent-isolation-2", baseDir)
	require.NoError(t, err)

	defer func() {
		_ = manager.Remove(ctx, wt1.Path())
		_ = manager.Remove(ctx, wt2.Path())
	}()

	// Create different files in each worktree
	file1 := filepath.Join(wt1.Path(), "agent1-file.txt")
	err = os.WriteFile(file1, []byte("agent 1 data"), 0o644)
	require.NoError(t, err)

	file2 := filepath.Join(wt2.Path(), "agent2-file.txt")
	err = os.WriteFile(file2, []byte("agent 2 data"), 0o644)
	require.NoError(t, err)

	// Verify file1 exists in wt1 but not in wt2
	_, err = os.Stat(file1)
	assert.NoError(t, err, "agent1-file.txt should exist in wt1")

	file1InWt2 := filepath.Join(wt2.Path(), "agent1-file.txt")
	_, err = os.Stat(file1InWt2)
	assert.True(t, os.IsNotExist(err), "agent1-file.txt should not exist in wt2")

	// Verify file2 exists in wt2 but not in wt1
	_, err = os.Stat(file2)
	assert.NoError(t, err, "agent2-file.txt should exist in wt2")

	file2InWt1 := filepath.Join(wt1.Path(), "agent2-file.txt")
	_, err = os.Stat(file2InWt1)
	assert.True(t, os.IsNotExist(err), "agent2-file.txt should not exist in wt1")

	// Verify they're on different branches
	assert.NotEqual(t, wt1.BranchName(), wt2.BranchName(), "Each worktree should have a unique branch")
}

// TestIntegration_WorktreeGitOperations tests git operations within a worktree.
func TestIntegration_WorktreeGitOperations(t *testing.T) {
	repoDir := setupIntegrationRepo(t)
	manager, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	ctx := context.Background()
	baseDir := filepath.Join(repoDir, "workspaces")

	// Create worktree
	wt, err := manager.Create(ctx, "agent-git-ops", baseDir)
	require.NoError(t, err)
	defer func() {
		_ = manager.Remove(ctx, wt.Path())
	}()

	// Create a new file in the worktree
	newFile := filepath.Join(wt.Path(), "new-feature.txt")
	err = os.WriteFile(newFile, []byte("new feature content"), 0o644)
	require.NoError(t, err)

	// Stage the file
	cmd := exec.Command("git", "add", "new-feature.txt")
	cmd.Dir = wt.Path()
	require.NoError(t, cmd.Run())

	// Commit the file
	cmd = exec.Command("git", "commit", "-m", "Add new feature")
	cmd.Dir = wt.Path()
	require.NoError(t, cmd.Run())

	// Verify commit exists in the worktree's branch
	cmd = exec.Command("git", "log", "--oneline", wt.BranchName())
	cmd.Dir = repoDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(output), "Add new feature", "Commit should exist in worktree branch")

	// Verify commit does NOT exist in main branch
	cmd = exec.Command("git", "log", "--oneline", "main")
	cmd.Dir = repoDir
	output, err = cmd.Output()
	require.NoError(t, err)
	assert.NotContains(t, string(output), "Add new feature", "Commit should not exist in main branch")
}

// TestIntegration_RemoveIdempotent tests that Remove() is safe to call multiple times.
func TestIntegration_RemoveIdempotent(t *testing.T) {
	repoDir := setupIntegrationRepo(t)
	manager, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	ctx := context.Background()
	baseDir := filepath.Join(repoDir, "workspaces")

	// Create worktree
	wt, err := manager.Create(ctx, "agent-idempotent", baseDir)
	require.NoError(t, err)

	worktreePath := wt.Path()

	// Remove once
	err = manager.Remove(ctx, worktreePath)
	require.NoError(t, err)

	// Verify removed
	_, err = os.Stat(worktreePath)
	assert.True(t, os.IsNotExist(err), "Worktree should be removed")

	// Remove again - should not error (idempotent)
	err = manager.Remove(ctx, worktreePath)
	assert.NoError(t, err, "Second Remove() should be idempotent")

	// Remove third time - still should not error
	err = manager.Remove(ctx, worktreePath)
	assert.NoError(t, err, "Third Remove() should be idempotent")
}

// TestIntegration_ListOnlyAgentWorktrees tests that List() only returns agent worktrees.
func TestIntegration_ListOnlyAgentWorktrees(t *testing.T) {
	repoDir := setupIntegrationRepo(t)
	manager, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	ctx := context.Background()
	baseDir := filepath.Join(repoDir, "workspaces")

	// Create agent worktrees
	wt1, err := manager.Create(ctx, "agent-list-1", baseDir)
	require.NoError(t, err)
	defer func() {
		_ = manager.Remove(ctx, wt1.Path())
	}()

	time.Sleep(10 * time.Millisecond)

	wt2, err := manager.Create(ctx, "agent-list-2", baseDir)
	require.NoError(t, err)
	defer func() {
		_ = manager.Remove(ctx, wt2.Path())
	}()

	// Create a non-agent worktree manually
	nonAgentPath := filepath.Join(baseDir, "non-agent-worktree")
	cmd := exec.Command("git", "worktree", "add", "-b", "feature-branch", nonAgentPath)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	defer func() {
		_ = exec.Command("git", "worktree", "remove", nonAgentPath, "--force").Run()
		_ = exec.Command("git", "branch", "-D", "feature-branch").Run()
	}()

	// List worktrees
	list, err := manager.List(ctx)
	require.NoError(t, err)

	// Should only return agent worktrees (branches starting with "agent-")
	assert.Len(t, list, 2, "List should return only agent worktrees")

	// Verify the returned worktrees are our agent worktrees
	foundWt1 := false
	foundWt2 := false
	for _, wt := range list {
		if wt.BranchName() == wt1.BranchName() {
			foundWt1 = true
		}
		if wt.BranchName() == wt2.BranchName() {
			foundWt2 = true
		}
		// Verify all returned worktrees have "agent-" prefix
		assert.True(t, len(wt.BranchName()) > 6 && wt.BranchName()[:6] == "agent-",
			"All listed worktrees should have 'agent-' prefix, got: %s", wt.BranchName())
	}

	assert.True(t, foundWt1, "wt1 should be in list")
	assert.True(t, foundWt2, "wt2 should be in list")
}

// TestIntegration_ContextCancellation tests context cancellation handling.
func TestIntegration_ContextCancellation(t *testing.T) {
	repoDir := setupIntegrationRepo(t)
	manager, err := worktree.NewAgentWorktreeManager(repoDir)
	require.NoError(t, err)

	baseDir := filepath.Join(repoDir, "workspaces")

	t.Run("CancelCreate", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		wt, err := manager.Create(ctx, "agent-cancel-create", baseDir)
		assert.Error(t, err)
		assert.Nil(t, wt)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("CancelRemove", func(t *testing.T) {
		// Create with valid context
		wt, err := manager.Create(context.Background(), "agent-cancel-remove", baseDir)
		require.NoError(t, err)
		defer func() {
			_ = manager.Remove(context.Background(), wt.Path())
		}()

		// Cancel context before remove
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = manager.Remove(ctx, wt.Path())
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("CancelList", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		list, err := manager.List(ctx)
		assert.Error(t, err)
		assert.Nil(t, list)
		assert.Equal(t, context.Canceled, err)
	})
}
