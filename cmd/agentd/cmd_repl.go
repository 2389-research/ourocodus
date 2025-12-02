package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	uirepl "github.com/2389-research/ourocodus/cmd/agentd/internal/tui/repl"
	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/spf13/cobra"
)

var replCmd = &cobra.Command{
	Use:   "repl <agent-id>",
	Short: "🔄 Interactive REPL with agent via ACP",
	Long: `Connect to a running agent and interact via ACP protocol.

The agent must be running (spawned). This command attaches directly to
the agent's stdin/stdout where the ACP process runs as PID 1.`,
	Example: `  # Connect to running agent
  agentd repl alice

  # Once connected, send messages
  > Hello agent!
  Echo: Hello agent!

  # Exit with Ctrl+D`,
	Args: cobra.ExactArgs(1),
	RunE: runREPL,
}

func runREPL(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cli.UsageError("agent ID required")
	}

	agentID := args[0]
	ctx := cmd.Context()

	// Get AppContext from CLI framework
	appCtx := cli.FromContext(ctx)
	if appCtx == nil {
		return cli.ContextError()
	}

	// REPL is inherently interactive - JSON mode doesn't make sense
	if appCtx.Mode.IsJSON() {
		return cli.UsageError("REPL command requires interactive mode (cannot use --json flag)")
	}

	// Find and validate agent
	agent, err := findAndValidateAgent(ctx, agentID, appCtx)
	if err != nil {
		return err
	}

	// Connect and run REPL
	return runREPLSession(ctx, agentID, agent, appCtx)
}

// findAndValidateAgent finds the agent and validates it is running.
func findAndValidateAgent(ctx context.Context, agentID string, appCtx *cli.AppContext) (agentInfo, error) {
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		return agentInfo{}, cli.IOError("failed to list agents: " + err.Error())
	}

	agent, found := findAgentByID(agents, agentID)
	if !found {
		printAgentNotFound(agentID, agents, appCtx)
		return agentInfo{}, cli.UsageError("agent not found")
	}

	if agent.Status != "running" {
		return agentInfo{}, cli.UsageError(fmt.Sprintf("agent '%s' is not running (status: %s)", agentID, agent.Status))
	}

	return agent, nil
}

// printAgentNotFound prints a helpful message when an agent is not found.
func printAgentNotFound(agentID string, agents []agentInfo, appCtx *cli.AppContext) {
	if appCtx.Mode.IsRich() {
		fmt.Println(appCtx.Theme.ErrorText.Render(fmt.Sprintf("✗ Agent '%s' not found", agentID)))
	} else {
		fmt.Printf("Agent '%s' not found\n", agentID)
	}
	fmt.Println("\nRunning agents:")
	if len(agents) == 0 {
		fmt.Println("  (none)")
	}
	for _, a := range agents {
		fmt.Printf("  - %s\n", a.AgentID)
	}
}

// runREPLSession connects to Docker and runs the REPL.
func runREPLSession(ctx context.Context, agentID string, agent agentInfo, appCtx *cli.AppContext) error {
	dockerClient, err := newDockerClient()
	if err != nil {
		return cli.IOError("failed to create Docker client: " + err.Error())
	}
	defer func() { _ = dockerClient.Close() }()

	// Print connection message
	if appCtx.Mode.IsRich() {
		fmt.Println(appCtx.Theme.SuccessText.Render(fmt.Sprintf("✓ Connected to agent '%s'", agentID)))
		fmt.Println(appCtx.Theme.MutedText.Render("  Press Ctrl+D to exit"))
	} else {
		appCtx.Output.Success(fmt.Sprintf("Connected to agent '%s'", agentID))
		appCtx.Output.Info("  Press Ctrl+D to exit")
	}

	attachResp, err := dockerClient.ContainerAttach(ctx, agent.ContainerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return cli.IOError("failed to attach to container: " + err.Error())
	}
	defer attachResp.Close()

	// Demultiplex Docker stdio
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	go func() {
		_, _ = stdcopy.StdCopy(stdoutW, stderrW, attachResp.Reader)
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	// Start TUI (works in both rich and plain modes)
	err = uirepl.Run(ctx, appCtx.Theme, agentID, attachResp.Conn, stdoutR, stderrR)

	// On exit, print disconnect message based on mode
	if err == nil || strings.Contains(err.Error(), "interrupt") {
		fmt.Println()
		if appCtx.Mode.IsRich() {
			fmt.Println(appCtx.Theme.SuccessText.Render(fmt.Sprintf("✓ Disconnected from agent '%s'", agentID)))
		} else {
			appCtx.Output.Success(fmt.Sprintf("Disconnected from agent '%s'", agentID))
		}
		return nil
	}
	return err
}

// findAgentByID searches for an agent by ID
func findAgentByID(agents []agentInfo, agentID string) (agentInfo, bool) {
	for _, agent := range agents {
		if agent.AgentID == agentID {
			return agent, true
		}
	}
	return agentInfo{}, false
}
