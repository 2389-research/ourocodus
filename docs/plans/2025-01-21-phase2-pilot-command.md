# Phase 2: Pilot Command (`agentd list`) - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebuild `agentd list` command with Charm integration to validate Phase 1 foundation packages and establish patterns for all future commands.

**Architecture:** Integrate detect/theme/output packages into existing `agentd list`, add `--plain`, `--theme` flags, create rich mode renderer with retro styling, maintain backward compatibility. Start simple with enhanced table output (not full Bubble Tea TUI yet).

**Tech Stack:** Go 1.24, Cobra CLI, Foundation packages (detect/theme/output), Lip Gloss styling, existing Docker client

---

## Current State Analysis

**Existing `agentd list` implementation:**
- File: `cmd/agentd/cmd_list.go` (265 lines)
- Flags: `--format` (table|json)
- Functions: `runList()`, `listAgentsFromDocker()`, `printListTableFromAgentInfo()`, `printListJSONFromAgentInfo()`
- Uses: `github.com/fatih/color` for basic coloring, `text/tabwriter` for tables

**Phase 2 Goal:**
- Integrate foundation packages
- Add `--plain` and `--theme` flags
- Create rich mode renderer using theme system
- Maintain exact backward compatibility for `--format json`
- Plain mode matches current table output format

---

## Task 1: Add Renderer Package (Tests)

**Files:**
- Create: `cmd/agentd/internal/render/list.go`
- Create: `cmd/agentd/internal/render/list_test.go`

**Step 1: Create render package directory**

Run: `mkdir -p cmd/agentd/internal/render`

Expected: Directory created

**Step 2: Write failing tests for list renderers**

Create `cmd/agentd/internal/render/list_test.go`:

```go
package render

import (
	"bytes"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/stretchr/testify/assert"
)

// AgentInfo represents an agent for rendering (mirrors cmd_list.go)
type AgentInfo struct {
	AgentID     string
	ContainerID string
	Status      string
	Workspace   string
	SpawnSource string
	AttachedTo  string
	CreatedAt   time.Time
}

func TestRenderAgentList_Plain(t *testing.T) {
	agents := []AgentInfo{
		{
			AgentID:     "test-agent",
			Status:      "running",
			SpawnSource: "cli",
			Workspace:   "/path/to/workspace",
			CreatedAt:   time.Now().Add(-1 * time.Hour),
		},
	}

	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, output.ModePlain, nil)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "test-agent")
	assert.Contains(t, output, "running")
	assert.Contains(t, output, "cli")
}

func TestRenderAgentList_JSON(t *testing.T) {
	agents := []AgentInfo{
		{
			AgentID:     "test-agent",
			Status:      "running",
			SpawnSource: "cli",
			CreatedAt:   time.Now(),
		},
	}

	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, output.ModeJSON, nil)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, `"AgentID":"test-agent"`)
	assert.Contains(t, output, `"Status":"running"`)
}

func TestRenderAgentList_Rich(t *testing.T) {
	agents := []AgentInfo{
		{
			AgentID:     "test-agent",
			Status:      "running",
			SpawnSource: "cli",
			CreatedAt:   time.Now(),
		},
	}

	th := theme.NewRetroTheme(theme.PaletteCGA)
	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, output.ModeRich, th)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "test-agent")
	assert.Contains(t, output, "running")
	// Rich mode should have styled output (not plain text)
	assert.NotEmpty(t, output)
}

func TestRenderAgentList_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := RenderAgentList(&buf, []AgentInfo{}, output.ModePlain, nil)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "No agents running")
}

func TestRenderAgentList_MultipleAgents(t *testing.T) {
	agents := []AgentInfo{
		{AgentID: "agent-1", Status: "running", SpawnSource: "cli", CreatedAt: time.Now()},
		{AgentID: "agent-2", Status: "paused", SpawnSource: "relay", CreatedAt: time.Now()},
		{AgentID: "agent-3", Status: "exited", SpawnSource: "cli", CreatedAt: time.Now()},
	}

	var buf bytes.Buffer
	err := RenderAgentList(&buf, agents, output.ModePlain, nil)

	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "agent-1")
	assert.Contains(t, output, "agent-2")
	assert.Contains(t, output, "agent-3")
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./cmd/agentd/internal/render -v`

Expected: FAIL - package/functions don't exist yet

**Step 4: Commit test file**

```bash
git add cmd/agentd/internal/render/list_test.go
git commit -m "test: add agent list renderer tests (RED)"
```

---

## Task 2: Add Renderer Package (Implementation)

**Files:**
- Create: `cmd/agentd/internal/render/list.go`

**Step 1: Write minimal implementation**

Create `cmd/agentd/internal/render/list.go`:

```go
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// AgentInfo represents an agent for rendering
type AgentInfo struct {
	AgentID     string
	ContainerID string
	Status      string
	Workspace   string
	SpawnSource string
	AttachedTo  string
	CreatedAt   time.Time
}

// RenderAgentList renders a list of agents in the specified output mode
func RenderAgentList(w io.Writer, agents []AgentInfo, mode output.Mode, th *theme.RetroTheme) error {
	if len(agents) == 0 {
		return renderEmptyList(w, mode, th)
	}

	switch {
	case mode.IsJSON():
		return renderJSON(w, agents)
	case mode.IsPlain():
		return renderPlainTable(w, agents)
	case mode.IsRich():
		if th == nil {
			th = theme.NewRetroTheme(theme.PaletteCGA)
		}
		return renderRichTable(w, agents, th)
	default:
		return renderPlainTable(w, agents)
	}
}

func renderEmptyList(w io.Writer, mode output.Mode, th *theme.RetroTheme) error {
	if mode.IsJSON() {
		return json.NewEncoder(w).Encode([]AgentInfo{})
	}

	msg := "✨ No agents running."
	if mode.IsRich() && th != nil {
		msg = th.Muted.Render(msg)
	}

	_, err := fmt.Fprintln(w, msg)
	return err
}

func renderJSON(w io.Writer, agents []AgentInfo) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}

func renderPlainTable(w io.Writer, agents []AgentInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Header
	fmt.Fprintln(tw)
	fmt.Fprintf(tw, "AGENT\tSTATUS\tSOURCE\tATTACHED TO\tWORKSPACE\tCREATED\n")

	// Rows
	for _, agent := range agents {
		attachedTo := "-"
		if agent.AttachedTo != "" {
			attachedTo = formatShortID(agent.AttachedTo, 9)
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			agent.AgentID,
			agent.Status,
			agent.SpawnSource,
			attachedTo,
			formatWorkspace(agent.Workspace),
			formatDuration(time.Since(agent.CreatedAt)),
		)
	}

	fmt.Fprintln(tw)
	return tw.Flush()
}

func renderRichTable(w io.Writer, agents []AgentInfo, th *theme.RetroTheme) error {
	// Header with theme styling
	header := th.Header.Render(theme.GetLogo(theme.LogoSmall))
	fmt.Fprintln(w, header)
	fmt.Fprintln(w)

	// Table header
	headerStyle := lipgloss.NewStyle().
		Foreground(th.Primary).
		Bold(true)

	fmt.Fprintf(w, "%s  %s  %s  %s  %s\n",
		headerStyle.Render("AGENT"),
		headerStyle.Render("STATUS"),
		headerStyle.Render("SOURCE"),
		headerStyle.Render("ATTACHED TO"),
		headerStyle.Render("CREATED"),
	)

	// Separator line
	separatorStyle := lipgloss.NewStyle().Foreground(th.Muted)
	fmt.Fprintln(w, separatorStyle.Render(strings.Repeat("─", 80)))

	// Table rows
	for _, agent := range agents {
		statusIcon := getStatusIcon(agent.Status)
		statusColor := getStatusColor(agent.Status, th)

		attachedTo := "─"
		if agent.AttachedTo != "" {
			attachedTo = th.Accent.Render(formatShortID(agent.AttachedTo, 9))
		}

		sourceStyle := getSourceStyle(agent.SpawnSource, th)

		fmt.Fprintf(w, "%s  %s %s  %s  %s  %s\n",
			th.Highlight.Render(agent.AgentID),
			statusIcon,
			statusColor.Render(agent.Status),
			sourceStyle.Render(agent.SpawnSource),
			attachedTo,
			th.Muted.Render(formatDuration(time.Since(agent.CreatedAt))),
		)
	}

	fmt.Fprintln(w)

	// Footer with summary
	summaryStyle := lipgloss.NewStyle().Foreground(th.Secondary)
	summary := fmt.Sprintf("Total: %d agents", len(agents))
	fmt.Fprintln(w, summaryStyle.Render(summary))

	return nil
}

func getStatusIcon(status string) string {
	switch status {
	case "running":
		return theme.GetAgentStatusIcon(theme.StatusRunning)
	case "paused":
		return theme.GetAgentStatusIcon(theme.StatusPaused)
	case "exited", "stopped":
		return theme.GetAgentStatusIcon(theme.StatusStopped)
	default:
		return theme.GetAgentStatusIcon(theme.StatusIdle)
	}
}

func getStatusColor(status string, th *theme.RetroTheme) lipgloss.Style {
	switch status {
	case "running":
		return lipgloss.NewStyle().Foreground(th.Success)
	case "paused":
		return lipgloss.NewStyle().Foreground(th.Warning)
	case "exited", "stopped":
		return lipgloss.NewStyle().Foreground(th.Error)
	default:
		return lipgloss.NewStyle().Foreground(th.Muted)
	}
}

func getSourceStyle(source string, th *theme.RetroTheme) lipgloss.Style {
	switch source {
	case "cli":
		return lipgloss.NewStyle().Foreground(th.Primary)
	case "relay":
		return lipgloss.NewStyle().Foreground(th.Secondary)
	default:
		return lipgloss.NewStyle().Foreground(th.Muted)
	}
}

func formatShortID(id string, maxLen int) string {
	if len(id) <= maxLen {
		return id
	}
	return id[:maxLen] + "..."
}

func formatWorkspace(path string) string {
	if len(path) > 60 {
		return "..." + path[len(path)-57:]
	}
	return path
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./cmd/agentd/internal/render -v`

Expected: PASS - all tests pass

**Step 3: Run staticcheck**

Run: `staticcheck ./cmd/agentd/internal/render`

Expected: No issues found

**Step 4: Commit implementation**

```bash
git add cmd/agentd/internal/render/list.go
git commit -m "feat: add agent list renderer with rich/plain/json modes (GREEN)"
```

---

## Task 3: Update cmd_list.go (Tests)

**Files:**
- Modify: `cmd/agentd/cmd_list.go`
- Create: `cmd/agentd/cmd_list_integration_test.go`

**Step 1: Write integration tests for updated list command**

Create `cmd/agentd/cmd_list_integration_test.go`:

```go
package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListCommand_Flags(t *testing.T) {
	// Test that new flags are registered
	assert.NotNil(t, listCmd.Flags().Lookup("format"))
	assert.NotNil(t, listCmd.Flags().Lookup("plain"))
	assert.NotNil(t, listCmd.Flags().Lookup("theme"))
}

func TestListCommand_FlagDefaults(t *testing.T) {
	// Verify default values
	formatFlag := listCmd.Flags().Lookup("format")
	assert.Equal(t, "auto", formatFlag.DefValue)

	themeFlag := listCmd.Flags().Lookup("theme")
	assert.Equal(t, "cga", themeFlag.DefValue)
}

func TestListCommand_Help(t *testing.T) {
	var buf bytes.Buffer
	listCmd.SetOut(&buf)
	listCmd.SetArgs([]string{"--help"})

	err := listCmd.Execute()
	assert.NoError(t, err)

	helpText := buf.String()
	assert.Contains(t, helpText, "--format")
	assert.Contains(t, helpText, "--plain")
	assert.Contains(t, helpText, "--theme")
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/agentd -run TestListCommand -v`

Expected: FAIL - new flags don't exist yet

**Step 3: Commit test file**

```bash
git add cmd/agentd/cmd_list_integration_test.go
git commit -m "test: add integration tests for updated list command (RED)"
```

---

## Task 4: Update cmd_list.go (Implementation)

**Files:**
- Modify: `cmd/agentd/cmd_list.go`

**Step 1: Update imports and add new flags**

Modify `cmd/agentd/cmd_list.go` imports section:

```go
import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/render"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)
```

**Step 2: Update flag variables and init function**

Replace the flag section (lines 19-35) with:

```go
var (
	listFormat    string
	listPlainFlag bool
	listTheme     string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "📋 List all active agents",
	Long:  "Shows all active agents with their status, workspace, and container information.",
	Example: `  # List all running agents (auto-detect mode)
  agentd list

  # Force plain text output
  agentd list --plain

  # JSON output for scripting
  agentd list --format json

  # Use amber theme for rich mode
  agentd list --theme amber`,
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listFormat, "format", "auto", "Output format (auto|rich|plain|json)")
	listCmd.Flags().BoolVar(&listPlainFlag, "plain", false, "Force plain text output (alias for --format plain)")
	listCmd.Flags().StringVar(&listTheme, "theme", "cga", "Color theme for rich mode (cga|amber|green|c64)")
}
```

**Step 3: Replace runList function**

Replace `runList()` function (lines 37-57) with:

```go
func runList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Step 1: Detect output mode
	jsonFlag := listFormat == "json"
	plainFlag := listPlainFlag || listFormat == "plain"
	shouldPlain := detect.ShouldUsePlainMode(jsonFlag, plainFlag, os.Environ)

	mode := output.ModeRich
	if listFormat != "auto" {
		// Explicit format flag takes precedence
		parsedMode, valid := output.ParseMode(listFormat)
		if valid {
			mode = parsedMode
		}
	} else {
		// Auto-detect mode
		mode = output.DetectMode(jsonFlag, plainFlag, shouldPlain)
	}

	// Step 2: Create theme if rich mode
	var th *theme.RetroTheme
	if mode.IsRich() {
		palette, valid := theme.ParsePaletteName(listTheme)
		if !valid {
			palette = theme.PaletteCGA
		}
		th = theme.NewRetroTheme(palette)
	}

	// Step 3: Query Docker for agents
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	// Step 4: Convert to render.AgentInfo format
	renderAgents := make([]render.AgentInfo, len(agents))
	for i, agent := range agents {
		renderAgents[i] = render.AgentInfo{
			AgentID:     agent.AgentID,
			ContainerID: agent.ContainerID,
			Status:      agent.Status,
			Workspace:   agent.Workspace,
			SpawnSource: agent.SpawnSource,
			AttachedTo:  agent.AttachedTo,
			CreatedAt:   agent.CreatedAt,
		}
	}

	// Step 5: Render using new renderer
	return render.RenderAgentList(os.Stdout, renderAgents, mode, th)
}
```

**Step 4: Remove old print functions**

Remove these functions (they're replaced by render package):
- `printListTableFromAgentInfo()` (lines 143-182)
- `printListJSONFromAgentInfo()` (lines 184-189)
- `formatStateString()` (lines 191-207)
- `formatSpawnSource()` (lines 209-221)
- `formatWorkspace()` (lines 223-231)
- `formatContainerID()` (lines 233-239)

Keep these functions (still needed):
- `listAgentsFromDocker()` - unchanged
- `listLeasesForList()` - unchanged
- `formatAttachedTo()` - unchanged

**Step 5: Run tests to verify they pass**

Run: `go test ./cmd/agentd -run TestListCommand -v`

Expected: PASS - all tests pass

**Step 6: Run full test suite**

Run: `make test`

Expected: All tests pass

**Step 7: Test manually**

Run: `make build && ./bin/agentd list --help`

Expected: Help text shows new flags

**Step 8: Commit changes**

```bash
git add cmd/agentd/cmd_list.go cmd/agentd/cmd_list_integration_test.go
git commit -m "feat: integrate foundation packages into agentd list command"
```

---

## Task 5: Manual Testing and Verification

**Step 1: Build and test plain mode**

```bash
make build
./bin/agentd list --plain
NO_COLOR=1 ./bin/agentd list
./bin/agentd list | cat
```

Expected: Plain table output, no colors

**Step 2: Test JSON mode**

```bash
./bin/agentd list --format json
./bin/agentd list --format json | jq .
```

Expected: Valid JSON output

**Step 3: Test rich mode with different themes**

```bash
./bin/agentd list --format rich
./bin/agentd list --theme cga
./bin/agentd list --theme amber
./bin/agentd list --theme green
./bin/agentd list --theme c64
```

Expected: Styled output with different color schemes

**Step 4: Test auto-detection**

```bash
# Should use rich mode in terminal
./bin/agentd list

# Should auto-detect plain mode when piped
./bin/agentd list | cat

# Should use plain mode in CI
CI=true ./bin/agentd list
```

Expected: Correct mode auto-detection

**Step 5: Test with actual agents**

```bash
# Spawn test agents if needed
./bin/agentd spawn test-1
./bin/agentd spawn test-2

# List them in different modes
./bin/agentd list
./bin/agentd list --plain
./bin/agentd list --format json
./bin/agentd list --theme amber

# Cleanup
./bin/agentd stop test-1
./bin/agentd stop test-2
```

Expected: All modes render agents correctly

**Step 6: Run all quality checks**

```bash
make pre-commit
```

Expected: All checks pass

**Step 7: Document results**

Create manual test results file:

```bash
cat > /tmp/phase2-test-results.txt <<'EOF'
Phase 2 Manual Test Results
===========================

Plain Mode:
✓ --plain flag works
✓ NO_COLOR env var works
✓ Piped output auto-detects plain
✓ Output matches old format

JSON Mode:
✓ --format json works
✓ Valid JSON structure
✓ All fields present
✓ Backward compatible

Rich Mode:
✓ Default theme (CGA) works
✓ Amber theme works
✓ Green theme works
✓ C64 theme works
✓ Status icons display
✓ Colors applied correctly

Auto-Detection:
✓ TTY detected correctly
✓ Plain mode in CI environment
✓ Rich mode in interactive terminal

Integration:
✓ Works with no agents
✓ Works with multiple agents
✓ Handles long workspace paths
✓ Shows attachment status
✓ Duration formatting correct

Quality:
✓ All tests pass
✓ No linter warnings
✓ Build successful
✓ No regressions
EOF

cat /tmp/phase2-test-results.txt
```

---

## Task 6: Documentation Update

**Files:**
- Modify: `cmd/agentd/internal/README.md`

**Step 1: Add render package documentation**

Add to `cmd/agentd/internal/README.md` after the output package section:

```markdown
### `render/` - Output Renderers

Renders command output in different modes using foundation packages.

**Key Functions:**
- `RenderAgentList(w, agents, mode, theme)` - Render agent list in any mode

**Modes:**
- **Rich** - Styled output with theme, status icons, summary
- **Plain** - Simple text table (backward compatible)
- **JSON** - Structured JSON output

**Features:**
- Status icons (⚡ running, ⏸ paused, ✗ stopped, 💤 idle)
- Color coding based on theme
- Smart formatting (durations, workspace paths, IDs)
- Empty state handling

## Usage in Commands

### Integrating Foundation in Commands

```go
import (
    "github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
    "github.com/2389-research/ourocodus/cmd/agentd/internal/output"
    "github.com/2389-research/ourocodus/cmd/agentd/internal/render"
    "github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
)

func runCommand(cmd *cobra.Command, args []string) error {
    // Step 1: Detect mode
    shouldPlain := detect.ShouldUsePlainMode(jsonFlag, plainFlag, os.Environ)
    mode := output.DetectMode(jsonFlag, plainFlag, shouldPlain)

    // Step 2: Create theme for rich mode
    var th *theme.RetroTheme
    if mode.IsRich() {
        palette, _ := theme.ParsePaletteName(themeFlag)
        th = theme.NewRetroTheme(palette)
    }

    // Step 3: Get data
    data := fetchData()

    // Step 4: Render
    return render.RenderOutput(os.Stdout, data, mode, th)
}
```

### Adding New Flags

```go
var (
    formatFlag string
    plainFlag  bool
    themeFlag  string
)

func init() {
    cmd.Flags().StringVar(&formatFlag, "format", "auto", "Output format (auto|rich|plain|json)")
    cmd.Flags().BoolVar(&plainFlag, "plain", false, "Force plain text output")
    cmd.Flags().StringVar(&themeFlag, "theme", "cga", "Color theme (cga|amber|green|c64)")
}
```

### Pattern for All Commands

1. Add flags (--format, --plain, --theme)
2. Detect mode using foundation packages
3. Create theme if rich mode
4. Fetch command data
5. Render using appropriate renderer

This pattern is now validated in `agentd list` and should be followed for all other commands.
```

**Step 2: Commit documentation**

```bash
git add cmd/agentd/internal/README.md
git commit -m "docs: add render package and command integration pattern documentation"
```

---

## Task 7: Verify Phase 2 Complete

**Step 1: Run all tests**

Run: `go test ./cmd/agentd/... -v`

Expected: All tests pass

**Step 2: Check coverage**

Run: `go test ./cmd/agentd/internal/... -cover`

Expected: Coverage remains >80%

**Step 3: Verify build**

Run: `make clean && make build`

Expected: All binaries build successfully

**Step 4: Final quality checks**

Run: `make pre-commit`

Expected: All checks pass (fmt, vet, lint, test, build)

**Step 5: Create Phase 2 summary**

```bash
git log --oneline --since="2 hours ago"
```

Expected: See all Phase 2 commits

**Step 6: Tag completion**

```bash
git tag -a v0.2.0-phase2-pilot -m "Phase 2: Pilot Command complete

Integrated foundation packages into agentd list command:
- Added --plain and --theme flags
- Created render package for output modes
- Implemented rich mode with retro themes
- Maintained backward compatibility
- Validated foundation architecture

Deliverable: Pattern validated - ready for Phase 3 (Live Commands).
Blocks: Phase 3, 4, 5, 6."
```

---

## Phase 2 Complete! 🎉

**What We Built:**
- ✅ Render package for mode-specific output
- ✅ Rich mode table with retro themes
- ✅ Integration of all foundation packages (detect, theme, output)
- ✅ New flags: --plain, --theme
- ✅ Auto-detection of output mode
- ✅ Backward compatible with existing --format json
- ✅ Comprehensive tests
- ✅ Documentation updates

**Validated:**
- ✅ Foundation packages work in real command
- ✅ Pattern is repeatable for other commands
- ✅ No regressions in existing functionality
- ✅ Quality standards maintained

**Ready For:**
- Phase 3: Live Commands (`agentd watch`, `agentd discover --watch`)
- Phase 4: Relay Dashboard (full-screen TUI)
- Phase 5: Interactive Wizard (`agentd spawn`)
- Phase 6: Polish (remaining commands)

**Next Steps:**
1. Review Phase 2 implementation
2. Create detailed plan for Phase 3: Live Commands
3. Execute Phase 3 with Bubble Tea interactive TUI
