package main

import (
	"strings"
	"testing"
)

// Test that attach command validates arguments
func TestAttachCommand_Args(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid args",
			args:    []string{"alice"},
			wantErr: false,
		},
		{
			name:    "missing agent ID",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "too many args",
			args:    []string{"alice", "bob"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Args validation
			err := attachCmd.Args(attachCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("attachCmd.Args() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test attach command metadata
func TestAttachCommand_Metadata(t *testing.T) {
	if attachCmd.Use != "attach <agent-id>" {
		t.Errorf("attach Use = %q, want 'attach <agent-id>'", attachCmd.Use)
	}

	if !strings.Contains(attachCmd.Short, "interactive shell") {
		t.Errorf("attach Short doesn't mention interactive shell: %s", attachCmd.Short)
	}

	if !strings.Contains(attachCmd.Long, "interactive bash session") {
		t.Errorf("attach Long doesn't explain bash session: %s", attachCmd.Long)
	}

	if !strings.Contains(attachCmd.Long, "Ctrl-D") {
		t.Errorf("attach Long doesn't explain how to detach: %s", attachCmd.Long)
	}

	if len(attachCmd.Example) == 0 {
		t.Error("attach command has no examples")
	}
}

// Test terminal state restoration functions exist
func TestAttachCommand_TerminalFunctions(t *testing.T) {
	// These functions should be defined and callable
	// We can't easily test them without a real terminal, but we can verify they exist

	// Test that setRawTerminal handles nil gracefully
	state, err := setRawTerminal()
	// This might fail if not run in a terminal, which is expected
	if err == nil && state != nil {
		// If it succeeded, test restoration
		if err := restoreTerminal(state); err != nil {
			t.Errorf("restoreTerminal() error = %v", err)
		}
	}

	// Test that restoreTerminal handles nil gracefully
	if err := restoreTerminal(nil); err != nil {
		t.Errorf("restoreTerminal(nil) should not error, got: %v", err)
	}
}
