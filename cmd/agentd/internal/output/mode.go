package output

import (
	"strings"
)

// Mode represents the output mode for CLI commands
type Mode int

const (
	ModeRich Mode = iota
	ModePlain
	ModeJSON
)

// String returns the string representation of the mode
func (m Mode) String() string {
	switch m {
	case ModeRich:
		return "rich"
	case ModePlain:
		return "plain"
	case ModeJSON:
		return "json"
	default:
		return "plain"
	}
}

// IsRich returns true if this is rich/TUI mode
func (m Mode) IsRich() bool {
	return m == ModeRich
}

// IsPlain returns true if this is plain text mode
func (m Mode) IsPlain() bool {
	return m == ModePlain
}

// IsJSON returns true if this is JSON mode
func (m Mode) IsJSON() bool {
	return m == ModeJSON
}

// ParseMode parses a mode string (case-insensitive).
// Returns the mode and true if valid, or ModePlain and false if invalid.
func ParseMode(s string) (Mode, bool) {
	switch strings.ToLower(s) {
	case "rich":
		return ModeRich, true
	case "plain":
		return ModePlain, true
	case "json":
		return ModeJSON, true
	default:
		return ModePlain, false
	}
}

// DetectMode determines the output mode based on flags and environment.
// Priority: --json > --plain > environment detection > rich mode
func DetectMode(jsonFlag bool, plainFlag bool, shouldUsePlain bool) Mode {
	if jsonFlag {
		return ModeJSON
	}
	if plainFlag {
		return ModePlain
	}
	if shouldUsePlain {
		return ModePlain
	}
	return ModeRich
}
