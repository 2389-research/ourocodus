# Claude Code Container Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the echo-agent placeholder with official claude-code-acp running in Docker containers with proper credential handling and runtime hardening.

**Architecture:** Two-layer Docker image (base + agent), entry script with credential sourcing, PID-file health check, runtime hardening flags. Integrates with existing `AgentContainerLauncher` and credential infrastructure.

**Tech Stack:** Docker, Node.js 22, tini, @zed-industries/claude-code-acp@0.10.10, Go container package

**Design Document:** `docs/plans/2025-12-02-claude-code-container-design.md`

---

## Task 1: Create Base Dockerfile

**Files:**
- Create: `Dockerfile.claude-code-base`

**Step 1: Create the base Dockerfile**

```dockerfile
# Base image for Claude Code agents
# Provides Node.js runtime, init process, and common tooling
FROM node:22-bookworm-slim

# Install tini for proper PID 1 behavior
# tini handles signal forwarding and zombie reaping
RUN apt-get update && apt-get install -y --no-install-recommends \
    tini \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root agent user
RUN useradd -m -u 1000 -s /bin/bash agent

# Set HOME explicitly for consistent credential resolution
ENV HOME=/home/agent

# Create standard directories
RUN mkdir -p /workspace /home/agent/.creds /home/agent/.claude \
    && chown -R agent:agent /workspace /home/agent

# Set working directory
WORKDIR /workspace

# Use tini as init
ENTRYPOINT ["/usr/bin/tini", "--"]
```

**Step 2: Build and verify base image**

Run: `docker build -f Dockerfile.claude-code-base -t ourocodus/claude-code-base:latest .`
Expected: Build succeeds with no errors

**Step 3: Verify tini is installed**

Run: `docker run --rm ourocodus/claude-code-base:latest /usr/bin/tini --version`
Expected: Output shows tini version

**Step 4: Commit**

```bash
git add Dockerfile.claude-code-base
git commit -m "feat(container): add claude-code base image with Node.js and tini"
```

---

## Task 2: Create Entry Script

**Files:**
- Create: `scripts/claude-code-entry.sh`

**Step 1: Create the entry script**

```bash
#!/bin/bash
set -e

# Credential sourcing with fallback
# Priority: 1) .creds/.env file  2) ~/.claude directory  3) Error
#
# SECURITY NOTE: Sourcing .env exposes API key to process environment.
# This is visible via /proc/$pid/environ to anyone with container exec access.
# Mitigations: read-only rootfs, drop capabilities, no-new-privileges.

if [ -f "/home/agent/.creds/.env" ]; then
    echo "[claude-code] Sourcing credentials from .creds/.env" >&2
    # Validate .env format before sourcing (basic safety check)
    if grep -qE '^[A-Z_][A-Z0-9_]*=' /home/agent/.creds/.env 2>/dev/null; then
        set -a
        source /home/agent/.creds/.env
        set +a
    else
        echo "[claude-code] ERROR: Invalid .env format" >&2
        exit 1
    fi
elif [ -f "/home/agent/.claude/.credentials.json" ]; then
    echo "[claude-code] Using existing Claude credentials from ~/.claude" >&2
    # claude-code-acp will read from standard location
else
    echo "[claude-code] ERROR: No credentials found" >&2
    echo "[claude-code] Provide ANTHROPIC_API_KEY via .creds/.env or mount ~/.claude" >&2
    exit 1
fi

# Verify API key is available (unless using OAuth from ~/.claude)
if [ -z "$ANTHROPIC_API_KEY" ] && [ ! -f "/home/agent/.claude/.credentials.json" ]; then
    echo "[claude-code] ERROR: ANTHROPIC_API_KEY not set and no Claude credentials found" >&2
    exit 1
fi

# Write PID file for health check (before exec replaces this process)
# The node process will inherit this PID
echo $$ > /tmp/claude-code.pid

# Start claude-code-acp
# stdin/stdout are used for ACP protocol communication
exec claude-code-acp --workspace /workspace "$@"
```

**Step 2: Make script executable and verify syntax**

Run: `chmod +x scripts/claude-code-entry.sh && bash -n scripts/claude-code-entry.sh`
Expected: No output (syntax OK)

**Step 3: Commit**

```bash
git add scripts/claude-code-entry.sh
git commit -m "feat(container): add claude-code entry script with credential handling"
```

---

## Task 3: Update Dockerfile.agent

**Files:**
- Modify: `Dockerfile.agent` (replace entire contents)

**Step 1: Read current Dockerfile.agent for context**

The current file uses echo-agent as a placeholder. We'll replace it entirely.

**Step 2: Replace Dockerfile.agent with new content**

```dockerfile
# Claude Code ACP Agent
# Runs official claude-code-acp with proper init and credential handling
#
# Build: docker build -f Dockerfile.agent -t ourocodus/agent:latest .
# Requires: Dockerfile.claude-code-base built first as ourocodus/claude-code-base:latest
#
# Runtime hardening (recommended):
#   docker run --read-only --cap-drop=ALL --security-opt=no-new-privileges \
#     --tmpfs /tmp:noexec,nosuid,size=100m --memory=2g --cpus=2 \
#     ourocodus/agent:latest

FROM ourocodus/claude-code-base:latest

# Install claude-code-acp globally
# Pin to specific version for reproducibility
RUN npm install -g @zed-industries/claude-code-acp@0.10.10

# Copy entry script
COPY scripts/claude-code-entry.sh /usr/local/bin/claude-code-entry.sh
RUN chmod +x /usr/local/bin/claude-code-entry.sh

# Health check: verify process is running via PID file
# Note: pgrep is fragile; PID file check is more reliable
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD test -f /tmp/claude-code.pid && kill -0 $(cat /tmp/claude-code.pid) 2>/dev/null || exit 1

# Switch to non-root user
USER agent

# Entry script handles credential sourcing and starts claude-code-acp
CMD ["/usr/local/bin/claude-code-entry.sh"]
```

**Step 3: Build agent image (requires base image from Task 1)**

Run: `docker build -f Dockerfile.agent -t ourocodus/agent:latest .`
Expected: Build succeeds, npm installs claude-code-acp

**Step 4: Verify claude-code-acp is installed**

Run: `docker run --rm ourocodus/agent:latest which claude-code-acp`
Expected: `/usr/local/bin/claude-code-acp` or similar path

**Step 5: Commit**

```bash
git add Dockerfile.agent
git commit -m "feat(container): update agent image to use claude-code-acp"
```

---

## Task 4: Add Runtime Hardening to SpawnConfig

**Files:**
- Modify: `pkg/agent/container/types.go:10-39`
- Test: `pkg/agent/container/types_test.go`

**Step 1: Write failing test for new SpawnConfig fields**

Add to `pkg/agent/container/types_test.go`:

```go
func TestSpawnConfig_RuntimeHardening(t *testing.T) {
	config := SpawnConfig{
		AgentID:   "test-agent",
		ImageName: "ourocodus/agent:latest",
		Command:   []string{"/usr/local/bin/claude-code-entry.sh"},
		RuntimeHardening: RuntimeHardening{
			ReadOnlyRootfs:   true,
			DropAllCaps:      true,
			NoNewPrivileges:  true,
			MemoryLimitMB:    2048,
			CPULimit:         2.0,
			TmpfsSizeMB:      100,
		},
	}

	if !config.RuntimeHardening.ReadOnlyRootfs {
		t.Error("expected ReadOnlyRootfs to be true")
	}
	if config.RuntimeHardening.MemoryLimitMB != 2048 {
		t.Errorf("expected MemoryLimitMB=2048, got %d", config.RuntimeHardening.MemoryLimitMB)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/agent/container/... -run TestSpawnConfig_RuntimeHardening -v`
Expected: FAIL - `RuntimeHardening` field undefined

**Step 3: Add RuntimeHardening struct to types.go**

Add after `SpawnConfig` struct (around line 40):

```go
// RuntimeHardening contains security hardening options for containers.
// These settings reduce attack surface and limit container capabilities.
type RuntimeHardening struct {
	// ReadOnlyRootfs makes the container's root filesystem read-only
	ReadOnlyRootfs bool

	// DropAllCaps drops all Linux capabilities
	DropAllCaps bool

	// NoNewPrivileges prevents privilege escalation
	NoNewPrivileges bool

	// MemoryLimitMB sets memory limit in megabytes (0 = no limit)
	MemoryLimitMB int64

	// CPULimit sets CPU limit as number of cores (0 = no limit)
	CPULimit float64

	// TmpfsSizeMB sets tmpfs size for /tmp in megabytes (0 = no tmpfs)
	TmpfsSizeMB int64
}
```

**Step 4: Add RuntimeHardening field to SpawnConfig**

Add to `SpawnConfig` struct (around line 37, before closing brace):

```go
	// RuntimeHardening contains security hardening options (optional)
	// If not set, containers run with default (less secure) settings
	RuntimeHardening RuntimeHardening
```

**Step 5: Run test to verify it passes**

Run: `go test ./pkg/agent/container/... -run TestSpawnConfig_RuntimeHardening -v`
Expected: PASS

**Step 6: Commit**

```bash
git add pkg/agent/container/types.go pkg/agent/container/types_test.go
git commit -m "feat(container): add RuntimeHardening to SpawnConfig"
```

---

## Task 5: Apply Runtime Hardening in Container Creation

**Files:**
- Modify: `pkg/containersession/manager.go` (CreateConfig struct)
- Modify: `pkg/containersession/manager.go` (container creation logic)
- Test: `pkg/containersession/manager_test.go`

**Step 1: Check current CreateConfig structure**

Read `pkg/containersession/manager.go` to find `CreateConfig` struct.

**Step 2: Write failing test for hardening options**

Add to `pkg/containersession/manager_test.go`:

```go
func TestCreateConfig_RuntimeHardening(t *testing.T) {
	config := CreateConfig{
		ImageName: "test:latest",
		Command:   []string{"echo", "test"},
		RuntimeHardening: RuntimeHardening{
			ReadOnlyRootfs:  true,
			DropAllCaps:     true,
			NoNewPrivileges: true,
			MemoryLimitMB:   2048,
			CPULimit:        2.0,
			TmpfsSizeMB:     100,
		},
	}

	if !config.RuntimeHardening.ReadOnlyRootfs {
		t.Error("expected ReadOnlyRootfs to be true")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./pkg/containersession/... -run TestCreateConfig_RuntimeHardening -v`
Expected: FAIL - `RuntimeHardening` field undefined in CreateConfig

**Step 4: Add RuntimeHardening to CreateConfig**

In `pkg/containersession/manager.go`, add to `CreateConfig` struct:

```go
	// RuntimeHardening contains security options for the container
	RuntimeHardening RuntimeHardening
```

Also add the `RuntimeHardening` type if not already imported:

```go
// RuntimeHardening contains security hardening options for containers.
type RuntimeHardening struct {
	ReadOnlyRootfs  bool
	DropAllCaps     bool
	NoNewPrivileges bool
	MemoryLimitMB   int64
	CPULimit        float64
	TmpfsSizeMB     int64
}
```

**Step 5: Apply hardening in container creation**

Find the `CreateContainerSessionWithConfig` method and update the `container.Config` and `container.HostConfig`:

```go
// In CreateContainerSessionWithConfig, update hostConfig:
hostConfig := &container.HostConfig{
	// ... existing config ...
}

// Apply runtime hardening if specified
if config.RuntimeHardening.ReadOnlyRootfs {
	hostConfig.ReadonlyRootfs = true
}
if config.RuntimeHardening.DropAllCaps {
	hostConfig.CapDrop = []string{"ALL"}
}
if config.RuntimeHardening.NoNewPrivileges {
	hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, "no-new-privileges")
}
if config.RuntimeHardening.MemoryLimitMB > 0 {
	hostConfig.Memory = config.RuntimeHardening.MemoryLimitMB * 1024 * 1024
}
if config.RuntimeHardening.CPULimit > 0 {
	hostConfig.NanoCPUs = int64(config.RuntimeHardening.CPULimit * 1e9)
}
if config.RuntimeHardening.TmpfsSizeMB > 0 {
	hostConfig.Tmpfs = map[string]string{
		"/tmp": fmt.Sprintf("size=%dm,noexec,nosuid", config.RuntimeHardening.TmpfsSizeMB),
	}
}
```

**Step 6: Run tests**

Run: `go test ./pkg/containersession/... -v`
Expected: All tests pass

**Step 7: Commit**

```bash
git add pkg/containersession/manager.go pkg/containersession/manager_test.go
git commit -m "feat(containersession): implement RuntimeHardening in container creation"
```

---

## Task 6: Update AgentContainerLauncher to Pass Hardening

**Files:**
- Modify: `pkg/agent/container/launcher.go:198-254`
- Test: `pkg/agent/container/launcher_test.go` (if exists) or `integration_test.go`

**Step 1: Update createContainerWithMounts to pass RuntimeHardening**

In `pkg/agent/container/launcher.go`, update `createContainerWithMounts`:

```go
func (l *AgentContainerLauncher) createContainerWithMounts(
	ctx context.Context,
	config SpawnConfig,
	wt *worktree.AgentWorktree,
	credFiles *CredentialFiles,
) (*containersession.ContainerSession, error) {
	// ... existing mount setup code ...

	// Create container session with custom configuration
	sess, err := l.containerMgr.CreateContainerSessionWithConfig(ctx, containersession.CreateConfig{
		ImageName:         config.ImageName,
		Command:           config.Command,
		Entrypoint:        config.Entrypoint,
		WorkspaceDir:      wt.Path(),
		CustomMounts:      credMounts,
		Env:               config.Env,
		Labels:            containerLabels,
		SkipOutputLogging: skipOutputLogging,
		// Pass runtime hardening options
		RuntimeHardening: containersession.RuntimeHardening{
			ReadOnlyRootfs:  config.RuntimeHardening.ReadOnlyRootfs,
			DropAllCaps:     config.RuntimeHardening.DropAllCaps,
			NoNewPrivileges: config.RuntimeHardening.NoNewPrivileges,
			MemoryLimitMB:   config.RuntimeHardening.MemoryLimitMB,
			CPULimit:        config.RuntimeHardening.CPULimit,
			TmpfsSizeMB:     config.RuntimeHardening.TmpfsSizeMB,
		},
	})
	if err != nil {
		return nil, err
	}

	return sess, nil
}
```

**Step 2: Run existing tests to ensure no regression**

Run: `go test ./pkg/agent/container/... -v`
Expected: All existing tests pass

**Step 3: Run integration tests**

Run: `go test ./pkg/agent/container/... -run Integration -v`
Expected: Integration tests pass (or skip if no Docker)

**Step 4: Commit**

```bash
git add pkg/agent/container/launcher.go
git commit -m "feat(container): pass RuntimeHardening to containersession"
```

---

## Task 7: Add Default Hardening for Claude Code Agent

**Files:**
- Modify: `pkg/agent/factory.go`
- Test: `pkg/agent/factory_test.go` (if exists)

**Step 1: Find where SpawnConfig is created in factory.go**

Look for `prepareSpawnConfig` or similar method.

**Step 2: Add default RuntimeHardening for Claude Code agents**

When creating SpawnConfig for Claude Code agents, set recommended defaults:

```go
// In the method that creates SpawnConfig for Claude Code:
spawnConfig := container.SpawnConfig{
	AgentID:    agentID,
	ImageName:  "ourocodus/agent:latest",
	Command:    []string{"/usr/local/bin/claude-code-entry.sh"},
	Entrypoint: []string{"/usr/bin/tini", "--"},
	APIKey:     config.AnthropicKey,
	// Apply recommended runtime hardening
	RuntimeHardening: container.RuntimeHardening{
		ReadOnlyRootfs:  true,
		DropAllCaps:     true,
		NoNewPrivileges: true,
		MemoryLimitMB:   2048,  // 2GB memory limit
		CPULimit:        2.0,   // 2 CPU cores
		TmpfsSizeMB:     100,   // 100MB tmpfs for /tmp
	},
}
```

**Step 3: Run tests**

Run: `go test ./pkg/agent/... -v`
Expected: All tests pass

**Step 4: Commit**

```bash
git add pkg/agent/factory.go
git commit -m "feat(agent): add default RuntimeHardening for Claude Code agents"
```

---

## Task 8: Update Credential Path for Non-Root User

**Files:**
- Modify: `pkg/agent/container/launcher.go:207-217` (creds mount path)
- Modify: `pkg/agent/container/credentials.go:123-140` (mount paths)

**Step 1: Update .creds mount target in launcher.go**

Change from `/root/.creds` to `/home/agent/.creds`:

```go
// In createContainerWithMounts, around line 210-217:
if _, err := os.Stat(credsPath); err == nil {
	credMounts = append(credMounts, mount.Mount{
		Type:     mount.TypeBind,
		Source:   credsPath,
		Target:   "/home/agent/.creds",  // Changed from /root/.creds
		ReadOnly: true,
	})
}
```

**Step 2: Update credential mounts in credentials.go**

Change SSH key and GitHub token paths from `/root/` to `/home/agent/`:

```go
// In GetMounts method:
if files.GitSSHKeyPath != "" {
	mounts = append(mounts, mount.Mount{
		Type:     mount.TypeBind,
		Source:   files.GitSSHKeyPath,
		Target:   "/home/agent/.ssh/id_ed25519",  // Changed from /root/.ssh
		ReadOnly: true,
	})
}

if files.GitHubTokenPath != "" {
	mounts = append(mounts, mount.Mount{
		Type:     mount.TypeBind,
		Source:   files.GitHubTokenPath,
		Target:   "/home/agent/.github-token",  // Changed from /root/.github-token
		ReadOnly: true,
	})
}
```

**Step 3: Run tests**

Run: `go test ./pkg/agent/container/... -v`
Expected: All tests pass

**Step 4: Commit**

```bash
git add pkg/agent/container/launcher.go pkg/agent/container/credentials.go
git commit -m "fix(container): update credential mount paths for non-root agent user"
```

---

## Task 9: Add Integration Test for Claude Code Container

**Files:**
- Create: `pkg/agent/container/claude_code_integration_test.go`

**Step 1: Create integration test file**

```go
//go:build integration

package container_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
)

func TestClaudeCodeContainer_Integration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test (set RUN_INTEGRATION_TESTS=true)")
	}

	// Skip if no API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping: ANTHROPIC_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "claude-code-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo in temp dir
	if err := initGitRepo(tmpDir); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create managers
	containerMgr, err := containersession.NewManager(containersession.ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create container manager: %v", err)
	}

	worktreeMgr := worktree.NewAgentWorktreeManager(tmpDir)
	credMounter := container.NewAgentCredentialMounter(filepath.Join(tmpDir, "creds"))

	launcher := container.NewAgentContainerLauncher(
		containerMgr,
		worktreeMgr,
		credMounter,
		filepath.Join(tmpDir, "workspaces"),
	)

	// Spawn Claude Code container with hardening
	handle, err := launcher.Spawn(ctx, container.SpawnConfig{
		AgentID:    "test-claude-code",
		ImageName:  "ourocodus/agent:latest",
		Command:    []string{"/usr/local/bin/claude-code-entry.sh"},
		Entrypoint: []string{"/usr/bin/tini", "--"},
		APIKey:     apiKey,
		RuntimeHardening: container.RuntimeHardening{
			ReadOnlyRootfs:  true,
			DropAllCaps:     true,
			NoNewPrivileges: true,
			MemoryLimitMB:   2048,
			CPULimit:        2.0,
			TmpfsSizeMB:     100,
		},
	})
	if err != nil {
		t.Fatalf("Failed to spawn container: %v", err)
	}
	defer launcher.Stop(context.Background(), "test-claude-code")

	// Verify container is running
	if handle.ContainerID() == "" {
		t.Error("Expected container ID to be set")
	}

	// Wait for health check to pass
	time.Sleep(10 * time.Second)

	t.Logf("Claude Code container running: %s", handle.ContainerID())
}

func initGitRepo(dir string) error {
	// Implementation: run git init in dir
	return nil
}
```

**Step 2: Run integration test (requires Docker and images built)**

Run: `RUN_INTEGRATION_TESTS=true go test ./pkg/agent/container/... -run TestClaudeCodeContainer_Integration -v`
Expected: PASS (or skip if no API key)

**Step 3: Commit**

```bash
git add pkg/agent/container/claude_code_integration_test.go
git commit -m "test(container): add Claude Code container integration test"
```

---

## Task 10: Update Documentation

**Files:**
- Modify: `docs/plans/2025-12-02-claude-code-container-design.md` (mark as implemented)
- Create: `docs/container-troubleshooting.md`

**Step 1: Update design doc status**

Change status from "Approved (with refinements)" to "Implemented".

**Step 2: Create troubleshooting guide**

```markdown
# Container Troubleshooting Guide

## Claude Code Container Issues

### Container fails to start

**Symptom:** Container exits immediately after starting

**Check:**
1. Verify credentials are mounted: `docker inspect <container> | grep Mounts`
2. Check entry script logs: `docker logs <container>`
3. Verify .env format: `cat .creds/.env` should have `ANTHROPIC_API_KEY=sk-...`

### Health check failing

**Symptom:** Container shows "unhealthy" status

**Check:**
1. Verify PID file exists: `docker exec <container> cat /tmp/claude-code.pid`
2. Check if process is running: `docker exec <container> ps aux | grep claude-code`
3. Check logs for errors: `docker logs <container>`

### Permission denied errors

**Symptom:** "Permission denied" when writing to /tmp or /workspace

**Check:**
1. Verify tmpfs is mounted: `docker inspect <container> | grep Tmpfs`
2. Check if running as agent user: `docker exec <container> whoami`
3. Verify workspace permissions: `docker exec <container> ls -la /workspace`

### API key not found

**Symptom:** "ANTHROPIC_API_KEY not set" error

**Check:**
1. Verify .creds/.env exists: `ls -la <workspace>/.creds/`
2. Check mount: `docker inspect <container> | grep -A5 .creds`
3. Verify format: File must have `ANTHROPIC_API_KEY=sk-...` format

## Runtime Hardening

The container runs with these security options by default:
- `--read-only`: Root filesystem is read-only
- `--cap-drop=ALL`: All Linux capabilities dropped
- `--security-opt=no-new-privileges`: Cannot gain privileges
- `--tmpfs /tmp`: Writable tmpfs for temporary files
- `--memory=2g`: 2GB memory limit
- `--cpus=2`: 2 CPU cores limit
```

**Step 3: Commit**

```bash
git add docs/plans/2025-12-02-claude-code-container-design.md docs/container-troubleshooting.md
git commit -m "docs: add container troubleshooting guide, mark design as implemented"
```

---

## Task 11: Final Verification

**Files:** None (verification only)

**Step 1: Build all images**

```bash
docker build -f Dockerfile.claude-code-base -t ourocodus/claude-code-base:latest .
docker build -f Dockerfile.agent -t ourocodus/agent:latest .
```

**Step 2: Run all tests**

```bash
go test ./pkg/agent/... ./pkg/containersession/... -v
```

**Step 3: Run integration test (if API key available)**

```bash
RUN_INTEGRATION_TESTS=true ANTHROPIC_API_KEY=sk-... go test ./pkg/agent/container/... -run Integration -v
```

**Step 4: Verify linting passes**

```bash
make lint
```

**Step 5: Create final commit**

```bash
git add -A
git commit -m "feat(container): complete Claude Code container integration

- Add base image with Node.js 22 and tini
- Update agent image to use claude-code-acp@0.10.10
- Add entry script with credential sourcing and PID file
- Implement RuntimeHardening for security
- Update credential paths for non-root agent user
- Add integration tests and documentation

Closes #XXX"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create base Dockerfile | `Dockerfile.claude-code-base` |
| 2 | Create entry script | `scripts/claude-code-entry.sh` |
| 3 | Update agent Dockerfile | `Dockerfile.agent` |
| 4 | Add RuntimeHardening struct | `pkg/agent/container/types.go` |
| 5 | Implement hardening in containersession | `pkg/containersession/manager.go` |
| 6 | Pass hardening to containersession | `pkg/agent/container/launcher.go` |
| 7 | Add default hardening | `pkg/agent/factory.go` |
| 8 | Update credential paths | `pkg/agent/container/*.go` |
| 9 | Add integration test | `pkg/agent/container/claude_code_integration_test.go` |
| 10 | Update documentation | `docs/*.md` |
| 11 | Final verification | N/A |

**Estimated commits:** 11
**Key dependencies:** Docker, claude-code-acp npm package, ANTHROPIC_API_KEY for testing
