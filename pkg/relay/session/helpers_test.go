package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestValidateNonEmpty(t *testing.T) {
	testErr := errors.New("test error")

	tests := []struct {
		name      string
		value     string
		err       error
		wantErr   bool
		wantError error
	}{
		{
			name:      "valid non-empty string",
			value:     "test",
			err:       testErr,
			wantErr:   false,
			wantError: nil,
		},
		{
			name:      "empty string",
			value:     "",
			err:       testErr,
			wantErr:   true,
			wantError: testErr,
		},
		{
			name:      "whitespace-only string",
			value:     "   ",
			err:       testErr,
			wantErr:   true,
			wantError: testErr,
		},
		{
			name:      "string with content and whitespace",
			value:     "  test  ",
			err:       testErr,
			wantErr:   false,
			wantError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonEmpty(tt.value, tt.err)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNonEmpty() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != tt.wantError {
				t.Errorf("ValidateNonEmpty() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateWorkspacePath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "workspace-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Get absolute path of temp dir
	baseDir, err := filepath.Abs(tempDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(baseDir, "subdir")
	if err := os.MkdirAll(subDir, 0o700); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	tests := []struct {
		name        string
		workspace   string
		baseDir     string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid subdirectory",
			workspace: subDir,
			baseDir:   baseDir,
			wantErr:   false,
		},
		{
			name:      "base directory itself",
			workspace: baseDir,
			baseDir:   baseDir,
			wantErr:   false,
		},
		{
			name:        "path traversal attempt with ..",
			workspace:   filepath.Join(baseDir, "..", "evil"),
			baseDir:     baseDir,
			wantErr:     true,
			errContains: "must be under base directory",
		},
		{
			name:        "absolute path outside base",
			workspace:   "/tmp/evil",
			baseDir:     baseDir,
			wantErr:     true,
			errContains: "must be under base directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath, err := ValidateWorkspacePath(tt.workspace, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspacePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && absPath == "" {
				t.Error("ValidateWorkspacePath() returned empty path for valid input")
			}
			if tt.wantErr && absPath != "" {
				t.Error("ValidateWorkspacePath() returned non-empty path for invalid input")
			}
		})
	}
}

func TestCloseACPClientSafely(t *testing.T) {
	tests := []struct {
		name       string
		setupAgent func() *AgentSession
		closeErr   error
		wantErr    bool
	}{
		{
			name: "successful close",
			setupAgent: func() *AgentSession {
				agent := NewAgentSession("agent1", "/workspace", time.Now())
				agent.acpClient = &mockACPClient{
					closeFunc: func(context.Context) error { return nil },
				}
				return agent
			},
			closeErr: nil,
			wantErr:  false,
		},
		{
			name: "close with error",
			setupAgent: func() *AgentSession {
				agent := NewAgentSession("agent1", "/workspace", time.Now())
				agent.acpClient = &mockACPClient{
					closeFunc: func(context.Context) error { return errors.New("close failed") },
				}
				return agent
			},
			closeErr: errors.New("close failed"),
			wantErr:  true,
		},
		{
			name: "no client to close",
			setupAgent: func() *AgentSession {
				agent := NewAgentSession("agent1", "/workspace", time.Now())
				agent.acpClient = nil
				return agent
			},
			closeErr: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := tt.setupAgent()
			logger := &mockLogger{
				messages: []string{},
				mu:       sync.Mutex{},
			}
			ctx := context.Background()

			err := CloseACPClientSafely(agent, ctx, logger, "session1", "agent1")
			if (err != nil) != tt.wantErr {
				t.Errorf("CloseACPClientSafely() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify client was cleared
			agent.mu.Lock()
			if agent.acpClient != nil {
				t.Error("ACP client was not cleared from agent session")
			}
			// Verify state was set to AgentTerminated
			if agent.state != AgentTerminated {
				t.Errorf("Agent state = %v, want %v", agent.state, AgentTerminated)
			}
			agent.mu.Unlock()
		})
	}
}
