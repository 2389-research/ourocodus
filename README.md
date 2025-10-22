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

# Open PWA
open http://localhost:3000
```

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for Phase 1 vs Long-term design.

**Phase 1 (Current):**
```
PWA (Browser) ←WebSocket→ Relay (Go) ←stdio→ 3x Claude Code ACP processes
                                              ↓
                                         Git Worktrees
```

## Documentation

- **[PRD.md](PRD.md)** - Product vision and requirements
- **[docs/PHASE1.md](docs/PHASE1.md)** - Current phase: Foundation with real ACP
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - Phase 1 vs Long-term architecture
- **[docs/ACP.md](docs/ACP.md)** - Agent Client Protocol integration details
- **[docs/PROTOCOLS.md](docs/PROTOCOLS.md)** - Communication patterns (long-term)
- **[docs/PHASES.md](docs/PHASES.md)** - Roadmap for future phases

## Project Status

**Current:** Phase 1 - Foundation implementation

**Progress:** [13 issues](https://github.com/2389-research/ourocodus/issues) | [Milestone](https://github.com/2389-research/ourocodus/milestone/1) | [Issue Map](docs/ISSUES.md)

### Quick Links
- [Issue #1: Project Initialization](https://github.com/2389-research/ourocodus/issues/1) ← **Start here**
- [Issue Dependency Graph](docs/ISSUES.md)
- [Phase 1 Spec](docs/PHASE1.md)

## Development

### Build System

The project uses a Makefile for building and managing the system:

```bash
# Build all components
make build
# → Produces: bin/relay, bin/cli, bin/echo-agent

# Run tests
make test
# → Runs: go test ./...

# Start system (when implemented)
make run
# → Starts relay server

# Stop system
make stop
# → Terminates running processes

# Clean build artifacts
make clean
# → Removes: bin/ directory
```

### Project Structure

```
ourocodus/
├── cmd/                  # Binary entry points
│   ├── relay/           # WebSocket relay server
│   ├── cli/             # Command-line interface
│   └── echo-agent/      # Echo test agent
├── pkg/                  # Shared packages
├── web/                  # PWA frontend
├── scripts/              # Build and setup scripts
└── docs/                 # Documentation
```

### Code Quality

The project uses automated quality gates:

**CI/CD (GitHub Actions)**
- Builds on all PRs and pushes to main
- Runs full test suite
- Lints code with golangci-lint
- Checks formatting with gofmt

**Pre-commit Hooks (Optional)**
```bash
# Install pre-commit
pip install pre-commit  # or: brew install pre-commit

# Install hooks
pre-commit install

# Run manually
pre-commit run --all-files
```

Hooks run:
- `gofmt` - Format Go code
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
```

## Contributing

1. Check [GitHub Issues](https://github.com/2389-research/ourocodus/issues)
2. Issues are ordered by dependency (work top-down)
3. Each issue has clear acceptance criteria
4. See labels for component/type/priority

## License

MIT
