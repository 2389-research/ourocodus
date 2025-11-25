package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/spf13/cobra"
)

var (
	executeTimeout int
	executeShell   string
)

var executeCmd = &cobra.Command{
	Use:   "execute <agent-id> <command>",
	Short: "⚡ Execute a command in an agent and get the response",
	Long: `Execute a shell command inside a running agent's container and wait for the response.

This runs a command inside the agent's container and returns the output.
Useful for quick checks, running scripts, or triggering agent actions.`,
	Example: `  # Run a simple command
  agentd execute alice "ls -la /workspace"

  # Check git status
  agentd execute bob "git status"

  # Run a script
  agentd execute charlie "./scripts/build.sh"

  # Get agent process info
  agentd execute alice "ps aux | grep acp"

  # With custom shell
  agentd execute alice "echo $SHELL" --shell /bin/zsh

  # With timeout
  agentd execute alice "sleep 10" --timeout 5`,
	Args: cobra.ExactArgs(2),
	RunE: runExecute,
}

func init() {
	executeCmd.Flags().IntVar(&executeTimeout, "timeout", 30, "Command timeout in seconds")
	executeCmd.Flags().StringVar(&executeShell, "shell", "/bin/bash", "Shell to use for command execution")
}

func runExecute(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(executeTimeout)*time.Second)
	defer cancel()

	agentID := args[0]
	command := args[1]

	// Find the container
	containerID, err := findAgentContainerID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to find agent: %w", err)
	}

	if containerID == "" {
		return fmt.Errorf("agent '%s' not found", agentID)
	}

	// Check if agent is running
	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	if !inspect.State.Running {
		return fmt.Errorf("agent '%s' is not running (status: %s)", agentID, inspect.State.Status)
	}

	// Print command info
	th := theme.NewRetroTheme(theme.PaletteCGA)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Primary))).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Muted)))

	fmt.Println(headerStyle.Render(fmt.Sprintf("⚡ Executing command on agent '%s'", agentID)))
	fmt.Println(mutedStyle.Render(fmt.Sprintf("   Container: %s", output.FormatContainerID(containerID))))
	fmt.Println(mutedStyle.Render(fmt.Sprintf("   Command: %s", command)))
	fmt.Println()

	// Create exec instance
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{executeShell, "-c", command},
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach and run
	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read and display output
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Success)))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Error)))

	fmt.Println(successStyle.Render("─── Output ───"))

	// Read all output using StdCopy to demultiplex stdout/stderr
	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to read output: %w", err)
	}

	// Print stdout
	if out := stdout.String(); out != "" {
		fmt.Print(strings.TrimSuffix(out, "\n"))
		fmt.Println()
	}

	// Print stderr in red
	if errOut := stderr.String(); errOut != "" {
		fmt.Print(errorStyle.Render(strings.TrimSuffix(errOut, "\n")))
		fmt.Println()
	}

	// Check exit code
	inspectResp, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect exec: %w", err)
	}

	fmt.Println(successStyle.Render("─────────────"))

	if inspectResp.ExitCode != 0 {
		fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Command failed with exit code %d", inspectResp.ExitCode)))
		return fmt.Errorf("command failed")
	}

	fmt.Println(successStyle.Render("✓ Command completed successfully"))
	return nil
}
