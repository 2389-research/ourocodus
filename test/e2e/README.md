# Phase 1 Agent Adoption - End-to-End Tests

This directory contains integration tests for Phase 1 of the Agent Adoption feature.

## Test Scripts

### 1. `agent-adoption-basic-test.sh` (Recommended)

**Dependencies**: Docker, Go (no Node.js required)

Basic integration test that verifies:
- ✅ Agent spawning with spawn-source label
- ✅ Docker container creation with correct labels
- ✅ Workspace mount configuration
- ✅ Credentials mount configuration
- ✅ Agent stop command
- ✅ Agent list command

**Usage**:
```bash
./test/e2e/agent-adoption-basic-test.sh
```

**What it tests**:
- Builds `agentd` binary
- Spawns agent with auto-generated ID
- Verifies Docker labels:
  - `ourocodus.agent=true`
  - `agent-id=<agent-id>`
  - `ourocodus.agent/spawn-source=cli` ⭐ (NEW in Phase 1)
- Verifies workspace mount at `/workspace`
- Verifies credentials mount at `/root/.creds` (if present)
- Tests `agentd stop` command
- Tests `agentd list` command

### 2. `agent-adoption-test.sh` (Full Test Suite)

**Dependencies**: Docker, Go, Node.js, `ws` npm package

Comprehensive test suite that includes everything from the basic test plus:
- ✅ WebSocket relay server integration
- ✅ Agent discovery via WebSocket
- ✅ Attach conflict scenarios (simultaneous attach to different sessions)
- ✅ Idempotent attach (same session, multiple times)
- ✅ Idempotent detach (multiple detach calls)

**Setup**:
```bash
# Install Node.js dependencies
npm install -g ws

# Run test
./test/e2e/agent-adoption-test.sh
```

**What it tests**:
- All basic tests from `agent-adoption-basic-test.sh`
- Starts relay server on port 8080
- WebSocket `agent:discover` message
- WebSocket `agent:attach` message with conflict detection
- WebSocket `agent:detach` message
- Concurrent attach/detach operations
- Idempotence guarantees

## Test Scenarios

### Basic Workflow
1. **Spawn**: Create isolated agent with `agentd spawn`
2. **Verify**: Check Docker labels and mounts
3. **Stop**: Clean up with `agentd stop`

### Full Workflow (WebSocket Tests)
1. **Spawn**: Create isolated agent
2. **Discover**: Query relay for available agents
3. **Attach**: Attach agent to user session via WebSocket
4. **Detach**: Detach agent from session
5. **Verify**: Check idempotence and conflict scenarios

## Expected Test Output

### Basic Test (Success)
```
╔════════════════════════════════════════════════════╗
║  Phase 1 Agent Adoption - Basic Integration Test  ║
╚════════════════════════════════════════════════════╝

==> Building agentd binary...
✓ Binary built

==> Test 1: Spawn agent with spawn-source label
✓ Agent spawned successfully

==> Test 2: Verify container exists with correct labels
✓ Container found: abc123...
✓ Label ourocodus.agent=true verified
✓ Label agent-id=basic-e2e-test-agent-123 verified
✓ Label ourocodus.agent/spawn-source=cli verified ⭐

==> Test 3: Verify workspace mount
✓ Workspace mount verified: /path/to/worktree

==> Test 4: Verify credentials mount
✓ .creds mount is read-only: /path/to/.creds

==> Test 5: Stop agent and verify cleanup
✓ Agent stopped and cleaned up

==> Test 6: Test list command
✓ List command works correctly

════════════════════════════════════════════════════
✓ All basic tests passed!

Note: For full WebSocket attach/detach testing, run:
  ./test/e2e/agent-adoption-test.sh
```

### Full Test (Success)
```
╔════════════════════════════════════════════════════╗
║  Phase 1 Agent Adoption E2E Integration Test      ║
╚════════════════════════════════════════════════════╝

==> Building binaries...
✓ Binaries built

==> Starting relay server...
✓ Relay server started (PID: 12345)

==> Test 1: Spawn agent with spawn-source label
✓ Agent spawned successfully (Container: abc123..., spawn-source: cli)

==> Test 2: Discover agents via WebSocket
✓ Agent discovered successfully (status: detached, spawnSource: cli)

==> Test 3: Test attach conflict (simultaneous attach to different sessions)
✓ Attach conflict handled correctly (one succeeded, one failed)

==> Test 4: Test idempotent attach (same session, multiple times)
✓ Idempotent attach works correctly (all succeeded)

==> Test 5: Test idempotent detach
✓ Idempotent detach works correctly (all succeeded)

════════════════════════════════════════════════════
✓ All tests passed!
```

## Troubleshooting

### Docker Not Running
```
Error: Cannot connect to the Docker daemon at unix:///var/run/docker.sock
```
**Solution**: Start Docker Desktop or Docker daemon

### Node.js/ws Not Found
```
Error: Node.js not found (required for WebSocket testing)
```
**Solution**:
- Install Node.js: https://nodejs.org/
- Install ws package: `npm install -g ws`
- Or use basic test: `./test/e2e/agent-adoption-basic-test.sh`

### Port 8080 Already In Use
```
Error: Relay server failed to start
```
**Solution**: Stop process using port 8080 or edit script to use different port

### Permission Denied
```
Error: permission denied while trying to connect to Docker daemon
```
**Solution**: Add user to docker group or use `sudo`

## Continuous Integration

These tests are designed to run in CI environments:

```yaml
# Example GitHub Actions workflow
- name: Run E2E Tests
  run: |
    # Basic test (no Node.js required)
    ./test/e2e/agent-adoption-basic-test.sh
```

For full WebSocket testing in CI:
```yaml
- name: Install Node.js
  uses: actions/setup-node@v3
  with:
    node-version: '18'
- name: Install ws package
  run: npm install -g ws
- name: Run Full E2E Tests
  run: ./test/e2e/agent-adoption-test.sh
```

## What's NOT Tested Here

These tests cover Phase 1 functionality. The following are tested elsewhere or in later phases:

- **Phase 2**: NATS heartbeat mechanism (tested in Phase 2 integration tests)
- **Phase 3**: ACP communication via WebSocket (tested in Phase 3)
- **Phase 4**: Graceful detachment and cleanup (tested in Phase 4)

## Test Coverage Summary

| Feature | Basic Test | Full Test | Unit Tests |
|---------|-----------|-----------|------------|
| Agent spawning | ✅ | ✅ | ✅ |
| spawn-source label | ✅ | ✅ | ✅ |
| Docker labels | ✅ | ✅ | ✅ |
| Workspace mount | ✅ | ✅ | ✅ |
| Credentials mount | ✅ | ✅ | ✅ |
| Agent stop | ✅ | ✅ | ✅ |
| Agent list | ✅ | ✅ | ✅ |
| WebSocket discovery | ❌ | ✅ | ✅ |
| Agent attach | ❌ | ✅ | ✅ |
| Agent detach | ❌ | ✅ | ✅ |
| Attach conflicts | ❌ | ✅ | ✅ |
| Idempotent attach | ❌ | ✅ | ✅ |
| Idempotent detach | ❌ | ✅ | ✅ |
| Concurrent operations | ❌ | ✅ | ✅ |

## Contributing

When adding new Phase 1 features:
1. Add test case to appropriate script
2. Update this README with new test scenarios
3. Ensure tests pass before submitting PR
