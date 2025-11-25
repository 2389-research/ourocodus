package list

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatAttached(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string returns dash",
			input:    "",
			expected: "–",
		},
		{
			name:     "short string unchanged",
			input:    "session-1",
			expected: "session-1",
		},
		{
			name:     "exactly 12 chars unchanged",
			input:    "123456789012",
			expected: "123456789012",
		},
		{
			name:     "long string truncated with ellipsis",
			input:    "session-abc123def456",
			expected: "session-abc1…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAttached(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time returns dash",
			input:    time.Time{},
			expected: "–",
		},
		{
			name:     "less than a minute ago",
			input:    now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "5 minutes ago",
			input:    now.Add(-5 * time.Minute),
			expected: "5m ago",
		},
		{
			name:     "2 hours ago",
			input:    now.Add(-2 * time.Hour),
			expected: "2h ago",
		},
		{
			name:     "3 days ago",
			input:    now.Add(-72 * time.Hour),
			expected: "3d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAge(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatLastBeat(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time returns dash",
			input:    time.Time{},
			expected: "–",
		},
		{
			name:     "just happened returns now",
			input:    now.Add(-500 * time.Millisecond),
			expected: "now",
		},
		{
			name:     "30 seconds ago",
			input:    now.Add(-30 * time.Second),
			expected: "30s",
		},
		{
			name:     "5 minutes ago",
			input:    now.Add(-5 * time.Minute),
			expected: "5m",
		},
		{
			name:     "2 hours ago",
			input:    now.Add(-2 * time.Hour),
			expected: "2h",
		},
		{
			name:     "3 days ago",
			input:    now.Add(-72 * time.Hour),
			expected: "3d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLastBeat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatStatus(t *testing.T) {
	// formatStatus is a passthrough function
	tests := []struct {
		input    string
		expected string
	}{
		{"running", "running"},
		{"stopped", "stopped"},
		{"paused", "paused"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderRainbowLogo(t *testing.T) {
	// Simple test to ensure it doesn't panic and returns colored output
	input := "Line 1\nLine 2\nLine 3"
	result := renderRainbowLogo(input)

	// Result should contain ANSI escape codes
	assert.NotEmpty(t, result)
	// Should still contain the original text
	assert.Contains(t, result, "Line 1")
	assert.Contains(t, result, "Line 2")
	assert.Contains(t, result, "Line 3")
}

func TestNewKeyMap(t *testing.T) {
	t.Run("stop enabled", func(t *testing.T) {
		km := newKeyMap(true)

		// Verify all key bindings are set
		assert.NotEmpty(t, km.Quit.Keys())
		assert.NotEmpty(t, km.Up.Keys())
		assert.NotEmpty(t, km.Down.Keys())
		assert.NotEmpty(t, km.Top.Keys())
		assert.NotEmpty(t, km.End.Keys())
		assert.NotEmpty(t, km.Stop.Keys())

		// Verify help text is set
		assert.Contains(t, km.Quit.Help().Key, "q")
		assert.Contains(t, km.Up.Help().Key, "k")
		assert.Contains(t, km.Down.Help().Key, "j")
		assert.Contains(t, km.Stop.Help().Key, "x")

		// Verify stop is enabled
		assert.True(t, km.Stop.Enabled())
	})

	t.Run("stop disabled", func(t *testing.T) {
		km := newKeyMap(false)

		// Verify stop is disabled
		assert.False(t, km.Stop.Enabled())
	})
}
