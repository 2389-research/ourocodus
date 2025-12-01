// Package tui provides a Bubble Tea TUI for the relay server.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay"
	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogEntry represents a log entry with level and message.
type LogEntry struct {
	Time    time.Time
	Level   string
	Tag     string
	Message string
}

// StatsMsg is sent periodically with updated stats.
type StatsMsg struct {
	SessionCount int
	AgentCount   int
}

// LogMsg is sent when a new log entry arrives.
type LogMsg LogEntry

// ShutdownMsg signals shutdown has started.
type ShutdownMsg struct{}

// TickMsg triggers periodic updates.
type TickMsg time.Time

// Model is the main relay TUI model.
type Model struct {
	// Theme and styles
	th *theme.Theme

	// Components
	viewport viewport.Model
	help     help.Model
	keys     keyMap

	// State
	logs          []LogEntry
	maxLogs       int
	sessionCount  int
	agentCount    int
	natsConnected bool
	dockerOK      bool
	ready         bool
	quitting      bool
	shuttingDown  bool

	// Dimensions
	width  int
	height int

	// Server info
	port int

	// Session manager for stats
	sessionManager relay.SessionManagerInterface
}

// keyMap defines key bindings.
type keyMap struct {
	keys.Navigation
	Clear        key.Binding
	ToggleFollow key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Navigation: keys.NewNavigation(),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear logs"),
		),
		ToggleFollow: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "toggle follow"),
		),
	}
}

// ShortHelp returns short help.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Clear, k.Quit}
}

// FullHelp returns full help.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.Home, k.End, k.ToggleFollow, k.Clear},
		{k.Help, k.Quit},
	}
}

// Config holds TUI configuration.
type Config struct {
	Port           int
	NATSConnected  bool
	DockerOK       bool
	SessionManager relay.SessionManagerInterface
}

// New creates a new relay TUI model.
func New(cfg Config) Model {
	th := theme.Default()

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Primary)

	return Model{
		th:             th,
		viewport:       vp,
		help:           help.New(),
		keys:           newKeyMap(),
		maxLogs:        1000,
		port:           cfg.Port,
		natsConnected:  cfg.NATSConnected,
		dockerOK:       cfg.DockerOK,
		sessionManager: cfg.SessionManager,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		statsCmd(m.sessionManager),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func statsCmd(sm relay.SessionManagerInterface) tea.Cmd {
	return func() tea.Msg {
		if sm == nil {
			return StatsMsg{SessionCount: 0, AgentCount: 0}
		}
		sessions := sm.List(nil)
		agentCount := 0
		for _, s := range sessions {
			agentCount += s.AgentCount()
		}
		return StatsMsg{
			SessionCount: len(sessions),
			AgentCount:   agentCount,
		}
	}
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
			m.logs = nil
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

		// Calculate viewport size (minus header, status bar, help)
		headerHeight := 10 // logo + status
		footerHeight := 3  // status bar + help
		vpHeight := m.height - headerHeight - footerHeight
		if vpHeight < 5 {
			vpHeight = 5
		}

		m.viewport.Width = m.width - 4
		m.viewport.Height = vpHeight
		m.updateViewport()

	case LogMsg:
		entry := LogEntry(msg)
		m.logs = append(m.logs, entry)
		if len(m.logs) > m.maxLogs {
			m.logs = m.logs[len(m.logs)-m.maxLogs:]
		}
		m.updateViewport()
		// Auto-scroll to bottom
		m.viewport.GotoBottom()

	case StatsMsg:
		m.sessionCount = msg.SessionCount
		m.agentCount = msg.AgentCount
		// Schedule next stats update
		cmds = append(cmds, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return statsCmd(m.sessionManager)()
		}))

	case ShutdownMsg:
		m.shuttingDown = true

	case TickMsg:
		cmds = append(cmds, tickCmd())
	}

	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewport() {
	var lines []string
	for _, entry := range m.logs {
		line := m.formatLogEntry(entry)
		lines = append(lines, line)
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m *Model) formatLogEntry(entry LogEntry) string {
	timeStyle := lipgloss.NewStyle().Foreground(m.th.Muted)

	var tagStyle lipgloss.Style
	switch entry.Tag {
	case "INIT", "ACP":
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Success).Bold(true)
	case "SHUTDOWN":
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Warning).Bold(true)
	case "ERROR":
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Error).Bold(true)
	case "NATS", "HEARTBEAT", "RELAY", "SERVER":
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	case "SESSION":
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Secondary).Bold(true)
	case "CONTAINER", "LEASE":
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Accent).Bold(true)
	case "CLEANUP", "AUDIT":
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Muted).Bold(true)
	case "RATELIMIT", "SECURITY":
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Warning).Bold(true)
	default:
		tagStyle = lipgloss.NewStyle().Foreground(m.th.Muted)
	}

	timestamp := entry.Time.Format("15:04:05")
	tag := ""
	if entry.Tag != "" {
		tag = tagStyle.Render("["+entry.Tag+"]") + " "
	}

	return fmt.Sprintf("%s %s%s",
		timeStyle.Render(timestamp),
		tag,
		entry.Message,
	)
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return m.renderShutdown()
	}

	var b strings.Builder

	// Header with logo and status
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Log viewport
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	// Help
	b.WriteString(m.help.View(m.keys))

	return b.String()
}

func (m Model) renderHeader() string {
	// Rainbow logo
	logo := ` ▄▄ ▗  ▖▗▄▄  ▄▄  ▗▄  ▄▄ ▗▄▖ ▗  ▖ ▄▄
▗▘▝▖▐  ▌▐ ▝▌▗▘▝▖▗▘ ▘▗▘▝▖▐ ▝▖▐  ▌▐▘ ▘
▐  ▌▐  ▌▐▄▄▘▐  ▌▐   ▐  ▌▐  ▌▐  ▌▝▙▄
▐  ▌▐  ▌▐▗▖ ▐  ▌▐   ▐  ▌▐  ▌▐  ▌  ▝▖
▝▙▟▘▝▄▄▘▐ ▝▖▝▙▟▘▝▙▄▐▝▙▟▘▐▄▟▘▝▄▄▘▝▄▟▘`

	rainbowColors := []lipgloss.Color{
		lipgloss.Color("#FF5555"),
		lipgloss.Color("#FFB86C"),
		lipgloss.Color("#F1FA8C"),
		lipgloss.Color("#50FA7B"),
		lipgloss.Color("#8BE9FD"),
	}

	lines := strings.Split(logo, "\n")
	var coloredLines []string
	for i, line := range lines {
		color := rainbowColors[i%len(rainbowColors)]
		coloredLine := lipgloss.NewStyle().Foreground(color).Render(line)
		coloredLines = append(coloredLines, coloredLine)
	}

	logoBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.th.Primary).
		Padding(0, 1).
		Render(strings.Join(coloredLines, "\n"))

	// Status info
	labelStyle := lipgloss.NewStyle().Foreground(m.th.Muted).Width(12)
	urlStyle := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(m.th.Success)
	warningStyle := lipgloss.NewStyle().Foreground(m.th.Warning)

	var status strings.Builder
	status.WriteString(fmt.Sprintf("%s %s\n",
		labelStyle.Render("PWA:"),
		urlStyle.Render(fmt.Sprintf("http://localhost:%d/", m.port))))
	status.WriteString(fmt.Sprintf("%s %s\n",
		labelStyle.Render("WebSocket:"),
		urlStyle.Render(fmt.Sprintf("ws://localhost:%d/ws", m.port))))

	natsStatus := warningStyle.Render("disabled")
	if m.natsConnected {
		natsStatus = successStyle.Render("connected")
	}
	status.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("NATS:"), natsStatus))

	dockerStatus := warningStyle.Render("error")
	if m.dockerOK {
		dockerStatus = successStyle.Render("connected")
	}
	status.WriteString(fmt.Sprintf("%s %s", labelStyle.Render("Docker:"), dockerStatus))

	// Combine logo and status side by side
	return lipgloss.JoinHorizontal(lipgloss.Top,
		logoBox,
		"  ",
		status.String(),
	)
}

func (m Model) renderStatusBar() string {
	labelStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
	valueStyle := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)

	status := fmt.Sprintf("%s %s  │  %s %s  │  %s %s",
		labelStyle.Render("Sessions:"),
		valueStyle.Render(fmt.Sprintf("%d", m.sessionCount)),
		labelStyle.Render("Agents:"),
		valueStyle.Render(fmt.Sprintf("%d", m.agentCount)),
		labelStyle.Render("Logs:"),
		valueStyle.Render(fmt.Sprintf("%d", len(m.logs))),
	)

	if m.shuttingDown {
		shutdownStyle := lipgloss.NewStyle().Foreground(m.th.Warning).Bold(true)
		status += "  │  " + shutdownStyle.Render("⏹ SHUTTING DOWN")
	}

	return status
}

func (m Model) renderShutdown() string {
	shutdownStyle := lipgloss.NewStyle().
		Foreground(m.th.Warning).
		Bold(true)

	return fmt.Sprintf("\n  %s\n\n", shutdownStyle.Render("⏹  Shutting down gracefully..."))
}

// AddLog adds a log entry to the TUI. Thread-safe via tea.Cmd.
func AddLog(entry LogEntry) tea.Cmd {
	return func() tea.Msg {
		return LogMsg(entry)
	}
}

// ParseLogLine parses a log line and extracts tag and message.
func ParseLogLine(line string) LogEntry {
	entry := LogEntry{
		Time:    time.Now(),
		Message: line,
	}

	// Extract tag like [INIT], [SHUTDOWN], etc.
	if idx := strings.Index(line, "["); idx != -1 {
		if endIdx := strings.Index(line[idx:], "]"); endIdx != -1 {
			entry.Tag = line[idx+1 : idx+endIdx]
			entry.Message = strings.TrimSpace(line[idx+endIdx+1:])
		}
	}

	return entry
}
