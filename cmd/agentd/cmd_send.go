package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	sendTimeout int
	sendShell   string
)

var sendCmd = &cobra.Command{
	Use:   "send <agent-id> <command>",
	Short: "💬 Send a command to an agent and get the response",
	Long: `Send a shell command to a running agent and wait for the response.

This executes a command inside the agent's container and returns the output.
Useful for quick checks, running scripts, or triggering agent actions.`,
	Example: `  # Run a simple command
  agentd send alice "ls -la /workspace"

  # Check git status
  agentd send bob "git status"

  # Run a script
  agentd send charlie "./scripts/build.sh"

  # Get agent process info
  agentd send alice "ps aux | grep acp"

  # With custom shell
  agentd send alice "echo $SHELL" --shell /bin/zsh

  # With timeout
  agentd send alice "sleep 10" --timeout 5`,
	Args: cobra.ExactArgs(2),
	RunE: runSend,
}

func init() {
	sendCmd.Flags().IntVar(&sendTimeout, "timeout", 30, "Command timeout in seconds")
	sendCmd.Flags().StringVar(&sendShell, "shell", "/bin/bash", "Shell to use for command execution")
}

func runSend(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sendTimeout)*time.Second)
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
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	if !inspect.State.Running {
		return fmt.Errorf("agent '%s' is not running (status: %s)", agentID, inspect.State.Status)
	}

	// Print command info
	headerColor := color.New(color.FgCyan, color.Bold)
	headerColor.Printf("💬 Sending command to agent '%s'\n", agentID)
	color.New(color.FgHiBlack).Printf("   Container: %s\n", formatContainerID(containerID))
	color.New(color.FgHiBlack).Printf("   Command: %s\n\n", command)

	// Create exec instance
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{sendShell, "-c", command},
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
	color.New(color.FgGreen).Println("─── Output ───")

	// Read all output using StdCopy to demultiplex stdout/stderr
	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		// Ignore EOF
		if err.Error() != "EOF" {
			return fmt.Errorf("failed to read output: %w", err)
		}
	}

	// Print stdout
	if out := stdout.String(); out != "" {
		fmt.Print(strings.TrimSuffix(out, "\n"))
		fmt.Println()
	}

	// Print stderr in red
	if errOut := stderr.String(); errOut != "" {
		color.New(color.FgRed).Print(strings.TrimSuffix(errOut, "\n"))
		fmt.Println()
	}

	// Check exit code
	inspectResp, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect exec: %w", err)
	}

	color.New(color.FgGreen).Println("─────────────")

	if inspectResp.ExitCode != 0 {
		color.New(color.FgRed).Printf("✗ Command failed with exit code %d\n", inspectResp.ExitCode)
		return fmt.Errorf("command failed")
	}

	color.New(color.FgGreen).Println("✓ Command completed successfully")
	return nil
}
