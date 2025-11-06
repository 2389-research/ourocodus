# Ourocodus Examples

Interactive demonstrations and educational resources for the Ourocodus system. Examples range from basic introductions to advanced performance testing.

## Directory Structure

Examples are organized by purpose and difficulty level:

```
examples/
├── basic-demo/           # 🎬 Basic relay and agent interaction
├── interactive-repl/     # 🎮 Interactive testing and exploration
├── smoke-tests/          # 🧪 Manual verification tests
├── container-reuse/      # 🐳 Container reuse patterns (Phase 2)
└── nats-basic/           # 📡 NATS pub/sub demo
```

Note: Additional `containersession/` examples (basic, echo-agent, multi) are available in PR #148.

## Available Examples

### Getting Started Examples

#### 🎬 Basic Demo (`basic-demo/`)

**Difficulty:** Beginner | **Duration:** 2-3 minutes

**What it shows:** Fundamental relay and agent interaction - the best starting point for understanding Ourocodus.

**Run it:**
```bash
cd examples/basic-demo
go run main.go
```

**Features demonstrated:**
- Session lifecycle (create, use, terminate)
- Agent spawning and communication
- Message routing through relay
- Error handling and recoverability

**Use case:** Learning the core concepts of how Ourocodus routes messages between clients and agents.

[📖 Full guide](./basic-demo/README.md)

---

#### 🎮 Interactive REPL (`interactive-repl/`)

**Difficulty:** Intermediate | **Duration:** Open-ended

**What it shows:** Interactive command-line interface for manual testing and exploration.

**Run it:**
```bash
cd examples/interactive-repl
go run main.go
```

**Prerequisites:** Understanding of basic demo concepts

**Features demonstrated:**
- Manual session and agent management
- Custom message sending
- System state inspection
- WebSocket protocol exploration

**Use case:** Experimenting with the system, debugging, learning by doing.

[📖 Full guide](./interactive-repl/README.md)

---

### Testing Examples

#### 🧪 Smoke Tests (`smoke-tests/`)

**Difficulty:** Intermediate | **Duration:** 1-2 minutes per test

**What it shows:** Quick verification tests for core functionality.

**Run it:**
```bash
# Test relay server
cd examples/smoke-tests/relay && go run main.go

# Test session management
cd examples/smoke-tests/session && go run main.go
```

**Features demonstrated:**
- Component-level testing
- Relay server validation
- Session management verification
- Integration testing patterns

**Use case:** Quick checks after code changes to ensure nothing broke.

[📖 Full guide](./smoke-tests/README.md)

---

### Container and Infrastructure Examples

#### 🐳 Container Reuse Demo (`container-reuse/`)

**What it shows:** Phase 2 container management - intelligent reuse and cross-process attachment

**Run it:**
```bash
cd examples/container-reuse
./demo-container-reuse.sh
```

**Features demonstrated:**
- Automatic container discovery and reuse
- Reconnecting to running containers instead of creating new ones
- Restarting stopped containers
- Cross-process attachment (relay crashes and recovers)
- Workspace persistence across container lifecycle

**Use case:** Understanding how Ourocodus minimizes container churn and enables relay server crash recovery.

### NATS Basic Messaging Demo (`nats-basic/`)

**What it shows:** Basic NATS publish/subscribe messaging patterns

**Run it:**
```bash
cd examples/nats-basic
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

### First-Time Setup

All examples require building the project first:

```bash
make build
```

### Running Examples

Navigate to the example directory and follow its instructions:

```bash
# 🎬 Basic demo - Best starting point!
cd examples/basic-demo && go run main.go

# 🎮 Interactive REPL - Manual exploration
cd examples/interactive-repl && go run main.go

# 🧪 Smoke tests - Quick verification
cd examples/smoke-tests/relay && go run main.go

# 🐳 Container reuse - Infrastructure demo
cd examples/container-reuse && ./demo-container-reuse.sh

# 📡 NATS messaging - Requires NATS server
cd examples/nats-basic && ./demo-nats-basic.sh
```

### Recommended Learning Path

```
1. basic-demo       → Understand fundamentals
   ↓
2. interactive-repl → Explore and experiment
   ↓
3. smoke-tests      → Learn testing approaches
```

## Prerequisites

- **Docker Desktop** running
- **Go 1.23+** installed
- Network access (to pull ubuntu:latest image)

## Demo Structure

Each demo is in its own subdirectory with:
- `main.go` - Demo implementation
- `*.sh` - Runner scripts (where applicable)
- `README.md` - Documentation (for containersession/)

All demos:
- Are self-contained in subdirectories
- Build and run independently
- Clean up resources automatically
- Use color-coded output for clarity
- Support pause-driven presentation flow (where applicable)
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

**ContainerSession (Phase 2 - Complete):**
Docker container lifecycle management with intelligent reuse and cross-process attachment.

**Future Phases:**
- Phase 3: AgentLauncher implementation using ContainerSession (1:1 Agent:Container mapping)
- Phase 4: NATS integration for agent coordination
- Phase 5: PWA integration

## Development Phases

- **Phase 1:** Basic container session management ✅
- **Phase 2:** Container reuse and attachment ✅ ← *demo-container-reuse shows this*
- **Phase 3:** AgentLauncher implementation (1:1 Agent:Container) 📋
- **Phase 4:** NATS integration for coordination 📋 ← *demo-nats-basic shows basics*
- **Phase 5:** PWA integration 📋
