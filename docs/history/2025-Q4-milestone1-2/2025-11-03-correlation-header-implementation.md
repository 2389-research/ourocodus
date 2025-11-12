# Correlation Header Consistency Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix `wrapNatsMessage` to use configured correlation header instead of hardcoded `"Correlation-Id"`.

**Architecture:** Parameter passing - add `correlationHeader string` parameter to function and update call sites to pass `config.CorrelationHeader`.

**Tech Stack:** Go 1.23.0, NATS.go, standard Go testing

---

## Task 1: Update wrapNatsMessage Function Signature

**Files:**
- Modify: `pkg/nats/message.go:20` (function signature)

**Step 1: Update function signature**

Change the function signature from:
```go
func wrapNatsMessage(msg *nats.Msg) *Message {
```

To:
```go
func wrapNatsMessage(msg *nats.Msg, correlationHeader string) *Message {
```

Location: Line 20 in `pkg/nats/message.go`

**Step 2: Commit signature change**

```bash
git add pkg/nats/message.go
git commit -m "refactor: add correlationHeader parameter to wrapNatsMessage

Add correlationHeader string parameter to make header name configurable
instead of hardcoded.

Related to #90"
```

---

## Task 2: Update Correlation ID Extraction Logic

**Files:**
- Modify: `pkg/nats/message.go:35` (extraction line)

**Step 1: Update extraction to use parameter**

Change line 35 from:
```go
// Extract correlation ID
m.CorrelationID = msg.Header.Get("Correlation-Id")
```

To:
```go
// Extract correlation ID using configured header name
m.CorrelationID = msg.Header.Get(correlationHeader)
```

**Step 2: Commit extraction update**

```bash
git add pkg/nats/message.go
git commit -m "fix: use correlationHeader parameter instead of hardcoded value

Replace hardcoded 'Correlation-Id' string with correlationHeader
parameter to enable consistent header handling.

Related to #90"
```

---

## Task 3: Update Call Site in subscription.go

**Files:**
- Modify: `pkg/nats/subscription.go:79` (messageHandler method)

**Step 1: Update function call**

Change line 79 from:
```go
wrappedMsg := wrapNatsMessage(msg)
```

To:
```go
wrappedMsg := wrapNatsMessage(msg, s.client.config.CorrelationHeader)
```

Context: The `Subscription` struct has access to `s.client.config` which contains the configured correlation header name.

**Step 2: Commit subscription update**

```bash
git add pkg/nats/subscription.go
git commit -m "fix: pass configured correlation header in subscription handler

Update subscription messageHandler to pass s.client.config.CorrelationHeader
to wrapNatsMessage, ensuring inbound subscription messages use the
configured header name.

Related to #90"
```

---

## Task 4: Update Call Site in client.go

**Files:**
- Modify: `pkg/nats/client.go:312` (Request method)

**Step 1: Update function call**

Change line 312 from:
```go
return wrapNatsMessage(resp), nil
```

To:
```go
return wrapNatsMessage(resp, c.config.CorrelationHeader), nil
```

Context: The `client` method has `c.config` in scope which contains the configured correlation header name.

**Step 2: Commit client update**

```bash
git add pkg/nats/client.go
git commit -m "fix: pass configured correlation header in Request handler

Update Request method to pass c.config.CorrelationHeader to
wrapNatsMessage, ensuring request-reply responses use the
configured header name.

Related to #90"
```

---

## Task 5: Update Existing Test

**Files:**
- Modify: `pkg/nats/message_test.go:79` (TestWrapNatsMessage)

**Step 1: Find the test function**

Locate `TestWrapNatsMessage` function (around line 79).

**Step 2: Update test call to include parameter**

Change the call from:
```go
wrapped := wrapNatsMessage(natsMsg)
```

To:
```go
wrapped := wrapNatsMessage(natsMsg, "Correlation-Id")
```

This maintains backward compatibility by testing with the default header name.

**Step 3: Run test to verify it passes**

```bash
cd pkg/nats
go test -run TestWrapNatsMessage -v
```

Expected: PASS

**Step 4: Commit test update**

```bash
git add pkg/nats/message_test.go
git commit -m "test: update TestWrapNatsMessage to pass correlation header

Update existing test to pass 'Correlation-Id' parameter to
wrapNatsMessage function after signature change.

Related to #90"
```

---

## Task 6: Add New Test for Custom Header Names

**Files:**
- Modify: `pkg/nats/message_test.go` (add new test function)

**Step 1: Add TestWrapNatsMessage_CustomCorrelationHeader**

Add this new test function after `TestWrapNatsMessage`:

```go
func TestWrapNatsMessage_CustomCorrelationHeader(t *testing.T) {
	natsMsg := nats.NewMsg("test.subject")
	natsMsg.Header = nats.Header{}
	natsMsg.Header.Set("X-Custom-Correlation", "custom-id-456")

	wrapped := wrapNatsMessage(natsMsg, "X-Custom-Correlation")

	if wrapped.CorrelationID != "custom-id-456" {
		t.Errorf("CorrelationID = %q, want %q", wrapped.CorrelationID, "custom-id-456")
	}
}
```

Location: After `TestWrapNatsMessage` (around line 95+)

**Step 2: Run new test to verify it passes**

```bash
cd pkg/nats
go test -run TestWrapNatsMessage_CustomCorrelationHeader -v
```

Expected: PASS

**Step 3: Commit new test**

```bash
git add pkg/nats/message_test.go
git commit -m "test: add test for custom correlation header names

Add TestWrapNatsMessage_CustomCorrelationHeader to verify that
wrapNatsMessage correctly extracts correlation IDs when using
a non-standard header name.

This ensures the fix works for any configured header name,
not just the default 'Correlation-Id'.

Related to #90"
```

---

## Task 7: Run Full Test Suite

**Files:**
- None (verification only)

**Step 1: Run all tests**

```bash
go test ./...
```

Expected: All tests pass

**Step 2: If any tests fail**

Investigate the failure:
1. Read the error message carefully
2. Identify which test failed and why
3. Fix the issue
4. Re-run tests
5. Commit the fix

---

## Task 8: Run Linting and Formatting

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

Run gofumpt, golangci-lint, and staticcheck to ensure code quality.

Related to #90"
```

---

## Task 9: Create Pull Request

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
git push -u origin fix/correlation-header-90
```

**Step 3: Create pull request**

```bash
gh pr create --title "Fix: Honor configured correlation header in wrapNatsMessage" --body "$(cat <<'EOF'
## Summary
- Fixed `wrapNatsMessage` to use configured correlation header instead of hardcoded `"Correlation-Id"`
- Updated both call sites (`subscription.go` and `client.go`) to pass `config.CorrelationHeader`
- Added test case for custom correlation header names
- Ensures consistency between outbound and inbound message handling

## Changes
- `pkg/nats/message.go`: Added `correlationHeader` parameter to function
- `pkg/nats/subscription.go`: Pass `s.client.config.CorrelationHeader` at call site
- `pkg/nats/client.go`: Pass `c.config.CorrelationHeader` at call site
- `pkg/nats/message_test.go`: Updated existing test and added new test case

## Testing
- All existing tests pass
- New test verifies custom header extraction works correctly
- No change in behavior with default config (backward compatible)

Fixes #90

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

**Step 4: Verify PR creation**

Check that the PR was created successfully and linked to issue #90.

---

## Verification Checklist

Before marking complete, verify:

- [ ] `wrapNatsMessage` signature includes `correlationHeader string` parameter
- [ ] `wrapNatsMessage` uses `correlationHeader` parameter instead of hardcoded string
- [ ] `subscription.go:79` passes `s.client.config.CorrelationHeader`
- [ ] `client.go:312` passes `c.config.CorrelationHeader`
- [ ] Existing test updated to pass correlation header parameter
- [ ] New test added for custom header names
- [ ] All tests pass: `go test ./...`
- [ ] Formatting: `mise run fmt`
- [ ] Linting: `mise run lint`
- [ ] Static analysis: `mise run check`
- [ ] PR created and linked to #90
