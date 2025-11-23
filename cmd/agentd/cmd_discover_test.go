package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscoverCommand_FlagsRegistered(t *testing.T) {
	// Test that all expected flags are registered
	formatFlag := discoverCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "Expected --format flag to be registered")
	assert.Equal(t, "table", formatFlag.DefValue, "Expected --format to default to 'table'")

	watchFlag := discoverCmd.Flags().Lookup("watch")
	assert.NotNil(t, watchFlag, "Expected --watch flag to be registered")
	assert.Equal(t, "false", watchFlag.DefValue, "Expected --watch to default to false")
}

func TestDiscoverCommand_HelpText(t *testing.T) {
	help := discoverCmd.Long
	assert.Contains(t, help, "adoption status", "Expected help text to mention adoption status")
	assert.Contains(t, help, "Docker", "Expected help text to mention Docker")
	assert.Contains(t, help, "lease", "Expected help text to mention lease status")
}

func TestDiscoverCommand_Examples(t *testing.T) {
	examples := discoverCmd.Example
	assert.Contains(t, examples, "--watch", "Expected examples to show --watch flag")
	assert.Contains(t, examples, "--format json", "Expected examples to show JSON format")
}

func TestDiscoverCommand_ShortDescription(t *testing.T) {
	assert.NotEmpty(t, discoverCmd.Short, "Expected short description to be set")
	assert.Contains(t, discoverCmd.Short, "Discover", "Expected short description to mention 'Discover'")
}
