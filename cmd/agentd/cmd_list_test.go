package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test pure formatting functions
func TestFormatWorkspace(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "short path",
			path: "/short/path",
			want: "/short/path",
		},
		{
			name: "long path gets truncated",
			path: "/very/long/path/that/exceeds/sixty/characters/and/should/be/truncated/significantly",
			want: "...", // Should start with ...
		},
		{
			name: "exactly 60 characters",
			path: "/exactly/sixty/characters/path/to/test/truncation/limit",
			want: "/exactly/sixty/characters/path/to/test/truncation/limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatWorkspace(tt.path)

			if len(tt.path) > 60 {
				// Should be truncated and start with ...
				if len(got) != 60 {
					t.Errorf("formatWorkspace() long path length = %d, want 60", len(got))
				}
				if got[:3] != "..." {
					t.Errorf("formatWorkspace() long path = %q, should start with '...'", got)
				}
			} else {
				// Should be unchanged
				if got != tt.path {
					t.Errorf("formatWorkspace() = %q, want %q", got, tt.path)
				}
			}
		})
	}
}

func TestFormatContainerID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want int // expected length
	}{
		{
			name: "long container ID",
			id:   "abc123def456ghi789jkl012mno345",
			want: 12,
		},
		{
			name: "short container ID",
			id:   "abc123",
			want: 6,
		},
		{
			name: "exactly 12 characters",
			id:   "abc123def456",
			want: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatContainerID(tt.id)
			if len(got) != tt.want {
				t.Errorf("formatContainerID() length = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			want:     "30s ago",
		},
		{
			name:     "minutes",
			duration: 5 * time.Minute,
			want:     "5m ago",
		},
		{
			name:     "hours",
			duration: 3 * time.Hour,
			want:     "3h ago",
		},
		{
			name:     "days",
			duration: 2 * 24 * time.Hour,
			want:     "2d ago",
		},
		{
			name:     "mixed - rounds down to minutes",
			duration: 5*time.Minute + 30*time.Second,
			want:     "5m ago",
		},
		{
			name:     "mixed - rounds down to hours",
			duration: 3*time.Hour + 30*time.Minute,
			want:     "3h ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

// TestListCommand_FlagsRegistered verifies that the list command has expected flags
func TestListCommand_FlagsRegistered(t *testing.T) {
	// Check for --format flag
	formatFlag := listCmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "Expected --format flag to be registered")
	assert.Equal(t, "auto", formatFlag.DefValue, "Expected --format to default to 'auto'")

	// Check for --plain flag
	plainFlag := listCmd.Flags().Lookup("plain")
	assert.NotNil(t, plainFlag, "Expected --plain flag to be registered")
	assert.Equal(t, "false", plainFlag.DefValue, "Expected --plain to default to false")

	// Check for --theme flag
	themeFlag := listCmd.Flags().Lookup("theme")
	assert.NotNil(t, themeFlag, "Expected --theme flag to be registered")
	assert.Equal(t, "cga", themeFlag.DefValue, "Expected --theme to default to 'cga'")

	// Check for --watch flag (newly added)
	watchFlag := listCmd.Flags().Lookup("watch")
	assert.NotNil(t, watchFlag, "Expected --watch flag to be registered")
	assert.Equal(t, "false", watchFlag.DefValue, "Expected --watch to default to false")
}
