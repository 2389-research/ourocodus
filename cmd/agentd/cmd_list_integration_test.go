package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListCommand_Flags(t *testing.T) {
	// Test that new flags are registered
	assert.NotNil(t, listCmd.Flags().Lookup("format"))
	assert.NotNil(t, listCmd.Flags().Lookup("plain"))
	assert.NotNil(t, listCmd.Flags().Lookup("theme"))
}

func TestListCommand_FlagDefaults(t *testing.T) {
	// Verify default values
	formatFlag := listCmd.Flags().Lookup("format")
	assert.Equal(t, "auto", formatFlag.DefValue)

	themeFlag := listCmd.Flags().Lookup("theme")
	assert.Equal(t, "cga", themeFlag.DefValue)
}

func TestListCommand_Help(t *testing.T) {
	var buf bytes.Buffer
	listCmd.SetOut(&buf)
	listCmd.SetArgs([]string{"--help"})

	err := listCmd.Execute()
	assert.NoError(t, err)

	helpText := buf.String()
	assert.Contains(t, helpText, "--format")
	assert.Contains(t, helpText, "--plain")
	assert.Contains(t, helpText, "--theme")
}
