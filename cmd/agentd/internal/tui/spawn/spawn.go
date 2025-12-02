// Package spawn provides a Bubble Tea TUI for the spawn command.
// This is an EPHEMERAL TUI - it performs an action and auto-exits.
// Do not wait for user input after completion.
package spawn

import (
	"fmt"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/tui/components/header"
	"github.com/2389-research/ourocodus/pkg/tui/components/progress"
	"github.com/2389-research/ourocodus/pkg/tui/components/spinner"
	"github.com/2389-research/ourocodus/pkg/tui/keys"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Step represents a spawn step (for control flow).
type Step int

const (
	StepCheckExisting Step = iota
	StepCreateLauncher
	StepBuildConfig
	StepSpawnAgent
	StepGenerateToken
	StepDone
)

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
	th       *theme.Theme
	help     help.Model
	keys     keyMap
	spinner  spinner.Model
	progress progress.Model

	agentID string
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
// If th is nil, the default theme is used.
func New(agentID string, th *theme.Theme) Model {
	th = theme.Ensure(th)

	s := spinner.New(th)
	p := progress.New(th)
	p.SetSteps([]string{
		"Checking for existing agent",
		"Creating launcher",
		"Building spawn config",
		"Spawning agent container",
		"Generating attach token",
	})

	return Model{
		th:       th,
		help:     help.New(),
		keys:     newKeyMap(),
		spinner:  s,
		progress: p,
		agentID:  agentID,
		current:  StepCheckExisting,
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick(),
		m.progress.Tick(),
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
	case StepStartMsg:
		return m.handleStepStartMsg(msg)
	case StepCompleteMsg:
		return m.handleStepCompleteMsg(msg)
	case StepErrorMsg:
		return m.handleStepErrorMsg(msg)
	case SpawnCompleteMsg:
		return m.handleSpawnCompleteMsg(msg)
	case TickMsg:
		return m.handleTickMsg()
	default:
		// Let spinner and progress handle their own tick messages
		var cmds []tea.Cmd
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		m.progress, cmd = m.progress.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
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

func (m Model) handleStepStartMsg(msg StepStartMsg) (tea.Model, tea.Cmd) {
	m.current = msg.Step
	m.progress.StartStep(int(msg.Step))
	return m, nil
}

func (m Model) handleStepCompleteMsg(_ StepCompleteMsg) (tea.Model, tea.Cmd) {
	m.progress.CompleteStep()
	return m, nil
}

func (m Model) handleStepErrorMsg(msg StepErrorMsg) (tea.Model, tea.Cmd) {
	m.progress.FailStep(msg.Error.Error())
	m.err = msg.Error
	m.done = true
	// Auto-exit after brief delay to show error
	return m, tea.Sequence(tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg { return nil }), tea.Quit)
}

func (m Model) handleSpawnCompleteMsg(msg SpawnCompleteMsg) (tea.Model, tea.Cmd) {
	m.handle = msg.Handle
	m.attachToken = msg.AttachToken
	m.tokenError = msg.TokenError
	m.done = true
	// Complete the current step if it's not already done
	m.progress.CompleteStep()
	// Auto-exit - the final view will be rendered before quit
	return m, tea.Quit
}

func (m Model) handleTickMsg() (tea.Model, tea.Cmd) {
	if !m.done {
		return m, tickCmd()
	}
	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	var b strings.Builder

	// Header with logo
	b.WriteString(header.Render(m.th))
	b.WriteString("\n\n")

	// Command title
	b.WriteString(m.th.Title.Render(fmt.Sprintf("Spawning agent '%s'", m.agentID)))
	b.WriteString("\n\n")

	// Steps using progress component
	b.WriteString(m.progress.View())
	b.WriteString("\n")

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
		b.WriteString(m.th.MutedText.Render("Press q to cancel"))
	}

	return b.String()
}

func (m Model) renderError() string {
	return m.th.ErrorText.Render(fmt.Sprintf("✗ Spawn failed: %v", m.err))
}

func (m Model) renderSuccess() string {
	var b strings.Builder

	if m.handle == nil {
		return m.th.SuccessText.Render("✓ Agent spawned successfully")
	}

	// Worktree
	b.WriteString("🌳 ")
	b.WriteString(m.th.LabelText.Render("Worktree: "))
	b.WriteString(m.th.ValueText.Render(m.handle.WorkspacePath()) + " ")
	b.WriteString(m.th.MutedText.Render(fmt.Sprintf("(branch: %s)", m.handle.BranchName())))
	b.WriteString("\n")

	// Container
	b.WriteString("📦 ")
	b.WriteString(m.th.LabelText.Render("Container: "))
	b.WriteString(m.th.ValueText.Render(m.handle.ContainerID()) + " ")
	b.WriteString(m.th.SuccessText.Render("(running)"))
	b.WriteString("\n")

	// Credentials
	b.WriteString("🔑 ")
	b.WriteString(m.th.LabelText.Render("Credentials: "))
	credPath := m.handle.CredentialsPath()
	if credPath != "" {
		b.WriteString(m.th.ValueText.Render("mounted at /root/.creds "))
		b.WriteString(m.th.MutedText.Render("(read-only)"))
	} else {
		b.WriteString(m.th.MutedText.Render("(none)"))
	}
	b.WriteString("\n")

	// Attach token
	if m.attachToken != "" {
		b.WriteString("\n")
		b.WriteString("🔐 ")
		b.WriteString(m.th.LabelText.Render("Attach Token:"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("   %s\n", m.th.ValueText.Render(m.attachToken)))
		b.WriteString("\n")
		b.WriteString(m.th.MutedText.Render("   Use this token when attaching from PWA or relay:"))
		b.WriteString("\n")
		b.WriteString(m.th.MutedText.Render(fmt.Sprintf("   → agent:attach {\"agentId\": \"%s\", \"token\": \"<token>\"}", m.handle.AgentID())))
		b.WriteString("\n")
	} else if m.tokenError != nil {
		b.WriteString("\n")
		b.WriteString(m.th.WarningText.Render(fmt.Sprintf("⚠️  Warning: Failed to generate attach token: %v", m.tokenError)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.th.SuccessText.Bold(true).Render(fmt.Sprintf("✓ Agent %s ready", m.handle.AgentID())))

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
