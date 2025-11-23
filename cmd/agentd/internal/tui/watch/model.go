package watch

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nats-io/nats.go"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/2389-research/ourocodus/pkg/heartbeat"
	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// Model represents the Bubble Tea model for the watch command
type Model struct {
	// Bubbles components
	viewport viewport.Model
	progress progress.Model
	spinner  spinner.Model

	// Configuration
	agentID string
	natsURL string
	theme   *theme.RetroTheme

	// Data state
	heartbeats []string       // Log lines of heartbeat events
	lease      *session.Lease // Current lease state, nil if detached
	nc         *nats.Conn     // NATS connection

	// UI state
	ready    bool  // Whether the UI is initialized and ready
	quitting bool  // Whether the user has quit
	err      error // Any error that occurred

	// Dimensions
	width  int
	height int
}

// NewModel creates a new watch model with the given configuration
func NewModel(agentID, natsURL string, th *theme.RetroTheme) Model {
	// Initialize viewport with default dimensions
	vp := viewport.New(80, 20)
	vp.YPosition = 0

	// Initialize progress bar with default gradient
	prog := progress.New(progress.WithDefaultGradient())

	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		viewport:   vp,
		progress:   prog,
		spinner:    sp,
		agentID:    agentID,
		natsURL:    natsURL,
		theme:      th,
		heartbeats: []string{},
		lease:      nil,
		ready:      false,
		quitting:   false,
		err:        nil,
		width:      0,
		height:     0,
	}
}

// Init initializes the model and returns initial commands to run
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,        // Start spinner animation
		m.connectToNATS(),     // Connect and subscribe
		m.startLeaseMonitor(), // Start polling leases
	)
}

// Update handles messages and updates the model state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 12
		return m, nil

	case connectedMsg:
		m.nc = msg.nc
		m.ready = true
		return m, m.waitForNextHeartbeat() // Start waiting for heartbeats

	case heartbeatMsg:
		m.appendHeartbeat(msg)
		return m, m.waitForNextHeartbeat() // Subscribe again

	case leaseMsg:
		m.lease = msg.Lease
		m.updateProgressBar()
		return m, m.waitForLeaseTick() // Poll again

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case errMsg:
		m.err = msg.err
		return m, nil

	case tickMsg:
		// General tick for periodic updates
		return m, m.waitForNextHeartbeat()

	case tea.QuitMsg:
		m.quitting = true
		if m.nc != nil {
			m.nc.Close()
		}
		return m, tea.Quit
	}

	return m, tea.Batch(cmds...)
}

// View renders the model to a string for display
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.\n", m.err)
	}

	if !m.ready {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.spinner.View()+" Connecting to agent "+m.agentID+"...",
		)
	}

	if m.quitting {
		return "Stopped watching.\n"
	}

	// Build layout
	header := m.renderHeader()
	heartbeatSection := m.renderHeartbeatSection()
	leaseSection := m.renderLeaseSection()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		heartbeatSection,
		"",
		leaseSection,
		"",
		footer,
	)
}

// connectToNATS connects to NATS and returns a command to subscribe
func (m *Model) connectToNATS() tea.Cmd {
	return func() tea.Msg {
		nc, err := nats.Connect(m.natsURL,
			nats.Name("agentd-watch-tui"),
			nats.Timeout(5*time.Second),
		)
		if err != nil {
			return errMsg{err}
		}

		// Return connection as message (don't modify model here!)
		return connectedMsg{nc: nc}
	}
}

// waitForNextHeartbeat waits for the next heartbeat message
func (m *Model) waitForNextHeartbeat() tea.Cmd {
	return func() tea.Msg {
		if m.nc == nil {
			return nil
		}

		subject := fmt.Sprintf("%s.%s", heartbeat.SubjectPrefix, m.agentID)

		// Subscribe with timeout
		sub, err := m.nc.SubscribeSync(subject)
		if err != nil {
			return errMsg{err}
		}
		defer func() { _ = sub.Unsubscribe() }()

		msg, err := sub.NextMsg(60 * time.Second)
		if err != nil {
			if err == nats.ErrTimeout {
				// Normal - no heartbeat yet, try again
				return tickMsg(time.Now())
			}
			return errMsg{err}
		}

		var hb heartbeat.Message
		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			return errMsg{err}
		}

		return heartbeatMsg{
			AgentID:   hb.AgentID,
			Timestamp: hb.Timestamp,
			Lag:       time.Since(hb.Timestamp),
			Status:    hb.Status,
		}
	}
}

// startLeaseMonitor starts the lease monitoring ticker
func (m *Model) startLeaseMonitor() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		lease, err := session.ReadLease(m.agentID)
		if err != nil {
			if err == session.ErrLeaseNotFound {
				return leaseMsg{Lease: nil} // Detached
			}
			return errMsg{err}
		}
		return leaseMsg{Lease: lease}
	})
}

// waitForLeaseTick waits for the next lease poll tick
func (m *Model) waitForLeaseTick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		lease, err := session.ReadLease(m.agentID)
		if err != nil {
			if err == session.ErrLeaseNotFound {
				return leaseMsg{Lease: nil} // Detached
			}
			return errMsg{err}
		}
		return leaseMsg{Lease: lease}
	})
}

// appendHeartbeat adds a heartbeat to the log
func (m *Model) appendHeartbeat(msg heartbeatMsg) {
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] 💓 Heartbeat (lag=%s, status=%s)",
		timestamp,
		formatDuration(msg.Lag),
		msg.Status,
	)

	m.heartbeats = append(m.heartbeats, line)

	// Keep last 100 heartbeats
	if len(m.heartbeats) > 100 {
		m.heartbeats = m.heartbeats[len(m.heartbeats)-100:]
	}

	// Update viewport content
	m.viewport.SetContent(strings.Join(m.heartbeats, "\n"))
	m.viewport.GotoBottom()
}

// updateProgressBar updates the progress bar based on lease TTL
func (m *Model) updateProgressBar() {
	if m.lease == nil {
		return
	}

	// Calculate progress (0.0 to 1.0)
	total := m.lease.ExpiresAt.Sub(m.lease.AttachedAt)
	remaining := time.Until(m.lease.ExpiresAt)
	progress := float64(remaining) / float64(total)

	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	m.progress.SetPercent(progress)
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		if m.nc != nil {
			m.nc.Close()
		}
		return m, tea.Quit

	case "r":
		// Refresh - reconnect to NATS
		return m, m.connectToNATS()
	}

	return m, nil
}

// renderHeader renders the header section
func (m Model) renderHeader() string {
	status := "⚡ RUNNING"
	statusColor := m.theme.Success
	if m.lease == nil {
		status = "💤 DETACHED"
		statusColor = m.theme.Warning
	}

	title := lipgloss.NewStyle().
		Foreground(m.theme.Primary).
		Bold(true).
		Render("AGENT MONITOR")

	statusStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true).
		Render(status)

	agentLine := fmt.Sprintf("Agent: %s    [%s]", m.agentID, statusStyle)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, agentLine))
}

// renderHeartbeatSection renders the heartbeat log section
func (m Model) renderHeartbeatSection() string {
	sectionTitle := lipgloss.NewStyle().
		Foreground(m.theme.Secondary).
		Bold(true).
		Render("💓 HEARTBEAT STREAM")

	viewportBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Secondary).
		Padding(0, 1).
		Render(m.viewport.View())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		sectionTitle,
		viewportBox,
	)
}

// renderLeaseSection renders the lease status section
func (m Model) renderLeaseSection() string {
	if m.lease == nil {
		return lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Render("🔓 No active lease")
	}

	timeUntil := time.Until(m.lease.ExpiresAt)
	label := fmt.Sprintf("🔐 Lease expires in: %s", formatDuration(timeUntil))

	labelStyle := lipgloss.NewStyle().
		Foreground(m.theme.Accent).
		Render(label)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		labelStyle,
		m.progress.View(),
	)
}

// renderFooter renders the footer with keyboard shortcuts
func (m Model) renderFooter() string {
	return lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render("[q]uit  [r]efresh")
}

// formatDuration formats a duration as a human-readable string
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
