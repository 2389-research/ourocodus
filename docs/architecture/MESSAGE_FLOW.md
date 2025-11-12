# End-to-End Message Flow

Complete journey of messages through the Ourocodus system, from browser to containerized ACP process and back.

## Overview

This document traces messages through all layers of the system, explaining protocols, multiplexing, and transformations at each boundary.

```mermaid
graph TB
    subgraph Browser
        PWA[PWA JavaScript<br/>WebSocket client]
    end

    subgraph "Relay Server (Go)"
        WS[WebSocket Handler<br/>:8080/ws]
        SM[Session Manager<br/>In-memory state]
        ACF[ACP Client Factory<br/>Launcher selection]
    end

    subgraph "Process Launcher"
        Host[Host Launcher<br/>os/exec]
        Container[Container Attach Launcher<br/>Docker API]
    end

    subgraph "Agent Container (Docker)"
        Attach[Container Attach<br/>Hijacked connection]
        Demux[Stream Demultiplexer<br/>stdcopy.StdCopy]
        ACP[ACP Binary<br/>claude-code-acp]
    end

    PWA -->|"1. WebSocket<br/>JSON"| WS
    WS -->|"2. Go channels"| SM
    SM -->|"3. Interface call"| ACF
    ACF -->|"4a. Spawn"| Host
    ACF -->|"4b. Attach"| Container
    Container -->|"5. Docker API"| Attach
    Attach -->|"6. Multiplexed<br/>stdout/stderr"| Demux
    Demux -->|"7. JSON-RPC<br/>stdin/stdout"| ACP

    style PWA fill:#e1f5ff
    style Container fill:#ffe1e1
    style ACP fill:#e1ffe1
```

## Layer 1: Browser → Relay (WebSocket)

### Protocol: WebSocket + JSON

Messages flow over WebSocket connection on port 8080.

```mermaid
sequenceDiagram
    participant PWA
    participant Browser WebSocket
    participant Relay /ws Handler

    PWA->>Browser WebSocket: send(JSON.stringify(msg))
    Browser WebSocket->>Relay /ws Handler: WebSocket frame<br/>[TEXT, payload]

    Note over Relay /ws Handler: gorilla/websocket<br/>ReadMessage()

    Relay /ws Handler->>Relay /ws Handler: json.Unmarshal(data, &msg)

    Note over Relay /ws Handler: Route by msg.Type:<br/>session:create<br/>agent:spawn<br/>agent:message
```

### Example: Session Creation

**PWA sends:**

```javascript
// Browser JavaScript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
    ws.send(JSON.stringify({
        type: 'session:create'
    }));
};
```

**Wire format (WebSocket frame):**

```
Opcode: TEXT (0x1)
Payload: {"type":"session:create"}
```

**Relay receives:**

```go
// pkg/relay/server.go
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, _ := s.upgrader.Upgrade(w, r, nil)

    for {
        messageType, data, err := conn.ReadMessage()
        // messageType = websocket.TextMessage
        // data = []byte(`{"type":"session:create"}`)

        var msg relay.Message
        json.Unmarshal(data, &msg)
        // msg.Type = "session:create"
    }
}
```

### Complete Session Lifecycle Flow

```mermaid
sequenceDiagram
    autonumber
    participant PWA
    participant Relay
    participant SessionManager

    Note over PWA: User clicks "Create Session"

    PWA->>Relay: WS: {"type":"session:create"}
    Relay->>SessionManager: CreateUserSession()
    SessionManager->>SessionManager: Generate UUID<br/>Store in map[sessionID]*UserSession

    SessionManager-->>Relay: UserSession{ID:"4ad2f420", State:"ACTIVE"}
    Relay-->>PWA: WS: {"type":"session:created",<br/>"sessionId":"4ad2f420"}<br/>(latency: 2-5ms)

    Note over PWA: Session established<br/>Can now spawn agents
```

## Layer 2: Relay → Session Manager (Go Channels)

### Protocol: In-Process Go

Within the relay process, components communicate via Go channels and interface calls.

```mermaid
graph LR
    A[WebSocket Handler<br/>Goroutine per connection] -->|Channel| B[Session Manager<br/>Synchronized with RWMutex]
    B -->|Method call| C[User Session<br/>Goroutine per session]
    C -->|Method call| D[Agent Session<br/>One per agent]

    style A fill:#e1f5ff
    style B fill:#ffe1f5
    style C fill:#f5e1ff
    style D fill:#e1ffe1
```

### Example: Agent Spawn Request

```go
// WebSocket handler goroutine
type Message struct {
    Type      string `json:"type"`
    Role      string `json:"role,omitempty"`
    Workspace string `json:"workspace,omitempty"`
}

msg := Message{
    Type:      "agent:spawn",
    Role:      "coder",
    Workspace: "./workspaces/coder",
}

// Call session manager synchronously
agentSession, err := sessionMgr.SpawnAgent(
    ctx,
    userSessionID,
    msg.Role,
    msg.Workspace,
)
```

### State Management

```mermaid
stateDiagram-v2
    [*] --> UserSession_ACTIVE: CreateUserSession()

    UserSession_ACTIVE --> AgentSession_SPAWNING: SpawnAgent()
    AgentSession_SPAWNING --> AgentSession_ACTIVE: ACP ready
    AgentSession_SPAWNING --> AgentSession_FAILED: Spawn error

    AgentSession_ACTIVE --> AgentSession_TERMINATED: TerminateAgent()
    AgentSession_FAILED --> AgentSession_TERMINATED: Cleanup

    UserSession_ACTIVE --> UserSession_TERMINATED: TerminateUserSession()<br/>(terminates all agents)

    note right of UserSession_ACTIVE
        In-memory storage:
        map[sessionID]*UserSession

        Each UserSession has:
        map[agentID]*AgentSession
    end note
```

## Layer 3: Agent Spawn → Container + ACP Launch

### Protocol: Docker API + Process Spawning

When spawning an agent, three layers initialize in sequence:

```mermaid
sequenceDiagram
    autonumber
    participant SessionMgr
    participant LauncherFactory
    participant WorktreeMgr
    participant CredentialMounter
    participant ContainerMgr
    participant ACPClientFactory
    participant Launcher

    Note over SessionMgr: SpawnAgent("coder", workspace)

    SessionMgr->>LauncherFactory: Create()
    LauncherFactory->>WorktreeMgr: CreateWorktree(agentID)
    Note over WorktreeMgr: git worktree add<br/>-b agent-coder-123<br/>/workspaces/coder

    LauncherFactory->>CredentialMounter: MountCredentials(agentID)
    Note over CredentialMounter: Copy SSH keys, tokens<br/>to /credentials/agent-coder/

    LauncherFactory->>ContainerMgr: CreateContainerSession()
    Note over ContainerMgr: docker create ourocodus/agent<br/>--mount workspace<br/>--mount credentials

    ContainerMgr->>ContainerMgr: docker start container_id

    LauncherFactory-->>SessionMgr: RuntimeContext{<br/>ContainerID: "abc123",<br/>Workspace: "/workspaces/coder"}

    SessionMgr->>ACPClientFactory: NewClient(ctx, runtime)
    ACPClientFactory->>Launcher: Start(launchConfig)

    alt Host Mode
        Launcher->>Launcher: os/exec: claude-code-acp<br/>--workspace /workspaces/coder
    else Container Mode
        Launcher->>ContainerMgr: docker attach container_id
        Note over Launcher: Hijacked connection<br/>to container stdio
    end

    Launcher-->>ACPClientFactory: Transport{stdin, stdout, stderr}
    ACPClientFactory-->>SessionMgr: ACP Client ready
```

### Container Creation

```go
// pkg/containersession/manager.go
func (m *Manager) CreateContainerSession(
    ctx context.Context,
    imageName string,
    cmd []string,
) (*ContainerSession, error) {
    // 1. Generate session ID
    sessionID := m.idGen.Generate()

    // 2. Create workspace directory
    workspace := filepath.Join(m.baseWorkspaceDir, sessionID)
    os.MkdirAll(workspace, 0700)

    // 3. Create Docker container
    resp, err := m.dockerClient.ContainerCreate(ctx, &container.Config{
        Image: imageName,  // "ourocodus/agent:latest"
        Cmd:   cmd,        // ["/usr/local/bin/acp", "--workspace", "/workspace"]
        Labels: map[string]string{
            "com.ourocodus.containersession.id": sessionID,
            "com.ourocodus.containersession.managed-by": "ourocodus",
        },
    }, &container.HostConfig{
        Mounts: []mount.Mount{
            {
                Type:   mount.TypeBind,
                Source: workspace,  // Host path
                Target: "/workspace", // Container path
            },
        },
    }, nil, nil, "")

    return &ContainerSession{
        sessionID:   sessionID,
        containerID: resp.ID,
        workspace:   workspace,
        state:       StatePending,
    }, nil
}
```

### Workspace Isolation Architecture

```mermaid
graph TB
    subgraph "Host Filesystem"
        Base["/workspaces"]
        W1["/workspaces/agent-coder<br/>(git worktree)"]
        W2["/workspaces/agent-analyzer<br/>(git worktree)"]
        C1["/credentials/agent-coder<br/>(SSH keys, tokens)"]
        C2["/credentials/agent-analyzer<br/>(SSH keys, tokens)"]
    end

    subgraph "Container: agent-coder"
        CW1["/workspace<br/>(bind mount)"]
        CC1["/root/.ssh<br/>(bind mount, ro)"]
        ACP1["ACP Process<br/>PID 1"]
    end

    subgraph "Container: agent-analyzer"
        CW2["/workspace<br/>(bind mount)"]
        CC2["/root/.ssh<br/>(bind mount, ro)"]
        ACP2["ACP Process<br/>PID 1"]
    end

    W1 -.->|mount| CW1
    C1 -.->|mount readonly| CC1
    W2 -.->|mount| CW2
    C2 -.->|mount readonly| CC2

    CW1 --> ACP1
    CC1 --> ACP1
    CW2 --> ACP2
    CC2 --> ACP2

    style W1 fill:#e1f5ff
    style W2 fill:#e1f5ff
    style CW1 fill:#ffe1e1
    style CW2 fill:#ffe1e1
```

## Layer 4: Container Attach (Docker API)

### Protocol: Docker Attach API + Multiplexed Streams

For container mode, ACP runs as the container's main process (PID 1). The relay attaches to the container's stdio streams.

```mermaid
sequenceDiagram
    participant Relay
    participant Docker API
    participant Container
    participant ACP Process

    Relay->>Docker API: POST /containers/{id}/attach<br/>?stdin=1&stdout=1&stderr=1&stream=1
    Docker API->>Container: Attach to stdio
    Docker API-->>Relay: 101 Switching Protocols<br/>Connection: Upgrade<br/>Upgrade: tcp

    Note over Relay,Docker API: Hijacked connection (raw TCP)

    Relay->>ACP Process: Write to stdin:<br/>[8-byte header][JSON-RPC payload]
    ACP Process->>Container: stdout/stderr output
    Container->>Docker API: Multiplex streams
    Docker API->>Relay: [8-byte header][stream data]

    Note over Relay: stdcopy.StdCopy()<br/>demultiplexes stdout/stderr
```

### Docker Stream Multiplexing Format

Docker multiplexes stdout and stderr into a single stream with 8-byte headers:

```
+--------+--------+--------+--------+--------+--------+--------+--------+
| Stream | 0x00   | 0x00   | 0x00   |          Size (uint32)           |
| Type   |        |        |        |                                  |
+--------+--------+--------+--------+--------+--------+--------+--------+
|                          Payload                                     |
|                          (Size bytes)                                |
+----------------------------------------------------------------------+
```

**Stream Types:**
- `0x01` = stdout
- `0x02` = stderr
- `0x00` = stdin (not used in attach response)

**Example stdout packet:**

```
01 00 00 00  00 00 00 2A  7B 22 6A 73 6F 6E 72 70 63 ...
│           │           │
│           │           └─ Payload: {"jsonrpc":"2.0", ...}
│           └─ Size: 42 bytes (0x0000002A)
└─ Stream: stdout (0x01)
```

### Implementation: Stream Demultiplexing

```go
// pkg/relay/session/container_attach_process_launcher.go
func (l *ContainerAttachProcessLauncher) Start(
    ctx context.Context,
    cfg acp.ProcessLaunchConfig,
) (acp.Transport, error) {
    // Attach to container
    resp, err := l.dockerClient.ContainerAttach(ctx, l.containerID, dockertypes.ContainerAttachOptions{
        Stream: true,
        Stdin:  true,
        Stdout: true,
        Stderr: true,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to attach to container: %w", err)
    }

    // Create pipes for demultiplexed streams
    stdinPipe, stdinWriter := io.Pipe()
    stdoutReader, stdoutPipe := io.Pipe()

    // Capture stderr separately for logging
    stderrBuf := &bytes.Buffer{}

    // Spawn goroutine to demultiplex Docker's multiplexed stream
    go func() {
        defer stdoutPipe.Close()

        // stdcopy.StdCopy demultiplexes the 8-byte header format
        // Writes stdout to stdoutPipe, stderr to stderrBuf
        _, _, err := stdcopy.StdCopy(stdoutPipe, stderrBuf, resp.Reader)

        if err != nil {
            l.logger.Printf("[ACP] Stream demux error: %v", err)
        }

        // Log stderr line-by-line
        scanner := bufio.NewScanner(stderrBuf)
        for scanner.Scan() {
            l.logger.Printf("[ACP stderr container=%s] %s",
                l.containerID[:12], scanner.Text())
        }
    }()

    return &containerAttachTransport{
        stdin:  stdinWriter,
        stdout: stdoutReader,
        stderr: stderrBuf,
        conn:   resp.Conn,
    }, nil
}
```

### Stream Flow Visualization

```mermaid
graph TB
    subgraph Relay Process
        Writer[Relay writes JSON-RPC<br/>to Transport.Write]
        Stdin[stdin pipe]
        Stdout[stdout pipe]
        Demux[Demux goroutine<br/>stdcopy.StdCopy]
        Reader[Relay reads JSON-RPC<br/>from Transport.Read]
        Stderr[stderr buffer]
        Logger[Logger goroutine<br/>Line-by-line output]
    end

    subgraph Docker Daemon
        HijackConn[Hijacked TCP Connection<br/>Raw stream with 8-byte headers]
    end

    subgraph Container
        ACP_stdin[ACP stdin]
        ACP_stdout[ACP stdout]
        ACP_stderr[ACP stderr]
    end

    Writer --> Stdin
    Stdin -->|Raw bytes| HijackConn
    HijackConn -->|Raw bytes| ACP_stdin

    ACP_stdout -->|Raw bytes| HijackConn
    ACP_stderr -->|Raw bytes| HijackConn

    HijackConn -->|Multiplexed<br/>8-byte headers| Demux
    Demux -->|Demultiplexed| Stdout
    Demux -->|Demultiplexed| Stderr

    Stdout --> Reader
    Stderr --> Logger

    style Demux fill:#ffe1e1
    style HijackConn fill:#f5e1ff
```

## Layer 5: JSON-RPC over stdio (ACP Protocol)

### Protocol: JSON-RPC 2.0

ACP (Agent Client Protocol) uses JSON-RPC 2.0 over stdin/stdout.

```mermaid
sequenceDiagram
    participant Relay
    participant Transport
    participant ACP

    Note over Relay: Send message to agent

    Relay->>Transport: Write(jsonrpcRequest)
    Transport->>ACP: stdin: {"jsonrpc":"2.0","id":1,<br/>"method":"agent/sendMessage",<br/>"params":{"content":"Hello"}}

    Note over ACP: Process message<br/>(may call Claude API)

    ACP->>Transport: stdout: {"jsonrpc":"2.0","id":1,<br/>"result":{"response":"Hi there!"}}
    Transport->>Relay: Read() → jsonrpcResponse

    Note over Relay: Forward to PWA via WebSocket
```

### JSON-RPC Request Format

```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "method": "agent/sendMessage",
  "params": {
    "content": "Write a function to parse JSON",
    "role": "user"
  }
}
```

**Field definitions:**
- `jsonrpc`: Always `"2.0"` (protocol version)
- `id`: Request ID for correlation (integer or string)
- `method`: RPC method name (e.g., `"agent/sendMessage"`)
- `params`: Method-specific parameters (object)

### JSON-RPC Response Format

**Success:**

```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "result": {
    "response": "Here's a JSON parsing function:\n\nfunc ParseJSON(data []byte) (map[string]interface{}, error) {\n    var result map[string]interface{}\n    err := json.Unmarshal(data, &result)\n    return result, err\n}",
    "messageId": "msg-abc123"
  }
}
```

**Error:**

```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "error": {
    "code": -32600,
    "message": "Invalid Request",
    "data": {
      "detail": "Missing required parameter: content"
    }
  }
}
```

### Available RPC Methods

```mermaid
graph LR
    A[Relay] -->|agent/sendMessage| B[Send user message to agent]
    A -->|agent/getState| C[Get agent state]
    A -->|agent/stop| D[Graceful shutdown]
    A -->|agent/ping| E[Health check]

    B --> F[ACP processes in Claude API]
    C --> G[Returns: ACTIVE/IDLE/BUSY]
    D --> H[Cleanup and exit]
    E --> I[Returns: pong]

    style B fill:#e1ffe1
```

### Implementation: Send Message to ACP

```go
// pkg/acp/client.go
func (c *Client) SendMessage(content string) (interface{}, error) {
    // Build JSON-RPC request
    request := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      c.nextID(),
        "method":  "agent/sendMessage",
        "params": map[string]interface{}{
            "content": content,
        },
    }

    // Encode to JSON
    reqBytes, _ := json.Marshal(request)

    // Write to ACP stdin (via transport)
    _, err := c.transport.Write(reqBytes)
    if err != nil {
        return nil, fmt.Errorf("failed to write request: %w", err)
    }
    _, err = c.transport.Write([]byte("\n")) // Newline delimiter
    if err != nil {
        return nil, fmt.Errorf("failed to write delimiter: %w", err)
    }

    // Read JSON-RPC response from ACP stdout
    respBytes := make([]byte, 65536)
    n, err := c.transport.Read(respBytes)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    // Parse response
    var response map[string]interface{}
    err = json.Unmarshal(respBytes[:n], &response)
    if err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    // Check for errors
    if errObj, ok := response["error"]; ok {
        return nil, fmt.Errorf("RPC error: %v", errObj)
    }

    return response["result"], nil
}
```

## Complete End-to-End Flow

Putting it all together: User sends message from PWA to agent running in container.

```mermaid
sequenceDiagram
    autonumber
    participant User
    participant PWA
    participant WS Handler
    participant SessionMgr
    participant ACPClient
    participant Transport
    participant Docker
    participant Container
    participant ACP

    User->>PWA: Types "Write a function"
    PWA->>WS Handler: WebSocket TEXT frame<br/>{"type":"agent:message", "agentId":"coder",<br/>"content":"Write a function"}

    Note over WS Handler: gorilla/websocket<br/>ReadMessage()

    WS Handler->>SessionMgr: RouteMessage(sessionID, msg)
    SessionMgr->>SessionMgr: Lookup UserSession<br/>Lookup AgentSession("coder")

    SessionMgr->>ACPClient: SendMessage("Write a function")

    ACPClient->>ACPClient: Build JSON-RPC request<br/>id:42, method:agent/sendMessage

    ACPClient->>Transport: Write(jsonBytes + "\n")

    alt Container Mode
        Transport->>Docker: Write to hijacked connection<br/>[no header, raw stdin]
        Docker->>Container: Forward to stdin
        Container->>ACP: stdin receives JSON-RPC
    else Host Mode
        Transport->>ACP: os/exec stdin pipe
    end

    Note over ACP: Process message<br/>Call Claude API<br/>Generate response

    ACP->>Container: stdout: {"jsonrpc":"2.0","id":42,<br/>"result":{"response":"..."}}
    Container->>Docker: stdout
    Docker->>Transport: Multiplexed stream<br/>[0x01 header][42 bytes][JSON]

    Note over Transport: Demux goroutine<br/>stdcopy.StdCopy

    Transport->>ACPClient: Read() → JSON bytes
    ACPClient->>ACPClient: json.Unmarshal<br/>Extract result

    ACPClient->>SessionMgr: result: {"response":"Here's a function..."}
    SessionMgr->>WS Handler: agent:response message
    WS Handler->>PWA: WebSocket TEXT frame<br/>{"type":"agent:response", "agentId":"coder",<br/>"content":"Here's a function..."}

    PWA->>User: Display response in chat

    Note over User,ACP: Total latency: 200ms-3s<br/>(depends on Claude API)
```

### Timing Breakdown

Typical latency for a message round-trip:

```mermaid
gantt
    title Message Latency Breakdown
    dateFormat X
    axisFormat %L ms

    section Browser
    PWA send           :0, 1
    section Network
    WebSocket          :1, 3
    section Relay
    Unmarshal          :3, 4
    Route to session   :4, 5
    section ACP Client
    Build JSON-RPC     :5, 6
    Write to transport :6, 7
    section Docker
    Stream multiplex   :7, 8
    section Container
    ACP receives       :8, 9
    section Claude API
    API call           :9, 2500
    section Return Path
    Response parsing   :2500, 2502
    WebSocket send     :2502, 2505

    section Total
    Round trip         :crit, 0, 2505
```

**Breakdown:**
- Browser → Relay: 2-5ms (WebSocket)
- Relay routing: 1-2ms (in-process)
- Transport write: <1ms (pipe/connection)
- Docker multiplex: 1-2ms
- **Claude API call: 500ms-3s** (95% of latency)
- Response path: 5-10ms

## Error Handling Across Layers

Errors can occur at any layer. Here's how they propagate:

```mermaid
graph TD
    A[PWA sends message] --> B{WebSocket connected?}
    B -->|No| C[Browser error:<br/>Connection closed]
    B -->|Yes| D{Session exists?}
    D -->|No| E[Relay error:<br/>SESSION_NOT_FOUND]
    D -->|Yes| F{Agent exists?}
    F -->|No| G[Relay error:<br/>AGENT_NOT_FOUND]
    F -->|Yes| H{Container running?}
    H -->|No| I[Container error:<br/>Container stopped]
    H -->|Yes| J{Attach successful?}
    J -->|No| K[Docker error:<br/>Attach failed]
    J -->|Yes| L{ACP responds?}
    L -->|No| M[Timeout error:<br/>ACP unresponsive]
    L -->|JSON-RPC error| N[ACP error:<br/>RPC error object]
    L -->|Success| O[Response delivered]

    style C fill:#ffcccc
    style E fill:#ffcccc
    style G fill:#ffcccc
    style I fill:#ffcccc
    style K fill:#ffcccc
    style M fill:#ffcccc
    style N fill:#ffcccc
    style O fill:#ccffcc
```

### Error Response Examples

**Layer 1 (WebSocket):**

```json
{
  "type": "error",
  "error": {
    "code": "WEBSOCKET_CLOSED",
    "message": "WebSocket connection closed unexpectedly",
    "recoverable": false
  }
}
```

**Layer 2 (Session Manager):**

```json
{
  "type": "error",
  "error": {
    "code": "SESSION_NOT_FOUND",
    "message": "Session not found: abc123",
    "recoverable": false
  }
}
```

**Layer 4 (Docker):**

```json
{
  "type": "error",
  "error": {
    "code": "CONTAINER_STOPPED",
    "message": "Container exited with code 137 (OOMKilled)",
    "recoverable": true,
    "details": {
      "containerID": "a1b2c3d4",
      "exitCode": 137
    }
  }
}
```

**Layer 5 (JSON-RPC):**

```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "error": {
    "code": -32603,
    "message": "Internal error",
    "data": {
      "detail": "Claude API rate limit exceeded"
    }
  }
}
```

## Observability

### Message Tracing

Each message gets a unique trace ID that flows through all layers:

```mermaid
sequenceDiagram
    participant PWA
    participant Relay
    participant ACP

    Note over PWA: Generate traceID:<br/>trace-abc123

    PWA->>Relay: {"type":"agent:message",<br/>"traceId":"trace-abc123", ...}

    Note over Relay: Log: [trace-abc123] Routing to agent

    Relay->>ACP: {"jsonrpc":"2.0", "id":42,<br/>"params":{"traceId":"trace-abc123", ...}}

    Note over ACP: Log: [trace-abc123] Processing message

    ACP->>Relay: {"result":{"traceId":"trace-abc123", ...}}

    Note over Relay: Log: [trace-abc123] Sending response

    Relay->>PWA: {"type":"agent:response",<br/>"traceId":"trace-abc123", ...}
```

### Logging at Each Layer

```go
// Layer 1: WebSocket
log.Printf("[WS] [trace=%s] Received message type=%s", traceID, msg.Type)

// Layer 2: Session Manager
log.Printf("[SESSION] [trace=%s] Routing to agent=%s session=%s", traceID, agentID, sessionID)

// Layer 3: ACP Client
log.Printf("[ACP] [trace=%s] Sending JSON-RPC method=%s", traceID, method)

// Layer 4: Container
log.Printf("[DOCKER] [trace=%s] Writing to container=%s", traceID, containerID[:12])

// Layer 5: ACP stderr
log.Printf("[ACP stderr container=%s] [trace=%s] Processing request", containerID[:12], traceID)
```

## Performance Optimization

### Batching Messages

For high-throughput scenarios, batch multiple messages:

```javascript
// PWA batches messages
const batch = [
    {type: "agent:message", agentId: "coder", content: "Task 1"},
    {type: "agent:message", agentId: "coder", content: "Task 2"},
    {type: "agent:message", agentId: "coder", content: "Task 3"}
];

ws.send(JSON.stringify({type: "batch", messages: batch}));
```

### Connection Pooling

Relay maintains persistent connections to containers:

```mermaid
graph LR
    A[Relay Server] --> B[Connection Pool]
    B --> C[Container 1<br/>Keep-alive]
    B --> D[Container 2<br/>Keep-alive]
    B --> E[Container 3<br/>Keep-alive]

    style B fill:#e1f5ff
```

### Stream Buffering

Transport layer uses buffered I/O:

```go
// Buffered writer for better throughput
writer := bufio.NewWriter(transport)
writer.Write(jsonBytes)
writer.Flush() // Explicit flush for request/response
```

## Related Documentation

- [WebSocket API Reference](WEBSOCKET_API.md) - Complete message schemas
- [ACP Protocol Specification](ACP_PROTOCOL.md) - JSON-RPC details
- [Container Modes](CONTAINER_MODES.md) - Host vs Container execution
- [Protocol Inspector](../operations/PROTOCOL_INSPECTOR.md) - Live debugging tool
