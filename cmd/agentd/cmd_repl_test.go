package main

import (
	"strings"
	"testing"
)

// Test that repl command validates arguments
func TestReplCommand_Args(t *testing.T) {
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
			err := replCmd.Args(replCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("replCmd.Args() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test repl command metadata and educational content
func TestReplCommand_Metadata(t *testing.T) {
	if replCmd.Use != "repl <agent-id>" {
		t.Errorf("repl Use = %q, want 'repl <agent-id>'", replCmd.Use)
	}

	if !strings.Contains(replCmd.Short, "ACP") {
		t.Errorf("repl Short doesn't mention ACP: %s", replCmd.Short)
	}

	// Check that long description explains the relay requirement
	if !strings.Contains(replCmd.Long, "relay") {
		t.Errorf("repl Long doesn't explain relay requirement: %s", replCmd.Long)
	}

	if !strings.Contains(replCmd.Long, "WebSocket") {
		t.Errorf("repl Long doesn't mention WebSocket: %s", replCmd.Long)
	}

	if len(replCmd.Example) == 0 {
		t.Error("repl command has no examples")
	}

	// Check examples mention the relay
	if !strings.Contains(replCmd.Example, "relay") {
		t.Errorf("repl Example doesn't mention relay: %s", replCmd.Example)
	}
}

// Test that repl command properly reports not implemented status
func TestReplCommand_NotImplemented(t *testing.T) {
	// The runREPL function should return an error indicating it's not implemented
	err := runREPL(replCmd, []string{"test-agent"})

	if err == nil {
		t.Error("runREPL() should return error for unimplemented feature")
	}

	if !strings.Contains(err.Error(), "relay") {
		t.Errorf("runREPL() error should mention relay, got: %v", err)
	}
}
