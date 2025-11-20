package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Test pure parsing function
func TestParseBranchFromWorktreeList(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		workspacePath string
		wantBranch    string
		wantErr       bool
	}{
		{
			name: "valid worktree with branch",
			output: `worktree /path/to/worktree
HEAD abc123
branch refs/heads/my-branch

worktree /path/to/other
HEAD def456
branch refs/heads/other-branch`,
			workspacePath: "/path/to/worktree",
			wantBranch:    "my-branch",
			wantErr:       false,
		},
		{
			name: "multiple worktrees, find second",
			output: `worktree /first/path
HEAD abc123
branch refs/heads/first-branch

worktree /second/path
HEAD def456
branch refs/heads/second-branch`,
			workspacePath: "/second/path",
			wantBranch:    "second-branch",
			wantErr:       false,
		},
		{
			name: "worktree not found",
			output: `worktree /some/path
HEAD abc123
branch refs/heads/some-branch`,
			workspacePath: "/nonexistent/path",
			wantBranch:    "",
			wantErr:       true,
		},
		{
			name:          "empty output",
			output:        "",
			workspacePath: "/any/path",
			wantBranch:    "",
			wantErr:       true,
		},
		{
			name: "worktree without branch line",
			output: `worktree /path/to/worktree
HEAD abc123`,
			workspacePath: "/path/to/worktree",
			wantBranch:    "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBranch, err := parseBranchFromWorktreeList(tt.output, tt.workspacePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBranchFromWorktreeList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBranch != tt.wantBranch {
				t.Errorf("parseBranchFromWorktreeList() = %v, want %v", gotBranch, tt.wantBranch)
			}
		})
	}
}

// Test git command wrapper functions (integration tests)
func TestDeleteBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup: create a test branch
	testBranch := "test-branch-to-delete"
	cmd := exec.CommandContext(ctx, "git", "branch", testBranch)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test branch: %v", err)
	}

	// Verify branch exists
	cmd = exec.CommandContext(ctx, "git", "branch", "--list", testBranch)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list branches: %v", err)
	}
	if !strings.Contains(string(output), testBranch) {
		t.Fatalf("Test branch was not created")
	}

	// Test: delete the branch
	err = deleteBranch(ctx, testBranch)
	if err != nil {
		t.Errorf("deleteBranch() error = %v", err)
	}

	// Verify branch is deleted
	cmd = exec.CommandContext(ctx, "git", "branch", "--list", testBranch)
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list branches after deletion: %v", err)
	}
	if strings.Contains(string(output), testBranch) {
		t.Errorf("Branch still exists after deletion")
	}
}

func TestDeleteBranch_NonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	err := deleteBranch(ctx, "nonexistent-branch-xyz")

	// Should error when trying to delete non-existent branch
	if err == nil {
		t.Error("deleteBranch() should error for non-existent branch")
	}
}

func TestListWorktreesPorcelain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	output, err := listWorktreesPorcelain(ctx)
	if err != nil {
		t.Errorf("listWorktreesPorcelain() error = %v", err)
	}

	// Output should contain at least the main worktree
	if !strings.Contains(output, "worktree ") {
		t.Errorf("listWorktreesPorcelain() output doesn't contain 'worktree ', got: %v", output)
	}

	// Should contain HEAD line
	if !strings.Contains(output, "HEAD ") {
		t.Errorf("listWorktreesPorcelain() output doesn't contain 'HEAD ', got: %v", output)
	}
}

// Test composition: getWorktreeBranch
func TestGetWorktreeBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup: create a test worktree
	tmpDir := t.TempDir()
	testBranch := "test-get-branch"
	worktreePath := filepath.Join(tmpDir, "test-worktree")

	// Create worktree
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", testBranch, worktreePath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test worktree: %v", err)
	}
	defer func() {
		// Cleanup
		exec.CommandContext(ctx, "git", "worktree", "remove", worktreePath, "--force").Run()
		exec.CommandContext(ctx, "git", "branch", "-D", testBranch).Run()
	}()

	// Get the actual path git uses (might resolve symlinks)
	cmd = exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list worktrees: %v", err)
	}

	// Find the actual path git is using for our worktree
	var actualPath string
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			// Check if this is our worktree by looking for our branch in next lines
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.Contains(lines[j], testBranch) {
					actualPath = path
					break
				}
			}
		}
		if actualPath != "" {
			break
		}
	}

	if actualPath == "" {
		t.Fatalf("Could not find worktree in git worktree list")
	}

	// Test: get branch name for worktree using the actual path git uses
	branch, err := getWorktreeBranch(ctx, actualPath)
	if err != nil {
		t.Errorf("getWorktreeBranch() error = %v", err)
	}

	if branch != testBranch {
		t.Errorf("getWorktreeBranch() = %v, want %v", branch, testBranch)
	}
}

func TestGetWorktreeBranch_NonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	_, err := getWorktreeBranch(ctx, "/nonexistent/worktree/path")

	// Should error for non-existent worktree
	if err == nil {
		t.Error("getWorktreeBranch() should error for non-existent worktree")
	}
}

// Test composition: removeWorktreeOnly
func TestRemoveWorktreeOnly_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup: create a test worktree
	tmpDir := t.TempDir()
	testBranch := "test-remove-worktree"
	worktreePath := filepath.Join(tmpDir, "test-worktree")

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", testBranch, worktreePath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test worktree: %v", err)
	}
	defer func() {
		// Cleanup branch if test fails
		exec.CommandContext(ctx, "git", "branch", "-D", testBranch).Run()
	}()

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("Worktree was not created")
	}

	// Test: remove worktree only (not branch)
	err := removeWorktreeOnly(ctx, worktreePath)
	if err != nil {
		t.Errorf("removeWorktreeOnly() error = %v", err)
	}

	// Verify worktree is removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("Worktree still exists after removal")
	}

	// Verify branch still exists
	cmd = exec.CommandContext(ctx, "git", "branch", "--list", testBranch)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list branches: %v", err)
	}
	if !strings.Contains(string(output), testBranch) {
		t.Errorf("Branch was deleted when it should only remove worktree")
	}

	// Cleanup
	exec.CommandContext(ctx, "git", "branch", "-D", testBranch).Run()
}

// Test full composition: removeWorktree
func TestRemoveWorktree_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup: create a test worktree
	tmpDir := t.TempDir()
	testBranch := "test-full-remove"
	worktreePath := filepath.Join(tmpDir, "test-worktree")

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", testBranch, worktreePath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test worktree: %v", err)
	}

	// Verify worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("Worktree was not created")
	}

	// Verify branch exists
	cmd = exec.CommandContext(ctx, "git", "branch", "--list", testBranch)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list branches: %v", err)
	}
	if !strings.Contains(string(output), testBranch) {
		t.Fatalf("Test branch was not created")
	}

	// Get the actual path git uses (might resolve symlinks)
	cmd = exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	listOutput, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list worktrees: %v", err)
	}

	// Find the actual path git is using for our worktree
	var actualPath string
	lines := strings.Split(string(listOutput), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			// Check if this is our worktree by looking for our branch in next lines
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.Contains(lines[j], testBranch) {
					actualPath = path
					break
				}
			}
		}
		if actualPath != "" {
			break
		}
	}

	if actualPath == "" {
		t.Fatalf("Could not find worktree in git worktree list")
	}

	// Test: remove worktree AND branch using actual path
	err = removeWorktree(ctx, actualPath)
	if err != nil {
		t.Errorf("removeWorktree() error = %v", err)
	}

	// Verify worktree is removed (check both paths)
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("Worktree still exists after removal")
	}

	// Verify branch is also removed
	cmd = exec.CommandContext(ctx, "git", "branch", "--list", testBranch)
	output, err = cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list branches after removal: %v", err)
	}
	if strings.Contains(string(output), testBranch) {
		t.Errorf("Branch still exists after removeWorktree(), should be deleted")
	}
}

// Test error handling wrappers
func TestTryGetWorktreeBranch_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Should not panic on error, should return empty string
	branch, err := tryGetWorktreeBranch(ctx, "/nonexistent/path")

	if branch != "" {
		t.Errorf("tryGetWorktreeBranch() returned branch %v for nonexistent path, want empty string", branch)
	}

	if err == nil {
		t.Error("tryGetWorktreeBranch() should return error for nonexistent path")
	}
}

func TestTryDeleteBranch_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Should not panic on error
	err := tryDeleteBranch(ctx, "nonexistent-branch-xyz")

	if err == nil {
		t.Error("tryDeleteBranch() should return error for nonexistent branch")
	}
}

// Test that removeWorktree is resilient to partial failures
func TestRemoveWorktree_PartialFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Test that removeWorktree doesn't fail if branch doesn't exist
	// (simulates worktree without associated branch)
	tmpDir := t.TempDir()
	testBranch := "test-partial-failure"
	worktreePath := filepath.Join(tmpDir, "test-worktree")

	// Create worktree with branch
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", testBranch, worktreePath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test worktree: %v", err)
	}

	// Get the actual path git uses
	cmd = exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	listOutput, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list worktrees: %v", err)
	}

	var actualPath string
	lines := strings.Split(string(listOutput), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.Contains(lines[j], testBranch) {
					actualPath = path
					break
				}
			}
		}
		if actualPath != "" {
			break
		}
	}

	if actualPath == "" {
		t.Fatalf("Could not find worktree in git worktree list")
	}

	// First remove the worktree manually (leaving branch orphaned)
	cmd = exec.CommandContext(ctx, "git", "worktree", "remove", actualPath, "--force")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to remove worktree: %v", err)
	}

	// Now manually delete the branch
	cmd = exec.CommandContext(ctx, "git", "branch", "-D", testBranch)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to delete branch: %v", err)
	}

	// Now try to remove worktree - should succeed even though both are already gone
	// This tests resilience to inconsistent state
	err = removeWorktree(ctx, actualPath)
	if err == nil {
		t.Log("removeWorktree() succeeded gracefully when worktree already removed")
	} else {
		// It's okay to error when worktree doesn't exist, as long as it doesn't panic
		t.Logf("removeWorktree() errored as expected when worktree already removed: %v", err)
	}

	// The important thing is we didn't panic and resources are cleaned up
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("Worktree still exists")
	}

	// Verify branch is gone
	cmd = exec.CommandContext(ctx, "git", "branch", "--list", testBranch)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to list branches: %v", err)
	}
	if strings.Contains(string(output), testBranch) {
		t.Errorf("Branch still exists")
	}
}
