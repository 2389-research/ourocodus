# Container Session Demo - Phase 2: Reuse & Attach

This demo showcases Phase 2 functionality of the containersession package: intelligent container reuse and explicit session attachment.

## Features Demonstrated

1. **Automatic Container Reuse** - CreateContainerSession checks for existing containers before creating new ones
2. **State-Based Recovery** - Handles different container states appropriately (running, stopped, dead)
3. **Cross-Process Attachment** - AttachContainerSession reconnects to running containers by session ID
4. **Workspace Persistence** - Demonstrates how workspace state survives container restarts

## Prerequisites

- Docker daemon running
- Go 1.23+ installed
- Network access to pull `ubuntu:latest` image

## Quick Start

```bash
# From repository root
cd examples/containersession

# Run the interactive demo
go run main.go

# Or build and run
go build -o demo main.go
./demo
```

## Demo Scenarios

### Scenario 1: Automatic Reuse of Running Container
1. Creates a container session
2. Starts the container
3. Simulates process restart by creating new Manager
4. Calls CreateContainerSession with same ID
5. Shows container is reused (not recreated)

### Scenario 2: Restart Stopped Container
1. Creates and starts a container
2. Stops the container
3. Calls CreateContainerSession again
4. Shows container is restarted (not recreated)

### Scenario 3: Explicit Cross-Process Attach
1. Creates a container session in "Process A"
2. Persists the session ID
3. Simulates "Process B" by creating new Manager
4. Uses AttachContainerSession to reconnect
5. Shows both processes can interact with same container

### Scenario 4: Workspace Persistence
1. Creates container and writes file to workspace
2. Stops container
3. Reuses container in new session
4. Verifies file still exists in workspace

## What to Watch For

- **Session IDs** - Same ID means same container
- **Container IDs** - Watch for container reuse vs recreation
- **State Transitions** - PENDING → RUNNING → STOPPED → RUNNING (reused)
- **Workspace Paths** - Same workspace path after reuse
- **Timestamps** - CreatedAt stays consistent across reuse

## Clean Up

The demo automatically cleans up containers and workspaces after each scenario.
To manually clean up:

```bash
# Remove all containersession containers
docker ps -a --filter "label=com.ourocodus.containersession.managed-by" -q | xargs docker rm -f

# Remove demo workspaces
rm -rf ./demo-workspaces
```

## Architecture Notes

This demo uses:
- Real Docker SDK (not mocked) for authentic behavior
- Temporary workspace directories under `./demo-workspaces`
- Standard Go logger for observability
- UUID-based session IDs for uniqueness

## Troubleshooting

**Docker daemon not running:**
```
Error: Cannot connect to the Docker daemon
Solution: Start Docker Desktop or docker daemon
```

**Permission denied:**
```
Error: permission denied while trying to connect to Docker daemon
Solution: Add your user to docker group or run with sudo
```

**Image pull failures:**
```
Error: failed to pull image ubuntu:latest
Solution: Check network connection and Docker Hub access
```

## Next Steps

After running this demo, explore:
- Integration with NATS for distributed coordination
- User session management on top of container sessions
- Agent autonomy patterns using persistent containers
