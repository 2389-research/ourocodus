package output

import (
	"fmt"
	"io"
	"os"
)

// PlainOutput provides plain text output without colors or TUI.
type PlainOutput struct {
	stdout io.Writer
	stderr io.Writer
	quiet  bool
}

// NewPlainOutput creates a new plain output writer.
func NewPlainOutput(quiet bool) *PlainOutput {
	return &PlainOutput{
		stdout: os.Stdout,
		stderr: os.Stderr,
		quiet:  quiet,
	}
}

// NewPlainOutputWithWriters creates a new plain output with custom writers.
func NewPlainOutputWithWriters(stdout, stderr io.Writer, quiet bool) *PlainOutput {
	return &PlainOutput{
		stdout: stdout,
		stderr: stderr,
		quiet:  quiet,
	}
}

// Success prints a success message.
func (p *PlainOutput) Success(msg string) {
	if p.quiet {
		return
	}
	_, _ = fmt.Fprintln(p.stdout, "✓", msg)
}

// Info prints an informational message.
func (p *PlainOutput) Info(msg string) {
	if p.quiet {
		return
	}
	_, _ = fmt.Fprintln(p.stdout, msg)
}

// Warning prints a warning message.
func (p *PlainOutput) Warning(msg string) {
	_, _ = fmt.Fprintln(p.stderr, "warning:", msg)
}

// Error prints an error message.
func (p *PlainOutput) Error(err error) {
	if err == nil {
		return
	}
	_, _ = fmt.Fprintln(p.stderr, "error:", err.Error())
}

// JSON does nothing in plain mode.
func (p *PlainOutput) JSON(_ any) error {
	return nil
}

// Progress returns a plain text progress indicator.
func (p *PlainOutput) Progress(label string) Progress {
	return &plainProgress{
		output: p,
		label:  label,
	}
}

// Stdout returns the stdout writer.
func (p *PlainOutput) Stdout() io.Writer {
	return p.stdout
}

// Stderr returns the stderr writer.
func (p *PlainOutput) Stderr() io.Writer {
	return p.stderr
}

// plainProgress is a simple text-based progress indicator.
type plainProgress struct {
	output *PlainOutput
	label  string
}

func (p *plainProgress) Update(label string) {
	if p.output.quiet {
		return
	}
	p.label = label
	_, _ = fmt.Fprintln(p.output.stdout, "...", label)
}

func (p *plainProgress) Complete() {
	if p.output.quiet {
		return
	}
	_, _ = fmt.Fprintln(p.output.stdout, "✓", p.label, "done")
}

func (p *plainProgress) Fail(err error) {
	_, _ = fmt.Fprintln(p.output.stderr, "✗", p.label, "failed:", err)
}
