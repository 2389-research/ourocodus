## ContainerSession Echo Agent Example

This example demonstrates bidirectional I/O communication with an agent running inside a container session.

## What It Does

1. Creates a container session
2. Copies `echo-script.sh` into the workspace
3. Starts the container running the echo script
4. Demonstrates the pattern for bidirectional communication
5. Shows expected agent input/output behavior
6. Cleans up resources

## Prerequisites

- Docker Desktop or Colima running
- Go 1.23+
- Ubuntu Docker image

## Running the Example

```bash
# From project root
go run examples/containersession/echo-agent/main.go
```

## The Echo Agent

The `echo-script.sh` agent:
- Reads messages from stdin
- Processes each message (converts to uppercase)
- Calculates message length
- Writes responses to stdout
- Exits on "exit" or "quit" command

## Expected Output

```
=== ContainerSession Echo Agent Example ===
This demonstrates bidirectional I/O with a container agent.

Step 1: Connecting to Docker...
✓ Connected

Step 2: Creating Manager...
✓ Manager created

Step 3: Creating session...
✓ Session created: <session-id>
  Workspace: ./workspaces/echo-agent/<session-id>

Step 4: Copying echo script to workspace...
✓ Script copied to ./workspaces/echo-agent/<session-id>/echo-script.sh

Step 5: Starting container with echo agent...
✓ Container running

Step 6: Attaching to container I/O streams...
✓ I/O streams attached (conceptual - see README for full implementation)

Step 7: Demonstrating message exchange pattern...
Messages we would send to the agent:
  1. hello world
  2. test message
  3. container sessions are great

Expected agent responses:
  Message 1:
    Received: hello world
    Processed: HELLO WORLD
    Length: 11 characters
    ---
  Message 2:
    Received: test message
    Processed: TEST MESSAGE
    Length: 12 characters
    ---
  Message 3:
    Received: container sessions are great
    Processed: CONTAINER SESSIONS ARE GREAT
    Length: 30 characters
    ---

Step 8: Stopping container...
✓ Container stopped

Step 9: Cleaning up workspace...
✓ Workspace cleaned up

=== Example Complete ===

Key Takeaways:
  • Container sessions can run interactive agents
  • Scripts are deployed via the workspace directory
  • Bidirectional I/O enables agent communication
  • See README.md for full I/O stream implementation
```

## Key Concepts Demonstrated

### Script Deployment via Workspace

Scripts and configuration files can be copied into the workspace directory before starting the container. The container mounts this directory at `/workspace`, making files immediately available.

```go
scriptDst := filepath.Join(session.WorkspacePath(), "echo-script.sh")
os.WriteFile(scriptDst, scriptData, 0755)
```

### Custom Container Commands

The container is started with a custom command that executes the deployed script:

```go
session, err := manager.CreateContainerSession(ctx, "ubuntu:latest",
    []string{"/bin/bash", "/workspace/echo-script.sh"})
```

### Bidirectional I/O Pattern

**Note:** This example demonstrates the *pattern* conceptually. For a full implementation with real I/O streams, you would use:

```go
// Attach to container I/O
resp, err := dockerClient.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
    Stream: true,
    Stdin:  true,
    Stdout: true,
    Stderr: true,
})
if err != nil {
    return err
}
defer resp.Close()

// Write to agent (stdin)
go func() {
    writer := resp.Conn
    fmt.Fprintln(writer, "hello world")
    fmt.Fprintln(writer, "exit")
}()

// Read from agent (stdout)
scanner := bufio.NewScanner(resp.Reader)
for scanner.Scan() {
    fmt.Println("Agent:", scanner.Text())
}
```

The `pkg/containersession` package handles I/O attachment internally in `manager.go` when you call `StartContainerSession()`.

### Agent Lifecycle

1. **Deploy**: Copy script to workspace
2. **Start**: Container runs the script
3. **Communicate**: Bidirectional stdin/stdout
4. **Monitor**: Read agent responses
5. **Shutdown**: Send exit command or stop container

## Testing the Echo Script Directly

You can test the echo script without the Go code:

```bash
cd examples/containersession/echo-agent

# Test locally
echo -e "hello\ntest\nexit" | bash echo-script.sh

# Test in Docker
docker run -i --rm -v $(pwd):/workspace ubuntu:latest /bin/bash /workspace/echo-script.sh
# Then type messages and press Enter
# Type 'exit' to quit
```

## Use Cases

This pattern is useful for:

- **Agent Runtime**: Running AI/LLM agents in isolated containers
- **Task Processing**: Workers that process messages from a queue
- **Interactive Tools**: CLI tools that need user input
- **Test Automation**: Automated testing of interactive applications

## Next Steps

- See `examples/containersession/basic/` for simpler lifecycle example
- See `examples/containersession/multi/` for concurrent sessions
- See `pkg/containersession/manager.go` for full I/O implementation details

## Troubleshooting

**Script not executing**
- Ensure echo-script.sh has execute permissions (chmod +x)
- Check that script uses Unix line endings (not Windows CRLF)

**Container exits immediately**
- Check script syntax: `bash -n echo-script.sh`
- Verify workspace path in container: `/workspace`

**No output from agent**
- Agent output goes to container stdout/stderr
- Check container logs: `docker logs <container-id>`
