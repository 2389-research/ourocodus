# ContainerSession Basic Example

This example demonstrates the simplest container session lifecycle using the `pkg/containersession` package.

## What It Does

1. Connects to Docker (tries Colima, then Docker Desktop)
2. Creates a Manager with default dependencies
3. Creates a container session with Ubuntu image
4. Starts the container
5. Writes a file to the workspace directory
6. Reads the file back to verify persistence
7. Stops the container
8. Cleans up the workspace

## Prerequisites

- Docker Desktop or Colima must be running
- Go 1.23+
- Ubuntu Docker image will be pulled if not present

## Running the Example

```bash
# From project root
go run examples/containersession/basic/main.go
```

## Expected Output

```
=== ContainerSession Basic Example ===
This demonstrates the simplest container session lifecycle:

Step 1: Connecting to Docker...
✓ Connected to Docker

Step 2: Creating Manager...
[manager] Session created: <session-id>
✓ Manager created

Step 3: Creating container session...
✓ Session created
  Session ID: <session-id>
  Container ID: <container-id>
  State: PENDING
  Workspace: ./workspaces/basic-example/<session-id>

Step 4: Starting container...
[manager] Starting session: <session-id>
✓ Container started
  State: RUNNING

Step 5: Writing data to workspace...
✓ Wrote file: ./workspaces/basic-example/<session-id>/example.txt
  Content: Hello from containersession basic example!

Step 6: Reading data from workspace...
✓ Read file: ./workspaces/basic-example/<session-id>/example.txt
  Content: Hello from containersession basic example!

Step 7: Stopping container...
[manager] Stopping session: <session-id>
✓ Container stopped
  State: STOPPED

Step 8: Cleaning up workspace...
✓ Workspace cleaned up

=== Example Complete ===

Key Takeaways:
  • Container sessions provide isolated execution environments
  • Workspace directories persist data between host and container
  • Session lifecycle: PENDING → RUNNING → STOPPED
  • Manager handles all Docker operations transparently
```

## Key Concepts Demonstrated

### Manager Creation
The Manager requires four dependencies:
- **DockerClient**: Docker SDK client for container operations
- **IDGenerator**: Generates unique session IDs (UUID)
- **Clock**: Provides timestamps for session metadata
- **Logger**: Logs session lifecycle events

### Session Lifecycle States
- **PENDING**: Session created but container not started
- **RUNNING**: Container is running
- **STOPPED**: Container stopped gracefully
- **FAILED**: Error occurred during operations

### Workspace Directories
Each session gets an isolated workspace directory at:
```
<baseWorkspaceDir>/<session-id>/
```

Files written to this directory are:
- Accessible from the host filesystem
- Mounted into the container at `/workspace`
- Persist across container restarts
- Secured with path validation

## Next Steps

- See `examples/containersession/multi/` for concurrent sessions
- See `examples/containersession/echo-agent/` for I/O interaction
- See `pkg/containersession/integration_test.go` for testing patterns

## Troubleshooting

**Error: "cannot connect to Docker"**
- Ensure Docker Desktop or Colima is running
- Check: `docker ps` works in your terminal

**Error: "permission denied" on workspace**
- Workspace directories are created with 0700 permissions
- Ensure you have write access to the examples directory

**Container not stopping**
- The example uses `sleep 60` command which should stop gracefully
- If needed, manually clean up: `docker ps -a | grep containersession`
