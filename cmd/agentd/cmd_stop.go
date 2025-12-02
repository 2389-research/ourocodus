package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	stoptui "github.com/2389-research/ourocodus/cmd/agentd/internal/tui/stop"
	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/cli/format"
	"github.com/2389-research/ourocodus/pkg/cli/output"
	"github.com/2389-research/ourocodus/pkg/labels"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

// stop command flags removed - now using centralized pkg/cli flags

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
  agentd stop alice  # Returns success even if already stopped

  # JSON output for scripting
  agentd stop alice --json

  # Plain text output (no colors)
  agentd stop alice --plain`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStop,
}

// No init needed - flags are now centralized in pkg/cli

// StopResult represents the output of a stop operation for JSON output
type StopResult struct {
	AgentID       string `json:"agentId"`
	ContainerID   string `json:"containerId,omitempty"`
	WorkspacePath string `json:"workspacePath,omitempty"`
	Status        string `json:"status"` // "stopped", "not_found", "failed"
	Error         string `json:"error,omitempty"`
}

// StopResults represents multiple stop results for JSON output
type StopResults struct {
	Results []StopResult `json:"results"`
	Success bool         `json:"success"`
}

func runStop(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Get mode from AppContext (set by cli.App wrapper)
	appCtx := cli.FromContext(ctx)
	if appCtx == nil {
		return cli.ContextError()
	}

	// Use TUI for rich mode
	if appCtx.Mode.IsRich() {
		return runStopTUI(ctx, args, appCtx)
	}

	// Non-TUI mode (JSON or plain)
	return runStopLegacy(ctx, args, appCtx)
}

// runStopTUI runs the stop command with a Bubble Tea TUI.
func runStopTUI(ctx context.Context, agentIDs []string, appCtx *cli.AppContext) error {
	m := stoptui.New(agentIDs, appCtx.Theme)
	p := tea.NewProgram(m)

	// Channel to receive final result
	resultCh := make(chan bool, 1)

	// Run stop operations in background
	go func() {
		allSucceeded := true

		for _, agentID := range agentIDs {
			p.Send(stoptui.AgentStartMsg{AgentID: agentID})

			// Step 1: Find container
			p.Send(stoptui.StepStartMsg{AgentID: agentID, Step: stoptui.StepFindContainer})
			time.Sleep(50 * time.Millisecond)

			containerID, workspacePath, err := findAgentContainer(ctx, agentID)
			if err != nil {
				p.Send(stoptui.StepErrorMsg{AgentID: agentID, Step: stoptui.StepFindContainer, Error: err})
				p.Send(stoptui.AgentCompleteMsg{AgentID: agentID, Status: "failed"})
				allSucceeded = false
				continue
			}

			if containerID == "" {
				p.Send(stoptui.StepSkipMsg{AgentID: agentID, Step: stoptui.StepFindContainer, Reason: "not found"})
				p.Send(stoptui.StepSkipMsg{AgentID: agentID, Step: stoptui.StepStopContainer, Reason: "no container"})
				p.Send(stoptui.StepSkipMsg{AgentID: agentID, Step: stoptui.StepRemoveWorktree, Reason: "no worktree"})
				p.Send(stoptui.AgentCompleteMsg{AgentID: agentID, Status: "not_found"})
				continue
			}

			p.Send(stoptui.StepCompleteMsg{AgentID: agentID, Step: stoptui.StepFindContainer, ContainerID: containerID, Workspace: workspacePath})

			// Step 2: Stop container
			p.Send(stoptui.StepStartMsg{AgentID: agentID, Step: stoptui.StepStopContainer})

			if err := stopContainer(ctx, containerID); err != nil {
				p.Send(stoptui.StepErrorMsg{AgentID: agentID, Step: stoptui.StepStopContainer, Error: err})
				p.Send(stoptui.AgentCompleteMsg{AgentID: agentID, Status: "failed"})
				allSucceeded = false
				continue
			}
			p.Send(stoptui.StepCompleteMsg{AgentID: agentID, Step: stoptui.StepStopContainer, ContainerID: "", Workspace: ""})

			// Step 3: Remove worktree
			p.Send(stoptui.StepStartMsg{AgentID: agentID, Step: stoptui.StepRemoveWorktree})

			if workspacePath == "" {
				p.Send(stoptui.StepSkipMsg{AgentID: agentID, Step: stoptui.StepRemoveWorktree, Reason: "no worktree"})
			} else if err := removeWorktree(ctx, workspacePath); err != nil {
				// Non-fatal warning
				p.Send(stoptui.StepCompleteMsg{AgentID: agentID, Step: stoptui.StepRemoveWorktree, ContainerID: "", Workspace: ""})
			} else {
				p.Send(stoptui.StepCompleteMsg{AgentID: agentID, Step: stoptui.StepRemoveWorktree, ContainerID: "", Workspace: ""})
			}

			p.Send(stoptui.AgentCompleteMsg{AgentID: agentID, Status: "stopped"})
		}

		p.Send(stoptui.AllCompleteMsg{})
		resultCh <- allSucceeded
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return err
	}

	// Get result
	allSucceeded := <-resultCh
	if !allSucceeded {
		return cli.IOError("one or more agents failed to stop")
	}

	return nil
}

// runStopLegacy runs the stop command without TUI (JSON or plain mode).
func runStopLegacy(ctx context.Context, agentIDs []string, appCtx *cli.AppContext) error {
	// Collect results for JSON output
	results := make([]StopResult, 0, len(agentIDs))
	allSucceeded := true

	for _, agentID := range agentIDs {
		result := stopAgentWithMode(ctx, agentID, appCtx)
		results = append(results, result)
		if result.Status == "failed" {
			allSucceeded = false
		}
	}

	// Output results based on mode
	if appCtx.Mode.IsJSON() {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(StopResults{
			Results: results,
			Success: allSucceeded,
		})
	}

	if !allSucceeded {
		return cli.IOError("one or more agents failed to stop")
	}

	return nil
}

// stopAgentWithMode stops an agent and returns a StopResult, with output based on mode
func stopAgentWithMode(ctx context.Context, agentID string, appCtx *cli.AppContext) StopResult {
	result := StopResult{AgentID: agentID}

	// Print progress message
	if !appCtx.Mode.IsJSON() {
		appCtx.Output.Info(fmt.Sprintf("Stopping agent '%s'...", agentID))
	}

	// Find the container
	containerID, workspacePath, err := findAgentContainer(ctx, agentID)
	if err != nil {
		return setStopFailed(&result, fmt.Sprintf("failed to find agent: %v", err), appCtx)
	}
	result.ContainerID = containerID
	result.WorkspacePath = workspacePath

	if containerID == "" {
		result.Status = "not_found"
		if !appCtx.Mode.IsJSON() {
			appCtx.Output.Warning(fmt.Sprintf("Agent '%s' not found (already stopped)", agentID))
		}
		return result
	}

	// Stop the container
	if err := stopContainer(ctx, containerID); err != nil {
		return setStopFailed(&result, fmt.Sprintf("failed to stop container: %v", err), appCtx)
	}
	if !appCtx.Mode.IsJSON() {
		appCtx.Output.Success(fmt.Sprintf("Stopped container %s", format.FormatContainerID(containerID)))
	}

	// Remove the worktree if it exists
	cleanupWorktree(workspacePath, appCtx)

	result.Status = "stopped"
	if !appCtx.Mode.IsJSON() {
		appCtx.Output.Success(fmt.Sprintf("Agent %s stopped and cleaned up", agentID))
	}
	return result
}

// setStopFailed sets the result to failed status with error message and prints if needed.
func setStopFailed(result *StopResult, errMsg string, appCtx *cli.AppContext) StopResult {
	result.Status = "failed"
	result.Error = errMsg
	if !appCtx.Mode.IsJSON() {
		appCtx.Output.Error(cli.IOError(errMsg))
	}
	return *result
}

// printIfNotJSON executes fn only if mode is not JSON.
// TODO: Remove this once all commands migrate to Output interface.
func printIfNotJSON(mode cli.Mode, fn func()) {
	if !mode.IsJSON() {
		fn()
	}
}

// cleanupWorktree removes the worktree if it exists, logging warnings on failure.
func cleanupWorktree(workspacePath string, appCtx *cli.AppContext) {
	if workspacePath == "" {
		return
	}
	if err := removeWorktree(context.Background(), workspacePath); err != nil {
		if !appCtx.Mode.IsJSON() {
			appCtx.Output.Warning(fmt.Sprintf("Failed to remove worktree: %v", err))
		}
	} else {
		if !appCtx.Mode.IsJSON() {
			displayPath := workspacePath
			if len(displayPath) > 60 {
				displayPath = "..." + displayPath[len(displayPath)-57:]
			}
			appCtx.Output.Success(fmt.Sprintf("Removed worktree %s", displayPath))
		}
	}
}

func findAgentContainer(ctx context.Context, agentID string) (string, string, error) {
	cli, err := newDockerClient()
	if err != nil {
		return "", "", err
	}
	defer func() { _ = cli.Close() }()

	// Use centralized filter builder from pkg/labels
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: labels.FindAgentFilter(agentID),
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
	dockerCli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer func() { _ = dockerCli.Close() }()

	// Stop the container with grace period
	timeout := 5
	stopCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout+3)*time.Second)
	defer cancel()
	if err := dockerCli.ContainerStop(stopCtx, containerID, container.StopOptions{
		Timeout: &timeout,
	}); err != nil {
		// If it's already stopped or gone, treat as non-fatal for idempotence
		if !strings.Contains(err.Error(), "is not running") && !cerrdefs.IsNotFound(err) {
			return err
		}
	}

	// Wait briefly for container to stop; if not, send SIGKILL
	if err := waitForStop(ctx, dockerCli, containerID, 7*time.Second); err != nil {
		killCtx, killCancel := context.WithTimeout(ctx, 3*time.Second)
		defer killCancel()
		if killErr := dockerCli.ContainerKill(killCtx, containerID, "SIGKILL"); killErr != nil {
			return cli.IOError(fmt.Sprintf("failed to stop container gracefully (%v) and kill failed (%v)", err, killErr))
		}
		if finalErr := waitForStop(ctx, dockerCli, containerID, 3*time.Second); finalErr != nil {
			return cli.IOError("container did not exit after SIGKILL: " + finalErr.Error())
		}
		return cli.IOError("container stop exceeded timeout; SIGKILL issued: " + err.Error())
	}

	// Remove the container to clean up artifacts (idempotent)
	removeCtx, removeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer removeCancel()
	if err := dockerCli.ContainerRemove(removeCtx, containerID, container.RemoveOptions{
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

// waitForStop polls Docker until the container is not running or timeout elapses.
func waitForStop(ctx context.Context, dockerCli *client.Client, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		// Use a short timeout for each poll to respect context cancellation
		pollCtx, cancel := context.WithTimeout(ctx, time.Second)
		inspect, err := dockerCli.ContainerInspect(pollCtx, containerID)
		cancel()

		if err == nil && !inspect.State.Running {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return cli.IOError(fmt.Sprintf("container still running after %s", timeout))
		}
		time.Sleep(200 * time.Millisecond)
	}
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
	cmd.Stderr = os.Stderr // Capture stderr for debugging
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
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
	return "", cli.IOError("branch not found for worktree " + workspacePath)
}

// deleteBranch deletes a git branch forcefully
func deleteBranch(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "branch", "-D", branchName)
	return cmd.Run()
}

// stopAgent is a test-compatible wrapper for stopAgentWithMode.
// The second parameter is ignored (legacy signature for tests).
// Uses plain mode with default theme since callers don't have AppContext access.
func stopAgent(ctx context.Context, _ *cobra.Command, agentID string) error {
	// Get AppContext from context if available, otherwise create a minimal one
	appCtx := cli.FromContext(ctx)
	if appCtx == nil {
		th := theme.Ensure(nil)
		appCtx = &cli.AppContext{
			Mode:   cli.ModePlain,
			Theme:  th,
			Output: output.NewPlainOutput(false),
		}
	}
	result := stopAgentWithMode(ctx, agentID, appCtx)
	if result.Status == "failed" {
		return cli.IOError(result.Error)
	}
	return nil
}
