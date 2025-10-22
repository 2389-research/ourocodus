# ACP Relay - Product Requirements Document

## Purpose

Bridge between internal NATS messaging and external ACP-speaking agents (Claude Code, OpenAI Codex, etc.) running in containers. Routes messages bidirectionally while maintaining protocol translation and connection management.

## Responsibilities

1. **Protocol translation** - NATS JSON ↔ ACP WebSocket/HTTP+SSE
2. **Connection management** - Maintain WebSocket connections to agent containers
3. **Session routing** - Route messages based on session_id + agent_role
4. **Agent registration** - Accept connections from new agent containers
5. **Health monitoring** - Detect and report agent disconnections
6. **Event logging** - Log all ACP messages for observability

## Non-Responsibilities

- Agent lifecycle (API server handles this via Docker)
- Workflow orchestration (Coordinator handles this)
- State persistence (Event logger handles this)
- Approval logic (Coordinator handles this)

## Technical Approach

**Language:** Go
**Framework:** `gorilla/websocket` for WebSocket, `net/http` for HTTP
**Port:** 8080 (ACP endpoint), internal NATS client
**Dependencies:** NATS client, WebSocket library

## Architecture

```
┌──────────────┐
│  NATS Bus    │
│              │
│  sessions.   │
│  sess_123.   │
│  work.coding │
└──────┬───────┘
       │
       │ Subscribe
       │
┌──────▼─────────────────────────┐
│  Relay                          │
│                                 │
│  ┌──────────────────────┐      │
│  │ NATS Subscriber      │      │
│  │ - Receives work msgs │      │
│  └──────────┬───────────┘      │
│             │                   │
│  ┌──────────▼───────────┐      │
│  │ Router               │      │
│  │ - session_id → conn  │      │
│  │ - agent_role → conn  │      │
│  └──────────┬───────────┘      │
│             │                   │
│  ┌──────────▼───────────┐      │
│  │ ACP Translator       │      │
│  │ - NATS JSON → ACP    │      │
│  │ - ACP → NATS JSON    │      │
│  └──────────┬───────────┘      │
│             │                   │
│  ┌──────────▼───────────┐      │
│  │ WebSocket Manager    │      │
│  │ - Connection pool    │      │
│  │ - Send/receive       │      │
│  └──────────┬───────────┘      │
└─────────────┼───────────────────┘
              │
              │ WebSocket
              │
    ┌─────────▼──────┐  ┌────────────────┐
    │ Claude Code    │  │ OpenAI Codex   │
    │ Container      │  │ Container      │
    └────────────────┘  └────────────────┘
```

## API Specification

### Agent Connection (WebSocket)

#### Connect to Relay
```
ws://relay:8080/acp/{session_id}/{role}

Query params:
  - agent_id (required)

Example:
ws://relay:8080/acp/sess_123/coding?agent_id=agent_abc
```

#### Connection Headers
```
X-Agent-ID: agent_abc123
X-Session-ID: sess_xyz789
X-Agent-Role: coding
X-ACP-Version: 1.0
```

#### Registration Response
```json
{
  "type": "connection.established",
  "relay_id": "relay_main",
  "session_id": "sess_123",
  "agent_id": "agent_abc",
  "timestamp": "2025-10-22T10:00:00Z"
}
```

### Message Flow

#### Inbound (NATS → Agent)

**NATS Message:**
```
Topic: sessions.sess_123.work.coding
Payload: {
  "id": "msg_123",
  "type": "work.coding",
  "payload": {
    "task": "Implement authentication",
    "requirements": [...]
  }
}
```

**Translated to ACP:**
```json
{
  "jsonrpc": "2.0",
  "id": "msg_123",
  "method": "sampling/createMessage",
  "params": {
    "messages": [
      {
        "role": "user",
        "content": "Implement authentication. Requirements: ..."
      }
    ],
    "model": "claude-sonnet-4",
    "max_tokens": 4096,
    "system": "You are a coding agent...",
    "tools": [...]
  }
}
```

#### Outbound (Agent → NATS)

**ACP Message from Agent:**
```json
{
  "jsonrpc": "2.0",
  "id": "msg_123",
  "result": {
    "role": "assistant",
    "content": [
      {
        "type": "tool_use",
        "name": "bash",
        "id": "tool_1",
        "input": {
          "command": "git checkout -b auth-feature"
        }
      }
    ]
  }
}
```

**Translated to NATS:**
```
Topic: sessions.sess_123.results.coding
Payload: {
  "id": "msg_123",
  "type": "tool_use",
  "payload": {
    "tool_name": "bash",
    "tool_id": "tool_1",
    "input": {
      "command": "git checkout -b auth-feature"
    }
  }
}
```

## Data Models

### Connection
```go
type Connection struct {
    ID           string
    SessionID    string
    AgentID      string
    Role         string
    WebSocket    *websocket.Conn
    ConnectedAt  time.Time
    LastActivity time.Time
    Alive        bool
}
```

### Message Envelope (Internal)
```go
type InternalMessage struct {
    ID        string                 `json:"id"`
    SessionID string                 `json:"session_id"`
    Type      string                 `json:"type"`
    Payload   map[string]interface{} `json:"payload"`
}
```

### ACP Message (External)
```go
type ACPMessage struct {
    JSONRPC string                 `json:"jsonrpc"`
    ID      string                 `json:"id"`
    Method  string                 `json:"method,omitempty"`
    Params  map[string]interface{} `json:"params,omitempty"`
    Result  map[string]interface{} `json:"result,omitempty"`
    Error   *ACPError              `json:"error,omitempty"`
}

type ACPError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}
```

## State Management

**In-memory connection registry:**

```go
type Relay struct {
    nats        *nats.Conn
    connections map[string]*Connection  // agent_id → connection
    routes      map[string]*Connection  // session_id:role → connection
    mu          sync.RWMutex
}
```

**Lookup strategies:**
1. By agent_id: Direct map lookup
2. By session_id + role: Composite key `sess_123:coding`

**Connection lifecycle:**
```
Agent connects → Register in maps → Subscribe to NATS topic
Agent disconnects → Remove from maps → Publish disconnect event
```

## Configuration

```go
type Config struct {
    Port         int    // WebSocket server port (default: 8080)
    NATSUrl      string // NATS server URL
    PingInterval int    // WebSocket ping interval (seconds)
    PongTimeout  int    // WebSocket pong timeout (seconds)
    MaxMsgSize   int    // Max WebSocket message size (bytes)
}
```

**Environment Variables:**
- `OUROCODUS_RELAY_PORT`
- `OUROCODUS_NATS_URL`
- `OUROCODUS_PING_INTERVAL`
- `OUROCODUS_PONG_TIMEOUT`

## Protocol Translation

### NATS → ACP

**Step 1:** Parse internal message
**Step 2:** Map message type to ACP method
**Step 3:** Construct ACP sampling request
**Step 4:** Send over WebSocket

**Mapping:**
```go
var messageTypeToACPMethod = map[string]string{
    "work.coding":  "sampling/createMessage",
    "work.testing": "sampling/createMessage",
    "work.review":  "sampling/createMessage",
    "tool_result":  "sampling/createMessage", // Continue conversation
}
```

### ACP → NATS

**Step 1:** Parse ACP message
**Step 2:** Extract result type (text, tool_use)
**Step 3:** Construct internal message
**Step 4:** Publish to NATS

**Mapping:**
```go
func translateACPToInternal(acp *ACPMessage) *InternalMessage {
    if acp.Result != nil {
        content := acp.Result["content"]
        if isToolUse(content) {
            return &InternalMessage{
                Type: "tool_use",
                Payload: extractToolUse(content),
            }
        } else {
            return &InternalMessage{
                Type: "result.success",
                Payload: extractText(content),
            }
        }
    }
    if acp.Error != nil {
        return &InternalMessage{
            Type: "result.failure",
            Payload: map[string]interface{}{
                "error": acp.Error.Message,
            },
        }
    }
    return nil
}
```

## Connection Management

### Heartbeat

**Mechanism:** WebSocket ping/pong
**Interval:** 30 seconds
**Timeout:** 10 seconds

```go
func (r *Relay) maintainConnection(conn *Connection) {
    ticker := time.NewTicker(30 * time.Second)
    for {
        select {
        case <-ticker.C:
            if err := conn.WebSocket.WriteMessage(websocket.PingMessage, nil); err != nil {
                r.handleDisconnect(conn)
                return
            }
        }
    }
}
```

### Reconnection

**POC:** No automatic reconnection (agent restarts)
**Post-POC:** Implement exponential backoff reconnection

### Disconnect Handling

**On disconnect:**
1. Remove from connection registry
2. Publish event to NATS: `sessions.{id}.events` → `agent.disconnected`
3. Close WebSocket
4. Log disconnect reason

## Error Handling

### WebSocket Errors

| Error | Response |
|-------|----------|
| Connection closed | Publish disconnect event, remove from registry |
| Read timeout | Close connection, remove from registry |
| Write timeout | Retry once, then disconnect |
| Invalid message | Log error, send error response, continue |

### NATS Errors

| Error | Response |
|-------|----------|
| Publish failed | Log error, retry with exponential backoff |
| Subscribe failed | Fatal error (relay cannot function) |
| Connection lost | Attempt reconnect, buffer messages |

### Protocol Errors

| Error | Response |
|-------|----------|
| Invalid ACP message | Send ACP error response to agent |
| Unknown message type | Log warning, drop message |
| Missing session/agent ID | Reject connection |

## Security (Deferred for POC)

**POC:** No authentication, localhost only
**Post-POC:**
- TLS for WebSocket connections
- Agent authentication tokens
- Message signing/verification
- Rate limiting per agent

## Logging

**Structured logging (JSON):**
```json
{
  "timestamp": "2025-10-22T10:00:00Z",
  "level": "info",
  "component": "relay",
  "action": "agent_connected",
  "session_id": "sess_123",
  "agent_id": "agent_abc",
  "role": "coding"
}
```

**Log events:**
- Agent connect/disconnect
- Message sent/received
- Protocol translation
- Errors

## Metrics (Future)

- Active connections
- Messages per second (inbound/outbound)
- Protocol translation errors
- Connection duration
- Message latency

## Testing

### Unit Tests
- Protocol translation (NATS ↔ ACP)
- Connection registry operations
- Message routing logic

### Integration Tests
- Real WebSocket connections
- Real NATS server
- End-to-end message flow

### Load Tests
- Multiple concurrent agents
- High message throughput
- Connection churn (connect/disconnect)

## Implementation Notes

1. **Goroutine per connection** - Each agent gets dedicated goroutine for reads/writes
2. **Buffered channels** - Use for message passing between goroutines
3. **Graceful shutdown** - Close all connections, drain NATS, wait for goroutines
4. **Message IDs** - Preserve IDs across translation for request/response correlation

## Dependencies

```
go.mod:
  - github.com/nats-io/nats.go
  - github.com/gorilla/websocket
```

## File Structure

```
cmd/relay/
  main.go              # Entry point
  server.go            # WebSocket server
  nats_handler.go      # NATS subscriber
  translator.go        # Protocol translation
  registry.go          # Connection management
  config.go            # Configuration
```

## Success Criteria

Relay is complete when:
1. Agent can connect via WebSocket
2. Messages route: NATS → Agent
3. Messages route: Agent → NATS
4. Protocol translation works (NATS JSON ↔ ACP)
5. Multiple agents can connect simultaneously
6. Disconnect detection works
7. Event logging implemented
8. Basic tests pass
9. Can handle full coding workflow (send task, receive tool uses, send results)
