// Package styles provides shared lipgloss styles for TUI components.
package styles

import (
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Styles contains pre-built styles derived from a theme.
type Styles struct {
	th *theme.Theme

	// Text styles
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Accent    lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
	Error     lipgloss.Style
	Muted     lipgloss.Style

	// Component styles
	Label     lipgloss.Style
	Value     lipgloss.Style
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	HelpKey   lipgloss.Style
	HelpValue lipgloss.Style

	// Box styles
	Box       lipgloss.Style
	SuccessBox lipgloss.Style
	ErrorBox   lipgloss.Style
	WarningBox lipgloss.Style
}

// New creates a new Styles instance from a theme.
func New(th *theme.Theme) *Styles {
	s := &Styles{th: th}

	// Text styles
	s.Primary = lipgloss.NewStyle().Foreground(th.Primary)
	s.Secondary = lipgloss.NewStyle().Foreground(th.Secondary)
	s.Accent = lipgloss.NewStyle().Foreground(th.Accent)
	s.Success = lipgloss.NewStyle().Foreground(th.Success)
	s.Warning = lipgloss.NewStyle().Foreground(th.Warning)
	s.Error = lipgloss.NewStyle().Foreground(th.Error)
	s.Muted = lipgloss.NewStyle().Foreground(th.Muted)

	// Component styles
	s.Label = lipgloss.NewStyle().Foreground(th.Muted)
	s.Value = lipgloss.NewStyle().Foreground(th.Primary).Bold(true)
	s.Title = lipgloss.NewStyle().Foreground(th.Primary).Bold(true)
	s.Subtitle = lipgloss.NewStyle().Foreground(th.Secondary)
	s.HelpKey = lipgloss.NewStyle().Foreground(th.Accent)
	s.HelpValue = lipgloss.NewStyle().Foreground(th.Muted)

	// Box styles
	s.Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Primary).
		Padding(0, 1)

	s.SuccessBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Success).
		Padding(0, 1)

	s.ErrorBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Error).
		Padding(0, 1)

	s.WarningBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Warning).
		Padding(0, 1)

	return s
}

// Default returns styles using the default theme.
func Default() *Styles {
	return New(theme.Default())
}

// Theme returns the underlying theme.
func (s *Styles) Theme() *theme.Theme {
	return s.th
}
