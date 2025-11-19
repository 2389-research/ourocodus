package main

import (
	"strings"
	"testing"
)

// Test that send command validates arguments
func TestSendCommand_Args(t *testing.T) {
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
			err := sendCmd.Args(sendCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("sendCmd.Args() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test send command flags
func TestSendCommand_Flags(t *testing.T) {
	// Check timeout flag exists
	timeoutFlag := sendCmd.Flags().Lookup("timeout")
	if timeoutFlag == nil {
		t.Error("send command missing --timeout flag")
	}
	if timeoutFlag.DefValue != "30" {
		t.Errorf("send timeout default = %s, want 30", timeoutFlag.DefValue)
	}

	// Check shell flag exists
	shellFlag := sendCmd.Flags().Lookup("shell")
	if shellFlag == nil {
		t.Error("send command missing --shell flag")
	}
	if shellFlag.DefValue != "/bin/bash" {
		t.Errorf("send shell default = %s, want /bin/bash", shellFlag.DefValue)
	}
}

// Test send command metadata
func TestSendCommand_Metadata(t *testing.T) {
	if sendCmd.Use != "send <agent-id> <command>" {
		t.Errorf("send Use = %q, want 'send <agent-id> <command>'", sendCmd.Use)
	}

	if !strings.Contains(sendCmd.Short, "Send a command") {
		t.Errorf("send Short doesn't mention sending commands: %s", sendCmd.Short)
	}

	if !strings.Contains(sendCmd.Long, "shell command") {
		t.Errorf("send Long doesn't explain shell commands: %s", sendCmd.Long)
	}

	if len(sendCmd.Example) == 0 {
		t.Error("send command has no examples")
	}
}
