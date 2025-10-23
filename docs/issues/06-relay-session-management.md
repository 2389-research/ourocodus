# Issue #6 · Relay Session Management

## Goal
Build the in-memory session manager that keeps per-session state inside the relay so that multiple ACP agents can be coordinated safely. The manager must expose primitives the WebSocket server (issue #5) and the forthcoming ACP integration layer (issue #7) can call to create, inspect, and tear down sessions while reinforcing our engineering preferences for narrow, composable functions and dependency-injected collaborators.

## Background
- [Session Lifecycle](../SESSION_LIFECYCLE.md) documents the state machine (CREATED → SPAWNING → ACTIVE → TERMINATING → CLEANED) that this issue must enforce.
- [Relay PRD](../prd/relay.md) expects the relay to own the "connection management" and "session routing" responsibilities, with composite keys of `session_id:role` for lookups.
- During Phase 1 the relay keeps everything in memory (see [Assumptions](../ASSUMPTIONS.md#4-session-lifecycle)), so no persistence layer is required yet.

## Design Principles
- **Separation of concerns** – Keep data containers, state-transition logic, and side-effectful coordination in distinct types so functions remain small and testable.
- **Pure logic first** – Encode lifecycle transitions in pure functions that accept current state + event and return the new state (or error) without touching shared data. Wrap them with thin methods that apply synchronization.
- **Dependency injection** – Inject pluggable collaborators (clock, UUID generator, cleanup hooks) through constructor functions so tests can supply fakes.
- **Interface boundaries** – Favor interfaces that model behaviour (`SessionStore`, `SessionLifecycle`, `ConnectionRegistry`) so higher layers depend on contracts rather than concrete structs.

## Scope
1. **Session domain model** – Define immutable-ish core types under `pkg/relay/session/`:
   - `type Session struct` limited to identifiers, declarative metadata, and counters. Provide read-only getters; all mutation should flow through lifecycle methods.
   - `type Handle struct` (or similar) encapsulating references to user/agent WebSockets and process handles so they are not spread across the codebase.
   - Enumerated `SessionState` plus helper methods such as `State.String()`.
2. **Lifecycle orchestration** – Implement a small `StateMachine` struct with pure transition helpers:
   - `func NextState(current SessionState, event Event) (SessionState, error)` describing valid transitions based on the documented lifecycle.
   - Separate orchestration methods on `Manager` that coordinate mutex access, call the pure transition helper, then trigger injected hooks (`OnActivate`, `OnTerminate`).
   - Expose explicit methods (`Activate`, `AttachAgent`, `RecordHeartbeat`, `MarkTerminating`, `CompleteCleanup`) rather than one catch-all `Update` so responsibilities stay narrow.
3. **Session storage** – Back the manager with a dependency-injected `SessionStore` interface (e.g., implemented by a `map` + `sync.RWMutex` now, later swappable for persistence).
   - Provide methods `Create(ctx, spec)`, `Get(id)`, `GetByRole(id, role)`, `List(filter)` that are each <20 LOC and primarily delegate to helpers.
   - Ensure constructors accept collaborators for ID generation, clocks, and cleanup scheduling (allowing deterministic tests).
4. **Cleanup coordination** – Model cleanup as a pluggable strategy interface (`type Cleaner interface { Cleanup(ctx context.Context, s *Session) error }`). For Phase 1 supply a no-op cleaner while allowing future issues to swap in real worktree/process cleanup logic.
   - Guarantee cleanup functions are idempotent and unit-tested via repeated calls.
5. **Testing strategy** – Add table-driven tests around the pure transition helper, plus concurrency-focused tests on the concrete manager verifying double-create guards, lookup correctness, and cleanup removal.

## Out of Scope
- Persisting session state anywhere other than memory.
- Spawning ACP binaries or creating git worktrees (handled in issue #7).
- User-facing REST endpoints (API server covers that in a different milestone).

## Deliverables
- New package `pkg/relay/session/` containing:
  - `manager.go` (constructor + public API that wires injected collaborators),
  - `state_machine.go` (pure transition helpers + tests),
  - `models.go` (Session, Handle, and related value objects), and
  - `store_memory.go` (Phase 1 in-memory implementation satisfying a small `Store` interface).
- Updated relay WebSocket handler to depend only on the `session.Manager` interface, obtained via constructor or dependency injection, never by instantiating concrete structs inline.
- Unit tests demonstrating:
  - Session creation + lookup + DI interactions (e.g., fake UUID/clock used).
  - State transition validation (rejecting illegal moves, e.g., `ACTIVE → CREATED`).
  - Cleanup calling the injected cleaner exactly once even when invoked repeatedly.
  - Concurrent `Create` + `List` calls do not race (validated with `-race`).

## Acceptance Criteria
- [ ] The relay exposes a session manager interface whose methods map directly to single-responsibility actions (`Create`, `AttachAgent`, `RecordHeartbeat`, `MarkTerminating`, `Cleanup`).
- [ ] Lifecycle transitions are implemented via pure helper functions with exhaustive table-driven tests for legal + illegal flows.
- [ ] The in-memory store and manager accept injected dependencies for ID generation, clock/time, and cleanup hooks; unit tests assert collaborators are invoked.
- [ ] The WebSocket handler depends on the manager interface via dependency injection and no longer reaches into registry internals.
- [ ] README/inline Go docstrings describe how the relay expects callers to drive session lifecycles pending ACP integration.

## Follow-Ups / Dependencies
- **Predecessor:** Issue #5 must land first so the agent WebSocket plumbing exists.
- **Successor:** Issue #7 will call into the manager to attach ACP clients and propagate ACP state.
- Consider adding a lightweight metrics hook (`Session.Count()`, `Session.StateCounts()`) once observability work begins (not required for this issue but leave extension points).
