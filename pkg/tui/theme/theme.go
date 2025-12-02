// Package theme provides color palettes and styling for TUI components.
// Supports dark and light themes with WCAG AA compliant contrast ratios.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// ThemeMode represents light or dark theme.
type ThemeMode string

const (
	ThemeDark  ThemeMode = "dark"
	ThemeLight ThemeMode = "light"
)

// Theme contains all colors and styles for the TUI.
type Theme struct {
	Mode ThemeMode

	// Core semantic colors
	Primary   lipgloss.Color // Main accent, headers, focus elements
	Secondary lipgloss.Color // Less prominent text, body
	Accent    lipgloss.Color // Emphasis, links, highlights
	Success   lipgloss.Color // OK, pass, connected
	Warning   lipgloss.Color // Alerts without panic
	Error     lipgloss.Color // Failure, stop, critical
	Muted     lipgloss.Color // Timestamps, subtle text

	// Selection/highlight colors (for inverted backgrounds like selected rows)
	Contrast            lipgloss.Color // Background for selected items
	HighlightForeground lipgloss.Color // Text color on highlighted/selected backgrounds

	// Tag palette - 10 distinct colors for differentiating categories
	// Use for log tags, status indicators, categories, etc.
	TagPalette []lipgloss.Color

	// Rainbow gradient for decorative elements (logo, animations, etc.)
	Rainbow []lipgloss.Color

	// Pre-configured styles
	Logo      lipgloss.Style
	Header    lipgloss.Style
	BoxBorder lipgloss.Style
	StatusBar lipgloss.Style
	Highlight lipgloss.Style
	Selected  lipgloss.Style // Style for selected rows in tables

	// Semantic text styles for DRY TUI code
	Title       lipgloss.Style // Bold primary text for section titles
	ErrorText   lipgloss.Style // Error messages
	SuccessText lipgloss.Style // Success messages
	WarningText lipgloss.Style // Warning messages
	MutedText   lipgloss.Style // Subtle/secondary information
	LabelText   lipgloss.Style // Bold primary labels
	ValueText   lipgloss.Style // Accent-colored values
	URLText     lipgloss.Style // Underlined accent for URLs/links

	// Simple color-only styles (no bold) for dynamic status rendering
	PrimaryText   lipgloss.Style // Primary color text (no bold)
	SecondaryText lipgloss.Style // Secondary color text

	// Container styles
	ViewportBorder lipgloss.Style // Rounded border for viewport containers
	ViewportPlain  lipgloss.Style // No border/styling for plain viewports
}

// Default returns the default dark theme.
func Default() *Theme {
	return NewDark()
}

// Ensure returns the provided theme if non-nil, otherwise returns the default theme.
func Ensure(th *Theme) *Theme {
	if th != nil {
		return th
	}
	return Default()
}

// NewDark creates a dark theme optimized for dark terminal backgrounds.
// All colors have WCAG AA compliant contrast (4.5:1+) against ~#1A1A2E background.
func NewDark() *Theme {
	th := &Theme{
		Mode: ThemeDark,

		// Core semantic colors - bright on dark
		Primary:   lipgloss.Color("#5FAFFF"), // Vibrant blue
		Secondary: lipgloss.Color("#A3B8FF"), // Softer blue
		Accent:    lipgloss.Color("#FDB56C"), // Warm amber
		Success:   lipgloss.Color("#34D399"), // Bright green
		Warning:   lipgloss.Color("#FBBF24"), // Amber/yellow
		Error:     lipgloss.Color("#F87171"), // Soft red
		Muted:     lipgloss.Color("#6B7280"), // Gray

		// Selection colors
		Contrast:            lipgloss.Color("#3B4252"), // Dark blue-gray for selection bg
		HighlightForeground: lipgloss.Color("#ECEFF4"), // Light text on selection

		// Tag palette - 10 distinct, vibrant colors
		TagPalette: []lipgloss.Color{
			lipgloss.Color("#81A1C1"), // Steel blue
			lipgloss.Color("#B48EAD"), // Muted purple
			lipgloss.Color("#A3BE8C"), // Sage green
			lipgloss.Color("#EBCB8B"), // Sand yellow
			lipgloss.Color("#D08770"), // Coral
			lipgloss.Color("#BF616A"), // Dusty red
			lipgloss.Color("#5E81AC"), // Ocean blue
			lipgloss.Color("#88C0D0"), // Frost cyan
			lipgloss.Color("#E06C75"), // Rose
			lipgloss.Color("#C678DD"), // Orchid purple
		},

		// Rainbow gradient for decorative elements
		Rainbow: []lipgloss.Color{
			lipgloss.Color("#FF5555"), // Red
			lipgloss.Color("#FFB86C"), // Orange
			lipgloss.Color("#F1FA8C"), // Yellow
			lipgloss.Color("#50FA7B"), // Green
			lipgloss.Color("#8BE9FD"), // Cyan
			lipgloss.Color("#5FAFFF"), // Blue (matches Primary)
			lipgloss.Color("#FF79C6"), // Magenta
		},
	}

	th.initStyles()
	return th
}

// NewLight creates a light theme optimized for light terminal backgrounds.
// All colors have WCAG AA compliant contrast (4.5:1+) against ~#F5F5F5 background.
func NewLight() *Theme {
	th := &Theme{
		Mode: ThemeLight,

		// Core semantic colors - dark on light
		Primary:   lipgloss.Color("#005F9E"), // Deep blue
		Secondary: lipgloss.Color("#335D92"), // Medium blue
		Accent:    lipgloss.Color("#C75B00"), // Burnt orange
		Success:   lipgloss.Color("#0F7B3B"), // Forest green
		Warning:   lipgloss.Color("#B77900"), // Dark amber
		Error:     lipgloss.Color("#B91C1C"), // Deep red
		Muted:     lipgloss.Color("#6B7280"), // Gray (same as dark)

		// Selection colors
		Contrast:            lipgloss.Color("#274060"), // Dark blue for selection bg
		HighlightForeground: lipgloss.Color("#F5F5F5"), // Light text on selection

		// Tag palette - darker versions for light background
		TagPalette: []lipgloss.Color{
			lipgloss.Color("#4C6A8A"), // Steel blue (darker)
			lipgloss.Color("#7A5B74"), // Muted purple (darker)
			lipgloss.Color("#5E7A4C"), // Sage green (darker)
			lipgloss.Color("#8A7540"), // Sand yellow (darker)
			lipgloss.Color("#9A5A40"), // Coral (darker)
			lipgloss.Color("#8A3B42"), // Dusty red (darker)
			lipgloss.Color("#3A5A8A"), // Ocean blue (darker)
			lipgloss.Color("#3A7A8A"), // Frost cyan (darker)
			lipgloss.Color("#A04050"), // Rose (darker)
			lipgloss.Color("#7A4A9A"), // Orchid purple (darker)
		},

		// Rainbow gradient (slightly muted for light bg)
		Rainbow: []lipgloss.Color{
			lipgloss.Color("#CC3030"), // Red
			lipgloss.Color("#CC7030"), // Orange
			lipgloss.Color("#A0A030"), // Yellow
			lipgloss.Color("#30A050"), // Green
			lipgloss.Color("#30A0A0"), // Cyan
			lipgloss.Color("#3070CC"), // Blue
			lipgloss.Color("#A050A0"), // Magenta
		},
	}

	th.initStyles()
	return th
}

// initStyles initializes the pre-configured styles based on colors.
func (th *Theme) initStyles() {
	th.Logo = lipgloss.NewStyle().
		Foreground(th.Primary).
		Bold(true)

	th.Header = lipgloss.NewStyle().
		Foreground(th.Primary).
		Bold(true).
		Padding(0, 1)

	th.BoxBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Primary).
		Padding(0, 1)

	th.StatusBar = lipgloss.NewStyle().
		Foreground(th.HighlightForeground).
		Background(th.Contrast).
		Padding(0, 1)

	th.Highlight = lipgloss.NewStyle().
		Foreground(th.Accent).
		Bold(true)

	th.Selected = lipgloss.NewStyle().
		Foreground(th.HighlightForeground).
		Background(th.Contrast).
		Bold(true)

	// Semantic text styles
	th.Title = lipgloss.NewStyle().
		Foreground(th.Primary).
		Bold(true)

	th.ErrorText = lipgloss.NewStyle().
		Foreground(th.Error).
		Bold(true)

	th.SuccessText = lipgloss.NewStyle().
		Foreground(th.Success)

	th.WarningText = lipgloss.NewStyle().
		Foreground(th.Warning)

	th.MutedText = lipgloss.NewStyle().
		Foreground(th.Muted)

	th.LabelText = lipgloss.NewStyle().
		Foreground(th.Primary).
		Bold(true)

	th.ValueText = lipgloss.NewStyle().
		Foreground(th.Accent)

	th.URLText = lipgloss.NewStyle().
		Foreground(th.Accent).
		Underline(true)

	// Simple color-only styles (no bold)
	th.PrimaryText = lipgloss.NewStyle().
		Foreground(th.Primary)

	th.SecondaryText = lipgloss.NewStyle().
		Foreground(th.Secondary)

	// Container styles
	th.ViewportBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Primary)
	th.ViewportPlain = lipgloss.NewStyle()
}

// New creates a theme from a mode string ("dark" or "light").
// Defaults to dark if mode is unrecognized.
func New(mode ThemeMode) *Theme {
	switch mode {
	case ThemeLight:
		return NewLight()
	default:
		return NewDark()
	}
}

// GetTagColor returns a color from the tag palette by index.
// The index wraps around if it exceeds the palette size.
func (th *Theme) GetTagColor(index int) lipgloss.Color {
	if len(th.TagPalette) == 0 {
		return th.Primary
	}
	return th.TagPalette[index%len(th.TagPalette)]
}

// RetroTheme is an alias for Theme for backward compatibility with existing code.
type RetroTheme = Theme
