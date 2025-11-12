# Correlation Header Consistency Fix Design

**Date:** 2025-11-03
**Issue:** #90 - Honor configured correlation header name in wrapNatsMessage
**Status:** Approved

## Problem

The `wrapNatsMessage` function in `pkg/nats/message.go:35` hardcodes `"Correlation-Id"` when extracting correlation IDs from inbound messages, ignoring the configured value in `ClientConfig.CorrelationHeader`.

This creates an inconsistency:
- **Outbound messages** (Publish, Request) correctly use `c.config.CorrelationHeader` to set headers
- **Inbound messages** (Subscribe, Request responses) always look for hardcoded `"Correlation-Id"`

## Impact

If the project's standard correlation header name ever changes (or differs from `"Correlation-Id"`), outbound and inbound message handling will be inconsistent. The correlation ID will be set on outbound messages but not extracted from inbound messages.

## Solution

Pass the configured correlation header name as a parameter to `wrapNatsMessage`, ensuring both outbound and inbound message handling use the same header name from `ClientConfig`.

## Design

### Function Signature Change

**Current:**
```go
func wrapNatsMessage(msg *nats.Msg) *Message
```

**New:**
```go
func wrapNatsMessage(msg *nats.Msg, correlationHeader string) *Message
```

### Implementation Update

**File:** `pkg/nats/message.go`

**Before (line 35):**
```go
// Extract correlation ID
m.CorrelationID = msg.Header.Get("Correlation-Id")
```

**After:**
```go
// Extract correlation ID using configured header name
m.CorrelationID = msg.Header.Get(correlationHeader)
```

### Call Site Updates

**Call Site 1: `pkg/nats/subscription.go:79`**

Context: Wrapping inbound subscription messages

**Before:**
```go
wrappedMsg := wrapNatsMessage(msg)
```

**After:**
```go
wrappedMsg := wrapNatsMessage(msg, s.client.config.CorrelationHeader)
```

The `Subscription` struct has access to `s.client.config` directly.

---

**Call Site 2: `pkg/nats/client.go:312`**

Context: Wrapping request-reply responses

**Before:**
```go
return wrapNatsMessage(resp), nil
```

**After:**
```go
return wrapNatsMessage(resp, c.config.CorrelationHeader), nil
```

The `client` method has `c.config` in scope.

## Testing Strategy

**Update existing test in `pkg/nats/message_test.go:79`:**

The test currently calls `wrapNatsMessage(natsMsg)` directly. Update to pass the standard header name:

```go
func TestWrapNatsMessage(t *testing.T) {
    natsMsg := nats.NewMsg("test.subject")
    natsMsg.Data = []byte("test data")
    natsMsg.Header = nats.Header{}
    natsMsg.Header.Set("Correlation-Id", "test-correlation-123")
    natsMsg.Header.Set("Custom-Header", "custom-value")

    wrapped := wrapNatsMessage(natsMsg, "Correlation-Id")

    if wrapped.Subject != "test.subject" {
        t.Errorf("Subject = %q, want %q", wrapped.Subject, "test.subject")
    }
    // ... rest of test
}
```

**Add new test for custom header names:**

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

## Rationale

**Why parameter passing over method approach:**
- Explicit and functional - makes the dependency clear at call sites
- No coupling to client lifecycle
- Easy to test - just pass different header names
- Follows Go idioms for simple functions

**Why not support per-subscription headers:**
- YAGNI - no use case for different correlation headers per subscription
- Simpler implementation and clearer semantics
- Correlation IDs are client-wide by definition

## Implementation Files

- `pkg/nats/message.go` - Update function signature and implementation
- `pkg/nats/subscription.go` - Update call site to pass config value
- `pkg/nats/client.go` - Update call site to pass config value
- `pkg/nats/message_test.go` - Update and add test cases

## Verification

After implementation:
1. All existing tests pass
2. New test verifies custom header extraction
3. No change in behavior with default config (backward compatible)
4. Consistency between outbound header injection and inbound extraction
