# ContainerSession Helpers Deduplication Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate duplicate helper utilities from containersession examples and tests into a shared `pkg/containersession/helpers` package.

**Architecture:** Create new helpers package with reference implementations of containersession interfaces (IDGenerator, Clock, Logger) plus Docker client setup utility. Migrate 3 examples and integration test incrementally.

**Tech Stack:** Go 1.23, Docker SDK, google/uuid

**Related Issues:** #160, #151, #150, #152

---

## Task 1: Create helpers package

**Files:**
- Create: `pkg/containersession/helpers/helpers.go`
- Create: `pkg/containersession/helpers/helpers_test.go`
- Create: `pkg/containersession/helpers/doc.go`

### Step 1: Create package documentation file

Create: `pkg/containersession/helpers/doc.go`

```go
// Package helpers provides reference implementations of containersession interfaces
// and utility functions for setting up container sessions in examples and tests.
//
// The helpers package is separate from the core containersession package to maintain
// pure dependency injection - the core package defines only interfaces, while helpers
// provides convenient concrete implementations.
package helpers
```

### Step 2: Create helpers implementation file

Create: `pkg/containersession/helpers/helpers.go`

```go
package helpers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// UUIDGenerator generates unique IDs using Google's UUID library.
// Implements containersession.IDGenerator interface.
type UUIDGenerator struct{}

// Generate returns a new UUID v4 string.
func (g *UUIDGenerator) Generate() string {
	return uuid.New().String()
}

// SystemClock provides real time.
// Implements containersession.Clock interface.
type SystemClock struct{}

// Now returns the current time.
func (c *SystemClock) Now() time.Time {
	return time.Now()
}

// StdLogger wraps standard logger to implement containersession.Logger interface.
type StdLogger struct {
	*log.Logger
}

// Printf logs a formatted message using the underlying logger.
func (l *StdLogger) Printf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// CreateDockerClient attempts to connect to Docker, trying Colima first, then Docker Desktop.
//
// Platform limitations:
// - This function assumes a Unix-like environment (macOS/Linux)
// - On Windows, Docker Desktop uses named pipes (npipe:////./pipe/docker_engine)
// - Windows support requires detecting OS and using appropriate connection string
//
// The function tries two common Docker socket locations:
// 1. ~/.colima/default/docker.sock (Colima on macOS)
// 2. /var/run/docker.sock (Docker Desktop on macOS/Linux)
//
// Returns an error if neither location is accessible.
func CreateDockerClient(ctx context.Context) (*client.Client, error) {
	// Try Colima socket first
	colimaSocket := filepath.Join(os.Getenv("HOME"), ".colima", "default", "docker.sock")
	if _, err := os.Stat(colimaSocket); err == nil {
		if err := os.Setenv("DOCKER_HOST", "unix://"+colimaSocket); err == nil {
			dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err == nil {
				pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				if _, err := dockerClient.Ping(pingCtx); err == nil {
					return dockerClient, nil
				}
				dockerClient.Close()
			}
		}
	}

	// Try Docker Desktop
	if err := os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock"); err == nil {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if _, err := dockerClient.Ping(pingCtx); err == nil {
				return dockerClient, nil
			}
			dockerClient.Close()
		}
	}

	return nil, fmt.Errorf("cannot connect to Docker - tried Colima (%s) and Docker Desktop (/var/run/docker.sock)", colimaSocket)
}
```

### Step 3: Create helpers test file

Create: `pkg/containersession/helpers/helpers_test.go`

```go
package helpers

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func TestUUIDGenerator_Generate(t *testing.T) {
	gen := &UUIDGenerator{}

	// Generate two UUIDs
	id1 := gen.Generate()
	id2 := gen.Generate()

	// Check they're not empty
	if id1 == "" {
		t.Error("Generated UUID is empty")
	}
	if id2 == "" {
		t.Error("Generated UUID is empty")
	}

	// Check they're different
	if id1 == id2 {
		t.Errorf("Generated UUIDs are identical: %s", id1)
	}

	// Check format (UUID v4 has dashes in specific positions)
	if len(id1) != 36 {
		t.Errorf("UUID has wrong length: got %d, want 36", len(id1))
	}
	if !strings.Contains(id1, "-") {
		t.Error("UUID doesn't contain dashes")
	}
}

func TestSystemClock_Now(t *testing.T) {
	clock := &SystemClock{}

	before := time.Now()
	result := clock.Now()
	after := time.Now()

	// Check result is between before and after
	if result.Before(before) || result.After(after) {
		t.Errorf("Clock.Now() returned time outside expected range: %v", result)
	}
}

func TestStdLogger_Printf(t *testing.T) {
	// Create a logger that writes to a buffer we can inspect
	var buf strings.Builder
	logger := &StdLogger{
		Logger: log.New(&buf, "", 0),
	}

	// Log a message
	logger.Printf("test message: %s", "hello")

	// Check output contains our message
	output := buf.String()
	if !strings.Contains(output, "test message: hello") {
		t.Errorf("Logger output doesn't contain expected message, got: %s", output)
	}
}

func TestCreateDockerClient(t *testing.T) {
	ctx := context.Background()

	client, err := CreateDockerClient(ctx)
	if err != nil {
		// Skip test if Docker is not available
		t.Skipf("Docker not available: %v", err)
		return
	}
	defer client.Close()

	// Verify we can ping Docker
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err = client.Ping(pingCtx)
	if err != nil {
		t.Errorf("Failed to ping Docker: %v", err)
	}
}
```

### Step 4: Run tests to verify helpers work

```bash
go test ./pkg/containersession/helpers/... -v
```

Expected: All tests pass (Docker test may skip if Docker unavailable)

### Step 5: Run linting and formatting

```bash
make fmt
make lint
```

Expected: No issues

### Step 6: Commit helpers package

```bash
git add pkg/containersession/helpers/
git commit -m "feat(containersession): add helpers package with reference implementations

Create pkg/containersession/helpers with:
- UUIDGenerator: implements IDGenerator using google/uuid
- SystemClock: implements Clock using time.Now()
- StdLogger: wraps log.Logger for Logger interface
- CreateDockerClient: Docker setup with Colima/Docker Desktop detection

Includes comprehensive unit tests.

Related: #160, #151, #150"
```

---

## Task 2: Migrate basic example

**Files:**
- Modify: `examples/containersession/basic/main.go`

### Step 1: Update basic example imports

In `examples/containersession/basic/main.go`, replace lines 3-15 with:

```go
import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/containersession/helpers"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)
```

### Step 2: Remove duplicate helper types

Delete lines 17-76 from `examples/containersession/basic/main.go` (UUIDGenerator, SystemClock, StdLogger, createDockerClient)

### Step 3: Update Docker client creation

Replace the dockerClient creation (around line 86) with:

```go
dockerClient, err := helpers.CreateDockerClient(ctx)
```

### Step 4: Update Manager creation

Replace the Manager creation (around line 96) with:

```go
manager := containersession.NewManager(
	dockerClient,
	&helpers.UUIDGenerator{},
	&helpers.SystemClock{},
	&helpers.StdLogger{Logger: log.New(os.Stdout, "[manager] ", 0)},
	baseWorkspace,
)
```

### Step 5: Test the basic example

```bash
go run examples/containersession/basic/main.go
```

Expected: Example runs successfully and creates a container session

### Step 6: Commit basic example migration

```bash
git add examples/containersession/basic/main.go
git commit -m "refactor(examples): migrate basic example to helpers package

Remove duplicate UUIDGenerator, SystemClock, StdLogger, and createDockerClient.
Import from pkg/containersession/helpers instead.

Related: #160, #151, #150"
```

---

## Task 3: Migrate multi example

**Files:**
- Modify: `examples/containersession/multi/main.go`

### Step 1: Update multi example imports

In `examples/containersession/multi/main.go`, replace the import section with:

```go
import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/containersession/helpers"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)
```

### Step 2: Remove duplicate helper types

Delete the UUIDGenerator, SystemClock, StdLogger, and createDockerClient definitions (should be around lines 17-76, similar to basic example).

### Step 3: Update Docker client creation

Find and replace the dockerClient creation with:

```go
dockerClient, err := helpers.CreateDockerClient(ctx)
```

### Step 4: Update Manager creation

Find and replace the Manager creation with:

```go
manager := containersession.NewManager(
	dockerClient,
	&helpers.UUIDGenerator{},
	&helpers.SystemClock{},
	&helpers.StdLogger{Logger: log.New(os.Stdout, "[manager] ", 0)},
	baseWorkspace,
)
```

### Step 5: Test the multi example

```bash
go run examples/containersession/multi/main.go
```

Expected: Example runs successfully and creates multiple container sessions

### Step 6: Commit multi example migration

```bash
git add examples/containersession/multi/main.go
git commit -m "refactor(examples): migrate multi example to helpers package

Remove duplicate helper implementations.
Import from pkg/containersession/helpers instead.

Related: #160, #151, #150"
```

---

## Task 4: Migrate echo-agent example and cleanup

**Files:**
- Modify: `examples/containersession/echo-agent/main.go`

### Step 1: Update echo-agent example imports

In `examples/containersession/echo-agent/main.go`, replace the import section with:

```go
import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/containersession/helpers"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)
```

### Step 2: Remove duplicate helper types

Delete the UUIDGenerator, SystemClock, StdLogger, and createDockerClient definitions (around lines 17-76).

### Step 3: Remove unused demonstrateRealIO function

Delete the entire `demonstrateRealIO` function (around lines 204-237) and remove the unused imports `bufio` and `io` if they're not used elsewhere.

### Step 4: Update Docker client creation

Find and replace the dockerClient creation with:

```go
dockerClient, err := helpers.CreateDockerClient(ctx)
```

### Step 5: Update Manager creation

Find and replace the Manager creation with:

```go
manager := containersession.NewManager(
	dockerClient,
	&helpers.UUIDGenerator{},
	&helpers.SystemClock{},
	&helpers.StdLogger{Logger: log.New(os.Stdout, "[manager] ", 0)},
	baseWorkspace,
)
```

### Step 6: Test the echo-agent example

```bash
go run examples/containersession/echo-agent/main.go
```

Expected: Example runs successfully

### Step 7: Commit echo-agent example migration

```bash
git add examples/containersession/echo-agent/main.go
git commit -m "refactor(examples): migrate echo-agent to helpers and remove dead code

- Remove duplicate helper implementations
- Import from pkg/containersession/helpers
- Remove unused demonstrateRealIO function (never called, only placeholder code)

Related: #160, #151, #150, #152"
```

---

## Task 5: Migrate integration tests

**Files:**
- Modify: `pkg/containersession/integration_test.go`

### Step 1: Add helpers import

In `pkg/containersession/integration_test.go`, add to the import section:

```go
"github.com/2389-research/ourocodus/pkg/containersession/helpers"
```

### Step 2: Replace setupDockerClient function

Replace the entire `setupDockerClient` function (lines 52-89) with:

```go
// setupDockerClient creates a Docker client for testing
func setupDockerClient(t *testing.T) *client.Client {
	t.Helper()

	dockerClient, err := helpers.CreateDockerClient(context.Background())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	return dockerClient
}
```

### Step 3: Run integration tests

```bash
go test ./pkg/containersession/... -v
```

Expected: All tests pass

### Step 4: Run full test suite

```bash
make test
```

Expected: All tests pass

### Step 5: Run linting and formatting

```bash
make fmt
make lint
```

Expected: No issues

### Step 6: Commit integration test migration

```bash
git add pkg/containersession/integration_test.go
git commit -m "refactor(containersession): migrate integration tests to helpers

Replace duplicate setupDockerClient implementation with thin wrapper
around helpers.CreateDockerClient for test-specific error handling.

Completes deduplication of containersession helper utilities.

Related: #160, #151, #150"
```

---

## Task 6: Final verification and documentation

**Files:**
- None (verification only)

### Step 1: Verify all examples run

```bash
echo "Testing basic example..."
go run examples/containersession/basic/main.go

echo "Testing multi example..."
go run examples/containersession/multi/main.go

echo "Testing echo-agent example..."
go run examples/containersession/echo-agent/main.go
```

Expected: All three examples run successfully

### Step 2: Verify all tests pass

```bash
make test
```

Expected: All tests pass

### Step 3: Verify code quality

```bash
make pre-commit
```

Expected: All quality checks pass (format, lint, vet, build)

### Step 4: Review changes summary

```bash
git log --oneline main..HEAD
```

Expected: Should show 6 commits:
1. Create helpers package
2. Migrate basic example
3. Migrate multi example
4. Migrate echo-agent example
5. Migrate integration tests
6. (if any) Documentation updates

---

## Success Criteria

- [ ] `pkg/containersession/helpers` package created with all utilities
- [ ] All three examples migrated and working
- [ ] Integration tests migrated and passing
- [ ] Unused `demonstrateRealIO` function removed
- [ ] All tests pass
- [ ] All linting/formatting checks pass
- [ ] No duplicate helper code remains

## Notes

- The helpers package maintains the same behavior as the original duplicated code
- Docker client function now accepts context for better timeout control
- Platform limitations (Unix-only) are documented
- Test code uses thin wrapper for test-specific error handling
- Changes are backward compatible (internal refactor only)
