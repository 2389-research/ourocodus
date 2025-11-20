# API Key Credential File and REPL Command Design

**Date:** 2025-11-19
**Status:** Approved
**Implementing:** Credential file pattern for API keys and working REPL command

## Context

Currently, `agentd spawn` does not inject the ANTHROPIC_API_KEY, which means spawned agents cannot communicate with the Anthropic API. The relay server passes API keys as container environment variables (visible in `docker inspect`). The `agentd repl` command is a placeholder that redirects users to the PWA.

**User Requirements:**
1. API keys should be provided via `--api-key` flag or `ANTHROPIC_API_KEY` environment variable at spawn time
2. API keys should be written to mounted credential files (not container environment variables)
3. ACP processes should run as PID 1 from spawn, with stdin/stdout available
4. REPL should attach directly to running ACP process via `docker attach`
5. Both CLI and PWA should communicate with agents the same way (PWA via relay WebSocket bridge)

## Credential File Architecture

### Directory Structure

```
.agentd/worktrees/<agent-id>/.creds/
├── .env                    # ANTHROPIC_API_KEY=sk-...
├── github_token (future)   # GitHub PAT
└── id_ed25519 (future)     # SSH key
```

### Security Model

**Host Permissions:**
- `.creds/` directory: 0700 (owner only)
- `.env` file: 0600 (owner read/write only)

**Container Mount:**
- Mount point: `/root/.creds/`
- Mode: Read-only
- ACP sources `/root/.creds/.env` at startup

**Benefits:**
- API key not visible in `docker inspect` output
- Prevents container modification (read-only mount)
- Extensible for other credentials (Git SSH keys, tokens)
- Standard `.env` format (shell-sourceable)

## Changes to agentd spawn Command

### New Flag

```bash
agentd spawn [agent-id] --api-key <key>
```

- `--api-key <key>`: Explicit API key (optional)
- Falls back to `ANTHROPIC_API_KEY` environment variable
- Error if neither provided

### Implementation Flow

1. Parse `--api-key` flag or read `ANTHROPIC_API_KEY` environment variable
2. Validate key is provided (error if missing)
3. Create `.agentd/worktrees/<agent-id>/.creds/` directory with 0700 permissions
4. Write `.env` file with `ANTHROPIC_API_KEY=<key>` content, 0600 permissions
5. Configure container to mount `.creds/` as read-only at `/root/.creds/`
6. Start container with ACP as PID 1: `/usr/local/bin/acp --workspace /workspace`
7. ACP sources `/root/.creds/.env` at startup

### Error Handling

| Error Condition | User Message | Recovery |
|----------------|--------------|----------|
| Missing API key | `ANTHROPIC_API_KEY required (via --api-key flag or ANTHROPIC_API_KEY environment variable)` | Provide key via flag or env |
| Permission denied | `Failed to create credentials directory: permission denied at <path>` | Check file system permissions |
| Container start failure | `Container failed to start. Check credentials mount: <details>` | Verify Docker and mount config |
| Invalid key format | `Warning: API key should start with 'sk-'` | Verify key is correct |

## Changes to Relay (pkg/relay and pkg/agent)

### Current Behavior

The relay passes `ANTHROPIC_API_KEY` as a container environment variable:

```go
// pkg/agent/factory.go:138-140
if a.launcherConfig.AnthropicKey != "" {
    env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", a.launcherConfig.AnthropicKey))
}
```

### New Behavior

**In `pkg/relay/session/manager.go` (SpawnAgent method, ~line 244):**

Before spawning container:
1. Extract `anthropicKey` from `ACPClientFactory`
2. Create `.creds/` directory in workspace: `<workspace>/.creds/`
3. Write `.env` file with API key
4. Pass credentials path to launcher

**In `pkg/agent/factory.go` (prepareSpawnConfig method):**

Remove environment variable injection (lines 136-140):
```go
// REMOVED: Don't pass API key as container environment variable
// if a.launcherConfig.AnthropicKey != "" {
//     env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", a.launcherConfig.AnthropicKey))
// }
```

Add credential file mount:
```go
// Add .creds directory as read-only mount
mounts = append(mounts, container.Mount{
    Type:     "bind",
    Source:   filepath.Join(workspace, ".creds"),
    Target:   "/root/.creds",
    ReadOnly: true,
})
```

### Backward Compatibility

- Existing containers without `.creds/` will fail gracefully with clear error
- No migration path needed (agents are ephemeral, just respawn)
- Error message guides users to stop and respawn agent with new version

## REPL Command Implementation

### Command Signature

```bash
agentd repl <agent-id>
```

### Implementation

**High-Level Flow:**
1. Find agent by ID using `listAgentsFromDocker()`
2. Verify agent is running (error if stopped/missing)
3. Use Docker API `ContainerAttach()` with `stdin=true`, `stdout=true`, `stderr=true`, `stream=true`
4. Set terminal to raw mode (disable line buffering and echo)
5. Bidirectional I/O copy: host stdin → container, container → host stdout
6. Handle signals: Ctrl+D exits cleanly, Ctrl+C sends SIGINT to container
7. Restore terminal state on exit

**Terminal Handling:**

```go
// Save terminal state
oldState, err := term.GetState(int(os.Stdin.Fd()))
if err != nil {
    return fmt.Errorf("failed to get terminal state: %w", err)
}
defer term.Restore(int(os.Stdin.Fd()), oldState)

// Set raw mode
if _, err := term.MakeRaw(int(os.Stdin.Fd())); err != nil {
    return fmt.Errorf("failed to set raw mode: %w", err)
}
```

**Docker Attach:**

```go
attachOptions := container.AttachOptions{
    Stream: true,
    Stdin:  true,
    Stdout: true,
    Stderr: true,
}

resp, err := dockerClient.ContainerAttach(ctx, containerID, attachOptions)
if err != nil {
    return fmt.Errorf("failed to attach to container: %w", err)
}
defer resp.Close()
```

**Bidirectional Copy:**

```go
// Copy container output to stdout
go func() {
    io.Copy(os.Stdout, resp.Reader)
}()

// Copy stdin to container
io.Copy(resp.Conn, os.Stdin)
```

### User Experience

```bash
$ agentd repl alice
✓ Connected to agent 'alice' (container: a1b2c3d4e5f6)
  Press Ctrl+D to exit, Ctrl+C to interrupt

> Hello agent!
Echo: Hello agent!
> help
Echo: help
> ^D

✓ Disconnected from agent 'alice'
```

### Error Handling

| Error Condition | User Message | Recovery |
|----------------|--------------|----------|
| Agent not found | `Agent 'alice' not found. Use 'agentd list' to see running agents` | Check agent ID, spawn if needed |
| Agent not running | `Agent 'alice' is stopped. Use 'agentd spawn alice' to start it` | Start the agent |
| Container unresponsive | `Timeout: agent 'alice' is not responding (10s). Retry? [y/N]` | Check container health, logs |
| Terminal not available | `Warning: Terminal not available, running in non-interactive mode` | Use send command instead |
| Attach fails | `Failed to attach to container: <docker error>` | Check Docker daemon, permissions |

## Testing Strategy

### Unit Tests

**API Key Handling (cmd/agentd/cmd_spawn_test.go):**
- Test `--api-key` flag parsing
- Test fallback to `ANTHROPIC_API_KEY` environment variable
- Test error when neither provided
- Test credential file creation with correct permissions
- Test credential file content format

**REPL Command (cmd/agentd/cmd_repl_test.go):**
- Test argument validation (requires agent ID)
- Test error when agent not found
- Test error when agent not running
- Test terminal state save/restore
- Mock Docker attach for non-TTY test environment

### Integration Tests

**API Key Flow (cmd/agentd/cmd_integration_test.go):**
- Test end-to-end spawn with API key
- Verify `.creds/.env` file exists in workspace
- Verify file permissions (0700 directory, 0600 file)
- Verify container has read-only mount at `/root/.creds/`
- Verify ACP process can read the API key

**REPL Workflow (cmd/agentd/cmd_integration_test.go):**
- Test spawn → attach → send message → receive response workflow
- Test Ctrl+D exit behavior
- Test attach to already-running agent
- Test multiple sequential REPL sessions

**Relay Migration (pkg/relay/session/manager_test.go):**
- Test relay creates `.creds/` directory
- Test relay writes `.env` file
- Test relay no longer passes API key as environment variable
- Verify container has credential mount

## Implementation Checklist

- [ ] Add `--api-key` flag to spawn command
- [ ] Implement credential file writing in spawn command
- [ ] Update container mount configuration for credentials
- [ ] Remove API key from container environment in pkg/agent/factory.go
- [ ] Add credential file creation in pkg/relay/session/manager.go
- [ ] Implement REPL command with docker attach
- [ ] Add terminal handling (raw mode, state save/restore)
- [ ] Add signal handling (Ctrl+C, Ctrl+D)
- [ ] Write unit tests for API key handling
- [ ] Write unit tests for REPL command
- [ ] Write integration tests for end-to-end workflow
- [ ] Update agentd.md documentation
- [ ] Update relay session documentation

## Security Considerations

1. **API Key Visibility**: Credential file approach prevents keys from appearing in `docker inspect` output or process environment listings
2. **File Permissions**: Strict permissions (0700/0600) prevent other users from reading keys
3. **Read-Only Mount**: Container cannot modify credential files, preventing key exfiltration
4. **No Logging**: API keys are never logged or printed to stdout/stderr
5. **Cleanup**: Credential files are cleaned up when agent is stopped (along with worktree)

## Future Enhancements

1. **Multiple Credential Types**: Extend `.creds/` to support GitHub tokens, SSH keys
2. **Credential Rotation**: Support updating API keys for running agents
3. **Relay Fallback**: Add `--relay` flag to REPL for remote agent support
4. **REPL History**: Add command history persistence (readline support)
5. **Multi-Agent REPL**: Interactive agent selector if no ID provided
