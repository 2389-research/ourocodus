// Package logview provides a scrollable log viewport component.
package logview

import (
	"fmt"
	"strings"

	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogEntry represents a single log entry.
type LogEntry struct {
	Level   string // "info", "warn", "error", "debug"
	Message string
}

// Model is the log viewport model.
type Model struct {
	viewport   viewport.Model
	th         *theme.Theme
	keys       keys.Navigation
	lines      []string
	maxLines   int
	autoScroll bool
	title      string
	ready      bool
}

// New creates a new log viewport.
func New(th *theme.Theme) Model {
	return Model{
		th:         th,
		keys:       keys.NewNavigation(),
		maxLines:   1000,
		autoScroll: true,
	}
}

// SetTitle sets the viewport title.
func (m *Model) SetTitle(title string) {
	m.title = title
}

// SetMaxLines sets the maximum number of lines to keep.
func (m *Model) SetMaxLines(n int) {
	m.maxLines = n
}

// SetAutoScroll enables or disables auto-scrolling.
func (m *Model) SetAutoScroll(enabled bool) {
	m.autoScroll = enabled
}

// SetSize sets the viewport size.
func (m *Model) SetSize(width, height int) {
	m.viewport.Width = width
	m.viewport.Height = height
	m.ready = true
	m.updateContent()
}

// AppendLine adds a line to the log.
func (m *Model) AppendLine(line string) {
	m.lines = append(m.lines, line)

	// Trim old lines if over max
	if len(m.lines) > m.maxLines {
		m.lines = m.lines[len(m.lines)-m.maxLines:]
	}

	m.updateContent()

	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

// AppendEntry adds a formatted log entry.
func (m *Model) AppendEntry(entry LogEntry) {
	var levelStyle lipgloss.Style

	switch entry.Level {
	case "error":
		levelStyle = lipgloss.NewStyle().Foreground(m.th.Error).Bold(true)
	case "warn":
		levelStyle = lipgloss.NewStyle().Foreground(m.th.Warning)
	case "info":
		levelStyle = lipgloss.NewStyle().Foreground(m.th.Primary)
	case "debug":
		levelStyle = lipgloss.NewStyle().Foreground(m.th.Muted)
	default:
		levelStyle = lipgloss.NewStyle().Foreground(m.th.Muted)
	}

	prefix := levelStyle.Render("[" + strings.ToUpper(entry.Level) + "]")
	m.AppendLine(prefix + " " + entry.Message)
}

// Clear clears all log lines.
func (m *Model) Clear() {
	m.lines = nil
	m.updateContent()
}

func (m *Model) updateContent() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Disable auto-scroll on manual navigation
		switch {
		case msg.String() == "up" || msg.String() == "k" ||
			msg.String() == "pgup" || msg.String() == "ctrl+u":
			m.autoScroll = false
		case msg.String() == "G" || msg.String() == "end":
			m.autoScroll = true
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the log viewport.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	if m.title != "" {
		titleStyle := lipgloss.NewStyle().
			Foreground(m.th.Primary).
			Bold(true).
			Padding(0, 1)
		b.WriteString(titleStyle.Render(m.title))
		b.WriteString("\n")
	}

	// Add border around viewport
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.th.Primary)

	b.WriteString(borderStyle.Render(m.viewport.View()))

	// Scroll indicator
	percent := int(m.viewport.ScrollPercent() * 100)
	scrollInfo := lipgloss.NewStyle().
		Foreground(m.th.Muted).
		Render(fmt.Sprintf("%d%%", percent))

	b.WriteString("\n")
	b.WriteString(scrollInfo)

	return b.String()
}

// ScrollPercent returns scroll percentage.
func (m Model) ScrollPercent() float64 {
	return m.viewport.ScrollPercent()
}

// AtBottom returns true if viewport is at the bottom.
func (m Model) AtBottom() bool {
	return m.viewport.AtBottom()
}
