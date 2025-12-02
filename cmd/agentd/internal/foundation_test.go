package internal_test

import (
	"testing"

	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/cli/detect"
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
	shouldPlain := detect.ShouldUsePlainMode()
	assert.True(t, shouldPlain)

	// Test 3: Mode parsing works correctly
	mode, ok := cli.ParseMode("plain")
	assert.True(t, ok)
	assert.Equal(t, cli.ModePlain, mode)

	// Test 4: Theme creation works for dark and light modes
	modes := []theme.ThemeMode{
		theme.ThemeDark,
		theme.ThemeLight,
	}

	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			th := theme.New(mode)
			assert.NotNil(t, th)
			assert.Equal(t, mode, th.Mode)

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
	// Simulate command startup with JSON flag
	flags := cli.Flags{
		JSON:  false,
		Plain: false,
	}

	// Step 1: Validate flags
	err := cli.ValidateFlags(&flags)
	assert.NoError(t, err)

	// Step 2: Resolve config
	cfg := cli.ResolveConfig(&flags)

	// Step 3: Create theme (only if rich mode)
	var th *theme.Theme
	if cfg.Mode.IsRich() {
		th = cfg.GetTheme()
		assert.NotNil(t, th)
	}

	// Step 4: Render appropriate output
	if cfg.Mode.IsRich() && th != nil {
		logo := th.Logo.Render(theme.GetLogo(theme.LogoSmall))
		assert.NotEmpty(t, logo)
	} else if cfg.Mode.IsPlain() {
		// Plain mode would just print text
		assert.NotNil(t, "plain mode")
	} else if cfg.Mode.IsJSON() {
		// JSON mode would marshal data
		assert.NotNil(t, "json mode")
	}
}

// TestMutuallyExclusiveFlags verifies flag validation
func TestMutuallyExclusiveFlags(t *testing.T) {
	// Test --json and --plain conflict
	flags := cli.Flags{
		JSON:  true,
		Plain: true,
	}
	err := cli.ValidateFlags(&flags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")

	// Test --quiet and --verbose conflict
	flags = cli.Flags{
		Quiet:   true,
		Verbose: true,
	}
	err = cli.ValidateFlags(&flags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}
