# agentd Architecture Review & Maximal Cleanup Plan

**Date:** 2025-11-24
**Scope:** `cmd/agentd/` and `cmd/relay/` binaries
**Goal:** Clean, crisp, pragmatic, elegant code following industry gold standards

---

## Executive Summary

The `agentd` CLI and relay server have accumulated technical debt through:
- Inconsistent patterns across commands
- Code duplication (time formatting, Docker client, rainbow logos)
- Mixed styling libraries (fatih/color + lipgloss)
- Dead/unused commands (`discover` duplicates `list`)
- Broken tests blocking CI
- NATS configuration inconsistency between agentd and relay

This document provides a **MAXIMAL cleanup plan** to address all issues comprehensively.

**Severity Ratings:**
- **P0 (Critical):** Broken tests, inconsistent behavior
- **P1 (High):** Must fix - significant code quality issues
- **P2 (Medium):** Should fix - maintainability issues
- **P3 (Low):** Could fix - style improvements

---

## Current Architecture

### File Structure
```
cmd/agentd/
├── main.go              # Root command, adds subcommands
├── labels.go            # Container label constants + Version
├── docker.go            # findAgentContainerID helper
├── launcher.go          # Container launcher factory
├── utils.go             # Time formatting utilities
├── cmd_doctor.go        # Environment validation
├── cmd_spawn.go         # Create isolated agent
├── cmd_list.go          # List active agents
├── cmd_discover.go      # Discover agents with adoption status
├── cmd_watch.go         # Watch heartbeats/lease events
├── cmd_stop.go          # Stop and cleanup agents
├── cmd_logs.go          # Stream container logs
├── cmd_attach.go        # Interactive shell
├── cmd_send.go          # Execute single command
├── cmd_repl.go          # Interactive REPL via ACP
└── internal/
    ├── README.md
    ├── detect/          # Terminal capability detection
    ├── output/          # Output mode management
    ├── render/          # Table rendering
    ├── theme/           # Retro color themes
    └── tui/
        ├── list/        # Bubble Tea list dashboard
        └── repl/        # Bubble Tea REPL interface
```

### Command Summary
| Command | Output Modes | TUI Support | Tests |
|---------|-------------|-------------|-------|
| doctor | Rich only | No | No |
| spawn | Rich/Plain/JSON | No | Yes |
| list | Rich/Plain/JSON | Yes (Bubble Tea) | Yes |
| discover | Table/JSON | Yes (watch mode) | No |
| watch | Rich/Plain/JSON | No | No |
| stop | Rich/Plain/JSON | No | Yes (broken) |
| logs | Rich only | No | No |
| attach | Rich only | No | Yes (broken) |
| send | Rich only | No | Yes |
| repl | Rich (TUI) | Yes (Bubble Tea) | Yes |

---

## Issues Identified

### P0: Critical - Broken Tests

**Issue:** `stopAgent` function was renamed to `stopAgentWithMode` but test files still reference the old function.

**Location:**
- `cmd/agentd/integration_test.go`
- `cmd/agentd/cmd_integration_test.go`

**Impact:** CI cannot pass, linter reports compilation errors.

**Files with broken references:**
```go
// Old signature (expected by tests):
func stopAgent(ctx context.Context, cmd *cobra.Command, agentID string) error

// New signature (current):
func stopAgentWithMode(ctx context.Context, agentID string, mode output.Mode) StopResult
```

**Recommendation:** Add a test-compatible wrapper:
```go
// stopAgent provides a test-compatible interface for stopping agents.
// For production use, see stopAgentWithMode.
func stopAgent(ctx context.Context, _ *cobra.Command, agentID string) error {
    result := stopAgentWithMode(ctx, agentID, output.ModePlain)
    if result.Status == "failed" {
        return fmt.Errorf(result.Error)
    }
    return nil
}
```

---

### P0: Critical - NATS Configuration Inconsistency

**Issue:** Relay and agentd have inconsistent NATS default behavior:
- **relay**: NATS disabled unless `NATS_URL` env var is explicitly set
- **agentd list/watch**: Default to `nats://localhost:4222`

**Impact:** User runs relay, sees "NATS: disabled", but agentd commands try to connect to localhost:4222.

**Recommendation:** Make relay default to `nats://localhost:4222` with graceful fallback:
```go
// initializeNATS creates a NATS client, defaulting to localhost if not configured
func initializeNATS() nats.Client {
    natsURL := os.Getenv("NATS_URL")
    if natsURL == "" {
        natsURL = "nats://localhost:4222" // Default to localhost
    }

    log.Printf("[NATS] Connecting to NATS at %s...", redactNATSURL(natsURL))

    natsClient, err := nats.NewClient(
        nats.WithURL(natsURL),
        nats.WithName("ourocodus-relay"),
    )
    if err != nil {
        // Graceful fallback - NATS is optional
        log.Printf("[NATS] Connection failed (NATS features disabled): %v", err)
        return nil
    }
    log.Printf("[NATS] Connected successfully")
    return natsClient
}
```

---

### P1: High - Code Duplication

#### 1. Time Formatting Functions (5+ implementations)

**Locations:**
- `utils.go`: `formatDuration`, `formatDurationWithoutSuffix`, `formatDurationWithSuffix`
- `render/list.go`: `formatDuration`, `formatLastBeat`
- `tui/list/list.go`: `formatAge`, `formatLastBeat`
- `cmd_watch.go`: `formatWatchDuration`
- `cmd_discover.go`: Uses `formatDuration` from utils but also has its own formatting

**Recommendation:** Create `internal/format/duration.go`:
```go
package format

// Duration formats a duration for display with configurable options.
type DurationFormatter struct {
    ShowSuffix bool
    Suffix     string // e.g., " ago", ""
    Precision  Precision
}

type Precision int

const (
    PrecisionAuto Precision = iota
    PrecisionSeconds
    PrecisionMinutes
)

// Format returns human-readable duration string.
func (f DurationFormatter) Format(d time.Duration) string { ... }

// Convenience functions
func Ago(d time.Duration) string { ... }
func Since(t time.Time) string { ... }
func LastBeat(t time.Time) string { ... }
```

#### 2. Rainbow Logo Rendering (2 implementations)

**Locations:**
- `render/list.go`: Lines 108-126
- `tui/list/list.go`: Lines 291-308

Both have identical color arrays and nearly identical logic.

**Recommendation:** Move to `theme/ascii.go`:
```go
func RenderRainbowLogo(text string) string { ... }
```

#### 3. Docker Client Creation (10+ locations)

**Pattern repeated:**
```go
cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
if err != nil {
    return ..., err
}
defer func() { _ = cli.Close() }()
```

**Recommendation:** Create shared client accessor in `internal/docker/client.go`:
```go
package docker

var defaultClient *client.Client
var clientOnce sync.Once

// Client returns a shared Docker client instance.
func Client() (*client.Client, error) {
    var initErr error
    clientOnce.Do(func() {
        defaultClient, initErr = client.NewClientWithOpts(
            client.FromEnv,
            client.WithAPIVersionNegotiation(),
        )
    })
    return defaultClient, initErr
}
```

---

### P1: High - Inconsistent Output Mode Handling

**Current State:**
| Command | Flag Pattern |
|---------|-------------|
| list | `--format auto\|rich\|plain\|json` + `--plain` shortcut |
| spawn | `--json` + `--plain` |
| stop | `--json` + `--plain` |
| watch | `--json` + `--plain` |
| discover | `--format table\|json` + `--watch` |
| doctor, logs, attach, send | No output flags |

**Recommendation:** Standardize on `--json` and `--plain` flags:
```go
// Add to all commands via shared flag setup
func addOutputFlags(cmd *cobra.Command, jsonFlag *bool, plainFlag *bool) {
    cmd.Flags().BoolVar(jsonFlag, "json", false, "Output in JSON format")
    cmd.Flags().BoolVar(plainFlag, "plain", false, "Output in plain text (no colors)")
}
```

Remove `--format` flag from `list` and `discover` for consistency.

---

### P1: High - Label Definition Redundancy

**Issue:** Labels defined in two places:
1. `cmd/agentd/labels.go` - Local constants
2. `pkg/labels/labels.go` - Centralized package

The comment in `labels.go` says "must match pkg/..." but they're maintained separately.

**Recommendation:** Remove local definitions, import from pkg:
```go
import "github.com/2389-research/ourocodus/pkg/labels"

// Use labels.AgentID, labels.Workspace, etc.
```

---

### P2: Medium - Theme/Color Library Fragmentation

**Current State:**
- `fatih/color`: Used in doctor, spawn, stop, discover, logs, attach, send commands
- `lipgloss`: Used in internal/theme, render, tui packages

**Impact:** Inconsistent styling, harder to maintain, larger binary size.

**Recommendation:** Migrate to lipgloss only:
1. Replace `color.New()` calls with lipgloss styles
2. Use theme system consistently across all commands
3. Remove `fatih/color` dependency

Example migration:
```go
// Before (fatih/color)
_, _ = color.New(color.FgGreen).Printf("✓ %s\n", msg)

// After (lipgloss via theme)
successStyle := lipgloss.NewStyle().Foreground(th.Success)
fmt.Println(successStyle.Render("✓ " + msg))
```

---

### P1: High - Dead Command: `discover`

**Issue:** The `discover` command is redundant and must be removed:
- Duplicates 90% of `list` functionality
- Both query Docker for agent containers
- Both show agent status, workspace, spawn source
- Both support JSON output
- Both can watch/refresh

The only unique feature is "adoption status" which can be shown in `list`.

**Action:** DELETE `cmd/agentd/cmd_discover.go` entirely.

**Migration:**
1. Add `--adoption` flag to `list` to show attached/discovered status
2. Remove `discover` from `main.go` command registration
3. Delete `cmd_discover.go`

---

### P1: High - Mixed Styling Libraries (fatih/color)

**Issue:** Two color libraries used side-by-side:
- `fatih/color`: Used in 8 command files (36 usages)
- `lipgloss`: Used in internal/theme and TUI components

**Files using fatih/color:**
- `main.go` (2 usages - bannerColor, taglineColor)
- `cmd_doctor.go` (7 usages)
- `cmd_spawn.go` (3 usages)
- `cmd_stop.go` (1 usage)
- `cmd_discover.go` (9 usages) - DELETE with command
- `cmd_attach.go` (4 usages)
- `cmd_send.go` (7 usages)
- `cmd_repl.go` (4 usages)

**Action:** Remove ALL `fatih/color` usage. Use lipgloss via internal/theme exclusively.

**Migration Pattern:**
```go
// BEFORE (fatih/color)
_, _ = color.New(color.FgGreen).Printf("✓ %s\n", msg)
_, _ = color.New(color.FgCyan, color.Bold).Printf("Header: %s\n", text)

// AFTER (lipgloss via theme)
th := theme.NewRetroTheme(theme.PaletteCGA)
successStyle := lipgloss.NewStyle().Foreground(th.Success)
headerStyle := lipgloss.NewStyle().Foreground(th.Primary).Bold(true)
fmt.Println(successStyle.Render("✓ " + msg))
fmt.Println(headerStyle.Render("Header: " + text))
```

After migration, remove from `go.mod`:
```
github.com/fatih/color v1.x.x
```

---

### P2: Medium - Dependency Injection Boilerplate

**Issue:** `launcher.go` creates 3 stub types to satisfy interfaces:
```go
type defaultIDGenerator struct{}
type defaultClock struct{}
type defaultLogger struct{}
```

**Recommendation:** Use function types or simplify interfaces:
```go
// Option 1: Function types in containersession package
type IDGeneratorFunc func() string
type ClockFunc func() time.Time
type LoggerFunc func(format string, v ...any)

// Option 2: Accept concrete types with defaults in NewManager
func NewManager(dockerClient *client.Client, opts ...Option) *Manager
```

---

### P3: Low - Unused/Dead Code

1. **`SupportsUnicode()`** in `detect/detect.go` - Only used in one place, could be inlined or removed if not needed elsewhere.

2. **`formatSpawnSource()`** in `cmd_list.go` - Returns input unchanged for most cases:
```go
func formatSpawnSource(source string) string {
    switch source {
    case "cli":
        return "cli"      // No transformation
    case "relay":
        return "relay"    // No transformation
    // ...
    }
}
```

3. **`formatStateString()`** in `cmd_list.go` - Comment says "kept for tests" but tests should use their own helpers.

**Recommendation:** Remove unless actively used. Add `//nolint:unused` comment if intentionally preserved for future use.

---

### P3: Low - Missing Tests

Commands without test coverage:
- `doctor`
- `discover`
- `watch`
- `logs`

**Recommendation:** Add at least smoke tests for each command to catch regressions.

---

## Maximal Remediation Plan

This is an aggressive, comprehensive cleanup. Execute in order.

### Phase 1: Fix Broken Tests (P0) - 30 min
**Goal:** Unblock CI immediately

1. Add `stopAgent` wrapper function in `cmd_stop.go`:
   ```go
   func stopAgent(ctx context.Context, _ *cobra.Command, agentID string) error {
       result := stopAgentWithMode(ctx, agentID, output.ModePlain)
       if result.Status == "failed" {
           return fmt.Errorf("%s", result.Error)
       }
       return nil
   }
   ```
2. Verify: `go test ./cmd/agentd/...`

### Phase 2: Fix NATS Default (P0) - 15 min
**Goal:** Consistent behavior between relay and agentd

1. Update `cmd/relay/main.go` `initializeNATS()`:
   - Default to `nats://localhost:4222`
   - Log graceful fallback on connection failure
   - Don't `log.Fatalf` on NATS error (it's optional)

### Phase 3: Delete discover Command (P1) - 30 min
**Goal:** Remove dead code

1. Delete `cmd/agentd/cmd_discover.go`
2. Remove from `main.go`: `rootCmd.AddCommand(discoverCmd)`
3. If adoption status needed, add `--adoption` flag to `list` later

### Phase 4: Eliminate fatih/color (P1) - 2-3 hours
**Goal:** Single styling library (lipgloss only)

**Order of migration:**
1. `main.go` - Replace bannerColor/taglineColor with lipgloss
2. `cmd_doctor.go` - Largest file, migrate successColor/errorColor/infoColor
3. `cmd_spawn.go` - 3 usages
4. `cmd_stop.go` - 1 usage
5. `cmd_attach.go` - 4 usages
6. `cmd_send.go` - 7 usages
7. `cmd_repl.go` - 4 usages
8. `cmd_logs.go` - Uses infoColor from cmd_doctor.go

**After all migrations:**
```bash
go mod tidy  # Remove fatih/color
```

### Phase 5: Consolidate Utilities (P1) - 2 hours
**Goal:** Single source of truth for common functions

1. Create `internal/format/duration.go`:
   - Consolidate all `formatDuration` variants
   - Export: `Ago()`, `Since()`, `LastBeat()`, `Compact()`

2. Create `internal/docker/client.go`:
   - Shared Docker client with singleton pattern
   - Export: `Client() (*client.Client, error)`

3. Move rainbow logo to `internal/theme/rainbow.go`:
   - Single `RenderRainbowLogo(text string) string` function
   - Used by render/list.go and tui/list/list.go

### Phase 6: Standardize Output Flags (P1) - 1.5 hours
**Goal:** Consistent flags across all commands

1. Create `internal/flags/output.go`:
   ```go
   func AddOutputFlags(cmd *cobra.Command) (json *bool, plain *bool) {
       j := cmd.Flags().Bool("json", false, "Output in JSON format")
       p := cmd.Flags().Bool("plain", false, "Output in plain text (no colors)")
       return j, p
   }
   ```

2. Migrate commands:
   - `list`: Remove `--format`, use `--json`/`--plain`
   - `doctor`: Add `--json`/`--plain`
   - `logs`: Add `--plain`
   - `attach`: Add `--plain`
   - `send`: Add `--json`/`--plain`

### Phase 7: Clean Up Labels (P1) - 30 min
**Goal:** Single source of truth for labels

1. In `cmd/agentd/labels.go`:
   - Keep only `Version = "0.1.0"`
   - Remove all Label* constants
2. Import labels from `pkg/labels` in all command files
3. Verify label values match

### Phase 8: Remove Dead Code (P2) - 1 hour
**Goal:** No unused code

1. Remove `formatSpawnSource()` from `cmd_list.go` (identity function)
2. Remove `formatStateString()` from `cmd_list.go` (test helper in prod code)
3. Audit `SupportsUnicode()` usage - inline or keep
4. Remove any orphaned helper functions

### Phase 9: Simplify Dependency Injection (P2) - 1 hour
**Goal:** Less boilerplate

1. In `launcher.go`, consider using function types:
   ```go
   // Instead of 3 struct types, use functional options or defaults
   type ManagerOptions struct {
       IDGenerator func() string
       Clock       func() time.Time
       Logger      func(string, ...any)
   }
   ```
2. Or accept this is fine as-is (explicit is good)

### Phase 10: Final Cleanup (P3) - 30 min
**Goal:** Polish

1. Run full linter: `golangci-lint run ./cmd/agentd/...`
2. Run full linter: `golangci-lint run ./cmd/relay/...`
3. Verify all tests pass
4. Update documentation if needed

---

## Success Criteria

After remediation:
- [ ] All tests pass: `go test ./cmd/agentd/...`
- [ ] All tests pass: `go test ./cmd/relay/...`
- [ ] Linter clean: `golangci-lint run ./...`
- [ ] `discover` command deleted
- [ ] Zero `fatih/color` imports (grep finds nothing)
- [ ] Single source of truth for utilities (format, docker, theme)
- [ ] Consistent `--json`/`--plain` flags on all commands
- [ ] Labels imported from `pkg/labels` only
- [ ] NATS defaults to localhost:4222 with graceful fallback

---

## Estimated Total Effort

| Phase | Task | Time |
|-------|------|------|
| 1 | Fix broken tests | 30 min |
| 2 | Fix NATS default | 15 min |
| 3 | Delete discover command | 30 min |
| 4 | Eliminate fatih/color | 2-3 hours |
| 5 | Consolidate utilities | 2 hours |
| 6 | Standardize output flags | 1.5 hours |
| 7 | Clean up labels | 30 min |
| 8 | Remove dead code | 1 hour |
| 9 | Simplify DI | 1 hour |
| 10 | Final cleanup | 30 min |
| **Total** | | **~10-12 hours** |

---

## Appendix: Industry Best Practices Applied

### Cobra CLI Standards
- [x] Use of `Use`, `Short`, `Long`, `Example` fields
- [x] Proper argument validation with `Args`
- [x] Context propagation via `cmd.Context()`
- [ ] **Fix:** Consistent flag patterns across commands

### 12-Factor CLI Principles
- [x] Configuration via environment variables
- [x] Explicit output modes for scripting
- [ ] **Fix:** Structured logging (currently ad-hoc)

### Go Project Layout
- [x] Binary code in `cmd/`
- [x] Internal packages properly scoped
- [x] Shared packages in `pkg/`
- [ ] **Consider:** Some internal code could move to pkg/ for reuse

### Charm Best Practices
- [x] Bubble Tea for interactive TUI
- [x] Lipgloss for styling
- [ ] **Fix:** Remove fatih/color entirely

### Testing Standards
- [x] Table-driven tests where appropriate
- [ ] **Fix:** Integration test coverage
- [ ] **Fix:** Broken test references

---

## Commands After Cleanup

Final command surface (9 commands, down from 10):

```
agentd
├── doctor    # Validate environment
├── spawn     # Create isolated agent
├── list      # List active agents (--adoption for adoption status)
├── watch     # Watch heartbeats/lease events
├── stop      # Stop and cleanup agents
├── logs      # Stream container logs
├── attach    # Interactive shell
├── send      # Execute single command
└── repl      # Interactive REPL via ACP
```

All commands support:
- `--json` - Machine-readable JSON output
- `--plain` - Plain text without colors (CI-friendly)
