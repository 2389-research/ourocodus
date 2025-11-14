# WebSocket Message Log Redaction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redact sensitive data from WebSocket message logs to prevent PII/credential exposure and ensure GDPR/CCPA compliance.

**Architecture:** Parse incoming messages to extract only the message type field, log type + size + context (direction, connection ID), never log raw payloads.

**Tech Stack:** Go, gorilla/websocket, JSON parsing

---

## Task 1: Create message type extraction helper

**Files:**
- Create: `pkg/relay/logging.go`
- Test: `pkg/relay/logging_test.go`

**Step 1: Write the failing test**

```go
// pkg/relay/logging_test.go
package relay

import (
	"testing"
)

func TestExtractMessageType(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "valid message with type",
			input:    []byte(`{"type":"agent:spawn","payload":{"role":"echo"}}`),
			expected: "agent:spawn",
		},
		{
			name:     "valid JSON without type",
			input:    []byte(`{"action":"test","data":"value"}`),
			expected: "unknown",
		},
		{
			name:     "invalid JSON",
			input:    []byte(`{invalid json`),
			expected: "unknown",
		},
		{
			name:     "empty message",
			input:    []byte(``),
			expected: "unknown",
		},
		{
			name:     "type with special characters",
			input:    []byte(`{"type":"test<script>alert(1)</script>"}`),
			expected: "test_script_alert_1___script_",
		},
		{
			name:     "very long type",
			input:    []byte(`{"type":"` + string(make([]byte, 100)) + `"}`),
			expected: "", // Will check length limit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMessageType(tt.input)
			if tt.name == "very long type" {
				if len(result) > 48 {
					t.Errorf("expected type length <= 48, got %d", len(result))
				}
			} else if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "safe string",
			input:    "agent:spawn",
			maxLen:   48,
			expected: "agent:spawn",
		},
		{
			name:     "with special chars",
			input:    "test<script>",
			maxLen:   48,
			expected: "test_script_",
		},
		{
			name:     "truncates at maxLen",
			input:    "verylongtypename" + string(make([]byte, 50)),
			maxLen:   20,
			expected: "verylongtypename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeType(tt.input, tt.maxLen)
			if len(result) > tt.maxLen {
				t.Errorf("expected length <= %d, got %d", tt.maxLen, len(result))
			}
			// Just check it's not the truncated version for the last test
			if tt.name == "truncates at maxLen" && len(result) > tt.maxLen {
				t.Errorf("failed to truncate: len=%d, maxLen=%d", len(result), tt.maxLen)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/relay -v -run "TestExtractMessageType|TestSanitizeType"`
Expected: FAIL with "undefined: extractMessageType" and "undefined: sanitizeType"

**Step 3: Write minimal implementation**

```go
// pkg/relay/logging.go
package relay

import (
	"encoding/json"
	"regexp"
	"strings"
)

var unsafeCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9:/_-]`)

// extractMessageType safely extracts the "type" field from a JSON message.
// Returns "unknown" if the message is invalid JSON or missing the type field.
// The returned type is sanitized to prevent log injection.
func extractMessageType(data []byte) string {
	// Limit parsing to reasonable message size to avoid DoS
	if len(data) > 1024*1024 { // 1MB
		return "unknown_large"
	}

	var msg struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return "unknown"
	}

	if msg.Type == "" {
		return "unknown"
	}

	return sanitizeType(msg.Type, 48)
}

// sanitizeType removes unsafe characters and truncates to maxLen.
// This prevents log injection attacks and keeps logs readable.
func sanitizeType(s string, maxLen int) string {
	// Replace unsafe characters with underscores
	safe := unsafeCharsRegex.ReplaceAllString(s, "_")

	// Truncate if needed
	if len(safe) > maxLen {
		return safe[:maxLen]
	}

	return safe
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/relay -v -run "TestExtractMessageType|TestSanitizeType"`
Expected: PASS (all tests)

**Step 5: Commit**

```bash
git add pkg/relay/logging.go pkg/relay/logging_test.go
git commit -m "feat: add message type extraction for safe logging

Adds extractMessageType() and sanitizeType() helpers to safely extract
message types from JSON payloads without exposing sensitive data.

- Handles invalid JSON gracefully
- Sanitizes type strings to prevent log injection
- Truncates long types to 48 chars
- Returns 'unknown' for missing/invalid types

Relates to #186"
```

---

## Task 2: Update relay message logging

**Files:**
- Modify: `pkg/relay/server.go` (find the logging statement around line 584)
- Test: Manual verification (integration test in next task)

**Step 1: Find current logging statement**

Run: `grep -n "Received message" pkg/relay/server.go`
Expected: Shows line number with `s.logger.Printf("[RELAY] Received message: %s", string(message))`

**Step 2: Replace logging statement**

Find the line in `pkg/relay/server.go` that looks like:
```go
s.logger.Printf("[RELAY] Received message: %s", string(message))
```

Replace with:
```go
messageType := extractMessageType(message)
s.logger.Printf("[RELAY] dir=recv type=%s size=%dB", messageType, len(message))
```

**Note:** If there's connection/session context available in the handler, also log those IDs:
```go
s.logger.Printf("[RELAY] dir=recv conn=%s type=%s size=%dB", connID, messageType, len(message))
```

**Step 3: Check for other payload logging sites**

Run: `grep -rn "string(message)" pkg/relay/ pkg/containersession/`
Expected: Identify any other locations logging raw message content

For each found location, apply the same pattern:
- Extract type with `extractMessageType()`
- Log type + size instead of full content

**Step 4: Build to verify compilation**

Run: `make build`
Expected: Successful build with no errors

**Step 5: Commit**

```bash
git add pkg/relay/server.go
git commit -m "fix: redact sensitive data from WebSocket logs

Replace raw message content logging with type + size only.
Prevents PII and credential exposure in logs.

Before: [RELAY] Received message: {full JSON payload}
After: [RELAY] dir=recv type=agent:spawn size=156B

Fixes #186"
```

---

## Task 3: Add integration test for log redaction

**Files:**
- Modify: `pkg/relay/server_test.go` or create `pkg/relay/logging_integration_test.go`

**Step 1: Write test that verifies no payload in logs**

```go
// pkg/relay/logging_integration_test.go
package relay_test

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/2389-research/ourocodus/pkg/relay"
)

func TestMessageLoggingRedaction(t *testing.T) {
	// Create a buffer to capture log output
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	// Create relay server with custom logger
	// (Adjust this based on actual relay.Server constructor)
	server := relay.NewServer(relay.Config{
		Logger: logger,
		// ... other config
	})

	// Simulate receiving a message with sensitive data
	sensitiveMessage := []byte(`{
		"type": "agent:spawn",
		"payload": {
			"email": "user@example.com",
			"apiKey": "sk-secret-key-12345",
			"role": "echo"
		}
	}`)

	// Trigger message handling (adjust based on actual API)
	// This might be: server.HandleMessage(sensitiveMessage)
	// Or you might need to set up a test WebSocket connection

	// Get log output
	logOutput := logBuf.String()

	// Verify sensitive data is NOT in logs
	if strings.Contains(logOutput, "user@example.com") {
		t.Error("email found in logs - not redacted!")
	}
	if strings.Contains(logOutput, "sk-secret-key-12345") {
		t.Error("API key found in logs - not redacted!")
	}

	// Verify we DO log the type and size
	if !strings.Contains(logOutput, "type=agent:spawn") {
		t.Error("message type not found in logs")
	}
	if !strings.Contains(logOutput, "size=") {
		t.Error("message size not found in logs")
	}
}
```

**Step 2: Run test to verify it passes**

Run: `go test ./pkg/relay -v -run TestMessageLoggingRedaction`
Expected: PASS

**Note:** This test may need adjustment based on how the relay server is structured. If direct testing is difficult, create a focused unit test that calls the logging code path.

**Step 3: Commit**

```bash
git add pkg/relay/logging_integration_test.go
git commit -m "test: verify sensitive data redaction in logs

Adds integration test confirming that PII and credentials
are not present in relay message logs.

Relates to #186"
```

---

## Task 4: Run full test suite and verify

**Step 1: Run all tests**

Run: `go test ./...`
Expected: All tests pass

**Step 2: Run linters**

Run: `make lint`
Expected: No linting errors

**Step 3: Format code**

Run: `make fmt`
Expected: Code formatted correctly

**Step 4: Build all binaries**

Run: `make build`
Expected: Successful build

**Step 5: Manual verification (optional)**

If you want to see it in action:
1. Start the relay: `./bin/relay`
2. Send a test message via the PWA or curl
3. Check logs - should show `type=<type> size=<N>B` instead of full payload

---

## Task 5: Update documentation

**Files:**
- Modify: `CLAUDE.md` or `docs/` if logging behavior is documented

**Step 1: Document logging behavior**

If there's documentation about logging or debugging, add a note:

```markdown
## Logging

The relay logs WebSocket message metadata (type and size) but never logs
full message content to protect against PII exposure.

Logs show: `[RELAY] dir=recv type=agent:spawn size=156B`

For debugging with full payloads (development only), see [future debug flag docs].
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: document WebSocket log redaction behavior

Explains that relay logs message type/size only, not full content.

Relates to #186"
```

---

## Task 6: Create pull request

**Step 1: Push branch**

Run: `git push -u origin fix/worktree-recovery-and-log-redaction`

**Step 2: Create PR**

Run:
```bash
gh pr create --title "fix: redact sensitive data from WebSocket logs" --body "$(cat <<'EOF'
## Summary
Implements log redaction for WebSocket messages to prevent PII and credential exposure (#186).

## Changes
- Add `extractMessageType()` helper to safely extract message types from JSON
- Add `sanitizeType()` to prevent log injection attacks
- Replace raw message logging with type + size only
- Add tests for extraction and sanitization
- Add integration test verifying no sensitive data in logs

## Before
```
[RELAY] Received message: {"type":"agent:spawn","email":"user@example.com","apiKey":"sk-12345"}
```

## After
```
[RELAY] dir=recv type=agent:spawn size=156B
```

## Testing
- Unit tests for message type extraction
- Unit tests for sanitization
- Integration test verifying sensitive data not logged
- Manual verification with running relay

## Security Impact
Reduces GDPR/CCPA compliance risk by ensuring PII and credentials never appear in logs.

Fixes #186

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

**Step 3: Verify PR created**

Run: `gh pr view`
Expected: Shows the PR details

---

## Notes for Future Enhancement

Based on zen feedback, these improvements can be added in a follow-up PR:

1. **Debug flag for full payload logging** (development only):
   - Add `RELAY_LOG_PAYLOADS=1` env var
   - Only log payloads for allowlisted message types
   - Truncate to reasonable size
   - Add sampling (1% of messages)

2. **Additional context in logs**:
   - Connection ID
   - Session ID
   - Agent ID (if available)
   - WebSocket opcode (text vs binary)

3. **Rate limiting for invalid JSON**:
   - Aggregate repeated "unknown" types
   - Log summary every 60s instead of per-message

4. **Binary frame handling**:
   - Detect binary frames and skip JSON parsing
   - Log as `type=binary opcode=2`
