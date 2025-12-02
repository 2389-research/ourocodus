package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/render"
	uilist "github.com/2389-research/ourocodus/cmd/agentd/internal/tui/list"
	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/cli/output"
	"github.com/2389-research/ourocodus/pkg/labels"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/docker/docker/api/types/container"
	"github.com/spf13/cobra"
)

var listNATSURL string

const defaultNATSURL = "nats://localhost:4222"

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "📋 List all active agents",
	Long:  "Shows all active agents with their status, workspace, and container information.",
	Example: `  # List all running agents (auto-detect mode)
  agentd list

  # Force plain text output
  agentd list --plain

  # JSON output for scripting
  agentd list --json

  # Use light theme for rich mode
  agentd list --light`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listNATSURL, "nats", defaultNATSURL, "NATS server URL for live heartbeats (use empty string to disable)")
}

func runList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Get mode from AppContext (set by cli.App wrapper)
	appCtx := cli.FromContext(ctx)
	if appCtx == nil {
		return cli.ContextError()
	}

	// Optional heartbeat monitor
	hbURL := listNATSURL
	if env := os.Getenv("AGENTD_NATS"); env != "" {
		hbURL = env
	}
	var hbMonitor *session.HeartbeatMonitor
	hbStatus := "Heartbeats: disabled (no NATS URL)"
	if hbURL != "" {
		monitor, err := session.NewHeartbeatMonitor(hbURL)
		if err != nil {
			hbStatus = fmt.Sprintf("Heartbeats disabled: %v", err)
		} else if err := monitor.Start(ctx); err != nil {
			hbStatus = fmt.Sprintf("Heartbeats disabled: %v", err)
			monitor.Stop()
		} else {
			hbMonitor = monitor
			hbStatus = fmt.Sprintf("Heartbeats: live (%s)", hbURL)
			go func() {
				<-ctx.Done()
				monitor.Stop()
			}()
		}
	}

	// Query Docker for agents
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		return cli.IOError("failed to list agents: " + err.Error())
	}

	// Convert to render.AgentInfo format
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
			LastBeat:    agent.LastBeat,
		}
	}
	applyHeartbeats(renderAgents, hbMonitor)

	// Render based on mode
	if appCtx.Mode.IsRich() {
		opts := uilist.RunOptions{
			Loader: func(ctx context.Context) ([]render.AgentInfo, error) {
				agents, err := listAgentsFromDocker(ctx)
				if err != nil {
					return nil, err
				}
				res := make([]render.AgentInfo, len(agents))
				for i, agent := range agents {
					res[i] = render.AgentInfo{
						AgentID:     agent.AgentID,
						ContainerID: agent.ContainerID,
						Status:      agent.Status,
						Workspace:   agent.Workspace,
						SpawnSource: agent.SpawnSource,
						AttachedTo:  agent.AttachedTo,
						CreatedAt:   agent.CreatedAt,
						LastBeat:    agent.LastBeat,
					}
				}
				applyHeartbeats(res, hbMonitor)
				return res, nil
			},
			Stopper: func(ctx context.Context, agentID string) error {
				// Create a silent AppContext to suppress output during TUI stop operations.
				// The TUI shows status in its own status bar.
				silentCtx := &cli.AppContext{
					Mode:   cli.ModePlain,
					Theme:  theme.Ensure(nil),
					Output: output.NewPlainOutputWithWriters(io.Discard, io.Discard, true),
				}
				return stopAgent(silentCtx.ToContext(ctx), nil, agentID)
			},
		}
		return uilist.Run(ctx, appCtx.Theme, renderAgents, hbStatus, opts)
	}

	// Plain/JSON mode - use render package
	mode := cli.ModePlain
	if appCtx.Mode.IsJSON() {
		mode = cli.ModeJSON
	}
	return render.RenderAgentList(os.Stdout, renderAgents, mode, appCtx.Theme)
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
	LastBeat    time.Time
}

// listAgentsFromDocker queries Docker for containers with agentd labels
func listAgentsFromDocker(ctx context.Context) ([]agentInfo, error) {
	dockerCli, err := newDockerClient()
	if err != nil {
		return nil, err
	}
	defer func() { _ = dockerCli.Close() }()

	// Use centralized filter builder from pkg/labels
	containers, err := dockerCli.ContainerList(ctx, container.ListOptions{
		All:     false, // Only running containers
		Filters: labels.ListAgentsFilter(),
	})
	if err != nil {
		return nil, cli.IOError("failed to list containers: " + err.Error())
	}

	// Get all leases to determine attached status and last heartbeat
	leases, err := listLeasesForList()
	if err != nil {
		// Don't fail if leases can't be read, just continue without adoption status
		leases = make(map[string]*session.Lease)
	}

	agents := make([]agentInfo, 0, len(containers))
	for _, c := range containers {
		// Get agent ID from centralized label
		agentID := c.Labels[labels.AgentID]
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
		spawnSource := c.Labels[labels.SpawnSource]
		if spawnSource == "" {
			spawnSource = "unknown"
		}

		// Get attachment status and last beat from leases
		var attachedTo string
		var lastBeat time.Time
		if lease, ok := leases[agentID]; ok {
			attachedTo = lease.UserSessionID
			lastBeat = lease.ExpiresAt.Add(-session.LeaseTTL)
		}

		agents = append(agents, agentInfo{
			AgentID:     agentID,
			ContainerID: c.ID,
			Status:      c.State,
			Workspace:   workspace,
			SpawnSource: spawnSource,
			AttachedTo:  attachedTo,
			CreatedAt:   time.Unix(c.Created, 0),
			LastBeat:    lastBeat,
		})
	}

	return agents, nil
}

// listLeasesForList returns a map of agentID -> lease for attached agents
func listLeasesForList() (map[string]*session.Lease, error) {
	leases, err := session.ListLeases()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*session.Lease)
	for _, lease := range leases {
		if !session.IsLeaseExpired(lease) {
			result[lease.AgentID] = lease
		}
	}

	return result, nil
}

func applyHeartbeats(agents []render.AgentInfo, monitor *session.HeartbeatMonitor) {
	if monitor == nil {
		return
	}
	for i := range agents {
		if ts := monitor.GetLastSeen(agents[i].AgentID); !ts.IsZero() {
			agents[i].LastBeat = ts
		}
	}
}
