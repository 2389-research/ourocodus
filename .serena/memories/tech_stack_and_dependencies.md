# Tech Stack and Dependencies

## Programming Languages
- **Go 1.24.0** - Backend services, core logic
- **TypeScript/JavaScript** - PWA frontend
- **Bash** - Build and automation scripts

## Core Go Dependencies

### Container Management
- `github.com/docker/docker` v28.5.1 - Docker SDK for Go
  - Container lifecycle management
  - Used in pkg/containersession
  - License: Apache 2.0

### Messaging
- `github.com/nats-io/nats.go` v1.46.1 - NATS client library
- `github.com/nats-io/nats-server/v2` v2.12.1 - NATS server (for embedded testing)
  - Pub/sub messaging
  - JetStream for event streams
  - Used in pkg/nats

### WebSocket
- `github.com/gorilla/websocket` v1.5.3 - WebSocket implementation
  - PWA ↔ Relay communication
  - Used in pkg/relay

### Utilities
- `github.com/google/uuid` v1.6.0 - UUID generation for session IDs
- `golang.org/x/sync` v0.17.0 - Advanced synchronization primitives
- `github.com/opencontainers/image-spec` v1.1.1 - OCI image specifications
- `github.com/containerd/errdefs` v1.0.0 - Standard error definitions

### Observability
- `github.com/prometheus/client_golang` v1.17.0 - Prometheus metrics
  - Used for NATS metrics export

### Testing
- `github.com/stretchr/testify` v1.11.1 - Test assertions and mocking

## External Services

### Required
- **Docker** - Container runtime (Docker Desktop or Colima on macOS)
  - Required for agent isolation
  - Must be running before starting relay

### Optional
- **NATS Server** - Message bus (optional in Phase 1)
  - `nats://localhost:4222` (client port)
  - `http://localhost:8222` (monitoring port)
  - `http://localhost:7777/metrics` (Prometheus metrics)
  - Started via `make nats-start` or docker-compose

### Development Tools (via mise)
- **golangci-lint** - Comprehensive Go linter
- **staticcheck** - Advanced static analysis tool
- **gofumpt** - Stricter Go formatter (stricter than gofmt)
- **esbuild** - TypeScript/JS bundler for PWA
- **minify** - HTML/CSS/JS minifier
- **typescript** - TypeScript compiler
- **vitest** - Unit testing framework for PWA

## Frontend Stack (PWA)
- **Vanilla TypeScript/JavaScript** - No heavy frameworks
- **Tailwind CSS** - Utility-first CSS framework
- **Service Workers** - Offline capabilities
- **WebSocket API** - Real-time communication with relay

## Environment Variables

### Required
- `ANTHROPIC_API_KEY` - Anthropic API key for Claude Code agents

### Optional
- `OUROCODUS_ACP_BINARY` - Path to custom ACP binary (default: claude-code-acp from PATH)
- `OUROCODUS_ACP_RUNTIME` - ACP runtime mode: `host` (default) or `container`
- `NATS_URL` - NATS server URL (default: disabled, events log to stdout)

## System Requirements
- **OS**: macOS (Darwin), Linux (likely works, not extensively tested)
- **Docker**: Docker Desktop or Colima
- **Git**: For worktree management
- **Go**: 1.24.0 (installed via mise)
