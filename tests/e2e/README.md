# End-to-End Integration Tests

This directory contains end-to-end (E2E) integration tests that validate the complete system flow:

```
PWA → Relay → Claude Code Agents → Back to PWA
```

## Overview

The E2E tests verify that:
- The relay server starts and serves WebSocket connections
- Sessions can be created via WebSocket
- Agents can be spawned for different roles (auth, db, tests)
- Messages can be sent to agents and responses received
- Agents make commits to their git worktrees
- The entire flow works end-to-end with real Claude API calls

## Prerequisites

Before running the E2E tests, ensure you have:

1. **ANTHROPIC_API_KEY** - Set your Anthropic API key:
   ```bash
   export ANTHROPIC_API_KEY='your-api-key-here'
   ```

2. **Go 1.23+** - Install from https://go.dev/doc/install

3. **claude-code-acp** - The Claude Code ACP binary must be available in your PATH
   - Follow installation instructions from the claude-code-acp repository

4. **Git worktrees** - The test will attempt to set up worktrees automatically, but you can also run:
   ```bash
   ./scripts/setup-worktrees.sh
   ```

## Running the Tests

### Quick Start

From the project root:

```bash
make test-e2e
```

Or directly:

```bash
./scripts/run-e2e.sh
```

### With Verbose Output

```bash
./scripts/run-e2e.sh --verbose
```

### Running Specific Tests

```bash
go test -v ./tests/e2e/... -run TestE2EFullFlow
```

## Test Flow

The main test (`TestE2EFullFlow`) performs the following steps:

1. **Prerequisites Check**
   - Verifies ANTHROPIC_API_KEY is set
   - Checks for required tools

2. **Environment Setup**
   - Sets up git worktrees for agents
   - Builds the relay binary
   - Starts relay server on port 8080

3. **Session Creation**
   - Connects to relay WebSocket (`ws://localhost:8080/ws`)
   - Receives `connection:established` message
   - Sends `session:create` message
   - Receives `session:created` with session ID

4. **Agent Spawning**
   - Spawns 3 agents: auth, db, tests
   - Waits for `agent:ready` message for each agent
   - Each agent gets its own git worktree

5. **Agent Communication**
   - Sends test messages to each agent
   - Verifies agent responses are received
   - Example messages:
     - auth: "What is your role?"
     - db: "What database are you working with?"
     - tests: "What testing framework should we use?"

6. **Worktree Verification**
   - Checks that each agent's worktree has commits
   - Verifies commits were made during the test run

7. **Cleanup**
   - Stops the relay server
   - Closes WebSocket connections

## Test Configuration

Key constants in `e2e_test.go`:

```go
relayPort       = "8080"
wsURL           = "ws://localhost:8080/ws"
protocolVersion = "1.0"

// Timeouts
setupTimeout    = 60 * time.Second
messageTimeout  = 30 * time.Second
worktreeTimeout = 90 * time.Second
testTimeout     = 5 * time.Minute
```

## Troubleshooting

### Test Timeout

If tests timeout, it may be due to:
- Slow Claude API responses (increase `messageTimeout`)
- Relay server taking too long to start (increase `setupTimeout`)
- Network issues

### ANTHROPIC_API_KEY Not Set

```
Error: ANTHROPIC_API_KEY environment variable is not set
```

**Solution:** Export your API key:
```bash
export ANTHROPIC_API_KEY='your-api-key-here'
```

### claude-code-acp Not Found

```
Warning: claude-code-acp not found in PATH
```

**Solution:** Install claude-code-acp and ensure it's in your PATH:
```bash
which claude-code-acp
```

### Relay Server Failed to Start

```
Error: relay server failed to become healthy
```

**Solution:**
- Check if port 8080 is already in use: `lsof -i :8080`
- Kill any existing relay processes: `pkill relay`
- Try running the test again

### Worktree Setup Failed

```
Warning: Worktree setup failed (may already be set up)
```

**Solution:** This is usually not a problem if worktrees already exist. To reset:
```bash
rm -rf agent/
./scripts/setup-worktrees.sh
```

## CI Integration

### GitHub Actions

The E2E tests can be integrated into GitHub Actions:

```yaml
name: E2E Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Install claude-code-acp
        run: |
          # Add installation steps here

      - name: Run E2E Tests
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: make test-e2e
```

**Note:** E2E tests make real API calls to Anthropic, which:
- Incur costs
- Take longer to run
- May be rate-limited

Consider:
- Running E2E tests only on main branch or manually triggered
- Using shorter timeouts in CI
- Mocking Claude responses for CI (future enhancement)

## Helper Packages

### `helpers/websocket.go`

Provides WebSocket client utilities:
- `Connect(url)` - Establish WebSocket connection
- `Send(message)` - Send JSON message
- `Receive(v, timeout)` - Receive JSON message with timeout
- `WaitForMessageType(type, timeout)` - Wait for specific message type
- `Close()` - Close connection gracefully

### `helpers/process.go`

Manages relay server process:
- `StartRelay(ctx, binary, port)` - Start relay as background process
- `WaitForHealth(url, timeout)` - Wait for server health check
- `Stop()` - Gracefully stop relay server
- `BuildRelay(ctx, output)` - Compile relay binary

### `helpers/git.go`

Verifies git worktree commits:
- `CheckWorktreeExists(path)` - Check if worktree directory exists
- `GetWorktreeCommits(ctx, path, since)` - Count commits since time
- `GetLatestCommitMessage(ctx, path)` - Get latest commit message
- `WaitForWorktreeCommits(ctx, path, since, timeout)` - Poll for commits

## Future Enhancements

- [ ] Add mock mode for Claude responses (deterministic tests)
- [ ] Test error scenarios (invalid messages, agent failures)
- [ ] Test session cleanup and termination
- [ ] Test concurrent sessions
- [ ] Add performance benchmarks
- [ ] Add load testing capabilities
- [ ] Test PWA UI interactions with Playwright
- [ ] Add visual regression testing

## Related Documentation

- [Architecture](../../docs/architecture/ARCHITECTURE.md) - System architecture overview
- [Relay Package](../../pkg/relay/) - WebSocket protocol details
- [ACP Package](../../pkg/acp/) - Agent communication protocol
- [Session Management](../../pkg/relay/session/) - Session layer details
