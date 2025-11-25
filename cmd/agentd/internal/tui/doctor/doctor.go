// Package doctor provides a Bubble Tea TUI for the doctor command.
package doctor

import (
	"fmt"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CheckStatus represents check completion status.
type CheckStatus int

const (
	StatusPending CheckStatus = iota
	StatusRunning
	StatusPassed
	StatusSkipped
	StatusFailed
)

// CheckInfo describes a diagnostic check.
type CheckInfo struct {
	Name    string
	Status  CheckStatus
	Message string
	Error   string
}

// Event types
type (
	// CheckStartMsg signals a check is starting.
	CheckStartMsg struct {
		Index int
	}

	// CheckPassMsg signals a check passed.
	CheckPassMsg struct {
		Index   int
		Message string
	}

	// CheckSkipMsg signals a check was skipped.
	CheckSkipMsg struct {
		Index   int
		Message string
	}

	// CheckFailMsg signals a check failed.
	CheckFailMsg struct {
		Index int
		Error string
	}

	// AllChecksCompleteMsg signals all checks are done.
	AllChecksCompleteMsg struct {
		AllPassed bool
	}

	// TickMsg triggers spinner updates.
	TickMsg time.Time
)

// Model is the doctor TUI model.
type Model struct {
	th      *theme.Theme
	help    help.Model
	keys    keyMap
	spinner spinner.Model

	checks    []CheckInfo
	current   int
	done      bool
	allPassed bool

	width  int
	height int
}

type keyMap struct {
	keys.Common
}

func newKeyMap() keyMap {
	return keyMap{
		Common: keys.NewCommon(),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Quit}}
}

// New creates a new doctor TUI model.
func New(checkNames []string) Model {
	th := theme.Default()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(th.Primary)

	checks := make([]CheckInfo, len(checkNames))
	for i, name := range checkNames {
		checks[i] = CheckInfo{
			Name:   name,
			Status: StatusPending,
		}
	}

	return Model{
		th:      th,
		help:    help.New(),
		keys:    newKeyMap(),
		spinner: s,
		checks:  checks,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
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
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case CheckStartMsg:
		if msg.Index < len(m.checks) {
			m.checks[msg.Index].Status = StatusRunning
			m.current = msg.Index
		}

	case CheckPassMsg:
		if msg.Index < len(m.checks) {
			m.checks[msg.Index].Status = StatusPassed
			m.checks[msg.Index].Message = msg.Message
		}

	case CheckSkipMsg:
		if msg.Index < len(m.checks) {
			m.checks[msg.Index].Status = StatusSkipped
			m.checks[msg.Index].Message = msg.Message
		}

	case CheckFailMsg:
		if msg.Index < len(m.checks) {
			m.checks[msg.Index].Status = StatusFailed
			m.checks[msg.Index].Error = msg.Error
		}

	case AllChecksCompleteMsg:
		m.done = true
		m.allPassed = msg.AllPassed

	case TickMsg:
		if !m.done {
			cmds = append(cmds, tickCmd())
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the TUI.
func (m Model) View() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	b.WriteString(headerStyle.Render("Validating environment"))
	b.WriteString("\n\n")

	// Checks
	for i, check := range m.checks {
		b.WriteString(m.renderCheck(i, check))
		b.WriteString("\n")
	}

	// Result summary
	if m.done {
		b.WriteString("\n")
		b.WriteString(m.renderSummary())
	}

	// Help
	if !m.done {
		b.WriteString("\n")
		mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		b.WriteString(mutedStyle.Render("Press q to cancel"))
	}

	return b.String()
}

func (m Model) renderCheck(index int, check CheckInfo) string {
	var icon string
	var style lipgloss.Style

	switch check.Status {
	case StatusPending:
		icon = "○"
		style = lipgloss.NewStyle().Foreground(m.th.Muted)
	case StatusRunning:
		icon = m.spinner.View()
		style = lipgloss.NewStyle().Foreground(m.th.Primary)
	case StatusPassed:
		icon = "✓"
		style = lipgloss.NewStyle().Foreground(m.th.Success)
	case StatusSkipped:
		icon = "⊘"
		style = lipgloss.NewStyle().Foreground(m.th.Muted)
	case StatusFailed:
		icon = "✗"
		style = lipgloss.NewStyle().Foreground(m.th.Error)
	}

	line := fmt.Sprintf("%s %s", icon, check.Name)
	if check.Message != "" {
		mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		line += " " + mutedStyle.Render(fmt.Sprintf("(%s)", check.Message))
	}
	if check.Error != "" {
		line += fmt.Sprintf(": %s", check.Error)
	}

	return style.Render(line)
}

func (m Model) renderSummary() string {
	var passed, skipped, failed int
	for _, check := range m.checks {
		switch check.Status {
		case StatusPassed:
			passed++
		case StatusSkipped:
			skipped++
		case StatusFailed:
			failed++
		}
	}

	if m.allPassed {
		successStyle := lipgloss.NewStyle().Foreground(m.th.Success).Bold(true)
		mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		return successStyle.Render("✨ Environment ready!") + " " + mutedStyle.Render("All systems go for spawning agents.")
	}

	errorStyle := lipgloss.NewStyle().Foreground(m.th.Error).Bold(true)
	return errorStyle.Render(fmt.Sprintf("✗ Validation failed: %d check(s) failed", failed))
}

// SendCheckStart sends a check start message.
func SendCheckStart(index int) tea.Cmd {
	return func() tea.Msg {
		return CheckStartMsg{Index: index}
	}
}

// SendCheckPass sends a check pass message.
func SendCheckPass(index int, message string) tea.Cmd {
	return func() tea.Msg {
		return CheckPassMsg{Index: index, Message: message}
	}
}

// SendCheckSkip sends a check skip message.
func SendCheckSkip(index int, message string) tea.Cmd {
	return func() tea.Msg {
		return CheckSkipMsg{Index: index, Message: message}
	}
}

// SendCheckFail sends a check fail message.
func SendCheckFail(index int, err string) tea.Cmd {
	return func() tea.Msg {
		return CheckFailMsg{Index: index, Error: err}
	}
}

// SendAllChecksComplete signals all checks are done.
func SendAllChecksComplete(allPassed bool) tea.Cmd {
	return func() tea.Msg {
		return AllChecksCompleteMsg{AllPassed: allPassed}
	}
}
