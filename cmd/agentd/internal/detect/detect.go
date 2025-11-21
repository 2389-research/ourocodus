package detect

import (
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	minTerminalWidth  = 80
	minTerminalHeight = 24
)

// IsTTY checks if stdout is a terminal (not piped/redirected).
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// ShouldUsePlainMode determines if plain mode should be used based on flags and environment.
// Priority: --json > --plain > environment detection > auto-detect TTY > terminal size
func ShouldUsePlainMode(jsonMode bool, plainMode bool, getenv func() []string) bool {
	// Explicit flags take precedence
	if jsonMode || plainMode {
		return true
	}

	// Check environment variables
	for _, env := range getenv() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := parts[1]

		switch key {
		case "NO_COLOR", "AGENTD_PLAIN":
			if val != "" {
				return true
			}
		case "CI":
			if val == "true" || val == "1" {
				return true
			}
		}
	}

	// Auto-detect: if not a TTY, use plain mode
	if !IsTTY() {
		return true
	}

	// Check terminal size
	width, height := GetTerminalSize()
	return IsTerminalTooSmall(width, height)
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
	return width < minTerminalWidth || height < minTerminalHeight
}
