package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscoverCommand_FlagsRegistered(t *testing.T) {
	// Test that all expected flags are registered
	formatFlag := discoverCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "Expected --format flag to be registered")
	assert.Equal(t, "auto", formatFlag.DefValue, "Expected --format to default to 'auto'")

	plainFlag := discoverCmd.Flags().Lookup("plain")
	assert.NotNil(t, plainFlag, "Expected --plain flag to be registered")
	assert.Equal(t, "false", plainFlag.DefValue, "Expected --plain to default to false")

	themeFlag := discoverCmd.Flags().Lookup("theme")
	assert.NotNil(t, themeFlag, "Expected --theme flag to be registered")
	assert.Equal(t, "cga", themeFlag.DefValue, "Expected --theme to default to 'cga'")

	watchFlag := discoverCmd.Flags().Lookup("watch")
	assert.NotNil(t, watchFlag, "Expected --watch flag to be registered")
	assert.Equal(t, "false", watchFlag.DefValue, "Expected --watch to default to false")
}

func TestDiscoverCommand_HelpText(t *testing.T) {
	help := discoverCmd.Long
	assert.Contains(t, help, "watch", "Expected help text to mention watch mode")
	assert.Contains(t, help, "auto-refresh", "Expected help text to mention auto-refresh")
	assert.Contains(t, help, "2 seconds", "Expected help text to specify refresh interval")
}

func TestDiscoverCommand_Examples(t *testing.T) {
	examples := discoverCmd.Example
	assert.Contains(t, examples, "--watch", "Expected examples to show --watch flag")
	assert.Contains(t, examples, "--theme", "Expected examples to show --theme flag")
	assert.Contains(t, examples, "--plain", "Expected examples to show --plain flag")
	assert.Contains(t, examples, "--format json", "Expected examples to show JSON format")
}

func TestDiscoverCommand_ShortDescription(t *testing.T) {
	assert.NotEmpty(t, discoverCmd.Short, "Expected short description to be set")
	assert.Contains(t, discoverCmd.Short, "Discover", "Expected short description to mention 'Discover'")
}
