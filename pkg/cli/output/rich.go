package output

import (
	"fmt"
	"io"
	"os"

	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// RichOutput provides styled terminal output using the theme system.
type RichOutput struct {
	stdout  io.Writer
	stderr  io.Writer
	theme   *theme.RetroTheme
	quiet   bool
	noColor bool

	// Cached styles
	successStyle lipgloss.Style
	infoStyle    lipgloss.Style
	warningStyle lipgloss.Style
	errorStyle   lipgloss.Style
}

// NewRichOutput creates a new rich output writer with the given theme.
func NewRichOutput(th *theme.RetroTheme, quiet, noColor bool) *RichOutput {
	r := &RichOutput{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		theme:   th,
		quiet:   quiet,
		noColor: noColor,
	}
	r.initStyles()
	return r
}

// NewRichOutputWithWriters creates a new rich output with custom writers.
func NewRichOutputWithWriters(stdout, stderr io.Writer, th *theme.RetroTheme, quiet, noColor bool) *RichOutput {
	r := &RichOutput{
		stdout:  stdout,
		stderr:  stderr,
		theme:   th,
		quiet:   quiet,
		noColor: noColor,
	}
	r.initStyles()
	return r
}

func (r *RichOutput) initStyles() {
	if r.noColor || r.theme == nil {
		// No styling when colors disabled
		r.successStyle = lipgloss.NewStyle()
		r.infoStyle = lipgloss.NewStyle()
		r.warningStyle = lipgloss.NewStyle()
		r.errorStyle = lipgloss.NewStyle()
		return
	}

	r.successStyle = lipgloss.NewStyle().Foreground(r.theme.Success)
	r.infoStyle = lipgloss.NewStyle().Foreground(r.theme.Primary)
	r.warningStyle = lipgloss.NewStyle().Foreground(r.theme.Warning)
	r.errorStyle = lipgloss.NewStyle().Foreground(r.theme.Error)
}

// Success prints a success message with styling.
func (r *RichOutput) Success(msg string) {
	if r.quiet {
		return
	}
	icon := r.successStyle.Render("✓")
	_, _ = fmt.Fprintln(r.stdout, icon, msg)
}

// Info prints an informational message with styling.
func (r *RichOutput) Info(msg string) {
	if r.quiet {
		return
	}
	_, _ = fmt.Fprintln(r.stdout, r.infoStyle.Render(msg))
}

// Warning prints a warning message with styling.
func (r *RichOutput) Warning(msg string) {
	icon := r.warningStyle.Render("⚠")
	_, _ = fmt.Fprintln(r.stderr, icon, msg)
}

// Error prints an error message with styling.
func (r *RichOutput) Error(err error) {
	if err == nil {
		return
	}
	icon := r.errorStyle.Render("✗")
	_, _ = fmt.Fprintln(r.stderr, icon, err.Error())
}

// JSON does nothing in rich mode (use plain or json mode for JSON output).
func (r *RichOutput) JSON(_ any) error {
	return nil
}

// Progress returns a styled progress indicator.
func (r *RichOutput) Progress(label string) Progress {
	return &richProgress{
		output: r,
		label:  label,
	}
}

// Stdout returns the stdout writer.
func (r *RichOutput) Stdout() io.Writer {
	return r.stdout
}

// Stderr returns the stderr writer.
func (r *RichOutput) Stderr() io.Writer {
	return r.stderr
}

// richProgress is a styled progress indicator for rich mode.
// Note: For full spinner support, use Bubble Tea components directly.
type richProgress struct {
	output *RichOutput
	label  string
}

func (p *richProgress) Update(label string) {
	if p.output.quiet {
		return
	}
	p.label = label
	_, _ = fmt.Fprintln(p.output.stdout, "⋯", label)
}

func (p *richProgress) Complete() {
	if p.output.quiet {
		return
	}
	icon := p.output.successStyle.Render("✓")
	_, _ = fmt.Fprintln(p.output.stdout, icon, p.label)
}

func (p *richProgress) Fail(err error) {
	icon := p.output.errorStyle.Render("✗")
	_, _ = fmt.Fprintln(p.output.stderr, icon, p.label+":", err)
}
