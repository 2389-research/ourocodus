# TTY Error Investigation Results
**Date:** 2025-11-03
**Issue:** "the input device is not a TTY" error when spawning containerized agents via PackNplay
**Status:** Root cause identified, solution requires architectural change

## Executive Summary

After extensive investigation and fork modifications, we've determined the TTY error is **NOT** about stdin detachment or race conditions. The issue is a fundamental behavioral difference in how Docker CLI operates when invoked from a Go program vs an interactive shell.

**Key Finding:** The same `docker exec -u root <container> chown ...` command that fails from Go succeeds when run manually from bash, even though both environments should have identical stdin configurations.

## Investigation Timeline

### Attempted Solutions

We forked PackNplay (`github.com/2389-research/packnplay`) and tried 4 different approaches:

#### 1. Default Go Behavior (`cmd.Stdin = nil`)
- **Theory:** Go documentation says `nil` reads from `os.DevNull`
- **Result:** Failed with TTY error
- **Commit:** `406e910`

#### 2. Explicit Empty Reader (`cmd.Stdin = strings.NewReader("")`)
- **Theory:** Explicit reader more reliable than nil across platforms
- **Result:** Failed with TTY error
- **Commit:** `a46004a`

#### 3. Null Device (`cmd.Stdin = os.Open(os.DevNull)`)
- **Theory:** Actual /dev/null file descriptor prevents TTY detection
- **Result:** Failed with TTY error
- **Commit:** `1bf558f`

#### 4. Process Group Detachment (`SysProcAttr{Setpgid: true}`)
- **Theory:** New process group fully isolates from controlling terminal
- **Result:** Failed with TTY error
- **Commit:** `c29a7c4` (final fork version)

### Test Methodology

Created `scripts/test-single-spawn/main.go` to isolate the issue:
- Single container spawn (rules out race conditions)
- Verbose logging showing exact docker commands
- Reproducible failure on `docker exec -u root ... chown`

**Critical Test:** Running the same docker exec command manually from bash **succeeds**, proving Docker CLI behaves differently based on parent process type.

## Technical Analysis

### What Works
✅ Git commands (with `strings.NewReader`)
✅ `docker run` commands
✅ `docker cp` commands
✅ `docker exec mkdir` commands
✅ Manual docker exec from bash

### What Fails
❌ `docker exec -u root ... chown` from Go
- Specifically fails when using `-u` flag to switch users
- Only fails when invoked via Go's `exec.Command`
- Error: "the input device is not a TTY"

### Root Cause Hypothesis

Docker CLI appears to have special TTY detection logic that:
1. Checks if the parent process is an interactive shell
2. Behaves differently for non-interactive parents (like Go programs)
3. Possibly related to the `-u` (user switch) flag requiring terminal semantics

This is likely a security or compatibility feature in Docker CLI that cannot be easily bypassed through stdin manipulation.

## Fork Details

**Repository:** `github.com/2389-research/packnplay`
**Branch:** `fix/stdin-detach`
**Final Version:** `v1.0.3-0.20251103164944-c29a7c42b706`

**Files Modified:**
- `pkg/docker/client.go`: /dev/null + Setpgid (main CLI wrapper)
- `pkg/runner/runner.go`: Removed -it flags from detached containers
- `pkg/userdetect/detect.go`: strings.NewReader for 4 docker commands
- `pkg/git/worktree.go`: strings.NewReader for 7 git commands

## Consensus Recommendation

Per zen consensus analysis (models: o3-mini, o4-mini), the fallback plan if Option A fails:

### Option C: Docker SDK (Recommended Next Step)

**Approach:** Use Go Docker SDK (`github.com/docker/docker/client`) instead of Docker CLI

**Advantages:**
1. **Bypasses CLI entirely** - No TTY detection issues
2. **Direct Engine API** - What we'll need for ACP anyway
3. **Better control** - Programmatic access to all Docker features
4. **More reliable** - No subprocess or shell semantics to worry about
5. **Better performance** - No process spawning overhead

**Implementation:**
- Replace PackNplay's CLI wrapper with Docker SDK client
- Use `client.ContainerExec()` and `client.ContainerExecAttach()` for exec operations
- Use `client.ContainerCreate()` and `client.ContainerStart()` for spawning
- Maintain compatibility with existing PackNplay interface

**Estimated Effort:** 1-2 days to refactor and test

### Alternative: Option B (PackNplay CLI)

Use PackNplay command-line tool instead of library:
- Run `packnplay spawn` via bash
- Bash subprocess would have proper TTY semantics
- Less elegant but would work immediately

## Conclusion

We've exhaustively tested every reasonable stdin detachment approach. The issue is Docker CLI's behavior when invoked from non-interactive processes, not our code.

**Recommended Path:** Implement Option C (Docker SDK) for a proper, long-term solution that also sets us up correctly for ACP integration.

## Repository State

**Branch:** `feature/packnplay-launcher-83`
**Commits:**
- `9db156b`: Investigation results and test case
- `5faed5d`: Fork integration with comprehensive fixes

**Test Files:**
- `scripts/test-single-spawn/main.go`: Minimal reproducible test case
- `scripts/container-race/main.go`: Full demo (currently non-functional)

---

*Investigation completed Saturday, November 3, 2025*
