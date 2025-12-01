// Package output provides mode-aware output interfaces for CLI applications.
package output

import (
	"io"
)

// Output provides a mode-aware interface for CLI output.
// Implementations adapt their behavior based on the current output mode
// (rich TUI, plain text, or JSON).
type Output interface {
	// Success prints a success message.
	Success(msg string)

	// Info prints an informational message.
	Info(msg string)

	// Warning prints a warning message.
	Warning(msg string)

	// Error prints an error message.
	Error(err error)

	// JSON outputs a value as JSON. Only produces output in JSON mode.
	// Returns an error if marshaling fails.
	JSON(v any) error

	// Progress returns a progress indicator (spinner in rich mode,
	// text updates in plain mode, silent in JSON mode).
	Progress(label string) Progress

	// Stdout returns the underlying stdout writer.
	Stdout() io.Writer

	// Stderr returns the underlying stderr writer.
	Stderr() io.Writer
}

// Progress represents a progress indicator that can be updated or completed.
type Progress interface {
	// Update changes the progress label.
	Update(label string)

	// Complete marks the progress as done.
	Complete()

	// Fail marks the progress as failed.
	Fail(err error)
}
