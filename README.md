# Ourocodus

Multi-agent AI coding system that orchestrates Claude Code, OpenAI Codex, and other ACP-compatible agents to collaboratively build software.

## Overview

Ourocodus enables users to spin up multiple AI coding agents (Claude Code instances) that work concurrently on different aspects of the same codebase. Users interact through a Progressive Web App, directing agents through isolated conversations while the system manages git worktrees and coordinates their work.

## Quick Start

```bash
# Install dependencies
npm install -g @zed-industries/claude-code-acp

# Set up git worktrees
./scripts/setup-worktrees.sh

# Set API key
export ANTHROPIC_API_KEY=sk-...

# Build and run
make build
make run

# Open WebSocket endpoint
# Note: PWA not yet implemented, relay WebSocket available at:
# ws://localhost:8080/ws
```text

## Environment Variables

**Required:**

- `ANTHROPIC_API_KEY` - Your Anthropic API key for Claude Code agents

**Optional:**

- `OUROCODUS_ACP_BINARY` - Path to custom ACP binary (default: `claude-code-acp` from PATH)
  - Set this to use alternative ACP implementations like the echo agent for testing
  - Example: `export OUROCODUS_ACP_BINARY=./bin/echo-agent`
  - Useful for: Running demos/tests without an API key, testing custom agent implementations

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for Phase 1 vs Long-term design.

**Phase 1 (Current):**

```text
PWA (Browser) ←WebSocket→ Relay (Go) ←stdio→ N× Claude Code ACP processes
                                              ↓
                                         Git Worktrees
```

The system supports multiple concurrent agents (no hard limit).

## Documentation

- **[PRD.md](PRD.md)** - Product vision and requirements
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - System architecture overview
- **[docs/SESSION_LIFECYCLE.md](docs/SESSION_LIFECYCLE.md)** - Session and agent lifecycle
- **[docs/ERROR_HANDLING.md](docs/ERROR_HANDLING.md)** - Error handling with structured codes
- **[docs/ACP.md](docs/ACP.md)** - Agent Client Protocol integration details
- **[docs/PROTOCOLS.md](docs/PROTOCOLS.md)** - Communication patterns
- **[docs/TESTING.md](docs/TESTING.md)** - Testing strategy

## Project Status

**Current:** Phase 1 - Foundation implementation

**Progress:** [13 issues](https://github.com/2389-research/ourocodus/issues) | [Milestone](https://github.com/2389-research/ourocodus/milestone/1) | [Issue Map](docs/ISSUES.md)

### Quick Links

- [Issue #1: Project Initialization](https://github.com/2389-research/ourocodus/issues/1) ← **Start here**
- [Issue Dependency Graph](docs/ISSUES.md)
- [Session Lifecycle](docs/SESSION_LIFECYCLE.md)
- [Error Handling](docs/ERROR_HANDLING.md)

## Development

### Build System

The project uses a Makefile for building and managing the system. Common commands are also encapsulated as [mise](https://mise.jdx.dev/) tasks so you can run them without remembering individual binaries. For example:

```bash
# Build binaries via Makefile
mise run build

# Run all Go tests
mise run test

# Execute the smoke test harness (all tests)
mise run smoke

# Run smoke tests with fuzzing (100 iterations)
./scripts/smoke-test.sh all --fuzz 100

# Full validation suite
mise run pre-commit
```text

Under the hood these call the following Make targets/scripts:

```bash
# Build all components
make build
# → Produces: bin/relay, bin/cli, bin/echo-agent

# Run tests
make test
# → Runs: go test ./...

# Run E2E integration tests
make test-e2e
# → Runs: ./scripts/run-e2e.sh (validates full PWA → Relay → ACP flow)

# Format code
make fmt
# → Runs: gofumpt -l -w .

# Run linter
make lint
# → Runs: golangci-lint run

# Run static analysis
make check
# → Runs: staticcheck ./...

# Run all checks
make pre-commit
# → Runs: fmt, vet, lint, tidy, build, test

# Start system (when implemented)
make run
# → Starts relay server

# Stop system
make stop
# → Terminates running processes

# Clean build artifacts
make clean
# → Removes: bin/ directory
```text

### Project Structure

```text
ourocodus/
├── cmd/                  # Binary entry points
│   ├── relay/           # WebSocket relay server
│   ├── cli/             # Command-line interface
│   └── echo-agent/      # Echo test agent
├── pkg/                  # Shared packages
├── web/                  # PWA frontend
├── scripts/              # Build and setup scripts
└── docs/                 # Documentation
```text

### Code Quality

The project uses automated quality gates:

**CI/CD (GitHub Actions)**

The project runs two GitHub Actions workflows on all PRs and pushes to main:

*ci.yml* - Core quality checks:

- Build all binaries (relay, cli, echo-agent)
- Run unit tests (`go test ./...`)
- golangci-lint verification
- gofmt formatting check
- shellcheck on scripts
- Binary smoke test

*smoke.yml* - Integration testing:

- Session management smoke tests (8 test scenarios)
- WebSocket relay integration tests (handshake, echo, session lifecycle, error handling)
- Fuzz testing available via `--fuzz N` flag (disabled by default for CI stability)

**End-to-End Testing**

The E2E test suite validates the complete system flow with real Claude Code agents:

```bash
# Run E2E tests (requires ANTHROPIC_API_KEY)
make test-e2e

# Or directly with the script
./scripts/run-e2e.sh
```

What it tests:
- Relay server startup and health
- WebSocket connection and session creation
- Spawning multiple agents (auth, db, tests)
- Bidirectional agent communication
- Git worktree commits from agents

See [tests/e2e/README.md](tests/e2e/README.md) for detailed documentation.

Note: Local dev uses gofumpt for stricter formatting than gofmt

**Pre-commit Hooks (Optional)**

```bash
# Install pre-commit
pip install pre-commit  # or: brew install pre-commit

# Install hooks
pre-commit install

# Run manually
pre-commit run --all-files
```text

Hooks run:

- `gofumpt` - Format Go code (stricter than gofmt)
- `go vet` - Static analysis
- `golangci-lint` - Comprehensive linting
- `go mod tidy` - Clean dependencies
- `make build` - Verify build succeeds

**Manual Linting**

```bash
# Install golangci-lint
brew install golangci-lint  # macOS
# or: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```text

## Demo

Two interactive demos are available to showcase the relay system features. Both run a local relay server with an echo agent and work without requiring an ANTHROPIC_API_KEY.

**Prerequisites for both demos:**

- Run `make build` first to ensure all binaries are compiled
- No ANTHROPIC_API_KEY required (uses echo-agent for testing)

### Option 1: Interactive REPL (Recommended for Exploration)

A full REPL interface for manual testing and experimentation.

```bash
# Via mise (recommended)
mise run interactive

# Or via Makefile
make interactive
```text

**Available Commands:**

- `create` - Create a new session
- `spawn <role> [workspace]` - Spawn an agent (default: ./workspaces/interactive)
- `msg <role> <message>` - Send message to agent
- `agents` - List spawned agents in current session
- `help` - Show command help
- `quit` - Exit the REPL

**Example Session:**

```text
[no session] > create
✅ Session created: 4ad2f420

[session:4ad2f420] > spawn assistant
✅ Agent 'assistant' spawned in ./workspaces/interactive

[session:4ad2f420] > msg assistant hello there!
🤖 assistant: Echo: hello there!

[session:4ad2f420] > msg assistant what can you do?
🤖 assistant: Echo: what can you do?

[session:4ad2f420] > spawn helper
✅ Agent 'helper' spawned in ./workspaces/interactive

[session:4ad2f420] > agents
🤖 Spawned Agents:
  - assistant
  - helper

[session:4ad2f420] > quit
👋 Goodbye!
```text

**Features:**

- Full REPL interface with command history
- Session tracking (shows current session ID in prompt)
- Agent management (tracks spawned agents)
- Real-time messaging (chat with agents and see responses instantly)
- Error handling (shows error codes and recoverability)
- Auto-starts and stops relay server

**Source code:** `scripts/interactive/main.go`

### Option 2: Automated Demo (For Quick Overview)

An automated demonstration that runs predefined scenarios.

```bash
# Via mise
mise run demo

# Or via Makefile
make demo
```text

**What the Automated Demo Showcases:**

**1. Session Lifecycle & Agent Communication**

- Complete workflow: `session:create` → `agent:spawn` → `agent:message`
- Bidirectional communication between user and agent
- Multiple message exchanges in a single session
- Agent state transitions (SPAWNING → ACTIVE)

Example output:

```text
━━━ Scenario 1: Session Lifecycle & Agent Communication ━━━
✅ Session created: 4ad2f420-73d5-4b98-9517-f7e78ebd7e11
✅ Agent spawned and ready (state: ACTIVE)

→ Testing bidirectional communication...
   User: Hello, agent!
   Agent: Echo: Hello, agent!
   User: Can you count to three?
   Agent: Echo: Can you count to three?
```text

**2. Clear Error Semantics with Recoverability**

- Structured error responses with error codes
- Recoverable vs non-recoverable error distinction
- `SESSION_NOT_FOUND` error (non-recoverable)
- `AGENT_NOT_FOUND` error (non-recoverable)

Example output:

```text
━━━ Scenario 2: Clear Error Semantics ━━━
✅ Error received:
   Code: SESSION_NOT_FOUND
   Message: session not found: 00000000-0000-0000-0000-000000000000
   Recoverable: false
```text

**Source code:** `scripts/demo/main.go`

### For Automated Testing

For non-interactive automated testing, use the smoke test suite:

```bash
mise run smoke
```text

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed setup instructions using mise for consistent development environments.

Quick overview:

1. Install [mise](https://mise.jdx.dev/) and run `mise install` to get all dev tools
2. Check [GitHub Issues](https://github.com/2389-research/ourocodus/issues)
3. Issues are ordered by dependency (work top-down)
4. Each issue has clear acceptance criteria
5. See labels for component/type/priority

## License

MIT
