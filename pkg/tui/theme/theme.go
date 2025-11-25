// Package theme provides color palettes and styling for TUI components.
package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PaletteName represents available color palettes.
type PaletteName string

const (
	PaletteCGA   PaletteName = "cga"
	PaletteAmber PaletteName = "amber"
	PaletteGreen PaletteName = "green"
	PaletteC64   PaletteName = "c64"
)

// String returns the string representation of the palette name.
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

// Theme contains all colors and styles for the TUI.
type Theme struct {
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

// Default returns the default CGA theme.
func Default() *Theme {
	return New(PaletteCGA)
}

// RetroTheme is an alias for Theme for backward compatibility.
type RetroTheme = Theme

// NewRetroTheme is an alias for New for backward compatibility.
func NewRetroTheme(palette PaletteName) *Theme {
	return New(palette)
}

// DefaultTheme is an alias for Default for backward compatibility.
func DefaultTheme() *Theme {
	return Default()
}

// New creates a new theme with the specified palette.
func New(palette PaletteName) *Theme {
	th := &Theme{
		Palette: palette,
	}

	switch palette {
	case PaletteCGA:
		th.Primary = lipgloss.Color("#00F6FF")   // Cyan
		th.Secondary = lipgloss.Color("#FF63D8") // Magenta
		th.Accent = lipgloss.Color("#FFEF5C")    // Soft yellow
		th.Success = lipgloss.Color("#39FF14")   // Green
		th.Warning = lipgloss.Color("#F8C537")   // Amber
		th.Error = lipgloss.Color("#FF5F5F")     // Red
		th.Muted = lipgloss.Color("#9CA3AF")     // Light gray

	case PaletteAmber:
		amber := lipgloss.Color("#FFB000")
		th.Primary = amber
		th.Secondary = lipgloss.Color("#FFC557")
		th.Accent = lipgloss.Color("#FFE28A")
		th.Success = lipgloss.Color("#FFD166")
		th.Warning = lipgloss.Color("#FF9800")
		th.Error = lipgloss.Color("#FF7043")
		th.Muted = lipgloss.Color("#C8A25A")

	case PaletteGreen:
		green := lipgloss.Color("#00FF87")
		th.Primary = green
		th.Secondary = lipgloss.Color("#48F7C1")
		th.Accent = lipgloss.Color("#7CFCFF")
		th.Success = green
		th.Warning = lipgloss.Color("#E7F562")
		th.Error = lipgloss.Color("#FF6B6B")
		th.Muted = lipgloss.Color("#7AC486")

	case PaletteC64:
		th.Primary = lipgloss.Color("#7CB7FF")   // Brighter C64 blue
		th.Secondary = lipgloss.Color("#C5A3FF") // Brighter C64 purple
		th.Accent = lipgloss.Color("#6DE0FF")    // C64 cyan
		th.Success = lipgloss.Color("#76D672")   // C64 green
		th.Warning = lipgloss.Color("#FFD38A")   // C64 yellow
		th.Error = lipgloss.Color("#FF8FA3")     // C64 red
		th.Muted = lipgloss.Color("#8D9BD1")     // C64 dark blue softened
	}

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
		Foreground(lipgloss.Color("#0A0A0A")).
		Background(th.Primary).
		Padding(0, 1)

	th.Highlight = lipgloss.NewStyle().
		Foreground(th.Accent).
		Bold(true)

	return th
}
