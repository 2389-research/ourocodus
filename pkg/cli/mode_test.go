package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeRich, "rich"},
		{ModePlain, "plain"},
		{ModeJSON, "json"},
		{Mode(99), "plain"}, // Unknown defaults to plain
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestMode_Is(t *testing.T) {
	assert.True(t, ModeRich.IsRich())
	assert.False(t, ModeRich.IsPlain())
	assert.False(t, ModeRich.IsJSON())

	assert.False(t, ModePlain.IsRich())
	assert.True(t, ModePlain.IsPlain())
	assert.False(t, ModePlain.IsJSON())

	assert.False(t, ModeJSON.IsRich())
	assert.False(t, ModeJSON.IsPlain())
	assert.True(t, ModeJSON.IsJSON())
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		expected Mode
		ok       bool
	}{
		{"rich", ModeRich, true},
		{"RICH", ModeRich, true},
		{"Rich", ModeRich, true},
		{"plain", ModePlain, true},
		{"PLAIN", ModePlain, true},
		{"json", ModeJSON, true},
		{"JSON", ModeJSON, true},
		{"", ModePlain, false},
		{"invalid", ModePlain, false},
		{"tui", ModePlain, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mode, ok := ParseMode(tt.input)
			assert.Equal(t, tt.expected, mode)
			assert.Equal(t, tt.ok, ok)
		})
	}
}
