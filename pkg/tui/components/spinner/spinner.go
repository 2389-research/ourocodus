// Package spinner provides a Bubble Tea spinner component with consistent styling.
package spinner

import (
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Style represents different spinner styles.
type Style int

const (
	StyleDots Style = iota
	StyleLine
	StyleMiniDot
	StyleJump
	StylePulse
	StylePoints
	StyleGlobe
	StyleMoon
	StyleMonkey
)

// Model is the spinner model.
type Model struct {
	spinner spinner.Model
	th      *theme.Theme
	message string
	style   Style
}

// New creates a new spinner with the default style.
func New(th *theme.Theme) Model {
	return NewWithStyle(th, StyleDots)
}

// NewWithStyle creates a new spinner with a specific style.
func NewWithStyle(th *theme.Theme, style Style) Model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(th.Primary)

	switch style {
	case StyleDots:
		s.Spinner = spinner.Dot
	case StyleLine:
		s.Spinner = spinner.Line
	case StyleMiniDot:
		s.Spinner = spinner.MiniDot
	case StyleJump:
		s.Spinner = spinner.Jump
	case StylePulse:
		s.Spinner = spinner.Pulse
	case StylePoints:
		s.Spinner = spinner.Points
	case StyleGlobe:
		s.Spinner = spinner.Globe
	case StyleMoon:
		s.Spinner = spinner.Moon
	case StyleMonkey:
		s.Spinner = spinner.Monkey
	default:
		s.Spinner = spinner.Dot
	}

	return Model{
		spinner: s,
		th:      th,
		style:   style,
	}
}

// SetMessage sets the message displayed next to the spinner.
func (m *Model) SetMessage(msg string) {
	m.message = msg
}

// Init initializes the spinner.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update updates the spinner.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the spinner.
func (m Model) View() string {
	if m.message != "" {
		msgStyle := lipgloss.NewStyle().Foreground(m.th.Muted)
		return m.spinner.View() + " " + msgStyle.Render(m.message)
	}
	return m.spinner.View()
}

// Tick returns the tick command for the spinner.
func (m Model) Tick() tea.Cmd {
	return m.spinner.Tick
}
