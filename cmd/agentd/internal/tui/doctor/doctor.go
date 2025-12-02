// Package doctor provides a Bubble Tea TUI for the doctor command.
// This is an EPHEMERAL TUI - it performs an action and auto-exits.
// Do not wait for user input after completion.
package doctor

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
	slCfg   steplist.Config

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
// If th is nil, the default theme is used.
func New(checkNames []string, th *theme.Theme) Model {
	th = theme.Ensure(th)

	s := spinner.New(th)
	slCfg := steplist.DefaultConfig(th, th.PrimaryText)

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
		slCfg:   slCfg,
		checks:  checks,
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
	case CheckStartMsg:
		return m.handleCheckStartMsg(msg)
	case CheckPassMsg:
		return m.handleCheckPassMsg(msg)
	case CheckSkipMsg:
		return m.handleCheckSkipMsg(msg)
	case CheckFailMsg:
		return m.handleCheckFailMsg(msg)
	case AllChecksCompleteMsg:
		return m.handleAllChecksCompleteMsg(msg)
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

func (m Model) handleCheckStartMsg(msg CheckStartMsg) (tea.Model, tea.Cmd) {
	if msg.Index < len(m.checks) {
		m.checks[msg.Index].Status = StatusRunning
		m.current = msg.Index
	}
	return m, nil
}

func (m Model) handleCheckPassMsg(msg CheckPassMsg) (tea.Model, tea.Cmd) {
	if msg.Index < len(m.checks) {
		m.checks[msg.Index].Status = StatusPassed
		m.checks[msg.Index].Message = msg.Message
	}
	return m, nil
}

func (m Model) handleCheckSkipMsg(msg CheckSkipMsg) (tea.Model, tea.Cmd) {
	if msg.Index < len(m.checks) {
		m.checks[msg.Index].Status = StatusSkipped
		m.checks[msg.Index].Message = msg.Message
	}
	return m, nil
}

func (m Model) handleCheckFailMsg(msg CheckFailMsg) (tea.Model, tea.Cmd) {
	if msg.Index < len(m.checks) {
		m.checks[msg.Index].Status = StatusFailed
		m.checks[msg.Index].Error = msg.Error
	}
	return m, nil
}

func (m Model) handleAllChecksCompleteMsg(msg AllChecksCompleteMsg) (tea.Model, tea.Cmd) {
	m.done = true
	m.allPassed = msg.AllPassed
	// Auto-exit immediately - ephemeral TUI completes its action
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
	b.WriteString(m.th.Title.Render("Validating environment"))
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
		b.WriteString(m.th.MutedText.Render("Press q to cancel"))
	}

	return b.String()
}

// toSteplistItem converts a CheckInfo to a steplist.Item.
func (c CheckInfo) toSteplistItem() steplist.Item {
	var status steplist.Status
	switch c.Status {
	case StatusPending:
		status = steplist.StatusPending
	case StatusRunning:
		status = steplist.StatusRunning
	case StatusPassed:
		status = steplist.StatusComplete
	case StatusSkipped:
		status = steplist.StatusSkipped
	case StatusFailed:
		status = steplist.StatusError
	}
	return steplist.Item{
		Name:    c.Name,
		Status:  status,
		Message: c.Message,
		Error:   c.Error,
	}
}

func (m Model) renderCheck(_ int, check CheckInfo) string {
	return steplist.RenderItem(check.toSteplistItem(), m.slCfg, m.spinner.View())
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
		return m.th.SuccessText.Bold(true).Render("✨ Environment ready!") + " " + m.th.MutedText.Render("All systems go for spawning agents.")
	}

	return m.th.ErrorText.Render(fmt.Sprintf("✗ Validation failed: %d check(s) failed", failed))
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
