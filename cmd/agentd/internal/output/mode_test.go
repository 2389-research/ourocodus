package output

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
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		expected Mode
		valid    bool
	}{
		{"rich", ModeRich, true},
		{"plain", ModePlain, true},
		{"json", ModeJSON, true},
		{"RICH", ModeRich, true},  // case insensitive
		{"JSON", ModeJSON, true},
		{"invalid", ModePlain, false},
		{"", ModePlain, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, valid := ParseMode(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestDetectMode(t *testing.T) {
	tests := []struct {
		name       string
		jsonFlag   bool
		plainFlag  bool
		shouldPlain bool
		expected   Mode
	}{
		{"json flag", true, false, false, ModeJSON},
		{"plain flag", false, true, false, ModePlain},
		{"both flags - json wins", true, true, false, ModeJSON},
		{"env plain", false, false, true, ModePlain},
		{"default rich", false, false, false, ModeRich},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectMode(tt.jsonFlag, tt.plainFlag, tt.shouldPlain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMode_IsRich(t *testing.T) {
	assert.True(t, ModeRich.IsRich())
	assert.False(t, ModePlain.IsRich())
	assert.False(t, ModeJSON.IsRich())
}

func TestMode_IsPlain(t *testing.T) {
	assert.False(t, ModeRich.IsPlain())
	assert.True(t, ModePlain.IsPlain())
	assert.False(t, ModeJSON.IsPlain())
}

func TestMode_IsJSON(t *testing.T) {
	assert.False(t, ModeRich.IsJSON())
	assert.False(t, ModePlain.IsJSON())
	assert.True(t, ModeJSON.IsJSON())
}
