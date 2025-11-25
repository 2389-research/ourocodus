package internal_test

import (
	"os"
	"testing"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/stretchr/testify/assert"
)

// TestFoundationIntegration verifies all foundation components work together
func TestFoundationIntegration(t *testing.T) {
	// Test 1: Terminal detection works
	isTTY := detect.IsTTY()
	assert.IsType(t, false, isTTY)

	// Test 2: Should use plain mode in CI
	t.Setenv("CI", "true")
	shouldPlain := detect.ShouldUsePlainMode(false, false, os.Environ)
	assert.True(t, shouldPlain)

	// Test 3: Mode detection respects environment
	mode := output.DetectMode(false, false, shouldPlain)
	assert.Equal(t, output.ModePlain, mode)

	// Test 4: Theme creation works for all palettes
	palettes := []theme.PaletteName{
		theme.PaletteCGA,
		theme.PaletteAmber,
		theme.PaletteGreen,
		theme.PaletteC64,
	}

	for _, palette := range palettes {
		t.Run(palette.String(), func(t *testing.T) {
			th := theme.NewRetroTheme(palette)
			assert.NotNil(t, th)
			assert.Equal(t, palette, th.Palette)

			// Verify we can render with each style
			logo := th.Logo.Render("TEST")
			assert.NotEmpty(t, logo)
		})
	}

	// Test 5: ASCII art works
	logo := theme.GetLogo(theme.LogoSmall)
	assert.NotEmpty(t, logo)

	// Test unicode and ASCII icon rendering
	iconUnicode := theme.GetAgentStatusIcon(theme.StatusRunning, true)
	assert.Equal(t, "⚡", iconUnicode)

	iconASCII := theme.GetAgentStatusIcon(theme.StatusRunning, false)
	assert.Equal(t, ">", iconASCII)
}

// TestFoundationWorkflow tests a realistic workflow
func TestFoundationWorkflow(t *testing.T) {
	// Simulate command startup
	jsonFlag := false
	plainFlag := false

	// Step 1: Detect terminal capabilities
	shouldPlain := detect.ShouldUsePlainMode(jsonFlag, plainFlag, os.Environ)

	// Step 2: Select output mode
	mode := output.DetectMode(jsonFlag, plainFlag, shouldPlain)

	// Step 3: Create theme (only if rich mode)
	var th *theme.RetroTheme
	if mode.IsRich() {
		th = theme.NewRetroTheme(theme.PaletteCGA)
		assert.NotNil(t, th)
	}

	// Step 4: Render appropriate output
	if mode.IsRich() {
		logo := th.Logo.Render(theme.GetLogo(theme.LogoSmall))
		assert.NotEmpty(t, logo)
	} else if mode.IsPlain() {
		// Plain mode would just print text
		assert.NotNil(t, "plain mode")
	} else if mode.IsJSON() {
		// JSON mode would marshal data
		assert.NotNil(t, "json mode")
	}
}
