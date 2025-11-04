# Implementation Plan: Phase 2 - Container Reuse & Attach (Issue #102)

**Date:** 2025-11-04
**Branch:** `feature/container-reuse-attach-102`
**Milestone:** Milestone 1 - Container Session Foundation
**Dependencies:** Issue #101 (Phase 1) - COMPLETED

## Overview

This phase adds intelligent container reuse and session attachment capabilities to the containersession package, allowing sessions to reconnect to existing containers instead of always creating new ones.

```
Current Flow:                    Enhanced Flow:
CreateSession()                  CreateSession()
    |                                |
    v                                v
Create New Container            Check for Existing Container
    |                                |
    v                         +------+------+
Start Container              |             |
                          Exists        Not Found
                             |             |
                        +----+----+        |
                        |         |        |
                    Running   Stopped      |
                        |         |        |
                    Reattach   Start    Create New
```

## Part 1: findContainer() Implementation

### Purpose
Private helper method for label-based container discovery using the Docker API.

### Location
`pkg/containersession/manager.go`

### Implementation Tasks

**Task 1.1: Add findContainer() method signature**
```go
// findContainer searches for an existing container by session ID
// Returns: containerID (string), state (string), error
func (m *Manager) findContainer(ctx context.Context, sessionID string) (string, string, error)
```

**Task 1.2: Implement label-based filtering**
- Use `m.client.ContainerList()` with filters
- Filter by label: `session-id=<sessionID>`
- Filter by label: `managed-by=<m.config.ManagedBy>`
- Handle empty results (no containers found)

**Task 1.3: Handle edge cases**
- Multiple containers with same session ID (return first, log warning)
- Container exists but is in "created" state (treat as stopped)
- Container exists but is "dead" or "exited" (return with state)
- Container is being removed (return not found)

**Task 1.4: Return container state**
- Extract container.State from inspection
- Return one of: "running", "stopped", "created", "paused", "dead"

**Acceptance Criteria:**
- [ ] Method correctly finds containers by session ID
- [ ] Returns accurate container state
- [ ] Handles no-results gracefully
- [ ] Logs warning for multiple matches
- [ ] Returns appropriate errors for API failures

---

## Part 2: CreateSession() Reuse Logic

### Purpose
Modify existing CreateSession() to check for and reuse existing containers before creating new ones.

### Location
`pkg/containersession/manager.go`

### Implementation Tasks

**Task 2.1: Add reuse check at start of CreateSession()**
```go
// Check for existing container
existingID, state, err := m.findContainer(ctx, sessionID)
if err != nil {
    return nil, fmt.Errorf("failed to check for existing container: %w", err)
}

if existingID != "" {
    return m.handleExistingContainer(ctx, existingID, state, sessionID, config)
}

// Proceed with normal container creation...
```

**Task 2.2: Implement handleExistingContainer() helper**
- Private method to handle found containers
- Decision tree based on container state:
  - `running`: Attach and return session
  - `stopped/created/exited`: Start container, then attach
  - `paused`: Unpause, then attach
  - `dead`: Remove old container, create new one

**Task 2.3: Extract workspace path from existing container**
- Inspect container to get volume mounts
- Find mount with container path matching workspace
- Use host path from mount for workspace field
- Validate workspace path exists and is accessible

**Task 2.4: Implement stream attachment logic**
- Reuse existing handleContainerOutput() method
- Ensure proper demuxing of stdout/stderr
- Handle case where streams might already be attached

**Task 2.5: Update session state tracking**
- Load/create ContainerSession with existing container ID
- Set state to Running if reattaching
- Preserve original creation timestamp from labels

**Acceptance Criteria:**
- [ ] CreateSession() checks for existing containers first
- [ ] Running containers are reattached without restart
- [ ] Stopped containers are started before reattachment
- [ ] Workspace path is correctly resolved from existing container
- [ ] I/O streams work correctly after reattachment
- [ ] Session state reflects actual container state

---

## Part 3: AttachSession() Implementation

### Purpose
Public API method to explicitly reconnect to an existing session by session ID.

### Location
`pkg/containersession/manager.go`

### Implementation Tasks

**Task 3.1: Add AttachSession() method signature**
```go
// AttachSession reconnects to an existing container session
// Returns error if session doesn't exist or container is not running
func (m *Manager) AttachSession(ctx context.Context, sessionID string) (*ContainerSession, error)
```

**Task 3.2: Implement session lookup**
- Call findContainer() to locate container
- Return descriptive error if not found
- Return error if container is not in "running" state

**Task 3.3: Implement container inspection**
- Use `m.client.ContainerInspect()` to get full details
- Extract workspace path from volume mounts
- Extract labels for session metadata

**Task 3.4: Implement I/O attachment**
- Use `m.client.ContainerAttach()` with:
  - Stream: true
  - Stdin: true (if config allows)
  - Stdout: true
  - Stderr: true
- Call handleContainerOutput() for stream management

**Task 3.5: Build and return ContainerSession**
- Create ContainerSession with:
  - ID: sessionID
  - ContainerID: found container ID
  - WorkspacePath: extracted from mounts
  - Labels: from container labels
  - State: Running
- Return fully initialized session

**Acceptance Criteria:**
- [ ] Method successfully attaches to running containers
- [ ] Returns clear error for non-existent sessions
- [ ] Returns clear error for stopped/dead containers
- [ ] I/O streams are properly connected and demuxed
- [ ] Workspace path is correctly extracted
- [ ] Session metadata matches container labels

---

## Part 4: Testing Strategy

### Unit Tests (manager_test.go)

**Test Suite 4.1: findContainer() Tests**
```
Test_findContainer_Success
Test_findContainer_NotFound
Test_findContainer_MultipleContainers
Test_findContainer_ContainerStates (running, stopped, created, dead)
Test_findContainer_DockerAPIError
```

**Test Suite 4.2: CreateSession() Reuse Tests**
```
Test_CreateSession_NoExistingContainer (baseline)
Test_CreateSession_ReuseRunningContainer
Test_CreateSession_ReuseStoppedContainer
Test_CreateSession_ReuseCreatedContainer
Test_CreateSession_ReusePausedContainer
Test_CreateSession_ReplaceDeadContainer
Test_CreateSession_WorkspacePathExtraction
```

**Test Suite 4.3: AttachSession() Tests**
```
Test_AttachSession_Success
Test_AttachSession_NotFound
Test_AttachSession_ContainerStopped
Test_AttachSession_ContainerDead
Test_AttachSession_IOStreamsWork
Test_AttachSession_WorkspaceExtraction
Test_AttachSession_DockerAPIError
```

### Integration Tests (integration_test.go)

**Test Suite 4.4: End-to-End Reuse Scenarios**
```go
//go:build integration

TestIntegration_CreateAndReuse
  - Create session with unique ID
  - Stop session
  - Call CreateSession with same ID
  - Verify container is started (not recreated)
  - Verify workspace is same

TestIntegration_AttachToRunning
  - Create and start session
  - Call AttachSession from different Manager instance
  - Verify can read/write to container
  - Verify workspace path matches

TestIntegration_ConcurrentAttach
  - Create session
  - Attach from multiple goroutines
  - Verify all attachments work
  - Verify no race conditions
```

### Testing Tools & Mocks

**Mock Docker Client:**
- Extend existing mocks in manager_test.go
- Add mock responses for:
  - ContainerList with filters
  - ContainerInspect results
  - ContainerAttach responses
  - Container state variations

**Test Fixtures:**
- Sample container inspection results
- Sample label sets for filtering
- Sample volume mount configurations

**Acceptance Criteria:**
- [ ] All unit tests pass with mocked Docker client
- [ ] Code coverage remains above 80%
- [ ] Integration tests pass with real Docker daemon
- [ ] No race conditions detected by `go test -race`

---

## Part 5: Documentation

### Code Documentation

**Task 5.1: Update doc.go**
- Add section on "Container Reuse Behavior"
- Explain when containers are reused vs created
- Document AttachSession() usage patterns
- Provide code examples for both scenarios

**Task 5.2: Add method documentation**
- Add comprehensive godoc comments to:
  - findContainer() (explain label filtering)
  - CreateSession() (explain reuse logic)
  - AttachSession() (explain requirements and errors)
  - handleExistingContainer() (explain decision tree)

**Task 5.3: Add inline comments**
- Comment the reuse decision tree in CreateSession()
- Comment workspace extraction logic
- Comment edge case handling

### Usage Examples

**Task 5.4: Add examples to doc.go**

Example 1: Automatic Reuse
```go
// First call creates container
session1, _ := manager.CreateSession(ctx, config)
session1.Stop()

// Second call with same session ID reuses container
config2 := config
config2.SessionID = session1.ID()
session2, _ := manager.CreateSession(ctx, config2)
// session2 uses same container, just restarted
```

Example 2: Explicit Attach
```go
// From one process
session, _ := manager.CreateSession(ctx, config)
sessionID := session.ID()

// From another process (or after restart)
manager2, _ := NewManager(...)
reattached, _ := manager2.AttachSession(ctx, sessionID)
// reattached is connected to same running container
```

### Edge Cases & Limitations

**Task 5.5: Document known limitations**
- Container must have matching session-id label
- Cannot attach to containers in "removing" state
- Workspace path must match between calls
- Concurrent CreateSession calls with same ID may race

**Acceptance Criteria:**
- [ ] doc.go includes comprehensive reuse documentation
- [ ] All public methods have godoc comments
- [ ] Code examples are runnable and accurate
- [ ] Edge cases and limitations are documented
- [ ] Troubleshooting guidance provided

---

## Implementation Sequence

### Phase A: Core Reuse Infrastructure
1. Implement findContainer() helper
2. Add unit tests for findContainer()
3. Code review and refinement

### Phase B: CreateSession Enhancement
4. Add reuse check to CreateSession()
5. Implement handleExistingContainer() helper
6. Add unit tests for reuse scenarios
7. Integration tests for CreateSession reuse

### Phase C: AttachSession Implementation
8. Implement AttachSession() method
9. Add unit tests for AttachSession()
10. Integration tests for AttachSession()

### Phase D: Documentation & Polish
11. Update doc.go with reuse behavior
12. Add method documentation
13. Add usage examples
14. Final testing and validation

```
Dependency Flow:
Phase A (findContainer)
    |
    v
Phase B (CreateSession reuse)
    |
    v
Phase C (AttachSession)
    |
    v
Phase D (Documentation)
```

---

## Success Metrics

### Functional Requirements
- [ ] CreateSession() successfully reuses running containers
- [ ] CreateSession() restarts stopped containers
- [ ] AttachSession() connects to existing sessions
- [ ] Workspace paths are correctly preserved
- [ ] I/O streams work after reattachment

### Quality Requirements
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Code coverage >= 80%
- [ ] No race conditions detected
- [ ] golangci-lint passes
- [ ] staticcheck passes

### Documentation Requirements
- [ ] Public APIs fully documented
- [ ] Usage examples provided
- [ ] Edge cases documented
- [ ] Troubleshooting guide complete

---

## Risk Mitigation

### Risk 1: Container State Race Conditions
**Mitigation:** Add retries and state validation in findContainer()

### Risk 2: Workspace Path Mismatch
**Mitigation:** Strict validation and clear error messages

### Risk 3: Stream Attachment Failures
**Mitigation:** Comprehensive error handling and fallback to recreation

---

## Notes

- No backward compatibility required - this is new functionality
- Phase 1 API can be modified as needed for better reuse patterns
- Focus on correctness and clarity over preserving old behavior
