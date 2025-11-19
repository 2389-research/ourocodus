# agentd MVP Design & Implementation Plan

**Status**: Approved for Implementation
**Target Demo Date**: Friday (5-day sprint)
**Consensus Validation**: Completed (gpt-5, gpt-5-mini, gpt-5-nano)
**Overall Confidence**: 8.5/10

## Executive Summary

**agentd** is a CLI tool that demonstrates Ourocodus's three-layer isolation architecture (git worktrees + Docker containers + credential isolation) to technical developers. The MVP is a stateless wrapper around `pkg/agent/container.AgentContainerLauncher` that enables spawning, managing, and observing multiple isolated AI coding agents through simple commands.

**Key Value**: Makes abstract isolation architecture concrete and observable through a 4-minute demo showing concurrent agents with visible workspace isolation, container boundaries, and credential scoping.

## Product Vision

### What We're Building

A developer-focused CLI tool that showcases:
- **Git worktree isolation** - Each agent works in a separate branch/directory
- **Docker container isolation** - Each agent runs in its own containerized environment
- **Credential isolation** - Each agent has scoped access via Docker secrets/volumes

### Target Audience

Technical developers who appreciate clean architecture and want to understand how multi-agent systems handle concurrent work without conflicts.

### Success Criteria

- ✅ Demo runs without errors in 4 minutes
- ✅ Clearly shows three-layer isolation
- ✅ Technical audience understands architecture immediately
- ✅ No manual Docker commands needed

## Architecture

### Core Design Principles

1. **Zero State** - No persistent state; query Docker/filesystem directly
2. **Label-Based Discovery** - Use Docker labels for lifecycle management
3. **Fail-Fast** - Doctor command validates environment before operations
4. **Idempotent Operations** - Safe to retry commands
5. **Secure-by-Default** - No implicit credential sharing
6. **Subtle Delight** - Visual elements teach architecture, minimal decoration (useful over all, sprinkle joy)

### System Architecture

```
┌─────────────────────────────────────────────────────┐
│                  agentd CLI                         │
│              (Cobra Framework)                      │
└──────────────┬──────────────────────────────────────┘
               │
               │ delegates to
               │
┌──────────────▼──────────────────────────────────────┐
│      pkg/agent/container.AgentContainerLauncher     │
│                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────┐ │
│  │  Worktree    │  │  Container   │  │  Creds    │ │
│  │  Manager     │  │  Session Mgr │  │  Mounter  │ │
│  └──────────────┘  └──────────────┘  └───────────┘ │
└─────────────────────────────────────────────────────┘
               │
               │ creates
               │
┌──────────────▼──────────────────────────────────────┐
│          Isolated Agent Environment                  │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │ Docker Container: agentd-<shortid>          │   │
│  │ Labels: org.ourocodus.agentd=true           │   │
│  │         agentd.id=<id>                      │   │
│  │         agentd.repo=<hash>                  │   │
│  │         agentd.worktree=<path>              │   │
│  │                                              │   │
│  │ Mounts:                                      │   │
│  │   /workspace → .agentd/worktrees/<id>       │   │
│  │   /root/.creds → (Docker secret/volume)     │   │
│  └──────────────────────────────────────────────┘   │
│                                                      │
│  Git Worktree: .agentd/worktrees/<id>              │
│  Branch: agentd/<id>-<shortid>                      │
└─────────────────────────────────────────────────────┘
```

### Label Schema

All agentd-managed containers carry these labels:

```go
const (
    LabelNamespace     = "org.ourocodus.agentd"
    LabelAgentID       = "agentd.id"
    LabelRepoHash      = "agentd.repo"
    LabelWorktreePath  = "agentd.worktree"
    LabelVersion       = "agentd.version"
)

labels := map[string]string{
    LabelNamespace:    "true",
    LabelAgentID:      agentID,
    LabelRepoHash:     repoHash,
    LabelWorktreePath: worktreePath,
    LabelVersion:      "0.1.0",
}
```

### Credential Isolation Strategy

**Implementation**: Docker secrets or per-agent volumes (NOT environment variables)

```bash
# Mount credential files as read-only volumes
docker run \
  -v /host/creds/agent-id:/root/.creds:ro \
  -l org.ourocodus.agentd=true \
  -l agentd.id=alice \
  ourocodus/agent:latest
```

**Rationale**: Demonstrates proper isolation boundaries using Docker primitives.

## Commands

### `agentd doctor`

**Purpose**: Validate environment before operations

**Checks**:
- Docker daemon running + version validation
- File-sharing permissions (macOS specific)
- Image presence/pull (ourocodus/agent:latest)
- Git worktree support
- Disk space (minimum 1GB free)
- **Spawn smoke test** - Create/destroy test container to validate full lifecycle

**Output**:
```
✓ Docker daemon running (v27.4.1)
✓ File sharing enabled: /Users/clint/code
✓ Image present: ourocodus/agent:latest
✓ Git worktree support confirmed
✓ Disk space: 5.2GB available
✓ Spawn smoke test passed
Environment ready!
```

**Exit codes**:
- 0: All checks passed
- 1: One or more checks failed (prints actionable error messages)

### `agentd spawn <agent-id>`

**Purpose**: Create isolated agent environment

**Flow**:
1. Generate agent ID if not provided (format: `agent-<shortid>`)
2. Create git worktree at `.agentd/worktrees/<id>`
3. Set up credential files/volumes
4. Launch Docker container with labels and mounts
5. Print agent details (ID, worktree path, container name, credentials)

**Flags**:
- `--workspace <path>` - Custom worktree path (default: `.agentd/worktrees/<id>`)
- `--image <name>` - Custom Docker image (default: `ourocodus/agent:latest`)
- `--env KEY=VALUE` - Additional environment variables

**Output**:
```
Creating isolated agent 'alice'...

🌳 Worktree: .agentd/worktrees/alice (branch: agentd/alice-a1b2c3)
📦 Container: agentd-a1b2c3 (running)
🔑 Credentials: mounted at /root/.creds (read-only)

✓ Agent alice ready
```

**Error handling**:
- Agent ID already exists → Exit with error
- Worktree creation fails → Cleanup and exit
- Container start fails → Cleanup worktree and exit

### `agentd list`

**Purpose**: Show all active agents

**Implementation**: Query Docker for containers with `org.ourocodus.agentd=true` label

**Output**:
```
AGENT  STATUS   WORKSPACE                  CONTAINER      CREATED
alice  running  .agentd/worktrees/alice    agentd-a1b2c3  2m ago
bob    running  .agentd/worktrees/bob      agentd-d4e5f6  1m ago
```

**Flags**:
- `--format <json|table>` - Output format (default: table)

### `agentd stop <agent-id>`

**Purpose**: Graceful cleanup of agent resources

**Flow**:
1. Stop Docker container (graceful shutdown)
2. Remove git worktree
3. Clean credential files/volumes
4. Remove handle from tracking

**Idempotent**: Safe to call multiple times (no-op if agent doesn't exist)

**Output**:
```
Stopping agent 'alice'...

✓ Stopped container agentd-a1b2c3
✓ Removed worktree .agentd/worktrees/alice
✓ Cleaned credentials
```

### `agentd logs <agent-id>`

**Purpose**: Stream container logs in real-time

**Implementation**: `docker logs -f` by container ID/name

**Flags**:
- `--follow` / `-f` - Follow log output (default: true)
- `--tail <n>` - Show last N lines (default: all)

**Output**:
```
[alice] ACP server listening on stdio...
[alice] Workspace: /workspace (isolated)
[alice] Credentials: /root/.creds
```

## Configuration (Optional)

### `.agentd.yml`

**Purpose**: Provide sensible defaults for demo smoothness

**Location**: Repository root (`.agentd.yml`) or `~/.agentd.yml`

**Read-only**: CLI reads defaults but allows flag overrides

**Example**:
```yaml
# agentd configuration (read-only defaults)
image: "ourocodus/agent:latest"
workspace_base: ".agentd/worktrees"
entrypoint: ["/usr/local/bin/claude-code-acp"]
command: ["server"]

env:
  # Credentials passed from host environment
  ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"

# Docker options
docker:
  auto_remove: false
  log_driver: json-file
```

**Priority**: CLI flags > .agentd.yml > hardcoded defaults

**Implementation**: Optional for MVP (Day 4 feature if time permits)

## Implementation Plan

### Day 1 (Monday): Foundation

**Goals**:
- [ ] Cobra CLI scaffold
- [ ] Hardened doctor command
- [ ] Label schema constants
- [ ] Validate pkg/ API

**Deliverables**:
- `cmd/agentd/main.go` - Root command
- `cmd/agentd/cmd_doctor.go` - Doctor command with all checks
- `cmd/agentd/labels.go` - Label schema constants
- `agentd doctor` passes all checks

**Time estimate**: 8 hours
**Risk level**: Low

### Day 2 (Tuesday): Spawn Command

**Goals**:
- [ ] Implement spawn command
- [ ] Docker secrets credential isolation
- [ ] Worktree creation
- [ ] Container lifecycle

**Deliverables**:
- `cmd/agentd/cmd_spawn.go` - Spawn implementation
- `agentd spawn alice` creates isolated agent

**Time estimate**: 8 hours
**Risk level**: Medium (expect macOS Docker mount debugging)

### Day 3 (Wednesday): List & Stop Commands

**Goals**:
- [ ] Implement list command (label-based query)
- [ ] Implement stop command (graceful cleanup)
- [ ] MVP core complete

**Deliverables**:
- `cmd/agentd/cmd_list.go` - List implementation
- `cmd/agentd/cmd_stop.go` - Stop implementation
- Full lifecycle works end-to-end

**Time estimate**: 6 hours
**Risk level**: Low

**Milestone**: MVP core complete - can spawn, list, and stop agents

### Day 4 (Thursday): Polish & Optional Features

**Goals** (pick based on progress):
- [ ] Implement logs command (if time permits)
- [ ] OR implement .agentd.yml config (if time permits)
- [ ] Create demo setup script
- [ ] Error message polish

**Deliverables**:
- `cmd/agentd/cmd_logs.go` OR `cmd/agentd/config.go`
- `scripts/demo-setup.sh` - Pre-create demo state
- Polished error messages

**Time estimate**: 6 hours
**Risk level**: Low

### Day 5 (Friday): Demo Prep

**Goals**:
- [ ] Demo script finalization
- [ ] Record fallback screencast
- [ ] Help text polish
- [ ] Rehearse demo

**Deliverables**:
- 4-minute demo script
- Fallback screencast recording
- Polished help text and colored output

**Time estimate**: 4 hours
**Risk level**: Low

## Risk Management

### Critical Risks & Mitigations

| Risk | Severity | Probability | Mitigation |
|------|----------|-------------|------------|
| **macOS Docker mount permissions** | HIGH | 60% | Hardened doctor with file-sharing check + spawn smoke test |
| **pkg/ API lacks needed hooks** | HIGH | 40% | Day 1 validation task; budget adapter layer |
| **Demo failure on Friday** | HIGH | 30% | Pre-recorded fallback screencast |
| **Credential isolation complexity** | MEDIUM | 50% | Use concrete Docker secrets/volumes (not env vars) |
| **Log streaming edge cases** | MEDIUM | 40% | Treat logs as optional Day 4 feature |
| **Timeline overrun** | MEDIUM | 50% | Hard stop at Day 3 for MVP core; Days 4-5 flex only |

### Contingency Plans

**If Day 2-3 overrun**:
- Drop logs command → defer to post-MVP
- Drop config support → use hardcoded defaults
- Deliver spawn/list/stop/doctor only

**If demo environment has issues**:
- Use pre-recorded fallback screencast
- Have demo setup script ready (pre-pull images, pre-create worktrees)

**If pkg/ API needs adapter layer**:
- Budget 4 hours on Day 2 for adapter implementation
- Keep adapter minimal (just the hooks we need)

## Demo Script

### 4-Minute Screencast Flow

**Setup** (pre-recorded or scripted):
```bash
# Ensure clean state
cd /Users/clint/code/ourocodus
agentd stop alice bob 2>/dev/null || true
rm -rf .agentd/worktrees
```

**Act 1: Environment Validation** (30s)
```bash
$ agentd doctor
✓ Docker daemon running (v27.4.1)
✓ File sharing enabled: /Users/clint/code
✓ Image present: ourocodus/agent:latest
✓ Git worktree support confirmed
✓ Spawn smoke test passed
Environment ready!
```

**Act 2: Spawn Agents** (1m)
```bash
$ agentd spawn alice
✓ Created worktree: .agentd/worktrees/alice (branch: agentd/alice-a1b2c3)
✓ Container started: agentd-a1b2c3
✓ Credentials: mounted at /root/.creds (read-only)
✓ Agent ready: alice

$ agentd spawn bob
✓ Created worktree: .agentd/worktrees/bob (branch: agentd/bob-d4e5f6)
✓ Container started: agentd-d4e5f6
✓ Agent ready: bob
```

**Act 3: Show Isolation** (1m30s)
```bash
$ agentd list
AGENT  STATUS   WORKSPACE                  CONTAINER      CREATED
alice  running  .agentd/worktrees/alice    agentd-a1b2c3  1m ago
bob    running  .agentd/worktrees/bob      agentd-d4e5f6  30s ago

$ agentd logs alice --tail 5
[alice] ACP server listening on stdio...
[alice] Workspace: /workspace (isolated)
[alice] Credentials: /root/.creds

$ git worktree list
/Users/clint/code/ourocodus      [main]
.agentd/worktrees/alice          [agentd/alice-a1b2c3]
.agentd/worktrees/bob            [agentd/bob-d4e5f6]
```

**Act 4: Cleanup** (1m)
```bash
$ agentd stop alice
✓ Stopped container agentd-a1b2c3
✓ Removed worktree .agentd/worktrees/alice
✓ Cleaned credentials

$ agentd list
AGENT  STATUS   WORKSPACE                  CONTAINER      CREATED
bob    running  .agentd/worktrees/bob      agentd-d4e5f6  2m ago

$ agentd stop bob
✓ Stopped container agentd-d4e5f6
✓ Removed worktree .agentd/worktrees/bob
✓ Cleaned credentials

$ agentd list
No agents running.
```

**Closing** (narration):
"agentd demonstrates Ourocodus's three-layer isolation: git worktrees isolate code, Docker containers isolate processes, and credential volumes isolate access. Multiple agents work concurrently without conflicts - the foundation for collaborative multi-agent coding."

## Code Structure

```
cmd/agentd/
├── main.go              # Cobra root + CLI entrypoint (~50 lines)
├── cmd_doctor.go        # Doctor command implementation (~200 lines)
├── cmd_spawn.go         # Spawn command (~150 lines)
├── cmd_list.go          # List command (~100 lines)
├── cmd_stop.go          # Stop command (~100 lines)
├── cmd_logs.go          # Logs command (~80 lines)
├── config.go            # .agentd.yml reader via viper (~100 lines)
└── labels.go            # Label schema constants (~30 lines)
```

**Total new code**: ~810 lines
**Zero changes to pkg/** - we only consume existing APIs

## Dependencies

### Required
- `github.com/spf13/cobra` - CLI framework
- `github.com/fatih/color` - Terminal colors for subtle visual delight
- `pkg/agent/container` - AgentContainerLauncher
- `pkg/worktree` - AgentWorktreeManager
- Docker SDK (already in use by pkg/)

### Optional (Day 4)
- `github.com/spf13/viper` - Config reading (.agentd.yml)

## Visual Design Language

**Philosophy**: "Useful over all but sprinkling in some joy"

**Minimal Emoji Usage** (3-5 total, teaching the architecture):
- 🌳 Worktree (code isolation layer)
- 📦 Container (process isolation layer)
- 🔑 Credentials (access isolation layer)
- ✓ Success indicator
- × Failure indicator

**Color Semantics** (via `github.com/fatih/color`):
- Green: Success, running state
- Red: Errors, failed state
- Yellow: Warnings, caution
- Cyan: Info, neutral details

**Typography**:
- Clean hierarchy
- ✓ × for clear outcomes
- Simple table formatting
- No ASCII art or heavy borders

**What We Avoid**:
- Themed characters (elves, gnomes, wizards, crystals)
- Decorative emoji (sparkles, celebrations, magic)
- ASCII art banners
- Heavy box-drawing characters
- Spinners or animations

**Joy Comes From**:
- Clarity and scannability
- Professional polish
- Aligned columns
- Clear information hierarchy
- Minimal, meaningful touches

## Success Metrics

### Technical Metrics
- [ ] Doctor command detects all environment issues
- [ ] Spawn creates isolated agent in <5 seconds
- [ ] List shows accurate agent status
- [ ] Stop cleans all resources (verified via docker ps, git worktree list)
- [ ] Logs stream in real-time without buffering

### Demo Metrics
- [ ] Demo completes in <4 minutes
- [ ] Zero manual Docker commands needed
- [ ] Audience understands three-layer isolation
- [ ] No errors or warnings during demo

### Quality Metrics
- [ ] All commands have --help text
- [ ] Error messages are actionable
- [ ] Idempotent operations (safe to retry)
- [ ] Clean exit codes (0=success, 1=error)

## Future Enhancements (Post-MVP)

**Not in scope for Friday demo**:

1. **State persistence** - Track agents across restarts
2. **Agent exec** - Run commands inside agent containers
3. **Resource limits** - CPU/memory quotas per agent
4. **Agent prune** - Cleanup orphaned resources
5. **Status command** - Detailed agent inspection
6. **Config templates** - Per-project .agentd.yml templates
7. **Multi-repo support** - Agent workspace outside repo
8. **Relay integration** - Connect agentd to existing relay/PWA

## Appendix: Consensus Validation Summary

**Models Consulted**: gpt-5 (for), gpt-5-mini (against), gpt-5-nano (neutral)

**Universal Agreement** (all 3 models):
- Core feasibility: 7-8/10 confidence
- Zero-state, label-based lifecycle is correct approach
- Doctor command is critical for demo success
- Need deterministic naming and fallback screencast
- Timeline buffer: MVP by Day 3, polish Days 4-5

**Key Refinements from Consensus**:
1. **Harden doctor** with spawn smoke test (gpt-5-mini)
2. **Use Docker secrets/volumes** for credentials, not env vars (gpt-5-nano)
3. **Pre-create demo state** to avoid flakiness (gpt-5-mini)
4. **Add 1-day contingency** for macOS Docker issues (gpt-5-mini)
5. **Validate pkg/ API early** on Day 1 (gpt-5-mini)

**Confidence Adjustment**: Base 7-8/10 → 8.5/10 with mitigations

## Approval

**Design Validated**: Yes
**Consensus Complete**: Yes (3 models)
**Ready for Implementation**: Yes

**Next Steps**:
1. Create implementation worktree (optional)
2. Begin Day 1 tasks (Cobra scaffold + doctor command)
3. Validate pkg/ API assumptions

---

*Document Version: 1.0*
*Last Updated: 2025-01-18*
*Author: Claude (via brainstorming + consensus validation)*
