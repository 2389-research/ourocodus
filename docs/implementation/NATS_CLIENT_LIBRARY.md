# NATS Client Library Implementation Plan

**Issue**: #43
**Branch**: `feature/nats-client-library-43`
**Consensus Confidence**: High (8/10 from all three models)
**Estimated Timeline**: 5 weeks

## Executive Summary

Implement a reusable NATS client library (`pkg/nats`) with a unified façade API that composes separate core NATS and JetStream functionality. The library will provide connection management, auto-reconnect, pub/sub/request-reply patterns, JetStream durable processing, comprehensive metrics, and graceful shutdown.

## Architecture Decision

After consulting multiple expert models, we've chosen the **Unified Façade** approach:

```go
type Client interface {
    // Core NATS operations
    Publish(ctx context.Context, subject string, data []byte, opts ...PubOption) error
    Subscribe(ctx context.Context, subject string, handler MsgHandler, opts ...SubOption) (*Subscription, error)
    Request(ctx context.Context, subject string, data []byte, opts ...ReqOption) (*Message, error)

    // JetStream access (explicit sub-interface)
    JS() JSClient

    // Health & lifecycle
    Health() HealthStatus
    Ready() error
    Drain(ctx context.Context) error
    Close() error
}
```

**Rationale**:
- Simple API for services needing only pub/sub (relay)
- Explicit JetStream interface for durable processing (event logger)
- Internal composition keeps code modular and testable
- Easy to mock with small interfaces
- Balances simplicity vs clarity

## Implementation Phases

### Phase 1: Foundation (2 weeks)

**Goals**: Core NATS operations, connection management, basic observability

#### Tasks:
1. **Package Structure**
   - `pkg/nats/client.go` - Main Client implementation
   - `pkg/nats/options.go` - Functional options pattern
   - `pkg/nats/errors.go` - Typed error wrappers
   - `pkg/nats/message.go` - Message wrapper with correlation IDs
   - `pkg/nats/subscription.go` - Subscription handle
   - `pkg/nats/health.go` - Health status types
   - `pkg/nats/metrics.go` - Prometheus metrics

2. **Connection Management**
   - Auto-reconnect with exponential backoff + jitter
   - Reconnect callbacks for metrics/logging
   - Connection health tracking
   - Graceful degradation during reconnect

3. **Core Operations**
   - `Publish()` - Fire-and-forget with correlation IDs
   - `Subscribe()` - Handler-based with wildcards
   - `Request()` - Request/reply with timeout
   - Correlation ID injection (W3C traceparent + Correlation-Id headers)

4. **Error Handling**
   - `TransientError` type (timeouts, no responders, reconnecting)
   - `PermanentError` type (auth, invalid subject, permissions)
   - Wrapped errors with context

5. **Metrics (Basic)**
   - `nats_messages_published_total{subject, status}`
   - `nats_messages_received_total{subject}`
   - `nats_publish_errors_total{subject, error_type}`
   - `nats_reconnects_total`
   - `nats_connection_up` (gauge)
   - `nats_request_duration_seconds` (histogram)

6. **Health & Lifecycle**
   - `Health()` - Returns connection status, last error, RTT
   - `Ready()` - Returns error if not ready for traffic
   - `Close()` - Immediate shutdown
   - `Drain()` - Graceful shutdown with timeout

7. **Testing**
   - Mock interfaces for unit tests
   - Test connection lifecycle
   - Test error classification
   - Test correlation ID propagation

**Deliverables**:
- Working core NATS client
- Unit tests with mocks
- Example usage in `examples/nats/basic/`

### Phase 2: JetStream (1.5 weeks)

**Goals**: Durable processing, stream/consumer management

#### Tasks:
1. **JetStream Client**
   - `pkg/nats/jetstream.go` - JSClient implementation
   - `pkg/nats/stream.go` - Stream configuration helpers
   - `pkg/nats/consumer.go` - Consumer management
   - `pkg/nats/pullconsumer.go` - Pull-based consumption

2. **Stream Management**
   - `EnsureStream()` - Idempotent stream creation
   - Stream configuration builder
   - Error handling for stream conflicts

3. **Consumer Management**
   - `EnsureConsumer()` - Idempotent consumer creation
   - Durable consumer support
   - Consumer configuration builder

4. **Pull Consumption**
   - `PullConsume()` - Batch fetch with backpressure
   - Per-message ACK after successful handler
   - NAK with delay for retryable failures
   - TERM for permanent failures
   - Configurable concurrency and batch size

5. **Publishing with Acks**
   - `PublishAsync()` - Publish with JetStream ack
   - Nats-Msg-Id for deduplication
   - PubAck response handling

6. **Batch Processing**
   - Per-message ack with optimized flush (not true batch ack)
   - Backpressure via max in-flight messages
   - Handler concurrency control

7. **JetStream Metrics**
   - `js_acks_total{type}` (ack, nak, term, progress)
   - `js_redeliveries_total{stream, consumer}`
   - `js_pull_batches_total{stream, consumer}`
   - `js_pull_fetch_errors_total{stream, consumer}`
   - `js_consumer_lag{stream, consumer}` (gauge)
   - `js_inflight_messages{stream, consumer}` (gauge)
   - `js_fetch_duration_seconds{stream, consumer}` (histogram)

8. **Testing**
   - Integration tests with testcontainers
   - Test stream/consumer lifecycle
   - Test pull consumption with redelivery
   - Test ack/nak/term semantics

**Deliverables**:
- Working JetStream client
- Integration tests with real NATS server
- Example usage in `examples/nats/jetstream/`

### Phase 3: Production Readiness (1.5 weeks)

**Goals**: Polish, documentation, performance, CI integration

#### Tasks:
1. **Configuration System**
   - Environment variable overrides
   - Configuration validation
   - Sensible defaults documentation
   - Config builder helpers

2. **Retry Policies**
   - Configurable retry strategies
   - Exponential backoff with jitter
   - Max attempts and deadlines
   - Per-operation retry configuration

3. **Advanced Metrics**
   - `nats_bytes_published_total{subject}`
   - `nats_bytes_received_total{subject}`
   - `nats_handler_duration_seconds{subject}` (histogram)
   - `nats_rtt_seconds` (gauge)
   - Metric namespace configuration

4. **Documentation**
   - API reference documentation
   - Architecture decision records
   - Usage examples for common patterns
   - Migration guide from raw nats.go
   - Troubleshooting guide

5. **Examples**
   - Basic pub/sub example
   - Request/reply example
   - JetStream durable consumer example
   - Graceful shutdown example
   - Metrics scraping example

6. **Performance Testing**
   - Benchmark publish throughput
   - Benchmark request/reply latency
   - Benchmark JetStream consumption
   - Memory profiling

7. **CI Integration**
   - Unit tests in main CI pipeline
   - Integration tests with `-tags=integration`
   - Race detector enabled
   - Coverage reporting
   - Linting with golangci-lint

**Deliverables**:
- Production-ready library
- Comprehensive documentation
- Performance benchmarks
- CI/CD pipeline

## Package Structure

```
pkg/nats/
├── client.go           # Main Client implementation
├── client_test.go      # Client unit tests
├── options.go          # Functional options
├── options_test.go
├── errors.go           # Typed errors
├── errors_test.go
├── message.go          # Message wrapper with headers
├── message_test.go
├── subscription.go     # Subscription handle
├── subscription_test.go
├── health.go           # Health status types
├── health_test.go
├── metrics.go          # Prometheus metrics
├── metrics_test.go
├── jetstream.go        # JetStream client
├── jetstream_test.go
├── stream.go           # Stream management
├── stream_test.go
├── consumer.go         # Consumer management
├── consumer_test.go
├── pullconsumer.go     # Pull consumption
├── pullconsumer_test.go
├── mock.go             # Mock implementations for testing
└── integration_test.go # Integration tests (tag: integration)

examples/nats/
├── basic/
│   └── main.go         # Basic pub/sub example
├── request-reply/
│   └── main.go         # Request/reply example
├── jetstream/
│   └── main.go         # JetStream example
└── graceful-shutdown/
    └── main.go         # Graceful shutdown example
```

## Configuration

### Connection Options

```go
type ClientConfig struct {
    URLs              []string          // NATS server URLs
    Name              string            // Client name
    Credentials       string            // Credentials file path
    JWT               string            // JWT for auth
    NKey              string            // NKey for auth
    TLS               *tls.Config       // TLS configuration

    // Reconnection
    ReconnectWait     time.Duration     // Default: 2s
    MaxReconnects     int               // Default: -1 (unlimited)
    ReconnectBufSize  int               // Default: 8MB

    // Timeouts
    ConnectTimeout    time.Duration     // Default: 10s
    RequestTimeout    time.Duration     // Default: 5s
    DrainTimeout      time.Duration     // Default: 30s

    // Retry policy
    RetryAttempts     int               // Default: 3
    RetryBackoff      BackoffStrategy   // Default: exponential with jitter

    // Correlation
    CorrelationHeader string            // Default: "Correlation-Id"
    TraceparentHeader string            // Default: "traceparent"
    GenerateID        func() string     // Default: UUID v4

    // Metrics
    MetricsNamespace  string            // Default: "nats_client"
    MetricsSubsystem  string            // Default: ""
    MetricsEnabled    bool              // Default: true

    // Callbacks
    ReconnectedCB     func(*nats.Conn)
    DisconnectedCB    func(*nats.Conn, error)
    ClosedCB          func(*nats.Conn)
}
```

### Environment Variables

```bash
NATS_URL=nats://localhost:4222
NATS_CREDENTIALS=/path/to/creds
NATS_RECONNECT_WAIT=2s
NATS_MAX_RECONNECTS=-1
NATS_CONNECT_TIMEOUT=10s
NATS_REQUEST_TIMEOUT=5s
```

## Error Handling Strategy

### Error Types

```go
type TransientError struct {
    Op      string
    Subject string
    Err     error
}

type PermanentError struct {
    Op      string
    Subject string
    Err     error
}
```

### Classification Rules

**Transient** (retry with backoff):
- `nats.ErrTimeout`
- `nats.ErrNoResponders`
- Network errors during reconnection
- 5xx JetStream API errors
- `ErrConnectionClosed` (during reconnect)

**Permanent** (fail immediately):
- `nats.ErrBadSubject`
- `nats.ErrAuthorization`
- `nats.ErrMaxPayload`
- 4xx JetStream API errors
- Invalid configuration

### Retry Strategy

```go
type BackoffStrategy interface {
    Next(attempt int) time.Duration
    Reset()
}

// Default: exponential with jitter
// Attempt 1: 200ms + jitter
// Attempt 2: 400ms + jitter
// Attempt 3: 800ms + jitter
// Max: 5s
```

## Metrics Specification

### Core NATS Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `nats_messages_published_total` | Counter | `subject`, `status` | Total messages published |
| `nats_messages_received_total` | Counter | `subject` | Total messages received |
| `nats_publish_errors_total` | Counter | `subject`, `error_type` | Total publish errors |
| `nats_handler_errors_total` | Counter | `subject`, `error_type` | Total handler errors |
| `nats_reconnects_total` | Counter | - | Total reconnection attempts |
| `nats_requests_total` | Counter | `subject`, `status` | Total requests sent |
| `nats_request_errors_total` | Counter | `subject`, `error_type` | Total request errors |
| `nats_connection_up` | Gauge | - | Connection status (0/1) |
| `nats_rtt_seconds` | Gauge | - | Round-trip time to server |
| `nats_request_duration_seconds` | Histogram | `subject` | Request latency |
| `nats_publish_latency_seconds` | Histogram | `subject` | Publish latency |
| `nats_handler_duration_seconds` | Histogram | `subject` | Handler execution time |

### JetStream Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `js_acks_total` | Counter | `stream`, `consumer`, `type` | Total acks (ack/nak/term/progress) |
| `js_redeliveries_total` | Counter | `stream`, `consumer` | Total message redeliveries |
| `js_pull_batches_total` | Counter | `stream`, `consumer` | Total pull batches |
| `js_pull_fetch_errors_total` | Counter | `stream`, `consumer`, `error_type` | Total fetch errors |
| `js_consumer_lag` | Gauge | `stream`, `consumer` | Consumer lag (messages) |
| `js_inflight_messages` | Gauge | `stream`, `consumer` | In-flight messages |
| `js_fetch_duration_seconds` | Histogram | `stream`, `consumer` | Fetch operation latency |

### Label Cardinality Guidelines

- **subject**: Use subject patterns, not individual subjects (e.g., `sessions.*.events` not `sessions.abc123.events`)
- **stream**: Stream name
- **consumer**: Consumer name
- **status**: `success`, `error`, `timeout`
- **error_type**: Error category (e.g., `transient`, `permanent`, `auth`)
- **type**: Ack type (e.g., `ack`, `nak`, `term`)

## Testing Strategy

### Unit Tests

**Approach**: Interface-based mocking

```go
type Publisher interface {
    Publish(ctx context.Context, subject string, data []byte) error
}

type Subscriber interface {
    Subscribe(ctx context.Context, subject string, handler MsgHandler) (*Subscription, error)
}

type JetStreamer interface {
    EnsureStream(ctx context.Context, cfg StreamConfig) error
    PullConsume(ctx context.Context, cfg PullConsumerConfig, handler MsgHandler) (*Consumer, error)
}
```

**Coverage**:
- Connection lifecycle (connect, reconnect, close)
- Error classification and retry logic
- Correlation ID propagation
- Metrics emission
- Graceful shutdown

### Integration Tests

**Approach**: Testcontainers with real NATS server

```go
// +build integration

func TestIntegrationPublishSubscribe(t *testing.T) {
    ctx := context.Background()

    // Start NATS container
    natsContainer := testcontainers.RunContainer(ctx, testcontainers.ContainerRequest{
        Image: "nats:2.10-alpine",
        Args:  []string{"-js"},
    })
    defer natsContainer.Terminate(ctx)

    // Test end-to-end flow
    // ...
}
```

**Coverage**:
- Publish/subscribe with real messages
- Request/reply with real server
- Reconnection scenarios (restart server)
- JetStream stream/consumer lifecycle
- Pull consumption with redelivery
- Graceful shutdown without message loss

### CI Pipeline

```yaml
test:
  runs-on: ubuntu-latest
  steps:
    - name: Run unit tests
      run: go test -race -coverprofile=coverage.txt ./pkg/nats/...

    - name: Run integration tests
      run: |
        docker-compose up -d nats
        go test -race -tags=integration ./pkg/nats/...
        docker-compose down
```

## Dependencies

```go
require (
    github.com/nats-io/nats.go v1.31.0
    github.com/prometheus/client_golang v1.17.0
    github.com/google/uuid v1.4.0
    github.com/testcontainers/testcontainers-go v0.26.0 // test only
)
```

## Success Criteria

### Phase 1 (Foundation)
- [ ] Core Client implementation complete
- [ ] Connection management with auto-reconnect working
- [ ] Publish/Subscribe/Request operations functional
- [ ] Correlation IDs propagated correctly
- [ ] Basic metrics exported to Prometheus
- [ ] Health/Ready endpoints implemented
- [ ] Unit tests with >80% coverage
- [ ] Example code runs successfully

### Phase 2 (JetStream)
- [ ] JSClient implementation complete
- [ ] Stream/Consumer management working
- [ ] Pull consumption with backpressure functional
- [ ] Ack/NAK/TERM semantics correct
- [ ] JetStream metrics exported
- [ ] Integration tests pass with real NATS server
- [ ] Example code runs successfully

### Phase 3 (Production Ready)
- [ ] Configuration system complete
- [ ] Retry policies configurable
- [ ] Documentation complete (API, examples, troubleshooting)
- [ ] Performance benchmarks established
- [ ] CI pipeline integrated
- [ ] All linting checks pass
- [ ] Ready for use in relay service (#45)

## Migration Path

### For Existing Code Using Raw nats.go

```go
// Before (raw nats.go)
nc, _ := nats.Connect(nats.DefaultURL)
nc.Publish("subject", []byte("data"))

// After (pkg/nats)
client, _ := nats.NewClient(nats.WithURL("nats://localhost:4222"))
client.Publish(ctx, "subject", []byte("data"))
```

Benefits:
- Automatic correlation IDs
- Retry with backoff
- Metrics out of the box
- Health monitoring

## Future Enhancements (Post-MVP)

- [ ] Support for NATS KV (Key-Value store)
- [ ] Support for NATS Object Store
- [ ] Distributed tracing integration (OpenTelemetry)
- [ ] Advanced consumer patterns (push-based)
- [ ] Circuit breaker for failing publishers
- [ ] Message batching for high-throughput scenarios
- [ ] Schema validation integration

## References

- [NATS.io Documentation](https://docs.nats.io/)
- [nats.go Client Library](https://github.com/nats-io/nats.go)
- [JetStream Documentation](https://docs.nats.io/nats-concepts/jetstream)
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [Project NATS Integration Design](../NATS.md)

## Timeline Summary

| Phase | Duration | Start | End |
|-------|----------|-------|-----|
| Phase 1: Foundation | 2 weeks | Week 1 | Week 2 |
| Phase 2: JetStream | 1.5 weeks | Week 3 | Mid Week 4 |
| Phase 3: Production Ready | 1.5 weeks | Mid Week 4 | Week 5 |
| **Total** | **5 weeks** | | |

## Questions & Decisions

### Q1: Should we support both push and pull subscribers for JetStream?
**Decision**: Start with pull-based (more scalable), add push later if needed.

### Q2: How do we handle "batch ack" which isn't native to JetStream?
**Decision**: Implement per-message ack with optimized flush operations. Document as "batch processing with individual acks."

### Q3: Should we expose raw nats.Conn for power users?
**Decision**: Yes, provide `Raw() *nats.Conn` method for advanced use cases not covered by our API.

### Q4: How aggressive should auto-reconnect be?
**Decision**: Unlimited attempts with exponential backoff (cap at 30s), configurable via options.

### Q5: Should metrics be enabled by default?
**Decision**: Yes, with ability to disable. Zero metrics overhead if Prometheus not initialized.
