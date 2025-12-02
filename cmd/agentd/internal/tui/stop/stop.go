// Package stop provides a Bubble Tea TUI for the stop command.
// This is an EPHEMERAL TUI - it performs an action and auto-exits.
// Do not wait for user input after completion.
package stop

import (
	"fmt"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/tui/components/header"
	"github.com/2389-research/ourocodus/pkg/tui/components/spinner"
	"github.com/2389-research/ourocodus/pkg/tui/components/steplist"
	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
	slCfg   steplist.Config

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
// If th is nil, the default theme is used.
func New(agentIDs []string, th *theme.Theme) Model {
	th = theme.Ensure(th)

	s := spinner.New(th)

	// Use WarningText for running steps, indent steps, use " - " separator
	slCfg := steplist.DefaultConfig(th, th.WarningText)
	slCfg.Indent = "  "
	slCfg.ErrorSep = " - "

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
		slCfg:   slCfg,
		agents:  agents,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick(),
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowSizeMsg(msg)
	case AgentStartMsg:
		return m.handleAgentStartMsg(msg)
	case StepStartMsg:
		return m.handleStepStartMsg(msg)
	case StepCompleteMsg:
		return m.handleStepCompleteMsg(msg)
	case StepSkipMsg:
		return m.handleStepSkipMsg(msg)
	case StepErrorMsg:
		return m.handleStepErrorMsg(msg)
	case AgentCompleteMsg:
		return m.handleAgentCompleteMsg(msg)
	case AllCompleteMsg:
		return m.handleAllCompleteMsg()
	case TickMsg:
		return m.handleTickMsg()
	default:
		// Let spinner handle its own tick messages
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	return m, nil
}

func (m Model) handleAgentStartMsg(msg AgentStartMsg) (tea.Model, tea.Cmd) {
	for i := range m.agents {
		if m.agents[i].AgentID == msg.AgentID {
			m.current = i
			break
		}
	}
	return m, nil
}

func (m Model) handleStepStartMsg(msg StepStartMsg) (tea.Model, tea.Cmd) {
	m.updateAgentStep(msg.AgentID, msg.Step, StatusRunning, "")
	return m, nil
}

func (m Model) handleStepCompleteMsg(msg StepCompleteMsg) (tea.Model, tea.Cmd) {
	m.updateAgentStep(msg.AgentID, msg.Step, StatusComplete, "")
	if msg.ContainerID != "" {
		m.setAgentContainerID(msg.AgentID, msg.ContainerID)
	}
	if msg.Workspace != "" {
		m.setAgentWorkspace(msg.AgentID, msg.Workspace)
	}
	return m, nil
}

func (m Model) handleStepSkipMsg(msg StepSkipMsg) (tea.Model, tea.Cmd) {
	m.updateAgentStep(msg.AgentID, msg.Step, StatusSkipped, msg.Reason)
	return m, nil
}

func (m Model) handleStepErrorMsg(msg StepErrorMsg) (tea.Model, tea.Cmd) {
	m.updateAgentStep(msg.AgentID, msg.Step, StatusError, msg.Error.Error())
	m.anyError = true
	return m, nil
}

func (m Model) handleAgentCompleteMsg(msg AgentCompleteMsg) (tea.Model, tea.Cmd) {
	m.markAgentComplete(msg.AgentID, msg.Status)
	return m, nil
}

func (m *Model) markAgentComplete(agentID, status string) {
	for i := range m.agents {
		if m.agents[i].AgentID == agentID {
			m.agents[i].Done = true
			m.agents[i].Status = status
			// Mark remaining steps complete or skipped
			for j := range m.agents[i].Steps {
				if m.agents[i].Steps[j].Status == StatusPending {
					m.agents[i].Steps[j].Status = StatusComplete
				}
			}
			break
		}
	}
}

func (m Model) handleAllCompleteMsg() (tea.Model, tea.Cmd) {
	m.done = true
	// Auto-exit immediately - ephemeral TUI completes its action
	return m, tea.Quit
}

func (m Model) handleTickMsg() (tea.Model, tea.Cmd) {
	if !m.done {
		return m, tickCmd()
	}
	return m, nil
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

	// Header with logo
	b.WriteString(header.Render(m.th))
	b.WriteString("\n\n")

	// Command title
	if len(m.agents) == 1 {
		b.WriteString(m.th.WarningText.Bold(true).Render(fmt.Sprintf("Stopping agent '%s'", m.agents[0].AgentID)))
	} else {
		b.WriteString(m.th.WarningText.Bold(true).Render(fmt.Sprintf("Stopping %d agents", len(m.agents))))
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
		b.WriteString(m.th.MutedText.Render("Press q to cancel"))
	}

	return b.String()
}

func (m Model) renderAgent(agent AgentState) string {
	var b strings.Builder

	// Agent header
	b.WriteString(m.th.Title.Render(fmt.Sprintf("Agent: %s", agent.AgentID)))

	if agent.Status == "not_found" {
		b.WriteString(" " + m.th.MutedText.Render("(not found)"))
	} else if agent.ContainerID != "" {
		shortID := agent.ContainerID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		b.WriteString(" " + m.th.MutedText.Render(fmt.Sprintf("[%s]", shortID)))
	}
	b.WriteString("\n")

	// Steps
	for i, step := range agent.Steps {
		b.WriteString(m.renderStep(i, step))
		b.WriteString("\n")
	}

	return b.String()
}

// toSteplistItem converts a StepInfo to a steplist.Item.
func (s StepInfo) toSteplistItem() steplist.Item {
	var status steplist.Status
	switch s.Status {
	case StatusPending:
		status = steplist.StatusPending
	case StatusRunning:
		status = steplist.StatusRunning
	case StatusComplete:
		status = steplist.StatusComplete
	case StatusSkipped:
		status = steplist.StatusSkipped
	case StatusError:
		status = steplist.StatusError
	}
	return steplist.Item{
		Name:   s.Name,
		Status: status,
		Error:  s.Error,
	}
}

func (m Model) renderStep(_ int, step StepInfo) string {
	return steplist.RenderItem(step.toSteplistItem(), m.slCfg, m.spinner.View())
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

	if stopped > 0 {
		parts = append(parts, m.th.SuccessText.Bold(true).Render(fmt.Sprintf("✓ %d stopped", stopped)))
	}
	if notFound > 0 {
		parts = append(parts, m.th.WarningText.Render(fmt.Sprintf("⊘ %d not found", notFound)))
	}
	if failed > 0 {
		parts = append(parts, m.th.ErrorText.Render(fmt.Sprintf("✗ %d failed", failed)))
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
