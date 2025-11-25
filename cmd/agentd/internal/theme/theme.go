package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PaletteName represents available color palettes
type PaletteName string

const (
	PaletteCGA   PaletteName = "cga"
	PaletteAmber PaletteName = "amber"
	PaletteGreen PaletteName = "green"
	PaletteC64   PaletteName = "c64"
)

// String returns the string representation of the palette name
func (p PaletteName) String() string {
	return string(p)
}

// ParsePaletteName parses a palette name string (case-insensitive).
// Returns the palette and true if valid, or PaletteCGA and false if invalid.
func ParsePaletteName(s string) (PaletteName, bool) {
	switch strings.ToLower(s) {
	case "cga":
		return PaletteCGA, true
	case "amber":
		return PaletteAmber, true
	case "green":
		return PaletteGreen, true
	case "c64":
		return PaletteC64, true
	default:
		return PaletteCGA, false
	}
}

// RetroTheme contains all colors and styles for the retro aesthetic
type RetroTheme struct {
	Palette PaletteName

	// Colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Muted     lipgloss.Color

	// Styles
	Logo      lipgloss.Style
	Header    lipgloss.Style
	BoxBorder lipgloss.Style
	StatusBar lipgloss.Style
	Highlight lipgloss.Style
}

// DefaultTheme returns the default CGA retro theme.
// Use this for CLI commands instead of NewRetroTheme(PaletteCGA) for consistency.
func DefaultTheme() *RetroTheme {
	return NewRetroTheme(PaletteCGA)
}

// NewRetroTheme creates a new retro theme with the specified palette
func NewRetroTheme(palette PaletteName) *RetroTheme {
	theme := &RetroTheme{
		Palette: palette,
	}

	// Set colors based on palette
	switch palette {
	case PaletteCGA:
		// Neon hues that stay legible on a black terminal background
		theme.Primary = lipgloss.Color("#00F6FF")   // Cyan
		theme.Secondary = lipgloss.Color("#FF63D8") // Magenta
		theme.Accent = lipgloss.Color("#FFEF5C")    // Soft yellow
		theme.Success = lipgloss.Color("#39FF14")   // Green
		theme.Warning = lipgloss.Color("#F8C537")   // Amber
		theme.Error = lipgloss.Color("#FF5F5F")     // Red
		theme.Muted = lipgloss.Color("#9CA3AF")     // Light gray for dark backgrounds

	case PaletteAmber:
		amber := lipgloss.Color("#FFB000")
		theme.Primary = amber
		theme.Secondary = lipgloss.Color("#FFC557")
		theme.Accent = lipgloss.Color("#FFE28A")
		theme.Success = lipgloss.Color("#FFD166")
		theme.Warning = lipgloss.Color("#FF9800")
		theme.Error = lipgloss.Color("#FF7043")
		theme.Muted = lipgloss.Color("#C8A25A")

	case PaletteGreen:
		green := lipgloss.Color("#00FF87")
		theme.Primary = green
		theme.Secondary = lipgloss.Color("#48F7C1")
		theme.Accent = lipgloss.Color("#7CFCFF")
		theme.Success = green
		theme.Warning = lipgloss.Color("#E7F562")
		theme.Error = lipgloss.Color("#FF6B6B")
		theme.Muted = lipgloss.Color("#7AC486")

	case PaletteC64:
		theme.Primary = lipgloss.Color("#7CB7FF")   // Brighter C64 blue
		theme.Secondary = lipgloss.Color("#C5A3FF") // Brighter C64 purple
		theme.Accent = lipgloss.Color("#6DE0FF")    // C64 cyan
		theme.Success = lipgloss.Color("#76D672")   // C64 green
		theme.Warning = lipgloss.Color("#FFD38A")   // C64 yellow
		theme.Error = lipgloss.Color("#FF8FA3")     // C64 red
		theme.Muted = lipgloss.Color("#8D9BD1")     // C64 dark blue softened
	}

	// Build styles
	theme.Logo = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	theme.Header = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Padding(0, 1)

	theme.BoxBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(0, 1)

	theme.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#0A0A0A")). // high contrast on bright background
		Background(theme.Primary).
		Padding(0, 1)

	theme.Highlight = lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)

	return theme
}
