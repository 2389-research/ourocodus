# Claude Code Container Integration Design

**Date:** 2025-12-02
**Status:** Implemented
**Implementing:** Running official claude-code-acp in Docker containers with ACP stdio communication
**Consensus:** GPT-5 (8/10), O3 (8/10), GPT-5-mini (8/10) - All approved with hardening requirements

## Context

The project currently uses an echo-agent placeholder in `Dockerfile.agent`. We need to run the official Claude Code agent (via `@zed-industries/claude-code-acp`) inside containers for:

1. **Multi-agent orchestration**: Relay service spawns and coordinates multiple Claude Code instances
2. **Sandboxed execution**: Isolated environments with controlled credentials and tool access

This design incorporates lessons from the `packnplay` project (credential handling, agent abstraction) and maintains compatibility with the existing credential file architecture from `docs/plans/2025-11-19-api-key-credential-file-and-repl-design.md`.

## Research Summary

### claude-code-acp Analysis

The `@zed-industries/claude-code-acp` package is the official ACP adapter for Claude Code:

- **Entry point**: TypeScript Node.js application
- **Communication**: ACP (Agent Client Protocol) over stdio using ndJSON framing
- **Dependencies**: `@anthropic-ai/claude-agent-sdk`, `@anthropic-ai/claude-code`
- **Authentication**: Reads `ANTHROPIC_API_KEY` from environment

Key code from `src/acp-agent.ts`:
```typescript
export function runAcp() {
  const input = nodeToWebWritable(process.stdout);
  const output = nodeToWebReadable(process.stdin);
  const stream = ndJsonStream(input, output);
  new AgentSideConnection((client) => new ClaudeAcpAgent(client), stream);
}
```

### packnplay Credential Patterns

The `packnplay` project handles Claude credentials via:

1. **Host directory mount**: Mounts `~/.claude` directory into container
2. **Credential overlay**: Overlays `.credentials.json` file on top
3. **Credential detection**: Checks if host credential file exists and has content (>20 bytes)

Key code from `pkg/runner/runner.go`:
```go
hostCredFile := filepath.Join(homeDir, ".claude", ".credentials.json")
if stat, err := os.Stat(hostCredFile); err == nil && stat.Size() >= 20 {
    hostHasCredentials = true
}
// Overlay mount credential file after .claude directory mount
if needsCredentialOverlay {
    args = append(args, "-v", fmt.Sprintf("%s:/home/%s/.claude/.credentials.json", credentialFile, devConfig.RemoteUser))
}
```

## Design Decisions

### Multi-Model Consensus

This design was validated with three AI models (GPT-5, O3, GPT-5-mini) via Zen consensus. All agreed on the approach with these refinements:

| Decision | Choice | Rationale |
|----------|--------|-----------|
| ACP Runtime | Official `claude-code-acp` | Maintained by Zed, uses official Anthropic SDK |
| Communication | stdio via Docker attach | Matches existing `ContainerAttachProcessLauncher` |
| Image Strategy | Base + overlay | Separates Node.js tooling from claude-code specifics |
| Init Process | tini | Proper signal handling, zombie reaping |
| Container User | `node` (UID 1000) | Uses existing user from Node.js base image |
| Credential Primary | `.creds/.env` file injection | Portable, matches existing design |
| Credential Fallback | `~/.claude` directory mount | For users with existing Claude CLI auth |

### Consensus Refinements (Must Address)

All three models identified these critical items:

1. **Health Check**: Replace fragile `pgrep` with PID file check
2. **Security Claim Correction**: Document that API key IS exposed to process env via `source`
3. **Runtime Hardening**: Add `--read-only`, `--cap-drop=ALL`, `--no-new-privileges`
4. **Resource Limits**: Add CPU/memory constraints
5. **Environment**: Set `ENV HOME=/home/node` explicitly

### Credential Strategy (Dual-Mode)

**Primary Method: `.creds/.env` file injection** (recommended)
- Uses existing credential infrastructure
- API key written to `.creds/.env` as `ANTHROPIC_API_KEY=sk-...`
- Container mounts `.creds/` read-only at `/home/node/.creds/`
- Entry script sources this file

**Fallback Method: Host `~/.claude` mount** (optional)
- For users who authenticate via `claude login` CLI
- Mounts `~/.claude` to `/home/node/.claude` read-only
- Preserves existing OAuth sessions and settings

**Why Both:**
- Primary method is portable and CI-friendly (just needs API key)
- Fallback supports local development with existing Claude CLI auth
- Container code checks for primary first, falls back to mounted credentials

## Architecture

### Container Image Strategy

```
Dockerfile.claude-code-base (shared foundation)
├── Node.js 22 (bookworm-slim)
├── tini (init process)
├── Common tools (git, curl, etc.)
└── node user (UID 1000, from base image)

Dockerfile.agent (application layer)
├── FROM claude-code-base
├── npm install -g @zed-industries/claude-code-acp
└── Entry script with credential sourcing
```

### Dockerfile.claude-code-base

```dockerfile
# Base image for Claude Code agents
# Provides Node.js runtime, init process, and common tooling
FROM node:22-bookworm-slim

# Install tini for proper PID 1 behavior
# tini handles signal forwarding and zombie reaping
RUN apt-get update && apt-get install -y --no-install-recommends \
    tini \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Use existing node user (UID 1000) from base image
# Set HOME explicitly for consistent credential resolution
ENV HOME=/home/node

# Create standard directories
RUN mkdir -p /workspace /home/node/.creds /home/node/.claude \
    && chown -R node:node /workspace /home/node

# Set working directory
WORKDIR /workspace

# Switch to non-root user for safety
USER node

# Use tini as init
ENTRYPOINT ["/usr/bin/tini", "--"]
```

### Dockerfile.agent (Updated)

```dockerfile
# Claude Code ACP Agent
# Runs official claude-code-acp with proper init and credential handling
FROM ourocodus/claude-code-base:latest

# Switch back to root for package installation
USER root

# Install claude-code-acp globally
# Pin to specific version for reproducibility
RUN npm install -g @zed-industries/claude-code-acp@0.10.10

# Copy entry script and health check
COPY scripts/claude-code-entry.sh /usr/local/bin/
COPY scripts/healthcheck.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/claude-code-entry.sh /usr/local/bin/healthcheck.sh

# Health check: 3-level verification (PID file + process running + process identity)
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD /usr/local/bin/healthcheck.sh

# Switch to non-root user
USER node

# Entry script handles credential sourcing and starts claude-code-acp
CMD ["/usr/local/bin/claude-code-entry.sh"]
```

### Entry Script (scripts/claude-code-entry.sh)

```bash
#!/bin/bash
set -e

# Cleanup PID file on exit
cleanup() {
    rm -f /tmp/claude-code.pid
}
trap cleanup EXIT

# Credential sourcing with fallback
# Priority: 1) .creds/.env file  2) ~/.claude directory  3) Error
#
# SECURITY NOTE: Sourcing .env exposes API key to process environment.
# This is visible via /proc/$pid/environ to anyone with container exec access.
# Mitigations: read-only rootfs, drop capabilities, no-new-privileges.

if [ -f "/home/node/.creds/.env" ]; then
    echo "[claude-code] Sourcing credentials from .creds/.env" >&2
    # Validate .env format before sourcing (basic safety check)
    if grep -qE '^[A-Z_][A-Z0-9_]*=' /home/node/.creds/.env 2>/dev/null; then
        set -a
        source /home/node/.creds/.env
        set +a
    else
        echo "[claude-code] ERROR: Invalid .env format" >&2
        exit 1
    fi
elif [ -f "/home/node/.claude/.credentials.json" ]; then
    echo "[claude-code] Using existing Claude credentials from ~/.claude" >&2
    # claude-code-acp will read from standard location
else
    echo "[claude-code] ERROR: No credentials found" >&2
    echo "[claude-code] Provide ANTHROPIC_API_KEY via .creds/.env or mount ~/.claude" >&2
    exit 1
fi

# Verify API key is available (unless using OAuth from ~/.claude)
if [ -z "$ANTHROPIC_API_KEY" ] && [ ! -f "/home/node/.claude/.credentials.json" ]; then
    echo "[claude-code] ERROR: ANTHROPIC_API_KEY not set and no Claude credentials found" >&2
    exit 1
fi

# Write PID file for health check (before exec replaces this process)
# The node process will inherit this PID
echo $$ > /tmp/claude-code.pid

# Start claude-code-acp
# stdin/stdout are used for ACP protocol communication
exec claude-code-acp --workspace /workspace "$@"
```

## Integration Points

### 1. ContainerAttachProcessLauncher

The existing `pkg/relay/session/container_attach_process_launcher.go` already supports stdio attachment. Changes needed:

```go
// Current: Works with any container exposing stdin/stdout
// No changes needed to the launcher itself

// The ACP protocol (ndJSON) flows through:
// Client -> WebSocket -> Relay -> Docker Attach -> Container stdin
// Container stdout -> Docker Attach -> Relay -> WebSocket -> Client
```

### 2. Credential Mounter (pkg/agent/container/credentials.go)

Extend existing credential infrastructure:

```go
// Add new credential type for claude-code
type ClaudeCredentialMounter struct {
    credsDir string  // Path to .creds directory
    claudeDir string // Optional path to ~/.claude directory
}

func (m *ClaudeCredentialMounter) GetMounts() []container.Mount {
    var mounts []container.Mount

    // Primary: .creds directory with .env file
    if m.credsDir != "" {
        mounts = append(mounts, container.Mount{
            Type:     "bind",
            Source:   m.credsDir,
            Target:   "/home/node/.creds",
            ReadOnly: true,
        })
    }

    // Fallback: ~/.claude directory if exists
    if m.claudeDir != "" && dirExists(m.claudeDir) {
        mounts = append(mounts, container.Mount{
            Type:     "bind",
            Source:   m.claudeDir,
            Target:   "/home/node/.claude",
            ReadOnly: true,
        })
    }

    return mounts
}
```

### 3. Agent Factory (pkg/agent/factory.go)

Update spawn configuration for Claude Code agent:

```go
func (f *Factory) prepareClaudeCodeConfig(workspace string) (*SpawnConfig, error) {
    config := &SpawnConfig{
        Image:      "ghcr.io/ourocodus/agent:latest",
        Entrypoint: []string{"/usr/bin/tini", "--"},
        Cmd:        []string{"/usr/local/bin/claude-code-entry.sh"},
        WorkDir:    "/workspace",
        User:       "node",
    }

    // Add credential mounts (already handled by existing credential infrastructure)
    credMounter := container.NewClaudeCredentialMounter(
        filepath.Join(workspace, ".creds"),
        os.Getenv("HOME") + "/.claude", // Optional fallback
    )
    config.Mounts = append(config.Mounts, credMounter.GetMounts()...)

    return config, nil
}
```

### 4. Health Monitoring

Add health check integration to session manager:

```go
// Check container health via Docker API
func (s *SessionManager) checkAgentHealth(containerID string) error {
    inspect, err := s.docker.ContainerInspect(ctx, containerID)
    if err != nil {
        return fmt.Errorf("inspect failed: %w", err)
    }

    if inspect.State.Health != nil {
        switch inspect.State.Health.Status {
        case "healthy":
            return nil
        case "unhealthy":
            return fmt.Errorf("agent unhealthy: %s",
                inspect.State.Health.Log[0].Output)
        case "starting":
            return fmt.Errorf("agent still starting")
        }
    }

    // Fallback: check if process is running
    if !inspect.State.Running {
        return fmt.Errorf("container not running")
    }

    return nil
}
```

## Message Flow

```
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌──────────────────┐
│  PWA    │────▶│  Relay  │────▶│   Docker    │────▶│  claude-code-acp │
│ Client  │◀────│ Server  │◀────│   Attach    │◀────│  (PID 1 via tini)│
└─────────┘     └─────────┘     └─────────────┘     └──────────────────┘
    │               │                 │                      │
    │   WebSocket   │    Docker API   │       stdio          │
    │   (JSON-RPC)  │                 │      (ndJSON)        │
```

## Security Considerations

1. **API Key Handling** (Updated per consensus)
   - Keys stored in `.creds/.env` with 0600 permissions
   - Container mounts read-only (cannot modify credentials)
   - Keys NOT visible in `docker inspect` output
   - **Note**: API key IS exposed in process environment after sourcing
   - Visible via `/proc/$pid/environ` to anyone with `docker exec` access
   - Mitigate with: read-only rootfs, drop capabilities, restrict exec access

2. **Container Isolation**
   - Non-root user (`node`, UID 1000)
   - Read-only credential mounts
   - Limited to workspace directory for file operations

3. **Runtime Hardening** (Required per consensus)
   ```bash
   docker run \
     --read-only \
     --cap-drop=ALL \
     --security-opt=no-new-privileges \
     --tmpfs /tmp:noexec,nosuid,size=256m \
     --memory=2g \
     --cpus=2 \
     ghcr.io/ourocodus/agent:latest
   ```

   Resource limits are configurable via environment variables:
   - `AGENT_MEMORY_LIMIT_MB` (default: 2048)
   - `AGENT_CPU_LIMIT` (default: 2.0)
   - `AGENT_TMPFS_SIZE_MB` (default: 256)

4. **Signal Handling**
   - tini ensures proper SIGTERM/SIGINT handling
   - Graceful shutdown on container stop
   - No zombie process accumulation

5. **Network Security**
   - Container network isolated by default
   - Only exposed via relay WebSocket bridge
   - No direct network access to containers

## Testing Strategy

### Unit Tests

```go
// pkg/agent/container/credentials_test.go
func TestClaudeCredentialMounter_GetMounts(t *testing.T) {
    tests := []struct {
        name      string
        credsDir  string
        claudeDir string
        wantCount int
    }{
        {"only creds", "/tmp/creds", "", 1},
        {"only claude", "", "/tmp/claude", 1},
        {"both", "/tmp/creds", "/tmp/claude", 2},
        {"neither", "", "", 0},
    }
    // ...
}
```

### Integration Tests

```go
// cmd/agentd/cmd_integration_test.go
func TestClaudeCodeSpawn(t *testing.T) {
    // 1. Create test workspace with .creds/.env
    // 2. Spawn claude-code container
    // 3. Verify container starts with correct mounts
    // 4. Send ACP message via attach
    // 5. Verify response
    // 6. Stop and verify cleanup
}
```

### Container Tests

```bash
# Test credential sourcing
docker run --rm \
  -v /tmp/test-creds:/home/node/.creds:ro \
  ourocodus/agent:latest \
  /bin/bash -c "source /home/node/.creds/.env && echo \$ANTHROPIC_API_KEY"

# Test health check
docker run -d --name test-agent ourocodus/agent:latest
sleep 35  # Wait for health check
docker inspect --format='{{.State.Health.Status}}' test-agent
docker rm -f test-agent
```

## Implementation Checklist

### Phase 1: Base Image
- [x] Create `Dockerfile.claude-code-base` with Node.js + tini
- [x] Add to CI/CD pipeline for building
- [x] Push to ghcr.io/ourocodus/claude-code-base

### Phase 2: Agent Image
- [x] Update `Dockerfile.agent` to use base image
- [x] Create `scripts/claude-code-entry.sh` entry script
- [x] Create `scripts/healthcheck.sh` with 3-level verification
- [x] Test credential sourcing (both methods)

### Phase 3: Integration
- [x] Add `RuntimeHardening` struct to `pkg/agent/container/types.go` (as alias)
- [x] Implement hardening in `pkg/containersession/config.go` and `manager.go`
- [x] Update `pkg/agent/container/launcher.go` to pass hardening options
- [x] Update `pkg/agent/factory.go` with default hardening and env var overrides
- [x] Add parity test to ensure RuntimeHardening types stay in sync

### Phase 4: Documentation
- [x] Update `Dockerfile.agent` inline documentation
- [x] Add container troubleshooting guide (`docs/container-troubleshooting.md`)
- [x] Create implementation plan (`docs/plans/2025-12-02-claude-code-container-implementation.md`)

## Future Enhancements

1. **MCP Server Support**: Mount additional MCP servers into container
2. **Tool Permissions**: Fine-grained control over which tools claude-code can use
3. **Credential Rotation**: Support updating API keys for running agents
4. **Multi-Model Support**: Support for other ACP-compatible agents (not just Claude)
5. **Docker/K8s Secrets**: Native secrets integration instead of file mounts (per consensus)
6. **Multi-arch Builds**: arm64 support for Apple Silicon
7. **Automated CVE Scanning**: Trivy/Grype integration in CI pipeline
8. **SBOM Generation**: Software Bill of Materials for provenance

## Appendix: Version Pinning

| Component | Version | Notes |
|-----------|---------|-------|
| Node.js | 22 (LTS) | bookworm-slim base |
| claude-code-acp | 0.10.10 | Pin to tested version |
| tini | latest | Stable, rarely changes |
| @anthropic-ai/claude-code | 2.0.55 | Via claude-code-acp deps |

## References

- [claude-code-acp source](https://github.com/zed-industries/claude-code-acp)
- [packnplay credential handling](https://github.com/obra/packnplay)
- [Existing credential design](./2025-11-19-api-key-credential-file-and-repl-design.md)
- [tini documentation](https://github.com/krallin/tini)
