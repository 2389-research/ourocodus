package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var replCmd = &cobra.Command{
	Use:   "repl <agent-id>",
	Short: "🔄 Interactive REPL for ACP communication (requires relay)",
	Long: `Start an interactive REPL session to communicate with an agent via ACP.

NOTE: This command requires the relay server to be running. The agent communication
architecture uses a WebSocket relay to broker messages between clients and agents.

To use this command:
1. Start the relay server: make relay (or: go run cmd/relay/main.go)
2. The relay will start on http://localhost:8080
3. Use this command to connect: agentd repl alice

The relay handles:
- WebSocket connections from clients (PWA, CLI)
- Agent lifecycle (spawn, attach, stop)
- Message routing between clients and agents
- Session management

Current Status: Relay integration not yet implemented in agentd.
For now, use the PWA at http://localhost:8080 for agent interaction.`,
	Example: `  # Start relay first
  make relay

  # Then connect to an agent (not yet implemented)
  agentd repl alice

  # For now, use the PWA
  open http://localhost:8080`,
	Args: cobra.ExactArgs(1),
	RunE: runREPL,
}

func runREPL(cmd *cobra.Command, args []string) error {
	agentID := args[0]

	color.New(color.FgYellow, color.Bold).Println("⚠️  REPL Mode Not Yet Implemented")
	fmt.Println()
	color.New(color.FgWhite).Printf("Agent '%s' uses the Agent Communication Protocol (ACP) for messaging.\n", agentID)
	fmt.Println()
	color.New(color.FgWhite).Println("To interact with agents via ACP, you need to:")
	fmt.Println()
	color.New(color.FgCyan).Println("  1. Start the relay server:")
	color.New(color.FgHiBlack).Println("     make relay")
	color.New(color.FgHiBlack).Println("     # or: go run cmd/relay/main.go")
	fmt.Println()
	color.New(color.FgCyan).Println("  2. Use the PWA interface:")
	color.New(color.FgHiBlack).Println("     open http://localhost:8080")
	fmt.Println()
	color.New(color.FgCyan).Println("  3. Connect to your agent from the UI")
	fmt.Println()
	fmt.Println()
	color.New(color.FgYellow).Println("Why is this needed?")
	color.New(color.FgWhite).Println("Agents run ACP servers inside containers. The relay:")
	color.New(color.FgWhite).Println("  • Provides WebSocket endpoints for clients")
	color.New(color.FgWhite).Println("  • Manages agent sessions and lifecycles")
	color.New(color.FgWhite).Println("  • Routes messages between clients and agents")
	color.New(color.FgWhite).Println("  • Handles connection multiplexing")
	fmt.Println()
	fmt.Println()
	color.New(color.FgGreen).Println("Alternative: Use 'attach' for shell access:")
	color.New(color.FgHiBlack).Printf("  agentd attach %s\n", agentID)
	fmt.Println()

	return fmt.Errorf("REPL mode requires relay integration (coming soon)")
}
