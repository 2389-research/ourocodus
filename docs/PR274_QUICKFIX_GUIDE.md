# PR #274 Quick Fix Guide - Start Here

## Current Status: 4 FAILING GHA CHECKS ❌

```
❌ Build and Test   - undefined functions, build fails
❌ Lint             - same build errors block linting
❌ Format Check     - gofmt issues
❌ Relay Smoke Test - build errors prevent smoke test
✅ Shellcheck       - PASSING
```

**Root Cause:** All 4 failures stem from **3 critical build errors** + **1 formatting issue**.

---

## IMMEDIATE ACTION REQUIRED

Fix these 4 issues IN ORDER to unblock all workflows:

### 1. Fix gofmt (1 minute) ⚡

```bash
gofmt -w pkg/relay/ratelimit/limiter.go
git add pkg/relay/ratelimit/limiter.go
git commit -m "fix: apply gofmt to rate limiter"
```

**Impact:** Unblocks Format Check workflow ✅

---

### 2. Implement `findAgentContainerID` (10 minutes) 🔧

**Problem:** Called in `pkg/relay/session/models.go:333` but doesn't exist in production code.

**Solution:** Use Phase 3 labels package we just merged!

Create `pkg/relay/session/docker_discovery.go`:

```go
package session

import (
	"context"
	"fmt"

	"github.com/2389-research/ourocodus/pkg/labels"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// findAgentContainerID discovers a CLI-spawned agent container by its agent ID.
// Returns containerID, workspace path, and error.
func findAgentContainerID(ctx context.Context, agentID string) (string, string, error) {
	// Validate agent ID against path traversal
	if err := validateAgentID(agentID); err != nil {
		return "", "", fmt.Errorf("invalid agent ID: %w", err)
	}

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	// Use Phase 3 label package to find agent
	filters := labels.FindAgentFilter(agentID)

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     false,
		Filters: filters,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return "", "", fmt.Errorf("no running container found for agent ID: %s", agentID)
	}

	// Extract container ID and workspace
	ctr := containers[0]
	containerID := ctr.ID

	// Get workspace from labels (Phase 3 pattern)
	workspace := ctr.Labels[labels.Workspace]
	if workspace == "" {
		// Fallback: check mounts
		for _, mnt := range ctr.Mounts {
			if mnt.Destination == "/workspace" {
				workspace = mnt.Source
				break
			}
		}
	}

	if workspace == "" {
		return "", "", fmt.Errorf("workspace not found for container %s", containerID[:12])
	}

	return containerID, workspace, nil
}
```

```bash
# Create the file
cat > pkg/relay/session/docker_discovery.go <<'EOF'
[paste code above]
EOF

git add pkg/relay/session/docker_discovery.go
```

**Impact:** Unblocks Build and Test, Lint, and Relay Smoke Test ✅✅✅

---

### 3. Fix unused import (1 minute) 🗑️

**Problem:** `pkg/relay/session/acp_bridge.go` imports `pkg/acp` but doesn't use it.

**Check the file first:**
```bash
grep -n "\"github.com/2389-research/ourocodus/pkg/acp\"" pkg/relay/session/acp_bridge.go
```

**If found, remove it:**
```bash
# Edit pkg/relay/session/acp_bridge.go and remove the unused import line
```

**Or if ACP types ARE used, verify they're properly referenced in the code.**

```bash
git add pkg/relay/session/acp_bridge.go
```

**Impact:** Cleans up lint errors ✅

---

### 4. Fix `NewACPBridge` undefined (5 minutes) 🔍

**Problem:** Called in `pkg/relay/session/models.go:348` but showing as undefined.

**Check if function exists:**
```bash
grep -n "func NewACPBridge" pkg/relay/session/acp_bridge.go
```

**Likely causes:**
1. Function is not exported (lowercase name)
2. Wrong package import
3. Function doesn't exist

**If function exists but lowercase:**
```bash
# Ensure it's exported: func NewACPBridge (capital N)
```

**If function doesn't exist, check Phase 3 merge:**
```bash
git log --oneline --grep="Phase 3" -n 5
# NewACPBridge should have been added in Phase 3
```

**Impact:** Final build fix ✅

---

## Commit and Push

After all 4 fixes:

```bash
git commit -m "fix: resolve build errors and formatting issues

- Apply gofmt to rate limiter
- Implement findAgentContainerID using Phase 3 labels package
- Remove unused pkg/acp import (or fix usage)
- Fix NewACPBridge export/import issue

Fixes all 4 failing GHA workflows (Build, Lint, Format, Smoke Test)"

git push origin feat/phase-4-security-hardening
```

---

## After Build Passes: Security Fixes Required

Once builds pass, **IMMEDIATELY** address these security issues:

### 🔴 CRITICAL SECURITY: Path Traversal (3 files)

Add `validateAgentID(agentID)` to prevent `../../../etc/passwd` attacks:

1. **`pkg/relay/session/token.go:43`** - verifyAttachToken
2. **`cmd/agentd/cmd_spawn.go:221`** - generateAttachToken
3. **`pkg/relay/session/token_test_helper.go:32`** - generateTestAttachToken

```go
func verifyAttachToken(agentID string, token string) error {
	// ADD THIS LINE
	if err := validateAgentID(agentID); err != nil {
		return err
	}

	tokenPath := filepath.Join(".agentd", "session", agentID+".token")
	// ... rest of function
}
```

### 🔴 CRITICAL: Race Condition Panic

**File:** `pkg/relay/session/acp_bridge.go`

**Fix:** Move channel close to readLoop:

```diff
func (b *ACPBridge) readLoop() {
+	defer close(b.notifCh)  // Only readLoop closes channel

	for {
		// ... existing code ...
	}
}

func (b *ACPBridge) Close(ctx context.Context) error {
	// ... existing code ...

-	// Close notifications channel
-	close(b.notifCh)  // REMOVE THIS LINE

	return nil
}
```

---

## Verification Commands

After each commit:

```bash
# Local verification
go build ./...                    # Should complete without errors
go test ./pkg/relay/session/...  # Should pass
gofmt -l .                        # Should show no files (or only non-Go files)
golangci-lint run --timeout=5m   # Should pass (may take time)

# Check GHA status
gh run watch                      # Watch latest workflow run
gh run list --branch feat/phase-4-security-hardening --limit 5
```

---

## Full Triage Document

For complete review comments, security analysis, and P1-P3 issues:
→ See `docs/PR274_REVIEW_TRIAGE.md`

---

**Priority:** Get builds passing FIRST, then address security issues IMMEDIATELY after.

Last Updated: 2025-11-23T01:17:00Z
