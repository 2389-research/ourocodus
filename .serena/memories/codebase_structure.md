# Codebase Structure

This document describes the organization and structure of the Ourocodus codebase.

## Top-Level Directory Structure

```
ourocodus/
├── cmd/                  # Binary entry points
├── pkg/                  # Shared packages (core logic)
├── web/                  # PWA frontend (future)
├── scripts/              # Build and setup scripts
├── tests/                # Test suites
├── docs/                 # Documentation
├── configs/              # Configuration files
├── workspaces/           # Agent workspaces
├── examples/             # Example code
├── agent/                # Agent-specific files
├── bin/                  # Compiled binaries (generated)
└── .github/              # GitHub Actions workflows
```

## cmd/ - Binary Entry Points

Contains main packages for executable binaries:

- **cmd/relay/** - WebSocket relay server (main component)
- **cmd/cli/** - Command-line interface for interacting with the system
- **cmd/echo-agent/** - Echo test agent for demos and testing

Each subdirectory has a `main.go` file that serves as the entry point.

## pkg/ - Shared Packages

Core business logic organized by domain:

### pkg/agent/
Agent launcher abstraction layer and implementations.

**Key files:**
- `launcher.go` - `AgentLauncher` interface definition
- `mock_launcher.go` - Mock implementation for testing

**Subdirectories:**
- `pkg/agent/packnplay/` - Packnplay-based launcher implementation
  - `launcher.go` - Packnplay launcher
  - `handle.go` - Agent handle implementation
  - `doc.go` - Package documentation
  - `launcher_test.go` - Unit tests
  - `integration_test.go` - Integration tests (requires Docker, build tag)

### pkg/relay/
WebSocket relay server implementation.

**Key files:**
- `server.go` - Main relay server
- `message.go` - Message types and handling
- `interfaces.go` - Interface definitions
- `adapters.go` - Adapter implementations
- `session_adapter.go` - Session adapter
- Various test files (`*_test.go`, `*_unit_test.go`, `integration_test.go`)

**Subdirectories:**
- `pkg/relay/session/` - Session management
  - `manager.go` - Session manager (core orchestration)
  - `models.go` - Data models
  - `store_memory.go` - In-memory session store
  - `client_factory.go` - Factory for creating session clients
  - `cleaner.go` - Session cleanup
  - `errors.go` - Structured error definitions
  - `README.md` - Session package documentation
  - Various test files covering different aspects
  - `testdata/` - Test data and fixtures

### pkg/acp/
Agent Client Protocol (ACP) implementation.

**Key files:**
- `types.go` - ACP message types and structures
- `client.go` - ACP client implementation
- `client_test.go` - Client tests

### pkg/nats/
NATS client library for message streaming.

**Key files:**
- `client.go` - NATS client
- `jetstream.go` - JetStream stream management
- `subscription.go` - Subscription handling
- `message.go` - Message types
- `options.go` - Configuration options
- `errors.go` - Error types
- `health.go` - Health checking
- `metrics.go` - Prometheus metrics

## scripts/

Build and utility scripts:

- `setup-worktrees.sh` - Git worktree setup
- `smoke-test.sh` - Smoke test harness
- `run-e2e.sh` - End-to-end test runner
- `demo/` - Automated demo scripts
- `interactive/` - Interactive REPL demo

## tests/

Test suites:

- `tests/e2e/` - End-to-end tests
  - `README.md` - E2E testing documentation
  - Test scenarios for full system validation

## docs/

Project documentation:

- `ARCHITECTURE.md` - System architecture
- `SESSION_LIFECYCLE.md` - Session and agent lifecycle
- `ERROR_HANDLING.md` - Error handling patterns
- `ACP.md` - Agent Client Protocol details
- `PROTOCOLS.md` - Communication patterns
- `TESTING.md` - Testing strategy
- `NATS.md` - NATS usage and troubleshooting
- `ISSUES.md` - Issue dependency graph

## Configuration Files

- `.mise.toml` - Tool versions and mise tasks
- `.golangci.yml` - golangci-lint configuration
- `.pre-commit-config.yaml` - Pre-commit hooks
- `Makefile` - Build targets and commands
- `go.mod` / `go.sum` - Go dependencies
- `docker-compose.yml` - Docker services (NATS, etc.)
- `.gitignore` - Git ignore patterns
- `.envrc.example` - Example environment variables

## Key Patterns

### Interface-Driven Design

The codebase uses interfaces for testability and flexibility:
- `AgentLauncher` - Abstract agent lifecycle management
- `SessionStore` - Abstract session storage
- `ClientFactory` - Abstract client creation

### Test Organization

- Unit tests: `*_test.go` alongside source files
- Integration tests: `integration_test.go` with build tags
- Test data: `testdata/` subdirectories
- Mock implementations: `mock_*.go` files

### Package Dependencies

```
cmd/relay → pkg/relay → pkg/relay/session → pkg/agent
                      → pkg/acp
                      → pkg/nats

pkg/agent → pkg/agent/packnplay (Docker/Packnplay integration)
```

### Build Tags

- `//go:build integration` - Integration tests requiring external services (Docker, NATS)

## Entry Points for Common Tasks

### Adding a New Agent Launcher
1. Create implementation in `pkg/agent/<name>/`
2. Implement `AgentLauncher` interface
3. Add tests and integration tests

### Modifying Session Logic
1. Update `pkg/relay/session/manager.go`
2. Update models in `pkg/relay/session/models.go`
3. Add/update tests in `pkg/relay/session/`

### Adding New ACP Message Types
1. Update `pkg/acp/types.go`
2. Update `pkg/acp/client.go` handling
3. Add tests

### Working with NATS
1. Modify `pkg/nats/` components
2. Update stream definitions in `jetstream.go`
3. See `docs/NATS.md` for usage patterns
