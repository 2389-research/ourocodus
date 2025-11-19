package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var listFormat string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active agents",
	Long:  "Shows all active agents with their status, workspace, and container information.",
	Example: `  # List all running agents
  agentd list

  # List in JSON format
  agentd list --format json`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listFormat, "format", "table", "Output format (table|json)")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Query Docker directly for agentd containers
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		fmt.Println("No agents running.")
		return nil
	}

	// Print based on format
	if listFormat == "json" {
		return printListJSONFromAgentInfo(agents)
	}

	return printListTableFromAgentInfo(agents)
}

// agentInfo represents an agent discovered from Docker
type agentInfo struct {
	AgentID     string
	ContainerID string
	Status      string
	Workspace   string
	CreatedAt   time.Time
}

// listAgentsFromDocker queries Docker for containers with agentd labels
func listAgentsFromDocker(ctx context.Context) ([]agentInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Filter for containers with ourocodus.agent label (from pkg/)
	// Note: We use the pkg-provided labels since SpawnConfig doesn't support custom labels
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "ourocodus.agent=true")

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     false, // Only running containers
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	agents := make([]agentInfo, 0, len(containers))
	for _, c := range containers {
		// Get agent ID from pkg-provided label
		agentID := c.Labels["agent-id"]
		if agentID == "" {
			continue // Skip containers without agent-id
		}

		// Extract workspace from mounts (since it's not in labels)
		workspace := ""
		if len(c.Mounts) > 0 {
			// The first mount is typically the workspace - use Source (host path)
			workspace = c.Mounts[0].Source
		}

		agents = append(agents, agentInfo{
			AgentID:     agentID,
			ContainerID: c.ID,
			Status:      c.State,
			Workspace:   workspace,
			CreatedAt:   time.Unix(c.Created, 0),
		})
	}

	return agents, nil
}

// printListTableFromAgentInfo prints agents in a formatted table
func printListTableFromAgentInfo(agents []agentInfo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintln(w)
	fmt.Fprintln(w, "AGENT\tSTATUS\tWORKSPACE\tCONTAINER\tCREATED")

	for _, agent := range agents {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			agent.AgentID,
			formatStateString(agent.Status),
			formatWorkspace(agent.Workspace),
			formatContainerID(agent.ContainerID),
			formatDuration(time.Since(agent.CreatedAt)),
		)
	}

	fmt.Fprintln(w)
	return w.Flush()
}

// printListJSONFromAgentInfo prints agents in JSON format
func printListJSONFromAgentInfo(agents []agentInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}

// formatStateString converts Docker state string to human-readable colored string
func formatStateString(state string) string {
	switch state {
	case "running":
		return successColor.Sprint("running")
	case "exited", "stopped":
		return color.New(color.FgHiBlack).Sprint("stopped")
	case "dead", "removing":
		return errorColor.Sprint("failed")
	case "created", "restarting":
		return color.New(color.FgYellow).Sprint("pending")
	case "paused":
		return color.New(color.FgYellow).Sprint("paused")
	default:
		return color.New(color.FgHiBlack).Sprint(state)
	}
}

// formatWorkspace shortens workspace path for display
func formatWorkspace(path string) string {
	// Show relative path from current directory if possible
	if len(path) > 60 {
		// Truncate long paths
		return "..." + path[len(path)-57:]
	}
	return path
}

// formatContainerID shows short container ID
func formatContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// formatDuration formats time.Duration as human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
