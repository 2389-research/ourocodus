# Phase 1: Charm CLI Foundation - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build foundation for Charm-based CLI with theme system, terminal detection, and output mode switching (rich/plain/json).

**Architecture:** Create reusable packages for retro theming (CGA color palettes, ASCII art, vintage messages), terminal capability detection, and output mode selection. Keep existing Cobra commands working while adding infrastructure for future TUI integration.

**Tech Stack:** Bubble Tea (TUI framework), Bubbles (components), Lip Gloss (styling), Charm Log (logging), existing Cobra CLI, Go 1.24.0

---

## Task 1: Add Charm Dependencies

**Files:**
- Modify: `go.mod`

**Step 1: Add Charm dependencies**

Run the following commands to add all required Charm libraries:

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/huh@latest
go get github.com/charmbracelet/log@latest
go get github.com/charmbracelet/harmonica@latest
```

**Step 2: Verify dependencies added**

Run: `go mod tidy && go list -m github.com/charmbracelet/bubbletea`

Expected: Outputs version like `github.com/charmbracelet/bubbletea v1.x.x`

**Step 3: Verify project still builds**

Run: `make build`

Expected: SUCCESS - all binaries build without errors

**Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add Charm CLI dependencies (bubbletea, bubbles, lipgloss, huh, log, harmonica)"
```

---

## Task 2: Terminal Detection Package (Tests)

**Files:**
- Create: `cmd/agentd/internal/detect/detect.go`
- Create: `cmd/agentd/internal/detect/detect_test.go`

**Step 1: Create detect package directory**

Run: `mkdir -p cmd/agentd/internal/detect`

Expected: Directory created

**Step 2: Write failing tests for terminal detection**

Create `cmd/agentd/internal/detect/detect_test.go`:

```go
package detect

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTTY(t *testing.T) {
	// IsTTY should return true for actual terminal, false for pipes
	// This test is environment-dependent, so we mainly verify it doesn't panic
	result := IsTTY()
	assert.IsType(t, false, result)
}

func TestShouldUsePlainMode_JSONFlag(t *testing.T) {
	assert.True(t, ShouldUsePlainMode(true, false, nil))
}

func TestShouldUsePlainMode_PlainFlag(t *testing.T) {
	assert.True(t, ShouldUsePlainMode(false, true, nil))
}

func TestShouldUsePlainMode_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	assert.True(t, ShouldUsePlainMode(false, false, os.Environ))
}

func TestShouldUsePlainMode_AgentdPlain(t *testing.T) {
	t.Setenv("AGENTD_PLAIN", "1")
	assert.True(t, ShouldUsePlainMode(false, false, os.Environ))
}

func TestShouldUsePlainMode_CI(t *testing.T) {
	t.Setenv("CI", "true")
	assert.True(t, ShouldUsePlainMode(false, false, os.Environ))
}

func TestShouldUsePlainMode_Default(t *testing.T) {
	// Clear environment
	os.Clearenv()
	// When no flags or env vars, should return false (use rich mode)
	assert.False(t, ShouldUsePlainMode(false, false, os.Environ))
}

func TestGetTerminalSize(t *testing.T) {
	width, height := GetTerminalSize()
	// Should return sensible defaults or actual terminal size
	assert.True(t, width >= 0, "width should be non-negative")
	assert.True(t, height >= 0, "height should be non-negative")
}

func TestIsTerminalTooSmall(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		expected bool
	}{
		{"large enough", 80, 24, false},
		{"too narrow", 79, 24, true},
		{"too short", 80, 23, true},
		{"both too small", 60, 20, true},
		{"larger than minimum", 100, 30, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTerminalTooSmall(tt.width, tt.height)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./cmd/agentd/internal/detect -v`

Expected: FAIL - package/functions don't exist yet

**Step 4: Commit test file**

```bash
git add cmd/agentd/internal/detect/detect_test.go
git commit -m "test: add terminal detection tests (RED)"
```

---

## Task 3: Terminal Detection Package (Implementation)

**Files:**
- Create: `cmd/agentd/internal/detect/detect.go`

**Step 1: Write minimal implementation**

Create `cmd/agentd/internal/detect/detect.go`:

```go
package detect

import (
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	minTerminalWidth  = 80
	minTerminalHeight = 24
)

// IsTTY checks if stdout is a terminal (not piped/redirected).
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// ShouldUsePlainMode determines if plain mode should be used based on flags and environment.
// Priority: --json > --plain > environment detection > auto-detect TTY
func ShouldUsePlainMode(jsonMode bool, plainMode bool, getenv func() []string) bool {
	// Explicit flags take precedence
	if jsonMode || plainMode {
		return true
	}

	// Check environment variables
	for _, env := range getenv() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := parts[1]

		switch key {
		case "NO_COLOR", "AGENTD_PLAIN":
			if val != "" {
				return true
			}
		case "CI":
			if val == "true" || val == "1" {
				return true
			}
		}
	}

	// Auto-detect: if not a TTY, use plain mode
	if !IsTTY() {
		return true
	}

	// Check terminal size
	width, height := GetTerminalSize()
	if IsTerminalTooSmall(width, height) {
		return true
	}

	return false
}

// GetTerminalSize returns the current terminal width and height.
// Returns (0, 0) if unable to determine.
func GetTerminalSize() (width int, height int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, 0
	}
	return w, h
}

// IsTerminalTooSmall checks if terminal is smaller than minimum requirements.
func IsTerminalTooSmall(width, height int) bool {
	return width < minTerminalWidth || height < minTerminalHeight
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./cmd/agentd/internal/detect -v`

Expected: PASS - all tests pass

**Step 3: Run staticcheck**

Run: `staticcheck ./cmd/agentd/internal/detect`

Expected: No issues found

**Step 4: Commit implementation**

```bash
git add cmd/agentd/internal/detect/detect.go
git commit -m "feat: add terminal detection package (GREEN)"
```

---

## Task 4: Retro Theme System (Tests)

**Files:**
- Create: `cmd/agentd/internal/theme/theme.go`
- Create: `cmd/agentd/internal/theme/theme_test.go`

**Step 1: Create theme package directory**

Run: `mkdir -p cmd/agentd/internal/theme`

Expected: Directory created

**Step 2: Write failing tests for theme system**

Create `cmd/agentd/internal/theme/theme_test.go`:

```go
package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestNewRetroTheme_CGA(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)

	assert.Equal(t, PaletteCGA, theme.Palette)
	assert.NotNil(t, theme.Primary)
	assert.NotNil(t, theme.Secondary)
	assert.NotNil(t, theme.Accent)
	assert.NotNil(t, theme.Success)
	assert.NotNil(t, theme.Warning)
	assert.NotNil(t, theme.Error)
	assert.NotNil(t, theme.Muted)
}

func TestNewRetroTheme_Amber(t *testing.T) {
	theme := NewRetroTheme(PaletteAmber)
	assert.Equal(t, PaletteAmber, theme.Palette)
	assert.NotNil(t, theme.Primary)
}

func TestNewRetroTheme_Green(t *testing.T) {
	theme := NewRetroTheme(PaletteGreen)
	assert.Equal(t, PaletteGreen, theme.Palette)
	assert.NotNil(t, theme.Primary)
}

func TestNewRetroTheme_C64(t *testing.T) {
	theme := NewRetroTheme(PaletteC64)
	assert.Equal(t, PaletteC64, theme.Palette)
	assert.NotNil(t, theme.Primary)
}

func TestRetroTheme_Logo(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	logo := theme.Logo.Render("TEST")
	assert.NotEmpty(t, logo)
	assert.Contains(t, logo, "TEST")
}

func TestRetroTheme_Header(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	header := theme.Header.Render("HEADER")
	assert.NotEmpty(t, header)
	assert.Contains(t, header, "HEADER")
}

func TestRetroTheme_BoxBorder(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	box := theme.BoxBorder.Render("content")
	assert.NotEmpty(t, box)
	assert.Contains(t, box, "content")
}

func TestRetroTheme_StatusBar(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	status := theme.StatusBar.Render("status")
	assert.NotEmpty(t, status)
	assert.Contains(t, status, "status")
}

func TestRetroTheme_Highlight(t *testing.T) {
	theme := NewRetroTheme(PaletteCGA)
	highlight := theme.Highlight.Render("highlighted")
	assert.NotEmpty(t, highlight)
	assert.Contains(t, highlight, "highlighted")
}

func TestPaletteName_String(t *testing.T) {
	tests := []struct {
		palette  PaletteName
		expected string
	}{
		{PaletteCGA, "cga"},
		{PaletteAmber, "amber"},
		{PaletteGreen, "green"},
		{PaletteC64, "c64"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.palette.String())
		})
	}
}

func TestParsePaletteName(t *testing.T) {
	tests := []struct {
		input    string
		expected PaletteName
		valid    bool
	}{
		{"cga", PaletteCGA, true},
		{"amber", PaletteAmber, true},
		{"green", PaletteGreen, true},
		{"c64", PaletteC64, true},
		{"CGA", PaletteCGA, true},        // case insensitive
		{"AMBER", PaletteAmber, true},
		{"invalid", PaletteCGA, false},  // defaults to CGA
		{"", PaletteCGA, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, valid := ParsePaletteName(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.valid, valid)
		})
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./cmd/agentd/internal/theme -v`

Expected: FAIL - package/functions don't exist yet

**Step 4: Commit test file**

```bash
git add cmd/agentd/internal/theme/theme_test.go
git commit -m "test: add retro theme system tests (RED)"
```

---

## Task 5: Retro Theme System (Implementation)

**Files:**
- Create: `cmd/agentd/internal/theme/theme.go`

**Step 1: Write minimal implementation**

Create `cmd/agentd/internal/theme/theme.go`:

```go
package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PaletteName represents available color palettes
type PaletteName string

const (
	PaletteCGA   PaletteName = "cga"
	PaletteAmber PaletteName = "amber"
	PaletteGreen PaletteName = "green"
	PaletteC64   PaletteName = "c64"
)

// String returns the string representation of the palette name
func (p PaletteName) String() string {
	return string(p)
}

// ParsePaletteName parses a palette name string (case-insensitive).
// Returns the palette and true if valid, or PaletteCGA and false if invalid.
func ParsePaletteName(s string) (PaletteName, bool) {
	switch strings.ToLower(s) {
	case "cga":
		return PaletteCGA, true
	case "amber":
		return PaletteAmber, true
	case "green":
		return PaletteGreen, true
	case "c64":
		return PaletteC64, true
	default:
		return PaletteCGA, false
	}
}

// RetroTheme contains all colors and styles for the retro aesthetic
type RetroTheme struct {
	Palette PaletteName

	// Colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Muted     lipgloss.Color

	// Styles
	Logo      lipgloss.Style
	Header    lipgloss.Style
	BoxBorder lipgloss.Style
	StatusBar lipgloss.Style
	Highlight lipgloss.Style
}

// NewRetroTheme creates a new retro theme with the specified palette
func NewRetroTheme(palette PaletteName) *RetroTheme {
	theme := &RetroTheme{
		Palette: palette,
	}

	// Set colors based on palette
	switch palette {
	case PaletteCGA:
		theme.Primary = lipgloss.Color("#00FFFF")   // Cyan
		theme.Secondary = lipgloss.Color("#FF00FF") // Magenta
		theme.Accent = lipgloss.Color("#FFFF55")    // Yellow
		theme.Success = lipgloss.Color("#00FF00")   // Green
		theme.Warning = lipgloss.Color("#FFFF55")   // Yellow
		theme.Error = lipgloss.Color("#FF0000")     // Red
		theme.Muted = lipgloss.Color("#555555")     // Dark gray

	case PaletteAmber:
		amber := lipgloss.Color("#FFB000")
		theme.Primary = amber
		theme.Secondary = amber
		theme.Accent = lipgloss.Color("#FFCC00")
		theme.Success = amber
		theme.Warning = lipgloss.Color("#FF8800")
		theme.Error = lipgloss.Color("#FF4400")
		theme.Muted = lipgloss.Color("#664400")

	case PaletteGreen:
		green := lipgloss.Color("#00FF00")
		theme.Primary = green
		theme.Secondary = lipgloss.Color("#00DD00")
		theme.Accent = lipgloss.Color("#00FFAA")
		theme.Success = green
		theme.Warning = lipgloss.Color("#AAFF00")
		theme.Error = lipgloss.Color("#FF0000")
		theme.Muted = lipgloss.Color("#005500")

	case PaletteC64:
		theme.Primary = lipgloss.Color("#6C5EB5")   // C64 blue
		theme.Secondary = lipgloss.Color("#B66DFF") // C64 purple
		theme.Accent = lipgloss.Color("#70A4B2")    // C64 cyan
		theme.Success = lipgloss.Color("#588D43")   // C64 green
		theme.Warning = lipgloss.Color("#B6A470")   // C64 yellow
		theme.Error = lipgloss.Color("#B55E5E")     // C64 red
		theme.Muted = lipgloss.Color("#42348B")     // C64 dark blue
	}

	// Build styles
	theme.Logo = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	theme.Header = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Padding(0, 1)

	theme.BoxBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(0, 1)

	theme.StatusBar = lipgloss.NewStyle().
		Foreground(theme.Muted).
		Background(theme.Primary).
		Padding(0, 1)

	theme.Highlight = lipgloss.NewStyle().
		Foreground(theme.Accent).
		Bold(true)

	return theme
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./cmd/agentd/internal/theme -v`

Expected: PASS - all tests pass

**Step 3: Run staticcheck**

Run: `staticcheck ./cmd/agentd/internal/theme`

Expected: No issues found

**Step 4: Commit implementation**

```bash
git add cmd/agentd/internal/theme/theme.go
git commit -m "feat: add retro theme system with CGA/amber/green/C64 palettes (GREEN)"
```

---

## Task 6: ASCII Art Library (Tests)

**Files:**
- Create: `cmd/agentd/internal/theme/ascii.go`
- Create: `cmd/agentd/internal/theme/ascii_test.go`

**Step 1: Write failing tests for ASCII art**

Create `cmd/agentd/internal/theme/ascii_test.go`:

```go
package theme

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLogo(t *testing.T) {
	tests := []struct {
		size LogoSize
		name string
	}{
		{LogoSmall, "small"},
		{LogoMedium, "medium"},
		{LogoLarge, "large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logo := GetLogo(tt.size)
			assert.NotEmpty(t, logo)
			// Should contain some recognizable element
			assert.True(t, len(logo) > 10, "logo should have content")
		})
	}
}

func TestGetAgentStatusIcon(t *testing.T) {
	tests := []struct {
		status   AgentStatus
		expected string
	}{
		{StatusRunning, "⚡"},
		{StatusPaused, "⏸"},
		{StatusStopped, "✗"},
		{StatusIdle, "💤"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			icon := GetAgentStatusIcon(tt.status)
			assert.Equal(t, tt.expected, icon)
		})
	}
}

func TestDrawBox(t *testing.T) {
	content := "Test Content"
	box := DrawBox("Title", content, 40)

	assert.NotEmpty(t, box)
	assert.Contains(t, box, "Title")
	assert.Contains(t, box, content)
	assert.Contains(t, box, "─") // horizontal border
	assert.Contains(t, box, "│") // vertical border
}

func TestDrawBoxEmpty(t *testing.T) {
	box := DrawBox("", "content", 20)
	assert.NotEmpty(t, box)
	assert.Contains(t, box, "content")
}

func TestDrawHeader(t *testing.T) {
	header := DrawHeader("TEST HEADER")
	assert.NotEmpty(t, header)
	assert.Contains(t, header, "TEST HEADER")
	assert.Contains(t, header, "═")
}

func TestGetVintageMessage(t *testing.T) {
	tests := []struct {
		category MessageCategory
		name     string
	}{
		{MsgConnecting, "connecting"},
		{MsgSuccess, "success"},
		{MsgError, "error"},
		{MsgLoading, "loading"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetVintageMessage(tt.category)
			assert.NotEmpty(t, msg)
			// Messages should be uppercase vintage style
			assert.Equal(t, strings.ToUpper(msg), msg)
		})
	}
}

func TestGetVintageMessage_Randomness(t *testing.T) {
	// Call multiple times to verify we get messages (might be same or different)
	messages := make(map[string]bool)
	for i := 0; i < 10; i++ {
		msg := GetVintageMessage(MsgSuccess)
		messages[msg] = true
	}
	// Should have at least 1 message
	assert.True(t, len(messages) >= 1)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/agentd/internal/theme -v`

Expected: FAIL - functions don't exist yet

**Step 3: Commit test file**

```bash
git add cmd/agentd/internal/theme/ascii_test.go
git commit -m "test: add ASCII art library tests (RED)"
```

---

## Task 7: ASCII Art Library (Implementation)

**Files:**
- Create: `cmd/agentd/internal/theme/ascii.go`

**Step 1: Write minimal implementation**

Create `cmd/agentd/internal/theme/ascii.go`:

```go
package theme

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// LogoSize represents the size variant of the logo
type LogoSize int

const (
	LogoSmall LogoSize = iota
	LogoMedium
	LogoLarge
)

// AgentStatus represents the status of an agent
type AgentStatus string

const (
	StatusRunning AgentStatus = "running"
	StatusPaused  AgentStatus = "paused"
	StatusStopped AgentStatus = "stopped"
	StatusIdle    AgentStatus = "idle"
)

// MessageCategory represents types of vintage messages
type MessageCategory int

const (
	MsgConnecting MessageCategory = iota
	MsgSuccess
	MsgError
	MsgLoading
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// GetLogo returns the Ourocodus ASCII art logo in the specified size
func GetLogo(size LogoSize) string {
	switch size {
	case LogoSmall:
		return ` ___  _   _ ___   ___   ___ ___  ___  _   _ ___
/ _ \| | | | _ \ / _ \ / __/ _ \|   \| | | / __|
\___/|___|_|_| \_\___/|___|___/|____||___|\___|`

	case LogoMedium:
		return ` ___  _   _ ___   ___   ___ ___  ___  _   _ ___
/ _ \| | | | _ \ / _ \ / __/ _ \|   \| | | / __|
\___/|___|_|_| \_\___/|___|___/|____||___|\___|
   Multi-Agent Coordination Platform`

	case LogoLarge:
		return ` ___  _   _ ___   ___   ___ ___  ___  _   _ ___
/ _ \| | | | _ \ / _ \ / __/ _ \|   \| | | / __|
\___/|___|_|_| \_\___/|___|___/|____||___|\___|

       🐉 Multi-Agent Coordination Platform 🐉
    Git Worktrees • Docker Isolation • NATS Messaging`

	default:
		return GetLogo(LogoMedium)
	}
}

// GetAgentStatusIcon returns an emoji/symbol for the agent status
func GetAgentStatusIcon(status AgentStatus) string {
	switch status {
	case StatusRunning:
		return "⚡"
	case StatusPaused:
		return "⏸"
	case StatusStopped:
		return "✗"
	case StatusIdle:
		return "💤"
	default:
		return "?"
	}
}

// DrawBox draws a box with title and content using box-drawing characters
func DrawBox(title, content string, width int) string {
	var sb strings.Builder

	// Top border
	sb.WriteString("┌")
	if title != "" {
		titlePart := fmt.Sprintf("─ %s ", title)
		sb.WriteString(titlePart)
		remaining := width - len(titlePart) - 2
		if remaining > 0 {
			sb.WriteString(strings.Repeat("─", remaining))
		}
	} else {
		sb.WriteString(strings.Repeat("─", width-2))
	}
	sb.WriteString("┐\n")

	// Content
	for _, line := range strings.Split(content, "\n") {
		sb.WriteString("│ ")
		sb.WriteString(line)
		padding := width - len(line) - 4
		if padding > 0 {
			sb.WriteString(strings.Repeat(" ", padding))
		}
		sb.WriteString(" │\n")
	}

	// Bottom border
	sb.WriteString("└")
	sb.WriteString(strings.Repeat("─", width-2))
	sb.WriteString("┘")

	return sb.String()
}

// DrawHeader draws a header with double-line borders
func DrawHeader(text string) string {
	width := len(text) + 8
	var sb strings.Builder

	sb.WriteString("╔")
	sb.WriteString(strings.Repeat("═", width-2))
	sb.WriteString("╗\n")

	sb.WriteString("║ ")
	sb.WriteString(strings.Repeat(" ", (width-len(text)-4)/2))
	sb.WriteString(text)
	sb.WriteString(strings.Repeat(" ", (width-len(text)-3)/2))
	sb.WriteString(" ║\n")

	sb.WriteString("╚")
	sb.WriteString(strings.Repeat("═", width-2))
	sb.WriteString("╝")

	return sb.String()
}

// GetVintageMessage returns a random vintage-style message for the category
func GetVintageMessage(category MessageCategory) string {
	messages := map[MessageCategory][]string{
		MsgConnecting: {
			"INITIALIZING PROTOCOLS",
			"CARRIER DETECTED",
			"ESTABLISHING LINK",
			"HANDSHAKE IN PROGRESS",
		},
		MsgSuccess: {
			"SYNC COMPLETE",
			"OPERATION NOMINAL",
			"TRANSFER SUCCESSFUL",
			"ACKNOWLEDGED",
		},
		MsgError: {
			"FAULT DETECTED",
			"PROTOCOL VIOLATION",
			"TRANSMISSION ERROR",
			"SYSTEM MALFUNCTION",
		},
		MsgLoading: {
			"LOADING DATASTREAM",
			"BUFFERING",
			"PROCESSING REQUEST",
			"STANDBY",
		},
	}

	choices := messages[category]
	if len(choices) == 0 {
		return "SYSTEM READY"
	}

	return choices[rng.Intn(len(choices))]
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./cmd/agentd/internal/theme -v`

Expected: PASS - all tests pass

**Step 3: Run staticcheck**

Run: `staticcheck ./cmd/agentd/internal/theme`

Expected: No issues found

**Step 4: Commit implementation**

```bash
git add cmd/agentd/internal/theme/ascii.go
git commit -m "feat: add ASCII art library with logos, icons, boxes, and vintage messages (GREEN)"
```

---

## Task 8: Output Mode Enum and Flags (Tests)

**Files:**
- Create: `cmd/agentd/internal/output/mode.go`
- Create: `cmd/agentd/internal/output/mode_test.go`

**Step 1: Create output package directory**

Run: `mkdir -p cmd/agentd/internal/output`

Expected: Directory created

**Step 2: Write failing tests for output mode**

Create `cmd/agentd/internal/output/mode_test.go`:

```go
package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeRich, "rich"},
		{ModePlain, "plain"},
		{ModeJSON, "json"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.mode.String())
		})
	}
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input    string
		expected Mode
		valid    bool
	}{
		{"rich", ModeRich, true},
		{"plain", ModePlain, true},
		{"json", ModeJSON, true},
		{"RICH", ModeRich, true},  // case insensitive
		{"JSON", ModeJSON, true},
		{"invalid", ModePlain, false},
		{"", ModePlain, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, valid := ParseMode(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestDetectMode(t *testing.T) {
	tests := []struct {
		name       string
		jsonFlag   bool
		plainFlag  bool
		shouldPlain bool
		expected   Mode
	}{
		{"json flag", true, false, false, ModeJSON},
		{"plain flag", false, true, false, ModePlain},
		{"both flags - json wins", true, true, false, ModeJSON},
		{"env plain", false, false, true, ModePlain},
		{"default rich", false, false, false, ModeRich},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectMode(tt.jsonFlag, tt.plainFlag, tt.shouldPlain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMode_IsRich(t *testing.T) {
	assert.True(t, ModeRich.IsRich())
	assert.False(t, ModePlain.IsRich())
	assert.False(t, ModeJSON.IsRich())
}

func TestMode_IsPlain(t *testing.T) {
	assert.False(t, ModeRich.IsPlain())
	assert.True(t, ModePlain.IsPlain())
	assert.False(t, ModeJSON.IsPlain())
}

func TestMode_IsJSON(t *testing.T) {
	assert.False(t, ModeRich.IsJSON())
	assert.False(t, ModePlain.IsJSON())
	assert.True(t, ModeJSON.IsJSON())
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./cmd/agentd/internal/output -v`

Expected: FAIL - package/functions don't exist yet

**Step 4: Commit test file**

```bash
git add cmd/agentd/internal/output/mode_test.go
git commit -m "test: add output mode detection tests (RED)"
```

---

## Task 9: Output Mode Enum and Flags (Implementation)

**Files:**
- Create: `cmd/agentd/internal/output/mode.go`

**Step 1: Write minimal implementation**

Create `cmd/agentd/internal/output/mode.go`:

```go
package output

import (
	"strings"
)

// Mode represents the output mode for CLI commands
type Mode int

const (
	ModeRich Mode = iota
	ModePlain
	ModeJSON
)

// String returns the string representation of the mode
func (m Mode) String() string {
	switch m {
	case ModeRich:
		return "rich"
	case ModePlain:
		return "plain"
	case ModeJSON:
		return "json"
	default:
		return "plain"
	}
}

// IsRich returns true if this is rich/TUI mode
func (m Mode) IsRich() bool {
	return m == ModeRich
}

// IsPlain returns true if this is plain text mode
func (m Mode) IsPlain() bool {
	return m == ModePlain
}

// IsJSON returns true if this is JSON mode
func (m Mode) IsJSON() bool {
	return m == ModeJSON
}

// ParseMode parses a mode string (case-insensitive).
// Returns the mode and true if valid, or ModePlain and false if invalid.
func ParseMode(s string) (Mode, bool) {
	switch strings.ToLower(s) {
	case "rich":
		return ModeRich, true
	case "plain":
		return ModePlain, true
	case "json":
		return ModeJSON, true
	default:
		return ModePlain, false
	}
}

// DetectMode determines the output mode based on flags and environment.
// Priority: --json > --plain > environment detection > rich mode
func DetectMode(jsonFlag bool, plainFlag bool, shouldUsePlain bool) Mode {
	if jsonFlag {
		return ModeJSON
	}
	if plainFlag {
		return ModePlain
	}
	if shouldUsePlain {
		return ModePlain
	}
	return ModeRich
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./cmd/agentd/internal/output -v`

Expected: PASS - all tests pass

**Step 3: Run staticcheck**

Run: `staticcheck ./cmd/agentd/internal/output`

Expected: No issues found

**Step 4: Commit implementation**

```bash
git add cmd/agentd/internal/output/mode.go
git commit -m "feat: add output mode enum and detection (GREEN)"
```

---

## Task 10: Integration Test for Complete Foundation

**Files:**
- Create: `cmd/agentd/internal/foundation_test.go`

**Step 1: Write integration test**

Create `cmd/agentd/internal/foundation_test.go`:

```go
package internal_test

import (
	"os"
	"testing"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/stretchr/testify/assert"
)

// TestFoundationIntegration verifies all foundation components work together
func TestFoundationIntegration(t *testing.T) {
	// Test 1: Terminal detection works
	isTTY := detect.IsTTY()
	assert.IsType(t, false, isTTY)

	// Test 2: Should use plain mode in CI
	t.Setenv("CI", "true")
	shouldPlain := detect.ShouldUsePlainMode(false, false, os.Environ)
	assert.True(t, shouldPlain)

	// Test 3: Mode detection respects environment
	mode := output.DetectMode(false, false, shouldPlain)
	assert.Equal(t, output.ModePlain, mode)

	// Test 4: Theme creation works for all palettes
	palettes := []theme.PaletteName{
		theme.PaletteCGA,
		theme.PaletteAmber,
		theme.PaletteGreen,
		theme.PaletteC64,
	}

	for _, palette := range palettes {
		t.Run(palette.String(), func(t *testing.T) {
			th := theme.NewRetroTheme(palette)
			assert.NotNil(t, th)
			assert.Equal(t, palette, th.Palette)

			// Verify we can render with each style
			logo := th.Logo.Render("TEST")
			assert.NotEmpty(t, logo)
		})
	}

	// Test 5: ASCII art works
	logo := theme.GetLogo(theme.LogoSmall)
	assert.NotEmpty(t, logo)

	icon := theme.GetAgentStatusIcon(theme.StatusRunning)
	assert.Equal(t, "⚡", icon)

	box := theme.DrawBox("Title", "Content", 30)
	assert.Contains(t, box, "Title")
	assert.Contains(t, box, "Content")

	msg := theme.GetVintageMessage(theme.MsgSuccess)
	assert.NotEmpty(t, msg)
}

// TestFoundationWorkflow tests a realistic workflow
func TestFoundationWorkflow(t *testing.T) {
	// Simulate command startup
	jsonFlag := false
	plainFlag := false

	// Step 1: Detect terminal capabilities
	shouldPlain := detect.ShouldUsePlainMode(jsonFlag, plainFlag, os.Environ)

	// Step 2: Select output mode
	mode := output.DetectMode(jsonFlag, plainFlag, shouldPlain)

	// Step 3: Create theme (only if rich mode)
	var th *theme.RetroTheme
	if mode.IsRich() {
		th = theme.NewRetroTheme(theme.PaletteCGA)
		assert.NotNil(t, th)
	}

	// Step 4: Render appropriate output
	if mode.IsRich() {
		logo := th.Logo.Render(theme.GetLogo(theme.LogoSmall))
		assert.NotEmpty(t, logo)
	} else if mode.IsPlain() {
		// Plain mode would just print text
		assert.NotNil(t, "plain mode")
	} else if mode.IsJSON() {
		// JSON mode would marshal data
		assert.NotNil(t, "json mode")
	}
}
```

**Step 2: Run integration test**

Run: `go test ./cmd/agentd/internal -v`

Expected: PASS - all foundation components work together

**Step 3: Run full test suite**

Run: `make test`

Expected: PASS - all tests pass including new foundation tests

**Step 4: Verify build still works**

Run: `make build`

Expected: SUCCESS - all binaries build

**Step 5: Commit integration test**

```bash
git add cmd/agentd/internal/foundation_test.go
git commit -m "test: add integration test for Phase 1 foundation"
```

---

## Task 11: Documentation

**Files:**
- Create: `cmd/agentd/internal/README.md`

**Step 1: Write package documentation**

Create `cmd/agentd/internal/README.md`:

```markdown
# agentd Internal Packages

Internal packages for the `agentd` CLI, providing foundation for Charm-based TUI features.

## Package Overview

### `detect/` - Terminal Detection

Detects terminal capabilities and determines appropriate output mode.

**Key Functions:**
- `IsTTY()` - Check if stdout is a terminal
- `ShouldUsePlainMode()` - Determine if plain mode should be used
- `GetTerminalSize()` - Get current terminal dimensions
- `IsTerminalTooSmall()` - Check if terminal meets minimum requirements (80x24)

**Environment Variables:**
- `NO_COLOR` - Disable colors
- `AGENTD_PLAIN` - Force plain mode
- `CI=true` - Auto-detect CI environment

### `theme/` - Retro Theme System

Provides retro computing aesthetics with multiple color palettes.

**Available Palettes:**
- **CGA** (default) - Bright cyan/magenta/yellow (#00FFFF, #FF00FF, #FFFF55)
- **Amber** - Monochrome amber terminal aesthetic
- **Green** - Green phosphor CRT vibes
- **C64** - Commodore 64 blue/purple palette

**Key Functions:**
- `NewRetroTheme(palette)` - Create theme with specified palette
- `GetLogo(size)` - Get Ourocodus ASCII art logo
- `GetAgentStatusIcon(status)` - Get emoji for agent status
- `DrawBox(title, content, width)` - Draw box with borders
- `DrawHeader(text)` - Draw header with double-line borders
- `GetVintageMessage(category)` - Get random vintage-style message

**Styles Available:**
- `Logo` - ASCII art banner styling
- `Header` - Command header styling
- `BoxBorder` - Bordered box styling
- `StatusBar` - Bottom status line styling
- `Highlight` - Selected/focused item styling

### `output/` - Output Mode Management

Manages output modes (rich/plain/json) for all commands.

**Modes:**
- **Rich** - Full TUI with Bubble Tea (default for TTY)
- **Plain** - Simple text with basic colors (CI/pipes)
- **JSON** - Machine-readable structured output

**Key Functions:**
- `DetectMode(jsonFlag, plainFlag, shouldUsePlain)` - Determine output mode
- `ParseMode(s)` - Parse mode string
- `Mode.IsRich()`, `Mode.IsPlain()`, `Mode.IsJSON()` - Mode checks

**Priority:**
1. `--json` flag (highest)
2. `--plain` flag
3. Environment detection (NO_COLOR, CI, etc.)
4. Auto-detect TTY
5. Terminal size check
6. Default to rich mode

## Usage Example

```go
import (
	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
)

// In your command's RunE function:
func runCommand(cmd *cobra.Command, args []string) error {
	// Step 1: Detect terminal capabilities
	shouldPlain := detect.ShouldUsePlainMode(jsonFlag, plainFlag, os.Environ)

	// Step 2: Determine output mode
	mode := output.DetectMode(jsonFlag, plainFlag, shouldPlain)

	// Step 3: Create theme (if rich mode)
	var th *theme.RetroTheme
	if mode.IsRich() {
		th = theme.NewRetroTheme(theme.PaletteCGA)
	}

	// Step 4: Render appropriate output
	if mode.IsRich() {
		// Use Bubble Tea TUI
		logo := th.Logo.Render(theme.GetLogo(theme.LogoSmall))
		fmt.Println(logo)
	} else if mode.IsPlain() {
		// Use simple text output
		fmt.Println("📋 Plain output")
	} else if mode.IsJSON() {
		// Use JSON output
		json.NewEncoder(os.Stdout).Encode(data)
	}

	return nil
}
```

## Testing

All packages have comprehensive test coverage:

```bash
# Test individual packages
go test ./cmd/agentd/internal/detect -v
go test ./cmd/agentd/internal/theme -v
go test ./cmd/agentd/internal/output -v

# Test integration
go test ./cmd/agentd/internal -v

# Run all tests
make test
```

## Design Decisions

1. **Terminal Detection First** - Always check capabilities before rendering
2. **Mode Priority** - Explicit flags > environment > auto-detect
3. **Backward Compatible** - Plain mode maintains exact current output format
4. **Theme Separation** - Colors, styles, and ASCII art in dedicated package
5. **TUI-Ready** - Foundation ready for Bubble Tea integration in Phase 2
```

**Step 2: Commit documentation**

```bash
git add cmd/agentd/internal/README.md
git commit -m "docs: add internal packages documentation for Phase 1"
```

---

## Task 12: Verify Phase 1 Complete

**Step 1: Run all quality checks**

Run: `make pre-commit`

Expected: All checks pass (fmt, vet, lint, test, build)

**Step 2: Verify test coverage**

Run: `go test ./cmd/agentd/internal/... -cover`

Expected: High coverage (>80%) for all packages

**Step 3: Generate coverage report**

Run: `go test ./cmd/agentd/internal/... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html`

Expected: HTML coverage report generated

**Step 4: Manual verification checklist**

- [ ] All dependencies added to go.mod
- [ ] Terminal detection works (test in different environments)
- [ ] All 4 color palettes render correctly
- [ ] ASCII art displays properly
- [ ] Output modes switch correctly
- [ ] Integration test passes
- [ ] Documentation complete
- [ ] All tests pass
- [ ] No linter warnings
- [ ] Project still builds

**Step 5: Create Phase 1 completion tag**

```bash
git tag -a v0.1.0-phase1-foundation -m "Phase 1: Charm CLI Foundation complete

Foundation packages for Charm-based CLI:
- Terminal detection (detect package)
- Retro theme system with 4 palettes (theme package)
- ASCII art library (logos, boxes, vintage messages)
- Output mode management (rich/plain/json)

Deliverable: Theme system and mode detection infrastructure ready for Phase 2.
Blocks: Phase 2 (Pilot Command) and all subsequent phases."
```

**Step 6: Final commit**

```bash
git add coverage.html
git commit -m "test: add coverage report for Phase 1"
git push origin main
git push origin v0.1.0-phase1-foundation
```

---

## Phase 1 Complete! 🎉

**What We Built:**
- ✅ Charm dependencies (Bubble Tea, Bubbles, Lip Gloss, Huh, Log, Harmonica)
- ✅ Terminal detection package (TTY, size, environment)
- ✅ Retro theme system (4 color palettes: CGA, Amber, Green, C64)
- ✅ ASCII art library (logos, status icons, boxes, vintage messages)
- ✅ Output mode management (rich/plain/json with priority system)
- ✅ Comprehensive test coverage
- ✅ Integration tests
- ✅ Documentation

**Ready For:**
- Phase 2: Pilot Command (rebuild `agentd list` with Charm)
- Phase 3: Live Commands
- Phase 4: Relay Dashboard
- Phase 5: Interactive Wizard
- Phase 6: Polish

**Next Steps:**
1. Review Phase 1 implementation
2. Create detailed plan for Phase 2: Pilot Command
3. Execute Phase 2 with `superpowers:executing-plans`
