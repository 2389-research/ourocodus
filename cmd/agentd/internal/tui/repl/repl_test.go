package repl

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/stretchr/testify/assert"
)

func TestJoinLines(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		line     string
		expected string
	}{
		{
			name:     "empty existing",
			existing: "",
			line:     "new line",
			expected: "new line",
		},
		{
			name:     "whitespace only existing",
			existing: "   \n  ",
			line:     "new line",
			expected: "new line",
		},
		{
			name:     "existing with content",
			existing: "first line",
			line:     "second line",
			expected: "first line\nsecond line",
		},
		{
			name:     "existing with trailing whitespace",
			existing: "first line  \n  ",
			line:     "second line",
			expected: "first line\nsecond line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinLines(tt.existing, tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKeyMap(t *testing.T) {
	// Verify key bindings are properly configured
	km := keyMap{
		Send: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send")),
		Quit: key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("ctrl+c", "quit")),
		Raw:  key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "toggle raw")),
	}

	assert.NotEmpty(t, km.Send.Keys())
	assert.NotEmpty(t, km.Quit.Keys())
	assert.NotEmpty(t, km.Raw.Keys())

	// Verify help text
	assert.Equal(t, "send", km.Send.Help().Desc)
	assert.Equal(t, "quit", km.Quit.Help().Desc)
	assert.Equal(t, "toggle raw", km.Raw.Help().Desc)
}
