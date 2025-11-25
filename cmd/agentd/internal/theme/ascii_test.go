package theme

import (
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

func TestGetAgentStatusIcon_Unicode(t *testing.T) {
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
			icon := GetAgentStatusIcon(tt.status, true)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

func TestGetAgentStatusIcon_ASCII(t *testing.T) {
	tests := []struct {
		status   AgentStatus
		expected string
	}{
		{StatusRunning, ">"},
		{StatusPaused, "||"},
		{StatusStopped, "X"},
		{StatusIdle, "~"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			icon := GetAgentStatusIcon(tt.status, false)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

func TestGetAgentStatusIcon_UnknownStatus(t *testing.T) {
	unknown := AgentStatus("unknown")

	// Both modes should return "?" for unknown status
	assert.Equal(t, "?", GetAgentStatusIcon(unknown, true))
	assert.Equal(t, "?", GetAgentStatusIcon(unknown, false))
}
