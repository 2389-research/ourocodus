package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var attachCmd = &cobra.Command{
	Use:   "attach <agent-id>",
	Short: "📎 Attach to an agent's interactive shell",
	Long: `Attach to a running agent's container and get an interactive shell.

This opens an interactive bash session inside the agent's container where you
can explore the workspace, run commands, and interact with the agent's environment.

Press Ctrl-D or type 'exit' to detach from the agent.`,
	Example: `  # Attach to agent alice
  agentd attach alice

  # Once attached, you can:
  #  - Explore the workspace: ls /workspace
  #  - Check agent status: ps aux
  #  - View logs: tail -f /var/log/*.log
  #  - Run git commands: git status

  # Detach: Ctrl-D or type 'exit'`,
	Args: cobra.ExactArgs(1),
	RunE: runAttach,
}

func runAttach(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	agentID := args[0]

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

	// Print attach message
	th := theme.NewRetroTheme(theme.PaletteCGA)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Primary))).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Muted)))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Warning)))

	fmt.Println()
	fmt.Println(headerStyle.Render(fmt.Sprintf("📎 Attaching to agent '%s'", agentID)))
	fmt.Println(mutedStyle.Render(fmt.Sprintf("   Container: %s", output.FormatContainerID(containerID))))
	fmt.Println(mutedStyle.Render("   Workspace: /workspace"))
	fmt.Println()
	fmt.Println(warnStyle.Render("   Press Ctrl-D or type 'exit' to detach"))
	fmt.Println()

	// Create exec instance with interactive TTY
	execConfig := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{"/bin/bash"},
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec
	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Set up raw terminal mode for interactive session
	oldState, err := setRawTerminal()
	if err != nil {
		return fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer func() { _ = restoreTerminal(oldState) }()

	// Handle signals to cleanup on interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Copy stdin/stdout/stderr
	errCh := make(chan error, 2)

	// Copy stdin to container
	go func() {
		_, err := io.Copy(resp.Conn, os.Stdin)
		errCh <- err
	}()

	// Copy container output to stdout
	go func() {
		_, err := io.Copy(os.Stdout, resp.Reader)
		errCh <- err
	}()

	// Wait for completion or signal
	select {
	case <-sigCh:
		// User interrupted - clean exit
		fmt.Println("\n\nDetached from agent")
		return nil
	case err := <-errCh:
		// Connection closed or error
		if err != nil && err != io.EOF {
			return fmt.Errorf("error during attach: %w", err)
		}
		fmt.Println("\nDetached from agent")
		return nil
	}
}

// setRawTerminal sets the terminal to raw mode and returns the old state
func setRawTerminal() (*term.State, error) {
	fd := int(os.Stdin.Fd())
	return term.MakeRaw(fd)
}

// restoreTerminal restores the terminal to its previous state
func restoreTerminal(oldState *term.State) error {
	if oldState == nil {
		return nil
	}
	fd := int(os.Stdin.Fd())
	return term.Restore(fd, oldState)
}
