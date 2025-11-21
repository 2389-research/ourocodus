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
	// When no flags or env vars, auto-detection kicks in
	// Result depends on TTY status and terminal size
	// During test execution (non-TTY), should return true (plain mode)
	result := ShouldUsePlainMode(false, false, os.Environ)
	// In non-TTY environment (like tests), expect plain mode
	if !IsTTY() {
		assert.True(t, result, "should use plain mode in non-TTY environment")
	} else {
		// In TTY, result depends on terminal size
		width, height := GetTerminalSize()
		if IsTerminalTooSmall(width, height) {
			assert.True(t, result, "should use plain mode for small terminal")
		} else {
			assert.False(t, result, "should use rich mode in adequately-sized TTY")
		}
	}
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

func TestSupportsUnicode(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		term     string
		lcAll    string
		lcCtype  string
		expected bool
	}{
		{
			name:     "UTF-8 lang",
			lang:     "en_US.UTF-8",
			term:     "xterm-256color",
			expected: true,
		},
		{
			name:     "UTF8 without dash",
			lang:     "en_US.UTF8",
			term:     "xterm",
			expected: true,
		},
		{
			name:     "non-UTF-8 lang",
			lang:     "C",
			term:     "xterm",
			expected: false,
		},
		{
			name:     "dumb terminal",
			lang:     "en_US.UTF-8",
			term:     "dumb",
			expected: false,
		},
		{
			name:     "linux terminal",
			lang:     "en_US.UTF-8",
			term:     "linux",
			expected: false,
		},
		{
			name:     "cons25 terminal",
			lang:     "en_US.UTF-8",
			term:     "cons25",
			expected: false,
		},
		{
			name:     "emacs terminal",
			lang:     "en_US.UTF-8",
			term:     "emacs",
			expected: false,
		},
		{
			name:     "LC_ALL overrides to non-UTF-8",
			lang:     "en_US.UTF-8",
			term:     "xterm",
			lcAll:    "C",
			expected: false,
		},
		{
			name:     "LC_ALL UTF-8",
			lang:     "C",
			term:     "xterm",
			lcAll:    "en_US.UTF-8",
			expected: true,
		},
		{
			name:     "LC_CTYPE overrides to non-UTF-8",
			lang:     "en_US.UTF-8",
			term:     "xterm",
			lcCtype:  "C",
			expected: false,
		},
		{
			name:     "LC_CTYPE UTF-8",
			lang:     "C",
			term:     "xterm",
			lcCtype:  "en_US.UTF-8",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			if tt.lang != "" {
				t.Setenv("LANG", tt.lang)
			}
			if tt.term != "" {
				t.Setenv("TERM", tt.term)
			}
			if tt.lcAll != "" {
				t.Setenv("LC_ALL", tt.lcAll)
			}
			if tt.lcCtype != "" {
				t.Setenv("LC_CTYPE", tt.lcCtype)
			}

			result := SupportsUnicode()
			assert.Equal(t, tt.expected, result, "unicode support detection mismatch")
		})
	}
}
