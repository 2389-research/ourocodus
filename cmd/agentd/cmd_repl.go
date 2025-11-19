package main

import (
	"context"
	"fmt"

	"github.com/fatih/color"
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
	ctx := context.Background()

	// Find agent
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	agent, found := findAgentByID(agents, agentID)
	if !found {
		_, _ = color.New(color.FgRed).Printf("✗ Agent '%s' not found\n", agentID)
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

	// TODO: Implement docker attach
	_, _ = color.New(color.FgGreen).Printf("✓ Found agent '%s' (container: %s)\n", agentID, formatContainerID(agent.ContainerID))
	fmt.Println("REPL implementation coming in next task...")

	return nil
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
