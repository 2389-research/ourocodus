# Issue #7 · Relay ACP Integration

## Goal
Wire the relay session manager to real ACP agent processes so that each active session has a spawned `claude-code-acp` client, a prepared git worktree, and deterministic hooks for message translation. The result should be narrowly scoped, dependency-injected components that the upcoming routing issues (#8/#9) can call without touching process or filesystem details directly.

## Background
- [Relay PRD](../prd/relay.md) defines ACP process supervision, connection management, and protocol translation responsibilities that begin with this issue.
- [Session Lifecycle](../SESSION_LIFECYCLE.md) shows which transitions (#6 implements) must be triggered while spawning and shutting down ACP processes.
- The [ACP client package](../../pkg/acp) already spawns `claude-code-acp`, exposes `SendMessage`, and handles JSON-RPC plumbing; this issue composes that client, rather than re-implementing protocol logic.
- [Phase 1 Plan](../PHASE1_PLAN.md#4-relay-with-acp-integration) sketches a monolithic relay; this brief decomposes that sketch into small, testable units aligned with our design preferences.

## Design Principles
- **Constructor-driven wiring** – Provide factories (`NewRuntime`, `NewSpawner`) that accept interfaces for filesystem prep, command execution, and cleanup; never instantiate concrete dependencies inline.
- **Pure translators** – Keep JSON ↔ internal model conversions in pure functions that take structs and return structs/errors. Avoid hidden globals so #8/#9 can reuse the same translators.
- **One responsibility per function** – Separate worktree preparation, ACP client spawning, session attachment, and I/O loop setup into dedicated functions with clear inputs/outputs.
- **State orchestration via session manager** – All lifecycle mutations must go through the `session.Manager` API from issue #6. The ACP integration layer only calls `Manager` methods; it never reaches into session internals.
- **Deterministic teardown** – Termination paths must use injected `Cleaner`/`Reaper` collaborators so tests can assert ordering without time-dependent sleeps.

## Scope
1. **Agent runtime package** – Create `pkg/relay/agent/` with narrow domain types:
   - `Spec` struct describing role, workspace root, env vars, and command template.
   - `Runtime` struct holding the active `session.Handle`, `acp.Client`, cancel func, and diagnostic metadata.
   - Read-only accessors and zero mutation logic; mutations happen through orchestrators.
2. **Client factory abstraction** – Define `type ClientFactory interface { New(worktree string) (ACPClient, error) }` and a tiny adapter that wraps `acp.NewClient(worktree, apiKey, opts...)` with injected API key lookup + command options. Provide a fake implementation for tests.
3. **Spawn orchestration** – Implement `Spawner` in `pkg/relay/agent/spawner.go`:
   - Methods `Prepare(ctx, sessionID, role)` → creates/returns worktree path using injected `WorktreeService` (e.g., interface with `Create(sessionID, role) (string, error)` and `Remove(path) error`).
   - `Spawn(ctx, sessionID, role)` → transitions session to SPAWNING via manager, invokes `Prepare`, constructs client via factory, registers result back through `Manager.AttachAgent` (or equivalent), and moves session to ACTIVE with injected clock timestamps.
   - All long-running work is offloaded to goroutines started by a coordinator function `Start(ctx, sessionID, role)` that simply sequences the pure helpers.
4. **I/O bridge scaffolding** – Provide composable streams under `pkg/relay/agent/bridge.go`:
   - `func BridgeACPToManager(ctx context.Context, runtime *Runtime, sink session.Sink) error` to read `AgentMessage`s and push them into a channel owned by #9.
   - `func BridgeManagerToACP(ctx context.Context, runtime *Runtime, source session.Source) error` to consume messages destined for ACP (populated by #8 later).
   - Both functions accept interfaces (`Source`, `Sink`) so routing issues can inject mocks without spinning processes.
5. **Termination workflow** – Implement `func (s *Spawner) Terminate(ctx, sessionID string)` that transitions session to TERMINATING, calls `ACPClient.Close`, invokes injected cleaner, and ensures session removal occurs via `Manager.Cleanup`. Validate idempotency by calling twice in tests.
6. **Testing strategy** – Add table-driven tests under `pkg/relay/agent/` that:
   - Use fake `WorktreeService`, `ClientFactory`, and `session.Manager` to assert ordering of transitions and injected dependencies.
   - Exercise spawn happy path, worktree failure, client spawn failure, and termination idempotency.
   - Cover bridge helpers with stubbed `Source/Sink` proving they are pure loops around the provided interfaces.

## Out of Scope
- Real message routing between PWA/NATS and ACP (issues #8 and #9 implement the actual pipelines).
- Observability/metrics hooks beyond simple logging stubs.
- Container orchestration; continue spawning local binaries per Phase 1 assumptions.

## Deliverables
- New package `pkg/relay/agent/` containing:
  - `spec.go` (`Spec`, validation helpers).
  - `runtime.go` (`Runtime`, getters, construction helpers).
  - `factory.go` (`ClientFactory` interface + concrete adapter around `pkg/acp`).
  - `spawner.go` (orchestrator coordinating session manager, worktree service, and client factory).
  - `bridge.go` (pure I/O bridge loops).
  - `terminate.go` (cleanup helpers).
  - `_test.go` files covering success/failure cases and verifying dependency injection usage.
- Updates to the session manager brief (if necessary) to document new interfaces consumed from issue #6 (e.g., `AttachAgent`, `DetachAgent`).
- Inline Go doc comments describing how the routing layer should interact with the new abstractions.

## Acceptance Criteria
- [ ] Spawner uses injected `WorktreeService`, `ClientFactory`, and `session.Manager` interfaces exclusively; unit tests assert collaborators are called in the correct sequence.
- [ ] Session transitions for SPAWNING, ACTIVE, TERMINATING, and CLEANED occur solely through the manager API, with table-driven tests verifying legal/illegal flows when dependencies fail.
- [ ] Bridge helpers are pure (no shared state), accept context cancellation, and include unit tests proving graceful shutdown when contexts are cancelled.
- [ ] Termination logic is idempotent: repeated calls close ACP clients at most once and do not panic if resources are already cleaned.
- [ ] Documentation/comments reference how #8/#9 should provide `Source/Sink` implementations without touching ACP details.

## Follow-Ups / Dependencies
- **Prerequisites:** Issues #5 and #6 must land so the WebSocket server and session manager abstractions exist.
- **Unblocked next steps:** Issues #8 and #9 will plug routing sources/sinks into the bridges created here.
- **Monitoring:** Capture TODOs for replacing direct binary spawning with container orchestration in Phase 2 once the daemon/API server takes over process management.
