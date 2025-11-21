package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	discoverFormat string
	discoverWatch  bool
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "🔍 Discover agents with adoption status",
	Long: `Discover all running agents and show their adoption status.

This command queries Docker for agent containers and checks their lease status
to determine if they are attached, detached, or orphaned (no heartbeat).`,
	Example: `  # Discover agents once
  agentd discover

  # Watch agents with live updates
  agentd discover --watch

  # Output as JSON
  agentd discover --format json`,
	RunE: runDiscover,
}

func init() {
	discoverCmd.Flags().StringVar(&discoverFormat, "format", "table", "Output format (table|json)")
	discoverCmd.Flags().BoolVar(&discoverWatch, "watch", false, "Watch for changes and update every 2 seconds")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Setup signal handling for clean exit on Ctrl+C
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if discoverWatch {
		return watchAgents(ctx)
	}

	agents, err := discoverAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover agents: %w", err)
	}

	return displayAgents(agents)
}

// discoveredAgent represents an agent with adoption status
type discoveredAgent struct {
	AgentID       string    `json:"agentId"`
	ContainerID   string    `json:"containerId"`
	Status        string    `json:"status"` // discovered, attached, orphaned
	Workspace     string    `json:"workspace"`
	SpawnSource   string    `json:"spawnSource"`
	AttachedTo    string    `json:"attachedTo,omitempty"`
	LeaseExpires  time.Time `json:"leaseExpires,omitempty"`
	HeartbeatAge  string    `json:"heartbeatAge,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// discoverAgents queries Docker and lease files to discover agents with adoption status
func discoverAgents(ctx context.Context) ([]discoveredAgent, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	// Filter for agentd containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=true", LabelNamespace))

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     false, // Only running containers
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Get all leases to determine attached status
	leases, err := session.ListLeases()
	if err != nil {
		// Don't fail if leases can't be read (directory might not exist yet)
		leases = []*session.Lease{}
	}

	// Build lease map (agent ID -> lease)
	leaseMap := make(map[string]*session.Lease)
	for _, lease := range leases {
		if !session.IsLeaseExpired(lease) {
			leaseMap[lease.AgentID] = lease
		}
	}

	agents := make([]discoveredAgent, 0, len(containers))
	for _, c := range containers {
		agentID := c.Labels[LabelAgentID]
		if agentID == "" {
			continue // Skip containers without agent-id
		}

		// Extract workspace from mounts
		workspace := ""
		for _, mnt := range c.Mounts {
			if mnt.Destination == "/workspace" {
				workspace = mnt.Source
				break
			}
		}

		// Get spawn source from label
		spawnSource := c.Labels[LabelSpawnSource]
		if spawnSource == "" {
			spawnSource = "unknown"
		}

		// Determine adoption status
		status := "discovered" // Default: not attached
		attachedTo := ""
		var leaseExpires time.Time

		if lease, ok := leaseMap[agentID]; ok {
			status = "attached"
			attachedTo = lease.UserSessionID
			leaseExpires = lease.ExpiresAt
		}

		// TODO: In future, check heartbeat age to determine if orphaned
		// For now, just show discovered vs attached

		agent := discoveredAgent{
			AgentID:      agentID,
			ContainerID:  c.ID,
			Status:       status,
			Workspace:    workspace,
			SpawnSource:  spawnSource,
			AttachedTo:   attachedTo,
			LeaseExpires: leaseExpires,
			CreatedAt:    time.Unix(c.Created, 0),
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// displayAgents prints agents based on format flag
func displayAgents(agents []discoveredAgent) error {
	if discoverFormat == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(agents)
	}

	return displayAgentsTable(agents)
}

// displayAgentsTable prints agents in a formatted table
func displayAgentsTable(agents []discoveredAgent) error {
	if len(agents) == 0 {
		_, _ = color.New(color.FgHiBlack).Println("✨ No agents discovered.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header with color
	headerColor := color.New(color.FgCyan, color.Bold)
	_, _ = fmt.Fprintln(w)
	_, _ = headerColor.Fprintln(w, "AGENT\tSTATUS\tSOURCE\tATTACHED TO\tLEASE EXPIRES\tCREATED")

	for _, agent := range agents {
		// Color the agent ID
		agentName := color.New(color.FgWhite, color.Bold).Sprint(agent.AgentID)

		// Format lease expiry
		leaseExpiry := "-"
		if !agent.LeaseExpires.IsZero() {
			timeUntil := time.Until(agent.LeaseExpires)
			if timeUntil > 0 {
				leaseExpiry = formatDuration(timeUntil)
			} else {
				leaseExpiry = color.New(color.FgRed).Sprint("expired")
			}
		}

		// Format attached to
		attachedTo := "-"
		if agent.AttachedTo != "" {
			attachedTo = agent.AttachedTo
			if len(attachedTo) > 16 {
				attachedTo = attachedTo[:13] + "..."
			}
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			agentName,
			formatAdoptionStatus(agent.Status),
			formatSpawnSource(agent.SpawnSource),
			attachedTo,
			leaseExpiry,
			formatDuration(time.Since(agent.CreatedAt)),
		)
	}

	_, _ = fmt.Fprintln(w)
	return w.Flush()
}

// formatAdoptionStatus formats the adoption status with colors
func formatAdoptionStatus(status string) string {
	switch status {
	case "attached":
		return successColor.Sprint("attached")
	case "discovered":
		return color.New(color.FgYellow).Sprint("discovered")
	case "orphaned":
		return errorColor.Sprint("orphaned")
	default:
		return color.New(color.FgHiBlack).Sprint(status)
	}
}

// watchAgents continuously polls and displays agent status
func watchAgents(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Clear screen and show initial state
	clearScreen()
	agents, err := discoverAgents(ctx)
	if err != nil {
		return err
	}
	if err := displayAgentsTable(agents); err != nil {
		return err
	}

	// Print watch footer
	fmt.Println(color.New(color.FgHiBlack).Sprint("Press Ctrl+C to stop watching..."))

	for {
		select {
		case <-ctx.Done():
			fmt.Println() // Add newline before exit
			return nil
		case <-ticker.C:
			clearScreen()

			agents, err := discoverAgents(ctx)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			if err := displayAgentsTable(agents); err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			// Print watch footer
			fmt.Println(color.New(color.FgHiBlack).Sprint("Press Ctrl+C to stop watching..."))
		}
	}
}

// clearScreen clears the terminal screen
func clearScreen() {
	fmt.Print("\033[2J\033[H")
}
