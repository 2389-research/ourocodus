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
