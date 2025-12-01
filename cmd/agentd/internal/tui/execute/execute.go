// Package execute provides a Bubble Tea TUI for the execute command.
package execute

import (
	"fmt"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Phase represents execution phase.
type Phase int

const (
	PhaseConnecting Phase = iota
	PhaseExecuting
	PhaseDone
)

// Event types
type (
	// ConnectedMsg signals connection to container.
	ConnectedMsg struct {
		ContainerID string
	}

	// ExecutingMsg signals command is running.
	ExecutingMsg struct{}

	// OutputMsg contains command output.
	OutputMsg struct {
		Stdout string
		Stderr string
	}

	// CompleteMsg signals command completed.
	CompleteMsg struct {
		ExitCode int
	}

	// ErrorMsg signals an error occurred.
	ErrorMsg struct {
		Error error
	}

	// TickMsg triggers spinner updates.
	TickMsg time.Time
)

// Model is the execute TUI model.
type Model struct {
	th       *theme.Theme
	help     help.Model
	keys     keyMap
	spinner  spinner.Model
	viewport viewport.Model

	agentID     string
	command     string
	containerID string

	phase    Phase
	done     bool
	err      error
	exitCode int

	stdout string
	stderr string

	width  int
	height int
}

type keyMap struct {
	keys.Common
	Scroll key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Common: keys.NewCommon(),
		Scroll: key.NewBinding(
			key.WithKeys("up", "down", "pgup", "pgdown"),
			key.WithHelp("↑/↓", "scroll"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Scroll, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Scroll, k.Quit}}
}

// New creates a new execute TUI model.
func New(agentID, command string) Model {
	th := theme.Default()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(th.Primary)

	vp := viewport.New(80, 10)

	return Model{
		th:       th,
		help:     help.New(),
		keys:     newKeyMap(),
		spinner:  s,
		viewport: vp,
		agentID:  agentID,
		command:  command,
		phase:    PhaseConnecting,
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
		// Reserve space for header (4 lines), status (2 lines), help (1 line)
		vpHeight := msg.Height - 8
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = vpHeight

	case ConnectedMsg:
		m.containerID = msg.ContainerID
		m.phase = PhaseExecuting

	case ExecutingMsg:
		m.phase = PhaseExecuting

	case OutputMsg:
		m.stdout = msg.Stdout
		m.stderr = msg.Stderr
		m.updateViewport()

	case CompleteMsg:
		m.exitCode = msg.ExitCode
		m.done = true
		m.phase = PhaseDone

	case ErrorMsg:
		m.err = msg.Error
		m.done = true
		m.phase = PhaseDone

	case TickMsg:
		if !m.done {
			cmds = append(cmds, tickCmd())
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Handle viewport scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateViewport() {
	var content strings.Builder

	if m.stdout != "" {
		content.WriteString(m.stdout)
	}

	if m.stderr != "" {
		if content.Len() > 0 {
			content.WriteString("\n")
		}
		errStyle := lipgloss.NewStyle().Foreground(m.th.Error)
		content.WriteString(errStyle.Render(m.stderr))
	}

	if content.Len() == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		content.WriteString(mutedStyle.Render("(no output)"))
	}

	m.viewport.SetContent(content.String())
}

// View renders the TUI.
func (m Model) View() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	b.WriteString(headerStyle.Render(fmt.Sprintf("⚡ Execute on agent '%s'", m.agentID)))
	b.WriteString("\n")

	// Container and command info
	mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
	if m.containerID != "" {
		shortID := m.containerID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		b.WriteString(mutedStyle.Render(fmt.Sprintf("   Container: %s", shortID)))
		b.WriteString("\n")
	}

	// Truncate command for display
	cmdDisplay := m.command
	if len(cmdDisplay) > 60 {
		cmdDisplay = cmdDisplay[:57] + "..."
	}
	b.WriteString(mutedStyle.Render(fmt.Sprintf("   Command: %s", cmdDisplay)))
	b.WriteString("\n\n")

	// Status
	switch m.phase {
	case PhaseConnecting:
		b.WriteString(fmt.Sprintf("  %s Connecting to container...\n", m.spinner.View()))
	case PhaseExecuting:
		b.WriteString(fmt.Sprintf("  %s Executing command...\n", m.spinner.View()))
	case PhaseDone:
		// Show nothing here, result shown below
	}

	// Output section
	if m.phase == PhaseExecuting || m.phase == PhaseDone {
		successStyle := lipgloss.NewStyle().Foreground(m.th.Success)
		b.WriteString("\n")
		b.WriteString(successStyle.Render("─── Output ───"))
		b.WriteString("\n")

		// Viewport content
		vpStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.th.Muted).
			Padding(0, 1)
		b.WriteString(vpStyle.Render(m.viewport.View()))
		b.WriteString("\n")
		b.WriteString(successStyle.Render("─────────────"))
		b.WriteString("\n")
	}

	// Result
	if m.done {
		b.WriteString("\n")
		if m.err != nil {
			errStyle := lipgloss.NewStyle().Foreground(m.th.Error).Bold(true)
			b.WriteString(errStyle.Render(fmt.Sprintf("✗ Error: %v", m.err)))
		} else if m.exitCode != 0 {
			errStyle := lipgloss.NewStyle().Foreground(m.th.Error).Bold(true)
			b.WriteString(errStyle.Render(fmt.Sprintf("✗ Command failed with exit code %d", m.exitCode)))
		} else {
			successStyle := lipgloss.NewStyle().Foreground(m.th.Success).Bold(true)
			b.WriteString(successStyle.Render("✓ Command completed successfully"))
		}
		b.WriteString("\n")
	}

	// Help
	if !m.done {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Press q to cancel"))
	} else {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Press q to exit"))
	}

	return b.String()
}

// SendConnected sends a connected message.
func SendConnected(containerID string) tea.Cmd {
	return func() tea.Msg {
		return ConnectedMsg{ContainerID: containerID}
	}
}

// SendExecuting sends an executing message.
func SendExecuting() tea.Cmd {
	return func() tea.Msg {
		return ExecutingMsg{}
	}
}

// SendOutput sends output message.
func SendOutput(stdout, stderr string) tea.Cmd {
	return func() tea.Msg {
		return OutputMsg{Stdout: stdout, Stderr: stderr}
	}
}

// SendComplete sends complete message.
func SendComplete(exitCode int) tea.Cmd {
	return func() tea.Msg {
		return CompleteMsg{ExitCode: exitCode}
	}
}

// SendError sends error message.
func SendError(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Error: err}
	}
}
