# PR #276 Quick Fix Guide

**Issue**: Idempotent container not-found handling (#6, #7 from triage)

## Problem

When a CLI-spawned agent container is already stopped/removed:
1. `FindAgentContainerIDForTesting()` returns "no running container found" error
2. `StopCLISpawnedContainer()` treats this as a failure (returns `true`)
3. Lease and token files are not cleaned up
4. `TerminateAgent()` counts this as a failed termination

**Expected**: Already-stopped containers should be treated as idempotent success.

---

## Fix #1: helpers.go - StopCLISpawnedContainer()

**File**: `pkg/relay/session/helpers.go`
**Function**: `StopCLISpawnedContainer()`
**Lines**: ~136-141

### Current Code
```go
containerID, workspacePath, err := FindAgentContainerIDForTesting(ctx, agentID)
if err != nil {
    logger.Printf("WARN: Failed to find CLI agent container %s: %v", agentID, err)
    return true  // Treats "not found" as failure
}

if containerID == "" {
    // Container not found - may have already been stopped
    logger.Printf("CLI agent container %s not found (may be already stopped)", agentID)
    // Continue with cleanup of other resources even if container is gone
} else {
    // Stop and remove container...
}
```

### Fixed Code
```go
containerID, workspacePath, err := FindAgentContainerIDForTesting(ctx, agentID)
if err != nil {
    // Treat "no running container found" as idempotent success (already stopped)
    if strings.Contains(err.Error(), "no running container found for agent ID") {
        logger.Printf("CLI agent container %s not found (may be already stopped)", agentID)
        // Container is gone, but still clean up leases/tokens
        failed := false

        // Clean up worktree if workspace path is known
        if workspacePath != "" {
            if err := cleanupWorktree(ctx, workspacePath, logger); err != nil {
                logger.Printf("WARN: Failed to cleanup worktree for agent %s: %v", agentID, err)
                // Don't mark as failed - worktree cleanup is best-effort
            }
        }

        // Release lease file
        if err := ReleaseLease(agentID); err != nil {
            logger.Printf("WARN: Failed to release lease for agent %s: %v", agentID, err)
            // Don't mark as failed - lease cleanup is best-effort
        } else {
            logger.Printf("Released lease for agent %s", agentID)
        }

        // Delete attach token file
        if err := deleteAttachToken(agentID); err != nil {
            logger.Printf("WARN: Failed to delete attach token for agent %s: %v", agentID, err)
            // Don't mark as failed - token cleanup is best-effort
        } else {
            logger.Printf("Deleted attach token for agent %s", agentID)
        }

        return failed
    }
    // Real lookup error (not "not found")
    logger.Printf("WARN: Failed to find CLI agent container %s: %v", agentID, err)
    return true
}

if containerID == "" {
    // Defensive: This should be unreachable now, but keep for safety
    logger.Printf("CLI agent container %s not found (may be already stopped)", agentID)
    return false
}

// Normal path: container found, stop and remove it
// ... existing code ...
```

---

## Fix #2: cmd_stop.go - Agent Stop Command

**File**: `cmd/agentd/cmd_stop.go`
**Function**: `runAgentStop()`
**Lines**: ~20-79

### Current Code (Lines 75-79)
```go
if containerID == "" {
    // Agent doesn't exist - this is okay (idempotent)
    printSuccess(fmt.Sprintf("Agent '%s' not found (already stopped)", agentID))
    return nil  // Returns early without cleanup
}
```

### Fixed Code
```go
if containerID == "" {
    // Agent doesn't exist - this is okay (idempotent)
    // But still clean up leases/tokens in case they're stale
    printSuccess(fmt.Sprintf("Agent '%s' not found (already stopped)", agentID))

    // Clean up worktree if workspace path is known
    if workspacePath != "" {
        if err := cleanupWorktree(workspacePath); err != nil {
            fmt.Fprintf(os.Stderr, "Warning: failed to cleanup worktree: %v\n", err)
        } else {
            printSuccess("Cleaned up worktree")
        }
    }

    // Attempt lease release (idempotent)
    if err := session.ReleaseLease(agentID); err != nil {
        fmt.Fprintf(os.Stderr, "Warning: failed to release lease: %v\n", err)
    } else {
        printSuccess("Released agent lease")
    }

    // Attempt token deletion (idempotent)
    if err := deleteAttachToken(agentID); err != nil {
        fmt.Fprintf(os.Stderr, "Warning: failed to delete attach token: %v\n", err)
    } else {
        printSuccess("Deleted attach token")
    }

    printSuccess("Cleaned up agent resources")
    return nil
}
```

---

## Testing

### Test Case 1: Stop Already-Stopped Agent
```bash
# Start an agent
agentd spawn --name test-agent

# Stop it
agentd stop test-agent

# Stop it again (should be idempotent)
agentd stop test-agent

# Expected:
# - No errors
# - Success message
# - Lease and token files cleaned up
```

### Test Case 2: Verify Cleanup
```bash
# Check lease file
ls -la .agentd/session/*.lease

# Check token file
ls -la .agentd/session/*.token

# Both should be gone after stop
```

### Test Case 3: Manual Container Cleanup
```bash
# Start agent
agentd spawn --name test-agent

# Manually remove container
docker stop <container-id>
docker rm <container-id>

# Now run stop (should cleanup leases/tokens)
agentd stop test-agent

# Verify cleanup
ls -la .agentd/session/
```

---

## Implementation Checklist

- [ ] Apply Fix #1 to `pkg/relay/session/helpers.go`
- [ ] Apply Fix #2 to `cmd/agentd/cmd_stop.go`
- [ ] Run `make fmt` to format code
- [ ] Run `make lint` to verify no new warnings
- [ ] Run `make test` to verify tests pass
- [ ] Test manual scenarios (see Testing section)
- [ ] Commit with descriptive message
- [ ] Push to PR branch

---

## Commit Message Template

```
fix: handle already-stopped containers idempotently

When FindAgentContainerIDForTesting returns "no running container
found", treat this as idempotent success instead of failure. This
ensures lease and token cleanup happens even when the container
is already gone.

Fixes:
- StopCLISpawnedContainer now cleans up leases/tokens even when
  container is not found
- agentd stop command runs cleanup when container is already stopped
- TerminateAgent no longer counts "already stopped" as failure

Addresses CodeRabbit review feedback on PR #276 (issues #6, #7).

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

---

## Notes

- Both fixes follow the same pattern: detect "not found", clean up anyway
- Cleanup operations (lease, token, worktree) are all best-effort
- Warnings are logged but don't fail the operation
- This maintains idempotent contract for CLI commands
- Aligns with existing Docker error handling (IsNotFound = success)
