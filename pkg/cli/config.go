package cli

import (
	"fmt"

	"github.com/2389-research/ourocodus/pkg/cli/detect"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
)

// Config holds the resolved configuration for a CLI application.
// This is computed from flags, environment variables, and auto-detection.
type Config struct {
	// Mode is the resolved output mode (rich, plain, json).
	Mode Mode
	// ThemeMode is the resolved theme mode (dark or light).
	ThemeMode theme.ThemeMode
	// NoColor indicates if colors should be disabled.
	NoColor bool
	// Quiet indicates if informational output should be suppressed.
	Quiet bool
	// Verbose indicates if verbose output is enabled.
	Verbose bool
	// Unicode indicates if unicode characters are supported.
	Unicode bool
}

// ValidateFlags checks for incompatible flag combinations and returns an error if any are found.
// This function should be called before ResolveConfig to ensure flag combinations are valid.
//
// Detected conflicts:
//   - --json and --plain are mutually exclusive (both set output mode)
//   - --quiet and --verbose are mutually exclusive (contradictory)
func ValidateFlags(f *Flags) error {
	// Check for mutually exclusive output mode flags
	if f.JSON && f.Plain {
		return fmt.Errorf("flags --json and --plain are mutually exclusive")
	}

	// Check for contradictory verbosity flags
	if f.Quiet && f.Verbose {
		return fmt.Errorf("flags --quiet and --verbose are mutually exclusive")
	}

	return nil
}

// ResolveConfig computes the final configuration from flags and environment.
//
// Note: This function does not validate flag conflicts. Callers should call
// ValidateFlags() first to ensure flag combinations are valid.
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
//  1. --light flag (selects light theme)
//  2. OUROCODUS_THEME environment variable ("light" or "dark")
//  3. Default: dark theme
//
// Resolution priority for no-color:
//  1. --no-color flag
//  2. NO_COLOR environment variable
//  3. TERM=dumb
//  4. Default: colors enabled
func ResolveConfig(flags *Flags) Config {
	cfg := Config{
		Mode:      resolveMode(flags),
		ThemeMode: resolveThemeMode(flags),
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

// resolveThemeMode determines the theme mode from flags and environment.
func resolveThemeMode(flags *Flags) theme.ThemeMode {
	// 1. Explicit --light flag
	if flags.Light {
		return theme.ThemeLight
	}

	// 2. Check environment variable
	if envTheme := detect.GetEnvTheme(); envTheme != "" {
		if envTheme == "light" {
			return theme.ThemeLight
		}
	}

	// 3. Default to dark theme
	return theme.ThemeDark
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

// GetTheme returns the theme.Theme for the config.
func (c Config) GetTheme() *theme.Theme {
	return theme.New(c.ThemeMode)
}
