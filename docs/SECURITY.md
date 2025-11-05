# Security Considerations

## Command Injection Prevention

### ACP Client Command Execution

**Location:** `pkg/acp/client.go:64`

**Finding:** `golangci-lint` flags `exec.Command()` with a configurable path as potential command injection (G204).

**Justification:** This usage is safe because:

1. **Not user input:** The command path is supplied via `WithCommand()` `ClientOption`, never from user or HTTP input.
2. **Controlled context:** Only used in tests (mock binaries), developer-configured installations, or the default `"claude-code-acp"` binary.
3. **Workspace validation:** The workspace parameter (user input) becomes a flag value and is not concatenated into the command.
4. **No shell execution:** Uses `exec.Command()` directly, bypassing the shell and argument parsing vulnerabilities.

**Example safe usage:**

```go
// Test with mock command
client, _ := acp.NewClient(workspace, apiKey,
 acp.WithCommand("./testdata/mock-acp"))

// Production with default binary
client, _ := acp.NewClient(workspace, apiKey)
```

**Mitigation (future):** If necessary, add allowlist validation:

```go
if !isAllowedCommand(cfg.commandPath) {
 return nil, fmt.Errorf("command path not in allowlist")
}
```

### API Key Handling

Current approach:

- API key passed via environment variable (`ANTHROPIC_API_KEY`)
- Not logged or exposed in error messages
- Cleared from memory once the process exits

Future considerations:

- Integrate with a secret management service
- Add automated key rotation in Phase 2

## Session Manager Constructor Rationale

**Location:** `pkg/relay/session/manager.go:44-58`

**Question:** Should the constructor panic on nil dependencies or return errors?

### Option A: Keep Panics (**recommended**)

- Panics surface misconfiguration immediately during service startup.
- Nil dependencies indicate programmer error, not runtime failure.
- All callers use dependency injection that ensures dependencies exist.
- Aligns with Phase 1 "fail-fast" philosophy.

### Option B: Return Errors

- Allows graceful degradation when dependencies are optional.
- Requires propagating error handling through all callers.

**Decision:** Retain panics with clear commentary. If requirements change in Phase 2, convert to `(*Manager, error)` and update callers.

## Container Session Workspace Validation

**Location:** `pkg/containersession/workspace.go:48`

**Threat:** Directory Traversal via Malicious Container Labels

A malicious actor could create a container with valid Ourocodus session labels but mount an arbitrary host path at the `/workspace` mount point. Without validation, the system would accept this container during reuse/attach operations, potentially exposing sensitive host directories.

### Attack Vector

1. Attacker creates container with labels:
   - `com.ourocodus.containersession.session-id=target-session`
   - `com.ourocodus.containersession.managed-by=ourocodus`
2. Container mounts `/etc` or `/root` at `/workspace`
3. System discovers container during `AttachContainerSession()`
4. **Without validation:** System grants access to attacker's container
5. **With validation:** System rejects container with `ErrInvalidWorkspacePath`

### Mitigation: ValidateWorkspacePath()

The validation function ensures workspace paths are descendants of the configured base directory using defense-in-depth:

1. **Prefix check:** Ensures path starts with base directory + separator
   - Prevents: `/var/workspaces-evil` bypassing `/var/workspaces`
2. **Relative path check:** Uses `filepath.Rel()` to detect `..` traversal
   - Prevents: `/var/workspaces/../../../etc` attacks
3. **Absolute path check:** Ensures relative result is not absolute
   - Prevents: Symbolic link attacks

### Implementation

**Applied in two code paths:**

1. **Container Reuse** (`manager.go:218`):
   ```go
   // Validate workspace path to prevent directory traversal attacks
   if err := ValidateWorkspacePath(m.baseWorkspaceDir, workspacePath); err != nil {
       return nil, fmt.Errorf("invalid workspace mount for container %s: %w", containerID, err)
   }
   ```

2. **Container Attach** (`manager.go:382`):
   ```go
   // Validate workspace path to prevent directory traversal attacks
   if err := ValidateWorkspacePath(m.baseWorkspaceDir, workspacePath); err != nil {
       return nil, fmt.Errorf("invalid workspace mount for container %s: %w", containerID, err)
   }
   ```

### Configuration

Set the base workspace directory when creating the manager:

```go
manager := containersession.NewManager(
    dockerClient,
    idGen,
    clock,
    logger,
    "/var/workspaces", // Base directory for validation
)
```

All container workspace mounts must be under this base directory.

### Testing

See `pkg/containersession/workspace_test.go` for validation test cases covering:
- Valid paths under base directory
- Directory traversal attempts with `..`
- Absolute path injection
- Directory name bypass attempts

### Related

- Implementation: PR #146 (Phase 2: Container Reuse & Attach)
- Godoc: `pkg/containersession.ValidateWorkspacePath()`
