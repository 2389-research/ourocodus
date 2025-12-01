package cli

import (
	"context"

	"github.com/2389-research/ourocodus/pkg/cli/output"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
)

// contextKey is used to store AppContext in context.Context.
type contextKey struct{}

// AppContext provides access to CLI configuration and utilities within commands.
// It is propagated through cmd.Context() to all subcommands.
type AppContext struct {
	// Mode is the resolved output mode (rich, plain, json).
	Mode Mode

	// Theme is the resolved theme (nil in JSON mode).
	Theme *theme.RetroTheme

	// Output provides mode-aware output methods.
	Output output.Output

	// Config holds the full resolved configuration.
	Config Config

	// Cancel can be called to trigger graceful shutdown.
	Cancel context.CancelFunc
}

// NewAppContext creates an AppContext from configuration.
func NewAppContext(cfg Config, cancel context.CancelFunc) *AppContext {
	ctx := &AppContext{
		Mode:   cfg.Mode,
		Config: cfg,
		Cancel: cancel,
	}

	// Create theme (not needed for JSON mode)
	if cfg.Mode != ModeJSON {
		ctx.Theme = cfg.GetTheme()
	}

	// Create mode-appropriate output
	ctx.Output = createOutput(cfg, ctx.Theme)

	return ctx
}

// createOutput creates the appropriate Output implementation for the mode.
func createOutput(cfg Config, th *theme.RetroTheme) output.Output {
	switch cfg.Mode {
	case ModeJSON:
		return output.NewJSONOutput()
	case ModePlain:
		return output.NewPlainOutput(cfg.Quiet)
	default:
		return output.NewRichOutput(th, cfg.Quiet, cfg.NoColor)
	}
}

// ToContext stores the AppContext in a context.Context.
func (a *AppContext) ToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, a)
}

// FromContext retrieves the AppContext from a context.Context.
// Returns nil if no AppContext is stored.
func FromContext(ctx context.Context) *AppContext {
	if ctx == nil {
		return nil
	}
	if appCtx, ok := ctx.Value(contextKey{}).(*AppContext); ok {
		return appCtx
	}
	return nil
}

// MustFromContext retrieves the AppContext from a context.Context.
// Panics if no AppContext is stored.
func MustFromContext(ctx context.Context) *AppContext {
	appCtx := FromContext(ctx)
	if appCtx == nil {
		panic("cli.AppContext not found in context - ensure cli.App is wrapping your command")
	}
	return appCtx
}
