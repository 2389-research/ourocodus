package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <agent-id> [agent-id...]",
	Short: "🛑 Stop agent(s) and cleanup resources",
	Long: `Stop gracefully shuts down agents and cleans up all resources:
  - Stops Docker container (30s graceful timeout)
  - Removes git worktree and branch
  - Cleans up credential files

This command is idempotent - safe to call multiple times.`,
	Example: `  # Stop single agent
  agentd stop alice

  # Stop multiple agents
  agentd stop alice bob charlie

  # Idempotent - safe to retry
  agentd stop alice  # Returns success even if already stopped`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStop,
}

func init() {
	// Command registered in main.go
}

func runStop(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Stop each agent (no need for launcher - we query Docker directly)
	allSucceeded := true
	for _, agentID := range args {
		if err := stopAgent(ctx, nil, agentID); err != nil {
			fmt.Fprintf(os.Stderr, "\n")
			printError(fmt.Sprintf("Failed to stop '%s': %v", agentID, err))
			fmt.Fprintf(os.Stderr, "\n")
			allSucceeded = false
			continue
		}
	}

	if !allSucceeded {
		return fmt.Errorf("one or more agents failed to stop")
	}

	return nil
}

func stopAgent(ctx context.Context, _ interface{}, agentID string) error {
	_, _ = color.New(color.FgYellow, color.Bold).Printf("🛑 Stopping agent '%s'...\n", agentID)

	// First, try to find the container in Docker by agent-id label
	containerID, workspacePath, err := findAgentContainer(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to find agent: %w", err)
	}

	if containerID == "" {
		// Agent doesn't exist - this is okay (idempotent)
		printSuccess(fmt.Sprintf("Agent '%s' not found (already stopped)", agentID))
		return nil
	}

	fmt.Println()

	// Stop the container using Docker directly
	if err := stopContainer(ctx, containerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	printSuccess(fmt.Sprintf("Stopped container %s", formatContainerID(containerID)))

	// Remove the worktree if it exists
	if workspacePath != "" {
		if err := removeWorktree(ctx, workspacePath); err != nil {
			// Log warning but don't fail - worktree might already be removed
			fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree: %v\n", err)
		} else {
			printSuccess(fmt.Sprintf("Removed worktree %s", formatWorkspace(workspacePath)))
		}
	}

	printSuccess("Cleaned up agent resources")
	fmt.Println()

	return nil
}

func findAgentContainer(ctx context.Context, agentID string) (string, string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", "", err
	}
	defer func() { _ = cli.Close() }()

	// Find container with matching agent-id label
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=true", LabelNamespace))
	filterArgs.Add("label", fmt.Sprintf("%s=%s", LabelAgentID, agentID))

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return "", "", err
	}

	if len(containers) == 0 {
		return "", "", nil // Not found
	}

	// Extract workspace from mounts (look for /workspace mount, not .creds)
	workspace := ""
	for _, mnt := range containers[0].Mounts {
		if mnt.Destination == "/workspace" {
			workspace = mnt.Source // Host path
			break
		}
	}

	return containers[0].ID, workspace, nil
}

func stopContainer(ctx context.Context, containerID string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	// Stop the container with grace period
	timeout := 30
	if err := cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	}); err != nil {
		// If it's already stopped or gone, treat as non-fatal for idempotence
		if !strings.Contains(err.Error(), "is not running") && !cerrdefs.IsNotFound(err) {
			return err
		}
	}

	// Remove the container to clean up artifacts (idempotent)
	if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		// Ignore NotFound errors for idempotence
		if !cerrdefs.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// removeWorktree removes a git worktree and its associated branch
func removeWorktree(ctx context.Context, workspacePath string) error {
	branchName, _ := tryGetWorktreeBranch(ctx, workspacePath)

	if err := removeWorktreeOnly(ctx, workspacePath); err != nil {
		return err
	}

	if branchName != "" {
		_ = tryDeleteBranch(ctx, branchName)
	}

	return nil
}

// tryGetWorktreeBranch attempts to get the branch name, logging warnings on failure
func tryGetWorktreeBranch(ctx context.Context, workspacePath string) (string, error) {
	branchName, err := getWorktreeBranch(ctx, workspacePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to get worktree branch: %v\n", err)
		return "", err
	}
	return branchName, nil
}

// tryDeleteBranch attempts to delete a branch, logging warnings on failure
func tryDeleteBranch(ctx context.Context, branchName string) error {
	if err := deleteBranch(ctx, branchName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to delete branch '%s': %v\n", branchName, err)
		return err
	}
	return nil
}

// removeWorktreeOnly removes the worktree without touching the branch
func removeWorktreeOnly(ctx context.Context, workspacePath string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", workspacePath, "--force")
	return cmd.Run()
}

// getWorktreeBranch returns the branch name for a given worktree path
func getWorktreeBranch(ctx context.Context, workspacePath string) (string, error) {
	output, err := listWorktreesPorcelain(ctx)
	if err != nil {
		return "", err
	}

	return parseBranchFromWorktreeList(output, workspacePath)
}

// listWorktreesPorcelain returns the output of 'git worktree list --porcelain'
func listWorktreesPorcelain(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// parseBranchFromWorktreeList extracts the branch name for a worktree path from porcelain output
func parseBranchFromWorktreeList(output, workspacePath string) (string, error) {
	currentWorktree := ""
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			currentWorktree = strings.TrimPrefix(line, "worktree ")
		}
		if currentWorktree == workspacePath && strings.HasPrefix(line, "branch ") {
			return strings.TrimPrefix(line, "branch refs/heads/"), nil
		}
	}
	return "", fmt.Errorf("branch not found for worktree %s", workspacePath)
}

// deleteBranch deletes a git branch forcefully
func deleteBranch(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "branch", "-D", branchName)
	return cmd.Run()
}
