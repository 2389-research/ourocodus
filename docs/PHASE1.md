# Phase 1: Foundation with Real ACP - Detailed Specification

## Goal

**Prove the multi-agent orchestration works.** Can PWA route messages to 3 concurrent Claude Code instances, each working on their own git worktree?

**Real ACP. Real Claude Code. Real git operations.**

## What Success Looks Like

```bash
# Terminal 1: Start the system
$ ourocodus start
✓ NATS running on localhost:4222
✓ Relay running on localhost:8080
✓ API running on localhost:9000
✓ PWA served on localhost:3000

# Browser: Open http://localhost:3000
# Click "New Project"
# System spawns 3 Claude Code instances:
#   - Agent 1 (auth) → worktree: agent/auth
#   - Agent 2 (db) → worktree: agent/db
#   - Agent 3 (tests) → worktree: agent/tests

# PWA shows 3 agent cards

# User clicks "Auth Agent"
# Types: "Implement JWT authentication in Go"
# Claude Code (via ACP):
#   - Creates auth.go
#   - Writes JWT implementation
#   - Commits to agent/auth branch
#   - Responds: "I've implemented JWT auth..."

# User clicks "Database Agent"
# Types: "Create PostgreSQL schema for users"
# Claude Code (via ACP):
#   - Creates schema.sql
#   - Defines user table
#   - Commits to agent/db branch

# Both agents work concurrently, isolated conversations
```

That's it. If that works, Phase 1 is done.

## Key Decisions (From Zen Consultation)

1. **Processes, not containers** - Run Claude Code ACP as processes for simplicity
2. **stdio communication** - Relay spawns processes, uses stdin/stdout for JSON-RPC
3. **Relay holds state** - In-memory conversation history per agent (acceptable for POC)
4. **Shell script for git** - Simple bash script creates worktrees before spawning agents
5. **Full scope** - 3 concurrent agents, real git ops, validates the real system

## Architecture

```
┌─────────────────────────────────────────┐
│  PWA (React/Vanilla JS)                 │
│  http://localhost:3000                  │
│                                         │
│  [Auth Agent]  [DB Agent]  [Tests]     │
│      ↓             ↓          ↓         │
│  WebSocket connection to relay          │
└──────────────────┬──────────────────────┘
                   │ WebSocket
                   │
┌──────────────────▼──────────────────────┐
│  Relay (Go)                              │
│  - Receives messages from PWA            │
│  - Routes to correct agent process       │
│  - Translates: internal JSON ↔ ACP      │
│  - Maintains session state (in-memory)   │
└─┬────────┬────────┬──────────────────────┘
  │ stdio  │ stdio  │ stdio
  │ (JSON- │        │
  │  RPC)  │        │
┌─▼────────▼────────▼──────────┐
│ Claude Code ACP Processes    │
│                               │
│ Process 1:                    │
│ claude-code-acp               │
│ --workspace=agent/auth        │
│                               │
│ Process 2:                    │
│ claude-code-acp               │
│ --workspace=agent/db          │
│                               │
│ Process 3:                    │
│ claude-code-acp               │
│ --workspace=agent/tests       │
└───────────┬───────────────────┘
            │
            │ git operations
            │
┌───────────▼───────────────────┐
│  Git Repository               │
│                               │
│  main/                        │
│  agent/                       │
│    auth/    (worktree)        │
│    db/      (worktree)        │
│    tests/   (worktree)        │
└───────────────────────────────┘
```

## Components to Build

### 1. Setup Script

Creates git worktrees before system starts.

**File:** `scripts/setup-worktrees.sh`

```bash
#!/bin/bash
set -e

REPO_PATH=${1:-.}
cd "$REPO_PATH"

echo "Creating git worktrees..."

# Create worktrees for each agent
git worktree add agent/auth -b agent/auth || echo "agent/auth exists"
git worktree add agent/db -b agent/db || echo "agent/db exists"
git worktree add agent/tests -b agent/tests || echo "agent/tests exists"

echo "✓ Worktrees ready"
echo "  - agent/auth"
echo "  - agent/db"
echo "  - agent/tests"
```

### 2. ACP Message Types (pkg/acp)

**File:** `pkg/acp/types.go`

```go
package acp

// JSON-RPC 2.0 message structures for ACP

type Request struct {
    JSONRPC string      `json:"jsonrpc"` // "2.0"
    ID      interface{} `json:"id"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string      `json:"jsonrpc"` // "2.0"
    ID      interface{} `json:"id"`
    Result  interface{} `json:"result,omitempty"`
    Error   *Error      `json:"error,omitempty"`
}

type Error struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// ACP-specific methods
const (
    MethodSendMessage = "agent/sendMessage"
    MethodGetContext  = "agent/getContext"
    MethodToolCall    = "agent/toolCall"
)

// Message from user to agent
type SendMessageParams struct {
    Content string   `json:"content"`
    Images  []string `json:"images,omitempty"`
}

// Agent response with possible tool calls
type AgentMessage struct {
    Type    string      `json:"type"` // "text" or "toolCall"
    Content string      `json:"content,omitempty"`
    ToolCall *ToolCall  `json:"toolCall,omitempty"`
}

type ToolCall struct {
    Name  string                 `json:"name"`
    Args  map[string]interface{} `json:"args"`
}
```

### 3. ACP Client (pkg/acp/client.go)

Manages communication with one Claude Code process.

```go
package acp

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "os/exec"
    "sync"
)

type Client struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    mu     sync.Mutex
    nextID int
}

func NewClient(workspace string, apiKey string) (*Client, error) {
    cmd := exec.Command("claude-code-acp", "--workspace", workspace)
    cmd.Env = append(cmd.Env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", apiKey))

    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, err
    }

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }

    if err := cmd.Start(); err != nil {
        return nil, err
    }

    return &Client{
        cmd:    cmd,
        stdin:  stdin,
        stdout: stdout,
    }, nil
}

func (c *Client) SendMessage(content string) (*AgentMessage, error) {
    c.mu.Lock()
    id := c.nextID
    c.nextID++
    c.mu.Unlock()

    req := Request{
        JSONRPC: "2.0",
        ID:      id,
        Method:  MethodSendMessage,
        Params: SendMessageParams{
            Content: content,
        },
    }

    // Write request to stdin
    data, _ := json.Marshal(req)
    data = append(data, '\n')
    if _, err := c.stdin.Write(data); err != nil {
        return nil, err
    }

    // Read response from stdout
    scanner := bufio.NewScanner(c.stdout)
    if !scanner.Scan() {
        return nil, fmt.Errorf("no response")
    }

    var resp Response
    if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
        return nil, err
    }

    if resp.Error != nil {
        return nil, fmt.Errorf("acp error: %s", resp.Error.Message)
    }

    // Parse result as AgentMessage
    var msg AgentMessage
    resultData, _ := json.Marshal(resp.Result)
    json.Unmarshal(resultData, &msg)

    return &msg, nil
}

func (c *Client) Close() error {
    c.stdin.Close()
    return c.cmd.Wait()
}
```

### 4. Relay with ACP Integration

**File:** `pkg/relay/relay.go`

```go
package relay

import (
    "encoding/json"
    "fmt"
    "log"
    "sync"

    "github.com/gorilla/websocket"
    "github.com/yourusername/ourocodus/pkg/acp"
)

type Relay struct {
    sessions map[string]*Session // session_id → session
    mu       sync.RWMutex
}

type Session struct {
    ID      string
    Agents  map[string]*Agent // role → agent
    PWAConn *websocket.Conn
}

type Agent struct {
    Role      string
    ACPClient *acp.Client
    History   []Message // Conversation history
}

type Message struct {
    From    string `json:"from"` // "user" or "agent"
    Content string `json:"content"`
}

func NewRelay() *Relay {
    return &Relay{
        sessions: make(map[string]*Session),
    }
}

func (r *Relay) CreateSession(sessionID string, pwaConn *websocket.Conn, apiKey string) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Create ACP clients for 3 agents
    agents := make(map[string]*Agent)

    roles := []string{"auth", "db", "tests"}
    for _, role := range roles {
        workspace := fmt.Sprintf("agent/%s", role)
        client, err := acp.NewClient(workspace, apiKey)
        if err != nil {
            return err
        }

        agents[role] = &Agent{
            Role:      role,
            ACPClient: client,
            History:   []Message{},
        }

        log.Printf("Spawned ACP agent: %s at %s", role, workspace)
    }

    r.sessions[sessionID] = &Session{
        ID:      sessionID,
        Agents:  agents,
        PWAConn: pwaConn,
    }

    return nil
}

func (r *Relay) SendToAgent(sessionID, role, message string) error {
    r.mu.RLock()
    session := r.sessions[sessionID]
    r.mu.RUnlock()

    if session == nil {
        return fmt.Errorf("session not found")
    }

    agent := session.Agents[role]
    if agent == nil {
        return fmt.Errorf("agent not found: %s", role)
    }

    // Add to history
    agent.History = append(agent.History, Message{From: "user", Content: message})

    // Send to ACP
    resp, err := agent.ACPClient.SendMessage(message)
    if err != nil {
        return err
    }

    // Add response to history
    agent.History = append(agent.History, Message{From: "agent", Content: resp.Content})

    // Send back to PWA
    pwaMsg := map[string]interface{}{
        "type":       "agent_response",
        "session_id": sessionID,
        "role":       role,
        "content":    resp.Content,
    }

    return session.PWAConn.WriteJSON(pwaMsg)
}

func (r *Relay) GetHistory(sessionID, role string) ([]Message, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    session := r.sessions[sessionID]
    if session == nil {
        return nil, fmt.Errorf("session not found")
    }

    agent := session.Agents[role]
    if agent == nil {
        return nil, fmt.Errorf("agent not found")
    }

    return agent.History, nil
}
```

### 5. PWA (Simple React/Vanilla JS)

**File:** `web/index.html`

```html
<!DOCTYPE html>
<html>
<head>
    <title>Ourocodus</title>
    <style>
        body {
            font-family: -apple-system, sans-serif;
            margin: 0;
            padding: 20px;
            background: #1e1e1e;
            color: #d4d4d4;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { color: #4ec9b0; }
        .agents {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 20px;
            margin: 20px 0;
        }
        .agent-card {
            background: #252526;
            border: 2px solid #3e3e42;
            border-radius: 8px;
            padding: 20px;
            cursor: pointer;
            transition: border-color 0.2s;
        }
        .agent-card:hover { border-color: #4ec9b0; }
        .agent-card.active { border-color: #4ec9b0; }
        .agent-title { font-size: 18px; font-weight: 600; margin-bottom: 8px; }
        .agent-status { font-size: 14px; color: #858585; }
        .chat-container {
            background: #252526;
            border-radius: 8px;
            padding: 20px;
            margin-top: 20px;
            display: none;
        }
        .chat-container.active { display: block; }
        .messages {
            height: 400px;
            overflow-y: auto;
            margin-bottom: 20px;
            padding: 10px;
            background: #1e1e1e;
            border-radius: 4px;
        }
        .message {
            margin: 10px 0;
            padding: 10px;
            border-radius: 4px;
        }
        .message.user {
            background: #264f78;
            margin-left: 20px;
        }
        .message.agent {
            background: #2d2d30;
            margin-right: 20px;
        }
        .input-area {
            display: flex;
            gap: 10px;
        }
        input {
            flex: 1;
            padding: 12px;
            background: #3c3c3c;
            border: 1px solid #3e3e42;
            border-radius: 4px;
            color: #d4d4d4;
            font-size: 14px;
        }
        button {
            padding: 12px 24px;
            background: #0e639c;
            border: none;
            border-radius: 4px;
            color: white;
            cursor: pointer;
            font-size: 14px;
        }
        button:hover { background: #1177bb; }
        #newProjectBtn {
            background: #4ec9b0;
            color: #000;
        }
        #newProjectBtn:hover { background: #5fd3bd; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Ourocodus</h1>
        <button id="newProjectBtn">New Project</button>

        <div class="agents" id="agentsContainer"></div>

        <div class="chat-container" id="chatContainer">
            <h2 id="chatTitle"></h2>
            <div class="messages" id="messages"></div>
            <div class="input-area">
                <input type="text" id="messageInput" placeholder="Type a message...">
                <button id="sendBtn">Send</button>
            </div>
        </div>
    </div>

    <script>
        let ws = null;
        let sessionId = null;
        let currentAgent = null;

        document.getElementById('newProjectBtn').addEventListener('click', createProject);
        document.getElementById('sendBtn').addEventListener('click', sendMessage);
        document.getElementById('messageInput').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') sendMessage();
        });

        function createProject() {
            sessionId = 'sess_' + Date.now();
            ws = new WebSocket('ws://localhost:8080/ws');

            ws.onopen = () => {
                ws.send(JSON.stringify({
                    type: 'create_session',
                    session_id: sessionId
                }));
            };

            ws.onmessage = (event) => {
                const data = JSON.parse(event.data);
                handleMessage(data);
            };

            // Show agent cards
            const roles = ['auth', 'db', 'tests'];
            const container = document.getElementById('agentsContainer');
            container.innerHTML = '';

            roles.forEach(role => {
                const card = document.createElement('div');
                card.className = 'agent-card';
                card.innerHTML = `
                    <div class="agent-title">${role.charAt(0).toUpperCase() + role.slice(1)} Agent</div>
                    <div class="agent-status">Connected</div>
                `;
                card.onclick = () => selectAgent(role);
                card.dataset.role = role;
                container.appendChild(card);
            });
        }

        function selectAgent(role) {
            currentAgent = role;
            document.querySelectorAll('.agent-card').forEach(card => {
                card.classList.toggle('active', card.dataset.role === role);
            });

            document.getElementById('chatTitle').textContent = `${role.charAt(0).toUpperCase() + role.slice(1)} Agent`;
            document.getElementById('chatContainer').classList.add('active');
            document.getElementById('messages').innerHTML = '';
        }

        function sendMessage() {
            const input = document.getElementById('messageInput');
            const content = input.value.trim();
            if (!content || !currentAgent) return;

            addMessage('user', content);
            input.value = '';

            ws.send(JSON.stringify({
                type: 'user_message',
                session_id: sessionId,
                role: currentAgent,
                content: content
            }));
        }

        function handleMessage(data) {
            if (data.type === 'agent_response' && data.role === currentAgent) {
                addMessage('agent', data.content);
            }
        }

        function addMessage(from, content) {
            const messagesDiv = document.getElementById('messages');
            const msg = document.createElement('div');
            msg.className = `message ${from}`;
            msg.textContent = content;
            messagesDiv.appendChild(msg);
            messagesDiv.scrollTop = messagesDiv.scrollHeight;
        }
    </script>
</body>
</html>
```

## Testing Strategy

### Manual Test

```bash
# 1. Setup worktrees
./scripts/setup-worktrees.sh

# 2. Install Claude Code ACP
npm install -g @zed-industries/claude-code-acp

# 3. Start relay
export ANTHROPIC_API_KEY=sk-...
go run cmd/relay/main.go

# 4. Open PWA
open http://localhost:3000

# 5. Test flow
# - Click "New Project"
# - Should see 3 agent cards
# - Click "Auth Agent"
# - Type: "Create a simple Go HTTP server with JWT auth"
# - Should receive response from Claude Code
# - Check agent/auth directory for created files
```

### Success Criteria

Phase 1 is complete when:

- [ ] PWA loads and connects to relay
- [ ] Can create new project (spawns 3 ACP processes)
- [ ] PWA shows 3 agent cards
- [ ] Can click agent and open chat
- [ ] Can send message to agent, receive response
- [ ] Claude Code creates actual files in worktree
- [ ] Files are committed to agent branch
- [ ] Can switch between agents, conversations isolated
- [ ] All 3 agents work concurrently

## What We Learn

After Phase 1:
- ACP protocol quirks and edge cases
- Process management for multiple Claude Code instances
- Conversation state management patterns
- Git worktree workflow validation
- Multi-agent concurrency challenges

## What We Defer

- Autonomous workflow generation (just hardcode 3 roles)
- Git merging logic (manual for now)
- Approval gates (Phase 3)
- Error recovery (Phase 4)
- Idea → PRD generation (Phase 3)

## WebSocket Protocol Specification

### Message Format

All WebSocket messages include a `version` field for protocol versioning:

```json
{
  "version": "1.0",
  "type": "session:create",
  "agentId": "auth"
}
```

**Version:** Protocol version string (format: `"major.minor"`)
- Phase 1: `"1.0"`
- Breaking changes increment major version
- Backward-compatible changes increment minor version

### Connection Handshake

**1. Client connects to WebSocket endpoint:**
```
ws://localhost:8080/ws
```

**2. Server acknowledges connection:**
```json
{
  "version": "1.0",
  "type": "connection:established",
  "serverId": "relay-uuid",
  "timestamp": "2025-10-22T12:34:56Z"
}
```

**3. Client must send first message within 10 seconds or connection closes**

### Message Types

**From PWA to Relay:**

**Create Session:**
```json
{
  "version": "1.0",
  "type": "session:create",
  "agentId": "auth"
}
```

**Send Message to Agent:**
```json
{
  "version": "1.0",
  "type": "agent:message",
  "sessionId": "uuid",
  "content": "Create a JWT auth module"
}
```

**Stop Session:**
```json
{
  "version": "1.0",
  "type": "session:stop",
  "sessionId": "uuid"
}
```

**From Relay to PWA:**

**Session Created:**
```json
{
  "version": "1.0",
  "type": "session:created",
  "sessionId": "uuid",
  "agentId": "auth",
  "timestamp": "2025-10-22T12:34:56Z"
}
```

**Session Ready (ACP process spawned):**
```json
{
  "version": "1.0",
  "type": "session:ready",
  "sessionId": "uuid"
}
```

**Agent Response:**
```json
{
  "version": "1.0",
  "type": "agent:response",
  "sessionId": "uuid",
  "agentId": "auth",
  "content": "I've created auth.go with JWT implementation...",
  "timestamp": "2025-10-22T12:34:57Z"
}
```

**Error:**
```json
{
  "version": "1.0",
  "type": "error",
  "sessionId": "uuid",
  "agentId": "auth",
  "error": {
    "code": "ACP_PROCESS_CRASHED",
    "message": "Agent process exited unexpectedly",
    "details": "exit status 1",
    "recoverable": false
  }
}
```

**Session Terminated:**
```json
{
  "version": "1.0",
  "type": "session:terminated",
  "sessionId": "uuid",
  "reason": "user requested"
}
```

### Heartbeat

**Phase 1:** No heartbeat/ping mechanism (kept simple for POC)

**Client disconnection detection:** Relay detects when `ws.ReadMessage()` returns error

**Server health:** Client assumes relay is healthy if WebSocket connection is open

**Future (Phase 2):**
```json
// Server → Client (every 30s)
{"version": "1.0", "type": "ping"}

// Client → Server
{"version": "1.0", "type": "pong"}
```

### Protocol Versioning Rules

**Version Compatibility:**
- Client and server MUST include `version` in all messages
- Server MUST reject messages with unsupported `version`
- Version mismatch returns error and closes connection

**Version Check Response:**
```json
{
  "version": "1.0",
  "type": "error",
  "error": {
    "code": "VERSION_MISMATCH",
    "message": "Protocol version 2.0 not supported (server supports 1.0)",
    "recoverable": false
  }
}
```

**Future Version Migration:**
- Phase 1: `"1.0"` (stdio + direct WebSocket)
- Phase 2: `"1.1"` (add heartbeat, may add NATS)
- Phase 3: `"2.0"` (breaking change: add coordinator messages)

See [docs/ERROR_HANDLING.md](ERROR_HANDLING.md) for error codes and handling strategy.
See [docs/SESSION_LIFECYCLE.md](SESSION_LIFECYCLE.md) for detailed session flow.

## Known Limitations (POC Trade-offs)

Phase 1 makes pragmatic compromises to validate the concept quickly:

**1. In-Memory State**
- Session/conversation state stored in relay memory
- Process crash = lost conversations
- No persistence across restarts
- **Future:** SQLite event store (Phase 4)

**2. Process Management**
- Relay directly spawns ACP processes
- No process supervision/restart
- No resource limits (CPU/memory)
- **Future:** Container orchestration (Phase 2-3)

**3. No NATS**
- Direct WebSocket (PWA→Relay) and stdio (Relay→ACP)
- Simpler but not scalable
- **Future:** NATS message bus (Phase 2)

**4. Manual Workflow**
- User manually directs each agent
- No coordinator, no automation
- **Future:** Coordinator with graph engine (Phase 3)

**5. Git Operations**
- Worktrees created by shell script
- No merge automation
- No conflict resolution
- **Future:** Automated git workflow (Phase 3-4)

**6. Error Handling**
- Basic error logging only
- No retry logic
- No graceful degradation
- **Future:** Comprehensive error recovery (Phase 4)

**7. Security**
- No authentication
- No rate limiting
- Localhost only
- **Future:** Production security (Phase 4)

**These limitations are acceptable** because Phase 1's goal is proving multi-agent ACP communication works, not building production infrastructure.

## Time Estimate

**Week 1:**
- Day 1-2: ACP client wrapper + relay integration
- Day 3: Process spawning and management
- Day 4-5: PWA with WebSocket + chat UI

**Week 2:**
- Day 1-2: Integration testing + bug fixes
- Day 3-4: Polish and edge cases
- Day 5: Documentation and demo

**Total: 2 weeks**
