package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/render"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

var (
	listFormat    string
	listPlainFlag bool
	listTheme     string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "📋 List all active agents",
	Long:  "Shows all active agents with their status, workspace, and container information.",
	Example: `  # List all running agents (auto-detect mode)
  agentd list

  # Force plain text output
  agentd list --plain

  # JSON output for scripting
  agentd list --format json

  # Use amber theme for rich mode
  agentd list --theme amber`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listFormat, "format", "auto", "Output format (auto|rich|plain|json)")
	listCmd.Flags().BoolVar(&listPlainFlag, "plain", false, "Force plain text output (alias for --format plain)")
	listCmd.Flags().StringVar(&listTheme, "theme", "cga", "Color theme for rich mode (cga|amber|green|c64)")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Step 1: Detect output mode
	jsonFlag := listFormat == "json"
	plainFlag := listPlainFlag || listFormat == "plain"
	shouldPlain := detect.ShouldUsePlainMode(jsonFlag, plainFlag, os.Environ)

	mode := output.ModeRich
	if listFormat != "auto" {
		// Explicit format flag takes precedence
		parsedMode, valid := output.ParseMode(listFormat)
		if valid {
			mode = parsedMode
		}
	} else {
		// Auto-detect mode
		mode = output.DetectMode(jsonFlag, plainFlag, shouldPlain)
	}

	// Step 2: Create theme if rich mode
	var th *theme.RetroTheme
	if mode.IsRich() {
		palette, valid := theme.ParsePaletteName(listTheme)
		if !valid {
			palette = theme.PaletteCGA
		}
		th = theme.NewRetroTheme(palette)
	}

	// Step 3: Query Docker for agents
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	// Step 4: Convert to render.AgentInfo format
	renderAgents := make([]render.AgentInfo, len(agents))
	for i, agent := range agents {
		renderAgents[i] = render.AgentInfo{
			AgentID:     agent.AgentID,
			ContainerID: agent.ContainerID,
			Status:      agent.Status,
			Workspace:   agent.Workspace,
			SpawnSource: agent.SpawnSource,
			AttachedTo:  agent.AttachedTo,
			CreatedAt:   agent.CreatedAt,
		}
	}

	// Step 5: Render using new renderer
	return render.RenderAgentList(os.Stdout, renderAgents, mode, th)
}

// agentInfo represents an agent discovered from Docker
type agentInfo struct {
	AgentID     string
	ContainerID string
	Status      string
	Workspace   string
	SpawnSource string
	AttachedTo  string
	CreatedAt   time.Time
}

// listAgentsFromDocker queries Docker for containers with agentd labels
func listAgentsFromDocker(ctx context.Context) ([]agentInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	// Filter for containers with ourocodus.agent label (from pkg/)
	// Note: We use the pkg-provided labels since SpawnConfig doesn't support custom labels
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
	leases, err := listLeasesForList()
	if err != nil {
		// Don't fail if leases can't be read, just continue without adoption status
		leases = make(map[string]string)
	}

	agents := make([]agentInfo, 0, len(containers))
	for _, c := range containers {
		// Get agent ID from pkg-provided label
		agentID := c.Labels[LabelAgentID]
		if agentID == "" {
			continue // Skip containers without agent-id
		}

		// Extract workspace from mounts (since it's not in labels)
		// Find the mount that targets /workspace (the worktree mount)
		workspace := ""
		for _, mnt := range c.Mounts {
			if mnt.Destination == "/workspace" {
				workspace = mnt.Source
				break
			}
		}
		// Fallback to first mount if no /workspace mount found
		if workspace == "" && len(c.Mounts) > 0 {
			workspace = c.Mounts[0].Source
		}

		// Get spawn source from label (defaults to "unknown")
		spawnSource := c.Labels[LabelSpawnSource]
		if spawnSource == "" {
			spawnSource = "unknown"
		}

		// Get attachment status from leases
		attachedTo := leases[agentID]

		agents = append(agents, agentInfo{
			AgentID:     agentID,
			ContainerID: c.ID,
			Status:      c.State,
			Workspace:   workspace,
			SpawnSource: spawnSource,
			AttachedTo:  attachedTo,
			CreatedAt:   time.Unix(c.Created, 0),
		})
	}

	return agents, nil
}

// listLeasesForList returns a map of agentID -> userSessionID for attached agents
func listLeasesForList() (map[string]string, error) {
	leases, err := session.ListLeases()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, lease := range leases {
		if !session.IsLeaseExpired(lease) {
			result[lease.AgentID] = lease.UserSessionID
		}
	}

	return result, nil
}

// formatContainerID shows short container ID
func formatContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
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

// formatSpawnSource formats the spawn source label (kept for compatibility with cmd_discover.go)
func formatSpawnSource(source string) string {
	switch source {
	case "cli":
		return "cli"
	case "relay":
		return "relay"
	case "unknown":
		return "unknown"
	default:
		return source
	}
}

// formatStateString converts Docker state string to human-readable string (kept for tests)
func formatStateString(state string) string {
	switch state {
	case "running":
		return "running"
	case "exited", "stopped":
		return "stopped"
	case "dead", "removing":
		return "failed"
	case "created", "restarting":
		return "pending"
	case "paused":
		return "paused"
	default:
		return state
	}
}
