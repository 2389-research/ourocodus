// Package spawn provides a Bubble Tea TUI for the spawn command.
package spawn

import (
	"fmt"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Step represents a spawn step.
type Step int

const (
	StepCheckExisting Step = iota
	StepCreateLauncher
	StepBuildConfig
	StepSpawnAgent
	StepGenerateToken
	StepDone
)

// StepStatus represents step completion status.
type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusComplete
	StatusError
)

// StepInfo describes a step.
type StepInfo struct {
	Name   string
	Status StepStatus
	Error  string
}

// Event types
type (
	// StepStartMsg signals a step is starting.
	StepStartMsg struct {
		Step Step
	}

	// StepCompleteMsg signals a step completed.
	StepCompleteMsg struct {
		Step Step
	}

	// StepErrorMsg signals a step failed.
	StepErrorMsg struct {
		Step  Step
		Error error
	}

	// SpawnCompleteMsg signals spawn is done.
	SpawnCompleteMsg struct {
		Handle      *container.AgentContainerHandle
		AttachToken string
		TokenError  error
	}

	// TickMsg triggers spinner updates.
	TickMsg time.Time
)

// Model is the spawn TUI model.
type Model struct {
	th      *theme.Theme
	help    help.Model
	keys    keyMap
	spinner spinner.Model

	agentID string
	steps   []StepInfo
	current Step
	done    bool
	err     error

	// Result data
	handle      *container.AgentContainerHandle
	attachToken string
	tokenError  error

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

// New creates a new spawn TUI model.
func New(agentID string) Model {
	th := theme.Default()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(th.Primary)

	steps := []StepInfo{
		{Name: "Checking for existing agent", Status: StatusPending},
		{Name: "Creating launcher", Status: StatusPending},
		{Name: "Building spawn config", Status: StatusPending},
		{Name: "Spawning agent container", Status: StatusPending},
		{Name: "Generating attach token", Status: StatusPending},
	}

	return Model{
		th:      th,
		help:    help.New(),
		keys:    newKeyMap(),
		spinner: s,
		agentID: agentID,
		steps:   steps,
		current: StepCheckExisting,
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

	case StepStartMsg:
		if int(msg.Step) < len(m.steps) {
			m.steps[msg.Step].Status = StatusRunning
			m.current = msg.Step
		}

	case StepCompleteMsg:
		if int(msg.Step) < len(m.steps) {
			m.steps[msg.Step].Status = StatusComplete
		}

	case StepErrorMsg:
		if int(msg.Step) < len(m.steps) {
			m.steps[msg.Step].Status = StatusError
			m.steps[msg.Step].Error = msg.Error.Error()
		}
		m.err = msg.Error
		m.done = true

	case SpawnCompleteMsg:
		m.handle = msg.Handle
		m.attachToken = msg.AttachToken
		m.tokenError = msg.TokenError
		m.done = true
		// Mark all steps complete
		for i := range m.steps {
			if m.steps[i].Status != StatusError {
				m.steps[i].Status = StatusComplete
			}
		}

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
	b.WriteString(headerStyle.Render(fmt.Sprintf("✨ Spawning agent '%s'", m.agentID)))
	b.WriteString("\n\n")

	// Steps
	for i, step := range m.steps {
		b.WriteString(m.renderStep(i, step))
		b.WriteString("\n")
	}

	// Result or error
	if m.done {
		b.WriteString("\n")
		if m.err != nil {
			b.WriteString(m.renderError())
		} else {
			b.WriteString(m.renderSuccess())
		}
	}

	// Help (only show quit hint while running)
	if !m.done {
		b.WriteString("\n")
		mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		b.WriteString(mutedStyle.Render("Press q to cancel"))
	}

	return b.String()
}

func (m Model) renderStep(index int, step StepInfo) string {
	var icon string
	var style lipgloss.Style

	switch step.Status {
	case StatusPending:
		icon = "○"
		style = lipgloss.NewStyle().Foreground(m.th.Muted)
	case StatusRunning:
		icon = m.spinner.View()
		style = lipgloss.NewStyle().Foreground(m.th.Primary)
	case StatusComplete:
		icon = "✓"
		style = lipgloss.NewStyle().Foreground(m.th.Success)
	case StatusError:
		icon = "✗"
		style = lipgloss.NewStyle().Foreground(m.th.Error)
	}

	line := fmt.Sprintf("  %s %s", icon, step.Name)
	if step.Error != "" {
		line += fmt.Sprintf(" - %s", step.Error)
	}

	return style.Render(line)
}

func (m Model) renderError() string {
	errStyle := lipgloss.NewStyle().Foreground(m.th.Error).Bold(true)
	return errStyle.Render(fmt.Sprintf("✗ Spawn failed: %v", m.err))
}

func (m Model) renderSuccess() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(m.th.Accent)
	mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
	successStyle := lipgloss.NewStyle().Foreground(m.th.Success)

	if m.handle == nil {
		return successStyle.Render("✓ Agent spawned successfully")
	}

	// Worktree
	b.WriteString("🌳 ")
	b.WriteString(labelStyle.Render("Worktree: "))
	b.WriteString(valueStyle.Render(m.handle.WorkspacePath()) + " ")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("(branch: %s)", m.handle.BranchName())))
	b.WriteString("\n")

	// Container
	b.WriteString("📦 ")
	b.WriteString(labelStyle.Render("Container: "))
	b.WriteString(valueStyle.Render(m.handle.ContainerID()) + " ")
	b.WriteString(successStyle.Render("(running)"))
	b.WriteString("\n")

	// Credentials
	b.WriteString("🔑 ")
	b.WriteString(labelStyle.Render("Credentials: "))
	credPath := m.handle.CredentialsPath()
	if credPath != "" {
		b.WriteString(valueStyle.Render("mounted at /root/.creds "))
		b.WriteString(mutedStyle.Render("(read-only)"))
	} else {
		b.WriteString(mutedStyle.Render("(none)"))
	}
	b.WriteString("\n")

	// Attach token
	if m.attachToken != "" {
		b.WriteString("\n")
		b.WriteString("🔐 ")
		tokenLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("#00F6FF")).Bold(true)
		b.WriteString(tokenLabel.Render("Attach Token:"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("   %s\n", valueStyle.Render(m.attachToken)))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("   Use this token when attaching from PWA or relay:"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(fmt.Sprintf("   → agent:attach {\"agentId\": \"%s\", \"token\": \"<token>\"}", m.handle.AgentID())))
		b.WriteString("\n")
	} else if m.tokenError != nil {
		warnStyle := lipgloss.NewStyle().Foreground(m.th.Warning)
		b.WriteString("\n")
		b.WriteString(warnStyle.Render(fmt.Sprintf("⚠️  Warning: Failed to generate attach token: %v", m.tokenError)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(successStyle.Bold(true).Render(fmt.Sprintf("✓ Agent %s ready", m.handle.AgentID())))

	return b.String()
}

// SendStepStart sends a step start message.
func SendStepStart(step Step) tea.Cmd {
	return func() tea.Msg {
		return StepStartMsg{Step: step}
	}
}

// SendStepComplete sends a step complete message.
func SendStepComplete(step Step) tea.Cmd {
	return func() tea.Msg {
		return StepCompleteMsg{Step: step}
	}
}

// SendStepError sends a step error message.
func SendStepError(step Step, err error) tea.Cmd {
	return func() tea.Msg {
		return StepErrorMsg{Step: step, Error: err}
	}
}

// SendSpawnComplete sends spawn complete message.
func SendSpawnComplete(handle *container.AgentContainerHandle, token string, tokenErr error) tea.Cmd {
	return func() tea.Msg {
		return SpawnCompleteMsg{
			Handle:      handle,
			AttachToken: token,
			TokenError:  tokenErr,
		}
	}
}
