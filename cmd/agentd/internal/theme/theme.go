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

// NewRetroTheme creates a new retro theme with the specified palette
func NewRetroTheme(palette PaletteName) *RetroTheme {
	theme := &RetroTheme{
		Palette: palette,
	}

	// Set colors based on palette
	switch palette {
	case PaletteCGA:
		theme.Primary = lipgloss.Color("#00FFFF")   // Cyan
		theme.Secondary = lipgloss.Color("#FF00FF") // Magenta
		theme.Accent = lipgloss.Color("#FFFF55")    // Yellow
		theme.Success = lipgloss.Color("#00FF00")   // Green
		theme.Warning = lipgloss.Color("#FFFF55")   // Yellow
		theme.Error = lipgloss.Color("#FF0000")     // Red
		theme.Muted = lipgloss.Color("#555555")     // Dark gray

	case PaletteAmber:
		amber := lipgloss.Color("#FFB000")
		theme.Primary = amber
		theme.Secondary = amber
		theme.Accent = lipgloss.Color("#FFCC00")
		theme.Success = amber
		theme.Warning = lipgloss.Color("#FF8800")
		theme.Error = lipgloss.Color("#FF4400")
		theme.Muted = lipgloss.Color("#664400")

	case PaletteGreen:
		green := lipgloss.Color("#00FF00")
		theme.Primary = green
		theme.Secondary = lipgloss.Color("#00DD00")
		theme.Accent = lipgloss.Color("#00FFAA")
		theme.Success = green
		theme.Warning = lipgloss.Color("#AAFF00")
		theme.Error = lipgloss.Color("#FF0000")
		theme.Muted = lipgloss.Color("#005500")

	case PaletteC64:
		theme.Primary = lipgloss.Color("#6C5EB5")   // C64 blue
		theme.Secondary = lipgloss.Color("#B66DFF") // C64 purple
		theme.Accent = lipgloss.Color("#70A4B2")    // C64 cyan
		theme.Success = lipgloss.Color("#588D43")   // C64 green
		theme.Warning = lipgloss.Color("#B6A470")   // C64 yellow
		theme.Error = lipgloss.Color("#B55E5E")     // C64 red
		theme.Muted = lipgloss.Color("#42348B")     // C64 dark blue
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
		Foreground(theme.Muted).
		Background(theme.Primary).
		Padding(0, 1)

	theme.Highlight = lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)

	return theme
}
