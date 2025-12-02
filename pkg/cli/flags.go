package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Flags holds the standard CLI flags that every ourocodus tool supports.
type Flags struct {
	// JSON outputs machine-readable JSON only.
	JSON bool
	// Plain outputs plain text without colors or TUI.
	Plain bool
	// Light enables the light color theme (default is dark).
	Light bool
	// NoColor disables ANSI colors (respects NO_COLOR standard).
	NoColor bool
	// Quiet suppresses informational output.
	Quiet bool
	// Verbose increases output verbosity.
	Verbose bool
}

// RegisterFlags adds the standard flags to a flag set.
// This is typically called on the root command's PersistentFlags().
func RegisterFlags(flags *pflag.FlagSet, f *Flags) {
	flags.BoolVar(&f.JSON, "json", false, "Output JSON only (machine-readable)")
	flags.BoolVar(&f.Plain, "plain", false, "Plain text output (no TUI, no colors)")
	flags.BoolVar(&f.Light, "light", false, "Use light theme (default is dark)")
	flags.BoolVar(&f.NoColor, "no-color", false, "Disable ANSI colors")
	flags.BoolVarP(&f.Quiet, "quiet", "q", false, "Suppress informational output")
	flags.BoolVarP(&f.Verbose, "verbose", "v", false, "Increase verbosity")
}

// RegisterPersistentFlags adds the standard flags as persistent flags on a cobra command.
// Persistent flags are inherited by all subcommands.
func RegisterPersistentFlags(cmd *cobra.Command, f *Flags) {
	RegisterFlags(cmd.PersistentFlags(), f)
}

// GetFlags retrieves the standard flags from a cobra command's context.
// Returns the Flags struct if found, or nil if not registered.
func GetFlags(cmd *cobra.Command) *Flags {
	f := &Flags{}

	// Try to get each flag, ignore errors for unset flags
	if val, err := cmd.Flags().GetBool("json"); err == nil {
		f.JSON = val
	}
	if val, err := cmd.Flags().GetBool("plain"); err == nil {
		f.Plain = val
	}
	if val, err := cmd.Flags().GetBool("light"); err == nil {
		f.Light = val
	}
	if val, err := cmd.Flags().GetBool("no-color"); err == nil {
		f.NoColor = val
	}
	if val, err := cmd.Flags().GetBool("quiet"); err == nil {
		f.Quiet = val
	}
	if val, err := cmd.Flags().GetBool("verbose"); err == nil {
		f.Verbose = val
	}

	return f
}
