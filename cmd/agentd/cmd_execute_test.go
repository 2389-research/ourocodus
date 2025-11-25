package main

import (
	"strings"
	"testing"
)

// Test that execute command validates arguments
func TestExecuteCommand_Args(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid args",
			args:    []string{"alice", "ls /workspace"},
			wantErr: false,
		},
		{
			name:    "missing command",
			args:    []string{"alice"},
			wantErr: true,
		},
		{
			name:    "missing agent ID",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Args validation
			err := executeCmd.Args(executeCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeCmd.Args() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test execute command flags
func TestExecuteCommand_Flags(t *testing.T) {
	// Check timeout flag exists
	timeoutFlag := executeCmd.Flags().Lookup("timeout")
	if timeoutFlag == nil {
		t.Error("execute command missing --timeout flag")
		return
	}
	if timeoutFlag.DefValue != "30" {
		t.Errorf("execute timeout default = %s, want 30", timeoutFlag.DefValue)
	}

	// Check shell flag exists
	shellFlag := executeCmd.Flags().Lookup("shell")
	if shellFlag == nil {
		t.Error("execute command missing --shell flag")
		return
	}
	if shellFlag.DefValue != "/bin/bash" {
		t.Errorf("execute shell default = %s, want /bin/bash", shellFlag.DefValue)
	}
}

// Test execute command metadata
func TestExecuteCommand_Metadata(t *testing.T) {
	if executeCmd.Use != "execute <agent-id> <command>" {
		t.Errorf("execute Use = %q, want 'execute <agent-id> <command>'", executeCmd.Use)
	}

	if !strings.Contains(executeCmd.Short, "Execute") {
		t.Errorf("execute Short doesn't mention executing commands: %s", executeCmd.Short)
	}

	if !strings.Contains(executeCmd.Long, "shell command") {
		t.Errorf("execute Long doesn't explain shell commands: %s", executeCmd.Long)
	}

	if len(executeCmd.Example) == 0 {
		t.Error("execute command has no examples")
	}
}
