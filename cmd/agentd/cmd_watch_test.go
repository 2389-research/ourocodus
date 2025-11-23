package main

import (
	"strings"
	"testing"
)

// TestWatchCommand_Flags verifies that the watch command has the required flags
func TestWatchCommand_Flags(t *testing.T) {
	// Check for --theme flag
	themeFlag := watchCmd.Flags().Lookup("theme")
	if themeFlag == nil {
		t.Error("--theme flag not found")
	}

	// Check for --plain flag
	plainFlag := watchCmd.Flags().Lookup("plain")
	if plainFlag == nil {
		t.Error("--plain flag not found")
	}

	// Check for --json flag
	jsonFlag := watchCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("--json flag not found")
	}

	// Verify flag types
	if themeFlag != nil && themeFlag.Value.Type() != "string" {
		t.Errorf("--theme flag should be string, got %s", themeFlag.Value.Type())
	}

	if plainFlag != nil && plainFlag.Value.Type() != "bool" {
		t.Errorf("--plain flag should be bool, got %s", plainFlag.Value.Type())
	}

	if jsonFlag != nil && jsonFlag.Value.Type() != "bool" {
		t.Errorf("--json flag should be bool, got %s", jsonFlag.Value.Type())
	}
}

// TestWatchCommand_Arguments verifies that the watch command requires exactly one argument
func TestWatchCommand_Arguments(t *testing.T) {
	// Check that Args is set (should require exactly 1 argument)
	if watchCmd.Args == nil {
		t.Error("watch command should validate arguments")
		return
	}

	// Test with no arguments (should fail)
	err := watchCmd.Args(watchCmd, []string{})
	if err == nil {
		t.Error("watch command should reject zero arguments")
	}

	// Test with one argument (should pass)
	err = watchCmd.Args(watchCmd, []string{"test-agent-id"})
	if err != nil {
		t.Errorf("watch command should accept exactly one argument, got error: %v", err)
	}

	// Test with two arguments (should fail)
	err = watchCmd.Args(watchCmd, []string{"test-agent-id", "extra-arg"})
	if err == nil {
		t.Error("watch command should reject more than one argument")
	}
}

// TestWatchCommand_Help verifies that the help text mentions keyboard shortcuts
func TestWatchCommand_Help(t *testing.T) {
	// Get the long description
	helpText := watchCmd.Long
	if helpText == "" {
		helpText = watchCmd.Short
	}

	// Check for keyboard shortcut mentions
	// The help should mention common shortcuts like q, r, ?
	shortcuts := []string{"q", "r", "?"}
	foundAny := false

	for _, shortcut := range shortcuts {
		if strings.Contains(strings.ToLower(helpText), shortcut) {
			foundAny = true
			break
		}
	}

	if !foundAny {
		t.Errorf("watch command help text should mention keyboard shortcuts (q, r, ?), got: %s", helpText)
	}

	// Verify it mentions interactive/TUI mode
	if !strings.Contains(strings.ToLower(helpText), "interactive") &&
		!strings.Contains(strings.ToLower(helpText), "tui") &&
		!strings.Contains(strings.ToLower(helpText), "rich") {
		t.Error("watch command help text should mention interactive/TUI/rich mode")
	}
}
