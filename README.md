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

See [GitHub Issues](https://github.com/2389-research/ourocodus/issues) for detailed task breakdown.

## Development

```bash
# Build all components
make build

# Run tests
make test

# Start system
make run

# Stop system
make stop
```

## Contributing

1. Check [GitHub Issues](https://github.com/2389-research/ourocodus/issues)
2. Issues are ordered by dependency (work top-down)
3. Each issue has clear acceptance criteria
4. See labels for component/type/priority

## License

MIT
