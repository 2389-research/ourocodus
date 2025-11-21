package detect

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTTY(t *testing.T) {
	// IsTTY should return true for actual terminal, false for pipes
	// This test is environment-dependent, so we mainly verify it doesn't panic
	result := IsTTY()
	assert.IsType(t, false, result)
}

func TestShouldUsePlainMode_JSONFlag(t *testing.T) {
	assert.True(t, ShouldUsePlainMode(true, false, nil))
}

func TestShouldUsePlainMode_PlainFlag(t *testing.T) {
	assert.True(t, ShouldUsePlainMode(false, true, nil))
}

func TestShouldUsePlainMode_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	assert.True(t, ShouldUsePlainMode(false, false, os.Environ))
}

func TestShouldUsePlainMode_AgentdPlain(t *testing.T) {
	t.Setenv("AGENTD_PLAIN", "1")
	assert.True(t, ShouldUsePlainMode(false, false, os.Environ))
}

func TestShouldUsePlainMode_CI(t *testing.T) {
	t.Setenv("CI", "true")
	assert.True(t, ShouldUsePlainMode(false, false, os.Environ))
}

func TestShouldUsePlainMode_Default(t *testing.T) {
	// Clear environment
	os.Clearenv()
	// When no flags or env vars, should return false (use rich mode)
	assert.False(t, ShouldUsePlainMode(false, false, os.Environ))
}

func TestGetTerminalSize(t *testing.T) {
	width, height := GetTerminalSize()
	// Should return sensible defaults or actual terminal size
	assert.True(t, width >= 0, "width should be non-negative")
	assert.True(t, height >= 0, "height should be non-negative")
}

func TestIsTerminalTooSmall(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		expected bool
	}{
		{"large enough", 80, 24, false},
		{"too narrow", 79, 24, true},
		{"too short", 80, 23, true},
		{"both too small", 60, 20, true},
		{"larger than minimum", 100, 30, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTerminalTooSmall(tt.width, tt.height)
			assert.Equal(t, tt.expected, result)
		})
	}
}
