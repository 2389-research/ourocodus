# Event Logger Service Design

**Date:** 2025-11-06
**Issue:** #44
**Status:** Approved

## Overview

The Event Logger Service is a standalone binary that consumes all NATS messages and logs them to stdout for observability. It provides a complete audit trail of system events without impacting relay operations.

## Goals

- Capture all NATS lifecycle events (session.created, agent.spawned, etc.)
- Simple implementation suitable for development and early production
- Graceful degradation (relay continues working if logger is down)
- Infrastructure-agnostic logging (stdout handled by Docker/systemd/k8s)

## Non-Goals (Future Work)

- Event filtering or subject-specific subscriptions
- Payload truncation or size limits
- Metrics or monitoring integration
- Durable consumption via JetStream
- Event replay capability

## Architecture

```
┌─────────────┐
│    NATS     │
│   Server    │
└──────┬──────┘
       │ subscribe(">")
       │ (wildcard - all subjects)
       ▼
┌─────────────────┐
│  Event Logger   │
│    (binary)     │
└─────────────────┘
       │
       ▼ JSON Lines
    stdout
       │
       ▼
  Docker logs /
  systemd journal /
  kubectl logs
```

## Implementation

### Binary Location
`cmd/event-logger/main.go`

### Dependencies
- `github.com/nats-io/nats.go` - NATS client library (already in go.mod via #43)

### Configuration

Environment variables:
- `NATS_URL` (required) - NATS server URL (e.g., `nats://localhost:4222`)

### Output Format

Each message is logged as a single JSON line:

```json
{
  "timestamp": "2025-11-06T20:30:00Z",
  "subject": "sessions.abc123.session.created",
  "data": {...original message payload...}
}
```

**Fields:**
- `timestamp` - When the logger received the message (RFC3339 UTC)
- `subject` - NATS subject the message was published to
- `data` - Raw message payload as JSON (no transformation)

### Core Logic

```go
// 1. Connect to NATS with auto-reconnect
nc, _ := nats.Connect(natsURL,
    nats.Name("event-logger"),
    nats.MaxReconnects(-1))

// 2. Subscribe to ALL subjects
nc.Subscribe(">", func(msg *nats.Msg) {
    // Format and print to stdout
    logEntry := map[string]interface{}{
        "timestamp": time.Now().UTC().Format(time.RFC3339),
        "subject":   msg.Subject,
        "data":      json.RawMessage(msg.Data),
    }
    json.NewEncoder(os.Stdout).Encode(logEntry)
})

// 3. Wait for shutdown signal
// 4. Drain NATS connection gracefully
```

## Behavior

### Normal Operation
- Connects to NATS on startup
- Logs all received messages to stdout as JSON lines
- Auto-reconnects if connection is lost

### Failure Modes

**NATS unavailable at startup:**
- Binary exits with error (fail-fast)
- Operator must ensure NATS is running

**NATS connection lost during operation:**
- Client auto-reconnects (MaxReconnects=-1)
- Misses events during downtime (acceptable for MVP)

**Event logger crashes:**
- Relay continues operating normally
- Events are lost while logger is down (graceful degradation)

## Integration with Existing System

### Relay Changes
**None required.** Issue #45 already implemented:
- `pkg/relay/nats_publisher.go` - Publishes events to NATS
- `pkg/relay/session/publisher.go` - EventPublisher interface
- `cmd/relay/main.go` - Optional NATS integration via NATS_URL

The relay is already publishing events. Event logger just needs to consume them.

### NATS Subject Pattern

Relay publishes to these subjects (from `pkg/relay/subjects.go`):
- `sessions.{sanitizedSessionID}.session.created`
- `sessions.{sanitizedSessionID}.session.terminated`
- `sessions.{sanitizedSessionID}.agent.spawned`
- `sessions.{sanitizedSessionID}.agent.terminated`

> **Note:** `{sanitizedSessionID}` is the user session ID with dots (`.`) replaced by underscores (`_`), as implemented in `pkg/relay/subjects.go`.

Event logger subscribes to `>` (all subjects) to capture these plus any future events.

## Testing Strategy

### Unit Tests
Not applicable - main.go is primarily integration code.

### Integration Tests
Manual testing workflow:
1. Start NATS server (`docker-compose up nats`)
2. Start event-logger binary
3. Start relay with NATS_URL configured
4. Trigger session events via WebSocket
5. Verify JSON logs appear in event-logger stdout

### Smoke Test
Can be added to `scripts/smoke-test.sh`:
```bash
# Start event-logger in background
./bin/event-logger > /tmp/events.log 2>&1 &
LOGGER_PID=$!

# Run relay smoke tests
# ...

# Verify events were logged
grep "session.created" /tmp/events.log
kill $LOGGER_PID
```

## Deployment

### Development
```bash
# Terminal 1: NATS server
docker-compose up nats

# Terminal 2: Event logger
NATS_URL=nats://localhost:4222 ./bin/event-logger

# Terminal 3: Relay
NATS_URL=nats://localhost:4222 ./bin/relay
```

### Production (Docker Compose)
```yaml
services:
  event-logger:
    image: ourocodus/event-logger:latest
    environment:
      - NATS_URL=nats://nats:4222
    depends_on:
      - nats
    # Logs automatically captured by Docker
```

### Production (Kubernetes)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: event-logger
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: event-logger
        image: ourocodus/event-logger:latest
        env:
        - name: NATS_URL
          value: "nats://nats-service:4222"
# Logs captured by kubectl logs
```

## Future Enhancements

When the simple approach becomes insufficient:

### Phase 2 Improvements
- **Filtering:** Exclude noisy subjects (`$SYS.>`, `_INBOX.>`)
- **Truncation:** Cap payload size, add `truncated: true` flag
- **Metrics:** Prometheus counters for messages received/dropped
- **Headers:** Include NATS message headers in output

### Phase 3 Durability
- **JetStream consumer:** Durable consumption for critical events
- **Replay capability:** Historical event queries
- **Persistent storage:** Write to database instead of stdout

### Phase 4 Scale
- **Subject filtering:** Allow/deny list configuration
- **Sampling:** Log only % of high-volume subjects
- **Horizontal scaling:** Multiple logger instances with queue groups

## Open Questions

None - design approved.

## References

- Issue #44: Event Logger Service
- Issue #45: Relay NATS Integration (already complete)
- `docs/NATS.md`: NATS integration architecture
- `pkg/relay/nats_publisher.go`: Existing publisher implementation
- `pkg/nats/`: NATS client library (#43)

## Approval

Design approved by: [User]
Date: 2025-11-06
Ready for implementation: Yes
