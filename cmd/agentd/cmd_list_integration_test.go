package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListCommand_Flags(t *testing.T) {
	// Test that list command has NATS flag
	assert.NotNil(t, listCmd.Flags().Lookup("nats"))
}

func TestListCommand_FlagDefaults(t *testing.T) {
	// Verify default values
	natsFlag := listCmd.Flags().Lookup("nats")
	assert.Equal(t, defaultNATSURL, natsFlag.DefValue)
}

func TestListCommand_Help(t *testing.T) {
	// Get the usage string which includes flags
	helpText := listCmd.UsageString()
	assert.Contains(t, helpText, "--nats")
	// Note: --json, --plain, --light are now global flags from pkg/cli
}
