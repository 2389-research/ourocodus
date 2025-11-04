# NATS Event Publisher Design (Issue #38)

**Date:** 2025-11-04
**Status:** Approved Design
**Issue:** [#38 - Relay NATS Event Publisher](https://github.com/2389-research/ourocodus/issues/38)
**Phase:** Phase 2 - Step 1 (Parallel Run)

## Executive Summary

Integrate NATS event publishing into the relay service to broadcast session and agent lifecycle events without affecting existing WebSocket functionality. This is Step 1 of the NATS migration strategy: running NATS in parallel with WebSocket to prove the integration works.

**Goals:**
- Publish session and agent lifecycle events to NATS
- Maintain 100% backward compatibility (WebSocket continues working)
- Graceful degradation when NATS unavailable
- Minimize new code by maximizing reuse of existing infrastructure

**Non-Goals (Deferred):**
- JetStream persistence (use Core NATS for MVP)
- Guaranteed delivery semantics
- Event replay capabilities
- Complex observability dashboards

## Requirements

### Functional Requirements

1. **Event Publishing:**
   - Publish `session.created` when session created
   - Publish `session.terminated` when session terminated
   - Publish `agent.spawned` when agent spawned
   - Publish `agent.terminated` when agent terminated

2. **Configuration:**
   - Optional via `NATS_URL` environment variable
   - No NATS_URL → relay works normally without events
   - NATS connection fails → relay continues with degraded mode

3. **Error Handling:**
   - Best-effort retry (3 attempts with exponential backoff)
   - Publishing failures logged but never block relay operations
   - Automatic reconnection on NATS server issues

### Non-Functional Requirements

1. **Performance:** Publishing must be async and non-blocking
2. **Reliability:** Best-effort delivery acceptable (Core NATS)
3. **Observability:** Leverage existing pkg/nats metrics
4. **Testing:** Unit tests + basic integration test with real NATS
5. **Maintainability:** Reuse existing components, minimal new code

## Architecture

### Design Pattern: Observer Pattern

The SessionManager accepts an optional `EventPublisher` interface and calls it after successful state changes. This decouples NATS publishing from core relay logic.

```
┌─────────────────┐
│ SessionManager  │
│                 │
│  CreateSession()│──┐
│  SpawnAgent()   │  │ (after state change)
│  Terminate...() │  │
└─────────────────┘  │
                     ▼
              ┌──────────────┐
              │EventPublisher│ (interface)
              └──────┬───────┘
                     │
        ┌────────────┴────────────┐
        │                         │
   ┌────▼────────┐     ┌─────────▼──────┐
   │NoOpPublisher│     │NATSEventPublisher│
   └─────────────┘     └──────┬──────────┘
                              │
                        ┌─────▼──────┐
                        │pkg/nats    │
                        │  Client    │
                        └────────────┘
```

### Components

#### 1. EventPublisher Interface (`pkg/relay/events.go`)

```go
// EventPublisher publishes lifecycle events to external systems.
type EventPublisher interface {
    PublishSessionCreated(ctx context.Context, userSessionID string) error
    PublishSessionTerminated(ctx context.Context, userSessionID string) error
    PublishAgentSpawned(ctx context.Context, userSessionID, agentID, workspace string) error
    PublishAgentTerminated(ctx context.Context, userSessionID, agentID string, exitCode int) error
    Close() error
}
```

#### 2. NoOpPublisher (`pkg/relay/events.go`)

Null object pattern - does nothing. Used when NATS disabled or connection fails.

```go
type NoOpPublisher struct{}

func (n *NoOpPublisher) PublishSessionCreated(ctx context.Context, userSessionID string) error {
    return nil
}
// ... other methods
```

#### 3. NATSEventPublisher (`pkg/relay/nats_publisher.go`)

Thin wrapper over `pkg/nats.Client` that:
- Constructs event payloads with proper schema
- Uses centralized subject builders
- Tracks per-session event sequence numbers
- Delegates retry/backoff/metrics to pkg/nats

```go
type NATSEventPublisher struct {
    client     nats.Client
    idGen      IDGenerator  // Reuse relay's UUID generator
    clock      Clock        // Reuse relay's clock
    logger     Logger       // Reuse relay's logger
    eventIndex sync.Map     // map[sessionID]*atomic.Int64
}
```

#### 4. Subject Builders (`pkg/relay/subjects.go`)

Centralized subject construction to prevent typos and enforce consistency.

```go
func SessionCreated(userSessionID string) string {
    return fmt.Sprintf("sessions.%s.session.created", sanitizeID(sessionID))
}

func SessionTerminated(userSessionID string) string {
    return fmt.Sprintf("sessions.%s.session.terminated", sanitizeID(sessionID))
}

func AgentSpawned(userSessionID string) string {
    return fmt.Sprintf("sessions.%s.agent.spawned", sanitizeID(sessionID))
}

func AgentTerminated(userSessionID string) string {
    return fmt.Sprintf("sessions.%s.agent.terminated", sanitizeID(sessionID))
}

func sanitizeID(id string) string {
    // Replace dots with underscores (NATS subject delimiter)
    // Validate length < 200 chars
    // Panic if invalid (fail fast in dev/test)
    return strings.ReplaceAll(id, ".", "_")
}
```

## Event Schema

### Topic Structure

Type-specific subjects for better filtering:

```
sessions.{sessionId}.session.created
sessions.{sessionId}.session.terminated
sessions.{sessionId}.agent.spawned
sessions.{sessionId}.agent.terminated
```

**Rationale:** Enables consumers to subscribe to:
- `sessions.*.session.created` - All session creations
- `sessions.{id}.*` - All events for one session
- `sessions.>` - Everything

### Event Payload Format

All events follow a consistent versioned schema:

```json
{
  "version": "1.0",
  "messageId": "msg_abc123",
  "correlationId": "req_xyz789",
  "eventIndex": 42,
  "occurredAt": "2025-11-04T10:30:00.123Z",
  "publishedAt": "2025-11-04T10:30:00.456Z",
  "type": "session.created",
  "payload": {
    "userSessionId": "sess_xyz789",
    "createdAt": "2025-11-04T10:30:00.123Z"
  }
}
```

**Field Descriptions:**

- `version`: Schema version (enables future evolution)
- `messageId`: Unique event identifier (UUID from IDGenerator)
- `correlationId`: Request trace ID (optional, from context)
- `eventIndex`: Monotonic counter per session (0-based, enables gap detection)
- `occurredAt`: When state change happened (from Clock)
- `publishedAt`: When event published to NATS (from Clock)
- `type`: Event type (matches subject suffix)
- `payload`: Event-specific data

### Event-Specific Payloads

#### session.created
```json
{
  "userSessionId": "sess_abc123",
  "createdAt": "2025-11-04T10:30:00.123Z"
}
```

#### session.terminated
```json
{
  "userSessionId": "sess_abc123",
  "terminatedAt": "2025-11-04T10:35:00.123Z",
  "reason": "user_requested"
}
```

#### agent.spawned
```json
{
  "userSessionId": "sess_abc123",
  "agentId": "coder",
  "workspace": "/workspaces/sess_abc123/coder",
  "spawnedAt": "2025-11-04T10:30:05.123Z"
}
```

#### agent.terminated
```json
{
  "userSessionId": "sess_abc123",
  "agentId": "coder",
  "terminatedAt": "2025-11-04T10:35:00.123Z",
  "exitCode": 0
}
```

## Integration with SessionManager

### Minimal Modifications

SessionManager accepts optional EventPublisher in constructor:

```go
func NewManager(logger Logger, clock Clock, idGen IDGenerator, publisher EventPublisher) (*Manager, error) {
    return &Manager{
        // ... existing fields
        publisher: publisher,  // Can be nil
    }, nil
}
```

### Publishing Call Sites

After each successful state change, publish event in goroutine:

**1. CreateUserSession:**
```go
session := &Session{...}
m.sessions[sessionID] = session

// Publish event (non-blocking)
if m.publisher != nil {
    go m.publisher.PublishSessionCreated(context.Background(), sessionID)
}

return sessionID, nil
```

**2. TerminateUserSession:**
```go
delete(m.sessions, sessionID)

if m.publisher != nil {
    go m.publisher.PublishSessionTerminated(context.Background(), sessionID)
}
```

**3. SpawnAgent:**
```go
agent := &Agent{...}
session.agents[role] = agent

if m.publisher != nil {
    go m.publisher.PublishAgentSpawned(context.Background(), sessionID, agentID, workspace)
}
```

**4. TerminateAgent:**
```go
delete(session.agents, role)

if m.publisher != nil {
    go m.publisher.PublishAgentTerminated(context.Background(), sessionID, agentID, exitCode)
}
```

**Key Design Decisions:**
- Goroutines ensure publishing never blocks SessionManager
- `context.Background()` makes publishing independent of request lifecycle
- Nil checks maintain backward compatibility
- Publish after state mutation ensures consistency

## Wiring in main.go

```go
func main() {
    logger := &relay.StdLogger{}
    clock := &relay.SystemClock{}
    idGen := &relay.UUIDGenerator{}

    // Initialize event publisher
    var publisher relay.EventPublisher
    if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
        natsClient, err := nats.NewClient(
            nats.WithURL(natsURL),
            nats.WithName("ourocodus-relay"),
        )
        if err != nil {
            logger.Printf("WARN: NATS disabled, events will not be published: %v", err)
            publisher = relay.NewNoOpPublisher()
        } else {
            publisher = relay.NewNATSEventPublisher(natsClient, idGen, clock, logger)
            defer natsClient.Close()  // Graceful drain on shutdown
        }
    } else {
        publisher = relay.NewNoOpPublisher()
    }

    // Create session manager with publisher
    sessionManager, err := session.NewManager(logger, clock, idGen, publisher)
    if err != nil {
        log.Fatalf("Failed to create session manager: %v", err)
    }

    // ... rest of setup
}
```

## Leveraging Existing Infrastructure

**Reused Components (NO new implementation needed):**

1. **pkg/nats.Client** - Already provides:
   - ✅ Exponential backoff with jitter (`RetryBackoff`)
   - ✅ Retry logic (3 attempts by default)
   - ✅ Correlation ID handling
   - ✅ Metrics collection (Prometheus)
   - ✅ Health tracking
   - ✅ Graceful drain on shutdown
   - ✅ Auto-reconnect on disconnect

2. **relay.IDGenerator** - UUID generation for messageId

3. **relay.Clock** - Timestamp generation for occurredAt/publishedAt

4. **relay.Logger** - Logging interface

**Configuration (pkg/nats defaults):**
- RetryAttempts: 3
- RetryBackoff: exponential (200ms initial, 5s max) + 25% jitter
- ReconnectBufSize: 8MB
- ReconnectWait: 2s
- MaxReconnects: -1 (infinite)
- DrainTimeout: 30s

## Error Handling

### Publishing Failures

1. **Transient errors** (connection loss, buffer full):
   - pkg/nats.Client automatically retries with backoff
   - After 3 attempts, logs error and returns
   - NATSEventPublisher logs but doesn't propagate to caller

2. **Permanent errors** (invalid subject, auth failure):
   - pkg/nats.Client detects non-retryable errors
   - Returns immediately without retry
   - NATSEventPublisher logs error

3. **NATS unavailable at startup:**
   - Log warning: "NATS disabled, events will not be published"
   - Fall back to NoOpPublisher
   - Relay continues normally

### Graceful Degradation

| Scenario | Behavior |
|----------|----------|
| NATS_URL not set | NoOpPublisher (zero overhead) |
| NATS connection fails at startup | NoOpPublisher + warning log |
| NATS disconnects during runtime | pkg/nats auto-reconnects, events buffered |
| Reconnect buffer overflows | pkg/nats drops oldest, metrics incremented |

### Logging Strategy

```go
// Startup
logger.Printf("WARN: NATS disabled, events will not be published: %v", err)

// Publish errors (sampled, not every failure)
logger.Printf("ERROR: Failed to publish %s event for session %s after %d attempts: %v",
    eventType, sessionID, attempts, err)
```

## Observability

### Metrics (from pkg/nats)

Already exposed by pkg/nats.Client:

```
nats_client_publish_total{subject}              # Total publishes attempted
nats_client_publish_errors_total{subject,type}  # Failed publishes by error type
nats_client_publish_duration_seconds{subject}   # Publish latency histogram
nats_client_connected                           # Connection status (0/1)
nats_client_reconnects_total                    # Reconnection count
```

**Example queries:**
```promql
# Publish rate by subject
rate(nats_client_publish_total[5m])

# Error rate by subject
rate(nats_client_publish_errors_total[5m])

# Publish latency p99
histogram_quantile(0.99, rate(nats_client_publish_duration_seconds_bucket[5m]))
```

### Additional Metrics (if needed)

```go
// In NATSEventPublisher
eventIndexGauge = prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "relay_event_index",
        Help: "Current event index per session",
    },
    []string{"session_id"},
)
```

### Health Checks

```go
func (p *NATSEventPublisher) Health() error {
    return p.client.Health()  // Returns error if disconnected
}
```

## Testing Strategy

### Unit Tests (Mock NATS)

**1. Mock EventPublisher:**
```go
type MockEventPublisher struct {
    calls []PublishCall
    mu    sync.Mutex
}

func (m *MockEventPublisher) PublishSessionCreated(ctx, sessionID) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = append(m.calls, PublishCall{Type: "session.created", SessionID: sessionID})
    return nil
}
```

**2. SessionManager Tests:**
```go
func TestCreateUserSession_PublishesEvent(t *testing.T) {
    mockPub := &MockEventPublisher{}
    manager := NewManager(logger, clock, idGen, mockPub)

    sessionID, _ := manager.CreateUserSession()

    // Verify event published
    assert.Equal(t, 1, len(mockPub.calls))
    assert.Equal(t, "session.created", mockPub.calls[0].Type)
    assert.Equal(t, sessionID, mockPub.calls[0].SessionID)
}
```

**3. NATSEventPublisher Unit Tests:**
- Verify event JSON structure and schema
- Verify subject construction correctness
- Verify eventIndex increments per session
- Mock pkg/nats.Client for isolation

**4. Subject Builder Tests:**
```go
func TestSubjectBuilders(t *testing.T) {
    assert.Equal(t, "sessions.test-123.session.created", SessionCreated("test-123"))
    assert.Equal(t, "sessions.test_123.session.created", SessionCreated("test.123"))  // Sanitized
}
```

### Integration Tests (Real NATS)

**Docker-based NATS:**

```go
func TestNATSEventPublisher_Integration(t *testing.T) {
    // Start NATS in Docker (testcontainers or docker-compose)
    natsClient := startNATSServer(t)
    defer natsClient.Close()

    // Create real publisher
    publisher := NewNATSEventPublisher(natsClient, idGen, clock, logger)

    // Subscribe to verify delivery
    msgs := make(chan *nats.Msg, 10)
    natsClient.Subscribe("sessions.>", func(msg *nats.Msg) {
        msgs <- msg
    })

    // Publish event
    err := publisher.PublishSessionCreated(context.Background(), "test-123")
    require.NoError(t, err)

    // Verify received
    select {
    case msg := <-msgs:
        assert.Equal(t, "sessions.test-123.session.created", msg.Subject)

        var event map[string]interface{}
        json.Unmarshal(msg.Data, &event)
        assert.Equal(t, "1.0", event["version"])
        assert.Equal(t, "session.created", event["type"])
    case <-time.After(2 * time.Second):
        t.Fatal("timeout waiting for event")
    }
}
```

**Test Coverage:**

- ✅ All EventPublisher methods (mocked)
- ✅ SessionManager publishes on state changes (mocked)
- ✅ Basic NATS publish + subscribe (real NATS)
- ✅ NoOpPublisher no-op behavior
- ✅ Subject builder validation
- ✅ Event schema validation
- ✅ Graceful degradation (NATS unavailable)

## Implementation Plan

### Phase 1: Core Infrastructure (2-3 hours)

**Files to create:**
1. `pkg/relay/events.go` - EventPublisher interface + NoOpPublisher
2. `pkg/relay/subjects.go` - Subject builders with sanitization
3. `pkg/relay/nats_publisher.go` - NATSEventPublisher implementation

**Tasks:**
- [ ] Define EventPublisher interface
- [ ] Implement NoOpPublisher
- [ ] Implement subject builders with tests
- [ ] Implement NATSEventPublisher
- [ ] Add per-session eventIndex tracking

### Phase 2: SessionManager Integration (1-2 hours)

**Files to modify:**
1. `pkg/relay/session/manager.go` - Add publisher field + publish calls

**Tasks:**
- [ ] Add EventPublisher field to Manager
- [ ] Update NewManager constructor signature
- [ ] Add publish calls to CreateUserSession
- [ ] Add publish calls to TerminateUserSession
- [ ] Add publish calls to SpawnAgent
- [ ] Add publish calls to TerminateAgent

### Phase 3: main.go Wiring (1 hour)

**Files to modify:**
1. `cmd/relay/main.go` - Initialize publisher based on NATS_URL

**Tasks:**
- [ ] Check NATS_URL environment variable
- [ ] Initialize pkg/nats.Client if URL present
- [ ] Create NATSEventPublisher or NoOpPublisher
- [ ] Pass publisher to SessionManager
- [ ] Add graceful drain on shutdown

### Phase 4: Testing (2-3 hours)

**Files to create:**
1. `pkg/relay/events_test.go` - Unit tests for interfaces
2. `pkg/relay/nats_publisher_test.go` - Unit tests for publisher
3. `pkg/relay/subjects_test.go` - Subject builder tests
4. `pkg/relay/nats_integration_test.go` - Integration test with real NATS

**Tasks:**
- [ ] Implement MockEventPublisher
- [ ] Unit test SessionManager with mock
- [ ] Unit test NATSEventPublisher with mock nats.Client
- [ ] Unit test subject builders
- [ ] Integration test with Docker NATS
- [ ] Test graceful degradation

### Phase 5: Documentation & Review (1 hour)

**Tasks:**
- [ ] Update docs/NATS.md with implementation details
- [ ] Update README with NATS_URL configuration
- [ ] Add inline code comments
- [ ] Update issue #38 with completion notes

**Total Estimate:** 7-10 hours

## Verification & Acceptance Criteria

### Manual Testing

1. **Without NATS:**
   ```bash
   # Relay should start normally
   ./relay
   # Verify: No NATS warnings, WebSocket works
   ```

2. **With NATS (valid):**
   ```bash
   NATS_URL=nats://localhost:4222 ./relay
   # Verify: NATS connected, events published

   # Subscribe to events
   nats sub "sessions.>"
   # Verify: See session.created, agent.spawned events
   ```

3. **With NATS (invalid URL):**
   ```bash
   NATS_URL=nats://invalid:4222 ./relay
   # Verify: Warning logged, relay continues with NoOpPublisher
   ```

### Acceptance Criteria Checklist

From Issue #38:

- [ ] Relay publishes session:created to NATS topic
- [ ] Relay publishes session:terminated to NATS topic
- [ ] Relay publishes agent:spawned to NATS topic
- [ ] Relay publishes agent:terminated to NATS topic
- [ ] WebSocket functionality unchanged when NATS disabled
- [ ] NATS failures don't crash relay (logs warning, continues)
- [ ] Events can be inspected with `nats sub sessions.>`
- [ ] Integration tests verify event publishing
- [ ] Unit tests with mock publisher pass
- [ ] Code review approved
- [ ] Documentation updated

## Security Considerations

### Authentication

Use NATS credentials for production:

```bash
# Generate credentials
nats-server -js -creds relay.creds

# Configure relay
NATS_URL=nats://localhost:4222
NATS_CREDENTIALS=/path/to/relay.creds
```

### Authorization

NATS ACLs should restrict relay to:
- **Publish:** `sessions.>`
- **Subscribe:** None (publish-only service)

### Data Sensitivity

Event payloads contain:
- Session IDs (non-sensitive UUIDs)
- Agent roles (non-sensitive names)
- Workspace paths (may contain session IDs)
- Exit codes (integers)

**No PII or secrets in events.**

## Future Enhancements (Out of Scope)

### Short-term (Follow-up PRs)

1. **JetStream Persistence:**
   - Add `WithJetStream()` option to NATSEventPublisher
   - Enable at-least-once delivery with pub-acks
   - Support event replay for debugging

2. **Enhanced Observability:**
   - Prometheus dashboard for event metrics
   - Grafana panels for publish rates and errors
   - Alerting on high error rates

3. **Agent State Change Events:**
   - Publish heartbeat events
   - Publish state transitions (starting, ready, busy, idle)

### Long-term (Phase 2+)

1. **Coordinator Integration:**
   - Subscribe to events in coordinator
   - React to lifecycle events
   - Orchestrate multi-agent workflows

2. **Event Logger Service:**
   - Standalone service that subscribes to all events
   - Writes to disk/database for audit trail
   - Provides replay API

3. **Load Balancing:**
   - Multiple relay instances with queue groups
   - Work distribution via NATS
   - Horizontal scaling

## References

- [Issue #38 - Relay NATS Event Publisher](https://github.com/2389-research/ourocodus/issues/38)
- [docs/NATS.md](../NATS.md) - NATS integration strategy
- [pkg/nats/client.go](../../pkg/nats/client.go) - Existing NATS client
- [NATS Documentation](https://docs.nats.io/)
- [NATS Core vs JetStream](https://docs.nats.io/nats-concepts/jetstream)

## Appendix: Event Flow Diagram

```
User Action
    ↓
WebSocket Message
    ↓
relay.Server.HandleWebSocket
    ↓
session.Manager.CreateSession()
    ├─→ Update in-memory state
    └─→ if publisher != nil:
            go publisher.PublishSessionCreated(sessionID)
                ↓
            NATSEventPublisher.PublishSessionCreated()
                ├─→ Build event JSON (messageId, eventIndex, timestamps)
                ├─→ Get subject: subjects.SessionCreated(sessionID)
                └─→ nats.Client.Publish(subject, json)
                        ├─→ Add correlation headers
                        ├─→ Retry with exponential backoff (3x)
                        ├─→ Record metrics
                        └─→ Return (non-blocking)
    ↓
Return response to WebSocket
```

## Appendix: Subject Hierarchy

```
sessions.{sessionId}.session.created
sessions.{sessionId}.session.terminated
sessions.{sessionId}.agent.spawned
sessions.{sessionId}.agent.terminated

# Wildcard subscriptions:
sessions.*                        # All sessions, single token
sessions.*.session.created        # All session creations
sessions.*.agent.*                # All agent events
sessions.>                        # Everything (multi-token)
```

## Appendix: Decision Log

| Decision | Rationale |
|----------|-----------|
| Observer pattern | Clean separation, testable, extensible |
| Type-specific subjects | Better filtering, NATS best practice |
| Core NATS (not JetStream) | Simpler for MVP, add persistence later when needed |
| Best-effort retry | Balance simplicity and reliability |
| Fire-and-forget | Never block SessionManager operations |
| Reuse pkg/nats | Avoid reinventing retry/metrics/health logic |
| NoOpPublisher | Graceful degradation, zero overhead when disabled |
| Goroutines for publishing | Guarantee non-blocking behavior |
| eventIndex per session | Enable gap detection for consumers |
| occurredAt vs publishedAt | Distinguish state change time from publish time |

---

**Document Version:** 1.0
**Last Updated:** 2025-11-04
**Author:** Claude (via brainstorming skill)
**Reviewers:** TBD
