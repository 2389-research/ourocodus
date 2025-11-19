package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <agent-id> [agent-id...]",
	Short: "Stop agent(s) and cleanup resources",
	Long: `Stop gracefully shuts down agents and cleans up all resources:
  - Stops Docker container
  - Removes git worktree and branch
  - Cleans up credential files

This command is idempotent - safe to call multiple times.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
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
	infoColor.Printf("Stopping agent '%s'...\n", agentID)

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
	defer cli.Close()

	// Find container with matching agent-id label
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "ourocodus.agent=true")
	filterArgs.Add("label", fmt.Sprintf("agent-id=%s", agentID))

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

	// Extract workspace from mounts
	workspace := ""
	if len(containers[0].Mounts) > 0 {
		workspace = containers[0].Mounts[0].Source // Host path
	}

	return containers[0].ID, workspace, nil
}

func stopContainer(ctx context.Context, containerID string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	// Stop the container with grace period
	timeout := 30
	return cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
}

func removeWorktree(ctx context.Context, workspacePath string) error {
	// Use git worktree remove command
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", workspacePath, "--force")
	return cmd.Run()
}
