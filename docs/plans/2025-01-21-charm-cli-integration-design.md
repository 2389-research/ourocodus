# Charm CLI Integration - Design Document

**Date**: 2025-01-21
**Status**: Design
**Approach**: Clean sheet rebuild with Charm ecosystem

## Vision

Transform `agentd` and `relay` into best-of-breed CLI tools with retro-computing aesthetics. Prioritize usability above all else—charm enhances, never obscures.

## Aesthetic Direction

**Retro Computing (1980s BBS/Terminal Era)**
- ASCII art and box-drawing characters (┌─┐│└─┘)
- CGA/EGA/VGA color palettes
- Vintage status messages ("CARRIER DETECTED", "PROTOCOL INITIALIZED")
- Old-school progress animations (spinners, bars with ▓▒░)

**Inspiration**: BBS splash screens, ANSI art galleries, vintage modem connections.

## Architecture

### Output Modes

Every command supports three output modes:

1. **Rich Mode** (default for TTY): Full TUI using Bubble Tea, Bubbles, Lip Gloss
2. **Plain Mode** (`--plain` or CI): Simple text output with basic colors
3. **JSON Mode** (`--json`): Machine-readable structured output

### Mode Detection

```
Priority: --json > --plain > auto-detect

Auto-detect triggers plain mode when:
- NO_COLOR or AGENTD_PLAIN environment variable set
- CI=true environment variable set
- Output is not a TTY (piped/redirected)
- Terminal too small (< 80x24)
```

### Directory Structure

```
cmd/agentd/
├── internal/
│   ├── tui/           # Bubble Tea models
│   │   ├── watch/     # Live heartbeat monitor
│   │   ├── list/      # Interactive agent table
│   │   ├── spawn/     # Wizard with Huh forms
│   │   └── common/    # Shared models (header, footer, help)
│   ├── theme/         # Retro theme system
│   │   ├── retro.go   # Color palettes (CGA, amber, green, C64)
│   │   ├── ascii.go   # Box-drawing, logos, art
│   │   └── messages.go # Vintage status messages
│   ├── render/        # Plain mode renderers
│   └── detect/        # Terminal capability detection
└── cmd_*.go          # Commands wire TUI or plain mode
```

## Theme System

### Color Palettes

Four switchable palettes (`--theme` flag):

- **CGA** (default): Bright cyan/magenta/yellow (#00FFFF, #FF00FF, #FFFF55)
- **Amber**: Monochrome amber terminal aesthetic
- **Green**: Green phosphor CRT vibes
- **C64**: Commodore 64 blue/purple palette

### Reusable Styles

```go
type RetroTheme struct {
    Primary    lipgloss.Color // CGA cyan
    Secondary  lipgloss.Color // CGA magenta
    Accent     lipgloss.Color // CGA yellow
    Success    lipgloss.Color // CGA green
    Warning    lipgloss.Color // CGA yellow
    Error      lipgloss.Color // CGA red
    Muted      lipgloss.Color // Dark gray

    Logo       lipgloss.Style // ASCII art banner
    Header     lipgloss.Style // ╔═══ COMMAND ═══╗
    BoxBorder  lipgloss.Style // ┌─┐│└─┘
    StatusBar  lipgloss.Style // Bottom status line
    Highlight  lipgloss.Style // Selected/focused items
}
```

### Message Bank

Randomized vintage messages for variety:

```go
Connecting:  "INITIALIZING PROTOCOLS", "CARRIER DETECTED"
Success:     "SYNC COMPLETE", "OPERATION NOMINAL"
Error:       "FAULT DETECTED", "PROTOCOL VIOLATION"
Loading:     "LOADING DATASTREAM", "BUFFERING"
```

### ASCII Art Library

- Ourocodus logo (small, medium, large sizes)
- Agent status icons (⚡running, ⏸paused, ✗stopped)
- Box-drawing templates for layouts

## Command Designs

### `agentd watch <agent-id>`

**Live Heartbeat Monitor** - Full-screen TUI showing real-time agent health.

**Rich Mode Features:**
- Live-scrolling heartbeat log with timestamps
- Animated lag bars (smooth transitions via Harmonica)
- Lease countdown with progress bar
- Color flashes on events (heartbeat arrival = quick pulse)
- Keyboard shortcuts: [q]uit, [r]efresh, [p]alette, [?]help

**Layout:**
```
╔══════════════════ AGENT MONITOR ══════════════════╗
║ Agent: demo-alice                    [⚡ RUNNING] ║
║ Uptime: 14m 32s        Last Signal: 847ms ago     ║
╠═══════════════════════════════════════════════════╣
║  💓 HEARTBEAT STREAM                              ║
║  ┌────────────────────────────────────────────┐  ║
║  │ [15:42:31] ▓▓▓▓▓▓▓▓░░ 823ms  STATUS: OK    │  ║
║  │ [15:42:01] ▓▓▓▓▓▓▓▓▓░ 891ms  STATUS: OK    │  ║
║  └────────────────────────────────────────────┘  ║
║  🔐 LEASE STATUS                                  ║
║  ┌────────────────────────────────────────────┐  ║
║  │ Expires in:  4m 15s  [████████░░] 85%     │  ║
║  └────────────────────────────────────────────┘  ║
╚═══════════════════════════════════════════════════╝
```

**Plain Mode:**
```
[15:42:31] 💓 Heartbeat (lag=823ms, status=OK)
[15:42:31] 🔐 Lease renewed (expires in 4m 15s)
```

**JSON Mode (streaming ndjson):**
```json
{"type":"heartbeat","agentId":"alice","timestamp":"...","lag":"823ms"}
{"type":"lease","agentId":"alice","expiresIn":"4m15s"}
```

### `relay`

**Mission Control Dashboard** - Full-screen split-pane TUI for system oversight.

**Rich Mode Features:**
- Multi-pane layout: connections, agent status, throughput, logs
- Live sparkline graph for message rate
- Scrollable agent table
- Color-coded log stream (success=green, error=red)
- Real-time NATS subscription updates
- Keyboard navigation between panes

**Layout:**
```
╔══════════════════ OUROCODUS RELAY ═══════════════╗
║  ┌─ CONNECTIONS ─┐  ┌─ AGENT STATUS ─────────┐  ║
║  │ sess-abc (PWA)│  │ alice  ⚡ RUN  2s ago   │  ║
║  │ └─ alice      │  │ bob    ⚡ RUN  1s ago   │  ║
║  │ Active: 3     │  │ Total: 4  Running: 2   │  ║
║  └───────────────┘  └────────────────────────┘  ║
║  ┌─ THROUGHPUT ────────────────────────────┐    ║
║  │ [▂▃▅▇█▇▅▃▂▁] 24 msg/s  Peak: 67 msg/s  │    ║
║  └──────────────────────────────────────────┘    ║
║  ┌─ SYSTEM LOG ────────────────────────────┐    ║
║  │ [15:42:31] CARRIER DETECTED - sess-abc  │    ║
║  │ [15:42:15] AGENT SPAWN - alice          │    ║
║  └──────────────────────────────────────────┘    ║
╠══════════════════════════════════════════════════╣
║ [q]uit  [a]gents  [l]ogs  [m]etrics  [?]help    ║
╚══════════════════════════════════════════════════╝
```

**Plain Mode:** Simple log output with periodic status summaries.

### `agentd list` & `agentd discover`

**Interactive Agent Tables** - Browse agents with keyboard navigation.

**Rich Mode Features:**
- Arrow keys navigate rows
- Enter shows agent details (expand/collapse)
- 's' cycles sort modes (name, status, age)
- Color coding: running=green, idle=yellow, stopped=red
- Auto-refresh every 2s in watch mode

**Layout:**
```
╔═══════════════ AGENT ROSTER ═══════════════╗
║  AGENT    STATUS      SOURCE   ATTACHED TO ║
║  ──────────────────────────────────────────║
║  alice    ⚡ running  cli      sess-abc... ║
║  bob      ⚡ running  relay    sess-def... ║
║  Total: 4  |  2 running  |  1 idle         ║
╠════════════════════════════════════════════╣
║ [↑↓] navigate  [enter] details  [q]uit    ║
╚════════════════════════════════════════════╝
```

**Difference:**
- `list`: Shows only your spawned agents
- `discover`: Shows all agents (CLI + relay-spawned)

**JSON Mode:**
```json
{
  "agents": [
    {
      "agentId": "alice",
      "status": "running",
      "containerId": "abc123...",
      "workspace": "/path/to/worktree",
      "spawnSource": "cli",
      "attachedTo": "sess-abc...",
      "createdAt": "2025-01-21T15:42:00Z"
    }
  ],
  "summary": {
    "total": 4,
    "running": 2
  }
}
```

### `agentd spawn`

**Interactive Wizard** - Guided agent creation with Huh forms.

**When run without args**, launch wizard with:
1. Welcome screen (ASCII art logo)
2. Agent ID input (validation)
3. Image selection (list or custom)
4. Environment variables (multi-input)
5. API key (env or manual)
6. Confirmation screen
7. Progress animation (spinner + steps)
8. Success screen (agent details)

**Progress Animation:**
```
╔═══════════════════════════════════════════╗
║  SPAWNING: demo-alice                     ║
║  [████████████░░░░░░░░] 67%              ║
║  ✓ Created worktree                       ║
║  ✓ Mounted credentials                    ║
║  ⟳ Building container...                  ║
╚═══════════════════════════════════════════╝
```

**With args** (existing behavior): `agentd spawn <id> --flags`

### Remaining Commands

**`agentd doctor`**: Styled health checks with visual status indicators.

**`agentd stop`**: Huh confirmation prompts before destructive actions.

**`agentd logs`**: Bubbles viewport with syntax-highlighted output.

## Implementation Strategy

### Phased Rollout

**Phase 1: Foundation**
- Add Charm dependencies (bubble tea, bubbles, lipgloss, huh, log)
- Build `internal/theme/` with palettes and ASCII art
- Create `internal/detect/` for terminal detection
- Implement mode switching (rich/plain/json)
- **Deliverable**: Theme system and mode detection work
- **Blocks**: All other phases

**Phase 2: Pilot Command**
- Rebuild `agentd list` with full Charm integration
- Implement all three modes (rich/plain/json)
- **Deliverable**: Proof of concept validates architecture
- **Critical**: Pattern must work before proceeding

**Phase 3: Live Commands**
- Build `agentd watch` TUI
- Build `agentd discover --watch` live table
- Test Bubble Tea event handling and performance
- **Deliverable**: Interactive monitoring works
- **Requires**: Phase 2 complete

**Phase 4: Relay Dashboard**
- Build `relay` full-screen TUI
- Implement multi-pane layout
- Integrate NATS for real-time updates
- **Deliverable**: Command center experience
- **Requires**: Phase 3 complete (complex TUI proven)

**Phase 5: Interactive Wizard**
- Build `agentd spawn` wizard with Huh
- Add progress animations (Bubbles spinner)
- Create success screen
- **Deliverable**: Guided agent creation
- **Requires**: Phase 2 complete

**Phase 6: Polish**
- Complete `agentd doctor`, `stop`, `logs`
- Add theme switching (`--theme`)
- Final testing and documentation
- **Deliverable**: Complete Charm-based CLI

### Testing Strategy

- **Unit tests**: Theme and detect packages
- **Integration tests**: Run in plain mode (CI)
- **Manual TUI testing**: Different terminals (iTerm, Terminal.app, Kitty)
- **Performance testing**: Bubble Tea rendering, large datasets

### Backwards Compatibility

Plain mode maintains current output format exactly. Existing scripts and automation continue working without modification.

## Dependencies

```
github.com/charmbracelet/bubbletea    # TUI framework
github.com/charmbracelet/bubbles      # Pre-built components
github.com/charmbracelet/lipgloss     # Styling
github.com/charmbracelet/huh          # Interactive forms
github.com/charmbracelet/log          # Structured logging
github.com/charmbracelet/harmonica    # Spring physics (animations)
```

Keep existing:
```
github.com/spf13/cobra                # CLI framework (keep)
```

## Success Criteria

- All commands support rich/plain/json modes
- Plain mode output identical to current (no breaking changes)
- TUI works in common terminals (iTerm, Terminal.app, Kitty, Alacritty)
- CI tests pass in plain mode
- Performance acceptable (< 16ms frame time for 60fps)
- Retro aesthetic consistent across all commands
- Users say "whoa, this is cool!" when they see it
