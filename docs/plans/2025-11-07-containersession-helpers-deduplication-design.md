# ContainerSession Helpers Deduplication Design

**Date:** 2025-11-07
**Status:** Approved
**Issues:** #160, #151, #150, #152

## Problem

Helper utilities for the containersession package are duplicated across multiple locations:

- **3 example files** each duplicate: `UUIDGenerator`, `SystemClock`, `StdLogger`, `createDockerClient()`
- **1 integration test** duplicates: `setupDockerClient()` (similar to example version)

This creates maintenance burden and risks inconsistency as the code evolves.

## Architecture Context

The containersession package uses dependency injection:

```go
// pkg/containersession/interfaces.go
type IDGenerator interface { Generate() string }
type Clock interface { Now() time.Time }
type Logger interface { Printf(format string, v ...interface{}) }

// Manager constructor requires implementations
func NewManager(dockerClient DockerClient, idGen IDGenerator,
                clock Clock, logger Logger, baseWorkspaceDir string) *Manager
```

The package intentionally provides **only interfaces**, not concrete implementations. Every caller brings their own implementations, causing the current duplication.

## Solution

Create a new **`pkg/containersession/helpers`** package with reference implementations.

### Package Structure

```
pkg/containersession/helpers/
  helpers.go         - Reference implementations
  helpers_test.go    - Unit tests
```

### Exported API

**Four utilities:**

1. **`UUIDGenerator`** - implements `containersession.IDGenerator` using google/uuid
2. **`SystemClock`** - implements `containersession.Clock` using `time.Now()`
3. **`StdLogger`** - wraps `log.Logger` to implement `containersession.Logger`
4. **`CreateDockerClient(ctx context.Context) (*client.Client, error)`** - Docker client setup with Colima/Docker Desktop detection

**Design decisions:**
- Keep pure dependency injection in main package (interfaces only)
- Provide reference implementations in separate subpackage
- Require explicit import of helpers package
- Accept `context.Context` for Docker client (enables timeout/cancellation)
- Document platform limitations (Unix-only, Windows needs named pipes)

## Migration Strategy

**Incremental migration in 5 commits:**

### Commit 1: Create helpers package
- Add `pkg/containersession/helpers/helpers.go`
- Implement all four utilities
- Add unit tests
- Document API and limitations

### Commit 2: Migrate basic example
- Update `examples/containersession/basic/main.go`
- Remove duplicated helpers
- Import from `pkg/containersession/helpers`
- Verify example runs

### Commit 3: Migrate multi example
- Update `examples/containersession/multi/main.go`
- Remove duplicated helpers
- Verify example runs

### Commit 4: Migrate echo-agent example
- Update `examples/containersession/echo-agent/main.go`
- Remove duplicated helpers
- **Bonus:** Remove unused `demonstrateRealIO()` function (issue #152)
- Verify example runs

### Commit 5: Migrate integration tests
- Update `pkg/containersession/integration_test.go`
- Wrap `CreateDockerClient()` with test-specific error handling
- Remove duplicate `setupDockerClient()`
- Verify tests pass

## Docker Client API

The Docker client setup differs slightly between examples and tests:

**Examples:** `createDockerClient() (*client.Client, error)`
**Tests:** `setupDockerClient(t *testing.T) *client.Client` (uses `t.Fatalf` on error)

**Unified design:**
- Helpers package exports: `CreateDockerClient(ctx context.Context) (*client.Client, error)`
- Test code wraps with error checking:
  ```go
  func setupDockerClient(t *testing.T) *client.Client {
      t.Helper()
      client, err := helpers.CreateDockerClient(context.Background())
      if err != nil {
          t.Fatalf("Failed to create Docker client: %v", err)
      }
      return client
  }
  ```

## Testing

**Unit tests for helpers package:**
- `UUIDGenerator.Generate()` produces valid UUIDs
- `SystemClock.Now()` returns current time
- `StdLogger.Printf()` logs correctly
- `CreateDockerClient()` behavior (may require Docker or skip)

**Integration verification:**
- All three examples run successfully
- Integration tests pass
- No behavioral changes

## Benefits

- **Reduces duplication** - 4 locations consolidated to 1
- **Easier maintenance** - single source of truth for utilities
- **Preserves DI pattern** - core package stays interface-only
- **Better discoverability** - helpers package clearly advertised
- **Consistent behavior** - all users get same implementations

## Trade-offs

- **Requires two imports** - users must import both `containersession` and `containersession/helpers`
- **Slightly more coupling** - examples depend on helpers package
- **Not backwards compatible** - examples change imports (but this is internal to repo)

## Future Considerations

- **Windows support** - CreateDockerClient could detect OS and use named pipes on Windows
- **More helpers** - could add test fakes/mocks in future (e.g., `FakeClock` for testing)
- **Production usage** - helpers are production-ready, not just for examples/tests
