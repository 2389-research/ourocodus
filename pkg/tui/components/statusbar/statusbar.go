// Package statusbar provides a Bubble Tea status bar component.
package statusbar

import (
	"fmt"
	"strings"

	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Item represents a single item in the status bar.
type Item struct {
	Label string
	Value string
	Icon  string // optional icon/emoji
}

// Model is the status bar model.
type Model struct {
	th     *theme.Theme
	items  []Item
	width  int
	height int
}

// New creates a new status bar model.
func New(th *theme.Theme) Model {
	return Model{
		th:     th,
		height: 1,
	}
}

// SetItems sets the status bar items.
func (m *Model) SetItems(items []Item) {
	m.items = items
}

// SetWidth sets the status bar width.
func (m *Model) SetWidth(w int) {
	m.width = w
}

// Height returns the status bar height.
func (m Model) Height() int {
	return m.height
}

// View renders the status bar.
func (m Model) View() string {
	if len(m.items) == 0 {
		return ""
	}

	labelStyle := lipgloss.NewStyle().
		Foreground(m.th.Muted)
	valueStyle := lipgloss.NewStyle().
		Foreground(m.th.Primary).
		Bold(true)
	iconStyle := lipgloss.NewStyle().
		Foreground(m.th.Accent)

	var parts []string
	for _, item := range m.items {
		var part string
		if item.Icon != "" {
			part = fmt.Sprintf("%s %s %s",
				iconStyle.Render(item.Icon),
				labelStyle.Render(item.Label+":"),
				valueStyle.Render(item.Value),
			)
		} else {
			part = fmt.Sprintf("%s %s",
				labelStyle.Render(item.Label+":"),
				valueStyle.Render(item.Value),
			)
		}
		parts = append(parts, part)
	}

	content := strings.Join(parts, "  │  ")

	barStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0A0A0A")).
		Background(m.th.Primary).
		Width(m.width).
		Padding(0, 1)

	return barStyle.Render(content)
}

// ViewInverted renders the status bar with inverted colors (for bottom placement).
func (m Model) ViewInverted() string {
	if len(m.items) == 0 {
		return ""
	}

	labelStyle := lipgloss.NewStyle().
		Foreground(m.th.Muted)
	valueStyle := lipgloss.NewStyle().
		Foreground(m.th.Primary).
		Bold(true)

	var parts []string
	for _, item := range m.items {
		var part string
		if item.Icon != "" {
			part = fmt.Sprintf("%s %s %s",
				item.Icon,
				labelStyle.Render(item.Label+":"),
				valueStyle.Render(item.Value),
			)
		} else {
			part = fmt.Sprintf("%s %s",
				labelStyle.Render(item.Label+":"),
				valueStyle.Render(item.Value),
			)
		}
		parts = append(parts, part)
	}

	content := "  " + strings.Join(parts, "  │  ")

	// Pad to full width
	if m.width > 0 && lipgloss.Width(content) < m.width {
		content += strings.Repeat(" ", m.width-lipgloss.Width(content))
	}

	return content
}
