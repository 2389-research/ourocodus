package main

import (
	"strings"
	"testing"
)

// TestWatchCommand_Flags verifies that the watch command has the required flags
func TestWatchCommand_Flags(t *testing.T) {
	// Check for --raw flag
	rawFlag := watchCmd.Flags().Lookup("raw")
	if rawFlag == nil {
		t.Error("--raw flag not found")
	}

	// Check for --nats flag
	natsFlag := watchCmd.Flags().Lookup("nats")
	if natsFlag == nil {
		t.Error("--nats flag not found")
	}

	// Verify flag types
	if rawFlag != nil && rawFlag.Value.Type() != "bool" {
		t.Errorf("--raw flag should be bool, got %s", rawFlag.Value.Type())
	}

	if natsFlag != nil && natsFlag.Value.Type() != "string" {
		t.Errorf("--nats flag should be string, got %s", natsFlag.Value.Type())
	}

	// Verify default values
	if natsFlag != nil && natsFlag.DefValue != "nats://localhost:4222" {
		t.Errorf("--nats should default to 'nats://localhost:4222', got %s", natsFlag.DefValue)
	}

	if rawFlag != nil && rawFlag.DefValue != "false" {
		t.Errorf("--raw should default to false, got %s", rawFlag.DefValue)
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

// TestWatchCommand_Help verifies that the help text is descriptive
func TestWatchCommand_Help(t *testing.T) {
	// Get the long description
	helpText := watchCmd.Long
	if helpText == "" {
		helpText = watchCmd.Short
	}

	// Check for mentions of key features
	keywords := []string{"heartbeat", "nats", "real-time", "agent"}
	foundCount := 0

	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(helpText), keyword) {
			foundCount++
		}
	}

	if foundCount < 2 {
		t.Errorf("watch command help text should mention at least 2 of: %v, got: %s", keywords, helpText)
	}

	// Verify examples are present
	if watchCmd.Example == "" {
		t.Error("watch command should have example usage")
	}

	// Check that examples mention the --raw flag
	if !strings.Contains(watchCmd.Example, "--raw") {
		t.Error("watch command examples should show --raw flag usage")
	}
}
