# Session Lifecycle Reliability – Implementation Plan

**Date:** 2025-11-10  
**Issues:** #190, #189, #187  
**Proposed Branch:** `feature/session-lifecycle-reliability`  
**Related Docs:** [2025-01-07-milestone-3-implementation.md](./2025-01-07-milestone-3-implementation.md)

## Overview

This plan delivers a cohesive "session lifecycle reliability" pull request that fixes the UX dead-ends around ending sessions, improves recovery from agent spawn failures, and surfaces the true cleanup status produced by the relay. The work spans both the relay server (Go) and the PWA frontend (TypeScript) but remains tightly scoped to shared session-lifecycle concerns.

## Success Criteria

1. Ending a session or disconnecting always returns the UI to the welcome state, hides stale controls, and reflects the relay-reported cleanup status (#190 + #187).
2. Failed agent spawns re-enable the Spawn button, communicate actionable remediation (build Docker image), and allow retries without reloading (#189).
3. Relay `session:ended` responses carry accurate cleanup metadata derived from real termination results (#187).
4. Logs no longer print full WebSocket payloads, preventing unintended data exposure while still aiding debugging (#186 dependency-lite).

## Work Breakdown

### Workstream A – Relay Cleanup Telemetry (#187)

**Task A1: Define termination summary contract**  
- Add `TerminationSummary` struct (agents terminated, agent failures, cleanup status enum, optional error text) in `pkg/relay/session`.  
- Update `SessionManagerInterface.TerminateUserSession` to return `(TerminationSummary, error)` and adjust all mock implementations/tests.

**Task A2: Populate summary in Manager**  
- In `Manager.TerminateUserSession`, capture:  
  - Total agents terminated vs. failures/timeouts.  
  - Cleaner hook outcome (set status `complete`, `partial`, `failed`).  
- Propagate notable errors into `TerminationSummary.Errors` for future UX use.  
- Extend existing tests (`manager_cleanup_test.go`, `manager_test.go`) to assert summary data for success, partial, and timeout paths.

**Task A3: Relay server propagation**  
- Update `handleSessionEnd` in `pkg/relay/server.go` to consume the new summary and call `NewSessionEndedMessage` with the actual `cleanupStatus`.  
- Adjust `pkg/relay/server_unit_test.go` mocks to verify the server forwards the reported status and agent counts.

### Workstream B – UI Reset + Status Surfacing (#190, ties to #187)

**Task B1: Centralize reset logic**  
- Introduce `resetSessionUI()` in `internal/webapp/src/connection.ts` that hides `#sessionInfo`, shows `#welcomeCard`, clears `userSessionId`, chat state, agent maps, and re-enables key controls.  
- Replace duplicated DOM manipulation in `handleSessionEnded`, `ws.onclose`, `disconnect()`, and any catastrophic error paths with the helper.

**Task B2: Reflect cleanup status**  
- Extend `handleSessionEnded` to read `message.cleanupStatus` and show a badge/line under the session card (e.g., `Cleanup: complete` | `partial` | `failed`).  
- When status is `partial/failed`, display a toast advising operators to inspect relay logs before starting a new session.

**Task B3: Harden button states**  
- Disable the End Session button immediately after user confirmation and re-enable only after reset.  
- Ensure Spawn/Send buttons check `this.userSessionId` so that reset always gates user actions.

### Workstream C – Agent Spawn Recovery (#189)

**Task C1: Frontend button state machine**  
- Track `this.pendingSpawnRole` (or similar) in `App.handleSpawnAgent`.  
- Add `resetSpawnButton()` utility that restores the button label/icon and clears the pending state.  
- Wire `resetSpawnButton()` into `handleAgentReady`, `handleSessionEnded`, and a new error hook.

**Task C2: Error propagation hook**  
- Extend `RelayConnection.handleError` to emit a `CustomEvent('relay:error', { detail })` or call a callback registered by `App`.  
- In App, catch `AGENT_SPAWN_FAILED` (or general recoverable errors) and invoke `resetSpawnButton()` immediately so users can retry without reloading.

**Task C3: Backend diagnostics + docs**  
- In `handleAgentSpawn`, detect container launcher errors caused by missing Docker images (`container.ErrContainerSetupFailed` and the "No such image" string) and override `errorMessage` with explicit build instructions.  
- Add `make agent-image` (runs `docker build -t ourocodus/agent:latest -f Dockerfile.agent .`) plus documentation snippets in `README.md` and `docs/AGENT_RUNTIME.md`.  
- Mention the requirement near the relay quick-start instructions.

### Workstream D – Logging Hygiene (quick win while touching relay)

**Task D1:** Update `pkg/relay/server.go` read loop to log `[RELAY] Received message (%d bytes, type=%s)` where `type` comes from a lightweight `BaseMessage` parse, removing full payload dumps.  
**Task D2:** Note the privacy improvement in the PR description and changelog (if applicable).

## Validation Plan

- **Go tests:** `go test ./pkg/relay/...` with special attention to updated manager/server suites.  
- **TypeScript checks:** `cd internal/webapp/src && mise exec -- tsc --noEmit` (if configured) and `mise exec -- vitest run` if tests exist.  
- **Manual PWA QA:**  
  1. Create session → spawn agent (happy path).  
  2. End session via button; verify welcome card, buttons, cleanup badge.  
  3. Disconnect via header button; verify same reset.  
  4. Trigger spawn failure by skipping Docker image build; ensure error toast shows build command and Spawn button re-enables without reload.  
  5. Build image and retry spawn; confirm success.  
- **Static analysis:** `mise run fmt`, `make lint`, `make check`, and `make pre-commit` before submission.

## Risks & Mitigations

- **Interface change ripple (Task A1):** Updating `SessionManagerInterface` touches tests and mocks. Mitigation: perform change in a dedicated commit with compiler guidance and ensure all mock helpers updated before larger refactors.
- **UI state race conditions:** Reset helper might conflict with ongoing DOM transitions. Mitigation: keep helper idempotent and guard DOM lookups; add console warnings when expected elements are missing.
- **User confusion on partial cleanup:** Provide clear copy around cleanup status and link to relay logs in docs/tooltip.

## Deliverables

1. Updated relay session termination flow with accurate cleanup telemetry returned to clients.  
2. PWA UI reset helper + visual cleanup status indicator.  
3. Robust spawn error recovery and actionable documentation for Docker image setup.  
4. Sanitized relay logs and regression-tested workflow.

## Rollout Notes

- Target a single PR containing the coordinated backend/frontend changes to keep E2E behavior testable.  
- Highlight in PR description how to reproduce improved flows (spawn failure retry, session reset).  
- After merge, update running relay environments to build the agent image once to avoid immediate errors.

