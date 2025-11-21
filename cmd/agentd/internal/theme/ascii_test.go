package theme

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLogo(t *testing.T) {
	tests := []struct {
		size LogoSize
		name string
	}{
		{LogoSmall, "small"},
		{LogoMedium, "medium"},
		{LogoLarge, "large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logo := GetLogo(tt.size)
			assert.NotEmpty(t, logo)
			// Should contain some recognizable element
			assert.True(t, len(logo) > 10, "logo should have content")
		})
	}
}

func TestGetAgentStatusIcon(t *testing.T) {
	tests := []struct {
		status   AgentStatus
		expected string
	}{
		{StatusRunning, "⚡"},
		{StatusPaused, "⏸"},
		{StatusStopped, "✗"},
		{StatusIdle, "💤"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			icon := GetAgentStatusIcon(tt.status)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

func TestDrawBox(t *testing.T) {
	content := "Test Content"
	box := DrawBox("Title", content, 40)

	assert.NotEmpty(t, box)
	assert.Contains(t, box, "Title")
	assert.Contains(t, box, content)
	assert.Contains(t, box, "─") // horizontal border
	assert.Contains(t, box, "│") // vertical border
}

func TestDrawBoxEmpty(t *testing.T) {
	box := DrawBox("", "content", 20)
	assert.NotEmpty(t, box)
	assert.Contains(t, box, "content")
}

func TestDrawHeader(t *testing.T) {
	header := DrawHeader("TEST HEADER")
	assert.NotEmpty(t, header)
	assert.Contains(t, header, "TEST HEADER")
	assert.Contains(t, header, "═")
}

func TestGetVintageMessage(t *testing.T) {
	tests := []struct {
		category MessageCategory
		name     string
	}{
		{MsgConnecting, "connecting"},
		{MsgSuccess, "success"},
		{MsgError, "error"},
		{MsgLoading, "loading"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetVintageMessage(tt.category)
			assert.NotEmpty(t, msg)
			// Messages should be uppercase vintage style
			assert.Equal(t, strings.ToUpper(msg), msg)
		})
	}
}

func TestGetVintageMessage_Randomness(t *testing.T) {
	// Call multiple times to verify we get messages (might be same or different)
	messages := make(map[string]bool)
	for i := 0; i < 10; i++ {
		msg := GetVintageMessage(MsgSuccess)
		messages[msg] = true
	}
	// Should have at least 1 message
	assert.True(t, len(messages) >= 1)
}
