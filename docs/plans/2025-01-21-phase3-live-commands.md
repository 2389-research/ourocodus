# Phase 3: Live Commands - Implementation Plan

**Date**: 2025-01-21
**Status**: Planning
**Depends On**: Phase 2 (Foundation + Pilot Command) ✓

## Overview

Phase 3 introduces **live interactive TUIs** using Bubble Tea's Model/Update/View architecture. We'll build two commands:

1. **`agentd watch <agent-id>`** - Full-screen heartbeat monitor with real-time updates
2. **`agentd discover --watch`** - Live agent table with auto-refresh

This phase validates our ability to build complex interactive TUIs and establishes patterns for Phase 4 (relay dashboard).

## Goals

- Prove Bubble Tea event loop works with NATS subscriptions
- Establish patterns for live data updates and keyboard navigation
- Validate performance with real-time rendering
- Build reusable TUI components for Phase 4

## Required Components (Prescribed Upfront)

### `agentd watch`
- **Bubbles `viewport`** - Scrollable heartbeat log container
- **Bubbles `progress`** - Lease countdown bar (percentage-based)
- **Bubbles `spinner`** - Initial connection animation
- **Harmonica** - Smooth lag bar width transitions (spring physics)
- **Lip Gloss `JoinVertical`** - Multi-pane layout (header/heartbeat/lease/footer)
- **Bubble Tea model** - Full TUI with Init/Update/View methods

### `agentd discover --watch`
- **Bubbles `table`** - Agent roster (reuse from Phase 2)
- **Bubble Tea model** - Simple TUI with timer-based refresh
- **Lip Gloss styling** - Status updates and refresh indicators

## Architecture Patterns

### Pattern 1: Bubble Tea Model Structure
```go
type watchModel struct {
    // Components
    viewport   viewport.Model
    progress   progress.Model
    spinner    spinner.Model

    // Data
    heartbeats []HeartbeatEntry
    lease      LeaseInfo
    agentID    string

    // State
    ready      bool
    quitting   bool
    err        error

    // Theme
    theme      *theme.RetroTheme
}

func (m watchModel) Init() tea.Cmd
func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m watchModel) View() string
```

### Pattern 2: Custom Message Types
```go
// NATS messages as Bubble Tea messages
type heartbeatMsg struct {
    AgentID   string
    Timestamp time.Time
    Lag       time.Duration
    Status    string
}

type leaseMsg struct {
    ExpiresAt time.Time
}

type errMsg struct { err error }
```

### Pattern 3: NATS Integration
```go
// Subscribe to NATS and send messages to Bubble Tea
func subscribeHeartbeats(agentID string) tea.Cmd {
    return func() tea.Msg {
        // NATS subscription in goroutine
        // Convert NATS messages to heartbeatMsg
        // Return via channel
    }
}
```

## Task Breakdown (TDD Cycles)

### Task 1: Reconnaissance - Understand Heartbeat System (No Code)
**Approach**: Read existing code to understand heartbeat and lease mechanics

**Substeps**:
1. Read `internal/heartbeat/` package to understand:
   - How heartbeats are published to NATS
   - Message format and subjects
   - Lease renewal logic
2. Read `cmd/agentd/cmd_watch.go` current implementation (if exists)
3. Document NATS subject patterns
4. Document heartbeat message structure
5. Identify integration points for Bubble Tea

**Deliverable**: Technical notes on NATS integration approach

**Estimated Effort**: 15-20 minutes (reading + documentation)

---

### Task 2: Build `agentd watch` Model (RED Phase)
**Approach**: Subagent-driven development with TDD RED phase

**Substeps**:
1. Create `cmd/agentd/internal/tui/watch/model.go`
2. Define `watchModel` struct with all required fields
3. Implement stub methods: `Init()`, `Update()`, `View()`
4. Create `cmd/agentd/internal/tui/watch/messages.go` for custom message types
5. Write integration test `cmd/agentd/cmd_watch_test.go`:
   - Test flag registration (`--theme`, `--plain`, `--json`)
   - Test that command exists and accepts `<agent-id>` arg
   - Test help text mentions keyboard shortcuts
6. Run tests → **EXPECT FAILURES** (model not integrated yet)

**Component Usage**:
```go
import (
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/bubbles/progress"
    "github.com/charmbracelet/bubbles/spinner"
    tea "github.com/charmbracelet/bubbletea"
)

// Initialize components
viewport := viewport.New(80, 20)
progressBar := progress.New(progress.WithDefaultGradient())
loadingSpinner := spinner.New()
```

**Deliverable**:
- `internal/tui/watch/model.go` (~150 lines)
- `internal/tui/watch/messages.go` (~50 lines)
- `cmd_watch_test.go` (~40 lines)
- Tests fail (RED) ✓

**Estimated Effort**: 1 task (subagent)

---

### Task 3: Implement `agentd watch` Logic (GREEN Phase)
**Approach**: Subagent-driven development with TDD GREEN phase

**Substeps**:
1. Create `cmd/agentd/cmd_watch.go` command file
2. Add flags: `--theme`, `--plain`, `--json` (mirror Phase 2 pattern)
3. Implement `runWatch()` function:
   - Detect output mode (plain/json/rich)
   - If rich mode: Launch Bubble Tea program with `watchModel`
   - If plain mode: Simple log stream (no TUI)
   - If JSON mode: NDJSON stream
4. Implement `watchModel.Init()`:
   - Return spinner tick command
   - Return NATS subscription command
5. Implement `watchModel.Update(msg)`:
   - Handle `heartbeatMsg` → append to viewport content
   - Handle `leaseMsg` → update progress bar
   - Handle `tea.KeyMsg` → keyboard shortcuts (q=quit, r=refresh, ?=help)
   - Handle `tea.WindowSizeMsg` → resize viewport
   - Handle `spinner.TickMsg` → animate spinner until ready
6. Implement `watchModel.View()`:
   - Use `lipgloss.JoinVertical()` for layout
   - Header box (agent ID, status, uptime)
   - Viewport (heartbeat log)
   - Progress bar (lease countdown)
   - Footer (keyboard shortcuts)
7. Run tests → **EXPECT PASS** (GREEN) ✓

**Layout Implementation**:
```go
func (m watchModel) View() string {
    if !m.ready {
        return m.spinner.View() + " Connecting to agent..."
    }

    header := m.renderHeader()
    heartbeatSection := m.viewport.View()
    leaseSection := m.renderLeaseProgress()
    footer := m.renderFooter()

    return lipgloss.JoinVertical(
        lipgloss.Left,
        header,
        heartbeatSection,
        leaseSection,
        footer,
    )
}
```

**Deliverable**:
- `cmd_watch.go` (~200 lines)
- Complete `model.go` implementation (~300 lines total)
- All tests pass (GREEN) ✓

**Estimated Effort**: 2 tasks (subagent for cmd file + subagent for model logic)

---

### Task 4: Manual Testing & Polish `agentd watch`
**Approach**: Human testing with real agents

**Manual Test Commands**:
```bash
# Terminal 1: Spawn test agent
./bin/agentd spawn test-watch-alice

# Terminal 2: Watch in rich mode
./bin/agentd watch test-watch-alice

# Test keyboard shortcuts
# Press 'q' to quit
# Press '?' for help

# Test plain mode
./bin/agentd watch test-watch-alice --plain

# Test JSON mode (stream)
./bin/agentd watch test-watch-alice --json

# Test theme switching
./bin/agentd watch test-watch-alice --theme=amber
./bin/agentd watch test-watch-alice --theme=green

# Cleanup
./bin/agentd stop test-watch-alice
```

**Polish Checklist**:
- [ ] Verify heartbeats appear in real-time
- [ ] Verify progress bar updates smoothly (Harmonica spring physics)
- [ ] Verify viewport scrolls with arrow keys
- [ ] Verify 'q' quits immediately
- [ ] Verify layout adapts to terminal resize
- [ ] Verify plain mode shows simple log stream
- [ ] Verify JSON mode outputs valid NDJSON
- [ ] Verify theme colors apply correctly

**Deliverable**: Fully functional `agentd watch` command

**Estimated Effort**: 30 minutes (manual testing + adjustments)

---

### Task 5: Implement `agentd discover --watch` Flag (RED+GREEN)
**Approach**: Extend existing `agentd discover` with live refresh

**Substeps**:
1. Add `--watch` flag to `cmd/agentd/cmd_discover.go`
2. Write integration test `cmd_discover_test.go`:
   - Test `--watch` flag registration
   - Test help text mentions auto-refresh
3. Run tests → **EXPECT FAILURE** (RED)
4. Implement watch logic in `runDiscover()`:
   - If `--watch` flag: Launch Bubble Tea program
   - Otherwise: Static table (existing Phase 2 behavior)
5. Create simple Bubble Tea model `internal/tui/discover/model.go`:
   - Timer-based refresh (every 2s)
   - Reuse `render.RenderAgentList()` for table display
   - Handle 'q' to quit, 'r' to force refresh
6. Run tests → **EXPECT PASS** (GREEN)

**Implementation Pattern**:
```go
type discoverModel struct {
    agents    []render.AgentInfo
    theme     *theme.RetroTheme
    lastFetch time.Time
    quitting  bool
}

func (m discoverModel) Init() tea.Cmd {
    return tea.Batch(
        m.fetchAgents(),
        tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
            return tickMsg(t)
        }),
    )
}

func (m discoverModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tickMsg:
        // Refresh agent list every 2s
        return m, tea.Batch(
            m.fetchAgents(),
            tea.Tick(2*time.Second, tickMsg),
        )
    case agentsMsg:
        m.agents = msg
        return m, nil
    case tea.KeyMsg:
        if msg.String() == "q" {
            m.quitting = true
            return m, tea.Quit
        }
    }
    return m, nil
}
```

**Deliverable**:
- Updated `cmd_discover.go` (~50 line delta)
- `internal/tui/discover/model.go` (~150 lines)
- `cmd_discover_test.go` (~30 lines)
- All tests pass ✓

**Estimated Effort**: 1 task (subagent for discover extension)

---

### Task 6: Manual Testing & Polish `agentd discover --watch`
**Approach**: Human testing with multiple agents

**Manual Test Commands**:
```bash
# Terminal 1: Spawn multiple test agents
./bin/agentd spawn test-alice
./bin/agentd spawn test-bob
./bin/agentd spawn test-charlie

# Terminal 2: Watch in live mode
./bin/agentd discover --watch

# Verify auto-refresh every 2s
# Press 'q' to quit

# Terminal 3: Stop one agent (verify table updates)
./bin/agentd stop test-bob

# Back to Terminal 2: Verify bob disappears from table within 2s

# Test plain mode (no watch, static)
./bin/agentd discover --plain

# Cleanup
./bin/agentd stop test-alice test-charlie
```

**Polish Checklist**:
- [ ] Verify table refreshes every 2 seconds
- [ ] Verify new agents appear automatically
- [ ] Verify stopped agents disappear
- [ ] Verify 'q' quits immediately
- [ ] Verify 'r' forces immediate refresh
- [ ] Verify layout matches static `agentd list` style
- [ ] Verify theme applies correctly

**Deliverable**: Fully functional `agentd discover --watch` command

**Estimated Effort**: 20 minutes (manual testing + adjustments)

---

## Component Integration Examples

### Viewport (Scrollable Heartbeat Log)
```go
// Initialize with fixed size
viewport := viewport.New(width, height)
viewport.SetContent(heartbeatLog)

// Update content on new heartbeat
func (m watchModel) appendHeartbeat(hb heartbeatMsg) {
    newLine := fmt.Sprintf("[%s] 💓 Heartbeat (lag=%s, status=%s)",
        hb.Timestamp.Format("15:04:05"),
        hb.Lag,
        hb.Status,
    )
    m.heartbeats = append(m.heartbeats, newLine)
    m.viewport.SetContent(strings.Join(m.heartbeats, "\n"))
}
```

### Progress Bar (Lease Countdown)
```go
// Initialize with gradient
progressBar := progress.New(
    progress.WithDefaultGradient(),
    progress.WithWidth(40),
)

// Update on lease renewal
func (m watchModel) updateLease(lease leaseMsg) {
    remaining := time.Until(lease.ExpiresAt)
    total := 5 * time.Minute // Typical lease duration
    percent := float64(remaining) / float64(total)

    m.progress.SetPercent(percent)
}

// Render with label
func (m watchModel) renderLeaseProgress() string {
    remaining := time.Until(m.lease.ExpiresAt)
    label := fmt.Sprintf("Lease expires in: %s", remaining.Round(time.Second))

    return lipgloss.JoinVertical(
        lipgloss.Left,
        label,
        m.progress.View(),
    )
}
```

### Spinner (Loading Animation)
```go
// Initialize with style
spinner := spinner.New()
spinner.Spinner = spinner.Dot
spinner.Style = lipgloss.NewStyle().Foreground(theme.Primary)

// Show during connection
func (m watchModel) View() string {
    if !m.ready {
        return lipgloss.JoinHorizontal(
            lipgloss.Left,
            m.spinner.View(),
            " Connecting to agent "+m.agentID+"...",
        )
    }
    // ... normal view
}
```

### Harmonica (Smooth Lag Bar Animation)
```go
import "github.com/charmbracelet/harmonica"

type watchModel struct {
    lagBar harmonica.Spring // Smooth width transitions
}

func (m watchModel) Init() tea.Cmd {
    m.lagBar = harmonica.NewSpring(harmonica.FPS(60), 5.0, 0.5)
    return nil
}

func (m watchModel) updateLagBar(lag time.Duration) {
    targetWidth := int(lag.Seconds() * 10) // Scale lag to bar width
    m.lagBar.SetTarget(float64(targetWidth))
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.FrameMsg:
        // Advance spring physics
        m.lagBar.Update()
    }
    return m, nil
}

func (m watchModel) renderLagBar() string {
    width := int(m.lagBar.Value())
    bar := strings.Repeat("▓", width) + strings.Repeat("░", 20-width)
    return bar
}
```

## Testing Strategy

### Unit Tests
- `internal/tui/watch/model_test.go` - Test message handling
- `internal/tui/discover/model_test.go` - Test refresh logic

### Integration Tests
- `cmd_watch_test.go` - Flag registration, help text
- `cmd_discover_test.go` - `--watch` flag behavior

### Manual TUI Tests
- Terminal resizing during watch
- Keyboard shortcuts (q, r, ?)
- Performance with rapid heartbeats
- Multiple agents in discover watch mode

## Dependencies

**From Phase 2** (already available):
- `internal/detect` - Terminal detection, unicode support
- `internal/output` - Mode detection (rich/plain/json)
- `internal/theme` - Retro themes and ASCII art
- `internal/render` - Agent list rendering

**New Packages**:
- `internal/tui/watch/` - Watch command TUI model
- `internal/tui/discover/` - Discover watch mode model

**External Dependencies** (add to `go.mod` if missing):
```
github.com/charmbracelet/harmonica  # Spring physics for animations
```

## Deliverables

### Code
- [x] `cmd/agentd/cmd_watch.go` - Watch command entry point
- [x] `internal/tui/watch/model.go` - Watch TUI model
- [x] `internal/tui/watch/messages.go` - Custom message types
- [x] `internal/tui/discover/model.go` - Discover watch mode
- [x] `cmd/agentd/cmd_watch_test.go` - Integration tests
- [x] `cmd/agentd/cmd_discover_test.go` - Updated tests

### Documentation
- [x] This implementation plan
- [ ] `docs/plans/PHASE3-COMPLETION.md` - Completion report (end of phase)

## Success Criteria

- [ ] `agentd watch <agent-id>` shows live heartbeat stream
- [ ] Progress bar smoothly animates lease countdown
- [ ] Keyboard shortcuts work ('q', 'r', '?')
- [ ] `agentd discover --watch` auto-refreshes every 2s
- [ ] Plain mode outputs simple log stream (no TUI)
- [ ] JSON mode outputs valid NDJSON
- [ ] Performance: < 16ms frame time (60fps)
- [ ] No flicker or visual glitches during updates
- [ ] Terminal resize handled gracefully
- [ ] All tests pass (100%)

## Estimated Timeline

| Task | Effort | Approach |
|------|--------|----------|
| Task 1: Reconnaissance | 20 min | Manual reading |
| Task 2: Watch model (RED) | 1 task | Subagent |
| Task 3: Watch logic (GREEN) | 2 tasks | Subagent |
| Task 4: Manual testing | 30 min | Human |
| Task 5: Discover --watch | 1 task | Subagent |
| Task 6: Manual testing | 20 min | Human |
| **Total** | **~4-5 tasks** | **Mix** |

## Next Steps After Phase 3

Once Phase 3 is complete and validated:

**Phase 4: Relay Dashboard** - Full-screen mission control with:
- Multi-pane layout (Lip Gloss `JoinHorizontal`/`JoinVertical`)
- Live agent table + connection list + logs + metrics
- Bubbles `sparkline` for message rate graph
- Complex Bubble Tea model with pane navigation

Phase 3 proves the patterns needed for Phase 4's more complex TUI.

---

**Status**: Ready for execution
**Blocked By**: None (Phase 2 complete ✓)
**Next Action**: Execute Task 1 (Reconnaissance)
