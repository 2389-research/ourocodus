// Package progress provides a multi-step progress component.
package progress

import (
	"fmt"
	"strings"

	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StepStatus represents the status of a step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepComplete
	StepError
)

// Step represents a single step in the progress.
type Step struct {
	Name   string
	Status StepStatus
	Error  string // error message if status is StepError
}

// Model is the progress model.
type Model struct {
	th       *theme.Theme
	steps    []Step
	current  int
	spinner  spinner.Model
	progress progress.Model
	width    int
	showBar  bool
}

// New creates a new progress model.
func New(th *theme.Theme) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(th.Primary)

	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	return Model{
		th:       th,
		spinner:  s,
		progress: p,
		current:  -1,
	}
}

// SetSteps sets the steps to track.
func (m *Model) SetSteps(names []string) {
	m.steps = make([]Step, len(names))
	for i, name := range names {
		m.steps[i] = Step{Name: name, Status: StepPending}
	}
	m.current = -1
}

// SetWidth sets the component width.
func (m *Model) SetWidth(w int) {
	m.width = w
	m.progress.Width = w - 4
}

// ShowProgressBar enables or disables the progress bar.
func (m *Model) ShowProgressBar(show bool) {
	m.showBar = show
}

// StartStep starts a step by index.
func (m *Model) StartStep(index int) {
	if index >= 0 && index < len(m.steps) {
		m.current = index
		m.steps[index].Status = StepRunning
	}
}

// CompleteStep completes the current step.
func (m *Model) CompleteStep() {
	if m.current >= 0 && m.current < len(m.steps) {
		m.steps[m.current].Status = StepComplete
	}
}

// FailStep marks the current step as failed.
func (m *Model) FailStep(err string) {
	if m.current >= 0 && m.current < len(m.steps) {
		m.steps[m.current].Status = StepError
		m.steps[m.current].Error = err
	}
}

// NextStep moves to and starts the next step.
func (m *Model) NextStep() bool {
	if m.current >= 0 && m.current < len(m.steps) {
		m.steps[m.current].Status = StepComplete
	}
	m.current++
	if m.current < len(m.steps) {
		m.steps[m.current].Status = StepRunning
		return true
	}
	return false
}

// CurrentStep returns the current step index.
func (m Model) CurrentStep() int {
	return m.current
}

// IsComplete returns true if all steps are complete.
func (m Model) IsComplete() bool {
	for _, step := range m.steps {
		if step.Status != StepComplete {
			return false
		}
	}
	return len(m.steps) > 0
}

// HasError returns true if any step has an error.
func (m Model) HasError() bool {
	for _, step := range m.steps {
		if step.Status == StepError {
			return true
		}
	}
	return false
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the progress component.
func (m Model) View() string {
	if len(m.steps) == 0 {
		return ""
	}

	var b strings.Builder

	pendingStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
	runningStyle := lipgloss.NewStyle().Foreground(m.th.Primary)
	completeStyle := lipgloss.NewStyle().Foreground(m.th.Success)
	errorStyle := lipgloss.NewStyle().Foreground(m.th.Error)

	for i, step := range m.steps {
		var icon string
		var style lipgloss.Style

		switch step.Status {
		case StepPending:
			icon = "○"
			style = pendingStyle
		case StepRunning:
			icon = m.spinner.View()
			style = runningStyle
		case StepComplete:
			icon = "✓"
			style = completeStyle
		case StepError:
			icon = "✗"
			style = errorStyle
		}

		line := fmt.Sprintf("%s %s", icon, style.Render(step.Name))
		b.WriteString(line)

		if step.Status == StepError && step.Error != "" {
			b.WriteString("\n")
			errLine := errorStyle.Render("  └─ " + step.Error)
			b.WriteString(errLine)
		}

		if i < len(m.steps)-1 {
			b.WriteString("\n")
		}
	}

	// Add progress bar if enabled
	if m.showBar && len(m.steps) > 0 {
		completed := 0
		for _, step := range m.steps {
			if step.Status == StepComplete {
				completed++
			}
		}
		percent := float64(completed) / float64(len(m.steps))

		b.WriteString("\n\n")
		b.WriteString(m.progress.ViewAs(percent))
	}

	return b.String()
}

// Tick returns the spinner tick command.
func (m Model) Tick() tea.Cmd {
	return m.spinner.Tick
}
