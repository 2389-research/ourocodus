package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	uirepl "github.com/2389-research/ourocodus/cmd/agentd/internal/tui/repl"
	"github.com/charmbracelet/lipgloss"
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
		return fmt.Errorf("agent ID required")
	}

	agentID := args[0]
	ctx := cmd.Context()

	// Find agent
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	th := theme.NewRetroTheme(theme.PaletteCGA)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Error)))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Success)))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Muted)))

	agent, found := findAgentByID(agents, agentID)
	if !found {
		fmt.Println(errorStyle.Render(fmt.Sprintf("✗ Agent '%s' not found", agentID)))
		fmt.Println("\nRunning agents:")
		if len(agents) == 0 {
			fmt.Println("  (none)")
		}
		for _, a := range agents {
			fmt.Printf("  - %s\n", a.AgentID)
		}
		return fmt.Errorf("agent not found")
	}

	if agent.Status != "running" {
		return fmt.Errorf("agent '%s' is not running (status: %s)", agentID, agent.Status)
	}

	// Connect to Docker
	dockerClient, err := newDockerClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer func() { _ = dockerClient.Close() }()

	// Print connection message
	fmt.Println(successStyle.Render(fmt.Sprintf("✓ Connected to agent '%s'", agentID)))
	fmt.Println(mutedStyle.Render("  Press Ctrl+D to exit"))

	// Attach to container without TTY (raw JSON lines)
	attachResp, err := dockerClient.ContainerAttach(ctx, agent.ContainerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to container: %w", err)
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

	// Start TUI
	err = uirepl.Run(ctx, th, agentID, attachResp.Conn, stdoutR, stderrR)

	// On exit, print disconnect unless user already saw one
	if err == nil || strings.Contains(err.Error(), "interrupt") {
		fmt.Println()
		fmt.Println(successStyle.Render(fmt.Sprintf("✓ Disconnected from agent '%s'", agentID)))
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
