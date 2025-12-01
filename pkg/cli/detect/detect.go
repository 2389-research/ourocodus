// Package detect provides terminal and environment detection utilities.
package detect

import (
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	// MinTerminalWidth is the minimum width for rich TUI mode.
	MinTerminalWidth = 80
	// MinTerminalHeight is the minimum height for rich TUI mode.
	MinTerminalHeight = 24
)

// IsTTY checks if stdout is a terminal (not piped/redirected).
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// IsStdinTTY checks if stdin is a terminal.
func IsStdinTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// IsStderrTTY checks if stderr is a terminal.
func IsStderrTTY() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// GetTerminalSize returns the current terminal width and height.
// Returns (0, 0) if unable to determine.
func GetTerminalSize() (width int, height int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, 0
	}
	return w, h
}

// IsTerminalTooSmall checks if terminal is smaller than minimum requirements.
func IsTerminalTooSmall(width, height int) bool {
	return width < MinTerminalWidth || height < MinTerminalHeight
}

// IsCI detects if running in a CI environment.
func IsCI() bool {
	// Check common CI environment variables
	ciVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"TRAVIS",
		"JENKINS_URL",
		"BUILDKITE",
		"DRONE",
	}

	for _, v := range ciVars {
		if val := os.Getenv(v); val != "" && val != "false" && val != "0" {
			return true
		}
	}
	return false
}

// NoColor checks if colors should be disabled.
// Respects the NO_COLOR standard (https://no-color.org/).
func NoColor() bool {
	// NO_COLOR takes precedence - any non-empty value disables color
	if os.Getenv("NO_COLOR") != "" {
		return true
	}

	// Also check for TERM=dumb
	if os.Getenv("TERM") == "dumb" {
		return true
	}

	return false
}

// SupportsUnicode checks if the terminal supports Unicode/UTF-8 rendering.
// Returns true if unicode emoji and box-drawing characters should work.
func SupportsUnicode() bool {
	// Check TERM for known non-unicode terminals first
	termVal := os.Getenv("TERM")
	switch termVal {
	case "dumb", "linux", "cons25", "emacs":
		return false
	}

	// Check locale settings in priority order: LC_ALL > LC_CTYPE > LANG
	// LC_ALL overrides everything
	if lcAll := os.Getenv("LC_ALL"); lcAll != "" {
		return containsUTF8(lcAll)
	}

	// LC_CTYPE overrides LANG for character type
	if lcCtype := os.Getenv("LC_CTYPE"); lcCtype != "" {
		return containsUTF8(lcCtype)
	}

	// Finally check LANG
	lang := os.Getenv("LANG")
	return containsUTF8(lang)
}

// containsUTF8 checks if a locale string indicates UTF-8 encoding.
func containsUTF8(s string) bool {
	upper := strings.ToUpper(s)
	return strings.Contains(upper, "UTF-8") || strings.Contains(upper, "UTF8")
}

// ShouldUsePlainMode determines if plain mode should be used based on environment.
// This does NOT check flags - those should be checked separately with higher priority.
func ShouldUsePlainMode() bool {
	// CI environments should use plain mode
	if IsCI() {
		return true
	}

	// Non-TTY (pipes) should use plain mode
	if !IsTTY() {
		return true
	}

	// Small terminals should use plain mode
	width, height := GetTerminalSize()
	if IsTerminalTooSmall(width, height) {
		return true
	}

	// NO_COLOR doesn't force plain mode, it just disables colors
	// But TERM=dumb typically means limited terminal capabilities
	if os.Getenv("TERM") == "dumb" {
		return true
	}

	return false
}

// GetEnvMode returns the mode from OUROCODUS_OUTPUT environment variable.
// Returns empty string if not set.
func GetEnvMode() string {
	return os.Getenv("OUROCODUS_OUTPUT")
}

// GetEnvTheme returns the theme from OUROCODUS_THEME environment variable.
// Returns empty string if not set.
func GetEnvTheme() string {
	return os.Getenv("OUROCODUS_THEME")
}
