// Package stop provides a Bubble Tea TUI for the stop command.
package stop

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

// StepType represents a cleanup step.
type StepType int

const (
	StepFindContainer StepType = iota
	StepStopContainer
	StepRemoveWorktree
	StepComplete
)

// StepStatus represents step completion status.
type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusComplete
	StatusSkipped
	StatusError
)

// AgentState tracks the state of stopping a single agent.
type AgentState struct {
	AgentID       string
	ContainerID   string
	WorkspacePath string
	Steps         []StepInfo
	CurrentStep   StepType
	Done          bool
	Error         error
	Status        string // "stopped", "not_found", "failed"
}

// StepInfo describes a step for an agent.
type StepInfo struct {
	Name   string
	Status StepStatus
	Error  string
}

// Event types
type (
	// AgentStartMsg signals agent processing is starting.
	AgentStartMsg struct {
		AgentID string
	}

	// StepStartMsg signals a step is starting for an agent.
	StepStartMsg struct {
		AgentID string
		Step    StepType
	}

	// StepCompleteMsg signals a step completed.
	StepCompleteMsg struct {
		AgentID     string
		Step        StepType
		ContainerID string
		Workspace   string
	}

	// StepSkipMsg signals a step was skipped.
	StepSkipMsg struct {
		AgentID string
		Step    StepType
		Reason  string
	}

	// StepErrorMsg signals a step failed.
	StepErrorMsg struct {
		AgentID string
		Step    StepType
		Error   error
	}

	// AgentCompleteMsg signals agent stop is done.
	AgentCompleteMsg struct {
		AgentID string
		Status  string // "stopped", "not_found", "failed"
	}

	// AllCompleteMsg signals all agents are done.
	AllCompleteMsg struct{}

	// TickMsg triggers spinner updates.
	TickMsg time.Time
)

// Model is the stop TUI model.
type Model struct {
	th      *theme.Theme
	help    help.Model
	keys    keyMap
	spinner spinner.Model

	agents   []AgentState
	current  int
	done     bool
	anyError bool

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

// New creates a new stop TUI model.
func New(agentIDs []string) Model {
	th := theme.Default()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(th.Warning)

	agents := make([]AgentState, len(agentIDs))
	for i, id := range agentIDs {
		agents[i] = AgentState{
			AgentID: id,
			Steps: []StepInfo{
				{Name: "Finding container", Status: StatusPending},
				{Name: "Stopping container", Status: StatusPending},
				{Name: "Removing worktree", Status: StatusPending},
			},
		}
	}

	return Model{
		th:      th,
		help:    help.New(),
		keys:    newKeyMap(),
		spinner: s,
		agents:  agents,
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

	case AgentStartMsg:
		for i := range m.agents {
			if m.agents[i].AgentID == msg.AgentID {
				m.current = i
				break
			}
		}

	case StepStartMsg:
		m.updateAgentStep(msg.AgentID, msg.Step, StatusRunning, "")

	case StepCompleteMsg:
		m.updateAgentStep(msg.AgentID, msg.Step, StatusComplete, "")
		if msg.ContainerID != "" {
			m.setAgentContainerID(msg.AgentID, msg.ContainerID)
		}
		if msg.Workspace != "" {
			m.setAgentWorkspace(msg.AgentID, msg.Workspace)
		}

	case StepSkipMsg:
		m.updateAgentStep(msg.AgentID, msg.Step, StatusSkipped, msg.Reason)

	case StepErrorMsg:
		m.updateAgentStep(msg.AgentID, msg.Step, StatusError, msg.Error.Error())
		m.anyError = true

	case AgentCompleteMsg:
		for i := range m.agents {
			if m.agents[i].AgentID == msg.AgentID {
				m.agents[i].Done = true
				m.agents[i].Status = msg.Status
				// Mark remaining steps complete or skipped
				for j := range m.agents[i].Steps {
					if m.agents[i].Steps[j].Status == StatusPending {
						m.agents[i].Steps[j].Status = StatusComplete
					}
				}
				break
			}
		}

	case AllCompleteMsg:
		m.done = true

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

func (m *Model) updateAgentStep(agentID string, step StepType, status StepStatus, errMsg string) {
	for i := range m.agents {
		if m.agents[i].AgentID == agentID {
			if int(step) < len(m.agents[i].Steps) {
				m.agents[i].Steps[step].Status = status
				m.agents[i].Steps[step].Error = errMsg
				m.agents[i].CurrentStep = step
			}
			break
		}
	}
}

func (m *Model) setAgentContainerID(agentID, containerID string) {
	for i := range m.agents {
		if m.agents[i].AgentID == agentID {
			m.agents[i].ContainerID = containerID
			break
		}
	}
}

func (m *Model) setAgentWorkspace(agentID, workspace string) {
	for i := range m.agents {
		if m.agents[i].AgentID == agentID {
			m.agents[i].WorkspacePath = workspace
			break
		}
	}
}

// View renders the TUI.
func (m Model) View() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().Foreground(m.th.Warning).Bold(true)
	if len(m.agents) == 1 {
		b.WriteString(headerStyle.Render(fmt.Sprintf("Stopping agent '%s'", m.agents[0].AgentID)))
	} else {
		b.WriteString(headerStyle.Render(fmt.Sprintf("Stopping %d agents", len(m.agents))))
	}
	b.WriteString("\n\n")

	// Each agent
	for _, agent := range m.agents {
		b.WriteString(m.renderAgent(agent))
		b.WriteString("\n")
	}

	// Summary when done
	if m.done {
		b.WriteString("\n")
		b.WriteString(m.renderSummary())
	}

	// Help
	if !m.done {
		mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		b.WriteString(mutedStyle.Render("Press q to cancel"))
	}

	return b.String()
}

func (m Model) renderAgent(agent AgentState) string {
	var b strings.Builder

	// Agent header
	agentStyle := lipgloss.NewStyle().Foreground(m.th.Primary).Bold(true)
	b.WriteString(agentStyle.Render(fmt.Sprintf("Agent: %s", agent.AgentID)))

	if agent.Status == "not_found" {
		mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		b.WriteString(" " + mutedStyle.Render("(not found)"))
	} else if agent.ContainerID != "" {
		mutedStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		shortID := agent.ContainerID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		b.WriteString(" " + mutedStyle.Render(fmt.Sprintf("[%s]", shortID)))
	}
	b.WriteString("\n")

	// Steps
	for i, step := range agent.Steps {
		b.WriteString(m.renderStep(i, step))
		b.WriteString("\n")
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
		style = lipgloss.NewStyle().Foreground(m.th.Warning)
	case StatusComplete:
		icon = "✓"
		style = lipgloss.NewStyle().Foreground(m.th.Success)
	case StatusSkipped:
		icon = "⊘"
		style = lipgloss.NewStyle().Foreground(m.th.Muted)
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

func (m Model) renderSummary() string {
	var stopped, notFound, failed int
	for _, agent := range m.agents {
		switch agent.Status {
		case "stopped":
			stopped++
		case "not_found":
			notFound++
		case "failed":
			failed++
		}
	}

	var parts []string
	successStyle := lipgloss.NewStyle().Foreground(m.th.Success).Bold(true)
	warningStyle := lipgloss.NewStyle().Foreground(m.th.Warning)
	errorStyle := lipgloss.NewStyle().Foreground(m.th.Error).Bold(true)

	if stopped > 0 {
		parts = append(parts, successStyle.Render(fmt.Sprintf("✓ %d stopped", stopped)))
	}
	if notFound > 0 {
		parts = append(parts, warningStyle.Render(fmt.Sprintf("⊘ %d not found", notFound)))
	}
	if failed > 0 {
		parts = append(parts, errorStyle.Render(fmt.Sprintf("✗ %d failed", failed)))
	}

	return strings.Join(parts, "  ")
}

// SendAgentStart sends an agent start message.
func SendAgentStart(agentID string) tea.Cmd {
	return func() tea.Msg {
		return AgentStartMsg{AgentID: agentID}
	}
}

// SendStepStart sends a step start message.
func SendStepStart(agentID string, step StepType) tea.Cmd {
	return func() tea.Msg {
		return StepStartMsg{AgentID: agentID, Step: step}
	}
}

// SendStepComplete sends a step complete message.
func SendStepComplete(agentID string, step StepType, containerID, workspace string) tea.Cmd {
	return func() tea.Msg {
		return StepCompleteMsg{
			AgentID:     agentID,
			Step:        step,
			ContainerID: containerID,
			Workspace:   workspace,
		}
	}
}

// SendStepSkip sends a step skip message.
func SendStepSkip(agentID string, step StepType, reason string) tea.Cmd {
	return func() tea.Msg {
		return StepSkipMsg{AgentID: agentID, Step: step, Reason: reason}
	}
}

// SendStepError sends a step error message.
func SendStepError(agentID string, step StepType, err error) tea.Cmd {
	return func() tea.Msg {
		return StepErrorMsg{AgentID: agentID, Step: step, Error: err}
	}
}

// SendAgentComplete sends an agent complete message.
func SendAgentComplete(agentID, status string) tea.Cmd {
	return func() tea.Msg {
		return AgentCompleteMsg{AgentID: agentID, Status: status}
	}
}

// SendAllComplete signals all agents are done.
func SendAllComplete() tea.Cmd {
	return func() tea.Msg {
		return AllCompleteMsg{}
	}
}
