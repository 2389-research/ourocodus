package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	stoptui "github.com/2389-research/ourocodus/cmd/agentd/internal/tui/stop"
	"github.com/2389-research/ourocodus/pkg/labels"
	tea "github.com/charmbracelet/bubbletea"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

var (
	stopJSON  bool
	stopPlain bool
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
  agentd stop alice  # Returns success even if already stopped

  # JSON output for scripting
  agentd stop alice --json

  # Plain text output (no colors)
  agentd stop alice --plain`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStop,
}

func init() {
	stopCmd.Flags().BoolVar(&stopJSON, "json", false, "Output in JSON format")
	stopCmd.Flags().BoolVar(&stopPlain, "plain", false, "Output in plain text (no colors)")
}

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

	// Detect output mode
	shouldPlain := detect.ShouldUsePlainMode(stopJSON, stopPlain, os.Environ)
	mode := output.DetectMode(stopJSON, stopPlain, shouldPlain)

	// Use TUI for rich mode
	if mode.IsRich() {
		return runStopTUI(ctx, args)
	}

	// Non-TUI mode (JSON or plain)
	return runStopLegacy(ctx, args, mode)
}

// runStopTUI runs the stop command with a Bubble Tea TUI.
func runStopTUI(ctx context.Context, agentIDs []string) error {
	m := stoptui.New(agentIDs)
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
		return fmt.Errorf("one or more agents failed to stop")
	}

	return nil
}

// runStopLegacy runs the stop command without TUI (JSON or plain mode).
func runStopLegacy(ctx context.Context, agentIDs []string, mode output.Mode) error {
	// Collect results for JSON output
	results := make([]StopResult, 0, len(agentIDs))
	allSucceeded := true

	for _, agentID := range agentIDs {
		result := stopAgentWithMode(ctx, agentID, mode)
		results = append(results, result)
		if result.Status == "failed" {
			allSucceeded = false
		}
	}

	// Output results based on mode
	if mode.IsJSON() {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(StopResults{
			Results: results,
			Success: allSucceeded,
		})
	}

	if !allSucceeded {
		return fmt.Errorf("one or more agents failed to stop")
	}

	return nil
}

// stopAgentWithMode stops an agent and returns a StopResult, with output based on mode
func stopAgentWithMode(ctx context.Context, agentID string, mode output.Mode) StopResult {
	result := StopResult{AgentID: agentID}
	printStopProgress(agentID, mode)

	// Find the container
	containerID, workspacePath, err := findAgentContainer(ctx, agentID)
	if err != nil {
		return setStopFailed(&result, fmt.Sprintf("failed to find agent: %v", err), mode)
	}
	result.ContainerID = containerID
	result.WorkspacePath = workspacePath

	if containerID == "" {
		result.Status = "not_found"
		printIfNotJSON(mode, func() { printStopNotFound(agentID, mode) })
		return result
	}

	printIfNotJSON(mode, func() { fmt.Println() })

	// Stop the container
	if err := stopContainer(ctx, containerID); err != nil {
		return setStopFailed(&result, fmt.Sprintf("failed to stop container: %v", err), mode)
	}
	printIfNotJSON(mode, func() { printStopContainerSuccess(containerID, mode) })

	// Remove the worktree if it exists
	cleanupWorktree(workspacePath, mode)

	result.Status = "stopped"
	printIfNotJSON(mode, func() {
		printStopCleanupSuccess(agentID, mode)
		fmt.Println()
	})
	return result
}

// printStopProgress prints the initial stopping message for plain mode.
func printStopProgress(agentID string, mode output.Mode) {
	if mode.IsJSON() {
		return
	}
	fmt.Printf("Stopping agent '%s'...\n", agentID)
}

// setStopFailed sets the result to failed status with error message and prints if needed.
func setStopFailed(result *StopResult, errMsg string, mode output.Mode) StopResult {
	result.Status = "failed"
	result.Error = errMsg
	printIfNotJSON(mode, func() { printError(errMsg) })
	return *result
}

// cleanupWorktree removes the worktree if it exists, logging warnings on failure.
func cleanupWorktree(workspacePath string, mode output.Mode) {
	if workspacePath == "" {
		return
	}
	if err := removeWorktree(context.Background(), workspacePath); err != nil {
		printIfNotJSON(mode, func() {
			fmt.Printf("Warning: failed to remove worktree: %v\n", err)
		})
	} else {
		printIfNotJSON(mode, func() { printStopWorktreeSuccess(workspacePath, mode) })
	}
}

// printIfNotJSON executes fn only if mode is not JSON.
func printIfNotJSON(mode output.Mode, fn func()) {
	if !mode.IsJSON() {
		fn()
	}
}

// printStopNotFound prints message when agent is not found
func printStopNotFound(agentID string, _ output.Mode) {
	fmt.Printf("Agent '%s' not found (already stopped)\n", agentID)
}

// printStopContainerSuccess prints container stop success
func printStopContainerSuccess(containerID string, _ output.Mode) {
	shortID := output.FormatContainerID(containerID)
	fmt.Printf("Stopped container %s\n", shortID)
}

// printStopWorktreeSuccess prints worktree removal success
func printStopWorktreeSuccess(workspacePath string, _ output.Mode) {
	displayPath := workspacePath
	if len(displayPath) > 60 {
		displayPath = "..." + displayPath[len(displayPath)-57:]
	}
	fmt.Printf("Removed worktree %s\n", displayPath)
}

// printStopCleanupSuccess prints final cleanup success
func printStopCleanupSuccess(agentID string, _ output.Mode) {
	fmt.Printf("Agent %s stopped and cleaned up\n", agentID)
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
	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	// Stop the container with grace period
	timeout := 5
	stopCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout+3)*time.Second)
	defer cancel()
	if err := cli.ContainerStop(stopCtx, containerID, container.StopOptions{
		Timeout: &timeout,
	}); err != nil {
		// If it's already stopped or gone, treat as non-fatal for idempotence
		if !strings.Contains(err.Error(), "is not running") && !cerrdefs.IsNotFound(err) {
			return err
		}
	}

	// Wait briefly for container to stop; if not, send SIGKILL
	if err := waitForStop(ctx, cli, containerID, 7*time.Second); err != nil {
		killCtx, killCancel := context.WithTimeout(ctx, 3*time.Second)
		defer killCancel()
		if killErr := cli.ContainerKill(killCtx, containerID, "SIGKILL"); killErr != nil {
			return fmt.Errorf("failed to stop container gracefully (%w) and kill failed (%v)", err, killErr)
		}
		if finalErr := waitForStop(ctx, cli, containerID, 3*time.Second); finalErr != nil {
			return fmt.Errorf("container did not exit after SIGKILL: %w", finalErr)
		}
		return fmt.Errorf("container stop exceeded timeout; SIGKILL issued: %w", err)
	}

	// Remove the container to clean up artifacts (idempotent)
	removeCtx, removeCancel := context.WithTimeout(ctx, 5*time.Second)
	defer removeCancel()
	if err := cli.ContainerRemove(removeCtx, containerID, container.RemoveOptions{
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
func waitForStop(ctx context.Context, cli *client.Client, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		// Use a short timeout for each poll to respect context cancellation
		pollCtx, cancel := context.WithTimeout(ctx, time.Second)
		inspect, err := cli.ContainerInspect(pollCtx, containerID)
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
			return fmt.Errorf("container still running after %s", timeout)
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
	return "", fmt.Errorf("branch not found for worktree %s", workspacePath)
}

// deleteBranch deletes a git branch forcefully
func deleteBranch(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "branch", "-D", branchName)
	return cmd.Run()
}

// stopAgent is a test-compatible wrapper for stopAgentWithMode.
// The second parameter is ignored (legacy signature for tests).
func stopAgent(ctx context.Context, _ *cobra.Command, agentID string) error {
	result := stopAgentWithMode(ctx, agentID, output.ModePlain)
	if result.Status == "failed" {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}
