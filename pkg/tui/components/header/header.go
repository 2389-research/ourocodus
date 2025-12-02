// Package header provides a reusable header component with the Ourocodus logo.
package header

import (
	"strings"

	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Height constants for layout calculations.
// These help TUIs properly calculate viewport heights without magic numbers.
const (
	// LogoHeight is the number of lines in the logo ASCII art.
	LogoHeight = 5

	// BorderHeight is the vertical space added by the border (top + bottom).
	BorderHeight = 2

	// Height is the total height of the header component with border.
	// Use this when calculating viewport heights: availableHeight = windowHeight - header.Height - footerHeight
	Height = LogoHeight + BorderHeight // 7 lines total
)

// Render renders the Ourocodus logo header with rainbow colors and a border.
// If th is nil, a default dark theme is used.
func Render(th *theme.Theme) string {
	return RenderWithContent(th, "")
}

// RenderWithContent renders the logo header with optional additional content
// displayed to the right of the logo box.
func RenderWithContent(th *theme.Theme, content string) string {
	th = theme.Ensure(th)
	logoBox := renderLogoBox(th)

	if content == "" {
		return logoBox
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, logoBox, "  ", content)
}

// RenderRainbow renders the logo with rainbow colors but no border.
// This is a convenience function for simpler layouts.
func RenderRainbow(th *theme.Theme) string {
	th = theme.Ensure(th)
	return renderRainbowLogo(th)
}

// renderLogoBox renders the logo with rainbow colors in a bordered box.
func renderLogoBox(th *theme.Theme) string {
	content := renderRainbowLogo(th)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Primary).
		Padding(0, 1).
		Render(content)
}

// renderRainbowLogo renders the logo text with rainbow gradient colors.
func renderRainbowLogo(th *theme.Theme) string {
	logo := theme.GetLogo(theme.LogoSmall)
	lines := strings.Split(logo, "\n")
	coloredLines := make([]string, 0, len(lines))
	for i, line := range lines {
		color := th.Rainbow[i%len(th.Rainbow)]
		coloredLine := lipgloss.NewStyle().Foreground(color).Render(line)
		coloredLines = append(coloredLines, coloredLine)
	}
	return strings.Join(coloredLines, "\n")
}
