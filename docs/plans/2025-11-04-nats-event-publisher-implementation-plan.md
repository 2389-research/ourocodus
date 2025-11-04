# NATS Event Publisher - Implementation Plan

**Design Document:** [2025-11-04-nats-event-publisher-design.md](./2025-11-04-nats-event-publisher-design.md)
**Issue:** [#38 - Relay NATS Event Publisher](https://github.com/2389-research/ourocodus/issues/38)
**Estimated Time:** 7-10 hours

## Prerequisites

- [ ] Design document reviewed and approved
- [ ] Local NATS server running (`docker-compose up -d`)
- [ ] Development branch created from `main`

## Phase 1: Core Infrastructure (2-3 hours)

### Task 1.1: Create EventPublisher Interface

**File:** `pkg/relay/events.go`

**Implementation:**

```go
package relay

import "context"

// EventPublisher publishes lifecycle events to external systems.
// Implementations must be safe for concurrent use and should not block.
type EventPublisher interface {
    // PublishSessionCreated publishes a session.created event.
    PublishSessionCreated(ctx context.Context, userSessionID string) error

    // PublishSessionTerminated publishes a session.terminated event.
    PublishSessionTerminated(ctx context.Context, userSessionID string) error

    // PublishAgentSpawned publishes an agent.spawned event.
    PublishAgentSpawned(ctx context.Context, userSessionID, agentID, workspace string) error

    // PublishAgentTerminated publishes an agent.terminated event.
    PublishAgentTerminated(ctx context.Context, userSessionID, agentID string, exitCode int) error

    // Close gracefully shuts down the publisher.
    Close() error
}

// NoOpPublisher is an EventPublisher that does nothing.
// Used when NATS is disabled or unavailable.
type NoOpPublisher struct{}

// NewNoOpPublisher creates a new no-op event publisher.
func NewNoOpPublisher() *NoOpPublisher {
    return &NoOpPublisher{}
}

func (n *NoOpPublisher) PublishSessionCreated(ctx context.Context, userSessionID string) error {
    return nil
}

func (n *NoOpPublisher) PublishSessionTerminated(ctx context.Context, userSessionID string) error {
    return nil
}

func (n *NoOpPublisher) PublishAgentSpawned(ctx context.Context, userSessionID, agentID, workspace string) error {
    return nil
}

func (n *NoOpPublisher) PublishAgentTerminated(ctx context.Context, userSessionID, agentID string, exitCode int) error {
    return nil
}

func (n *NoOpPublisher) Close() error {
    return nil
}
```

**Tests:** `pkg/relay/events_test.go`

```go
package relay_test

import (
    "context"
    "testing"

    "github.com/2389-research/ourocodus/pkg/relay"
    "github.com/stretchr/testify/assert"
)

func TestNoOpPublisher(t *testing.T) {
    pub := relay.NewNoOpPublisher()
    ctx := context.Background()

    // All methods should succeed and do nothing
    assert.NoError(t, pub.PublishSessionCreated(ctx, "test-session"))
    assert.NoError(t, pub.PublishSessionTerminated(ctx, "test-session"))
    assert.NoError(t, pub.PublishAgentSpawned(ctx, "test-session", "coder", "/workspace"))
    assert.NoError(t, pub.PublishAgentTerminated(ctx, "test-session", "coder", 0))
    assert.NoError(t, pub.Close())
}
```

**Verification:**
```bash
go test ./pkg/relay -run TestNoOpPublisher -v
```

---

### Task 1.2: Create Subject Builders

**File:** `pkg/relay/subjects.go`

**Implementation:**

```go
package relay

import (
    "fmt"
    "strings"
)

// SessionCreated returns the subject for session.created events.
func SessionCreated(userSessionID string) string {
    return fmt.Sprintf("sessions.%s.session.created", sanitizeID(userSessionID))
}

// SessionTerminated returns the subject for session.terminated events.
func SessionTerminated(userSessionID string) string {
    return fmt.Sprintf("sessions.%s.session.terminated", sanitizeID(userSessionID))
}

// AgentSpawned returns the subject for agent.spawned events.
func AgentSpawned(userSessionID string) string {
    return fmt.Sprintf("sessions.%s.agent.spawned", sanitizeID(userSessionID))
}

// AgentTerminated returns the subject for agent.terminated events.
func AgentTerminated(userSessionID string) string {
    return fmt.Sprintf("sessions.%s.agent.terminated", sanitizeID(userSessionID))
}

// sanitizeID sanitizes a session ID for use in NATS subjects.
// NATS uses dots as subject delimiters, so we replace them with underscores.
func sanitizeID(id string) string {
    // Replace dots with underscores
    sanitized := strings.ReplaceAll(id, ".", "_")

    // Validate length (NATS subjects have ~1KB limit, be conservative)
    if len(sanitized) > 200 {
        panic(fmt.Sprintf("session ID too long for NATS subject: %d chars", len(sanitized)))
    }

    return sanitized
}
```

**Tests:** `pkg/relay/subjects_test.go`

```go
package relay_test

import (
    "strings"
    "testing"

    "github.com/2389-research/ourocodus/pkg/relay"
    "github.com/stretchr/testify/assert"
)

func TestSessionCreated(t *testing.T) {
    tests := []struct {
        name      string
        userSessionID string
        want      string
    }{
        {
            name:      "normal session ID",
            userSessionID: "sess-abc123",
            want:      "sessions.sess-abc123.session.created",
        },
        {
            name:      "session ID with dots",
            userSessionID: "sess.abc.123",
            want:      "sessions.sess_abc_123.session.created",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := relay.SessionCreated(tt.userSessionID)
            assert.Equal(t, tt.want, got)
        })
    }
}

func TestSessionTerminated(t *testing.T) {
    got := relay.SessionTerminated("test-session")
    assert.Equal(t, "sessions.test-session.session.terminated", got)
}

func TestAgentSpawned(t *testing.T) {
    got := relay.AgentSpawned("test-session")
    assert.Equal(t, "sessions.test-session.agent.spawned", got)
}

func TestAgentTerminated(t *testing.T) {
    got := relay.AgentTerminated("test-session")
    assert.Equal(t, "sessions.test-session.agent.terminated", got)
}

func TestSanitizeID_TooLong(t *testing.T) {
    // Create a session ID longer than 200 chars
    longID := strings.Repeat("a", 201)

    assert.Panics(t, func() {
        relay.SessionCreated(longID)
    })
}
```

**Verification:**
```bash
go test ./pkg/relay -run TestSession -v
go test ./pkg/relay -run TestAgent -v
```

---

### Task 1.3: Create NATSEventPublisher

**File:** `pkg/relay/nats_publisher.go`

**Implementation:**

```go
package relay

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "sync/atomic"
    "time"

    "github.com/2389-research/ourocodus/pkg/nats"
)

// NATSEventPublisher publishes events to NATS.
type NATSEventPublisher struct {
    client     nats.Client
    idGen      IDGenerator
    clock      Clock
    logger     Logger
    eventIndex sync.Map // map[userSessionID]*atomic.Int64
}

// NewNATSEventPublisher creates a new NATS event publisher.
func NewNATSEventPublisher(client nats.Client, idGen IDGenerator, clock Clock, logger Logger) *NATSEventPublisher {
    return &NATSEventPublisher{
        client: client,
        idGen:  idGen,
        clock:  clock,
        logger: logger,
    }
}

// PublishSessionCreated publishes a session.created event.
func (p *NATSEventPublisher) PublishSessionCreated(ctx context.Context, userSessionID string) error {
    payload := map[string]interface{}{
        "userSessionId": userSessionID,
        "createdAt": p.clock.Now().Format(time.RFC3339Nano),
    }

    return p.publish(ctx, SessionCreated(userSessionID), "session.created", userSessionID, payload)
}

// PublishSessionTerminated publishes a session.terminated event.
func (p *NATSEventPublisher) PublishSessionTerminated(ctx context.Context, userSessionID string) error {
    payload := map[string]interface{}{
        "userSessionId":     userSessionID,
        "terminatedAt": p.clock.Now().Format(time.RFC3339Nano),
    }

    return p.publish(ctx, SessionTerminated(userSessionID), "session.terminated", userSessionID, payload)
}

// PublishAgentSpawned publishes an agent.spawned event.
func (p *NATSEventPublisher) PublishAgentSpawned(ctx context.Context, userSessionID, agentID, workspace string) error {
    payload := map[string]interface{}{
        "userSessionId": userSessionID,
        "agentId":  agentID,
        "workspace": workspace,
        "spawnedAt": p.clock.Now().Format(time.RFC3339Nano),
    }

    return p.publish(ctx, AgentSpawned(userSessionID), "agent.spawned", userSessionID, payload)
}

// PublishAgentTerminated publishes an agent.terminated event.
func (p *NATSEventPublisher) PublishAgentTerminated(ctx context.Context, userSessionID, agentID string, exitCode int) error {
    payload := map[string]interface{}{
        "userSessionId":     userSessionID,
        "agentId":      agentID,
        "terminatedAt": p.clock.Now().Format(time.RFC3339Nano),
        "exitCode":      exitCode,
    }

    return p.publish(ctx, AgentTerminated(userSessionID), "agent.terminated", userSessionID, payload)
}

// Close gracefully shuts down the publisher.
func (p *NATSEventPublisher) Close() error {
    return p.client.Close()
}

// publish publishes an event to NATS.
func (p *NATSEventPublisher) publish(ctx context.Context, subject, eventType, userSessionID string, payload interface{}) error {
    now := p.clock.Now()

    event := map[string]interface{}{
        "version":     "1.0",
        "messageId":   p.idGen.Generate(),
        "eventIndex":  p.nextEventIndex(userSessionID),
        "occurredAt":  now.Format(time.RFC3339Nano),
        "publishedAt": now.Format(time.RFC3339Nano),
        "type":        eventType,
        "payload":     payload,
    }

    // Add correlationId from context if present
    if correlationID := ctx.Value("correlationId"); correlationID != nil {
        event["correlationId"] = correlationID
    }

    data, err := json.Marshal(event)
    if err != nil {
        p.logger.Printf("ERROR: Failed to marshal event %s: %v", eventType, err)
        return fmt.Errorf("marshal event: %w", err)
    }

    // Publish to NATS (pkg/nats.Client handles retry/backoff/metrics)
    if err := p.client.Publish(ctx, subject, data); err != nil {
        p.logger.Printf("ERROR: Failed to publish %s event for session %s: %v", eventType, userSessionID, err)
        return err
    }

    return nil
}

// nextEventIndex returns the next event index for a session.
func (p *NATSEventPublisher) nextEventIndex(userSessionID string) int64 {
    // Load or create atomic counter for this session
    val, _ := p.eventIndex.LoadOrStore(userSessionID, &atomic.Int64{})
    counter := val.(*atomic.Int64)

    // Increment and return
    return counter.Add(1) - 1 // Return pre-increment value (0-based)
}
```

**Tests:** `pkg/relay/nats_publisher_test.go`

```go
package relay_test

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/2389-research/ourocodus/pkg/relay"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Mock NATS client for testing
type mockNATSClient struct {
    publishes []mockPublish
}

type mockPublish struct {
    subject string
    data    []byte
}

func (m *mockNATSClient) Publish(ctx context.Context, subject string, data []byte) error {
    m.publishes = append(m.publishes, mockPublish{subject: subject, data: data})
    return nil
}

func (m *mockNATSClient) Close() error {
    return nil
}

// Implement other nats.Client methods as no-ops...

func TestNATSEventPublisher_SessionCreated(t *testing.T) {
    mockClient := &mockNATSClient{}
    idGen := &relay.MockIDGenerator{ID: "test-message-id"}
    clock := &relay.MockClock{Time: time.Date(2025, 11, 4, 10, 0, 0, 0, time.UTC)}
    logger := &relay.MockLogger{}

    publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

    err := publisher.PublishSessionCreated(context.Background(), "test-session")
    require.NoError(t, err)

    // Verify publish was called
    require.Len(t, mockClient.publishes, 1)
    publish := mockClient.publishes[0]

    // Verify subject
    assert.Equal(t, "sessions.test-session.session.created", publish.subject)

    // Verify payload structure
    var event map[string]interface{}
    err = json.Unmarshal(publish.data, &event)
    require.NoError(t, err)

    assert.Equal(t, "1.0", event["version"])
    assert.Equal(t, "test-message-id", event["messageId"])
    assert.Equal(t, float64(0), event["eventIndex"]) // First event
    assert.Equal(t, "session.created", event["type"])

    payload := event["payload"].(map[string]interface{})
    assert.Equal(t, "test-session", payload["userSessionId"])
}

func TestNATSEventPublisher_EventIndexIncrement(t *testing.T) {
    mockClient := &mockNATSClient{}
    idGen := &relay.MockIDGenerator{ID: "test-id"}
    clock := &relay.MockClock{Time: time.Now()}
    logger := &relay.MockLogger{}

    publisher := relay.NewNATSEventPublisher(mockClient, idGen, clock, logger)

    // Publish multiple events for same session
    publisher.PublishSessionCreated(context.Background(), "test-session")
    publisher.PublishAgentSpawned(context.Background(), "test-session", "coder", "/workspace")
    publisher.PublishAgentTerminated(context.Background(), "test-session", "coder", 0)

    // Verify event indices increment
    for i, publish := range mockClient.publishes {
        var event map[string]interface{}
        json.Unmarshal(publish.data, &event)
        assert.Equal(t, float64(i), event["eventIndex"])
    }
}
```

**Verification:**
```bash
go test ./pkg/relay -run TestNATSEventPublisher -v
```

---

## Phase 2: SessionManager Integration (1-2 hours)

### Task 2.1: Add EventPublisher to SessionManager

**File:** `pkg/relay/session/manager.go`

**Changes:**

```go
// Add field to Manager struct
type Manager struct {
    // ... existing fields ...
    publisher EventPublisher  // NEW: optional event publisher
}

// Update NewManager signature
func NewManager(logger Logger, clock Clock, idGen IDGenerator, publisher EventPublisher) (*Manager, error) {
    return &Manager{
        // ... existing initialization ...
        publisher: publisher,
    }, nil
}
```

**Note:** Need to import `EventPublisher` from `pkg/relay`:
```go
import "github.com/2389-research/ourocodus/pkg/relay"
```

But this creates a circular dependency! SessionManager is in `pkg/relay/session`, and EventPublisher is in `pkg/relay`.

**Resolution:** Move `EventPublisher` interface to `pkg/relay/session/events.go` instead, or define it in a shared package.

**Better approach:** Define interface in `pkg/relay/session/publisher.go`:

```go
package session

import "context"

// EventPublisher publishes session lifecycle events.
type EventPublisher interface {
    PublishSessionCreated(ctx context.Context, userSessionID string) error
    PublishSessionTerminated(ctx context.Context, userSessionID string) error
    PublishAgentSpawned(ctx context.Context, userSessionID, agentID, workspace string) error
    PublishAgentTerminated(ctx context.Context, userSessionID, agentID string, exitCode int) error
}
```

Then `pkg/relay/nats_publisher.go` implements `session.EventPublisher`.

---

### Task 2.2: Add Publish Calls

**File:** `pkg/relay/session/manager.go`

**In CreateUserSession method:**
```go
func (m *Manager) CreateUserSession(ctx context.Context) (string, error) {
    // ... existing session creation code ...

    // Publish event (non-blocking)
    if m.publisher != nil {
        go func() {
            if err := m.publisher.PublishSessionCreated(context.Background(), userSessionID); err != nil {
                m.logger.Printf("Failed to publish session.created: %v", err)
            }
        }()
    }

    return userSessionID, nil
}
```

**In TerminateUserSession method:**
```go
func (m *Manager) TerminateUserSession(ctx context.Context, userSessionID string) error {
    // ... existing termination code ...

    // Publish event (non-blocking)
    if m.publisher != nil {
        go func() {
            if err := m.publisher.PublishSessionTerminated(context.Background(), userSessionID); err != nil {
                m.logger.Printf("Failed to publish session.terminated: %v", err)
            }
        }()
    }

    return nil
}
```

**In SpawnAgent method:**
```go
func (m *Manager) SpawnAgent(ctx context.Context, userSessionID, agentID, workspace string) error {
    // ... existing spawn code ...

    // Publish event (non-blocking)
    if m.publisher != nil {
        go func() {
            if err := m.publisher.PublishAgentSpawned(context.Background(), userSessionID, agentID, workspace); err != nil {
                m.logger.Printf("Failed to publish agent.spawned: %v", err)
            }
        }()
    }

    return nil
}
```

**In TerminateAgent method:**
```go
func (m *Manager) TerminateAgent(ctx context.Context, userSessionID, agentID string) error {
    // ... existing termination code ...

    // Get exit code before cleanup
    agent, err := m.GetAgent(userSessionID, agentID)
    exitCode := 0
    if agent != nil && agent.ExitCode != nil {
        exitCode = *agent.ExitCode
    }

    // ... cleanup code ...

    // Publish event (non-blocking)
    if m.publisher != nil {
        go func() {
            if err := m.publisher.PublishAgentTerminated(context.Background(), userSessionID, agentID, exitCode); err != nil {
                m.logger.Printf("Failed to publish agent.terminated: %v", err)
            }
        }()
    }

    return nil
}
```

---

### Task 2.3: Update Tests

**File:** `pkg/relay/session/manager_test.go`

Add tests with mock publisher:

```go
type mockPublisher struct {
    calls []string
    mu    sync.Mutex
}

func (m *mockPublisher) PublishSessionCreated(ctx context.Context, userSessionID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = append(m.calls, fmt.Sprintf("session.created:%s", userSessionID))
    return nil
}

// ... implement other methods ...

func TestManager_CreateSession_PublishesEvent(t *testing.T) {
    mockPub := &mockPublisher{}
    manager, _ := NewManager(logger, clock, idGen, mockPub)

    userSessionID, err := manager.CreateUserSession(context.Background())
    require.NoError(t, err)

    // Wait for goroutine (use sync or sleep)
    time.Sleep(10 * time.Millisecond)

    // Verify event published
    assert.Contains(t, mockPub.calls, fmt.Sprintf("session.created:%s", userSessionID))
}
```

**Verification:**
```bash
go test ./pkg/relay/session -v
```

---

## Phase 3: main.go Wiring (1 hour)

### Task 3.1: Initialize NATS Client and Publisher

**File:** `cmd/relay/main.go`

**Add imports:**
```go
import (
    "os"
    "github.com/2389-research/ourocodus/pkg/nats"
    "github.com/2389-research/ourocodus/pkg/relay"
)
```

**Add publisher initialization after dependency creation:**

```go
func main() {
    // Create dependencies
    logger := &relay.StdLogger{}
    clock := &relay.SystemClock{}
    idGen := &relay.UUIDGenerator{}

    // Initialize event publisher
    var publisher session.EventPublisher
    if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
        logger.Printf("Connecting to NATS at %s", natsURL)

        natsClient, err := nats.NewClient(
            nats.WithURL(natsURL),
            nats.WithName("ourocodus-relay"),
        )

        if err != nil {
            logger.Printf("WARN: NATS connection failed, events will not be published: %v", err)
            publisher = relay.NewNoOpPublisher()
        } else {
            logger.Printf("NATS connected successfully")
            publisher = relay.NewNATSEventPublisher(natsClient, idGen, clock, logger)

            // Ensure graceful drain on shutdown
            defer func() {
                logger.Println("Draining NATS connection...")
                if err := natsClient.Close(); err != nil {
                    logger.Printf("NATS drain error: %v", err)
                }
            }()
        }
    } else {
        logger.Printf("NATS_URL not set, events will not be published")
        publisher = relay.NewNoOpPublisher()
    }

    // Create session manager with publisher
    sessionManager, err := session.NewManager(logger, clock, idGen, publisher)
    if err != nil {
        log.Fatalf("Failed to create session manager: %v", err)
    }

    // ... rest of main.go ...
}
```

**Verification:**
```bash
# Without NATS
./relay
# Should see: "NATS_URL not set, events will not be published"

# With NATS
NATS_URL=nats://localhost:4222 ./relay
# Should see: "NATS connected successfully"

# With invalid NATS
NATS_URL=nats://invalid:4222 ./relay
# Should see: "WARN: NATS connection failed..."
```

---

## Phase 4: Testing (2-3 hours)

### Task 4.1: Integration Test with Real NATS

**File:** `pkg/relay/nats_integration_test.go`

**Implementation:**

```go
// +build integration

package relay_test

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/2389-research/ourocodus/pkg/nats"
    "github.com/2389-research/ourocodus/pkg/relay"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNATSEventPublisher_Integration(t *testing.T) {
    // Connect to local NATS (assumes docker-compose running)
    natsClient, err := nats.NewClient(
        nats.WithURL("nats://localhost:4222"),
        nats.WithName("integration-test"),
    )
    require.NoError(t, err)
    defer natsClient.Close()

    // Create publisher
    idGen := &relay.UUIDGenerator{}
    clock := &relay.SystemClock{}
    logger := &relay.StdLogger{}
    publisher := relay.NewNATSEventPublisher(natsClient, idGen, clock, logger)

    // Subscribe to all session events
    msgs := make(chan *nats.Msg, 10)
    sub, err := natsClient.Subscribe("sessions.>", func(msg *nats.Msg) {
        msgs <- msg
    })
    require.NoError(t, err)
    defer sub.Unsubscribe()

    // Publish test event
    testSessionID := "integration-test-" + idGen.Generate()
    err = publisher.PublishSessionCreated(context.Background(), testSessionID)
    require.NoError(t, err)

    // Wait for event
    select {
    case msg := <-msgs:
        // Verify subject
        expectedSubject := "sessions." + testSessionID + ".session.created"
        assert.Equal(t, expectedSubject, msg.Subject)

        // Verify payload
        var event map[string]interface{}
        err := json.Unmarshal(msg.Data, &event)
        require.NoError(t, err)

        assert.Equal(t, "1.0", event["version"])
        assert.NotEmpty(t, event["messageId"])
        assert.Equal(t, float64(0), event["eventIndex"])
        assert.Equal(t, "session.created", event["type"])

        payload := event["payload"].(map[string]interface{})
        assert.Equal(t, testSessionID, payload["userSessionId"])

    case <-time.After(5 * time.Second):
        t.Fatal("Timeout waiting for event")
    }
}

func TestNATSEventPublisher_Integration_AllEvents(t *testing.T) {
    // Similar test but verifies all 4 event types
    // ... implementation ...
}
```

**Run with:**
```bash
# Start NATS first
docker-compose up -d

# Run integration tests
go test ./pkg/relay -tags=integration -v
```

---

### Task 4.2: End-to-End Test

**File:** `pkg/relay/e2e_test.go`

```go
// +build integration

package relay_test

import (
    "context"
    "testing"
    "time"

    "github.com/2389-research/ourocodus/pkg/nats"
    "github.com/2389-research/ourocodus/pkg/relay"
    "github.com/2389-research/ourocodus/pkg/relay/session"
    "github.com/stretchr/testify/require"
)

func TestEndToEnd_SessionLifecycle(t *testing.T) {
    // Setup NATS
    natsClient, err := nats.NewClient(nats.WithURL("nats://localhost:4222"))
    require.NoError(t, err)
    defer natsClient.Close()

    // Create publisher and session manager
    idGen := &relay.UUIDGenerator{}
    clock := &relay.SystemClock{}
    logger := &relay.StdLogger{}
    publisher := relay.NewNATSEventPublisher(natsClient, idGen, clock, logger)

    manager, err := session.NewManager(logger, clock, idGen, publisher)
    require.NoError(t, err)

    // Subscribe to events
    events := make(chan string, 10)
    natsClient.Subscribe("sessions.>", func(msg *nats.Msg) {
        events <- msg.Subject
    })

    // Create session
    userSessionID, err := manager.CreateUserSession(context.Background())
    require.NoError(t, err)

    // Verify session.created event
    select {
    case subject := <-events:
        require.Contains(t, subject, "session.created")
    case <-time.After(2 * time.Second):
        t.Fatal("Timeout waiting for session.created")
    }

    // Spawn agent
    err = manager.SpawnAgent(context.Background(), userSessionID, "test-agent", "/workspace")
    require.NoError(t, err)

    // Verify agent.spawned event
    select {
    case subject := <-events:
        require.Contains(t, subject, "agent.spawned")
    case <-time.After(2 * time.Second):
        t.Fatal("Timeout waiting for agent.spawned")
    }

    // Terminate agent
    err = manager.TerminateAgent(context.Background(), userSessionID, "test-agent")
    require.NoError(t, err)

    // Verify agent.terminated event
    select {
    case subject := <-events:
        require.Contains(t, subject, "agent.terminated")
    case <-time.After(2 * time.Second):
        t.Fatal("Timeout waiting for agent.terminated")
    }

    // Terminate session
    err = manager.TerminateUserSession(context.Background(), userSessionID)
    require.NoError(t, err)

    // Verify session.terminated event
    select {
    case subject := <-events:
        require.Contains(t, subject, "session.terminated")
    case <-time.After(2 * time.Second):
        t.Fatal("Timeout waiting for session.terminated")
    }
}
```

**Verification:**
```bash
go test ./pkg/relay -tags=integration -run TestEndToEnd -v
```

---

## Phase 5: Documentation & Cleanup (1 hour)

### Task 5.1: Update Documentation

**File:** `README.md`

Add section on NATS configuration:

```markdown
## Event Publishing

The relay can publish session and agent lifecycle events to NATS for observability and coordination.

### Configuration

Set the `NATS_URL` environment variable to enable event publishing:

```bash
export NATS_URL=nats://localhost:4222
./relay
```

If `NATS_URL` is not set, the relay operates normally without publishing events.

### Events Published

- `sessions.{sessionId}.session.created` - Session created
- `sessions.{sessionId}.session.terminated` - Session terminated
- `sessions.{sessionId}.agent.spawned` - Agent spawned
- `sessions.{sessionId}.agent.terminated` - Agent terminated

### Subscribing to Events

Use the NATS CLI to inspect events:

```bash
# All events
nats sub "sessions.>"

# Session events only
nats sub "sessions.*.session.*"

# Agent events only
nats sub "sessions.*.agent.*"
```
```

---

### Task 5.2: Update NATS.md

**File:** `docs/NATS.md`

Add "Implementation Status" section:

```markdown
## Implementation Status

### ✅ Completed: Step 1 - Parallel Run (Issue #38)

**Status:** Implemented (PR #XXX)

The relay now publishes lifecycle events to NATS while maintaining full backward compatibility:

- Session events (`created`, `terminated`)
- Agent events (`spawned`, `terminated`)
- Optional via `NATS_URL` environment variable
- Best-effort delivery with retry (3 attempts, exponential backoff)
- Fire-and-forget (non-blocking)

**Architecture:**
- Observer pattern with `EventPublisher` interface
- `NATSEventPublisher` wraps `pkg/nats.Client`
- `NoOpPublisher` for graceful degradation
- Type-specific subjects: `sessions.{id}.{type}`

**Testing:**
- Unit tests with mock publisher
- Integration tests with real NATS
- E2E test covering full session lifecycle

**Next Steps:** Add coordinator in observer mode (Step 2)
```

---

### Task 5.3: Add Inline Comments

Review all new code and ensure:
- [ ] Public functions have godoc comments
- [ ] Complex logic has explanatory comments
- [ ] TODOs for future improvements are tagged

---

## Verification Checklist

### Manual Testing

- [ ] Start relay without NATS_URL → no warnings, works normally
- [ ] Start relay with valid NATS_URL → connects successfully
- [ ] Start relay with invalid NATS_URL → warning logged, continues with NoOp
- [ ] Create session → verify `session.created` event published
- [ ] Spawn agent → verify `agent.spawned` event published
- [ ] Terminate agent → verify `agent.terminated` event published
- [ ] Terminate session → verify `session.terminated` event published
- [ ] Stop NATS server during runtime → relay continues, reconnects when NATS restarts

### Automated Tests

- [ ] All unit tests pass: `go test ./pkg/relay/... -v`
- [ ] All integration tests pass: `go test ./pkg/relay/... -tags=integration -v`
- [ ] No race conditions: `go test ./pkg/relay/... -race`
- [ ] Code coverage > 80%: `go test ./pkg/relay/... -cover`

### Code Quality

- [ ] `make lint` passes (no golangci-lint errors)
- [ ] `make fmt` applied (gofumpt formatting)
- [ ] `go vet ./...` passes
- [ ] No new dependencies added (reused existing pkg/nats)

### Documentation

- [ ] Design doc complete and reviewed
- [ ] README updated with NATS configuration
- [ ] NATS.md updated with implementation status
- [ ] Inline godoc comments added

## Rollback Plan

If issues are discovered after merge:

1. **Disable NATS in production:**
   ```bash
   unset NATS_URL
   # OR
   export NATS_URL=""
   ```
   Relay will use NoOpPublisher automatically.

2. **Revert PR:**
   ```bash
   git revert <commit-sha>
   git push origin main
   ```

3. **Remove NATS wiring but keep interface:**
   - Change main.go to always use NoOpPublisher
   - Keep EventPublisher interface for future retry

## Success Metrics

- [ ] Zero impact to existing WebSocket functionality
- [ ] Event publishing latency < 10ms (p99)
- [ ] Event delivery rate > 99.9% (best-effort)
- [ ] No relay crashes or panics related to NATS
- [ ] Clean shutdown (graceful drain within 30s)

---

**Plan Version:** 1.0
**Last Updated:** 2025-11-04
**Estimated Total Time:** 7-10 hours
**Actual Time:** _TBD after implementation_
