# Function Decomposition Sprint - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Decompose 10 largest functions in `pkg/` (1,073 lines) into focused, testable helpers (~302 lines main + reusable helpers), eliminating ~1,000 lines through extraction and reuse.

**Architecture:** Five-phase approach extracting reusable helpers first (validation, I/O, lifecycle management), then decomposing large functions to use these helpers. Eliminates ~230 lines of exact duplication (container I/O, ACP client close, launcher cleanup) plus ~771 lines through decomposition.

**Tech Stack:** Go 1.24, Docker API, NATS, existing test infrastructure (testify, mock interfaces)

**Key Principles:**
- TDD: Write failing test → implement → verify → commit
- DRY: Extract reusable helpers before decomposing functions
- YAGNI: Only extract what's actually duplicated
- Frequent commits: After each passing test

**Estimated Time:** 2-3 days (10-15 tasks × ~30min each)

---

## Phase 1: Foundation Helpers (No Dependencies)

### Task 1: Input Validation Helpers

**Files:**
- Create: `pkg/validation/basic.go`
- Create: `pkg/validation/basic_test.go`

**Step 1: Write failing test for ValidateNonEmpty**

Create `pkg/validation/basic_test.go`:

```go
package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateNonEmpty(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid non-empty string",
			fieldName: "username",
			value:     "alice",
			wantErr:   false,
		},
		{
			name:      "empty string",
			fieldName: "username",
			value:     "",
			wantErr:   true,
			errMsg:    "username cannot be empty",
		},
		{
			name:      "whitespace only",
			fieldName: "password",
			value:     "   ",
			wantErr:   true,
			errMsg:    "password cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonEmpty(tt.fieldName, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAgentID(t *testing.T) {
	tests := []struct {
		name    string
		agentID string
		wantErr bool
	}{
		{
			name:    "valid agent ID",
			agentID: "agent-123",
			wantErr: false,
		},
		{
			name:    "empty agent ID",
			agentID: "",
			wantErr: true,
		},
		{
			name:    "whitespace agent ID",
			agentID: "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgentID(tt.agentID)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "agent ID")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/validation -v
```

Expected: FAIL with "undefined: ValidateNonEmpty"

**Step 3: Write minimal implementation**

Create `pkg/validation/basic.go`:

```go
// Package validation provides common input validation helpers.
package validation

import (
	"fmt"
	"strings"
)

// ValidateNonEmpty validates that a field is not empty or whitespace-only.
// Returns an error with a user-friendly message if validation fails.
func ValidateNonEmpty(fieldName, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	return nil
}

// ValidateAgentID validates that an agent ID is not empty.
// This is a specialized version of ValidateNonEmpty for agent IDs.
func ValidateAgentID(agentID string) error {
	return ValidateNonEmpty("agent ID", agentID)
}

// ValidateImageName validates that a container image name is not empty.
func ValidateImageName(imageName string) error {
	return ValidateNonEmpty("image name", imageName)
}

// ValidateCommand validates that a command slice is not empty.
func ValidateCommand(cmd []string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("command cannot be empty")
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/validation -v
```

Expected: PASS (all tests pass)

**Step 5: Commit**

```bash
git add pkg/validation/
git commit -m "feat(validation): add reusable input validation helpers

Extract common validation logic used across SpawnAgent, Spawn,
and CreateContainerSessionWithConfig. Provides consistent error
messages and eliminates duplication.

- ValidateNonEmpty: generic field validation
- ValidateAgentID: agent ID validation
- ValidateImageName: container image validation
- ValidateCommand: command slice validation

Part of function decomposition sprint (Task 1/15)."
```

---

### Task 2: Retry Logic with Backoff

**Files:**
- Create: `pkg/nats/retry.go`
- Create: `pkg/nats/retry_test.go`
- Modify: `pkg/nats/client.go:174-231` (Publish method)

**Step 1: Write failing test for RetryWithBackoff**

Create `pkg/nats/retry_test.go`:

```go
package nats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryWithBackoff(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0

		err := retryWithBackoff(
			ctx,
			3,
			func(attempt int) time.Duration { return time.Millisecond },
			func(err error) bool { return true },
			func() error {
				attempts++
				return nil
			},
		)

		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("retries on transient errors", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0
		transientErr := errors.New("transient error")

		err := retryWithBackoff(
			ctx,
			3,
			func(attempt int) time.Duration { return time.Millisecond },
			func(err error) bool { return err == transientErr },
			func() error {
				attempts++
				if attempts < 3 {
					return transientErr
				}
				return nil
			},
		)

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("stops on permanent error", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0
		permanentErr := errors.New("permanent error")

		err := retryWithBackoff(
			ctx,
			3,
			func(attempt int) time.Duration { return time.Millisecond },
			func(err error) bool { return false },
			func() error {
				attempts++
				return permanentErr
			},
		)

		assert.Equal(t, permanentErr, err)
		assert.Equal(t, 1, attempts, "should not retry permanent errors")
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		attempts := 0
		err := retryWithBackoff(
			ctx,
			3,
			func(attempt int) time.Duration { return time.Millisecond },
			func(err error) bool { return true },
			func() error {
				attempts++
				return errors.New("some error")
			},
		)

		assert.Equal(t, context.Canceled, err)
		assert.LessOrEqual(t, attempts, 1, "should stop quickly on cancellation")
	})

	t.Run("exhausts retries", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0
		transientErr := errors.New("transient error")

		err := retryWithBackoff(
			ctx,
			2, // Max 2 attempts (initial + 1 retry)
			func(attempt int) time.Duration { return time.Millisecond },
			func(err error) bool { return true },
			func() error {
				attempts++
				return transientErr
			},
		)

		assert.Equal(t, transientErr, err, "should return last error")
		assert.Equal(t, 3, attempts, "initial + 2 retries")
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/nats -run TestRetryWithBackoff -v
```

Expected: FAIL with "undefined: retryWithBackoff"

**Step 3: Write minimal implementation**

Create `pkg/nats/retry.go`:

```go
package nats

import (
	"context"
	"time"
)

// retryWithBackoff executes an operation with exponential backoff retry logic.
//
// Parameters:
//   - ctx: Context for cancellation
//   - maxAttempts: Maximum retry attempts (0 = initial attempt only, 1 = one retry, etc.)
//   - backoffFunc: Function returning wait duration for each attempt (attempt starts at 1 for first retry)
//   - isRetryable: Function determining if an error should trigger retry
//   - operation: The operation to execute
//
// Returns the operation's error, or context.Canceled if context is cancelled.
func retryWithBackoff(
	ctx context.Context,
	maxAttempts int,
	backoffFunc func(attempt int) time.Duration,
	isRetryable func(error) bool,
	operation func() error,
) error {
	var lastErr error

	// Initial attempt + retries
	for attempt := 0; attempt <= maxAttempts; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			return err
		}

		// Wait before retry (skip on first attempt)
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffFunc(attempt)):
			}
		}

		// Execute operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryable(err) {
			return err // Permanent error, don't retry
		}

		// Check context before retrying
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	// Exhausted all retries
	return lastErr
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/nats -run TestRetryWithBackoff -v
```

Expected: PASS (all tests pass)

**Step 5: Refactor Publish to use retryWithBackoff**

Modify `pkg/nats/client.go` - replace the retry loop in `Publish` (lines 198-227):

Before:
```go
// Publish with retry
var lastErr error
for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
	if attempt > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.config.RetryBackoff.Next(attempt)):
		}
	}

	err := c.conn.PublishMsg(msg)
	if err == nil {
		c.metrics.recordPublish(subject, time.Since(start), nil)
		return nil
	}

	lastErr = err

	// Check if error is retryable
	if !isTransientError(err) {
		c.metrics.recordPublish(subject, time.Since(start), err)
		return WrapPermanentError("publish", subject, err)
	}

	// Check context before retrying
	if ctx.Err() != nil {
		return ctx.Err()
	}
}

c.metrics.recordPublish(subject, time.Since(start), lastErr)
return WrapTransientError("publish", subject, lastErr)
```

After:
```go
// Publish with retry
err := retryWithBackoff(
	ctx,
	c.config.RetryAttempts,
	c.config.RetryBackoff.Next,
	isTransientError,
	func() error {
		return c.conn.PublishMsg(msg)
	},
)

if err == nil {
	c.metrics.recordPublish(subject, time.Since(start), nil)
	return nil
}

// Determine error type for wrapping
if isTransientError(err) {
	c.metrics.recordPublish(subject, time.Since(start), err)
	return WrapTransientError("publish", subject, err)
}

c.metrics.recordPublish(subject, time.Since(start), err)
return WrapPermanentError("publish", subject, err)
```

**Step 6: Run NATS client tests to verify refactoring**

```bash
go test ./pkg/nats -v
```

Expected: PASS (all existing tests still pass)

**Step 7: Commit**

```bash
git add pkg/nats/retry.go pkg/nats/retry_test.go pkg/nats/client.go
git commit -m "refactor(nats): extract retry logic with backoff

Extract retryWithBackoff helper from Publish method. This helper
will be reused by Request and Subscribe methods.

Changes:
- Add pkg/nats/retry.go with retryWithBackoff function
- Add comprehensive tests for retry logic
- Refactor Publish to use retryWithBackoff (reduces from 58→35 lines)

Benefits:
- Reusable across Publish, Request, Subscribe
- Eliminates ~75 lines of duplication
- Isolated, testable retry logic

Part of function decomposition sprint (Task 2/15)."
```

---

### Task 3: Protocol Error Mapping

**Files:**
- Create: `pkg/relay/protocol_errors.go`
- Create: `pkg/relay/protocol_errors_test.go`

**Step 1: Write failing test for MapToProtocolError**

Create `pkg/relay/protocol_errors_test.go`:

```go
package relay

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/2389-research/ourocodus/pkg/relay/session"
)

func TestMapToProtocolError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    string
		wantMsg     string
		recoverable bool
	}{
		{
			name:        "session not found",
			err:         fmt.Errorf("%w: session-123", session.ErrSessionNotFound),
			wantCode:    "SESSION_NOT_FOUND",
			wantMsg:     "Session not found",
			recoverable: false,
		},
		{
			name:        "agent not found",
			err:         fmt.Errorf("%w: agent-456", session.ErrAgentNotFound),
			wantCode:    "AGENT_NOT_FOUND",
			wantMsg:     "Agent not found",
			recoverable: false,
		},
		{
			name:        "empty agent ID",
			err:         session.ErrEmptyAgentID,
			wantCode:    "INVALID_REQUEST",
			wantMsg:     "Invalid request",
			recoverable: true,
		},
		{
			name:        "empty workspace",
			err:         session.ErrEmptyWorkspace,
			wantCode:    "INVALID_REQUEST",
			wantMsg:     "Invalid request",
			recoverable: true,
		},
		{
			name:        "generic error",
			err:         errors.New("something went wrong"),
			wantCode:    "INTERNAL_ERROR",
			wantMsg:     "Internal server error",
			recoverable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, msg, recoverable := MapToProtocolError(tt.err)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantMsg, msg)
			assert.Equal(t, tt.recoverable, recoverable)
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/relay -run TestMapToProtocolError -v
```

Expected: FAIL with "undefined: MapToProtocolError"

**Step 3: Write minimal implementation**

Create `pkg/relay/protocol_errors.go`:

```go
package relay

import (
	"errors"

	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// MapToProtocolError maps internal errors to WebSocket protocol error codes.
//
// Returns:
//   - code: Protocol error code (e.g., "SESSION_NOT_FOUND")
//   - message: User-friendly error message
//   - recoverable: Whether the client can retry or take corrective action
func MapToProtocolError(err error) (code string, message string, recoverable bool) {
	switch {
	case errors.Is(err, session.ErrSessionNotFound):
		return "SESSION_NOT_FOUND", "Session not found", false

	case errors.Is(err, session.ErrAgentNotFound):
		return "AGENT_NOT_FOUND", "Agent not found", false

	case errors.Is(err, session.ErrEmptyAgentID),
		errors.Is(err, session.ErrEmptyWorkspace):
		return "INVALID_REQUEST", "Invalid request", true

	default:
		return "INTERNAL_ERROR", "Internal server error", false
	}
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/relay -run TestMapToProtocolError -v
```

Expected: PASS (all tests pass)

**Step 5: Commit**

```bash
git add pkg/relay/protocol_errors.go pkg/relay/protocol_errors_test.go
git commit -m "feat(relay): add protocol error mapping helper

Extract error mapping logic from handleAgentMessage. Maps internal
errors (session.ErrSessionNotFound, etc.) to WebSocket protocol
error codes with user-friendly messages.

Will be reused by:
- handleAgentMessage
- handleSessionCreate
- handleAgentSpawn
- handleSessionEnd

Part of function decomposition sprint (Task 3/15)."
```

---

## Phase 2: Container Operations (Uses Phase 1)

### Task 4: Container I/O Attachment Helper (EXACT DUPLICATE!)

**Files:**
- Create: `pkg/containersession/io.go`
- Create: `pkg/containersession/io_test.go`
- Modify: `pkg/containersession/manager.go:599-611` (AttachContainerSession)
- Modify: `pkg/containersession/manager.go:644-657` (StartContainerSession)

**Step 1: Write failing test for attachContainerIO**

Create `pkg/containersession/io_test.go`:

```go
package containersession

import (
	"context"
	"io"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: This test requires Docker daemon. For unit tests, we'd need to mock the Docker client.
// For now, we'll create a simple integration test that can be skipped in CI if Docker is unavailable.

func TestAttachContainerIO_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Create real Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer dockerClient.Close()

	// Create a test container (alpine with sleep)
	resp, err := dockerClient.ContainerCreate(ctx,
		&container.Config{
			Image: "alpine:latest",
			Cmd:   []string{"sh", "-c", "echo 'test output' && sleep 1"},
		},
		nil, nil, nil, "",
	)
	require.NoError(t, err)
	containerID := resp.ID
	defer dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	// Start container
	err = dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
	require.NoError(t, err)

	// Test attachContainerIO
	reader, err := attachContainerIO(ctx, dockerClient, containerID)
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	// Read some output
	if reader != nil {
		buf := make([]byte, 100)
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			t.Logf("Read error (expected in some cases): %v", err)
		}
		if n > 0 {
			t.Logf("Read %d bytes from container", n)
		}
	}
}

func TestAttachContainerIO_InvalidContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Create real Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer dockerClient.Close()

	// Test with non-existent container
	reader, err := attachContainerIO(ctx, dockerClient, "nonexistent-container-id")
	assert.Error(t, err)
	assert.Nil(t, reader)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/containersession -run TestAttachContainerIO -v
```

Expected: FAIL with "undefined: attachContainerIO"

**Step 3: Write minimal implementation**

Create `pkg/containersession/io.go`:

```go
package containersession

import (
	"context"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// attachContainerIO attaches to a container's stdout and stderr streams for logging.
//
// This is the standard attachment configuration used across the codebase:
// - Stream: true (receive output)
// - Stdin: false (don't send input)
// - Stdout: true (receive stdout)
// - Stderr: true (receive stderr)
// - Logs: true (include logs since container start)
//
// Returns an io.Reader for the multiplexed stdout/stderr streams, or an error.
func attachContainerIO(
	ctx context.Context,
	dockerClient *client.Client,
	containerID string,
) (io.Reader, error) {
	attachResp, err := dockerClient.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		return nil, err
	}

	return attachResp.Reader, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/containersession -run TestAttachContainerIO -v
```

Expected: PASS (or SKIP if Docker not available)

**Step 5: Refactor AttachContainerSession to use helper**

Modify `pkg/containersession/manager.go` in `AttachContainerSession` method (around line 599):

Before:
```go
// Attach to container I/O
attachResp, err := m.dockerClient.ContainerAttach(ctx, containerID, container.AttachOptions{
	Stream: true,
	Stdin:  false,
	Stdout: true,
	Stderr: true,
	Logs:   true,
})
if err != nil {
	m.logger.Printf("Container attach failed: session=%s container=%s error=%v", sessionID, containerID, err)
	// Return session anyway - container is still accessible even if I/O attach failed
} else {
	m.startOutputHandler(session, sessionID, containerID, attachResp.Reader)
}
```

After:
```go
// Attach to container I/O
reader, err := attachContainerIO(ctx, m.dockerClient, containerID)
if err != nil {
	m.logger.Printf("Container attach failed: session=%s container=%s error=%v", sessionID, containerID, err)
	// Return session anyway - container is still accessible even if I/O attach failed
} else {
	m.startOutputHandler(session, sessionID, containerID, reader)
}
```

**Step 6: Refactor StartContainerSession to use helper**

Modify `pkg/containersession/manager.go` in `StartContainerSession` method (around line 644):

Before:
```go
// Attach to container I/O for logging (unless external attachment is used)
if !session.skipOutputLogging {
	attachResp, err := m.dockerClient.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		m.logger.Printf("Container attach failed: session=%s container=%s error=%v", sessionID, containerID, err)
		// Continue even if attach fails - container is still running
	} else {
		// Start goroutines to demux stdout/stderr
		m.startOutputHandler(session, sessionID, containerID, attachResp.Reader)
	}
}
```

After:
```go
// Attach to container I/O for logging (unless external attachment is used)
if !session.skipOutputLogging {
	reader, err := attachContainerIO(ctx, m.dockerClient, containerID)
	if err != nil {
		m.logger.Printf("Container attach failed: session=%s container=%s error=%v", sessionID, containerID, err)
		// Continue even if attach fails - container is still running
	} else {
		// Start goroutines to demux stdout/stderr
		m.startOutputHandler(session, sessionID, containerID, reader)
	}
}
```

**Step 7: Run all containersession tests to verify refactoring**

```bash
go test ./pkg/containersession -v
```

Expected: PASS (all existing tests still pass)

**Step 8: Commit**

```bash
git add pkg/containersession/io.go pkg/containersession/io_test.go pkg/containersession/manager.go
git commit -m "refactor(containersession): extract container I/O attachment

Extract attachContainerIO from AttachContainerSession and
StartContainerSession. These methods had EXACT duplicate code
for attaching to container I/O streams.

Changes:
- Add pkg/containersession/io.go with attachContainerIO function
- Add integration tests for I/O attachment
- Refactor AttachContainerSession (87→79 lines)
- Refactor StartContainerSession (62→56 lines)

Benefits:
- Eliminates 15 lines of exact duplication
- Single source of truth for container I/O configuration
- Consistent behavior across attach scenarios

Part of function decomposition sprint (Task 4/15)."
```

---

### Task 5: Workspace Path Validation

**Files:**
- Create: `pkg/workspace/validation.go`
- Create: `pkg/workspace/validation_test.go`

**Step 1: Write failing test for ValidateAndConstrainPath**

Create `pkg/workspace/validation_test.go`:

```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAndConstrainPath(t *testing.T) {
	// Create temp base directory for tests
	baseDir, err := os.MkdirTemp("", "workspace-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(baseDir)

	tests := []struct {
		name      string
		workspace string
		baseDir   string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid relative path",
			workspace: "project/agent-1",
			baseDir:   baseDir,
			wantErr:   false,
		},
		{
			name:      "valid absolute path within base",
			workspace: filepath.Join(baseDir, "project", "agent-2"),
			baseDir:   baseDir,
			wantErr:   false,
		},
		{
			name:      "directory traversal attempt with ..",
			workspace: "../../../etc/passwd",
			baseDir:   baseDir,
			wantErr:   true,
			errMsg:    "must be under base directory",
		},
		{
			name:      "absolute path outside base",
			workspace: "/tmp/outside",
			baseDir:   baseDir,
			wantErr:   true,
			errMsg:    "must be under base directory",
		},
		{
			name:      "path with .. in middle",
			workspace: "project/../../../etc/passwd",
			baseDir:   baseDir,
			wantErr:   true,
			errMsg:    "must be under base directory",
		},
		{
			name:      "exact base directory",
			workspace: ".",
			baseDir:   baseDir,
			wantErr:   false,
		},
		{
			name:      "base directory absolute path",
			workspace: baseDir,
			baseDir:   baseDir,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath, err := ValidateAndConstrainPath(tt.workspace, tt.baseDir)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Empty(t, absPath)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, absPath)
				// Verify returned path is absolute
				assert.True(t, filepath.IsAbs(absPath))
				// Verify returned path is within base directory
				relPath, err := filepath.Rel(tt.baseDir, absPath)
				assert.NoError(t, err)
				assert.NotContains(t, relPath, "..")
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/workspace -v
```

Expected: FAIL with "undefined: ValidateAndConstrainPath"

**Step 3: Write minimal implementation**

Create `pkg/workspace/validation.go`:

```go
// Package workspace provides utilities for workspace path validation and management.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateAndConstrainPath validates and resolves a workspace path, ensuring it's
// constrained under the base directory. Prevents directory traversal attacks.
//
// Security measures:
//  1. Cleans path to remove . and .. components
//  2. Converts to absolute path
//  3. Checks prefix with separator to prevent directory name bypass
//  4. Uses filepath.Rel to detect .. escapes
//
// Returns the absolute path within baseDir, or an error if validation fails.
func ValidateAndConstrainPath(workspace, baseDir string) (string, error) {
	// Clean and get absolute paths
	cleanPath := filepath.Clean(workspace)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid workspace path: %w", err)
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %w", err)
	}

	// Defense-in-depth: Check prefix with separator to prevent directory name bypass
	// Example: "/base" should not match "/base-evil"
	if absPath != baseAbs && !strings.HasPrefix(absPath, baseAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("workspace path must be under base directory %s", baseDir)
	}

	// Use filepath.Rel to prevent directory traversal with ".."
	relPath, err := filepath.Rel(baseAbs, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == ".." || filepath.IsAbs(relPath) {
		return "", fmt.Errorf("workspace path must be under base directory %s", baseDir)
	}

	return absPath, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/workspace -v
```

Expected: PASS (all tests pass)

**Step 5: Commit**

```bash
git add pkg/workspace/
git commit -m "feat(workspace): add path validation with traversal prevention

Extract workspace path validation logic from SpawnAgent. Provides
defense-in-depth protection against directory traversal attacks:

1. Path cleaning (remove . and ..)
2. Absolute path conversion
3. Prefix checking with separator (prevents name bypass)
4. Relative path validation (detects .. escapes)

Will be used by SpawnAgent and potentially other workspace operations.

Part of function decomposition sprint (Task 5/15)."
```

---

## Phase 3: ACP & Launcher Operations (Uses Phase 1, 2)

### Task 6: ACP Client Safe Close (EXACT DUPLICATE!)

**Files:**
- Create: `pkg/relay/session/acp_lifecycle.go`
- Create: `pkg/relay/session/acp_lifecycle_test.go`
- Modify: `pkg/relay/session/manager.go:461-475` (TerminateAgent)
- Modify: `pkg/relay/session/manager.go:537-562` (TerminateUserSession goroutine)

**Step 1: Write failing test for closeACPClientSafely**

Create `pkg/relay/session/acp_lifecycle_test.go`:

```go
package session

import (
	"errors"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockACPClient for testing
type MockACPClient struct {
	mock.Mock
}

func (m *MockACPClient) SendMessage(msg string) (interface{}, error) {
	args := m.Called(msg)
	return args.Get(0), args.Error(1)
}

func (m *MockACPClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestCloseACPClientSafely(t *testing.T) {
	t.Run("closes client successfully", func(t *testing.T) {
		logger := log.Default()
		now := time.Now()

		// Create agent with mock ACP client
		agent := NewAgentSession("agent-1", "/workspace", now)

		mockClient := new(MockACPClient)
		mockClient.On("Close").Return(nil)

		agent.mu.Lock()
		agent.acpClient = mockClient
		agent.mu.Unlock()

		// Close client
		err := closeACPClientSafely(agent, logger, "session-1", "agent-1")
		assert.NoError(t, err)

		// Verify client was closed
		mockClient.AssertExpectations(t)

		// Verify agent state
		agent.mu.Lock()
		defer agent.mu.Unlock()
		assert.Nil(t, agent.acpClient, "client should be cleared")
		assert.Equal(t, AgentTerminated, agent.state, "state should be TERMINATED")
	})

	t.Run("handles close error", func(t *testing.T) {
		logger := log.Default()
		now := time.Now()

		agent := NewAgentSession("agent-2", "/workspace", now)

		mockClient := new(MockACPClient)
		closeErr := errors.New("close failed")
		mockClient.On("Close").Return(closeErr)

		agent.mu.Lock()
		agent.acpClient = mockClient
		agent.mu.Unlock()

		// Close client
		err := closeACPClientSafely(agent, logger, "session-2", "agent-2")
		assert.Equal(t, closeErr, err)

		// Verify client was still cleared despite error
		agent.mu.Lock()
		defer agent.mu.Unlock()
		assert.Nil(t, agent.acpClient, "client should be cleared even on error")
		assert.Equal(t, AgentTerminated, agent.state)
	})

	t.Run("handles nil client gracefully", func(t *testing.T) {
		logger := log.Default()
		now := time.Now()

		agent := NewAgentSession("agent-3", "/workspace", now)

		// No client set
		err := closeACPClientSafely(agent, logger, "session-3", "agent-3")
		assert.NoError(t, err)

		// Verify state updated
		agent.mu.Lock()
		defer agent.mu.Unlock()
		assert.Equal(t, AgentTerminated, agent.state)
	})

	t.Run("prevents double close", func(t *testing.T) {
		logger := log.Default()
		now := time.Now()

		agent := NewAgentSession("agent-4", "/workspace", now)

		mockClient := new(MockACPClient)
		mockClient.On("Close").Return(nil).Once() // Should only be called once

		agent.mu.Lock()
		agent.acpClient = mockClient
		agent.mu.Unlock()

		// First close
		err := closeACPClientSafely(agent, logger, "session-4", "agent-4")
		assert.NoError(t, err)

		// Second close (should not call Close again)
		err = closeACPClientSafely(agent, logger, "session-4", "agent-4")
		assert.NoError(t, err)

		// Verify Close was only called once
		mockClient.AssertExpectations(t)
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/relay/session -run TestCloseACPClientSafely -v
```

Expected: FAIL with "undefined: closeACPClientSafely"

**Step 3: Write minimal implementation**

Create `pkg/relay/session/acp_lifecycle.go`:

```go
package session

import (
	"log"
)

// closeACPClientSafely closes an ACP client with double-close protection.
//
// Thread-safe implementation:
// 1. Locks agent
// 2. Gets acpClient reference and clears it to nil
// 3. Sets agent state to AgentTerminated
// 4. Unlocks agent
// 5. Closes client outside the lock (prevents deadlock)
//
// Double-close protection: Clearing client to nil before Close ensures
// that if this function is called again, it won't attempt to close twice.
//
// Returns any error from client.Close(), but always clears the client reference.
func closeACPClientSafely(
	agent *AgentSession,
	logger *log.Logger,
	userSessionID, agentID string,
) error {
	agent.mu.Lock()
	acpClient := agent.acpClient
	if acpClient != nil {
		agent.acpClient = nil // Clear before Close to prevent double-close
	}
	agent.setAgentState(AgentTerminated)
	agent.mu.Unlock()

	// Close outside the lock to prevent potential deadlock
	if acpClient != nil {
		if err := acpClient.Close(); err != nil {
			logger.Printf("Error closing ACP client: session=%s agentID=%s error=%v",
				userSessionID, agentID, err)
			return err
		}
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/relay/session -run TestCloseACPClientSafely -v
```

Expected: PASS (all tests pass)

**Step 5: Refactor TerminateAgent to use helper**

Modify `pkg/relay/session/manager.go` in `TerminateAgent` method (around line 460):

Before:
```go
// Close ACP client if present (with double-close protection)
agent.mu.Lock()
acpClient := agent.acpClient
if acpClient != nil {
	agent.acpClient = nil // Clear before Close to prevent double-close
}
agent.setAgentState(AgentTerminated)
agent.mu.Unlock()

// Close outside the lock
if acpClient != nil {
	if err := acpClient.Close(); err != nil {
		m.logger.Printf("Error closing ACP client: session=%s agentID=%s error=%v", userSessionID, agentID, err)
		// Continue with cleanup even if close fails
	}
}
```

After:
```go
// Close ACP client if present (with double-close protection)
if err := closeACPClientSafely(agent, m.logger, userSessionID, agentID); err != nil {
	// Continue with cleanup even if close fails
	// Error already logged by closeACPClientSafely
}
```

**Step 6: Refactor TerminateUserSession goroutine to use helper**

Modify `pkg/relay/session/manager.go` in `TerminateUserSession` method (around line 536):

Before:
```go
// Close ACP client (with double-close protection)
a.mu.Lock()
acpClient := a.acpClient
if acpClient != nil {
	a.acpClient = nil // Clear before Close to prevent double-close
}
a.setAgentState(AgentTerminated)
a.mu.Unlock()

// Close outside the lock
if acpClient != nil {
	done := make(chan error, 1)
	go func() {
		done <- acpClient.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			m.logger.Printf("Error closing agent: userSession=%s agentID=%s error=%v", userSessionID, id, err)
			failed = true
		}
	case <-agentCtx.Done():
		m.logger.Printf("Agent close timeout: userSession=%s agentID=%s", userSessionID, id)
		failed = true
	}
}
```

After:
```go
// Close ACP client (with double-close protection and timeout)
done := make(chan error, 1)
go func() {
	done <- closeACPClientSafely(a, m.logger, userSessionID, id)
}()

select {
case err := <-done:
	if err != nil {
		// Error already logged by closeACPClientSafely
		failed = true
	}
case <-agentCtx.Done():
	m.logger.Printf("Agent close timeout: userSession=%s agentID=%s", userSessionID, id)
	failed = true
}
```

**Step 7: Run all session manager tests**

```bash
go test ./pkg/relay/session -v
```

Expected: PASS (all existing tests still pass)

**Step 8: Commit**

```bash
git add pkg/relay/session/acp_lifecycle.go pkg/relay/session/acp_lifecycle_test.go pkg/relay/session/manager.go
git commit -m "refactor(session): extract ACP client safe close

Extract closeACPClientSafely from TerminateAgent and
TerminateUserSession. These methods had EXACT duplicate logic
for safely closing ACP clients with double-close protection.

Implementation:
- Lock agent, get and clear client, set state, unlock
- Close outside lock (prevents deadlock)
- Double-close protection via nil check

Changes:
- Add pkg/relay/session/acp_lifecycle.go
- Add comprehensive tests including double-close prevention
- Refactor TerminateAgent (72→66 lines)
- Refactor TerminateUserSession goroutine (26→12 lines)

Benefits:
- Eliminates 30 lines of exact duplication
- Single source of truth for ACP client lifecycle
- Consistent double-close protection

Part of function decomposition sprint (Task 6/15)."
```

---

### Task 7: Container/Launcher Cleanup (EXACT DUPLICATE!)

**Files:**
- Create: `pkg/relay/session/launcher_lifecycle.go`
- Create: `pkg/relay/session/launcher_lifecycle_test.go`
- Modify: `pkg/relay/session/manager.go:438-458` (TerminateAgent)
- Modify: `pkg/relay/session/manager.go:564-585` (TerminateUserSession goroutine)
- Modify: `pkg/relay/session/manager.go:338-352` (SpawnAgent cleanup)

**Step 1: Write failing test for stopContainerAndCleanupLauncher**

Create `pkg/relay/session/launcher_lifecycle_test.go`:

```go
package session

import (
	"context"
	"errors"
	"log"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/2389-research/ourocodus/pkg/agent"
)

// MockLauncherFactory for testing
type MockLauncherFactory struct {
	mock.Mock
}

func (m *MockLauncherFactory) CreateLauncher(ctx context.Context, agentID string, config agent.LauncherConfig) (agent.Launcher, error) {
	args := m.Called(ctx, agentID, config)
	if launcher := args.Get(0); launcher != nil {
		return launcher.(agent.Launcher), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockLauncher for testing
type MockLauncher struct {
	mock.Mock
}

func (m *MockLauncher) Spawn(ctx context.Context, config *agent.SpawnConfig) (agent.AgentHandle, error) {
	args := m.Called(ctx, config)
	if handle := args.Get(0); handle != nil {
		return handle.(agent.AgentHandle), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockLauncher) Stop(ctx context.Context, handle agent.AgentHandle) error {
	args := m.Called(ctx, handle)
	return args.Error(0)
}

func (m *MockLauncher) GetHandle(agentID string) (agent.AgentHandle, error) {
	args := m.Called(agentID)
	if handle := args.Get(0); handle != nil {
		return handle.(agent.AgentHandle), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockLauncher) ListHandles() []agent.AgentHandle {
	args := m.Called()
	return args.Get(0).([]agent.AgentHandle)
}

func (m *MockLauncher) Attach(ctx context.Context, handle agent.AgentHandle) error {
	args := m.Called(ctx, handle)
	return args.Error(0)
}

// MockAgentHandle for testing
type MockAgentHandle struct {
	mock.Mock
}

func (m *MockAgentHandle) ContainerID() string {
	args := m.Called()
	return args.String(0)
}

func TestStopContainerAndCleanupLauncher(t *testing.T) {
	t.Run("stops container and cleans up launcher", func(t *testing.T) {
		ctx := context.Background()
		logger := log.Default()

		mockFactory := new(MockLauncherFactory)
		mockLauncher := new(MockLauncher)
		mockHandle := new(MockAgentHandle)

		launchers := make(map[string]agent.Launcher)
		handles := make(map[string]agent.AgentHandle)
		var mu sync.RWMutex

		key := "session-1:agent-1"
		launchers[key] = mockLauncher
		handles[key] = mockHandle

		mockHandle.On("ContainerID").Return("container-123")
		mockLauncher.On("Stop", ctx, mockHandle).Return(nil)

		err := stopContainerAndCleanupLauncher(
			ctx,
			mockFactory,
			launchers,
			handles,
			&mu,
			key,
			logger,
			"agent-1",
		)

		assert.NoError(t, err)
		mockLauncher.AssertExpectations(t)

		// Verify cleanup
		assert.NotContains(t, launchers, key)
		assert.NotContains(t, handles, key)
	})

	t.Run("handles stop error gracefully", func(t *testing.T) {
		ctx := context.Background()
		logger := log.Default()

		mockFactory := new(MockLauncherFactory)
		mockLauncher := new(MockLauncher)
		mockHandle := new(MockAgentHandle)

		launchers := make(map[string]agent.Launcher)
		handles := make(map[string]agent.AgentHandle)
		var mu sync.RWMutex

		key := "session-2:agent-2"
		launchers[key] = mockLauncher
		handles[key] = mockHandle

		stopErr := errors.New("stop failed")
		mockHandle.On("ContainerID").Return("container-456")
		mockLauncher.On("Stop", ctx, mockHandle).Return(stopErr)

		err := stopContainerAndCleanupLauncher(
			ctx,
			mockFactory,
			launchers,
			handles,
			&mu,
			key,
			logger,
			"agent-2",
		)

		assert.Equal(t, stopErr, err)
		mockLauncher.AssertExpectations(t)

		// Verify cleanup still happened despite error
		assert.NotContains(t, launchers, key)
		assert.NotContains(t, handles, key)
	})

	t.Run("handles missing launcher gracefully", func(t *testing.T) {
		ctx := context.Background()
		logger := log.Default()

		mockFactory := new(MockLauncherFactory)
		launchers := make(map[string]agent.Launcher)
		handles := make(map[string]agent.AgentHandle)
		var mu sync.RWMutex

		key := "session-3:agent-3"
		// No launcher or handle

		err := stopContainerAndCleanupLauncher(
			ctx,
			mockFactory,
			launchers,
			handles,
			&mu,
			key,
			logger,
			"agent-3",
		)

		assert.NoError(t, err)
	})

	t.Run("handles missing handle gracefully", func(t *testing.T) {
		ctx := context.Background()
		logger := log.Default()

		mockFactory := new(MockLauncherFactory)
		mockLauncher := new(MockLauncher)

		launchers := make(map[string]agent.Launcher)
		handles := make(map[string]agent.AgentHandle)
		var mu sync.RWMutex

		key := "session-4:agent-4"
		launchers[key] = mockLauncher
		// No handle

		err := stopContainerAndCleanupLauncher(
			ctx,
			mockFactory,
			launchers,
			handles,
			&mu,
			key,
			logger,
			"agent-4",
		)

		assert.NoError(t, err)

		// Verify launcher still cleaned up
		assert.NotContains(t, launchers, key)
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/relay/session -run TestStopContainerAndCleanupLauncher -v
```

Expected: FAIL with "undefined: stopContainerAndCleanupLauncher"

**Step 3: Write minimal implementation**

Add to `pkg/relay/session/launcher_lifecycle.go`:

```go
package session

import (
	"context"
	"log"
	"sync"

	"github.com/2389-research/ourocodus/pkg/agent"
)

// stopContainerAndCleanupLauncher stops a container and removes launcher/handle from maps.
//
// This is the standard cleanup pattern for agent containers:
// 1. Get launcher and handle from maps (with read lock)
// 2. Stop container if both exist
// 3. Remove from maps (with write lock)
//
// Always removes from maps even if stop fails, ensuring cleanup progresses.
// Logs errors but returns them so caller can decide how to handle.
func stopContainerAndCleanupLauncher(
	ctx context.Context,
	launcherFactory agent.LauncherFactory,
	launchers map[string]agent.Launcher,
	handles map[string]agent.AgentHandle,
	launchersMu *sync.RWMutex,
	key string,
	logger *log.Logger,
	agentID string,
) error {
	// Get launcher and handle
	launchersMu.RLock()
	launcher := launchers[key]
	handle := handles[key]
	launchersMu.RUnlock()

	var stopErr error
	if launcher != nil && handle != nil {
		stopErr = launcher.Stop(ctx, handle)
		if stopErr != nil {
			logger.Printf("WARN: Failed to stop container for agent %s: %v", agentID, stopErr)
			// Continue cleanup despite error
		}
	}

	// Remove from launcher maps (always cleanup, even if stop failed)
	launchersMu.Lock()
	delete(launchers, key)
	delete(handles, key)
	launchersMu.Unlock()

	return stopErr
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/relay/session -run TestStopContainerAndCleanupLauncher -v
```

Expected: PASS (all tests pass)

**Step 5: Refactor TerminateAgent to use helper**

Modify `pkg/relay/session/manager.go` in `TerminateAgent` method (around line 438):

Before:
```go
// Stop container if launcher exists (container mode only)
if m.launcherFactory != nil && runtime.IsContainerMode() {
	key := launcherKey(userSessionID, agentID)
	m.launchersMu.RLock()
	launcher := m.launchers[key]
	handle := m.handles[key]
	m.launchersMu.RUnlock()

	if launcher != nil && handle != nil {
		if err := launcher.Stop(ctx, handle); err != nil {
			m.logger.Printf("WARN: Failed to stop container for agent %s: %v", agentID, err)
			// Continue cleanup despite error
		}
	}

	// Remove from launcher maps
	m.launchersMu.Lock()
	delete(m.launchers, key)
	delete(m.handles, key)
	m.launchersMu.Unlock()
}
```

After:
```go
// Stop container if launcher exists (container mode only)
if m.launcherFactory != nil && runtime.IsContainerMode() {
	key := launcherKey(userSessionID, agentID)
	_ = stopContainerAndCleanupLauncher(
		ctx,
		m.launcherFactory,
		m.launchers,
		m.handles,
		m.launchersMu,
		key,
		m.logger,
		agentID,
	)
	// Error already logged, continue with cleanup
}
```

**Step 6: Refactor TerminateUserSession goroutine to use helper**

Modify `pkg/relay/session/manager.go` in `TerminateUserSession` method (around line 564):

Before:
```go
// Stop container if launcher exists (container mode only)
if m.launcherFactory != nil && runtime.IsContainerMode() {
	key := launcherKey(userSessionID, id)
	m.launchersMu.RLock()
	launcher := m.launchers[key]
	handle := m.handles[key]
	m.launchersMu.RUnlock()

	if launcher != nil && handle != nil {
		if err := launcher.Stop(agentCtx, handle); err != nil {
			m.logger.Printf("WARN: Failed to stop container for agent %s: %v", id, err)
			failed = true
			// Continue cleanup despite error
		}
	}

	// Remove from launcher maps
	m.launchersMu.Lock()
	delete(m.launchers, key)
	delete(m.handles, key)
	m.launchersMu.Unlock()
}
```

After:
```go
// Stop container if launcher exists (container mode only)
if m.launcherFactory != nil && runtime.IsContainerMode() {
	key := launcherKey(userSessionID, id)
	if err := stopContainerAndCleanupLauncher(
		agentCtx,
		m.launcherFactory,
		m.launchers,
		m.handles,
		m.launchersMu,
		key,
		m.logger,
		id,
	); err != nil {
		failed = true
		// Error already logged, continue cleanup
	}
}
```

**Step 7: Refactor SpawnAgent cleanup to use helper**

Modify `pkg/relay/session/manager.go` in `SpawnAgent` method (around line 338):

Before:
```go
// Cleanup launcher on client creation failure
if m.launcherFactory != nil && runtime.IsContainerMode() {
	key := launcherKey(userSessionID, agentID)
	m.launchersMu.Lock()
	launcher := m.launchers[key]
	handle := m.handles[key]
	delete(m.launchers, key)
	delete(m.handles, key)
	m.launchersMu.Unlock()

	if launcher != nil && handle != nil {
		m.logger.Printf("[SESSION] ├─ Cleaning up container after ACP client failure")
		_ = launcher.Stop(ctx, handle)
	}
}
```

After:
```go
// Cleanup launcher on client creation failure
if m.launcherFactory != nil && runtime.IsContainerMode() {
	key := launcherKey(userSessionID, agentID)
	m.logger.Printf("[SESSION] ├─ Cleaning up container after ACP client failure")
	_ = stopContainerAndCleanupLauncher(
		ctx,
		m.launcherFactory,
		m.launchers,
		m.handles,
		m.launchersMu,
		key,
		m.logger,
		agentID,
	)
}
```

**Step 8: Run all session manager tests**

```bash
go test ./pkg/relay/session -v
```

Expected: PASS (all existing tests still pass)

**Step 9: Commit**

```bash
git add pkg/relay/session/launcher_lifecycle.go pkg/relay/session/launcher_lifecycle_test.go pkg/relay/session/manager.go
git commit -m "refactor(session): extract container/launcher cleanup

Extract stopContainerAndCleanupLauncher from TerminateAgent,
TerminateUserSession, and SpawnAgent. These methods had EXACT
duplicate logic for stopping containers and cleaning up launchers.

Implementation:
- Get launcher and handle (read lock)
- Stop container if both exist
- Remove from maps (write lock)
- Always cleanup, even if stop fails

Changes:
- Add pkg/relay/session/launcher_lifecycle.go
- Add comprehensive tests with mocks
- Refactor TerminateAgent (72→59 lines)
- Refactor TerminateUserSession goroutine (22→11 lines)
- Refactor SpawnAgent cleanup (15→7 lines)

Benefits:
- Eliminates 50 lines of exact duplication (3 locations)
- Single source of truth for container cleanup
- Consistent error handling

Part of function decomposition sprint (Task 7/15)."
```

---

## Phase 4: Advanced Patterns

### Task 8: Cleanup Chain Pattern

**Files:**
- Create: `pkg/cleanup/chain.go`
- Create: `pkg/cleanup/chain_test.go`

**Step 1: Write failing test for CleanupChain**

Create `pkg/cleanup/chain_test.go`:

```go
package cleanup

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanupChain(t *testing.T) {
	t.Run("executes cleanups in reverse order", func(t *testing.T) {
		var executed []string

		chain := NewChain()
		chain.Add("first", func() error {
			executed = append(executed, "first")
			return nil
		})
		chain.Add("second", func() error {
			executed = append(executed, "second")
			return nil
		})
		chain.Add("third", func() error {
			executed = append(executed, "third")
			return nil
		})

		errs := chain.Execute()
		assert.Empty(t, errs)
		assert.Equal(t, []string{"third", "second", "first"}, executed)
	})

	t.Run("collects all errors", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		chain := NewChain()
		chain.Add("first", func() error {
			return err1
		})
		chain.Add("second", func() error {
			return nil
		})
		chain.Add("third", func() error {
			return err2
		})

		errs := chain.Execute()
		assert.Len(t, errs, 2)
		// Executed in reverse order, so err2 comes first
		assert.ErrorIs(t, errs[0], err2)
		assert.ErrorIs(t, errs[1], err1)
	})

	t.Run("disables all cleanups", func(t *testing.T) {
		var executed []string

		chain := NewChain()
		chain.Add("first", func() error {
			executed = append(executed, "first")
			return nil
		})
		chain.Add("second", func() error {
			executed = append(executed, "second")
			return nil
		})

		chain.DisableAll()

		errs := chain.Execute()
		assert.Empty(t, errs)
		assert.Empty(t, executed, "no cleanups should execute")
	})

	t.Run("disables individual cleanup", func(t *testing.T) {
		var executed []string

		chain := NewChain()
		chain.Add("first", func() error {
			executed = append(executed, "first")
			return nil
		})
		entry := chain.Add("second", func() error {
			executed = append(executed, "second")
			return nil
		})
		chain.Add("third", func() error {
			executed = append(executed, "third")
			return nil
		})

		entry.Disable()

		errs := chain.Execute()
		assert.Empty(t, errs)
		assert.Equal(t, []string{"third", "first"}, executed)
	})

	t.Run("empty chain", func(t *testing.T) {
		chain := NewChain()
		errs := chain.Execute()
		assert.Empty(t, errs)
	})
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./pkg/cleanup -v
```

Expected: FAIL with "undefined: NewChain"

**Step 3: Write minimal implementation**

Create `pkg/cleanup/chain.go`:

```go
// Package cleanup provides utilities for managing cleanup operations with error collection.
package cleanup

import (
	"fmt"
)

// Chain manages a sequence of cleanup operations that can be disabled and executed in reverse order.
//
// Usage pattern:
//   chain := cleanup.NewChain()
//   entry1 := chain.Add("worktree", func() error { return cleanupWorktree() })
//   entry2 := chain.Add("credentials", func() error { return cleanupCreds() })
//   entry3 := chain.Add("container", func() error { return stopContainer() })
//
//   // On success, disable all cleanups
//   if success {
//       chain.DisableAll()
//   }
//
//   // Execute remaining cleanups (in reverse order: container, creds, worktree)
//   errs := chain.Execute()
type Chain struct {
	entries []*Entry
}

// Entry represents a single cleanup operation in the chain.
type Entry struct {
	name     string
	cleanup  func() error
	disabled bool
}

// NewChain creates a new cleanup chain.
func NewChain() *Chain {
	return &Chain{
		entries: make([]*Entry, 0),
	}
}

// Add adds a cleanup operation to the chain.
// Returns an Entry that can be used to disable this specific cleanup.
func (c *Chain) Add(name string, cleanup func() error) *Entry {
	entry := &Entry{
		name:    name,
		cleanup: cleanup,
	}
	c.entries = append(c.entries, entry)
	return entry
}

// DisableAll disables all cleanup operations.
// Call this when the operation succeeds and no cleanup is needed.
func (c *Chain) DisableAll() {
	for _, entry := range c.entries {
		entry.disabled = true
	}
}

// Execute runs all enabled cleanup operations in reverse order (LIFO).
// Collects and returns all errors, but continues executing remaining cleanups.
func (c *Chain) Execute() []error {
	var errs []error

	// Execute in reverse order (LIFO - last added, first executed)
	for i := len(c.entries) - 1; i >= 0; i-- {
		entry := c.entries[i]
		if entry.disabled {
			continue
		}

		if err := entry.cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("cleanup '%s' failed: %w", entry.name, err))
		}
	}

	return errs
}

// Disable disables this specific cleanup operation.
func (e *Entry) Disable() {
	e.disabled = true
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./pkg/cleanup -v
```

Expected: PASS (all tests pass)

**Step 5: Commit**

```bash
git add pkg/cleanup/
git commit -m "feat(cleanup): add cleanup chain pattern

Add CleanupChain utility for managing cleanup operations with
disable control. Replaces multiple boolean flags with cleaner
abstraction.

Features:
- Add cleanups with named entries
- Execute in reverse order (LIFO)
- Disable individual cleanups or all
- Collect all errors

Will be used by:
- AgentContainerLauncher.Spawn (3 cleanup booleans)
- SpawnAgent (ACP cleanup)
- CreateContainerSessionWithConfig (workspace cleanup)

Part of function decomposition sprint (Task 8/15)."
```

---

## Phase 5: Function Decomposition (Using All Helpers)

Now we'll decompose the large functions, using all the helpers we've extracted.

### Task 9: Decompose Publish (NATS Client)

**Status:** Already completed in Task 2, Step 5-7

---

### Task 10: Decompose AttachContainerSession

**Status:** Already completed in Task 4, Step 5

---

### Task 11: Decompose StartContainerSession

**Status:** Already completed in Task 4, Step 6

---

### Task 12: Decompose TerminateAgent

**Status:** Already completed in Task 6, Step 5 and Task 7, Step 5

Let's verify the final state and add remaining small helpers if needed.

**Step 1: Verify current TerminateAgent implementation**

```bash
# Check current line count
wc -l pkg/relay/session/manager.go
```

**Step 2: Check if further decomposition is beneficial**

The function should now be around 55-60 lines after using closeACPClientSafely and stopContainerAndCleanupLauncher.

If there are still opportunities, extract:

```go
// removeAgentFromSession removes an agent from its parent session and updates last active time.
func removeAgentFromSession(userSession *UserSession, agentID string, now time.Time) {
	userSession.mu.Lock()
	userSession.removeAgent(agentID)
	userSession.setLastActive(now)
	userSession.mu.Unlock()
}
```

But this is small enough that extraction may not be worth it. The function is now manageable.

---

### Task 13: Decompose TerminateUserSession

**Status:** Already used helpers in Task 6, Step 6 and Task 7, Step 6

The function is now ~120 lines (down from 150). The goroutine is much cleaner.

We could extract the agent termination goroutine into a helper:

**Step 1: Create terminateAgentAsync helper**

Create in `pkg/relay/session/acp_lifecycle.go`:

```go
// terminateAgentAsync terminates a single agent with timeout, tracking success/failure.
//
// Used by TerminateUserSession to terminate multiple agents in parallel.
// Updates atomic counters for successes and failures.
func terminateAgentAsync(
	ctx context.Context,
	m *Manager,
	userSessionID, agentID string,
	agent *AgentSession,
	timeout time.Duration,
	successCounter, failureCounter *int32,
) {
	failed := false

	// Create context with timeout for this agent
	agentCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Close ACP client (with double-close protection and timeout)
	done := make(chan error, 1)
	go func() {
		done <- closeACPClientSafely(agent, m.logger, userSessionID, agentID)
	}()

	select {
	case err := <-done:
		if err != nil {
			// Error already logged by closeACPClientSafely
			failed = true
		}
	case <-agentCtx.Done():
		m.logger.Printf("Agent close timeout: userSession=%s agentID=%s", userSessionID, agentID)
		failed = true
	}

	// Stop container if launcher exists (container mode only)
	if m.launcherFactory != nil && runtime.IsContainerMode() {
		key := launcherKey(userSessionID, agentID)
		if err := stopContainerAndCleanupLauncher(
			agentCtx,
			m.launcherFactory,
			m.launchers,
			m.handles,
			m.launchersMu,
			key,
			m.logger,
			agentID,
		); err != nil {
			failed = true
			// Error already logged, continue cleanup
		}
	}

	if failed {
		atomic.AddInt32(failureCounter, 1)
	} else {
		atomic.AddInt32(successCounter, 1)
	}
}
```

**Step 2: Refactor TerminateUserSession to use terminateAgentAsync**

The goroutine body becomes:

```go
for agentID, agent := range agents {
	wg.Add(1)
	go func(id string, a *AgentSession) {
		defer wg.Done()
		terminateAgentAsync(
			ctx,
			m,
			userSessionID,
			id,
			a,
			agentTimeout,
			&agentSuccesses,
			&agentFailures,
		)
	}(agentID, agent)
}
```

**Step 3: Add test and commit**

This is optional - the existing code is already much cleaner. Consider this a "stretch goal."

---

### Task 14: Decompose handleAgentMessage

**Files:**
- Modify: `pkg/relay/server.go:335-464` (handleAgentMessage)

**Step 1: Extract getActiveAgent helper**

Add to `pkg/relay/server.go` (before handleAgentMessage):

```go
// getActiveAgent retrieves an agent and validates it's in ACTIVE state.
// Returns the agent on success, or an ErrorMessage to send to client on failure.
func (s *Server) getActiveAgent(
	userSessionID, agentID string,
) (*session.AgentSession, *ErrorMessage) {
	// Get agent from user session
	agent, err := s.sessionManager.GetAgent(userSessionID, agentID)
	if err != nil {
		s.logger.Printf("Failed to get agent: %v", err)

		// Map error to protocol error code
		errorCode, errorMessage, recoverable := MapToProtocolError(err)
		return nil, NewErrorMessage(errorCode, errorMessage, recoverable)
	}

	// Check agent state
	if agent.GetState() != session.AgentActive {
		s.logger.Printf("Agent not ready: userSession=%s agentID=%s state=%s",
			userSessionID, agentID, agent.GetState())

		return nil, NewErrorMessage(
			"AGENT_NOT_READY",
			fmt.Sprintf("Agent is not ready (current state: %s)", agent.GetState()),
			true, // Recoverable - client can wait and retry
		)
	}

	// Get ACP client
	acpClient := agent.GetACPClient()
	if acpClient == nil {
		s.logger.Printf("Agent has no ACP client: userSession=%s agentID=%s", userSessionID, agentID)

		return nil, NewErrorMessage(
			"AGENT_NOT_READY",
			"Agent ACP client not initialized",
			true, // Recoverable
		)
	}

	return agent, nil
}

// sendToAgent sends a message to an agent's ACP client.
// Returns the response content on success, or an ErrorMessage on failure.
func (s *Server) sendToAgent(
	agent *session.AgentSession,
	userSessionID, agentID, content string,
) (string, *ErrorMessage) {
	acpClient := agent.GetACPClient()

	// Send message to agent
	response, err := acpClient.SendMessage(content)
	if err != nil {
		s.logger.Printf("Agent message failed: %v", err)

		return "", NewErrorMessage(
			"AGENT_MESSAGE_FAILED",
			fmt.Sprintf("Failed to send message to agent: %v", err),
			true, // Recoverable - client can retry
		)
	}

	s.logger.Printf("[RELAY] Agent response received: userSession=%s agentID=%s",
		userSessionID, agentID)

	// Type assert response to *acp.AgentMessage and extract content
	agentMsg, ok := response.(*acp.AgentMessage)
	if !ok {
		s.logger.Printf("Invalid response type from agent: %T", response)
		return "", NewErrorMessage(
			"AGENT_MESSAGE_FAILED",
			"Invalid response format from agent",
			true, // Recoverable - client can retry
		)
	}

	return agentMsg.Content, nil
}
```

**Step 2: Refactor handleAgentMessage to use helpers**

Replace the body of `handleAgentMessage`:

```go
func (s *Server) handleAgentMessage(ctx context.Context, conn WebSocketConn, rawMessage []byte) bool {
	// Parse message
	msg, err := parseAgentMessageRequest(rawMessage)
	if err != nil {
		return s.handleValidationError(conn, err)
	}

	// Validate message
	if validationErr := validateAgentMessageRequest(msg); validationErr != nil {
		return s.handleValidationError(conn, validationErr)
	}

	_ = ctx // TODO: Pass context to ACP client when timeout/cancellation support is implemented

	s.logger.Printf("[RELAY] Routing message to agent: userSession=%s agentID=%s",
		msg.UserSessionID, msg.AgentID)

	// Get active agent
	agent, errorMsg := s.getActiveAgent(msg.UserSessionID, msg.AgentID)
	if errorMsg != nil {
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true // Close on write error
		}
		return false // Keep connection open for retry
	}

	// Send message to agent
	responseContent, errorMsg := s.sendToAgent(agent, msg.UserSessionID, msg.AgentID, msg.Content)
	if errorMsg != nil {
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true // Close on write error
		}
		return false // Keep connection open for retry
	}

	// Store both messages in history after successful ACP response
	now := s.sessionClock.Now()
	agent.AddMessage("user", msg.Content, now)
	agent.AddMessage("agent", responseContent, now)

	// Send agent:response
	responseMsg := NewAgentMessageResponse(
		msg.UserSessionID,
		msg.AgentID,
		responseContent,
		s.clock.Now(),
	)
	if err := conn.WriteJSON(responseMsg); err != nil {
		s.logger.Printf("Failed to send agent:response: %v", err)
		return true // Close on write failure
	}

	return false // Continue processing messages
}
```

**Step 3: Run relay tests**

```bash
go test ./pkg/relay -v
```

Expected: PASS (all existing tests still pass)

**Step 4: Commit**

```bash
git add pkg/relay/server.go
git commit -m "refactor(relay): decompose handleAgentMessage

Extract helpers from handleAgentMessage (130 lines → ~35 lines):

- getActiveAgent: retrieves agent and validates state (40 lines)
- sendToAgent: sends message to ACP client (35 lines)

Main function becomes clean pipeline:
1. Parse and validate message
2. Get active agent (or return error)
3. Send to agent (or return error)
4. Record history
5. Send response

Benefits:
- 77% reduction in main function
- Each step independently testable
- Clear error handling paths

Part of function decomposition sprint (Task 14/15)."
```

---

### Task 15: Decompose CreateContainerSessionWithConfig

**Files:**
- Modify: `pkg/containersession/manager.go:180-308` (CreateContainerSessionWithConfig)
- Modify: `pkg/containersession/manager.go:86-159` (CreateContainerSession)

**Step 1: Extract workspace resolution helper**

Add to `pkg/containersession/workspace.go` (create new file):

```go
package containersession

import (
	"fmt"
	"os"
	"path/filepath"
)

// resolveWorkspacePath determines the workspace path for a container session.
//
// If config.WorkspaceDir is provided:
//   - Validates it exists and is accessible
//   - Resolves symlinks for Docker compatibility (e.g., macOS /tmp -> /private/tmp)
//   - Returns the resolved path
//
// If config.WorkspaceDir is empty:
//   - Creates default workspace under baseWorkspaceDir/sessionID
//   - Returns the created path
func resolveWorkspacePath(
	workspaceDir, baseWorkspaceDir, sessionID string,
) (string, error) {
	if workspaceDir != "" {
		// Use provided workspace path
		// Basic validation: ensure path exists and is accessible
		if _, err := os.Stat(workspaceDir); err != nil {
			return "", fmt.Errorf("workspace directory not accessible: %w", err)
		}

		// Resolve symlinks for Docker compatibility (e.g., macOS /tmp -> /private/tmp)
		resolvedPath, err := filepath.EvalSymlinks(workspaceDir)
		if err != nil {
			// If symlink resolution fails, use the original path
			resolvedPath = workspaceDir
		}

		return resolvedPath, nil
	}

	// Create default workspace
	workspacePath, err := PrepareWorkspace(baseWorkspaceDir, sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to prepare workspace: %w", err)
	}

	return workspacePath, nil
}
```

**Step 2: Extract container config builder**

Add to `pkg/containersession/config.go` (create new file):

```go
package containersession

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

// buildContainerConfig creates Docker container and host configurations.
//
// Container config includes:
//   - Image, command, labels, environment
//   - stdin/stdout/stderr configuration for JSON-RPC
//
// Host config includes:
//   - Workspace mount at /workspace
//   - Custom mounts from config
func buildContainerConfig(
	imageName string,
	command []string,
	entrypoint []string,
	labels map[string]string,
	env []string,
	workspacePath string,
	customMounts []mount.Mount,
) (*container.Config, *container.HostConfig) {
	containerConfig := &container.Config{
		Image:        imageName,
		Cmd:          command,
		Labels:       labels,
		Env:          env,
		OpenStdin:    true,  // Keep stdin open even when not attached
		StdinOnce:    false, // Don't close stdin after first attach
		AttachStdin:  true,  // Enable stdin attachment
		AttachStdout: true,  // Enable stdout attachment
		AttachStderr: true,  // Enable stderr attachment
		Tty:          false, // No TTY (we need raw streams for JSON-RPC)
	}

	// Override entrypoint if specified (nil = use image default, empty slice = clear entrypoint)
	if entrypoint != nil {
		containerConfig.Entrypoint = entrypoint
	}

	// Build host config with mounts
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: workspacePath,
			Target: "/workspace",
		},
	}
	mounts = append(mounts, customMounts...)

	hostConfig := &container.HostConfig{
		Mounts: mounts,
	}

	return containerConfig, hostConfig
}
```

**Step 3: Refactor CreateContainerSessionWithConfig**

Replace the body with:

```go
func (m *Manager) CreateContainerSessionWithConfig(ctx context.Context, config CreateConfig) (*ContainerSession, error) {
	// Validate required fields
	if err := validation.ValidateImageName(config.ImageName); err != nil {
		return nil, err
	}
	if err := validation.ValidateCommand(config.Command); err != nil {
		return nil, err
	}

	// Generate unique session ID
	sessionID := m.idGen.Generate()
	now := m.clock.Now()

	// Check for existing container with this session ID (for reuse scenarios)
	existingID, state, err := m.findContainer(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing container: %w", err)
	}

	// If container exists, handle it based on state
	if existingID != "" {
		return m.handleExistingContainer(ctx, existingID, state, sessionID)
	}

	// No existing container - proceed with creation
	// Build labels (merge defaults with custom labels)
	labels := BuildLabels(sessionID, now)
	for k, v := range config.Labels {
		labels[k] = v
	}

	// Resolve workspace path
	workspacePath, err := resolveWorkspacePath(config.WorkspaceDir, m.baseWorkspaceDir, sessionID)
	if err != nil {
		return nil, err
	}

	// Create session in PENDING state
	session := NewContainerSession(sessionID, workspacePath, labels, now)
	session.skipOutputLogging = config.SkipOutputLogging

	// Store session (with TOCTOU prevention)
	m.mu.Lock()
	if _, exists := m.sessions[sessionID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrSessionAlreadyExists, sessionID)
	}
	m.sessions[sessionID] = session
	m.mu.Unlock()

	// Build container config
	containerConfig, hostConfig := buildContainerConfig(
		config.ImageName,
		config.Command,
		config.Entrypoint,
		labels,
		config.Env,
		workspacePath,
		config.CustomMounts,
	)

	// Create container
	resp, err := m.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		// Cleanup on failure
		m.mu.Lock()
		delete(m.sessions, sessionID)
		m.mu.Unlock()

		// Clean up workspace directory if we created it
		if config.WorkspaceDir == "" {
			if cleanupErr := CleanupWorkspace(workspacePath, m.logger); cleanupErr != nil {
				m.logger.Printf("Workspace cleanup failed: session=%s path=%s error=%v",
					sessionID, workspacePath, cleanupErr)
			}
		}

		session.SetError(err.Error())
		m.logger.Printf("Container creation failed: session=%s error=%v", sessionID, err)
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	session.SetContainerID(resp.ID)
	m.logger.Printf("[CONTAINER] Session created: id=%s container=%s state=PENDING mounts=%d",
		sessionID, resp.ID, len(hostConfig.Mounts))

	return session, nil
}
```

**Step 4: Refactor CreateContainerSession to delegate**

Replace the entire function with:

```go
func (m *Manager) CreateContainerSession(ctx context.Context, imageName string, cmd []string) (*ContainerSession, error) {
	config := CreateConfig{
		ImageName: imageName,
		Command:   cmd,
		// WorkspaceDir empty = use default
		// Labels, Env, etc. use zero values = defaults
	}
	return m.CreateContainerSessionWithConfig(ctx, config)
}
```

**Step 5: Run containersession tests**

```bash
go test ./pkg/containersession -v
```

Expected: PASS (all existing tests still pass)

**Step 6: Commit**

```bash
git add pkg/containersession/workspace.go pkg/containersession/config.go pkg/containersession/manager.go pkg/validation/
git commit -m "refactor(containersession): decompose create session methods

Extract helpers from CreateContainerSessionWithConfig (129→65 lines):

- resolveWorkspacePath: workspace resolution (25 lines)
- buildContainerConfig: container config builder (30 lines)

Refactor CreateContainerSession to delegate (74→7 lines):
- Now thin wrapper around CreateContainerSessionWithConfig
- Eliminates all duplication

Changes:
- Add pkg/containersession/workspace.go
- Add pkg/containersession/config.go
- Use validation helpers from pkg/validation (Task 1)

Benefits:
- 91% reduction in CreateContainerSession (duplication eliminated)
- 50% reduction in CreateContainerSessionWithConfig
- Config building reusable for other container operations

Part of function decomposition sprint (Task 15/15)."
```

---

## Final Verification & Summary

### Task 16: Run Full Test Suite

**Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: PASS (all tests across all packages)

**Step 2: Run linter**

```bash
make lint
```

Expected: No new issues

**Step 3: Run formatter**

```bash
make fmt
```

Expected: No changes (or auto-format applied)

**Step 4: Verify line count reductions**

```bash
# Original top 10 functions: ~1,073 lines
# After decomposition:

wc -l pkg/relay/session/manager.go  # Should be significantly reduced
wc -l pkg/containersession/manager.go  # Should be significantly reduced
wc -l pkg/relay/server.go  # Should be significantly reduced
wc -l pkg/nats/client.go  # Should be reduced
wc -l pkg/agent/container/launcher.go  # Should be reduced

# Count new helper packages
find pkg/ -name "*lifecycle*" -o -name "*validation*" -o -name "*retry*" | wc -l
```

**Step 5: Generate summary report**

Create a summary showing:
- Lines eliminated through extraction: ~230 lines
- Lines eliminated through decomposition: ~771 lines
- Total impact: ~1,000 lines → cleaner, testable code
- Number of reusable helpers created: 10+
- Test coverage added: 15+ new test files

---

## Execution Options

Plan complete and saved to `docs/plans/2025-11-13-function-decomposition-sprint.md`.

**Two execution options:**

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?**

---

## Notes for Implementation

**Key Success Factors:**
1. **Follow TDD religiously**: Test → Fail → Implement → Pass → Commit
2. **Extract before refactor**: Create helpers first (Tasks 1-8), then use them (Tasks 9-15)
3. **Frequent commits**: After each passing test (16 commits total)
4. **Run tests often**: After each refactoring to catch regressions early
5. **Keep PRs focused**: Each task is atomic and can be reviewed independently

**Risk Mitigation:**
- All existing tests must pass after each task
- New helpers have comprehensive test coverage
- Refactorings are mechanical (no logic changes)
- Each commit can be reviewed in isolation

**Estimated Timeline:**
- Phase 1 (Foundation): 2-3 hours (Tasks 1-3)
- Phase 2 (Container Ops): 1-2 hours (Tasks 4-5)
- Phase 3 (ACP & Launcher): 2-3 hours (Tasks 6-7)
- Phase 4 (Cleanup Chain): 1 hour (Task 8)
- Phase 5 (Decomposition): 3-4 hours (Tasks 9-15)
- Final Verification: 30 minutes (Task 16)
- **Total: 10-14 hours** (1.5-2 days)
