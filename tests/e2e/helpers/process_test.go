package helpers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateScriptPath(t *testing.T) {
	// Find project root for testing
	projectRoot, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}

	scriptsDir := filepath.Join(projectRoot, "scripts")

	tests := []struct {
		name       string
		scriptPath string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid script in scripts directory",
			scriptPath: filepath.Join(scriptsDir, "setup-worktrees.sh"),
			wantErr:    false,
		},
		{
			name:       "relative path rejected",
			scriptPath: "scripts/setup-worktrees.sh",
			wantErr:    true,
			errMsg:     "script path must be absolute",
		},
		{
			name:       "path traversal with ..",
			scriptPath: filepath.Join(scriptsDir, "..", "cmd", "relay", "main.go"),
			wantErr:    true,
			errMsg:     "script path must be within project scripts directory",
		},
		{
			name:       "path outside scripts directory",
			scriptPath: filepath.Join(projectRoot, "cmd", "relay", "main.go"),
			wantErr:    true,
			errMsg:     "script path must be within project scripts directory",
		},
		{
			name:       "nonexistent file",
			scriptPath: filepath.Join(scriptsDir, "nonexistent-script.sh"),
			wantErr:    true,
			errMsg:     "script file does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScriptPath(tt.scriptPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateScriptPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if len(err.Error()) < len(tt.errMsg) || err.Error()[:len(tt.errMsg)] != tt.errMsg {
					t.Errorf("validateScriptPath() error message = %v, want to start with %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateScriptPath_Directory(t *testing.T) {
	// Find project root for testing
	projectRoot, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}

	scriptsDir := filepath.Join(projectRoot, "scripts")

	// Test that directory path is rejected
	err = validateScriptPath(scriptsDir)
	if err == nil {
		t.Error("validateScriptPath() should reject directory path")
	}
	if err != nil && err.Error()[:35] != "script path must be a regular file:" {
		t.Errorf("validateScriptPath() error = %v, want 'script path must be a regular file:'", err)
	}
}

func TestValidateScriptPath_Symlink(t *testing.T) {
	// Find project root for testing
	projectRoot, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}

	scriptsDir := filepath.Join(projectRoot, "scripts")

	// Create a temporary symlink in scripts directory for testing
	targetFile := filepath.Join(scriptsDir, "setup-worktrees.sh")

	// Skip if target doesn't exist
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Skip("Target file doesn't exist, skipping symlink test")
	}

	symlinkPath := filepath.Join(scriptsDir, "test-symlink.sh")

	// Create symlink
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Skipf("Failed to create symlink (may need permissions): %v", err)
	}

	// Clean up symlink after test
	defer func() {
		_ = os.Remove(symlinkPath)
	}()

	// Test that symlink path is rejected
	err = validateScriptPath(symlinkPath)
	if err == nil {
		t.Error("validateScriptPath() should reject symlink path")
	}
	if err != nil && len(err.Error()) >= 33 && err.Error()[:33] != "script path must not be a symlink" {
		t.Errorf("validateScriptPath() error = %v, want to start with 'script path must not be a symlink'", err)
	}
}
