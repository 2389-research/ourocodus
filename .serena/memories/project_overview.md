# Project Overview: Ourocodus

## Purpose

Ourocodus is a multi-agent AI coding system that orchestrates Claude Code, OpenAI Codex, and other ACP-compatible agents to collaboratively build software. It enables users to spin up multiple AI coding agents that work concurrently on different aspects of the same codebase.

## Architecture

**Phase 1 (Current):**
```
PWA (Browser) ←WebSocket→ Relay (Go) ←stdio→ N× Claude Code ACP processes
                                              ↓
                                         Git Worktrees
```

**Key Components:**
- **Relay Server** - WebSocket relay that coordinates agent communication
- **ACP (Agent Client Protocol)** - Protocol for communicating with AI agents
- **Session Management** - Handles lifecycle of agent sessions and coordination
- **NATS Message Bus** - JetStream for persistent message streaming between services
- **Packnplay** - Docker containerization and git worktree management

## Tech Stack

- **Language**: Go 1.24.0
- **Core Dependencies**:
  - `github.com/obra/packnplay@v1.0.2` - Docker containerization and git worktree management
  - `github.com/gorilla/websocket@v1.5.3` - WebSocket relay
  - `github.com/nats-io/nats.go@v1.31.0` - Message bus with JetStream
  - `github.com/docker/docker` - Container management
  - `github.com/google/uuid` - UUID generation
  - `github.com/stretchr/testify` - Testing framework
- **Development Tools**:
  - mise - Tool version management
  - golangci-lint - Comprehensive linting
  - staticcheck - Advanced static analysis
  - gofumpt - Stricter Go formatter (stricter than gofmt)
  - pre-commit - Optional pre-commit hooks

## Project Structure

```
ourocodus/
├── cmd/                  # Binary entry points
│   ├── relay/           # WebSocket relay server
│   ├── cli/             # Command-line interface
│   └── echo-agent/      # Echo test agent
├── pkg/                  # Shared packages
│   ├── agent/           # Agent launcher abstraction
│   ├── relay/           # Relay server implementation
│   ├── acp/             # Agent Client Protocol
│   ├── nats/            # NATS client library
│   └── session/         # Session management
├── web/                  # PWA frontend (future)
├── scripts/              # Build and setup scripts
├── tests/e2e/           # End-to-end tests
└── docs/                # Documentation
```

## Environment Variables

**Required:**
- `ANTHROPIC_API_KEY` - Your Anthropic API key for Claude Code agents

**Optional:**
- `OUROCODUS_ACP_BINARY` - Path to custom ACP binary (default: `claude-code-acp` from PATH)

## Documentation

- **PRD.md** - Product vision and requirements
- **docs/ARCHITECTURE.md** - System architecture overview
- **docs/SESSION_LIFECYCLE.md** - Session and agent lifecycle
- **docs/ERROR_HANDLING.md** - Error handling with structured codes
- **docs/ACP.md** - Agent Client Protocol integration details
- **docs/PROTOCOLS.md** - Communication patterns
- **docs/TESTING.md** - Testing strategy
- **docs/NATS.md** - NATS usage and troubleshooting

## Current Status

Phase 1 - Foundation implementation
Progress tracking: GitHub Issues and Milestone
