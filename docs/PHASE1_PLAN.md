# Phase 1 Implementation Plan

## Overview

Build foundation infrastructure in order of dependencies. Each step builds on the previous.

## Task Breakdown

### 1. Project Setup (1 day)

#### 1.1 Initialize Go Module
```bash
cd /Users/clint/code/ourocodus
go mod init github.com/yourusername/ourocodus
```

**Files to create:**
- `go.mod`
- `go.sum` (generated)

#### 1.2 Create Directory Structure
```bash
mkdir -p cmd/{relay,echo-agent,api,cli}
mkdir -p pkg/{relay,nats}
mkdir -p web
mkdir -p config
mkdir -p test
```

#### 1.3 Add Dependencies
```bash
go get github.com/nats-io/nats.go
go get github.com/gorilla/websocket
go get github.com/spf13/cobra  # for CLI
```

#### 1.4 Create Makefile
See Phase 1 spec for content

#### 1.5 Create .gitignore
```
bin/
*.log
.env
```

**Success:** Project structure exists, dependencies installed

---

### 2. NATS Client Wrapper (2 hours)

Build a simple wrapper around NATS client for consistent usage.

**File:** `pkg/nats/client.go`

```go
package nats

import (
    "github.com/nats-io/nats.go"
)

type Client struct {
    conn *nats.Conn
}

func Connect(url string) (*Client, error) {
    nc, err := nats.Connect(url)
    if err != nil {
        return nil, err
    }
    return &Client{conn: nc}, nil
}

func (c *Client) Publish(topic string, data []byte) error {
    return c.conn.Publish(topic, data)
}

func (c *Client) Subscribe(topic string, handler func([]byte)) error {
    _, err := c.conn.Subscribe(topic, func(msg *nats.Msg) {
        handler(msg.Data)
    })
    return err
}

func (c *Client) Close() {
    c.conn.Close()
}
```

**File:** `pkg/nats/client_test.go`

```go
package nats

import (
    "testing"
)

func TestConnect(t *testing.T) {
    // Test with invalid URL
    _, err := Connect("nats://invalid:9999")
    if err == nil {
        t.Error("expected error for invalid URL")
    }
}
```

**Success:** Can connect to NATS, publish, subscribe

---

### 3. Relay Core Types (1 hour)

Define data structures used throughout relay.

**File:** `pkg/relay/types.go`

```go
package relay

import (
    "sync"
    "github.com/gorilla/websocket"
)

// Connection represents an agent WebSocket connection
type Connection struct {
    SessionID string
    Role      string
    Conn      *websocket.Conn
}

// Registry tracks active agent connections
type Registry struct {
    mu          sync.RWMutex
    connections map[string]*Connection // key: "session_id:role"
}

func NewRegistry() *Registry {
    return &Registry{
        connections: make(map[string]*Connection),
    }
}

func (r *Registry) Add(sessionID, role string, conn *websocket.Conn) {
    r.mu.Lock()
    defer r.mu.Unlock()

    key := sessionID + ":" + role
    r.connections[key] = &Connection{
        SessionID: sessionID,
        Role:      role,
        Conn:      conn,
    }
}

func (r *Registry) Get(sessionID, role string) *Connection {
    r.mu.RLock()
    defer r.mu.RUnlock()

    key := sessionID + ":" + role
    return r.connections[key]
}

func (r *Registry) Remove(sessionID, role string) {
    r.mu.Lock()
    defer r.mu.Unlock()

    key := sessionID + ":" + role
    delete(r.connections, key)
}

// Message is the standard message format
type Message struct {
    ID      string                 `json:"id"`
    Type    string                 `json:"type"`
    Payload map[string]interface{} `json:"payload"`
}
```

**File:** `pkg/relay/types_test.go`

```go
package relay

import (
    "testing"
)

func TestRegistryAddGet(t *testing.T) {
    r := NewRegistry()
    r.Add("sess_123", "test", nil) // nil conn for test

    conn := r.Get("sess_123", "test")
    if conn == nil {
        t.Error("expected connection, got nil")
    }
    if conn.SessionID != "sess_123" {
        t.Errorf("expected sess_123, got %s", conn.SessionID)
    }
}

func TestRegistryRemove(t *testing.T) {
    r := NewRegistry()
    r.Add("sess_123", "test", nil)
    r.Remove("sess_123", "test")

    conn := r.Get("sess_123", "test")
    if conn != nil {
        t.Error("expected nil after remove")
    }
}
```

**Success:** Registry can add/get/remove connections

---

### 4. Relay WebSocket Server (4 hours)

**File:** `pkg/relay/server.go`

```go
package relay

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // Allow all origins for POC
    },
}

type Server struct {
    registry *Registry
    natsConn NATSPublisher // interface for testing
}

type NATSPublisher interface {
    Publish(topic string, data []byte) error
}

func NewServer(natsConn NATSPublisher) *Server {
    return &Server{
        registry: NewRegistry(),
        natsConn: natsConn,
    }
}

func (s *Server) HandleAgentConnection(w http.ResponseWriter, r *http.Request) {
    // Get query params
    sessionID := r.URL.Query().Get("session_id")
    role := r.URL.Query().Get("role")

    if sessionID == "" || role == "" {
        http.Error(w, "missing session_id or role", http.StatusBadRequest)
        return
    }

    // Upgrade to WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("upgrade error: %v", err)
        return
    }

    log.Printf("Agent connected: session=%s role=%s", sessionID, role)

    // Register connection
    s.registry.Add(sessionID, role, conn)
    defer s.registry.Remove(sessionID, role)
    defer conn.Close()

    // Read messages from agent
    for {
        var msg Message
        if err := conn.ReadJSON(&msg); err != nil {
            log.Printf("read error: %v", err)
            break
        }

        log.Printf("Received from agent: %+v", msg)

        // Publish to NATS results topic
        topic := fmt.Sprintf("sessions.%s.results.%s", sessionID, role)
        data, _ := json.Marshal(msg)

        if err := s.natsConn.Publish(topic, data); err != nil {
            log.Printf("publish error: %v", err)
        }
    }
}

func (s *Server) ForwardToAgent(sessionID, role string, msg Message) error {
    conn := s.registry.Get(sessionID, role)
    if conn == nil {
        return fmt.Errorf("no connection for session=%s role=%s", sessionID, role)
    }

    log.Printf("Forwarding to agent: %+v", msg)
    return conn.Conn.WriteJSON(msg)
}
```

**Success:** WebSocket server accepts connections, forwards messages

---

### 5. Relay NATS Handler (2 hours)

**File:** `pkg/relay/nats_handler.go`

```go
package relay

import (
    "encoding/json"
    "log"
    "strings"
)

type NATSHandler struct {
    server *Server
}

func NewNATSHandler(server *Server) *NATSHandler {
    return &NATSHandler{server: server}
}

func (h *NATSHandler) HandleWorkMessage(data []byte) {
    var msg Message
    if err := json.Unmarshal(data, &msg); err != nil {
        log.Printf("unmarshal error: %v", err)
        return
    }

    log.Printf("Received work message: %+v", msg)

    // Extract session_id and role from message or topic
    // For now, expect them in payload
    sessionID, _ := msg.Payload["session_id"].(string)
    role, _ := msg.Payload["role"].(string)

    if sessionID == "" || role == "" {
        log.Printf("missing session_id or role in payload")
        return
    }

    // Forward to agent
    if err := h.server.ForwardToAgent(sessionID, role, msg); err != nil {
        log.Printf("forward error: %v", err)
    }
}

// ParseTopic extracts session_id and role from NATS topic
// Topic format: sessions.{session_id}.work.{role}
func ParseTopic(topic string) (sessionID, role string, ok bool) {
    parts := strings.Split(topic, ".")
    if len(parts) != 4 {
        return "", "", false
    }
    return parts[1], parts[3], true
}
```

**Success:** Can parse topics, forward NATS messages to agents

---

### 6. Relay Main (2 hours)

**File:** `cmd/relay/main.go`

```go
package main

import (
    "log"
    "net/http"
    "os"

    "github.com/yourusername/ourocodus/pkg/nats"
    "github.com/yourusername/ourocodus/pkg/relay"
)

func main() {
    // Get config from env
    natsURL := getEnv("NATS_URL", "nats://localhost:4222")
    port := getEnv("PORT", "8080")

    // Connect to NATS
    nc, err := nats.Connect(natsURL)
    if err != nil {
        log.Fatalf("nats connect: %v", err)
    }
    defer nc.Close()

    log.Printf("Connected to NATS: %s", natsURL)

    // Create server
    server := relay.NewServer(nc)
    handler := relay.NewNATSHandler(server)

    // Subscribe to work messages (wildcard)
    if err := nc.Subscribe("sessions.*.work.*", func(data []byte) {
        handler.HandleWorkMessage(data)
    }); err != nil {
        log.Fatalf("subscribe: %v", err)
    }

    log.Println("Subscribed to sessions.*.work.*")

    // Start WebSocket server
    http.HandleFunc("/agent", server.HandleAgentConnection)

    log.Printf("Relay listening on :%s", port)
    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Fatalf("listen: %v", err)
    }
}

func getEnv(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}
```

**File:** `cmd/relay/Dockerfile`

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o relay cmd/relay/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/relay .

EXPOSE 8080
CMD ["./relay"]
```

**Success:** Relay runs, connects to NATS, serves WebSocket

---

### 7. Echo Agent (2 hours)

**File:** `cmd/echo-agent/main.go`

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/gorilla/websocket"
)

type Message struct {
    ID      string                 `json:"id"`
    Type    string                 `json:"type"`
    Payload map[string]interface{} `json:"payload"`
}

func main() {
    sessionID := os.Getenv("SESSION_ID")
    role := os.Getenv("ROLE")
    relayURL := getEnv("RELAY_URL", "ws://localhost:8080")

    if sessionID == "" || role == "" {
        log.Fatal("SESSION_ID and ROLE required")
    }

    url := fmt.Sprintf("%s/agent?session_id=%s&role=%s", relayURL, sessionID, role)

    log.Printf("Connecting to %s", url)

    conn, _, err := websocket.DefaultDialer.Dial(url, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    log.Println("Connected to relay")

    for {
        var msg Message
        if err := conn.ReadJSON(&msg); err != nil {
            log.Printf("read error: %v", err)
            break
        }

        log.Printf("Received: %+v", msg)

        // Echo back
        response := Message{
            ID:   msg.ID,
            Type: "result",
            Payload: map[string]interface{}{
                "echo":      msg.Payload,
                "timestamp": time.Now().Format(time.RFC3339),
                "agent":     fmt.Sprintf("%s:%s", sessionID, role),
            },
        }

        if err := conn.WriteJSON(response); err != nil {
            log.Printf("write error: %v", err)
            break
        }

        log.Println("Sent response")
    }
}

func getEnv(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}
```

**File:** `cmd/echo-agent/Dockerfile`

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o echo-agent cmd/echo-agent/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/echo-agent .

CMD ["./echo-agent"]
```

**Success:** Echo agent connects to relay, echoes messages

---

### 8. CLI (4 hours)

**File:** `cmd/cli/main.go`

Using `cobra` for CLI structure:

```go
package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "ourocodus",
    Short: "Ourocodus CLI",
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

**File:** `cmd/cli/cmd/send.go`

```go
package cmd

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/spf13/cobra"
    "github.com/yourusername/ourocodus/pkg/nats"
)

var sendCmd = &cobra.Command{
    Use:   "send",
    Short: "Send message to NATS",
}

var sendWorkCmd = &cobra.Command{
    Use:   "work",
    Short: "Send work message",
    Run:   runSendWork,
}

var (
    sessionID string
    role      string
    message   string
)

func init() {
    sendWorkCmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
    sendWorkCmd.Flags().StringVar(&role, "role", "", "Agent role")
    sendWorkCmd.Flags().StringVar(&message, "message", "", "Message payload")
    sendWorkCmd.MarkFlagRequired("session")
    sendWorkCmd.MarkFlagRequired("role")
    sendWorkCmd.MarkFlagRequired("message")

    sendCmd.AddCommand(sendWorkCmd)
    rootCmd.AddCommand(sendCmd)
}

func runSendWork(cmd *cobra.Command, args []string) {
    nc, err := nats.Connect("nats://localhost:4222")
    if err != nil {
        log.Fatal(err)
    }
    defer nc.Close()

    msg := map[string]interface{}{
        "id":   generateID(),
        "type": "work",
        "payload": map[string]interface{}{
            "session_id": sessionID,
            "role":       role,
            "message":    message,
        },
    }

    data, _ := json.Marshal(msg)
    topic := fmt.Sprintf("sessions.%s.work.%s", sessionID, role)

    if err := nc.Publish(topic, data); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("✓ Published to %s\n", topic)
}

func generateID() string {
    return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
```

**File:** `cmd/cli/cmd/tail.go`

```go
package cmd

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/spf13/cobra"
    "github.com/yourusername/ourocodus/pkg/nats"
)

var tailCmd = &cobra.Command{
    Use:   "tail",
    Short: "Tail NATS messages",
}

var tailResultsCmd = &cobra.Command{
    Use:   "results",
    Short: "Tail results",
    Run:   runTailResults,
}

func init() {
    tailResultsCmd.Flags().StringVar(&sessionID, "session", "", "Session ID")
    tailResultsCmd.Flags().StringVar(&role, "role", "", "Agent role")
    tailResultsCmd.MarkFlagRequired("session")

    tailCmd.AddCommand(tailResultsCmd)
    rootCmd.AddCommand(tailCmd)
}

func runTailResults(cmd *cobra.Command, args []string) {
    nc, err := nats.Connect("nats://localhost:4222")
    if err != nil {
        log.Fatal(err)
    }
    defer nc.Close()

    topic := fmt.Sprintf("sessions.%s.results.*", sessionID)
    if role != "" {
        topic = fmt.Sprintf("sessions.%s.results.%s", sessionID, role)
    }

    fmt.Printf("Waiting for messages on %s...\n", topic)

    if err := nc.Subscribe(topic, func(data []byte) {
        var msg map[string]interface{}
        json.Unmarshal(data, &msg)

        pretty, _ := json.MarshalIndent(msg, "", "  ")
        fmt.Println(string(pretty))
    }); err != nil {
        log.Fatal(err)
    }

    // Block forever
    select {}
}
```

**Additional commands to implement:**
- `cmd/cli/cmd/agent.go` - Start/stop/list agents (uses Docker SDK)
- `cmd/cli/cmd/start.go` - Start system (docker-compose up)
- `cmd/cli/cmd/stop.go` - Stop system (docker-compose down)

**Success:** CLI can send messages and tail results

---

### 9. API Server (3 hours)

**File:** `cmd/api/main.go`

```go
package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os"

    "github.com/yourusername/ourocodus/pkg/nats"
)

var nc *nats.Client

func main() {
    var err error
    nc, err = nats.Connect(getEnv("NATS_URL", "nats://localhost:4222"))
    if err != nil {
        log.Fatal(err)
    }
    defer nc.Close()

    http.HandleFunc("/health", handleHealth)
    http.HandleFunc("/api/agents", handleAgents)
    http.HandleFunc("/api/events", handleEvents)
    http.Handle("/", http.FileServer(http.Dir("./web")))

    port := getEnv("PORT", "9000")
    log.Printf("API server listening on :%s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "nats":   "connected",
    })
}

func handleAgents(w http.ResponseWriter, r *http.Request) {
    // TODO: Query Docker for running containers
    agents := []map[string]interface{}{
        {
            "id":         "agent_abc",
            "session_id": "sess_123",
            "role":       "test",
            "status":     "running",
        },
    }
    json.NewEncoder(w).Encode(map[string]interface{}{
        "agents": agents,
    })
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
    // SSE endpoint
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    // Subscribe to events
    nc.Subscribe("sessions.*.events", func(data []byte) {
        fmt.Fprintf(w, "data: %s\n\n", string(data))
        if f, ok := w.(http.Flusher); ok {
            f.Flush()
        }
    })

    // Block
    <-r.Context().Done()
}

func getEnv(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}
```

**Success:** API server serves health, agents, events endpoints

---

### 10. Web UI (1 hour)

**File:** `web/index.html`

```html
<!DOCTYPE html>
<html>
<head>
    <title>Ourocodus Monitor</title>
    <style>
        body {
            font-family: monospace;
            padding: 20px;
            background: #1e1e1e;
            color: #d4d4d4;
        }
        h1 { color: #4ec9b0; }
        h2 { color: #dcdcaa; }
        pre {
            background: #252526;
            padding: 10px;
            border-radius: 4px;
            overflow-x: auto;
        }
        ul { list-style: none; padding: 0; }
        li {
            padding: 8px;
            margin: 4px 0;
            background: #252526;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <h1>Ourocodus - Phase 1 Monitor</h1>

    <h2>System Health</h2>
    <div id="health">Loading...</div>

    <h2>Active Agents</h2>
    <ul id="agents">Loading...</ul>

    <h2>Event Log</h2>
    <pre id="events"></pre>

    <script>
        // Health check
        fetch('/health')
            .then(r => r.json())
            .then(data => {
                document.getElementById('health').textContent =
                    `Status: ${data.status} | NATS: ${data.nats}`;
            });

        // Agents
        function loadAgents() {
            fetch('/api/agents')
                .then(r => r.json())
                .then(data => {
                    const list = document.getElementById('agents');
                    list.innerHTML = '';
                    data.agents.forEach(a => {
                        const li = document.createElement('li');
                        li.textContent = `${a.id} - ${a.role} - ${a.status}`;
                        list.appendChild(li);
                    });
                });
        }
        loadAgents();
        setInterval(loadAgents, 5000);

        // Event stream
        const events = new EventSource('/api/events');
        events.onmessage = (e) => {
            const log = document.getElementById('events');
            log.textContent = e.data + '\n' + log.textContent;
        };
    </script>
</body>
</html>
```

**Success:** Web UI shows health, agents, live events

---

### 11. Docker Compose (1 hour)

**File:** `docker-compose.yml`

```yaml
version: '3.8'

services:
  nats:
    image: nats:latest
    ports:
      - "4222:4222"
    networks:
      - ourocodus

  relay:
    build:
      context: .
      dockerfile: cmd/relay/Dockerfile
    ports:
      - "8080:8080"
    environment:
      - NATS_URL=nats://nats:4222
      - PORT=8080
    depends_on:
      - nats
    networks:
      - ourocodus

  api:
    build:
      context: .
      dockerfile: cmd/api/Dockerfile
    ports:
      - "9000:9000"
    environment:
      - NATS_URL=nats://nats:4222
      - PORT=9000
    volumes:
      - ./web:/root/web
    depends_on:
      - nats
    networks:
      - ourocodus

networks:
  ourocodus:
    driver: bridge
```

**Success:** `docker-compose up` starts full system

---

### 12. Testing (2 days)

#### Integration Test

**File:** `test/integration_test.go`

```go
package test

import (
    "testing"
    "time"
    // ... imports
)

func TestE2E(t *testing.T) {
    // Start NATS
    // Start relay
    // Connect echo agent
    // Publish message
    // Assert response received
}
```

#### E2E Script

**File:** `test/e2e.sh`

```bash
#!/bin/bash
set -e

echo "Starting system..."
docker-compose up -d

echo "Waiting for system..."
sleep 5

echo "Building CLI..."
go build -o bin/ourocodus cmd/cli/main.go

echo "Starting echo agent..."
docker run -d --name test-agent \
  --network ourocodus_ourocodus \
  -e SESSION_ID=test \
  -e ROLE=test \
  -e RELAY_URL=ws://relay:8080 \
  ourocodus/echo-agent

sleep 2

echo "Sending message..."
./bin/ourocodus send work --session test --role test --message "hello"

echo "Checking results..."
timeout 5s ./bin/ourocodus tail results --session test --role test | grep -q "hello"

echo "✓ E2E test passed"

docker-compose down
docker rm -f test-agent
```

**Success:** E2E test passes

---

### 13. Documentation (1 day)

- README.md with quickstart
- ARCHITECTURE.md with diagrams
- Update PHASE1.md with learnings

---

## Execution Order

```
Day 1:
  - Task 1: Project setup
  - Task 2: NATS wrapper
  - Task 3: Relay types

Day 2:
  - Task 4: Relay WebSocket server
  - Task 5: Relay NATS handler

Day 3:
  - Task 6: Relay main
  - Task 7: Echo agent
  - Test relay + echo agent manually

Day 4:
  - Task 8: CLI (send, tail, agent commands)
  - Test full flow via CLI

Day 5:
  - Task 9: API server
  - Task 10: Web UI
  - Task 11: Docker Compose

Week 2:
  - Task 12: Testing (write tests, fix bugs)
  - Task 13: Documentation
  - Demo and polish
```

## Success Criteria Checklist

At end of Phase 1, we should be able to:

- [ ] Run `make build` - all binaries compile
- [ ] Run `docker-compose up` - system starts
- [ ] Run `ourocodus agent start --session test --role test` - agent connects
- [ ] Run `ourocodus send work --session test --message "hello"` - message sends
- [ ] Run `ourocodus tail results --session test` - see echo response
- [ ] Open `http://localhost:9000` - see web UI
- [ ] Web UI shows active agent
- [ ] Web UI shows live events
- [ ] Run `test/e2e.sh` - passes
- [ ] All unit tests pass
- [ ] All integration tests pass

## Next Actions

1. Initialize Go module
2. Create directory structure
3. Start with Task 2 (NATS wrapper)
4. Build incrementally, test as you go

Ready to start implementation?
