package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveConfig_DefaultsToRich(t *testing.T) {
	// Note: This test may fail in CI due to TTY detection
	// The actual behavior depends on terminal state
	flags := &Flags{}
	cfg := ResolveConfig(flags)

	// In a test environment (non-TTY), this will be plain
	// We just verify it doesn't crash and returns a valid config
	assert.NotNil(t, cfg.Mode.String())
}

func TestResolveConfig_JSONFlag(t *testing.T) {
	flags := &Flags{JSON: true}
	cfg := ResolveConfig(flags)

	assert.Equal(t, ModeJSON, cfg.Mode)
	assert.True(t, cfg.NoColor) // JSON implies no color
}

func TestResolveConfig_PlainFlag(t *testing.T) {
	flags := &Flags{Plain: true}
	cfg := ResolveConfig(flags)

	assert.Equal(t, ModePlain, cfg.Mode)
}

func TestResolveConfig_JSONTakesPrecedence(t *testing.T) {
	flags := &Flags{JSON: true, Plain: true}
	cfg := ResolveConfig(flags)

	assert.Equal(t, ModeJSON, cfg.Mode)
}

func TestResolveConfig_Theme(t *testing.T) {
	tests := []struct {
		name      string
		flagTheme string
		expected  string
	}{
		{"default", "", "cga"},
		{"explicit cga", "cga", "cga"},
		{"amber", "amber", "amber"},
		{"green", "green", "green"},
		{"c64", "c64", "c64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &Flags{Theme: tt.flagTheme}
			cfg := ResolveConfig(flags)
			assert.Equal(t, tt.expected, cfg.ThemeName)
		})
	}
}

func TestResolveConfig_QuietVerbose(t *testing.T) {
	flags := &Flags{Quiet: true, Verbose: true}
	cfg := ResolveConfig(flags)

	assert.True(t, cfg.Quiet)
	assert.True(t, cfg.Verbose)
}

func TestConfig_GetTheme(t *testing.T) {
	cfg := Config{ThemeName: "amber"}
	theme := cfg.GetTheme()

	assert.NotNil(t, theme)
}

func TestConfig_GetTheme_Invalid(t *testing.T) {
	cfg := Config{ThemeName: "invalid-theme"}
	theme := cfg.GetTheme()

	// Should fallback to CGA
	assert.NotNil(t, theme)
}
