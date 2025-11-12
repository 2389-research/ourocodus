# ACP Containerization Implementation Plan

**Date:** 2025-11-10
**Target Issue:** #193 – Run ACP client inside spawned container
**Status:** Draft

## Executive Summary

Agents currently spawn a Docker container for workspace isolation but still launch the ACP binary (`claude-code-acp` or `echo-agent`) on the **host**. This breaks the mental model (operators expect agent code inside the container), weakens isolation, and complicates dependency management. This plan moves ACP execution fully into the spawned container while preserving our principles: small, testable units, least responsibility, and strong domain modeling.

Key outcomes:
- ACP process starts **inside** the agent container created by `agent.Launcher`.
- Relay interacts with ACP via the same JSON-RPC client, but transport becomes injectable.
- Session lifecycle (spawn, terminate, cleanup) treats “container + ACP” as a single responsibility domain.

## Goals

1. **Container runtime as single source of truth** – When an agent container dies, the ACP process dies with it (and vice versa).
2. **Maintainable abstractions** – Introduce transport/process interfaces so ACP client logic stays pure.
3. **Testability** – New components are injectable and unit-testable without Docker.
4. **Incremental rollout** – Provide a safe fallback to the host path until container exec proves stable.

## Non-Goals

- Rewriting the ACP protocol or message formats.
- Implementing streaming stdin/stdout for ACP yet (future milestone).
- Changing the user-facing PWA behavior (only backend plumbing).

---

## Current State Snapshot

| Layer | Responsibility | Current Behavior | Pain Points |
|-------|----------------|------------------|-------------|
| `session.Manager` | Orchestrates workspaces, containers, ACP clients | Calls `launcherFactory.CreateLauncher` (spawns container) **and** `clientFactory.NewClient` (spawns host ACP) | Double lifecycle, confusing logs, cleanup races |
| `pkg/acp.Client` | JSON-RPC over stdio + process spawning | Bundles `exec.Command` creation with request handling | Hard to test; cannot reuse inside Docker exec |
| `pkg/agent/container.AgentContainerLauncher` | Creates worktrees, credentials, containers | No visibility into ACP runtime | Container only holds workspace; agent logic remains outside |

---

## Design Directions Explored

### Direction A – Minimal change (host ↔ container switch inside `acp.Client`)
- Add env flag like `ACP_RUN_MODE=container`; `acp.Client` shells out `docker exec` instead of binary path.
- **Pros:** Small diff, reuses existing client.
- **Cons:** `acp.Client` now owns Docker concerns, violates separation, difficult to test.

### Direction B – Transport abstraction (chosen)
- Split ACP responsibilities into `Transport` (stdin/stdout conduit) and `Client` (JSON-RPC parser).
- Inject `ProcessLauncher` implementations (host vs container exec) via `ClientFactory`.
- **Pros:** Clean domain model; easy to test; future runtimes (SSH, remote) pluggable.
- **Cons:** Requires refactor touches across session + client factories.

### Direction C – Socket proxy between relay and container
- Run sidecar inside container exposing ACP over TCP; relay connects via forwarded port.
- **Pros:** Keeps `acp.Client` unchanged.
- **Cons:** Adds networking hop, TLS risk, decreases pragmatism.

**Decision:** Direction B aligns best with our coding philosophies (pragmatic yet clean, DI-friendly, single responsibility). Directions A/C either muddy responsibilities or over-engineer the milestone.

---

## Architecture Overview

```
[session.Manager]
    ↳ agent.Launcher (spawns container)
    ↳ ClientFactory.NewClient(ctx, AgentRuntime) ------┐
                                                     v
                                       [ProcessLauncher]
                                (host exec or container exec)
                                                     |
                                               Transport (stdin/stdout)
                                                     |
                                            [acp.Client JSON-RPC]
```

### Key Roles
- **AgentRuntimeContext** – Pure struct describing session ID, agent ID, workspace path, container handle, credentials. Shared across collaborators.
- **ProcessLauncher interface** – `Start(ctx, AgentRuntimeContext) (Transport, error)`; implementations:
  - `HostProcessLauncher` wraps `exec.Command` (current behavior).
  - `ContainerExecLauncher` uses Docker exec API via `containersession.Manager`.
- **Transport interface** – Owns stdin/stdout/stderr pipes; allows acp.Client to stay pure.
- **ClientFactory** – Composes API key + ProcessLauncher to return fully wired `ACPClient`.

---

## Implementation Plan

### Phase 1 – Refactor ACP client to accept transports
1. Introduce `acp.Transport` interface (Read/Write/Close helpers) and `ProcessLauncher` definitions in `pkg/acp`.
2. Modify `acp.Client` so `NewClientFromTransport(transport Transport, logger Logger)` constructs the JSON-RPC client without spawning processes.
3. Adapt current `NewClient(workspace, apiKey, opts...)` to be a thin wrapper that creates a `HostProcessLauncher` + transport. Ensure zero behavior change for existing callers.
4. Update tests in `pkg/acp/client_test.go` to use fake transports; add regression tests for buffering/ID ordering.

### Phase 2 – Extend session client factory and manager wiring
1. Define `session.AgentRuntimeContext` struct (session ID, agent ID, workspace, optional `agent.AgentHandle`, config knobs).
2. Change `ClientFactory` interface to `NewClient(ctx context.Context, runtime AgentRuntimeContext) (ACPClient, error)`; update mocks/tests.
3. Provide `ACPProcessFactory` that wraps API key loading + `ProcessLauncher` selection (host vs container). Default to host until ContainerExec is ready.
4. Update `session.Manager.SpawnAgent` to populate runtime context immediately after launcher.Spawn succeeds.
5. Ensure cleanup paths (failure/termination) still stop launchers and close clients.

### Phase 3 – Implement container exec launcher (#194, #195 dependency)
1. Enhance `agent.AgentHandle` or add helper to expose container session ID and Docker client reference (already in `AgentContainerHandle`).
2. Add method to `containersession.Manager` (or helper) to `ExecInSession(ctx, sessionID string, execCfg ExecConfig) (Transport, error)` that:
   - Calls `ContainerExecCreate` with command (ACP binary) and env (`ANTHROPIC_API_KEY`, workspace path).
   - Attaches to stdin/stdout/stderr pipes and returns a transport implementation.
3. Implement `ContainerExecLauncher.Start` using the exec helper, wiring environment and workspace from `AgentRuntimeContext`.
4. Introduce feature flag (env var `OUROCODUS_ACP_RUNTIME=container|host`, default host). Session manager chooses ProcessLauncher accordingly.
5. Add integration test under `pkg/relay/session/launcher_integration_test.go` (tagged docker) that spawns the echo-agent image, verifies ACP process PID inside container, and ensures `agent/sendMessage` round-trips.

### Phase 4 – Cleanup & rollout
1. Remove fallback flag once container path stabilizes (future milestone).
2. Update docs (`docs/ISSUES.md`, `docs/testing/milestone-3-integration-tests.md`) with new runtime behavior.
3. Ensure logging clearly states runtime location (e.g., `ACP runtime running inside container <id>`).

---

## Execution Work Plan

### Workstreams & Ownership
| ID | Workstream | Primary Owner | Supporting Roles | Definition of Done |
|----|------------|---------------|------------------|--------------------|
| WS-1 | Transport abstraction (Phase 1) | Relay Platform pod | QA-infra for test harness updates | `pkg/acp` exposes `Transport`/`ProcessLauncher`, host path green on CI |
| WS-2 | Session wiring (Phase 2) | Session Lifecycle pod | Runtime observability | `session.Manager` constructs runtime context + DI surfaces are migrated |
| WS-3 | Container exec runtime (Phase 3) | Containersession pod | DevInfra for Docker image prep | ACP launched via `docker exec` under feature flag with integration test |
| WS-4 | Rollout & docs (Phase 4) | Relay Experience | Support / SRE | Host path fallback removed, docs + runbooks updated |

### Timeline & Checkpoints
- **Nov 10 (Kickoff):**
  - Confirm owners above; align on success metrics (container/host parity latency <5%).
  - File tracking issues (#193 parent, child issues for WS-1…WS-4) and link to roadmap.
- **Nov 11–12:** Deliver WS-1 PR stack; land unit tests + fake transports. Exit criterion: host client unaffected, CI green.
- **Nov 13–14:** Deliver WS-2 refactor; ensure relay env vars plumbed via `AgentRuntimeContext`. Blocker review with session SMEs before merge.
- **Nov 17–19:** Execute WS-3; build docker exec helper, add feature flag + integration tests. Hold optional swarm test on Nov 19.
- **Nov 20:** Run WS-4 activities: documentation refresh, ops runbook updates, dry-run pre-commit pipeline, production rollout plan sign-off.

### Task Breakdown & Dependencies
1. **Kickoff checklist**
   - Create shared tracking doc linking to this plan.
   - Validate ACP binary availability inside images (dependency (#194)).
2. **Implementation tasks**
   - WS-1: Refactor `pkg/acp` – owners pair, review by API maintainers.
   - WS-2: Update `session.Manager`, `ClientFactory`, mocks, cleanup flows.
   - WS-3: Container exec helper + launcher + feature flag plumbing.
   - WS-4: Remove flag after bake, refresh docs, add operational alerts.
3. **Validation tasks**
   - Extend unit tests (WS-1/WS-2) to assert transport injection.
   - Add docker-tagged integration test (WS-3) gated behind `make docker-test`.
   - Manual smoke in staging verifying ACP PID inside container.

### Coordination & Tracking
- Daily async update in #relay-runtime (template: owner, workstream, blockers, ETA).
- Use GitHub Projects swimlanes per WS; require linked issue before merge.
- Schedule 15-min checkpoint on Nov 14 and Nov 20 for go/no-go decisions.
- Capture metrics (container startup duration, failure rate) in Datadog dashboard before enabling flag in production.

### Exit Criteria Summary
- **Phase 1:** Host transport abstraction merged, unit tests passing.
- **Phase 2:** Relay constructs ACP clients solely via runtime context, cleanup symmetrical.
- **Phase 3:** Feature flag `OUROCODUS_ACP_RUNTIME=container` defaults to container in staging with green docker integration test.
- **Phase 4:** Docs/runbooks updated, fallback path documented, oncall comfortable, telemetry proves parity for 48h burn-in.

---

## Testing Strategy

| Layer | Test Type | Details |
|-------|-----------|---------|
| `pkg/acp` | Unit | Fake transport returning scripted responses; verifies serialization and error paths |
| `session.Manager` | Unit | Use FakeClientFactory to ensure runtime context is passed correctly, cleanup handles remove both transport + container |
| `containersession` | Integration (docker) | New exec helper test: run busybox container, exec `/bin/echo` via helper, ensure output captured |
| End-to-end | Doc-guided manual (until CI docker available) | `make agent-image` → `bin/relay` → spawn agent; confirm `docker top` shows ACP binary |

---

## Risks & Mitigations

- **Docker exec stream handling complexity** – Mitigate by reusing `stdcopy.StdCopy` patterns from `containersession.Manager`; write focused tests on the exec helper.
- **Regression in existing host-based flow** – Keep host launcher available behind feature flag until container path verified.
- **API churn** – Changes to `ClientFactory` ripple through tests; mitigate with small shims and parallel constructors.

---

## Next Steps Checklist

- [ ] Phase 1 PR: transport abstraction + acp.Client refactor
- [ ] Phase 2 PR: session runtime context + factory wiring
- [ ] Phase 3 PRs:
  - [ ] Docker exec helper in containersession
  - [ ] ContainerExecLauncher implementation
- [ ] Update docs + testing instructions (Phase 4)

Once Phase 3 lands, issue #193 can close (ACP fully inside containers). Follow-up issues (#194, #195) ensure images contain ACP runtimes and the relay launches them via exec.
