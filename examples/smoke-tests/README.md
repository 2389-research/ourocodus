# Smoke Tests - Manual Verification Examples

## Purpose

This directory contains smoke test examples for manually verifying core functionality of the Ourocodus system. These examples demonstrate:

- **Component testing**: Testing individual system components in isolation
- **Integration testing**: Verifying components work together correctly
- **Regression prevention**: Catching breaking changes early
- **Manual verification**: Step-by-step validation of key features

Smoke tests are quick, focused tests that verify the system "doesn't catch fire" after changes.

## Educational Value

Learn how to:
- Write focused component tests
- Test relay server functionality
- Validate session management
- Check message routing
- Verify agent communication

## Directory Structure

```
smoke-tests/
├── relay/          # Relay server smoke tests
│   └── main.go     # Tests relay startup, connections, basic routing
└── session/        # Session management smoke tests
    └── main.go     # Tests session lifecycle and state management
```

## Prerequisites

1. **Built binaries**: Run `make build` from repository root
2. **Go installed**: Required to run smoke tests
3. **Clean environment**: No other relay instances running

## Available Smoke Tests

### Relay Smoke Test

**Location**: `relay/main.go`

**What it tests**:
- Relay server starts successfully
- WebSocket server listens on correct port
- Handshake protocol works
- Basic message routing functions
- Server handles invalid messages gracefully
- Server shuts down cleanly

**Usage**:
```bash
cd examples/smoke-tests/relay
go run main.go
```

**Expected output**:
```
🧪 Relay Server Smoke Test
==========================

✅ Test 1: Server starts successfully
✅ Test 2: WebSocket connection established
✅ Test 3: Handshake received and valid
✅ Test 4: Basic message routing works
✅ Test 5: Invalid message handled gracefully
✅ Test 6: Server shuts down cleanly

All tests passed! ✨
```

### Session Smoke Test

**Location**: `session/main.go`

**What it tests**:
- Session creation works
- Session IDs are unique
- Session state is tracked correctly
- Session termination works
- Sessions are cleaned up properly
- Multiple sessions can coexist

**Usage**:
```bash
cd examples/smoke-tests/session
go run main.go
```

**Expected output**:
```
🧪 Session Management Smoke Test
=================================

✅ Test 1: Create session
✅ Test 2: Session ID is unique
✅ Test 3: Session state tracked correctly
✅ Test 4: Multiple sessions coexist
✅ Test 5: Terminate session works
✅ Test 6: Session cleanup successful

All tests passed! ✨
```

## When to Run Smoke Tests

### During Development

Run after making changes to:
- Core relay functionality
- Session management code
- Message routing logic
- Protocol changes

```bash
# Quick check before committing
cd examples/smoke-tests/relay && go run main.go
cd examples/smoke-tests/session && go run main.go
```

### Before Releases

Verify everything still works:

```bash
# Run all smoke tests
for dir in examples/smoke-tests/*/; do
    cd "$dir"
    echo "Running smoke test in $dir"
    go run main.go || exit 1
    cd -
done
```

### After Deployment

Validate deployment in new environment:

```bash
# Point tests at deployed instance
RELAY_URL=https://production.example.com examples/smoke-tests/relay/main.go
```

### Debugging Issues

Isolate problems to specific components:

```bash
# If relay is suspected
cd examples/smoke-tests/relay && go run main.go

# If session management is suspected
cd examples/smoke-tests/session && go run main.go
```

## Understanding the Tests

### Test Structure

Each smoke test follows this pattern:

```go
func main() {
    fmt.Println("🧪 Component Smoke Test")

    // Test 1: Basic functionality
    if err := testBasicFunctionality(); err != nil {
        fail("Test 1 failed: %v", err)
    }
    pass("Test 1: Basic functionality works")

    // Test 2: Edge case
    if err := testEdgeCase(); err != nil {
        fail("Test 2 failed: %v", err)
    }
    pass("Test 2: Edge case handled")

    // ... more tests

    fmt.Println("All tests passed! ✨")
}
```

### Writing New Smoke Tests

To add a new smoke test:

1. **Create directory**: `mkdir examples/smoke-tests/component-name/`
2. **Write test**: Create `main.go` with focused tests
3. **Document**: Add section to this README
4. **Keep it fast**: Each test should complete in < 10 seconds

Example structure:

```go
package main

import (
    "fmt"
    "log"
)

func main() {
    fmt.Println("🧪 Component X Smoke Test")

    // Setup
    if err := setup(); err != nil {
        log.Fatalf("Setup failed: %v", err)
    }
    defer cleanup()

    // Test core functionality
    runTests()

    fmt.Println("All tests passed! ✨")
}
```

## Differences from Unit Tests

| Aspect | Smoke Tests | Unit Tests |
|--------|-------------|-----------|
| **Scope** | Component-level, end-to-end | Function-level, isolated |
| **Speed** | Seconds | Milliseconds |
| **Setup** | Real services, actual I/O | Mocks, in-memory |
| **Goal** | Verify "it works" | Verify correctness |
| **When** | Before commits, deploys | Every code change |

## Smoke Tests vs Integration Tests

**Smoke tests** (these examples):
- Quick verification of core paths
- Manual execution
- Catch major breakage
- Run in < 1 minute total

**Integration tests** (see test suite):
- Comprehensive scenario coverage
- Automated in CI/CD
- Catch subtle issues
- May run for several minutes

## Troubleshooting

### Test fails with "connection refused"

**Cause**: No relay server running or wrong port

**Solution**:
```bash
# Make sure relay binary is built
make build

# Test starts its own relay instance
# Check if port 8080 is available
lsof -i :8080
```

### Test hangs indefinitely

**Cause**: Deadlock or server not responding

**Solution**:
- Check for port conflicts
- Look for relay startup errors
- Increase timeout values
- Check system resources

### Inconsistent test results

**Cause**: Timing issues or leftover state

**Solution**:
- Run tests in isolation
- Add cleanup between tests
- Increase sleep/timeout values
- Check for race conditions

### Test passes locally but fails in CI

**Cause**: Environment differences

**Solution**:
- Check CI environment configuration
- Verify dependencies are available
- Look for timing assumptions
- Add more robust waits

## Best Practices

1. **Keep tests focused**: One component per test file
2. **Test happy path first**: Verify basic functionality works
3. **Include error cases**: Test graceful error handling
4. **Clean up resources**: Use defer for cleanup
5. **Make tests idempotent**: Can run multiple times safely
6. **Fast execution**: Complete in under 10 seconds
7. **Clear output**: Show what's being tested
8. **Fail fast**: Exit on first failure for quick feedback

## Extending Smoke Tests

Add tests for:

- **Agent lifecycle**: Spawning, communication, termination
- **Message validation**: Protocol compliance
- **Error recovery**: System resilience
- **Authentication**: Security features
- **Metrics**: Monitoring endpoints
- **Health checks**: Service status

## Integration with CI/CD

Example GitHub Actions workflow:

```yaml
name: Smoke Tests

on: [push, pull_request]

jobs:
  smoke-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Build binaries
        run: make build

      - name: Run relay smoke test
        run: cd examples/smoke-tests/relay && go run main.go

      - name: Run session smoke test
        run: cd examples/smoke-tests/session && go run main.go
```

## Related Documentation

- [Basic Demo](../basic-demo/README.md) - Start with basic functionality
- [Interactive REPL](../interactive-repl/README.md) - Manual testing interface
- [Testing Guide](../../docs/development/TESTING.md) - Comprehensive testing documentation

## Next Steps

After running smoke tests:

1. **If tests pass**: Proceed with confidence
2. **If tests fail**: Investigate and fix before proceeding
3. **Add more tests**: Cover components you're working on
4. **Automate**: Add to CI/CD pipeline

## Notes

- Smoke tests are not a replacement for comprehensive testing
- They provide quick confidence that major functionality works
- Run them frequently during development
- Keep them simple and fast
- Add new tests when new components are added
