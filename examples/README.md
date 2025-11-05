# Ourocodus Demos

Interactive demonstrations of Ourocodus features. All demos are self-contained at the top level of this directory.

## Available Demos

### 1. Container Reuse Demo (START HERE)

**What it shows:** Phase 2 container management - intelligent reuse and cross-process attachment

**Run it:**
```bash
cd examples
./demo-container-reuse.sh
```

**Features demonstrated:**
- Automatic container discovery and reuse
- Reconnecting to running containers instead of creating new ones
- Restarting stopped containers
- Cross-process attachment (relay crashes and recovers)
- Workspace persistence across container lifecycle

**Use case:** Understanding how Ourocodus minimizes container churn and enables relay server crash recovery.

---

### 2. Session Hierarchy Demo

**What it shows:** Three-tier session architecture - UserSession → AgentSession → ContainerSession

**Run it:**
```bash
cd examples
./demo-session-hierarchy.sh
```

**Features demonstrated:**
- UserSession creation backed by ContainerSession
- Multiple AgentSessions per UserSession (multi-agent coordination)
- Crash recovery at session level using container reuse
- Independent agent lifecycle management
- Resource efficiency (one container, multiple agents)

**Use case:** Understanding the full session architecture and how multiple agents coordinate within a user's workspace.

---

### 3. NATS Basic Messaging Demo

**What it shows:** Basic NATS publish/subscribe messaging patterns

**Run it:**
```bash
cd examples
./demo-nats-basic.sh
```

**Prerequisites:** NATS server running on localhost:4222
```bash
docker run -d -p 4222:4222 nats:latest
```

**Features demonstrated:**
- Simple publish to NATS subjects
- Subscribe to receive messages
- Message delivery with correlation IDs
- Basic NATS client usage

**Use case:** Understanding NATS messaging fundamentals used for agent coordination.

---

## Quick Start

Each demo is self-contained with setup, execution, and cleanup:

```bash
# Run demos in recommended order
./demo-container-reuse.sh       # Start here
./demo-session-hierarchy.sh     # Then this
./demo-nats-basic.sh             # NATS messaging

# Manual cleanup if needed
./demo-container-reuse-reset.sh
./demo-session-hierarchy-reset.sh
```

## Prerequisites

- **Docker Desktop** running
- **Go 1.23+** installed
- Network access (to pull ubuntu:latest image)

## Demo Structure

Each demo consists of:
- `demo-{name}.sh` - Main script (setup + run)
- `demo-{name}.go` - Demo implementation
- `demo-{name}-reset.sh` - Cleanup script (where applicable)

All demos:
- Are self-contained at top level (no nested directories)
- Build binaries on first run
- Clean up resources automatically
- Use color-coded output for clarity
- Support pause-driven presentation flow
- Have explicit names describing what they demonstrate

## Troubleshooting

**Docker daemon not running:**
```bash
# Start Docker Desktop first
open -a Docker
```

**Build failures:**
```bash
# Ensure you're in the examples directory
cd examples
go mod download
```

**Container cleanup:**
```bash
# Remove all demo containers
docker ps -a --filter "label=com.ourocodus.containersession.managed-by" -q | xargs docker rm -f
```

## Architecture Overview

```
┌─────────────────────────────────────────┐
│          UserSession                    │
│  (User's workspace container)           │
│                                          │
│  ├─> AgentSession (agent 1)            │
│  ├─> AgentSession (agent 2)            │
│  └─> AgentSession (agent N)            │
│                                          │
│       └─> ContainerSession              │
│            (Docker backing with reuse)  │
└─────────────────────────────────────────┘
```

**ContainerSession (Phase 2):** Docker container lifecycle management with intelligent reuse

**UserSession (Phase 3 - In Progress):** User workspace backed by a container

**AgentSession (Phase 3 - In Progress):** Individual Claude agent processes within a user session

## Development Phases

- **Phase 1:** Basic container session management ✅
- **Phase 2:** Container reuse and attachment ✅ ← *container-reuse demo*
- **Phase 3:** Multi-agent session hierarchy 🚧 ← *session-hierarchy demo*
- **Phase 4:** NATS integration for coordination 📋
- **Phase 5:** PWA integration 📋
