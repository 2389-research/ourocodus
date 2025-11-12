# NATS Pending Limits Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make NATS pending limits configurable with safe defaults instead of unlimited buffering.

**Architecture:** Add fields to subOptions, create functional options, update subscription.start() to use configured limits.

**Tech Stack:** Go 1.24, NATS.go, testify for assertions

---

## Task 1: Add Fields to subOptions Struct

**Files:**
- Modify: `pkg/nats/options.go:271-274` (subOptions struct)

**Step 1: Add pending limit fields**

Update the subOptions struct (around line 271):

```go
type subOptions struct {
	queueGroup        string
	maxInflight       int
	pendingLimitMsgs  int // NATS default: 512*1024
	pendingLimitBytes int // NATS default: 64*1024*1024
}
```

Location: After line 271 (after existing fields)

**Step 2: Commit struct addition**

```bash
git add pkg/nats/options.go
git commit -m "feat: add pending limit fields to subOptions

Add pendingLimitMsgs and pendingLimitBytes fields to enable
configurable pending limits for NATS subscriptions.

Related to #91"
```

---

## Task 2: Update defaultSubOptions

**Files:**
- Modify: `pkg/nats/options.go:276-279` (defaultSubOptions function)

**Step 1: Set safe defaults**

Update defaultSubOptions function:

```go
func defaultSubOptions() *subOptions {
	return &subOptions{
		queueGroup:        "",
		maxInflight:       1,
		pendingLimitMsgs:  512 * 1024,      // 524,288 messages (NATS recommended)
		pendingLimitBytes: 64 * 1024 * 1024, // 67,108,864 bytes (64 MB)
	}
}
```

**Step 2: Commit default values**

```bash
git add pkg/nats/options.go
git commit -m "feat: set NATS-recommended defaults for pending limits

Set pendingLimitMsgs to 524,288 and pendingLimitBytes to 64MB,
matching NATS recommended safe defaults for production use.

This replaces the previous unlimited buffering (-1, -1) with
safe limits that prevent unbounded memory growth.

Related to #91"
```

---

## Task 3: Add WithPendingLimits Option

**Files:**
- Modify: `pkg/nats/options.go` (add after WithMaxInflight, around line 294)

**Step 1: Add WithPendingLimits function**

Add after WithMaxInflight function:

```go
// WithPendingLimits sets custom pending message and byte limits for subscriptions.
//
// The pending limits control how many messages and bytes can be buffered by the
// NATS client when the subscriber cannot keep up with the message rate. When either
// limit is exceeded, the subscription will be considered a "slow consumer" and may
// be dropped by the server.
//
// Use -1 for either parameter to disable that specific limit (not recommended).
//
// Default values (if not specified):
//   - Messages: 524,288 (512 * 1024)
//   - Bytes: 67,108,864 (64 MB)
//
// Example:
//
//	// High-throughput subscription with 1M message buffer and 128MB byte buffer
//	sub, err := client.Subscribe(ctx, "orders", handler,
//	    nats.WithPendingLimits(1_000_000, 128*1024*1024))
func WithPendingLimits(msgs, bytes int) SubOption {
	return func(opts *subOptions) {
		opts.pendingLimitMsgs = msgs
		opts.pendingLimitBytes = bytes
	}
}
```

Location: After WithMaxInflight (around line 294)

**Step 2: Commit WithPendingLimits**

```bash
git add pkg/nats/options.go
git commit -m "feat: add WithPendingLimits option for custom limits

Add WithPendingLimits() functional option to allow users to
customize pending message and byte limits for specific
subscriptions.

Includes comprehensive documentation with examples and
default value references.

Related to #91"
```

---

## Task 4: Add WithUnlimitedPending Option

**Files:**
- Modify: `pkg/nats/options.go` (add after WithPendingLimits)

**Step 1: Add WithUnlimitedPending function**

Add after WithPendingLimits function:

```go
// WithUnlimitedPending disables all pending limits for the subscription.
//
// WARNING: This allows unbounded memory growth if the subscriber cannot keep up
// with the message rate. Only use this if you have external backpressure mechanisms
// in place (e.g., bounded channels, rate limiting, or guaranteed fast processing).
//
// This is equivalent to calling WithPendingLimits(-1, -1).
//
// Example:
//
//	// Only use when you control message rate externally
//	sub, err := client.Subscribe(ctx, "logs", handler,
//	    nats.WithUnlimitedPending())
func WithUnlimitedPending() SubOption {
	return func(opts *subOptions) {
		opts.pendingLimitMsgs = -1
		opts.pendingLimitBytes = -1
	}
}
```

**Step 2: Commit WithUnlimitedPending**

```bash
git add pkg/nats/options.go
git commit -m "feat: add WithUnlimitedPending option for explicit opt-out

Add WithUnlimitedPending() to explicitly disable pending limits.
Includes strong warning about unbounded memory growth.

Forces users to explicitly opt-in to dangerous unlimited
buffering behavior.

Related to #91"
```

---

## Task 5: Update Subscription to Use Configured Limits

**Files:**
- Modify: `pkg/nats/subscription.go:58-62` (start method)

**Step 1: Update SetPendingLimits call**

Update the SetPendingLimits call (around line 59):

```go
// Before:
// Set pending limits
if err := s.natsSub.SetPendingLimits(-1, -1); err != nil {
	_ = s.natsSub.Unsubscribe()
	return fmt.Errorf("set pending limits: %w", err)
}

// After:
// Set pending limits from options (defaults: 512K msgs, 64MB bytes)
if err := s.natsSub.SetPendingLimits(s.opts.pendingLimitMsgs, s.opts.pendingLimitBytes); err != nil {
	_ = s.natsSub.Unsubscribe()
	return fmt.Errorf("set pending limits: %w", err)
}
```

**Step 2: Commit subscription update**

```bash
git add pkg/nats/subscription.go
git commit -m "fix: use configured pending limits instead of unlimited

Replace hardcoded SetPendingLimits(-1, -1) with values from
subscription options, enabling safe defaults and per-subscription
configuration.

This prevents unbounded memory growth from slow consumers while
maintaining flexibility for high-throughput scenarios.

Fixes #91"
```

---

## Task 6: Add Test for Default Values

**Files:**
- Modify: `pkg/nats/options_test.go` (add new test)

**Step 1: Add TestDefaultSubOptions_PendingLimits**

Add this test to options_test.go:

```go
func TestDefaultSubOptions_PendingLimits(t *testing.T) {
	opts := defaultSubOptions()

	// Verify NATS default values
	assert.Equal(t, 512*1024, opts.pendingLimitMsgs, "default message limit should be NATS default")
	assert.Equal(t, 64*1024*1024, opts.pendingLimitBytes, "default byte limit should be NATS default")
}
```

**Step 2: Run test to verify it passes**

```bash
cd pkg/nats
go test -run TestDefaultSubOptions_PendingLimits -v
```

Expected: PASS

**Step 3: Commit test**

```bash
git add pkg/nats/options_test.go
git commit -m "test: verify default pending limits match NATS recommendations

Add test confirming that defaultSubOptions() returns NATS-recommended
safe defaults (524,288 messages, 64MB bytes).

Related to #91"
```

---

## Task 7: Add Test for WithPendingLimits

**Files:**
- Modify: `pkg/nats/options_test.go` (add new test)

**Step 1: Add TestWithPendingLimits**

Add this test to options_test.go:

```go
func TestWithPendingLimits(t *testing.T) {
	opts := defaultSubOptions()

	// Apply custom limits
	WithPendingLimits(1000, 5*1024*1024)(opts)

	assert.Equal(t, 1000, opts.pendingLimitMsgs)
	assert.Equal(t, 5*1024*1024, opts.pendingLimitBytes)
}
```

**Step 2: Run test to verify it passes**

```bash
cd pkg/nats
go test -run TestWithPendingLimits -v
```

Expected: PASS

**Step 3: Commit test**

```bash
git add pkg/nats/options_test.go
git commit -m "test: verify WithPendingLimits sets custom values

Add test confirming that WithPendingLimits() correctly overrides
default pending limit values.

Related to #91"
```

---

## Task 8: Add Test for WithUnlimitedPending

**Files:**
- Modify: `pkg/nats/options_test.go` (add new test)

**Step 1: Add TestWithUnlimitedPending**

Add this test to options_test.go:

```go
func TestWithUnlimitedPending(t *testing.T) {
	opts := defaultSubOptions()

	// Apply unlimited
	WithUnlimitedPending()(opts)

	assert.Equal(t, -1, opts.pendingLimitMsgs, "unlimited should use -1")
	assert.Equal(t, -1, opts.pendingLimitBytes, "unlimited should use -1")
}
```

**Step 2: Run test to verify it passes**

```bash
cd pkg/nats
go test -run TestWithUnlimitedPending -v
```

Expected: PASS

**Step 3: Commit test**

```bash
git add pkg/nats/options_test.go
git commit -m "test: verify WithUnlimitedPending disables limits

Add test confirming that WithUnlimitedPending() sets both
limits to -1 (unlimited).

Related to #91"
```

---

## Task 9: Add Test for Explicit -1 Values

**Files:**
- Modify: `pkg/nats/options_test.go` (add new test)

**Step 1: Add TestWithPendingLimits_NegativeValues**

Add this test to options_test.go:

```go
func TestWithPendingLimits_NegativeValues(t *testing.T) {
	opts := defaultSubOptions()

	// Can explicitly set -1 via WithPendingLimits
	WithPendingLimits(-1, -1)(opts)

	assert.Equal(t, -1, opts.pendingLimitMsgs)
	assert.Equal(t, -1, opts.pendingLimitBytes)
}
```

**Step 2: Run test to verify it passes**

```bash
cd pkg/nats
go test -run TestWithPendingLimits_NegativeValues -v
```

Expected: PASS

**Step 3: Commit test**

```bash
git add pkg/nats/options_test.go
git commit -m "test: verify explicit -1 values work via WithPendingLimits

Add test confirming that WithPendingLimits(-1, -1) correctly
sets unlimited buffering, providing alternative to
WithUnlimitedPending().

Related to #91"
```

---

## Task 10: Run Full Test Suite

**Files:**
- None (verification only)

**Step 1: Run all tests**

```bash
go test ./...
```

Expected: All tests pass

**Step 2: Run NATS package tests with verbose output**

```bash
cd pkg/nats
go test -v
```

Expected: All tests pass, including new pending limit tests

**Step 3: If any tests fail**

Investigate and fix before proceeding. The baseline should be clean.

---

## Task 11: Run Linting and Formatting

**Files:**
- All modified files

**Step 1: Format code**

```bash
mise run fmt
```

Expected: No output if all files properly formatted

**Step 2: Run linter**

```bash
mise run lint
```

Expected: No errors

**Step 3: Run static analysis**

```bash
mise run check
```

Expected: No warnings

**Step 4: If any issues found**

Fix them and commit:

```bash
git add .
git commit -m "style: apply formatting and fix lint issues

Run gofumpt, golangci-lint, and staticcheck to ensure
code quality.

Related to #91"
```

---

## Task 12: Update Issue and Prepare PR

**Files:**
- None (Git operations only)

**Step 1: View final diff**

```bash
git log --oneline origin/main..HEAD
git diff origin/main..HEAD --stat
```

Verify all changes are intentional and complete.

**Step 2: Push branch**

```bash
git push -u origin feature/pending-limits-91
```

**Step 3: Create pull request**

```bash
gh pr create --title "Make NATS pending limits configurable" --body "$(cat <<'EOF'
## Summary
- Replaces unlimited buffering with safe NATS-recommended defaults
- Adds WithPendingLimits() for custom limits
- Adds WithUnlimitedPending() for explicit opt-out
- Prevents unbounded memory growth from slow consumers

## Breaking Change
This changes the default behavior from unlimited buffering to limited buffering (524,288 messages, 64MB bytes).

**Rationale:** The current unlimited default is unsafe and can cause production issues. NATS documentation explicitly warns against unlimited buffering.

**Migration:** Code that relies on unlimited buffering must add:
```go
sub, err := client.Subscribe(ctx, subject, handler, nats.WithUnlimitedPending())
```

## Impact
- Prevents unbounded memory growth in production
- Protects against slow consumer problems
- Fast failure detection (better than silent memory exhaustion)

## Testing
- Added 4 comprehensive test cases
- All existing tests pass
- Lint and formatting checks pass

Fixes #91

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

**Step 4: Link PR to issue**

The "Fixes #91" in the PR body will automatically link and close the issue when merged.

---

## Verification Checklist

Before marking complete, verify:

- [ ] pendingLimitMsgs field added to subOptions
- [ ] pendingLimitBytes field added to subOptions
- [ ] defaultSubOptions() sets NATS defaults (524,288 msgs, 64MB)
- [ ] WithPendingLimits() function implemented
- [ ] WithUnlimitedPending() function implemented
- [ ] subscription.go uses s.opts.pendingLimitMsgs
- [ ] subscription.go uses s.opts.pendingLimitBytes
- [ ] Test: default values match NATS recommendations
- [ ] Test: WithPendingLimits sets custom values
- [ ] Test: WithUnlimitedPending sets -1, -1
- [ ] Test: explicit -1 values work
- [ ] All tests pass: `go test ./...`
- [ ] Formatting: `mise run fmt`
- [ ] Linting: `mise run lint`
- [ ] Static analysis: `mise run check`
- [ ] PR created and linked to #91
