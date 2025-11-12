# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **ACP Container Execution Mode**: New `OUROCODUS_ACP_RUNTIME` environment variable enables running ACP processes inside agent containers via `docker exec`
  - Default: `host` mode (ACP runs as host process)
  - Optional: `container` mode (ACP runs inside agent containers for enhanced isolation)
  - Automatic launcher selection based on runtime context
  - See [docs/ACP.md](docs/ACP.md) for details

- **Container Exec API**: New `ExecInContainer` method in `pkg/containersession` for running commands inside existing containers
  - Full stdio access (stdin/stdout/stderr)
  - Environment variable injection
  - Working directory and user configuration
  - Proper resource cleanup and goroutine lifecycle management

- **Workspace Path Rewriting**: Automatic translation of host workspace paths to container mount paths
  - Handles both `--workspace /path` and `--workspace=/path` formats
  - Transparent to ACP client code
  - Prevents path mismatch errors in container mode

- **Runtime Context Propagation**: New `AgentRuntimeContext` struct carries session/agent metadata through the system
  - Includes optional `ContainerID` for container mode
  - Enables launcher selection based on execution environment
  - Context cancellation support for graceful shutdown

### Fixed

- **Resource Leaks in Container Exec**: Fixed goroutine and connection leaks in `pkg/containersession/exec.go`
  - Added context cancellation for goroutine lifecycle management
  - Proper cleanup in `closeFn` to prevent orphaned resources
  - Documented that `attachResp.Reader` is `*bufio.Reader` without Close method

- **API Key Validation**: Added missing API key validation in `ContainerExecProcessLauncher`
  - Matches validation behavior of `HostProcessLauncher`
  - Returns clear error message when API key missing
  - Prevents silent failures during container exec

- **Flaky Container Tests**: Replaced hardcoded `time.Sleep` with proper container readiness checks
  - Uses `ContainerInspect` with 10-second timeout and 100ms polling
  - Applied to both smoke tests in `tests/e2e/acp_container_exec_test.go`
  - Eliminates race conditions in CI/CD

### Changed

- **ClientFactory Interface**: Updated signature to accept `context.Context` and `AgentRuntimeContext`
  - Breaking change: `NewClient(workspace string)` → `NewClient(ctx context.Context, runtime *AgentRuntimeContext)`
  - Enables cancellation and richer runtime metadata
  - See migration guide in [pkg/relay/session/README.md](pkg/relay/session/README.md)

- **Dependency Clarification**: Updated README.md to clarify packnplay status
  - Marked as "future integration (planned)" rather than actively used
  - Current implementation uses direct Docker SDK via `pkg/containersession`
  - packnplay v1.0.2 remains imported as placeholder for future features

### Documentation

- Added `OUROCODUS_ACP_RUNTIME` to environment variable documentation in:
  - [README.md](README.md)
  - [docs/architecture/AGENT_RUNTIME.md](docs/architecture/AGENT_RUNTIME.md)
  - [docs/architecture/ACP.md](docs/architecture/ACP.md)

- Updated [pkg/containersession/README.md](pkg/containersession/README.md) with:
  - `ExecInContainer` API reference and examples
  - ExecAttachment usage patterns
  - Resource management guidelines

- Updated [pkg/relay/session/README.md](pkg/relay/session/README.md) with:
  - Runtime context and container integration section
  - ACP launcher selection logic
  - Workspace path rewriting behavior

## [0.1.0] - 2025-01-XX

### Added

- Initial Phase 1 implementation
- WebSocket relay server for multi-agent coordination
- In-memory session management (UserSession + AgentSession)
- ACP (Agent Client Protocol) integration via pkg/acp
- Container session management via pkg/containersession
- NATS event publishing for session lifecycle
- Progressive Web App foundation
- Smoke test suite for relay and session management
- E2E integration tests with Docker

---

[Unreleased]: https://github.com/2389-research/ourocodus/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/2389-research/ourocodus/releases/tag/v0.1.0
