// Package watch provides a Bubble Tea TUI for the watch command.
package watch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Event types
type (
	// HeartbeatMsg is sent when a heartbeat is received.
	HeartbeatMsg struct {
		AgentID   string
		Timestamp time.Time
		Status    string
		Lag       time.Duration
	}

	// LeaseChangeMsg is sent when lease state changes.
	LeaseChangeMsg struct {
		Lease *session.Lease // nil means detached
	}

	// LogLineMsg is sent when a log line is received.
	LogLineMsg struct {
		Line  string
		IsErr bool
	}

	// ErrorMsg is sent on errors.
	ErrorMsg struct {
		Err error
	}

	// SubscribedMsg indicates successful subscription.
	SubscribedMsg struct {
		Subject string
	}

	// TickMsg triggers periodic updates.
	TickMsg time.Time
)

// Model is the watch TUI model.
type Model struct {
	th       *theme.Theme
	viewport viewport.Model
	help     help.Model
	keys     keyMap

	agentID    string
	subject    string
	events     []string
	maxEvents  int
	ready      bool
	quitting   bool
	subscribed bool

	width  int
	height int
}

type keyMap struct {
	keys.Navigation
	Clear key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Navigation: keys.NewNavigation(),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Clear, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.Home, k.End, k.Clear},
		{k.Help, k.Quit},
	}
}

// New creates a new watch TUI model.
func New(agentID string) Model {
	th := theme.Default()

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle()

	return Model{
		th:        th,
		viewport:  vp,
		help:      help.New(),
		keys:      newKeyMap(),
		agentID:   agentID,
		maxEvents: 500,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Clear):
			m.events = nil
			m.updateViewport()
		case key.Matches(msg, m.keys.Home):
			m.viewport.GotoTop()
		case key.Matches(msg, m.keys.End):
			m.viewport.GotoBottom()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		headerHeight := 5 // header info
		footerHeight := 3 // help
		vpHeight := m.height - headerHeight - footerHeight
		if vpHeight < 5 {
			vpHeight = 5
		}

		m.viewport.Width = m.width - 2
		m.viewport.Height = vpHeight
		m.updateViewport()

	case SubscribedMsg:
		m.subscribed = true
		m.subject = msg.Subject
		m.addEvent(m.formatSubscribed(msg.Subject))

	case HeartbeatMsg:
		m.addEvent(m.formatHeartbeat(msg))

	case LeaseChangeMsg:
		m.addEvent(m.formatLeaseChange(msg.Lease))

	case LogLineMsg:
		m.addEvent(m.formatLogLine(msg))

	case ErrorMsg:
		m.addEvent(m.formatError(msg.Err))

	case TickMsg:
		cmds = append(cmds, tickCmd())
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) addEvent(event string) {
	m.events = append(m.events, event)
	if len(m.events) > m.maxEvents {
		m.events = m.events[len(m.events)-m.maxEvents:]
	}
	m.updateViewport()
	m.viewport.GotoBottom()
}

func (m *Model) updateViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(strings.Join(m.events, "\n"))
}

func (m Model) formatSubscribed(subject string) string {
	successStyle := lipgloss.NewStyle().Foreground(m.th.Success)
	return successStyle.Render(fmt.Sprintf("✓ Subscribed to: %s", subject))
}

func (m Model) formatHeartbeat(hb HeartbeatMsg) string {
	timestamp := time.Now().Format("15:04:05")
	timeStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
	heartStyle := lipgloss.NewStyle().Foreground(m.th.Success)
	lagStyle := lipgloss.NewStyle().Foreground(m.th.Primary)

	return fmt.Sprintf("%s %s Heartbeat (lag=%s, status=%s)",
		timeStyle.Render(timestamp),
		heartStyle.Render("💓"),
		lagStyle.Render(formatDuration(hb.Lag)),
		hb.Status,
	)
}

func (m Model) formatLeaseChange(lease *session.Lease) string {
	timestamp := time.Now().Format("15:04:05")
	timeStyle := lipgloss.NewStyle().Foreground(m.th.Muted)

	if lease == nil {
		warnStyle := lipgloss.NewStyle().Foreground(m.th.Warning)
		return fmt.Sprintf("%s %s Lease detached",
			timeStyle.Render(timestamp),
			warnStyle.Render("🔓"),
		)
	}

	leaseStyle := lipgloss.NewStyle().Foreground(m.th.Primary)
	timeUntil := time.Until(lease.ExpiresAt)
	shortSession := lease.UserSessionID
	if len(shortSession) > 8 {
		shortSession = shortSession[:8]
	}

	return fmt.Sprintf("%s %s Lease renewed (expires in %s, session=%s)",
		timeStyle.Render(timestamp),
		leaseStyle.Render("🔐"),
		formatDuration(timeUntil),
		shortSession,
	)
}

func (m Model) formatLogLine(msg LogLineMsg) string {
	if msg.IsErr {
		errStyle := lipgloss.NewStyle().Foreground(m.th.Error)
		return errStyle.Render(msg.Line)
	}
	return msg.Line
}

func (m Model) formatError(err error) string {
	errStyle := lipgloss.NewStyle().Foreground(m.th.Error).Bold(true)
	return errStyle.Render(fmt.Sprintf("✗ Error: %v", err))
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		warnStyle := lipgloss.NewStyle().Foreground(m.th.Warning)
		return "\n" + warnStyle.Render("Stopped watching.") + "\n"
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Events viewport
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.th.Primary)
	b.WriteString(borderStyle.Render(m.viewport.View()))
	b.WriteString("\n")

	// Help
	b.WriteString(m.help.View(m.keys))

	return b.String()
}

func (m Model) renderHeader() string {
	headerStyle := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)

	header := headerStyle.Render(fmt.Sprintf("👁️  Watching agent: %s", m.agentID))
	hint := mutedStyle.Render("Press q to quit, ? for help")

	return header + "\n" + hint
}

// SendHeartbeat sends a heartbeat message to the TUI.
func SendHeartbeat(agentID string, timestamp time.Time, status string, lag time.Duration) tea.Cmd {
	return func() tea.Msg {
		return HeartbeatMsg{
			AgentID:   agentID,
			Timestamp: timestamp,
			Status:    status,
			Lag:       lag,
		}
	}
}

// SendLeaseChange sends a lease change message to the TUI.
func SendLeaseChange(lease *session.Lease) tea.Cmd {
	return func() tea.Msg {
		return LeaseChangeMsg{Lease: lease}
	}
}

// SendLogLine sends a log line to the TUI.
func SendLogLine(line string, isErr bool) tea.Cmd {
	return func() tea.Msg {
		return LogLineMsg{Line: line, IsErr: isErr}
	}
}

// SendError sends an error to the TUI.
func SendError(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: err}
	}
}

// SendSubscribed notifies the TUI of successful subscription.
func SendSubscribed(subject string) tea.Cmd {
	return func() tea.Msg {
		return SubscribedMsg{Subject: subject}
	}
}

// Run starts the watch TUI with the given context and event sources.
func Run(ctx context.Context, agentID string) error {
	m := New(agentID)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
