package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nats-io/nats.go"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/render"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/2389-research/ourocodus/pkg/heartbeat"
)

// tickMsg is sent by the ticker for periodic refresh
type tickMsg time.Time

// agentsMsg contains the result of an agent query
type agentsMsg struct {
	agents []render.AgentInfo
	err    error
}

// heartbeatMsg contains heartbeat data for an agent
type heartbeatMsg struct {
	agentID   string
	timestamp time.Time
	lag       time.Duration
}

// connectedMsg indicates NATS connection is established
type connectedMsg struct {
	nc *nats.Conn
}

// AgentFetcher is a function type that fetches agents from the system
type AgentFetcher func(context.Context) ([]render.AgentInfo, error)

// Model represents the Bubble Tea model for discover watch mode
type Model struct {
	agents      []render.AgentInfo
	theme       *theme.RetroTheme
	spinner     spinner.Model
	loading     bool
	err         error
	quitting    bool
	lastFetch   time.Time
	fetchAgents AgentFetcher
	ctx         context.Context

	// Heartbeat tracking
	nc              *nats.Conn
	natsURL         string
	heartbeats      map[string]heartbeatInfo      // agentID -> heartbeat info
	heartbeatsMutex *sync.RWMutex                 // Pointer to avoid copying
	subscriptions   map[string]*nats.Subscription // agentID -> subscription
}

// heartbeatInfo tracks heartbeat data for an agent
type heartbeatInfo struct {
	lastSeen time.Time
	lag      time.Duration
}

// NewModel creates a new discover watch model
func NewModel(ctx context.Context, th *theme.RetroTheme, fetcher AgentFetcher) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(th.Primary)

	// Get NATS URL from environment or use default
	natsURL := "nats://127.0.0.1:4222" // Default NATS URL (use 127.0.0.1 not localhost for DNS reliability)
	// TODO: Read from NATS_URL env var if set

	return Model{
		theme:           th,
		spinner:         s,
		loading:         true,
		fetchAgents:     fetcher,
		ctx:             ctx,
		natsURL:         natsURL,
		heartbeats:      make(map[string]heartbeatInfo),
		heartbeatsMutex: &sync.RWMutex{},
		subscriptions:   make(map[string]*nats.Subscription),
	}
}

// Init initializes the model and starts the first fetch
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.connectToNATS(),
		m.fetchAgentsCmd(),
		tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.cleanup()
			return m, tea.Quit
		case "r":
			// Force refresh
			m.loading = true
			return m, m.fetchAgentsCmd()
		}

	case tickMsg:
		// Auto-refresh every 2s
		return m, tea.Batch(
			m.fetchAgentsCmd(),
			tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return tickMsg(t)
			}),
		)

	case connectedMsg:
		m.nc = msg.nc
		return m, nil

	case heartbeatMsg:
		// Only update if we received a real heartbeat (not a timeout)
		if !msg.timestamp.IsZero() {
			m.updateHeartbeat(msg.agentID, msg.timestamp, msg.lag)
		}
		// Always poll again, whether timeout or real heartbeat
		return m, m.pollHeartbeat(msg.agentID)

	case agentsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.agents = m.enrichWithHeartbeats(msg.agents)
			m.lastFetch = time.Now()
			m.err = nil // Clear previous errors

			// Subscribe to heartbeats for new agents
			return m, m.subscribeToNewAgents(msg.agents)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the model
func (m Model) View() string {
	if m.quitting {
		return "Stopped watching.\n"
	}

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(m.theme.Error)
		mutedStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)
		return fmt.Sprintf("%s\n\n%s\n",
			errStyle.Render(fmt.Sprintf("Error: %v", m.err)),
			mutedStyle.Render("Press q to quit, r to retry."),
		)
	}

	if m.loading && len(m.agents) == 0 {
		return m.spinner.View() + " Loading agents..."
	}

	// Render agent table using existing renderer
	var buf bytes.Buffer
	if err := render.RenderAgentList(&buf, m.agents, output.ModeRich, m.theme); err != nil {
		return fmt.Sprintf("Error rendering: %v", err)
	}

	// Add refresh indicator
	timeSince := time.Since(m.lastFetch).Round(time.Second)
	refreshLine := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render(fmt.Sprintf("\nLast refresh: %s ago  |  [q]uit  [r]efresh", formatDuration(timeSince)))

	return buf.String() + refreshLine
}

// fetchAgentsCmd returns a command that fetches agents
func (m *Model) fetchAgentsCmd() tea.Cmd {
	return func() tea.Msg {
		agents, err := m.fetchAgents(m.ctx)
		if err != nil {
			return agentsMsg{err: err}
		}
		return agentsMsg{agents: agents}
	}
}

// formatDuration formats a duration as a human-readable string
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	if d < time.Second {
		return "0s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// connectToNATS establishes connection to NATS server
func (m *Model) connectToNATS() tea.Cmd {
	return func() tea.Msg {
		nc, err := nats.Connect(m.natsURL,
			nats.Name("agentd-list-watch"),
			nats.Timeout(5*time.Second),
		)
		if err != nil {
			// Silently fail - heartbeats just won't be available
			return nil
		}
		return connectedMsg{nc: nc}
	}
}

// subscribeToNewAgents subscribes to heartbeats for agents we're not already tracking
func (m *Model) subscribeToNewAgents(agents []render.AgentInfo) tea.Cmd {
	if m.nc == nil {
		return nil // NATS not connected
	}

	var cmds []tea.Cmd
	for _, agent := range agents {
		// Skip if already subscribed
		if _, exists := m.subscriptions[agent.AgentID]; exists {
			continue
		}

		// Subscribe to this agent's heartbeats using SubscribeSync for polling
		subject := fmt.Sprintf("%s.%s", heartbeat.SubjectPrefix, agent.AgentID)
		sub, err := m.nc.SubscribeSync(subject)

		if err == nil {
			m.subscriptions[agent.AgentID] = sub

			// Start polling this subscription
			agentID := agent.AgentID // Capture for closure
			cmds = append(cmds, m.pollHeartbeat(agentID))
		}
	}

	return tea.Batch(cmds...)
}

// pollHeartbeat continuously polls for heartbeats from a specific agent
func (m *Model) pollHeartbeat(agentID string) tea.Cmd {
	return func() tea.Msg {
		sub, exists := m.subscriptions[agentID]
		if !exists || sub == nil {
			return nil
		}

		// Wait for next message with timeout
		msg, err := sub.NextMsg(10 * time.Second)
		if err != nil {
			// On timeout, return a message with zero timestamp to reschedule
			return heartbeatMsg{
				agentID:   agentID,
				timestamp: time.Time{}, // Zero time indicates timeout
				lag:       0,
			}
		}

		var hb heartbeat.Message
		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			// On parse error, still reschedule
			return heartbeatMsg{
				agentID:   agentID,
				timestamp: time.Time{},
				lag:       0,
			}
		}

		// Calculate lag
		lag := time.Since(hb.Timestamp)

		return heartbeatMsg{
			agentID:   hb.AgentID,
			timestamp: hb.Timestamp,
			lag:       lag,
		}
	}
}

// updateHeartbeat updates the heartbeat info for an agent
func (m *Model) updateHeartbeat(agentID string, timestamp time.Time, lag time.Duration) {
	m.heartbeatsMutex.Lock()
	defer m.heartbeatsMutex.Unlock()

	m.heartbeats[agentID] = heartbeatInfo{
		lastSeen: timestamp,
		lag:      lag,
	}
}

// enrichWithHeartbeats adds heartbeat information to agent list
func (m *Model) enrichWithHeartbeats(agents []render.AgentInfo) []render.AgentInfo {
	m.heartbeatsMutex.RLock()
	defer m.heartbeatsMutex.RUnlock()

	enriched := make([]render.AgentInfo, len(agents))
	for i, agent := range agents {
		enriched[i] = agent

		// Add heartbeat info if available
		if info, exists := m.heartbeats[agent.AgentID]; exists {
			enriched[i].LastHeartbeat = info.lastSeen
			enriched[i].HeartbeatLag = info.lag
		}
	}

	return enriched
}

// cleanup closes NATS connection and unsubscribes
func (m *Model) cleanup() {
	for _, sub := range m.subscriptions {
		if sub != nil {
			_ = sub.Unsubscribe()
		}
	}

	if m.nc != nil {
		m.nc.Close()
	}
}
