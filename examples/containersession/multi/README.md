# ContainerSession Multi-Session Example

This example demonstrates concurrent container sessions coordinating via a shared workspace directory.

## What It Does

Launches 3 concurrent container sessions that communicate through file-based coordination:

1. **Producer Session**: Creates 3 task files in the shared workspace
2. **Consumer Session**: Polls for tasks, processes them, writes results
3. **Monitor Session**: Watches progress, writes final summary

## Prerequisites

- Docker Desktop or Colima running
- Go 1.23+
- Ubuntu Docker image

## Running the Example

```bash
# From project root
go run examples/containersession/multi/main.go
```

## Expected Output

```
=== ContainerSession Multi-Session Example ===
Demonstrates concurrent sessions coordinating via shared workspace.

Step 1: Connecting to Docker...
✓ Connected

Step 2: Created shared workspace: ./workspaces/multi-session/shared

Step 3: Creating Manager for all sessions...
✓ Manager created

Step 4: Launching 3 concurrent container sessions...
[Producer] Starting...
[Consumer] Starting...
[Monitor] Starting...
✓ All sessions launched

[Producer] Session <id> started
[Consumer] Session <id> started
[Monitor] Session <id> started
[Monitor] Status: 0 tasks, 0 results, Producer:false, Consumer:false
[Producer] Created task-1.txt: Process data file A
[Producer] Created task-2.txt: Generate report B
[Consumer] Processing task-1: Process data file A
[Consumer] Wrote result-1.txt
[Monitor] Status: 2 tasks, 1 results, Producer:false, Consumer:false
[Producer] Created task-3.txt: Validate results C
[Producer] All tasks created, signaling completion
[Producer] Session stopped
[Consumer] Processing task-2: Generate report B
[Consumer] Wrote result-2.txt
[Consumer] Processing task-3: Validate results C
[Consumer] Wrote result-3.txt
[Consumer] Session stopped
[Monitor] Status: 3 tasks, 3 results, Producer:true, Consumer:true
[Monitor] Both sessions completed!
[Monitor] Wrote summary.txt
[Monitor] Session stopped

Step 5: Waiting for sessions to complete...
✓ All sessions completed

Step 6: Final workspace state...
Shared workspace contents (8 files):
  consumer-done.flag        complete
  producer-done.flag        complete
  result-1.txt              Completed: Process data file A (at 12:30:15)
  result-2.txt              Completed: Generate report B (at 12:30:16)
  result-3.txt              Completed: Validate results C (at 12:30:16)
  summary.txt               Summary: 3 tasks processed successfully...
  task-1.txt                Process data file A
  task-2.txt                Generate report B
  task-3.txt                Validate results C

=== Example Complete ===

Key Takeaways:
  • Multiple sessions can run concurrently
  • Shared workspace enables inter-session communication
  • File-based coordination is simple and reliable
  • Each session has its own isolated container
```

## Key Concepts Demonstrated

### Concurrent Session Management

Three sessions run simultaneously, each in its own isolated container:

```go
var wg sync.WaitGroup

wg.Add(1)
go func() {
    defer wg.Done()
    runProducer(ctx, manager, sharedDir)
}()

wg.Add(1)
go func() {
    defer wg.Done()
    runConsumer(ctx, manager, sharedDir)
}()

wg.Wait() // Wait for all to complete
```

### Shared Workspace Coordination

Each session links to a shared directory for file-based communication:

```go
// In each session
sessionShared := filepath.Join(session.WorkspacePath(), "shared")
os.Symlink(sharedDir, sessionShared)
```

This creates a directory structure:
```
workspaces/multi-session/
├── shared/                      # Shared coordination directory
│   ├── task-1.txt              # Producer writes tasks
│   ├── task-2.txt
│   ├── task-3.txt
│   ├── result-1.txt            # Consumer writes results
│   ├── result-2.txt
│   ├── result-3.txt
│   ├── producer-done.flag      # Completion signals
│   ├── consumer-done.flag
│   └── summary.txt             # Monitor writes summary
├── <producer-session-id>/       # Producer's isolated workspace
│   └── shared -> ../shared     # Symlink to shared dir
├── <consumer-session-id>/       # Consumer's isolated workspace
│   └── shared -> ../shared
└── <monitor-session-id>/        # Monitor's isolated workspace
    └── shared -> ../shared
```

### Producer-Consumer Pattern

**Producer**: Creates work items
```go
task := "Process data file A"
taskFile := filepath.Join(sharedDir, "task-1.txt")
os.WriteFile(taskFile, []byte(task), 0644)
```

**Consumer**: Polls for work, processes, writes results
```go
taskData, err := os.ReadFile(taskFile)
// ... process task ...
result := fmt.Sprintf("Completed: %s", taskData)
os.WriteFile(resultFile, []byte(result), 0644)
```

**Monitor**: Watches progress
```go
entries, _ := os.ReadDir(sharedDir)
// Count tasks, results, check completion flags
```

### Completion Signaling

Sessions signal completion via flag files:

```go
doneFile := filepath.Join(sharedDir, "producer-done.flag")
os.WriteFile(doneFile, []byte("complete"), 0644)
```

The monitor polls for both flags to determine when all work is done.

### Timeout Handling

Each session has timeout protection:

```go
timeout := time.After(10 * time.Second)
for {
    select {
    case <-timeout:
        goto cleanup
    default:
        // Do work
    }
}
```

## Coordination Patterns

This example demonstrates **file-based coordination**, which is:

✅ **Simple**: No external dependencies
✅ **Reliable**: Filesystem operations are atomic
✅ **Debuggable**: You can inspect files manually
✅ **Cross-platform**: Works everywhere

**Limitations**:
- Not suitable for high-frequency updates (use sockets/pipes instead)
- Race conditions possible without proper locking
- Polling introduces latency

**When to use**:
- Agent coordination (like this example)
- Batch processing workflows
- Checkpoint/restart patterns
- Status reporting

## Alternative Coordination Methods

For production systems, consider:

1. **Docker Networks**: Container-to-container HTTP/gRPC communication
2. **Message Queues**: Redis, RabbitMQ, NATS for reliable messaging
3. **Shared Volumes**: More complex shared storage with proper locking
4. **Unix Sockets**: Bind-mounted sockets for IPC

This example focuses on the simplest approach to demonstrate the container session capabilities.

## Use Cases

This pattern is useful for:

- **Agent Workflows**: Multiple agents collaborating on tasks
- **Pipeline Processing**: Stage-based data processing
- **MapReduce**: Distributed task processing
- **Multi-Agent Systems**: Coordinated AI agents

## Next Steps

- See `examples/containersession/basic/` for simpler single-session example
- See `examples/containersession/echo-agent/` for I/O interaction patterns
- See `pkg/containersession/` for implementation details

## Platform Support

### Supported Platforms

**macOS and Linux:**
- Docker Desktop
- Colima (macOS)
- Docker Engine (Linux)

**Windows:**
- Requires manual `DOCKER_HOST` configuration
- Docker Desktop with WSL2 backend (recommended)
- See Windows setup instructions below

### Connection Order

The helper function (`pkg/containersession/helpers.CreateDockerClient`) attempts connection in this order:
1. **DOCKER_HOST environment variable** - Honors standard Docker configuration
2. **macOS Colima** (macOS only) - Automatic fallback to `~/.colima/default/docker.sock`
3. **Standard Unix socket** - Falls back to `/var/run/docker.sock`

### Windows Setup

The Docker client's fallback paths use Unix sockets (macOS/Linux). For Windows, configure `DOCKER_HOST` before running:

**Docker Desktop with Named Pipe:**
```powershell
# PowerShell
$env:DOCKER_HOST="npipe:////./pipe/docker_engine"

# Or Command Prompt
set DOCKER_HOST=npipe:////./pipe/docker_engine
```

**Docker Desktop with TCP (if exposed):**
```powershell
$env:DOCKER_HOST="tcp://localhost:2375"
```

**WSL2 (Recommended):**
Run the example inside WSL2 where Unix socket behavior matches macOS/Linux:
```bash
# Inside WSL2
go run examples/containersession/multi/main.go
```

## Troubleshooting

**Error: "cannot connect to Docker"**
- Ensure Docker Desktop or Colima is running
- Check: `docker ps` works in your terminal
- Windows: Verify `DOCKER_HOST` is set correctly (see Platform Support)
- Check: `echo $DOCKER_HOST` (Unix) or `echo %DOCKER_HOST%` (Windows)

**Sessions not starting**
- Check Docker is running: `docker ps`
- Verify workspace permissions

**Coordination timeouts**
- Increase timeout values for slower systems
- Check shared directory is accessible from all sessions

**Race conditions**
- Add file locking if needed (see `flock` command)
- Use atomic rename operations: `os.Rename(temp, final)`

**Symlink errors on Windows**
- Windows requires admin rights for symlinks
- Alternative: Copy files to shared directory instead
