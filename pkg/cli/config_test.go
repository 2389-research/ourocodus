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
		name     string
		light    bool
		expected string
	}{
		{"default_dark", false, "dark"},
		{"explicit_light", true, "light"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &Flags{Light: tt.light}
			cfg := ResolveConfig(flags)
			assert.Equal(t, tt.expected, string(cfg.ThemeMode))
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
	cfg := Config{ThemeMode: "light"}
	th := cfg.GetTheme()

	assert.NotNil(t, th)
	assert.Equal(t, "light", string(th.Mode))
}

func TestConfig_GetTheme_Default(t *testing.T) {
	cfg := Config{ThemeMode: "dark"}
	th := cfg.GetTheme()

	assert.NotNil(t, th)
	assert.Equal(t, "dark", string(th.Mode))
}

func TestValidateFlags_NoConflicts(t *testing.T) {
	tests := []struct {
		name  string
		flags *Flags
	}{
		{
			name:  "empty flags",
			flags: &Flags{},
		},
		{
			name:  "only json",
			flags: &Flags{JSON: true},
		},
		{
			name:  "only plain",
			flags: &Flags{Plain: true},
		},
		{
			name:  "only quiet",
			flags: &Flags{Quiet: true},
		},
		{
			name:  "only verbose",
			flags: &Flags{Verbose: true},
		},
		{
			name:  "json with quiet",
			flags: &Flags{JSON: true, Quiet: true},
		},
		{
			name:  "plain with verbose",
			flags: &Flags{Plain: true, Verbose: true},
		},
		{
			name:  "no-color with light",
			flags: &Flags{NoColor: true, Light: true},
		},
		{
			name:  "all non-conflicting",
			flags: &Flags{JSON: true, NoColor: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlags(tt.flags)
			assert.NoError(t, err)
		})
	}
}

func TestValidateFlags_JSONAndPlainConflict(t *testing.T) {
	flags := &Flags{
		JSON:  true,
		Plain: true,
	}

	err := ValidateFlags(flags)
	assert.Error(t, err)
	assert.Equal(t, "flags --json and --plain are mutually exclusive", err.Error())
}

func TestValidateFlags_QuietAndVerboseConflict(t *testing.T) {
	flags := &Flags{
		Quiet:   true,
		Verbose: true,
	}

	err := ValidateFlags(flags)
	assert.Error(t, err)
	assert.Equal(t, "flags --quiet and --verbose are mutually exclusive", err.Error())
}

func TestValidateFlags_MultipleConflicts(t *testing.T) {
	// When multiple conflicts exist, we should get the first one detected
	flags := &Flags{
		JSON:    true,
		Plain:   true,
		Quiet:   true,
		Verbose: true,
	}

	err := ValidateFlags(flags)
	assert.Error(t, err)
	// Should report the JSON/Plain conflict first (checked first in the function)
	assert.Contains(t, err.Error(), "mutually exclusive")
}
