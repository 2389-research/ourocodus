// Command build provides a TUI-based build experience for the ourocodus project.
// It wraps the standard build process with visual progress indicators.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/tui/components/header"
	"github.com/2389-research/ourocodus/pkg/tui/components/progress"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// knownCommands is a whitelist of commands that are safe to execute.
// This prevents command injection via untrusted input.
var knownCommands = map[string]bool{
	"make": true,
	"go":   true,
}

// buildStep defines a build step with its command.
type buildStep struct {
	name string
	cmd  string
	args []string
}

// Build steps in order.
var buildSteps = []buildStep{
	{"Compiling web assets", "make", []string{"assets"}},
	{"Building relay", "go", []string{"build", "-o", "bin/relay", "./cmd/relay"}},
	{"Building echo-agent", "go", []string{"build", "-o", "bin/echo-agent", "./cmd/echo-agent"}},
	{"Building agentd", "go", []string{"build", "-o", "bin/agentd", "./cmd/agentd"}},
	{"Building build", "go", []string{"build", "-o", "bin/build", "./cmd/build"}},
}

// Message types.
type (
	stepDoneMsg  struct{ index int }
	stepErrorMsg struct {
		index int
		err   string
	}
	buildDoneMsg struct{ duration time.Duration }
)

type model struct {
	th       *theme.Theme
	progress *progress.Model
	spinner  spinner.Model

	width, height int
	ready         bool
	quitting      bool
	done          bool
	failed        bool
	startTime     time.Time
	duration      time.Duration
	currentStep   int
	currentErr    string
}

func newModel() model {
	th := theme.Default()

	// Extract step names for progress component
	stepNames := make([]string, len(buildSteps))
	for i, s := range buildSteps {
		stepNames[i] = s.name
	}

	prog := progress.New(th)
	prog.SetSteps(stepNames)
	prog.ShowProgressBar(true)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = th.PrimaryText

	return model{
		th:          th,
		progress:    &prog,
		spinner:     s,
		startTime:   time.Now(),
		currentStep: -1,
	}
}

func (m model) Init() tea.Cmd {
	// Ensure bin directory exists
	_ = os.MkdirAll("bin", 0o750)

	return tea.Batch(
		m.spinner.Tick,
		m.runNextStep(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.progress.SetWidth(msg.Width - 4)

	case stepDoneMsg:
		m.progress.CompleteStep()
		m.currentStep = msg.index

		// Move to next step or finish
		if msg.index+1 < len(buildSteps) {
			cmds = append(cmds, m.runStep(msg.index+1))
		} else {
			m.done = true
			m.duration = time.Since(m.startTime)
			return m, tea.Quit
		}

	case stepErrorMsg:
		m.progress.FailStep(msg.err)
		m.currentErr = msg.err
		m.failed = true
		m.done = true
		m.duration = time.Since(m.startTime)

	case buildDoneMsg:
		m.done = true
		m.duration = msg.duration

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		// Also update progress spinner
		updated, progCmd := m.progress.Update(msg)
		m.progress = &updated
		cmds = append(cmds, progCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.quitting && !m.done {
		return m.th.WarningText.Render("\nBuild cancelled.\n")
	}

	var b strings.Builder

	// Header
	var statusContent string
	switch {
	case m.failed:
		statusContent = m.th.ErrorText.Render(fmt.Sprintf("Build failed after %s", m.duration.Round(time.Millisecond)))
	case m.done:
		statusContent = m.th.SuccessText.Render(fmt.Sprintf("Build complete in %s", m.duration.Round(time.Millisecond)))
	default:
		statusContent = m.th.Title.Render("Building ourocodus")
	}
	b.WriteString(header.RenderWithContent(m.th, statusContent))
	b.WriteString("\n")

	// Progress steps
	b.WriteString(m.progress.View())
	b.WriteString("\n")

	// Error display
	if m.currentErr != "" {
		b.WriteString("\n")
		// Wrap long error messages
		width := m.width
		if width == 0 {
			width = 80
		}
		errLines := wrapText(m.currentErr, width-4)
		for _, line := range errLines {
			b.WriteString(m.th.ErrorText.Render("  " + line))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// runNextStep starts the first step.
func (m model) runNextStep() tea.Cmd {
	return m.runStep(0)
}

// runStep executes a build step and returns a command.
func (m model) runStep(index int) tea.Cmd {
	if index >= len(buildSteps) {
		return func() tea.Msg {
			return buildDoneMsg{duration: time.Since(m.startTime)}
		}
	}

	step := buildSteps[index]
	m.progress.StartStep(index)

	return func() tea.Msg {
		// Validate command is in whitelist to prevent command injection
		if !knownCommands[step.cmd] {
			return stepErrorMsg{index: index, err: "unknown command: " + step.cmd}
		}

		cmd := exec.Command(step.cmd, step.args...) //nolint:gosec // command is validated against knownCommands whitelist
		cmd.Dir = "."

		output, err := cmd.CombinedOutput()
		if err != nil {
			errMsg := err.Error()
			if len(output) > 0 {
				// Get last few lines of output for context
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				if len(lines) > 5 {
					lines = lines[len(lines)-5:]
				}
				errMsg = strings.Join(lines, "\n")
			}
			return stepErrorMsg{index: index, err: errMsg}
		}
		return stepDoneMsg{index: index}
	}
}

// wrapText wraps text to fit within width.
func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if len(line) <= width {
			lines = append(lines, line)
			continue
		}
		// Simple word wrap
		words := strings.Fields(line)
		current := ""
		for _, word := range words {
			if len(current)+len(word)+1 > width {
				if current != "" {
					lines = append(lines, current)
				}
				current = word
			} else {
				if current != "" {
					current += " "
				}
				current += word
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}

func main() {
	// Check we're in the right directory
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		fmt.Println("Error: must be run from project root (where go.mod exists)")
		os.Exit(1)
	}

	p := tea.NewProgram(newModel())
	if m, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	} else if model, ok := m.(model); ok && model.failed {
		os.Exit(1)
	}
}
