package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/cli/format"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/spf13/cobra"
)

var (
	executeTimeout int
	executeShell   string
)

// ExecuteResult represents the output of an execute operation for JSON output
type ExecuteResult struct {
	AgentID     string `json:"agentId"`
	ContainerID string `json:"containerId"`
	Command     string `json:"command"`
	Shell       string `json:"shell"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exitCode"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

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
	ctx := cmd.Context()

	// Get mode from AppContext (set by cli.App wrapper)
	appCtx := cli.FromContext(ctx)
	if appCtx == nil {
		return cli.ContextError()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(executeTimeout)*time.Second)
	defer cancel()

	agentID := args[0]
	command := args[1]

	// Execute the command and collect result
	result := executeCommand(ctx, agentID, command, appCtx.Mode, appCtx.Theme)

	// Output based on mode
	if appCtx.Mode.IsJSON() {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return cli.IOError("failed to encode JSON: " + err.Error())
		}
	}

	// Return error if command failed
	if !result.Success {
		if result.Error != "" {
			// Categorize errors based on message content
			if strings.Contains(result.Error, "not found") || strings.Contains(result.Error, "not running") {
				return cli.UsageError(result.Error)
			}
			return cli.IOError(result.Error)
		}
		return cli.IOError("command failed")
	}

	return nil
}

// executeCommand executes a command in an agent container and returns the result
func executeCommand(ctx context.Context, agentID, command string, mode cli.Mode, th *theme.Theme) ExecuteResult {
	result := ExecuteResult{
		AgentID: agentID,
		Command: command,
		Shell:   executeShell,
	}

	// Find the container
	containerID, err := findAgentContainerID(ctx, agentID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to find agent: %v", err)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}

	if containerID == "" {
		result.Error = fmt.Sprintf("agent '%s' not found", agentID)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}

	result.ContainerID = containerID

	// Check if agent is running
	dockerCli, err := newDockerClient()
	if err != nil {
		result.Error = fmt.Sprintf("failed to create Docker client: %v", err)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}
	defer func() { _ = dockerCli.Close() }()

	inspect, err := dockerCli.ContainerInspect(ctx, containerID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to inspect container: %v", err)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}

	if !inspect.State.Running {
		result.Error = fmt.Sprintf("agent '%s' is not running (status: %s)", agentID, inspect.State.Status)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}

	// Print command info (non-JSON modes)
	printIfNotJSON(mode, func() {
		printExecuteHeader(agentID, containerID, command, mode, th)
	})

	// Create exec instance
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{executeShell, "-c", command},
	}

	execID, err := dockerCli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create exec: %v", err)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}

	// Attach and run
	resp, err := dockerCli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		result.Error = fmt.Sprintf("failed to attach to exec: %v", err)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}
	defer resp.Close()

	// Read all output using StdCopy to demultiplex stdout/stderr
	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil && !errors.Is(err, io.EOF) {
		result.Error = fmt.Sprintf("failed to read output: %v", err)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	// Check exit code
	inspectResp, err := dockerCli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to inspect exec: %v", err)
		printIfNotJSON(mode, func() { printError(result.Error, th) })
		return result
	}

	result.ExitCode = inspectResp.ExitCode
	result.Success = inspectResp.ExitCode == 0

	// Print output (non-JSON modes)
	printIfNotJSON(mode, func() {
		printExecuteOutput(result, mode, th)
	})

	return result
}

// printExecuteHeader prints the command execution header
func printExecuteHeader(agentID, containerID, command string, mode cli.Mode, th *theme.Theme) {
	if mode.IsRich() {
		fmt.Println(th.Title.Render(fmt.Sprintf("⚡ Executing command on agent '%s'", agentID)))
		fmt.Println(th.MutedText.Render(fmt.Sprintf("   Container: %s", format.FormatContainerID(containerID))))
		fmt.Println(th.MutedText.Render(fmt.Sprintf("   Command: %s", command)))
		fmt.Println()
	} else {
		// Plain mode
		fmt.Printf("Executing command on agent '%s'\n", agentID)
		fmt.Printf("Container: %s\n", format.FormatContainerID(containerID))
		fmt.Printf("Command: %s\n", command)
		fmt.Println()
	}
}

// printExecuteOutput prints the command output and result
func printExecuteOutput(result ExecuteResult, mode cli.Mode, th *theme.Theme) {
	if mode.IsRich() {
		fmt.Println(th.SuccessText.Render("─── Output ───"))

		// Print stdout
		if result.Stdout != "" {
			fmt.Print(strings.TrimSuffix(result.Stdout, "\n"))
			fmt.Println()
		}

		// Print stderr in red
		if result.Stderr != "" {
			fmt.Print(th.ErrorText.Render(strings.TrimSuffix(result.Stderr, "\n")))
			fmt.Println()
		}

		fmt.Println(th.SuccessText.Render("─────────────"))

		if result.Success {
			fmt.Println(th.SuccessText.Render("✓ Command completed successfully"))
		} else {
			fmt.Println(th.ErrorText.Render(fmt.Sprintf("✗ Command failed with exit code %d", result.ExitCode)))
		}
	} else {
		// Plain mode
		fmt.Println("--- Output ---")

		// Print stdout
		if result.Stdout != "" {
			fmt.Print(strings.TrimSuffix(result.Stdout, "\n"))
			fmt.Println()
		}

		// Print stderr
		if result.Stderr != "" {
			fmt.Fprintf(os.Stderr, "%s\n", strings.TrimSuffix(result.Stderr, "\n"))
		}

		fmt.Println("-------------")

		if result.Success {
			fmt.Println("Command completed successfully")
		} else {
			fmt.Fprintf(os.Stderr, "Command failed with exit code %d\n", result.ExitCode)
		}
	}
}
