# agentd - Multi-Agent Isolation CLI

`agentd` is a command-line tool that demonstrates Ourocodus's three-layer isolation architecture for running multiple AI coding agents concurrently without conflicts.

## Three-Layer Isolation

- 🌳 **Git Worktrees** - Isolated code workspaces on dedicated branches
- 📦 **Docker Containers** - Isolated processes with resource limits
- 🔑 **Credentials** - Mounted read-only for security

## Installation

```bash
# Build the binary
make build

# Binary will be in bin/agentd
```

## Quick Start

```bash
# Set Docker host (for Colima users)
export DOCKER_HOST=unix:///Users/$USER/.colima/default/docker.sock

# Validate your environment
agentd doctor

# Spawn an agent
agentd spawn alice

# List running agents
agentd list

# View agent logs
agentd logs alice

# Stop an agent (cleans up everything)
agentd stop alice
```

## Commands

### ✨ spawn - Create an Isolated Agent

Spawns a new agent in an isolated environment.

```bash
# Spawn with auto-generated ID
agentd spawn --api-key sk-...

# Spawn with custom ID
agentd spawn alice --api-key sk-...

# Spawn using environment variable for API key
export ANTHROPIC_API_KEY=sk-...
agentd spawn alice

# Spawn with custom Docker image
agentd spawn bob --api-key sk-... --image ourocodus/agent:dev

# Spawn with environment variables
agentd spawn charlie --api-key sk-... --env "DEBUG=1" --env "LOG_LEVEL=trace"
```

**Flags:**
- `--workspace <path>` - Custom worktree path (default: .agentd/worktrees/<id>)
- `--image <name>` - Docker image (default: ourocodus/agent:latest)
- `--env KEY=VALUE` - Environment variables (repeatable)
- `--api-key <key>` - Anthropic API key (or set ANTHROPIC_API_KEY env var)

**API Key Requirement:**
The spawn command requires an Anthropic API key for agent communication. Provide it via:
1. `--api-key` flag: `agentd spawn alice --api-key sk-...`
2. `ANTHROPIC_API_KEY` environment variable: `export ANTHROPIC_API_KEY=sk-...`

The API key is written to `.creds/.env` in the agent's workspace with 0600 permissions and mounted read-only at `/root/.creds/` in the container. This ensures credentials are never visible in `docker inspect` or container environment variables.

**What happens:**
1. Creates a new git worktree at `.agentd/worktrees/agent-<id>`
2. Creates a new branch `agent-<id>-<timestamp>`
3. Writes API key to `.creds/.env` with secure permissions (0700 directory, 0600 file)
4. Starts a Docker container with the worktree mounted at `/workspace`
5. Mounts `.creds` directory read-only at `/root/.creds` in the container

**Output:**
```text
✨ Creating isolated agent 'alice'...

🌳 Worktree: /path/to/.agentd/worktrees/agent-alice (branch: agent-alice-20251119-120000)
📦 Container: abc123def456 (running)
🔑 Credentials: /root/.creds (read-only)

✓ Agent alice ready
```

### 📋 list - View Active Agents

Shows all running agents with their status.

```bash
# Table format (default)
agentd list

# JSON format
agentd list --format json
```

**Output:**
```text
AGENT  STATUS   WORKSPACE                              CONTAINER     CREATED
alice  running  .../.agentd/worktrees/agent-alice     abc123def456  2m ago
bob    running  .../.agentd/worktrees/agent-bob       def456ghi789  5m ago
```

### 🛑 stop - Cleanup Agent Resources

Stops agents and removes all resources (container, worktree, branch).

```bash
# Stop single agent
agentd stop alice

# Stop multiple agents
agentd stop alice bob charlie
```

**What gets cleaned up:**
1. Docker container (30s graceful timeout)
2. Git worktree directory
3. Git branch
4. Credential files

**Idempotent:** Safe to call multiple times - succeeds even if agent is already stopped.

### 📜 logs - Stream Agent Logs

Stream logs from an agent's container in real-time.

```bash
# Follow logs (default)
agentd logs alice

# Show last 50 lines without following
agentd logs alice --tail 50 --follow=false
```

Press `Ctrl-C` to stop streaming.

### 💬 send - Send Command to Agent

Send a shell command to a running agent and get the response.

```bash
# Run a simple command
agentd send alice "ls -la /workspace"

# Check git status
agentd send bob "git status"

# Run a script
agentd send charlie "./scripts/build.sh"

# Multiple commands
agentd send alice "pwd && whoami"

# With custom timeout (default: 30s)
agentd send alice "sleep 10" --timeout 15

# With custom shell (default: /bin/bash)
agentd send alice "echo $SHELL" --shell /bin/zsh
```

**Use cases:**
- Quick checks without SSHing into containers
- Running scripts or one-off commands
- Triggering agent actions
- Debugging agent environment

### 📎 attach - Interactive Agent Shell

Attach to a running agent's container and get an interactive bash shell.

```bash
# Attach to agent alice
agentd attach alice
```

**Once attached:**
- Explore workspace: `ls /workspace`
- Check processes: `ps aux`
- View logs: `tail -f /var/log/*.log`
- Run git commands: `git status`
- Debug environment: `env | grep AGENT`

**Detach:** Press `Ctrl-D` or type `exit`

**Use cases:**
- Interactive debugging
- Exploring agent environment
- Running multiple commands interactively
- Troubleshooting issues

### 🔄 repl - Interactive REPL with Agent

Connect to a running agent and interact via the Agent Communication Protocol (ACP).

```bash
agentd repl <agent-id>
```

Opens an interactive REPL session with the agent. The agent must be running (spawned). This command attaches directly to the agent's stdin/stdout where the ACP process runs as PID 1.

**Usage:**

```bash
# Start REPL with agent
$ agentd repl alice
✓ Connected to agent 'alice'
  Press Ctrl+D to exit

> Hello agent!
Echo: Hello agent!

> help
Echo: help

> ^D
✓ Disconnected from agent 'alice'
```

**Controls:**
- `Ctrl+D` - Exit REPL cleanly
- `Ctrl+C` - Interrupt current operation and exit

**Notes:**
- Agent must be in "running" state (verify with `agentd list`)
- Uses direct docker attach to the ACP process (PID 1)
- All input is sent to the agent's stdin
- All output from the agent is displayed in real-time
- For non-interactive commands, use `agentd send` instead
- For full shell access, use `agentd attach` instead

**Requirements:**
- Running agent spawned with `agentd spawn`
- Agent container must be accessible via Docker API
- Terminal must support raw mode for proper I/O handling

**Alternative interaction methods:**
- Use `agentd send <agent-id> "<command>"` for one-off commands
- Use `agentd attach <agent-id>` for interactive shell access
- Use the relay server + PWA for WebSocket-based communication

### 🩺 doctor - Validate Environment

Runs comprehensive environment checks before spawning agents.

```bash
agentd doctor
```

**Validates:**
- ✓ Docker daemon connectivity and version (>= 20.10)
- ✓ File sharing permissions (macOS)
- ✓ Agent image availability (`ourocodus/agent:latest`)
- ✓ Git worktree support
- ✓ Disk space requirements (>= 1GB)
- ✓ Container spawn smoke test

**Output:**
```text
✓ Docker daemon running (v27.4.0)
✓ Docker version supported (>= 20.10)
✓ File sharing enabled: /path/to/repo
✓ Image present: ourocodus/agent:latest
✓ Git worktree support confirmed
✓ Disk space: 530.9GB available
✓ Spawn smoke test passed

✨ Environment ready! All systems go for spawning agents.
```

## Shell Completion

`agentd` supports tab completion for bash, zsh, fish, and PowerShell.

### Zsh (macOS/Linux)

**One-time setup:**

```bash
# macOS (with Homebrew)
agentd completion zsh > $(brew --prefix)/share/zsh/site-functions/_agentd

# Linux
agentd completion zsh > "${fpath[1]}/_agentd"

# Restart your shell
exec zsh
```

**For current session only:**

```bash
source <(agentd completion zsh)
```

### Bash

**One-time setup:**

```bash
# Linux
agentd completion bash > /etc/bash_completion.d/agentd

# macOS (with Homebrew)
agentd completion bash > $(brew --prefix)/etc/bash_completion.d/agentd

# Restart your shell
exec bash
```

**For current session only:**

```bash
source <(agentd completion bash)
```

### Fish

```bash
agentd completion fish > ~/.config/fish/completions/agentd.fish
```

### PowerShell

Add to your PowerShell profile:

```powershell
agentd completion powershell | Out-String | Invoke-Expression
```

### What You Get

With completion enabled, you can:

```bash
# Tab complete commands
agentd sp<TAB>  → agentd spawn

# Tab complete flags
agentd spawn --im<TAB>  → agentd spawn --image

# Tab complete subcommands
agentd completion z<TAB>  → agentd completion zsh
```

## Demo Scripts

### Automated Demo

Runs through all features with 2-second pauses between steps:

```bash
export DOCKER_HOST=unix:///Users/$USER/.colima/default/docker.sock
./scripts/demo-agentd.sh
```

### Interactive Demo

Step through at your own pace (press ENTER to advance):

```bash
export DOCKER_HOST=unix:///Users/$USER/.colima/default/docker.sock
./scripts/demo-agentd-interactive.sh
```

The interactive demo includes:
- Clear explanations of host vs container path mapping
- Peek inside containers to see `/workspace` mount
- Step-by-step walkthrough of the isolation architecture

## Architecture Details

### Path Mapping

The `list` command shows **host paths** (where files actually live):
```text
.agentd/worktrees/agent-alice  ← on your machine
```

This directory is mounted into the container at `/workspace`:
```text
/workspace  ← inside the container
```

Both paths refer to the same files, just viewed from different perspectives.

### Credentials

Credentials are stored in `.agentd/credentials/<agent-id>/` and mounted read-only into containers at `/root/.creds`.

**Currently supported credentials:**
- SSH keys (for git operations)
- GitHub tokens
- API keys

If no credential files exist, agents spawn without credentials (suitable for demos/testing).

### Resource Cleanup

The `stop` command performs complete cleanup:

1. **Container**: Stops gracefully (30s timeout) then removes
2. **Worktree**: Removes the isolated workspace directory
3. **Branch**: Deletes the agent-specific git branch
4. **Credentials**: Removes agent-specific credential directory

This ensures no resource leaks and clean slate for subsequent spawns.

## Troubleshooting

### Docker Connection Issues

**Error:** `Cannot connect to the Docker daemon`

**Solution:**
```bash
# Colima users
export DOCKER_HOST=unix:///Users/$USER/.colima/default/docker.sock

# Docker Desktop users (default)
unset DOCKER_HOST
```

### Permission Issues

**Error:** `permission denied` on worktree creation

**Solution:**
```bash
# Ensure git worktree directory is writable
mkdir -p .agentd/worktrees
chmod 755 .agentd/worktrees
```

### Image Not Found

**Error:** `image not found: ourocodus/agent:latest`

**Solution:**
```bash
# Build the image
make agent-image

# Or specify a different image
agentd spawn alice --image custom/image:tag
```

### Branch Already Exists

**Error:** `branch already exists`

**Cause:** Previous agent wasn't stopped properly

**Solution:**
```bash
# Clean up manually
git branch -D agent-<id>-<timestamp>
git worktree prune

# Or let agentd handle it
agentd stop <id>  # Will clean up even if container is gone
```

## Testing

Run the end-to-end test suite:

```bash
export DOCKER_HOST=unix:///Users/$USER/.colima/default/docker.sock
./scripts/test-agentd-mvp.sh
```

**Tests cover:**
- Environment validation (doctor)
- Agent spawning
- Multi-agent isolation
- List functionality
- Graceful cleanup
- Workspace verification

## Development

Run unit tests:

```bash
# All tests
go test ./cmd/agentd/...

# Specific test
go test -v ./cmd/agentd -run TestRemoveWorktree
```

Build:

```bash
make build
# Binary at bin/agentd
```

Format and lint:

```bash
make fmt
make lint
```

## See Also

- [Architecture Documentation](../README.md#architecture)
- [Demo Script Source](../scripts/demo-agentd.sh)
- [Interactive Demo Source](../scripts/demo-agentd-interactive.sh)
- [Test Suite Source](../scripts/test-agentd-mvp.sh)
