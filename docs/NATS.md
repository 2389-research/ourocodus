# NATS Integration Plan for Ourocodus

## Executive Summary

**Decision:** Use NATS (with JetStream) as the message bus for Ourocodus Phase 2

**Confidence:** 8-9/10 across multiple model consensus

**Key Benefits:**
- Native request/reply pattern for approval gates and RPC
- Built-in queue groups for horizontal relay scaling
- 10M+ msg/sec performance for parallel agent coordination
- Simple operational model (stateless core)
- Excellent Go ecosystem support (nats.go)

**Key Risks Identified:**
- Message size limits (1MB default, configurable)
- Exactly-once semantics require additional design
- JetStream adds operational complexity (stateful component)
- Team learning curve if unfamiliar with NATS

---

## Why NATS Over MQTT

### Multi-Model Consensus Analysis

Three AI models (o3-mini for/against, gpt-5-mini neutral) evaluated NATS vs MQTT:

**Unanimous Agreement:**
- **Request/Reply**: NATS has first-class `Request()` with automatic correlation; MQTT requires manual correlation IDs and response topics
- **Queue Groups**: NATS has native queue groups; MQTT requires MQTT 5.0 shared subscriptions (broker-dependent)
- **Performance**: NATS designed for microservices (10M+ msg/sec); MQTT designed for IoT (lower throughput)
- **Patterns**: NATS supports pub/sub + request/reply + queue groups; MQTT primarily pub/sub with QoS
- **Operational**: NATS has stateless core (simple clustering); MQTT maintains QoS state (complex HA)

**Use Case Alignment:**
- Ourocodus needs: Coordinator RPC, parallel agents, approval gates, event streaming, load balancing
- NATS strengths: Microservices orchestration, low-latency RPC, native load balancing
- MQTT strengths: IoT reliability, offline device support (not relevant here)

**Alternatives Considered:**
- **Kafka**: Better for event-sourcing and analytics, but heavier operational burden and poor for low-latency RPC
- **RabbitMQ**: Mature with RPC support, but heavier and lower throughput than NATS
- **Redis Streams**: Simple, but less robust for multi-tenant RPC patterns

**Verdict:** NATS is the clear winner for this use case.

---

## Topic Hierarchy Design

Topic design is critical - affects routing, permissions, observability, and scaling.

### Proposed Hierarchy

```
# Session-level events (broadcast to all observers)
sessions.{session_id}.events
  Examples:
    - session.created
    - session.terminated
    - agent.spawned
    - agent.terminated
  Subscribers: Coordinator, API Server (SSE), Logger

# Work distribution (queue groups for load balancing)
sessions.{session_id}.work.{role}
  Purpose: Coordinator → Relay work assignments
  Example: sessions.sess_abc123.work.auth
  Queue Group: "relay-workers" (multiple relay instances compete)

# Results (point-to-point back to coordinator)
sessions.{session_id}.results.{role}
  Purpose: Relay → Coordinator task completion
  Example: sessions.sess_abc123.results.auth

# Approvals (request/reply pattern with timeout)
approvals.requests
  Purpose: Coordinator → API Server approval requests
  Uses: NATS Request() with timeout
  Response: Auto-correlated via NATS inbox

# Agent heartbeats (monitoring and health)
agents.{session_id}.{role}.heartbeat
  Purpose: Relay → Monitoring system
  Frequency: Every 30s
  Content: { state, lastActive, errorMsg? }

# Logging (fire-and-forget to event logger)
logs.events
  Purpose: All components → centralized logger
  Subscribers: Event Logger (writes to disk/DB)
```

### Wildcard Subscriptions

Enable powerful monitoring patterns:

```go
// Monitor all session events across all sessions
nc.Subscribe("sessions.*.events", handler)

// Monitor all work for a specific role across sessions
nc.Subscribe("sessions.*.work.auth", handler)

// Monitor all heartbeats
nc.Subscribe("agents.>", handler)  // '>' matches multiple tokens
```

### Topic Design Principles

1. **Hierarchical** - Enables permission boundaries (future: API keys per session/namespace)
2. **Predictable** - Easy to construct topics programmatically
3. **Wildcard-friendly** - Supports monitoring and debugging
4. **Version-tolerant** - Can add new topics without breaking existing subscribers

---

## Message Schemas

All messages are JSON with versioned schemas.

### Common Fields

Every message includes:

```json
{
  "version": "1.0",
  "timestamp": "2025-10-28T12:34:56Z",
  "messageId": "msg_abc123",
  "type": "session.created"
}
```

### Session Events

```json
{
  "version": "1.0",
  "timestamp": "2025-10-28T12:34:56Z",
  "messageId": "msg_001",
  "type": "session.created",
  "payload": {
    "sessionId": "sess_abc123",
    "state": "ACTIVE",
    "createdAt": "2025-10-28T12:34:56Z"
  }
}
```

### Work Requests

```json
{
  "version": "1.0",
  "timestamp": "2025-10-28T12:35:00Z",
  "messageId": "msg_002",
  "type": "work.spawn_agent",
  "payload": {
    "sessionId": "sess_abc123",
    "role": "auth",
    "workspace": "workspaces/sess_abc123/auth",
    "task": {
      "description": "Implement user authentication using bcrypt",
      "requirements": [...]
    }
  }
}
```

### Results

```json
{
  "version": "1.0",
  "timestamp": "2025-10-28T12:45:00Z",
  "messageId": "msg_003",
  "type": "result.success",
  "correlationId": "msg_002",
  "payload": {
    "sessionId": "sess_abc123",
    "role": "auth",
    "summary": "Implemented authentication with bcrypt. Tests passing.",
    "filesChanged": ["src/auth.go", "src/auth_test.go"],
    "commitSha": "abc123def"
  }
}
```

### Approval Requests (Request/Reply)

```go
// Request
msg := &nats.Msg{
  Subject: "approvals.requests",
  Data: json.Marshal(ApprovalRequest{
    SessionID: "sess_abc123",
    Phase: "post-coding",
    Summary: "Review auth implementation before testing?",
    ChangesURL: "https://github.com/.../compare/abc123",
  }),
}

// Reply (auto-correlated by NATS)
resp, err := nc.RequestMsg(msg, 5*time.Minute)
var approval ApprovalResponse
json.Unmarshal(resp.Data, &approval)
// approval.Approved == true/false
```

---

## Integration with Relay

### Current Architecture (Phase 1)

```
PWA → WebSocket → Relay → stdio → N× ACP processes
```

- Direct WebSocket connection
- No message bus
- Manual message routing in relay code

### Target Architecture (Phase 2)

```
PWA → WebSocket → API Server → NATS → Relay → stdio → N× ACP processes
                                  ↓
                             Coordinator
```

- API Server handles WebSocket (decoupled from relay)
- NATS message bus for all internal communication
- Relay subscribes to work topics via queue groups
- Coordinator orchestrates workflows

### Relay NATS Client Setup

```go
// pkg/relay/nats_client.go
type NATSClient struct {
    nc      *nats.Conn
    js      nats.JetStreamContext
    manager *session.Manager
    logger  Logger
}

func NewNATSClient(natsURL string, manager *session.Manager) (*NATSClient, error) {
    // Connect to NATS
    nc, err := nats.Connect(natsURL,
        nats.Name("relay-worker"),
        nats.MaxReconnects(-1),
        nats.ReconnectWait(2*time.Second),
    )
    if err != nil {
        return nil, err
    }

    // Create JetStream context
    js, err := nc.JetStream()
    if err != nil {
        return nil, err
    }

    client := &NATSClient{
        nc:      nc,
        js:      js,
        manager: manager,
    }

    return client, nil
}

func (c *NATSClient) SubscribeToWork() error {
    // Subscribe to ALL work topics with queue group
    // Multiple relay instances will load-balance via queue group
    _, err := c.nc.QueueSubscribe("sessions.*.work.*", "relay-workers", c.handleWork)
    return err
}

func (c *NATSClient) handleWork(msg *nats.Msg) {
    var work WorkRequest
    if err := json.Unmarshal(msg.Data, &work); err != nil {
        c.logger.Printf("Invalid work message: %v", err)
        return
    }

    // Route to existing session manager
    switch work.Type {
    case "work.spawn_agent":
        err := c.manager.SpawnAgent(context.Background(),
            work.Payload.SessionID,
            work.Payload.Role,
            work.Payload.Workspace)

        // Publish result
        result := Result{
            Type: "result.success",
            CorrelationID: work.MessageID,
            Payload: ResultPayload{
                SessionID: work.Payload.SessionID,
                Role: work.Payload.Role,
            },
        }
        resultJSON, _ := json.Marshal(result)
        c.nc.Publish(
            fmt.Sprintf("sessions.%s.results.%s", work.Payload.SessionID, work.Payload.Role),
            resultJSON,
        )
    }
}
```

---

## JetStream for Persistence

### Why JetStream?

**Core NATS** is ephemeral - messages are lost if no subscriber is listening.

**JetStream** adds:
- Persistent message storage (disk-backed)
- Message replay and time-travel
- At-least-once delivery guarantees
- Consumer acknowledgment and retries

**Use Cases for Ourocodus:**
- Event logging and auditing (replay for debugging)
- Coordinator workflow state recovery (restart without loss)
- Observability dashboard (historical event queries)

### JetStream Streams

```go
// Create stream for session events (durable storage)
js.AddStream(&nats.StreamConfig{
    Name:     "SESSION_EVENTS",
    Subjects: []string{"sessions.*.events"},
    Storage:  nats.FileStorage,
    Retention: nats.LimitsPolicy,
    MaxAge:   7 * 24 * time.Hour,  // 7 days
    MaxBytes: 10 * 1024 * 1024 * 1024,  // 10GB
})

// Create stream for work/results (shorter retention)
js.AddStream(&nats.StreamConfig{
    Name:     "WORK_RESULTS",
    Subjects: []string{"sessions.*.work.*", "sessions.*.results.*"},
    Storage:  nats.FileStorage,
    Retention: nats.LimitsPolicy,
    MaxAge:   24 * time.Hour,  // 1 day
    MaxBytes: 5 * 1024 * 1024 * 1024,  // 5GB
})
```

### Consumer Groups

```go
// Event logger consumes all events durably
js.AddConsumer("SESSION_EVENTS", &nats.ConsumerConfig{
    Durable:   "event-logger",
    AckPolicy: nats.AckExplicitPolicy,
})

// Subscribe with consumer
sub, err := js.PullSubscribe("sessions.*.events", "event-logger")
msgs, err := sub.Fetch(10)  // Batch processing
for _, msg := range msgs {
    // Process and write to disk
    msg.Ack()
}
```

---

## Edge Cases and Failure Modes

### 1. Large Payloads (>1MB)

**Problem:** NATS default max payload is 1MB. Git diffs, test outputs, or large file contents may exceed this.

**Solution:** Externalize large artifacts to object storage

```go
// Instead of sending large payload in message:
type WorkRequest struct {
    Task struct {
        Description string
        RequirementsURL string  // → S3/MinIO URL
    }
}

// Coordinator uploads to S3, sends URL in message
requirementsURL := uploadToS3(requirements)
workMsg.Task.RequirementsURL = requirementsURL

// Relay downloads from S3 when processing work
requirements := downloadFromS3(workMsg.Task.RequirementsURL)
```

**Configuration:**
```go
// Increase max payload if needed (not recommended >10MB)
nc, err := nats.Connect(natsURL, nats.MaxPayload(10*1024*1024))
```

### 2. Exactly-Once Semantics

**Problem:** NATS provides at-least-once delivery with JetStream. A relay might process the same work twice if it crashes mid-processing.

**Solution:** Implement idempotency with persistent workflow state

```go
// Database schema
type WorkflowState struct {
    MessageID string  // Primary key (idempotency key)
    Status    string  // pending|processing|completed|failed
    Result    string
    UpdatedAt time.Time
}

func (c *NATSClient) handleWork(msg *nats.Msg) {
    var work WorkRequest
    json.Unmarshal(msg.Data, &work)

    // Check if already processed
    state, err := c.db.GetWorkflowState(work.MessageID)
    if err == nil && state.Status == "completed" {
        // Already processed, acknowledge and skip
        msg.Ack()
        return
    }

    // Mark as processing
    c.db.UpsertWorkflowState(WorkflowState{
        MessageID: work.MessageID,
        Status:    "processing",
        UpdatedAt: time.Now(),
    })

    // Do work
    result := c.manager.SpawnAgent(...)

    // Mark as completed
    c.db.UpsertWorkflowState(WorkflowState{
        MessageID: work.MessageID,
        Status:    "completed",
        Result:    result,
        UpdatedAt: time.Now(),
    })

    msg.Ack()
}
```

### 3. NATS Server Failure

**Problem:** NATS server crashes or becomes unreachable.

**Mitigation:**
```go
// Client auto-reconnects
nc, err := nats.Connect(natsURL,
    nats.MaxReconnects(-1),  // Infinite retries
    nats.ReconnectWait(2*time.Second),
    nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
        log.Printf("NATS disconnected: %v", err)
    }),
    nats.ReconnectHandler(func(nc *nats.Conn) {
        log.Printf("NATS reconnected")
    }),
)
```

**NATS Clustering:**
```bash
# Connect to cluster of NATS servers for HA
nats-server --cluster nats://node1:6222 --routes nats://node2:6222,nats://node3:6222
```

```go
// Client connects to cluster
nc, err := nats.Connect("nats://node1:4222,nats://node2:4222,nats://node3:4222")
```

### 4. Message Ordering

**Problem:** NATS does not guarantee message ordering across different publishers.

**Solution:** Use JetStream with single consumer per session

```go
// Create session-specific consumer
js.AddConsumer("WORK_RESULTS", &nats.ConsumerConfig{
    Durable:       fmt.Sprintf("coordinator-%s", sessionID),
    FilterSubject: fmt.Sprintf("sessions.%s.>", sessionID),
    AckPolicy:     nats.AckExplicitPolicy,
    DeliverPolicy: nats.DeliverAllPolicy,
})
```

### 5. Slow Consumer

**Problem:** Event logger can't keep up with message rate.

**Solution:** Use JetStream with persistent queue

```go
// Messages buffer in JetStream stream
// Slow consumer processes at its own pace
sub, err := js.PullSubscribe("sessions.*.events", "event-logger",
    nats.PullMaxWaiting(1000),  // Buffer up to 1000 messages
)

for {
    msgs, err := sub.Fetch(100, nats.MaxWait(5*time.Second))
    for _, msg := range msgs {
        processEvent(msg)
        msg.Ack()
    }
}
```

### 6. Network Partitions

**Problem:** Relay loses connection to NATS but agents are still running.

**Detection:**
```go
func (c *NATSClient) MonitorConnection() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        if !c.nc.IsConnected() {
            c.logger.Printf("NATS connection lost - buffering messages")
            // Switch to local queuing mode
        }
    }
}
```

**Mitigation:**
- Buffer messages locally during partition
- Relay publishes buffered messages when reconnected
- JetStream deduplicates based on message ID

---

## Migration Strategy

### Phase 1 → Phase 2 Transition

**Current State:**
- Direct WebSocket connections
- Relay handles all routing
- No coordinator

**Target State:**
- API Server handles WebSocket
- NATS message bus
- Coordinator orchestrates workflows

**Migration Path:**

#### Step 1: Add NATS alongside WebSocket (Parallel Run)

```
PWA → WebSocket → Relay → stdio → Agents
                    ↓ (also publish to NATS)
                  NATS → Event Logger
```

- Relay connects to NATS
- Relay publishes events to NATS (fire-and-forget)
- Existing WebSocket path continues working
- Validate NATS integration with event logging

#### Step 2: Add Coordinator (Observer Mode)

```
PWA → WebSocket → Relay → stdio → Agents
                    ↓
                  NATS ← Coordinator (read-only)
```

- Coordinator subscribes to session events
- Coordinator logs workflow states but doesn't interfere
- Validate coordinator state tracking matches reality

#### Step 3: Coordinator Drives (Dual-Write)

```
PWA → WebSocket → API Server → NATS → Relay (queue group)
                                 ↓
                            Coordinator
```

- API Server translates WebSocket to NATS messages
- Coordinator publishes work to NATS
- Relay subscribes via queue group (load balancing enabled)
- Legacy WebSocket path deprecated but still functional

#### Step 4: Remove WebSocket (NATS Only)

```
PWA → WebSocket → API Server → NATS → Relay (queue group)
                                 ↓
                            Coordinator
```

- Remove legacy direct-WebSocket-to-relay path
- All communication via NATS
- Full Coordinator orchestration enabled

### Rollback Plan

Each step can be rolled back:
- Step 1: Stop publishing to NATS, remove event logger
- Step 2: Stop coordinator
- Step 3: Route directly from API Server to Relay (bypass NATS)
- Step 4: Re-enable legacy WebSocket path

---

## Operational Concerns

### Monitoring

**Metrics to track:**
- Message throughput (msgs/sec per topic)
- Message latency (publish → receive time)
- Queue group distribution (work spread across relays)
- JetStream storage usage (disk space)
- Consumer lag (messages pending acknowledgment)

**Prometheus Integration:**
```yaml
# NATS exports Prometheus metrics on /metrics
- job_name: 'nats'
  static_configs:
    - targets: ['nats-server:7777']
```

**Key Metrics:**
- `nats_server_sent_msgs_total` - Total messages sent
- `nats_server_slow_consumers` - Consumers falling behind
- `nats_jetstream_storage_bytes` - Disk usage

### Authentication and Authorization

**NKeys (Recommended):**
```bash
# Generate keypair for relay
nk -gen user -pubout

# relay.nk (private key - keep secret)
SUAG...

# relay.pub (public key - add to NATS server config)
UABC...
```

**NATS Server Config:**
```conf
authorization {
  users = [
    {
      nkey: UABC...  # relay public key
      permissions: {
        publish: ["sessions.>", "logs.events"]
        subscribe: ["sessions.*.work.*"]
      }
    },
    {
      nkey: UXYZ...  # coordinator public key
      permissions: {
        publish: ["sessions.*.work.*", "approvals.requests"]
        subscribe: ["sessions.*.results.*", "approvals.responses.*"]
      }
    }
  ]
}
```

**Client Authentication:**
```go
opt, err := nats.NkeyOptionFromSeed("relay.nk")
nc, err := nats.Connect(natsURL, opt)
```

### TLS Encryption

```conf
# NATS server config
tls {
  cert_file: "/path/to/server-cert.pem"
  key_file:  "/path/to/server-key.pem"
  ca_file:   "/path/to/ca.pem"
  verify:    true
}
```

```go
// Client with TLS
nc, err := nats.Connect(natsURL, nats.RootCAs("/path/to/ca.pem"))
```

### Backups

**JetStream Storage:**
```bash
# Backup JetStream data directory
tar -czf jetstream-backup-$(date +%Y%m%d).tar.gz /var/lib/nats/jetstream

# Schedule daily backups
0 2 * * * /usr/local/bin/backup-jetstream.sh
```

**Configuration as Code:**
```go
// Store stream configurations in Git
// Apply on startup
type StreamConfig struct {
    Name     string
    Subjects []string
    MaxAge   time.Duration
}

configs := loadStreamConfigs("nats-streams.yaml")
for _, cfg := range configs {
    js.AddStream(&nats.StreamConfig{...})
}
```

---

## Next Steps

### Immediate Actions

1. **Proof of Concept** (1 week)
   - Set up local NATS server
   - Implement basic pub/sub in relay
   - Publish session events to NATS
   - Validate event logger integration

2. **Topic Hierarchy Validation** (3 days)
   - Review topic structure with team
   - Test wildcard subscriptions
   - Validate permissions model

3. **Message Schema Design** (3 days)
   - Define all message types
   - Create JSON schemas
   - Implement validation

4. **Integration Spike** (1 week)
   - Integrate NATS client into relay
   - Implement queue group subscription
   - Test load balancing with 2+ relay instances

### Phase 2 Rollout (Estimated 6-8 weeks)

**Week 1-2:** NATS alongside WebSocket (Step 1)
**Week 3-4:** Add Coordinator in observer mode (Step 2)
**Week 5-6:** Coordinator drives workflows (Step 3)
**Week 7-8:** Remove WebSocket, NATS only (Step 4)

### Success Criteria

- [ ] All relay instances load-balance via queue groups
- [ ] Approval gates work with <5s latency
- [ ] Event logger captures 100% of events
- [ ] Coordinator successfully orchestrates coding→testing→review workflow
- [ ] Zero message loss during normal operation
- [ ] Graceful degradation during NATS server restart
- [ ] Prometheus dashboards show healthy metrics

---

## Local Development with NATS

### Quick Start

Start the local NATS server with JetStream:

```bash
# Start services
docker-compose up -d

# Verify NATS is healthy
curl http://localhost:8222/healthz
# Expected: {"status":"ok"}

# Check JetStream streams were created
docker-compose logs nats-init
# Expected: ✓ SESSION_EVENTS stream created
#           ✓ WORK_RESULTS stream created
```

### NATS CLI Installation

Install the [NATS CLI](https://github.com/nats-io/natscli) for interactive testing:

```bash
# macOS
brew install nats-io/nats-tools/nats

# Linux
curl -sf https://binaries.nats.dev/nats-io/natscli/nats@latest | sh

# Windows
scoop bucket add nats https://github.com/nats-io/scoop.git
scoop install nats
```

### Inspecting JetStream Streams

```bash
# List all streams
nats stream list

# View SESSION_EVENTS stream details
nats stream info SESSION_EVENTS

# View WORK_RESULTS stream details
nats stream info WORK_RESULTS

# Monitor stream activity in real-time
nats stream events
```

### Publishing Test Messages

```bash
# Publish a session created event
nats pub "sessions.test-123.events" '{
  "type": "session.created",
  "session_id": "test-123",
  "timestamp": "2025-10-30T20:00:00Z"
}'

# Publish multiple test events
for i in {1..10}; do
  nats pub "sessions.test-$i.events" "{\"session_id\": \"test-$i\"}"
done

# Publish a work result
nats pub "sessions.test-123.results.coder" '{
  "session_id": "test-123",
  "role": "coder",
  "status": "completed",
  "output": "// Generated code here"
}'
```

### Subscribing to Messages

```bash
# Subscribe to all session events
nats sub "sessions.*.events"

# Subscribe to events for a specific session
nats sub "sessions.test-123.events"

# Subscribe to all work results
nats sub "sessions.*.results.*"

# Subscribe with queue group (load balancing)
nats sub "sessions.*.events" --queue=processors

# Multiple subscribers in same queue group will load-balance
# Terminal 1:
nats sub "sessions.*.events" --queue=workers
# Terminal 2:
nats sub "sessions.*.events" --queue=workers
# Only one subscriber receives each message
```

### Request/Reply Testing

```bash
# Start a responder (simulates approval service)
nats reply "approvals.request" "APPROVED"

# Send a request and wait for reply
nats request "approvals.request" '{
  "session_id": "test-123",
  "agent_role": "coder",
  "action": "commit_code"
}'

# Request with timeout
nats request "approvals.request" "test" --timeout=5s
```

### Monitoring and Debugging

```bash
# View server statistics
curl http://localhost:8222/varz | jq

# View JetStream statistics
curl http://localhost:8222/jsz | jq

# View connections
curl http://localhost:8222/connz | jq

# Monitor all messages (use with caution in production!)
nats sub ">"

# Monitor specific subject patterns
nats sub "sessions.*.events"
nats sub "sessions.>"
```

### JetStream Consumer Testing

```bash
# Create a consumer for SESSION_EVENTS stream
nats consumer add SESSION_EVENTS test-consumer \
  --filter="sessions.*.events" \
  --deliver=all \
  --ack=explicit \
  --max-deliver=3

# Pull messages from consumer
nats consumer next SESSION_EVENTS test-consumer --count=10

# View consumer information
nats consumer info SESSION_EVENTS test-consumer

# Delete consumer when done
nats consumer rm SESSION_EVENTS test-consumer
```

### Cleaning Up

```bash
# Stop services
docker-compose down

# Remove all data (including JetStream streams)
docker-compose down -v

# Restart fresh
docker-compose up -d
```

### Common Issues

**Issue:** `nats` command not found
**Solution:** Install NATS CLI using instructions above

**Issue:** Connection refused on port 4222
**Solution:** Ensure Docker services are running: `docker-compose ps`

**Issue:** Streams not found
**Solution:** Check init script ran successfully: `docker-compose logs nats-init`

**Issue:** Messages not persisting after restart
**Solution:** Ensure volume is mounted: `docker volume ls | grep nats-data`

---

## References

- [NATS Documentation](https://docs.nats.io/)
- [JetStream Guide](https://docs.nats.io/nats-concepts/jetstream)
- [nats.go Client](https://github.com/nats-io/nats.go)
- [NATS vs MQTT Comparison](https://docs.nats.io/nats-concepts/overview/compare-nats)
- [Request/Reply Pattern](https://docs.nats.io/nats-concepts/core-nats/reqreply)
- [Queue Groups](https://docs.nats.io/nats-concepts/core-nats/queue)
