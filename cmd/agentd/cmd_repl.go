package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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

	// Connect to Docker
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer func() { _ = dockerClient.Close() }()

	// Print connection message
	_, _ = color.New(color.FgGreen).Printf("✓ Connected to agent '%s'\n", agentID)
	_, _ = color.New(color.FgHiBlack).Println("  Press Ctrl+D to exit")

	// Attach to container
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

	// Set up terminal
	oldState, err := setRawTerminal()
	if err != nil {
		_, _ = color.New(color.FgYellow).Printf("Warning: Failed to set raw mode: %v\n", err)
		// Continue without raw mode
	}
	if oldState != nil {
		defer func() { _ = restoreTerminal(oldState) }()
	}

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	// Bidirectional copy
	errChan := make(chan error, 2)

	// Copy container output to stdout
	go func() {
		_, err := io.Copy(os.Stdout, attachResp.Reader)
		errChan <- err
	}()

	// Copy stdin to container
	go func() {
		_, err := io.Copy(attachResp.Conn, os.Stdin)
		errChan <- err
	}()

	// Wait for completion or signal
	select {
	case <-sigChan:
		// User interrupted - clean exit
		fmt.Println()
		_, _ = color.New(color.FgGreen).Printf("✓ Disconnected from agent '%s'\n", agentID)
		return nil
	case err := <-errChan:
		// Connection closed or error
		if err != nil && err != io.EOF {
			return fmt.Errorf("REPL error: %w", err)
		}
		_, _ = color.New(color.FgGreen).Printf("\n✓ Disconnected from agent '%s'\n", agentID)
		return nil
	}
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
