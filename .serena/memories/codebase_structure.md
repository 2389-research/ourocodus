# Codebase Structure

## Top-Level Organization

```
ourocodus/
├── cmd/                    # Binary entry points (main packages)
│   ├── relay/             # WebSocket relay server (main service)
│   ├── cli/               # Command-line interface
│   ├── echo-agent/        # Echo test agent for testing
│   └── event-logger/      # NATS event logging utility
│
├── pkg/                    # Shared reusable packages (core subsystems)
│   ├── relay/             # Relay server implementation (session management, WebSocket)
│   ├── containersession/  # Docker container lifecycle management
│   ├── worktree/          # Git worktree management for agent isolation
│   ├── acp/               # Agent Client Protocol (ACP) implementation
│   ├── nats/              # NATS client and messaging abstractions
│   ├── runtime/           # Runtime mode configuration (host vs container)
│   └── agent/             # Agent-specific orchestration layer
│
├── internal/               # Private application code (not importable)
│   └── webapp/            # Progressive Web App (PWA) frontend
│       ├── src/           # TypeScript source code
│       ├── web/           # Built assets (HTML, CSS, JS bundles)
│       └── tools/         # Build tools (asset hashing, icon generation)
│
├── tests/                  # Test suites
│   └── e2e/               # End-to-end integration tests
│
├── examples/               # Example applications and demos
│   ├── basic-demo/        # Automated demo showcasing features
│   └── interactive-repl/  # Interactive REPL for manual testing
│
├── scripts/                # Build, setup, and automation scripts
│   ├── setup-worktrees.sh
│   ├── smoke-test.sh
│   └── run-e2e.sh
│
├── docs/                   # Documentation
│   ├── architecture/      # System architecture docs
│   ├── development/       # Development guides
│   └── operations/        # Operational docs (security, etc.)
│
├── configs/                # Configuration files
├── workspaces/            # Agent worktree directories (created at runtime)
├── agent/                 # Agent-related configurations
└── bin/                   # Built binaries (created by make build)
```

## Key Package Responsibilities

### pkg/relay
- UserSession management (WebSocket connections)
- AgentSession management (spawned agents)
- Message routing between PWA and agents
- Session lifecycle (create, spawn, terminate)
- WebSocket protocol handling

### pkg/containersession
- Docker container lifecycle (create, start, stop, attach)
- Container session management
- Output streaming from containers
- Container reuse and recovery
- Workspace validation and security

### pkg/worktree
- Git worktree creation/removal for agent isolation
- Branch management (agent-{id}-{timestamp})
- Worktree cleanup and validation
- Prevents git conflicts between concurrent agents

### pkg/acp
- Agent Client Protocol client implementation
- JSON-RPC message transport
- Process launching (host or container mode)
- Bidirectional communication with ACP servers
- Error handling and logging

### pkg/nats
- NATS client wrapper with observability
- JetStream stream management
- Health checking and metrics
- Message publishing/subscribing
- Connection management

### pkg/agent
- Agent-specific orchestration
- Credential mounting and isolation
- Container launcher (coordinates worktree + container + credentials)

### internal/webapp
- TypeScript/JavaScript PWA frontend
- WebSocket client for relay communication
- Session UI and agent management
- Real-time messaging interface
- Service Worker for offline capabilities
