package cli

import (
	"github.com/2389-research/ourocodus/pkg/cli/detect"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
)

// Config holds the resolved configuration for a CLI application.
// This is computed from flags, environment variables, and auto-detection.
type Config struct {
	// Mode is the resolved output mode (rich, plain, json).
	Mode Mode
	// Theme is the resolved theme name.
	ThemeName string
	// NoColor indicates if colors should be disabled.
	NoColor bool
	// Quiet indicates if informational output should be suppressed.
	Quiet bool
	// Verbose indicates if verbose output is enabled.
	Verbose bool
	// Unicode indicates if unicode characters are supported.
	Unicode bool
}

// ResolveConfig computes the final configuration from flags and environment.
//
// Resolution priority for mode:
//  1. --json flag
//  2. --plain flag
//  3. OUROCODUS_OUTPUT environment variable
//  4. CI detection (forces plain)
//  5. TTY detection (non-TTY forces plain)
//  6. Terminal size (too small forces plain)
//  7. Default: rich mode
//
// Resolution priority for theme:
//  1. --theme flag
//  2. OUROCODUS_THEME environment variable
//  3. Default: "cga"
//
// Resolution priority for no-color:
//  1. --no-color flag
//  2. NO_COLOR environment variable
//  3. TERM=dumb
//  4. Default: colors enabled
func ResolveConfig(flags *Flags) Config {
	cfg := Config{
		Mode:      resolveMode(flags),
		ThemeName: resolveTheme(flags),
		NoColor:   resolveNoColor(flags),
		Quiet:     flags.Quiet,
		Verbose:   flags.Verbose,
		Unicode:   detect.SupportsUnicode(),
	}

	// JSON mode always implies no color
	if cfg.Mode == ModeJSON {
		cfg.NoColor = true
	}

	return cfg
}

// resolveMode determines the output mode from flags and environment.
func resolveMode(flags *Flags) Mode {
	// 1. Explicit flags take precedence
	if flags.JSON {
		return ModeJSON
	}
	if flags.Plain {
		return ModePlain
	}

	// 2. Check environment variable
	if envMode := detect.GetEnvMode(); envMode != "" {
		if mode, ok := ParseMode(envMode); ok {
			return mode
		}
	}

	// 3. Auto-detect based on environment
	if detect.ShouldUsePlainMode() {
		return ModePlain
	}

	// 4. Default to rich mode
	return ModeRich
}

// resolveTheme determines the theme from flags and environment.
func resolveTheme(flags *Flags) string {
	// 1. Explicit flag
	if flags.Theme != "" {
		return flags.Theme
	}

	// 2. Environment variable
	if envTheme := detect.GetEnvTheme(); envTheme != "" {
		return envTheme
	}

	// 3. Default to CGA
	return theme.PaletteCGA.String()
}

// resolveNoColor determines if colors should be disabled.
func resolveNoColor(flags *Flags) bool {
	// 1. Explicit flag
	if flags.NoColor {
		return true
	}

	// 2. Environment (NO_COLOR standard, TERM=dumb)
	return detect.NoColor()
}

// GetTheme returns the theme.RetroTheme for the config.
// Returns nil if the theme name is invalid.
func (c Config) GetTheme() *theme.RetroTheme {
	palette, ok := theme.ParsePaletteName(c.ThemeName)
	if !ok {
		palette = theme.PaletteCGA
	}
	return theme.NewRetroTheme(palette)
}
