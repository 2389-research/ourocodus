// Package steplist provides a reusable step/check list renderer for TUIs.
// Used for ephemeral TUIs that show progress through a series of steps.
package steplist

import (
	"fmt"

	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Status represents the completion status of a step.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusComplete
	StatusSkipped
	StatusError
)

// Item represents a single step or check in the list.
type Item struct {
	Name    string
	Status  Status
	Message string // Optional success/info message (shown in parentheses)
	Error   string // Optional error message (shown after dash or colon)
}

// Icons defines the icons used for each status.
type Icons struct {
	Pending  string
	Running  string // Usually a spinner view
	Complete string
	Skipped  string
	Error    string
}

// DefaultIcons returns the standard icon set.
func DefaultIcons() Icons {
	return Icons{
		Pending:  "○",
		Running:  "◐", // Fallback if no spinner provided
		Complete: "✓",
		Skipped:  "⊘",
		Error:    "✗",
	}
}

// StyleSet defines which theme styles to use for each status.
type StyleSet struct {
	Pending  lipgloss.Style
	Running  lipgloss.Style
	Complete lipgloss.Style
	Skipped  lipgloss.Style
	Error    lipgloss.Style
	Message  lipgloss.Style // For optional message text
}

// DefaultStyles returns styles using the theme's semantic colors.
// runningStyle allows customization (e.g., PrimaryText vs WarningText).
func DefaultStyles(th *theme.Theme, runningStyle lipgloss.Style) StyleSet {
	return StyleSet{
		Pending:  th.MutedText,
		Running:  runningStyle,
		Complete: th.SuccessText,
		Skipped:  th.MutedText,
		Error:    th.ErrorText,
		Message:  th.MutedText,
	}
}

// Config holds rendering configuration.
type Config struct {
	Icons      Icons
	Styles     StyleSet
	Indent     string // Prefix for each line (e.g., "  " for indentation)
	ErrorSep   string // Separator before error text (e.g., " - " or ": ")
	ShowErrors bool   // Whether to show error messages inline
}

// DefaultConfig returns a standard configuration.
func DefaultConfig(th *theme.Theme, runningStyle lipgloss.Style) Config {
	return Config{
		Icons:      DefaultIcons(),
		Styles:     DefaultStyles(th, runningStyle),
		Indent:     "",
		ErrorSep:   ": ",
		ShowErrors: true,
	}
}

// RenderItem renders a single step/check item.
func RenderItem(item Item, cfg Config, spinnerView string) string {
	var icon string
	var style lipgloss.Style

	switch item.Status {
	case StatusPending:
		icon = cfg.Icons.Pending
		style = cfg.Styles.Pending
	case StatusRunning:
		if spinnerView != "" {
			icon = spinnerView
		} else {
			icon = cfg.Icons.Running
		}
		style = cfg.Styles.Running
	case StatusComplete:
		icon = cfg.Icons.Complete
		style = cfg.Styles.Complete
	case StatusSkipped:
		icon = cfg.Icons.Skipped
		style = cfg.Styles.Skipped
	case StatusError:
		icon = cfg.Icons.Error
		style = cfg.Styles.Error
	}

	line := fmt.Sprintf("%s%s %s", cfg.Indent, icon, item.Name)

	// Add optional message in parentheses
	if item.Message != "" {
		line += " " + cfg.Styles.Message.Render(fmt.Sprintf("(%s)", item.Message))
	}

	// Add error text
	if cfg.ShowErrors && item.Error != "" {
		line += cfg.ErrorSep + item.Error
	}

	return style.Render(line)
}

// RenderList renders a list of items, one per line.
func RenderList(items []Item, cfg Config, spinnerView string) string {
	var result string
	for i, item := range items {
		if i > 0 {
			result += "\n"
		}
		result += RenderItem(item, cfg, spinnerView)
	}
	return result
}
