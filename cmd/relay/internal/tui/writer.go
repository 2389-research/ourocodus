package tui

import (
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// LogWriter is an io.Writer that sends log entries to the TUI.
type LogWriter struct {
	program *tea.Program
	mu      sync.Mutex
	buffer  strings.Builder
}

// NewLogWriter creates a new log writer.
func NewLogWriter(p *tea.Program) *LogWriter {
	return &LogWriter{
		program: p,
	}
}

// Write implements io.Writer.
func (w *LogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Accumulate partial lines
	w.buffer.Write(p)

	// Process complete lines
	content := w.buffer.String()
	lines := strings.Split(content, "\n")

	// Keep the last incomplete line in buffer
	if len(lines) > 0 {
		w.buffer.Reset()
		lastLine := lines[len(lines)-1]
		if lastLine != "" {
			w.buffer.WriteString(lastLine)
		}
		lines = lines[:len(lines)-1]
	}

	// Send complete lines to TUI
	for _, line := range lines {
		if line == "" {
			continue
		}
		entry := ParseLogLine(line)
		entry.Time = time.Now()
		if w.program != nil {
			w.program.Send(LogMsg(entry))
		}
	}

	return len(p), nil
}

// Flush sends any remaining buffered content.
func (w *LogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buffer.Len() > 0 {
		entry := ParseLogLine(w.buffer.String())
		entry.Time = time.Now()
		if w.program != nil {
			w.program.Send(LogMsg(entry))
		}
		w.buffer.Reset()
	}
}

// SetProgram sets the tea.Program after it's created.
func (w *LogWriter) SetProgram(p *tea.Program) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.program = p
}
