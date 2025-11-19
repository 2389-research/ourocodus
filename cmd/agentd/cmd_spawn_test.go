package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Test pure functions first
func TestGenerateShortID(t *testing.T) {
	id := generateShortID()

	// Should be 6 characters
	if len(id) != 6 {
		t.Errorf("generateShortID() = %q, want 6 characters, got %d", id, len(id))
	}

	// Should be hex characters only
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateShortID() = %q, contains non-hex character %q", id, c)
		}
	}
}

func TestGenerateAgentID(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string // empty means we just check the format
	}{
		{
			name: "with explicit ID",
			args: []string{"alice"},
			want: "alice",
		},
		{
			name: "with empty string",
			args: []string{""},
			want: "", // should generate
		},
		{
			name: "no args",
			args: []string{},
			want: "", // should generate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateAgentID(tt.args)

			if tt.want != "" {
				// Explicit ID provided
				if got != tt.want {
					t.Errorf("generateAgentID() = %v, want %v", got, tt.want)
				}
			} else {
				// Generated ID - check format
				if len(got) == 0 {
					t.Error("generateAgentID() returned empty string")
				}
				// Should start with "agent-"
				if len(got) < 7 || got[:6] != "agent-" {
					t.Errorf("generateAgentID() = %q, want format 'agent-<id>'", got)
				}
			}
		})
	}
}

func TestParseEnvFlags(t *testing.T) {
	tests := []struct {
		name    string
		flags   []string
		wantErr bool
	}{
		{
			name:    "valid flags",
			flags:   []string{"KEY=VALUE", "DEBUG=1"},
			wantErr: false,
		},
		{
			name:    "empty list",
			flags:   []string{},
			wantErr: false,
		},
		{
			name:    "invalid format - no equals",
			flags:   []string{"INVALID"},
			wantErr: true,
		},
		{
			name:    "valid with empty value",
			flags:   []string{"KEY="},
			wantErr: false,
		},
		{
			name:    "mixed valid and invalid",
			flags:   []string{"VALID=1", "INVALID"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnvFlags(tt.flags)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEnvFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.flags) {
				t.Errorf("parseEnvFlags() returned %d flags, want %d", len(got), len(tt.flags))
			}
		})
	}
}

func TestHasCredentialFiles(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(dir string)
		want      bool
	}{
		{
			name: "with credential file",
			setupFunc: func(dir string) {
				os.WriteFile(filepath.Join(dir, "api-key"), []byte("secret"), 0600)
			},
			want: true,
		},
		{
			name: "with hidden file only",
			setupFunc: func(dir string) {
				os.WriteFile(filepath.Join(dir, ".hidden"), []byte("data"), 0600)
			},
			want: false,
		},
		{
			name: "empty directory",
			setupFunc: func(dir string) {
				// No files
			},
			want: false,
		},
		{
			name: "with subdirectory",
			setupFunc: func(dir string) {
				os.Mkdir(filepath.Join(dir, "subdir"), 0755)
			},
			want: false,
		},
		{
			name: "nonexistent directory",
			setupFunc: func(dir string) {
				// Use a path that doesn't exist
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dir string
			if tt.name != "nonexistent directory" {
				dir = t.TempDir()
				tt.setupFunc(dir)
			} else {
				dir = "/nonexistent/path/12345"
			}

			got := hasCredentialFiles(dir)
			if got != tt.want {
				t.Errorf("hasCredentialFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test buildSpawnConfig
func TestBuildSpawnConfig(t *testing.T) {
	tests := []struct {
		name     string
		agentID  string
		envFlags []string
		wantErr  bool
	}{
		{
			name:     "basic config",
			agentID:  "test-agent",
			envFlags: []string{},
			wantErr:  false,
		},
		{
			name:     "with environment variables",
			agentID:  "test-agent",
			envFlags: []string{"DEBUG=1", "LOG_LEVEL=trace"},
			wantErr:  false,
		},
		{
			name:     "invalid env format",
			agentID:  "test-agent",
			envFlags: []string{"INVALID"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore the global variable
			oldSpawnEnv := spawnEnv
			defer func() { spawnEnv = oldSpawnEnv }()

			// Set the global variable
			spawnEnv = tt.envFlags

			config, err := buildSpawnConfig(tt.agentID)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildSpawnConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify config fields
				if config.AgentID != tt.agentID {
					t.Errorf("buildSpawnConfig() AgentID = %v, want %v", config.AgentID, tt.agentID)
				}
				if config.ImageName != "ourocodus/agent:latest" {
					t.Errorf("buildSpawnConfig() ImageName = %v, want default", config.ImageName)
				}
				if len(config.Command) == 0 {
					t.Error("buildSpawnConfig() Command is empty")
				}
				if len(config.Entrypoint) == 0 {
					t.Error("buildSpawnConfig() Entrypoint is empty")
				}
			}
		})
	}
}
