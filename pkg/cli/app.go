package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// App wraps a cobra command with standard CLI behavior.
// It handles:
//   - Standard flag registration
//   - Signal handling (SIGINT, SIGTERM)
//   - Mode detection and configuration resolution
//   - AppContext propagation to all subcommands
//   - Consistent exit codes
type App struct {
	root  *cobra.Command
	flags Flags
}

// NewApp creates a new CLI application wrapping the given root command.
// Standard flags (--json, --plain, --theme, etc.) are automatically registered.
func NewApp(root *cobra.Command) *App {
	app := &App{
		root: root,
	}

	// Register standard flags on the root command
	RegisterPersistentFlags(root, &app.flags)

	return app
}

// Execute runs the CLI application and returns an exit code.
// This should be called from main() like: os.Exit(app.Execute())
func (a *App) Execute() int {
	// Create cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	// Wrap execution to resolve config and inject context
	a.root.PersistentPreRunE = a.chainPreRun(a.root.PersistentPreRunE)

	// Execute with our context
	a.root.SetContext(ctx)
	err := a.root.Execute()

	// Check if we were interrupted
	select {
	case <-ctx.Done():
		if ctx.Err() == context.Canceled {
			return ExitInterrupted
		}
	default:
	}

	return ExitCodeFromError(err)
}

// chainPreRun chains our config resolution with any existing PreRunE.
func (a *App) chainPreRun(existing func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Get current context
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Create cancellable context
		ctx, cancel := context.WithCancel(ctx)

		// Resolve configuration from flags and environment
		cfg := ResolveConfig(&a.flags)

		// Create and inject AppContext
		appCtx := NewAppContext(cfg, cancel)
		cmd.SetContext(appCtx.ToContext(ctx))

		// Call existing PreRunE if any
		if existing != nil {
			return existing(cmd, args)
		}
		return nil
	}
}

// Root returns the underlying root cobra command.
// This is useful for adding subcommands or customizing behavior.
func (a *App) Root() *cobra.Command {
	return a.root
}

// Flags returns a pointer to the standard flags.
// This is useful for commands that need direct flag access.
func (a *App) Flags() *Flags {
	return &a.flags
}
