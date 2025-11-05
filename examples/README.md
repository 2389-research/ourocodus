# Ourocodus Demos

Interactive demonstrations of Ourocodus features. All demos are self-contained at the top level of this directory.

## Directory Structure

Examples are organized by feature area, with each demo in its own subdirectory:

```
examples/
├── container-reuse/    # Container reuse patterns (Phase 2 demo)
│   ├── main.go
│   └── *.sh           # Runner scripts
└── nats-basic/         # NATS pub/sub demo
    ├── main.go
    └── *.sh           # Runner scripts
```

Note: Additional `containersession/` examples (basic, echo-agent, multi) are available in PR #148.

## Available Demos

### Container Reuse Demo (`container-reuse/`)

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

Navigate to the demo directory and run its script:

```bash
# Container reuse demo
cd examples/container-reuse && ./demo-container-reuse.sh

# NATS messaging demo (requires NATS server)
cd examples/nats-basic && ./demo-nats-basic.sh

# Manual cleanup if needed
cd examples/container-reuse && ./demo-container-reuse-reset.sh
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
