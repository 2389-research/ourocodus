package output

import (
	"encoding/json"
	"io"
	"os"
)

// JSONOutput provides JSON-only output for machine consumption.
type JSONOutput struct {
	stdout  io.Writer
	stderr  io.Writer
	encoder *json.Encoder
}

// NewJSONOutput creates a new JSON output writer.
func NewJSONOutput() *JSONOutput {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return &JSONOutput{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		encoder: encoder,
	}
}

// NewJSONOutputWithWriters creates a new JSON output with custom writers.
func NewJSONOutputWithWriters(stdout, stderr io.Writer) *JSONOutput {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return &JSONOutput{
		stdout:  stdout,
		stderr:  stderr,
		encoder: encoder,
	}
}

// Success does nothing in JSON mode (use JSON method instead).
func (j *JSONOutput) Success(_ string) {}

// Info does nothing in JSON mode (use JSON method instead).
func (j *JSONOutput) Info(_ string) {}

// Warning does nothing in JSON mode (errors should be in JSON output).
func (j *JSONOutput) Warning(_ string) {}

// Error does nothing in JSON mode (errors should be in JSON output).
func (j *JSONOutput) Error(_ error) {}

// JSON outputs a value as JSON.
func (j *JSONOutput) JSON(v any) error {
	return j.encoder.Encode(v)
}

// Progress returns a no-op progress indicator (silent in JSON mode).
func (j *JSONOutput) Progress(_ string) Progress {
	return &jsonProgress{}
}

// Stdout returns the stdout writer.
func (j *JSONOutput) Stdout() io.Writer {
	return j.stdout
}

// Stderr returns the stderr writer.
func (j *JSONOutput) Stderr() io.Writer {
	return j.stderr
}

// jsonProgress is a no-op progress indicator for JSON mode.
type jsonProgress struct{}

func (p *jsonProgress) Update(_ string) {}
func (p *jsonProgress) Complete()       {}
func (p *jsonProgress) Fail(_ error)    {}
