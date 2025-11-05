# Session Hierarchy Demo

This demo showcases the **three-tier session architecture** that Ourocodus is building:

```
UserSession (User's workspace)
    └─> AgentSession(s) (Individual Claude agents)
            └─> ContainerSession (Docker container backing)
```

## Architecture Overview

### Layer 1: ContainerSession (Phase 2 - COMPLETE)
- Manages Docker container lifecycle
- **Intelligent reuse**: Reconnects to existing containers
- **State-based recovery**: Handles running/stopped/dead states
- **Cross-process attach**: Multiple managers can connect

### Layer 2: UserSession (Phase 3 - IN PROGRESS)
- User's workspace and environment
- State: ACTIVE | TERMINATED
- Can have 0-N agents running simultaneously
- Backed by a ContainerSession

### Layer 3: AgentSession (Phase 3 - IN PROGRESS)
- Individual Claude agent process
- State: SPAWNING | ACTIVE | FAILED | TERMINATED
- Conversation history and workspace
- Multiple agents can share a UserSession

## Demo Scenarios

### Scenario 1: User Session Creation
- Create UserSession with backing ContainerSession
- Show session IDs at both layers
- Verify container is running

### Scenario 2: Multi-Agent Coordination
- Create UserSession
- Spawn Agent1 ("coder")
- Spawn Agent2 ("reviewer")
- Show both agents backed by same container
- Demonstrate agent isolation within shared environment

### Scenario 3: Crash Recovery with Container Reuse
- Create UserSession with Agent
- Simulate relay server crash
- New relay server reconnects using AttachContainerSession
- Agent state preserved, ready to resume

### Scenario 4: Agent Lifecycle Management
- Create UserSession
- Spawn Agent ("analyzer")
- Send messages to agent
- Terminate agent (TERMINATED state)
- UserSession remains ACTIVE
- Spawn new Agent in same UserSession

## What This Enables

**Resilient Multi-Agent Systems:**
- Agents survive relay crashes (via container reuse)
- Multiple agents collaborate in shared workspace
- Independent agent lifecycles within user session
- Resource efficiency (one container, multiple agents)

**Production Scenarios:**
- User has "coder" agent writing code
- User spawns "reviewer" agent to critique
- Both agents access same workspace files
- Relay crashes and restarts → agents reconnect automatically

## Prerequisites

- Docker daemon running
- Go 1.23+
- Network access for ubuntu:latest image

## Quick Start

```bash
cd examples/sessions
./demo-setup.sh    # One-time setup
./demo-run.sh      # Interactive demo
./demo-reset.sh    # Cleanup
```

## Architecture Notes

This demo uses:
- **containersession** package for Docker management
- **relay/session** package for user/agent state
- Mock ACP clients (actual agent processes not spawned)
- Real Docker containers with label-based discovery

## Future Phases

**Phase 4: NATS Integration**
- Agent coordination via NATS messaging
- Event-driven state synchronization
- Distributed agent communication

**Phase 5: PWA Integration**
- Web UI for session management
- Real-time agent status updates
- Multi-agent conversation interface

## Troubleshooting

**Error: Docker daemon not running**
```
Solution: Start Docker Desktop
```

**Error: Port conflicts**
```
Solution: Run ./demo-reset.sh to clean up
```

**Error: Image pull failures**
```
Solution: Check network and Docker Hub access
```
