# Demo Scripts Cleanup - Implementation Plan

**Date:** 2025-11-06
**Issue:** #163
**Branch:** `feature/clean-up-demo-scripts-163`
**Design Doc:** [2025-11-06-demo-scripts-cleanup-design.md](./2025-11-06-demo-scripts-cleanup-design.md)

## Overview

This plan implements the conservative audit-and-migrate approach for cleaning up demo scripts, incorporating feedback from design review.

## Pre-Migration Setup

### Task 1: Create Examples Directory Structure

```bash
mkdir -p examples/{basic-demo,interactive-repl,performance-testing,smoke-tests,debugging}
```

**Deliverable:** Empty directory structure ready for migration

### Task 2: Create Automated Path Verification Script

Create `scripts/verify-paths.sh` to catch hard-coded references:

```bash
#!/bin/bash
# Verify no hard-coded references to old script locations
grep -r "scripts/demo" --exclude-dir=".git" --exclude-dir="docs" . && exit 1
grep -r "scripts/interactive" --exclude-dir=".git" --exclude-dir="docs" . && exit 1
grep -r "scripts/demo-performance" --exclude-dir=".git" --exclude-dir="docs" . && exit 1
grep -r "scripts/smoketest" --exclude-dir=".git" --exclude-dir="docs" . && exit 1
grep -r "scripts/container-race" --exclude-dir=".git" --exclude-dir="docs" . && exit 1
echo "✓ No hard-coded script paths found"
```

**Deliverable:** Verification script that can run in CI

**Commit:** `chore: add path verification script for migration`

### Task 3: Audit Current CI/CD References

Check for references in:
- `.github/workflows/*.yml`
- `Makefile`
- `docker-compose.yml`
- Root `README.md`
- `CONTRIBUTING.md`

**Deliverable:** List of all files that reference scripts to be moved

## Migration Phase (Incremental Rollout)

### Batch 1: Basic Demo (Low Risk)

#### Task 4: Migrate scripts/demo/

1. **Audit:**
   - Read `scripts/demo/main.go`
   - Verify it references current binaries (`bin/relay`, `bin/echo-agent`)
   - Check for hard-coded paths

2. **Test current location:**
   ```bash
   make build
   cd scripts/demo
   go run main.go
   # Verify: Relay starts, websocket connects, demo runs
   ```

3. **Migrate:**
   ```bash
   git mv scripts/demo examples/basic-demo
   ```

4. **Write README:**
   Create `examples/basic-demo/README.md` with:
   - Purpose: Demonstrate basic relay+agent interaction
   - Prerequisites: Built binaries (`make build`)
   - Run instructions: `cd examples/basic-demo && go run main.go`
   - Expected output: Step-by-step walkthrough
   - Key concepts: WebSocket connection, agent spawning, message routing
   - Troubleshooting: Common errors and solutions

5. **Update paths in script:**
   - Change relative paths from `../../bin/` to match new location
   - Test: `cd examples/basic-demo && go run main.go`

6. **Verify:**
   ```bash
   scripts/verify-paths.sh
   make build && cd examples/basic-demo && go run main.go
   ```

**Commit:** `docs: migrate demo script to examples/basic-demo`

**Rollback plan:** `git revert HEAD` if issues found

#### Task 5: Migrate scripts/interactive/

Follow same process as Task 4:
1. Audit and test current location
2. `git mv scripts/interactive examples/interactive-repl`
3. Write comprehensive README
4. Update paths
5. Test from new location
6. Verify

**Commit:** `docs: migrate interactive script to examples/interactive-repl`

### Batch 2: Testing Scripts (Medium Risk)

#### Task 6: Migrate scripts/demo-performance/

1. **Audit entire directory:**
   - List all scripts: `ls scripts/demo-performance/`
   - Read each script to understand purpose
   - Check dependencies on external services (NATS, databases, etc.)

2. **Test current location:**
   ```bash
   cd scripts/demo-performance
   ./demo-setup.sh
   ./demo-run.sh
   # Verify: Performance metrics generated
   ./demo-reset.sh
   ```

3. **Migrate:**
   ```bash
   git mv scripts/demo-performance examples/performance-testing
   ```

4. **Write comprehensive README:**
   - Purpose: Load testing and performance benchmarking
   - Prerequisites: Running NATS, built binaries, required env vars
   - Scripts overview:
     - `demo-setup.sh`: Initialization
     - `demo-run.sh`: Run test
     - `demo-load-test.sh`: Heavy load scenario
     - `demo-reset.sh`: Cleanup
     - `generate-metrics-table.sh`: Results formatting
   - Expected output: Metrics tables and graphs
   - Key concepts: Load testing, performance measurement
   - Troubleshooting: Service dependencies, environment setup

5. **Update paths** in all scripts

6. **Test from new location**

**Commit:** `docs: migrate performance scripts to examples/performance-testing`

#### Task 7: Migrate scripts/smoketest/

Follow same process:
1. Audit `scripts/smoketest/relay/main.go` and `scripts/smoketest/session/main.go`
2. Test current location
3. `git mv scripts/smoketest examples/smoke-tests`
4. Write README explaining smoke test purpose and usage
5. Update paths
6. Test from new location

**Commit:** `docs: migrate smoke test scripts to examples/smoke-tests`

### Batch 3: Debug Scripts (Low Usage)

#### Task 8: Migrate scripts/container-race/

1. **Audit:**
   - Determine if this is still relevant for debugging
   - Check if it demonstrates current bug or is historical

2. **Decision point:**
   - If still relevant: Migrate to `examples/debugging/container-race/`
   - If outdated: Document why and remove

3. **If migrating:**
   ```bash
   git mv scripts/container-race examples/debugging/container-race
   ```

4. **Write README** explaining the race condition scenario

**Commit:** `docs: migrate container-race debug script to examples/debugging` OR
`chore: remove outdated container-race debug script`

**Commit message detail:** If removing, explain: "This debug script was created to reproduce a specific race condition that has since been fixed in commit [hash]. Removing as it no longer demonstrates current functionality."

## Post-Migration Tasks

### Task 9: Create Examples Overview README

Create `examples/README.md`:

```markdown
# Ourocodus Examples

This directory contains example scripts demonstrating various features of the Ourocodus system.

## Available Examples

### Basic Demo
**Location:** `basic-demo/`
**Purpose:** Simple relay + agent interaction
**Difficulty:** Beginner
**Prerequisites:** Built binaries

[More details](./basic-demo/README.md)

### Interactive REPL
**Location:** `interactive-repl/`
**Purpose:** Interactive testing and exploration
**Difficulty:** Intermediate
**Prerequisites:** Built binaries, understanding of protocol

[More details](./interactive-repl/README.md)

### Performance Testing
**Location:** `performance-testing/`
**Purpose:** Load testing and benchmarking
**Difficulty:** Advanced
**Prerequisites:** Built binaries, NATS running, understanding of metrics

[More details](./performance-testing/README.md)

### Smoke Tests
**Location:** `smoke-tests/`
**Purpose:** Manual verification of core functionality
**Difficulty:** Intermediate
**Prerequisites:** Built binaries

[More details](./smoke-tests/README.md)

### Debugging Examples
**Location:** `debugging/`
**Purpose:** Debug scenarios and edge cases
**Difficulty:** Advanced
**Prerequisites:** Deep understanding of system

[More details](./debugging/README.md)

## Running Examples

All examples assume you've built the project first:

```bash
make build
```

Then navigate to the specific example directory and follow its README.

## Contributing Examples

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines on adding new examples.
```

**Commit:** `docs: add examples overview README`

### Task 10: Update Documentation References

Update references in:

1. **Root README.md:**
   - Search for any references to `scripts/demo` or similar
   - Update to point to `examples/`
   - Add link to `examples/README.md` in appropriate section

2. **CONTRIBUTING.md:**
   - Update any developer workflow references
   - Mention examples/ for learning

3. **Check CI workflows:**
   - Verify no workflows reference moved scripts
   - If found, update paths

**Commit:** `docs: update references to moved examples`

### Task 11: Verify scripts/ Directory

Confirm `scripts/` now contains only infrastructure:

```bash
ls -la scripts/
# Should show:
# - nats-init.sh
# - setup-worktrees.sh
# - run-e2e.sh
# - smoke-test.sh
# - verify-paths.sh (new)
```

**Deliverable:** Clean scripts/ directory with clear purpose

### Task 12: Update .gitignore (if needed)

Verify `.gitignore` doesn't exclude `examples/`:

```bash
# Check if examples/ would be ignored
git check-ignore examples/
```

If ignored, update `.gitignore` to explicitly include it.

**Commit:** `chore: ensure examples/ is tracked by git` (if needed)

### Task 13: Final Verification

Run complete verification:

```bash
# 1. No hard-coded paths
./scripts/verify-paths.sh

# 2. Build succeeds
make build

# 3. Run each example
cd examples/basic-demo && go run main.go
cd ../interactive-repl && go run main.go
# ... test each example

# 4. CI passes
git push origin feature/clean-up-demo-scripts-163
# Wait for CI checks
```

**Deliverable:** All checks passing

### Task 14: Final Cleanup Commit

Create final commit with any minor adjustments:

**Commit:** `chore: finalize scripts/ directory structure cleanup for #163`

## Testing Checklist

Before marking complete:

- [ ] All examples run successfully from new locations
- [ ] No hard-coded references to old paths (`verify-paths.sh` passes)
- [ ] Each example has comprehensive README
- [ ] Root README updated
- [ ] CONTRIBUTING.md updated (if needed)
- [ ] CI/CD workflows verified
- [ ] `.gitignore` doesn't exclude examples/
- [ ] All commits follow format and have clear messages
- [ ] Git history preserved with `git mv`

## Rollback Strategy

If issues discovered after merge:

1. **Individual script issues:** Revert specific migration commit
2. **Widespread issues:** Revert entire branch
3. **CI breakage:** Emergency fix with direct path updates

Keep migration commits atomic (one script/directory per commit) for easy selective rollback.

## Success Criteria

- ✅ `scripts/` contains only production infrastructure (5 files)
- ✅ `examples/` has 5 subdirectories with working demos
- ✅ Each example has comprehensive, tested README
- ✅ No references to old script locations in codebase
- ✅ CI/CD passes
- ✅ Git history preserved with `git mv`
- ✅ Issue #163 acceptance criteria met

## Time Estimate

- Pre-migration setup: 1 hour
- Batch 1 (2 scripts): 2 hours
- Batch 2 (2 script dirs): 3 hours
- Batch 3 (1 script dir): 1 hour
- Post-migration tasks: 2 hours
- Testing and verification: 1 hour

**Total:** ~10 hours

## Notes

- Use incremental commits for easy rollback
- Test each example in clean environment (not just locally)
- Update this plan if new issues discovered during migration
- Document any deviations from plan in commit messages
