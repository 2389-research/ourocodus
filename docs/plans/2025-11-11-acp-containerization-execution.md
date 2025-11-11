# ACP Containerization Execution Plan

**Date:** 2025-11-11  
**Owner:** Relay Platform (coordination by runtime team)  
**Goal:** Land issue #193 by running the ACP runtime inside each spawned container while preserving a host fallback behind `OUROCODUS_ACP_RUNTIME`.

---

## Context Recap
- Agents currently spawn containers for workspace isolation but execute ACP on the host, causing lifecycle skew and weaker isolation.
- Direction B (transport + process launcher abstraction) is partially implemented; host launches now travel through `acp.Transport`, `ProcessLauncher`, and `AgentRuntimeContext` plumbing.
- Remaining work focuses on enabling a Docker exec transport, plumbing the runtime flag, and hardening integration + rollout.

## Workstreams
| ID | Scope | Lead | Dependencies | Exit Criteria |
|----|-------|------|--------------|----------------|
| WS-1 | Container exec transport | Containersession pod | Docker API access, images incl. ACP binary (#194) | `ContainerExecLauncher` returns an `acp.Transport` via Docker exec with parity tests |
| WS-2 | Runtime selection + wiring | Session Lifecycle pod | WS-1, feature flag config | `ClientFactory` selects launcher per flag, telemetry/logging in place |
| WS-3 | Validation & rollout | Relay Experience + SRE | WS-1/2, staging env | Docker-tagged integration test green, staged rollout checklist signed |

---

## Task Breakdown
### WS-1 – Container Exec Transport
1. **Docker exec helper** (`pkg/containersession`)
   - Add `ExecInSession(ctx, sessionID, cfg)` returning attached stdio streams and container PID metadata.
   - Reuse `stdcopy.StdCopy` for demux; ensure cleanup on ctx cancel.
2. **Transport implementation**
   - New `containersession.Transport` adapter satisfying `acp.Transport` (wraps Docker attach conn, handles Close semantics).
   - Wire helper through a `ContainerExecLauncher` that satisfies `acp.ProcessLauncher`.
3. **Unit & integration tests**
   - Fake Docker client tests for error paths (session missing, attach failure, EOF handling).
   - `make docker-test` scenario: start echo-agent image, exec ACP, send message, assert container PID hosts ACP binary.

### WS-2 – Runtime Selection & Wiring
1. **Flag + config surface**
   - Env `OUROCODUS_ACP_RUNTIME` (values `host`, `container`). Default `host`; allow per-session override for experiments.
   - Emit structured log showing runtime choice + container ID when applicable.
2. **Factory updates**
   - Extend `ACPClientFactory` to inject either `HostProcessLauncher` or the new `ContainerExecLauncher` based on runtime + presence of `runtime.ContainerID`.
   - Ensure host path remains functional when containers disabled (unit tests).
3. **Session manager cleanup**
   - Populate runtime context with launcher handle metadata (container ID, workspace). Confirm host-only flows skip container info but still function.

### WS-3 – Validation, Rollout, Docs
1. **Testing matrix**
   - Unit: acp transport, session factory flag logic.
   - Integration: docker-tagged test (CI opt-in) verifying ACP runs inside container.
   - Manual: staging relay -> spawn agent, `docker top` validation, failover test toggling flag to host.
2. **Observability**
   - Add metric `relay.acp.runtime_location` with labels `{runtime:host|container}`; alert if container error rate >2% during ramp.
   - Ensure stderr surfacing works via new transport (log sample lines).
3. **Docs & rollout**
   - Update `docs/SESSION_LIFECYCLE.md`, ops runbooks, and `README` quickstart describing flag.
   - Run 48h staging burn-in, then prod rollout in two phases (10% traffic, then 100%) with on-call signoff.

---

## Timeline (Target)
- **Nov 11–13:** WS-1 implementation + tests.
- **Nov 14–15:** WS-2 wiring + flag plumbing; host regression tests.
- **Nov 17–19:** WS-3 validation, docker integration test in CI, staging burn-in kickoff.
- **Nov 20:** Go/No-go with SRE; production rollout if metrics clean.

---

## Risks & Mitigations
- **Docker exec stream instability** – Put exec helper behind retries, add watchdog to Close transport on timeout.
- **Host regression** – Keep host launcher as default until staging proves parity; add unit tests for host v. container selection.
- **Operational readiness** – Coordinate with SRE for telemetry dashboards before rollout; document manual fallback procedure (toggle env + restart relay).

---

## Success Criteria
- `agent.SendMessage` works identically regardless of runtime selection (verified via tests + staging telemetry).
- Killing the container terminates ACP immediately (observed via integration test + manual smoke).
- Feature flag off switches back to host mode without relay restart (config hot reload or env switch on process restart documented).
