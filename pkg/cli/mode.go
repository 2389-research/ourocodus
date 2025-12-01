// Package cli provides a standard CLI/TUI framework for ourocodus tools.
//
// The framework ensures consistent behavior, flags, and user experience across
// all tools including agentd, relay, and future applications.
//
// Features:
//   - Smart defaults: Rich TUI when interactive, plain text in CI/pipes, JSON when requested
//   - Standard flags: --json, --plain, --theme, --no-color, --quiet, --verbose
//   - Environment support: NO_COLOR, OUROCODUS_THEME, OUROCODUS_OUTPUT, CI
//   - Consistent exit codes across all tools
//
// Usage:
//
//	app := cli.NewApp(rootCmd)
//	os.Exit(app.Execute())
//
// Commands access configuration through AppContext:
//
//	ctx := cli.FromContext(cmd.Context())
//	if ctx.Mode.IsRich() {
//	    // Run TUI
//	} else {
//	    // Run plain/JSON output
//	}
package cli

import (
	"strings"
)

// Mode represents the output mode for CLI commands.
type Mode int

const (
	// ModeRich uses full TUI with colors and interactivity.
	ModeRich Mode = iota
	// ModePlain uses plain text output without colors.
	ModePlain
	// ModeJSON outputs machine-readable JSON.
	ModeJSON
)

// String returns the string representation of the mode.
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

// IsRich returns true if this is rich/TUI mode.
func (m Mode) IsRich() bool {
	return m == ModeRich
}

// IsPlain returns true if this is plain text mode.
func (m Mode) IsPlain() bool {
	return m == ModePlain
}

// IsJSON returns true if this is JSON mode.
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
