# Agent Artifact Sharing Design

**Date:** 2025-01-21
**Status:** Draft
**Author:** Claude Code (via brainstorming session)

## Problem

Agents currently communicate through stdio and file modifications in worktrees. This works for small interactions but breaks down when agents need to exchange large artifacts (code generation outputs, analysis results, test data). We need a filesystem-based mechanism for agents to share artifacts efficiently during bidirectional collaboration.

## Requirements

1. **Artifact Types**: Code generation outputs (source files, configs) and analysis results (JSON, XML, profiling data)
2. **Workflow**: Bidirectional collaboration where agents iteratively exchange artifacts
3. **Size**: Small artifacts (< 1MB typically)
4. **Lifecycle**: Short-term persistence (hours to days) with automatic cleanup
5. **Integration**: File-based storage consistent with existing lease system
6. **Container-native**: Works naturally with Docker bind mounts

## Design: Inbox/Outbox Architecture

### Core Concept

Each agent has an inbox (receives artifacts) and an outbox (publishes final outputs). To send an artifact to another agent, write directly to that agent's inbox through a shared filesystem view.

### Directory Structure

**Host filesystem** (single source of truth):
```
~/.agentd/
├── session/              # Existing: lease files
└── runs/<runId>/         # New: artifact storage
    └── agents/
        ├── agent-abc123/
        │   ├── inbox/
        │   └── outbox/
        └── agent-xyz789/
            ├── inbox/
            └── outbox/
```

**Agent container view** (via bind mounts):
```
/inbox/              # This agent's inbox (receives artifacts)
/outbox/             # This agent's final outputs
/agents/
  ├── agent-abc/     # Peer agent's inbox (send artifacts here)
  ├── agent-xyz/     # Another peer's inbox
  └── agent-def/     # Yet another peer's inbox
```

### Key Insight

One physical file, multiple views through bind mounts:
- Agent A writes to `/agents/agent-b/analysis.json`
- Agent B reads from `/inbox/analysis.json`
- Both reference the same file: `~/.agentd/runs/<runId>/agents/agent-b/inbox/analysis.json`

### Mount Configuration

For agent-abc in run xyz:
- **Own inbox**: `~/.agentd/runs/xyz/agents/agent-abc/inbox/` → `/inbox` (rw)
- **Own outbox**: `~/.agentd/runs/xyz/agents/agent-abc/outbox/` → `/outbox` (rw)
- **Peer inboxes**: For each peer P:
  - `~/.agentd/runs/xyz/agents/<P>/inbox/` → `/agents/<P>` (rw)

### Completion Semantics

Writers use atomic operations to signal completion:
1. Write to temporary file: `artifact.tmp`
2. Rename atomically: `mv artifact.tmp artifact_v1.json`
3. Update manifest: `manifest.json` lists available artifacts
4. Publish NATS event: `runs.<runId>.agents.<recipient>.inbox.artifact`

Readers either poll the manifest, watch with fsnotify, or subscribe to NATS events.

### Versioning

Use timestamped or version-suffixed filenames to avoid overwrites:
- `analysis_v1.json`, `analysis_v2.json`, `analysis_v3.json`
- Manifest tracks the latest version
- Cleanup prunes old versions when run completes

### Security

Apply multiple layers of isolation:
1. **Container security**: Run agents as non-root with dropped capabilities
2. **Filesystem permissions**: Inbox owner can read/write, peers can only write (append-only via group permissions)
3. **AppArmor profiles**: Restrict filesystem access to designated directories
4. **Mount restrictions**: Only mount peer inboxes when collaboration graph authorizes the edge
5. **Rate limiting**: Host-level watchdog monitors write throughput per agent

### Error Handling

Fail fast with clear feedback:
- **Mount validation**: Container entrypoint verifies expected directories exist
- **Disk quotas**: Enforce per-run storage limits using XFS/ext4 project quotas
- **Partial writes**: Atomic rename prevents consumers from seeing incomplete files
- **Mount failures**: Orchestrator validates mount success before starting agent

## Collaboration Example

User asks Agent A to analyze code, Agent B to refactor based on analysis, then Agent A to review:

1. **Orchestrator** creates run directory structure and mounts:
   ```
   runs/run-123/agents/agent-a/inbox/
   runs/run-123/agents/agent-a/outbox/
   runs/run-123/agents/agent-b/inbox/
   runs/run-123/agents/agent-b/outbox/
   ```

2. **User** provides initial input:
   ```
   echo "codebase.tar.gz" > runs/run-123/agents/agent-a/inbox/task-input.tar.gz
   ```

3. **Agent A** analyzes and sends to Agent B:
   ```go
   // Agent A code
   analysis := performAnalysis()
   writeArtifact("/agents/agent-b/analysis_v1.json", analysis)
   publishEvent("runs.run-123.agents.agent-b.inbox.artifact")
   ```

4. **Agent B** receives notification, reads, refactors, and responds:
   ```go
   // Agent B code
   analysis := readArtifact("/inbox/analysis_v1.json")
   refactored := performRefactor(analysis)
   writeArtifact("/agents/agent-a/refactored_v1.tar.gz", refactored)
   publishEvent("runs.run-123.agents.agent-a.inbox.artifact")
   ```

5. **Agent A** reviews and finalizes:
   ```go
   // Agent A code
   refactored := readArtifact("/inbox/refactored_v1.tar.gz")
   review := performReview(refactored)
   if review.approved {
       writeArtifact("/outbox/final-code.tar.gz", refactored)
   }
   ```

6. **Orchestrator** retrieves final output from `runs/run-123/agents/agent-a/outbox/`

## Implementation Plan

### Phase 1: Storage Infrastructure
- Create run directory management (create, list, cleanup)
- Implement atomic write helpers (temp + rename)
- Add manifest file format and helpers
- Build cleanup scheduler with retention policies

### Phase 2: Orchestration
- Extend launcher to create run directories before agent start
- Compute collaboration graph from task dependencies
- Generate Docker bind mount specifications
- Add mount validation in container entrypoint

### Phase 3: Agent API
- Provide helper library for atomic artifact writes
- Add manifest reading utilities
- Implement discovery via polling or NATS subscription
- Document agent contract (inbox/outbox/agents conventions)

### Phase 4: Monitoring & Security
- Add Prometheus metrics (artifact counts, sizes, latencies)
- Implement per-run disk quotas
- Deploy host-level rate limiting watchdog
- Apply AppArmor profiles to containers

## Open Questions

1. **Run ID generation**: UUID? Timestamp-based? User-provided?
2. **Run to session mapping**: Should runs be owned by UserSessions, or independent?
3. **Broadcast artifacts**: If one artifact needs many recipients, use pointer files or duplicate?
4. **Discovery mechanism**: Default to NATS events, polling, or fsnotify?
5. **Initial inputs**: How do users provide seed artifacts to first agent?

## Alternatives Considered

### Reference-Based Output Model
Agents write to own output directory, pass references (`agent-a:artifact.json`). Consumers read from peer outputs.
- **Rejected**: Less intuitive than push-to-inbox model, no clear separation of WIP vs final artifacts

### NATS JetStream ObjectStore
Store artifacts in NATS rather than filesystem.
- **Deferred**: Adds complexity, contradicts Unix-style file philosophy. Consider for Phase 5 if cross-host sharing needed.

### Content-Addressable Storage
Store artifacts by SHA-256 hash, pass references.
- **Deferred**: Useful for deduplication but adds complexity. Consider if fan-out patterns dominate.

## Success Metrics

- Agents exchange artifacts < 1MB with sub-second latency
- Zero data loss (atomic writes, crash recovery)
- Run cleanup removes artifacts within retention window
- Disk usage bounded by quota enforcement
- Container mount setup < 100ms per agent

## References

- Zen MCP analysis: Comprehensive security, scalability, and atomicity recommendations
- Existing lease system: `pkg/relay/session/lease.go` (file-based, atomic operations)
- NATS events: `pkg/heartbeat` (pub/sub messaging)
