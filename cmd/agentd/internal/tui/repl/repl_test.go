package repl

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/stretchr/testify/assert"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			width:    20,
			expected: "hello",
		},
		{
			name:     "exact width",
			input:    "hello",
			width:    8,
			expected: "hello",
		},
		{
			name:     "needs truncation",
			input:    "hello world this is long",
			width:    15,
			expected: "hello world ...",
		},
		{
			name:     "very small width",
			input:    "hello",
			width:    3,
			expected: "hello", // Don't truncate if width too small
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.width)
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
		Up:   key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "prev msg")),
		Down: key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next msg")),
	}

	assert.NotEmpty(t, km.Send.Keys())
	assert.NotEmpty(t, km.Quit.Keys())
	assert.NotEmpty(t, km.Raw.Keys())
	assert.NotEmpty(t, km.Up.Keys())
	assert.NotEmpty(t, km.Down.Keys())

	// Verify help text
	assert.Equal(t, "send", km.Send.Help().Desc)
	assert.Equal(t, "quit", km.Quit.Help().Desc)
	assert.Equal(t, "toggle raw", km.Raw.Help().Desc)
	assert.Equal(t, "prev msg", km.Up.Help().Desc)
	assert.Equal(t, "next msg", km.Down.Help().Desc)
}

func TestMessage(t *testing.T) {
	// Test message struct
	msg := message{
		display: "You: hello",
		json:    `{"id":1,"jsonrpc":"2.0","method":"send_message"}`,
		color:   "#FF0000",
	}

	assert.Equal(t, "You: hello", msg.display)
	assert.Contains(t, msg.json, "send_message")
	assert.Equal(t, "#FF0000", string(msg.color))
}

func TestTruncateWithANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		wantLen  int  // Expected visible length (excluding ANSI codes)
		hasReset bool // Should end with reset sequence if truncated
	}{
		{
			name:    "plain text no truncation",
			input:   "hello",
			width:   20,
			wantLen: 5,
		},
		{
			name:     "plain text truncated",
			input:    "hello world this is long",
			width:    15,
			wantLen:  15, // 12 chars + "..."
			hasReset: true,
		},
		{
			name:    "with ANSI codes no truncation",
			input:   "\x1b[31mred\x1b[0m",
			width:   20,
			wantLen: 3, // Only "red" is visible
		},
		{
			name:     "with ANSI codes truncated",
			input:    "\x1b[31mhello world this is very long\x1b[0m",
			width:    15,
			wantLen:  15, // 12 visible + "..."
			hasReset: true,
		},
		{
			name:    "very small width",
			input:   "\x1b[31mhello\x1b[0m",
			width:   3,
			wantLen: 5, // Returns as-is when width too small
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateWithANSI(tt.input, tt.width)

			// Count visible characters
			visibleLen := 0
			inEscape := false
			for i := 0; i < len(result); i++ {
				c := result[i]
				if c == '\x1b' && i+1 < len(result) && result[i+1] == '[' {
					inEscape = true
					continue
				}
				if inEscape {
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
						inEscape = false
					}
					continue
				}
				visibleLen++
			}

			assert.Equal(t, tt.wantLen, visibleLen, "visible length mismatch")

			if tt.hasReset {
				assert.Contains(t, result, "\x1b[0m", "should contain reset sequence")
				assert.Contains(t, result, "...", "should contain ellipsis")
			}
		})
	}
}
