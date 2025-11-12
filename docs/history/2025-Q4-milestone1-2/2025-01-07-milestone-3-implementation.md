# Milestone 3: PWA Polish & Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete core PWA features and infrastructure polish for multi-agent platform

**Architecture:** Dependency-first implementation covering embedded assets (go:embed), configurable logging, server protocol handlers, agent cards UI, chat interface, and multi-agent support

**Tech Stack:** Go 1.23, standard library, vanilla JavaScript, WebSocket protocol

---

## Task 1: Embed PWA Assets with go:embed (#46)

**Files:**
- Modify: `cmd/relay/main.go:50-51`
- Test: Manual verification (build and run from different directories)

**Step 1: Add embed directive and import**

In `cmd/relay/main.go`, add at the top after imports:

```go
import (
    "embed"
    // ... existing imports
)

//go:embed web/*
var embeddedFS embed.FS
```

**Step 2: Replace file server with embedded FS**

In `cmd/relay/main.go`, find the line:
```go
http.Handle("/", http.FileServer(http.Dir("./web")))
```

Replace with:
```go
webFS, err := fs.Sub(embeddedFS, "web")
if err != nil {
    log.Fatalf("Failed to create web filesystem: %v", err)
}
http.Handle("/", http.FileServer(http.FS(webFS)))
```

Add the fs import:
```go
import (
    "io/fs"
    // ... other imports
)
```

**Step 3: Build binary**

Run: `make build`
Expected: Binary builds successfully with embedded assets

**Step 4: Test from different directory**

```bash
cd /tmp
/path/to/ourocodus/bin/relay
curl http://localhost:8080/
```

Expected: PWA HTML loads successfully from any directory

**Step 5: Commit**

```bash
git add cmd/relay/main.go
git commit -m "feat: embed PWA assets with go:embed

Replace file serving from ./web directory with embedded filesystem.
Binary now self-contained and works from any directory.

Fixes #46"
```

---

## Task 2: Create Logger System (#65)

**Files:**
- Create: `web/logger.js`
- Modify: `web/index.html` (add script tag)
- Modify: `web/relay.js` (replace console calls)
- Modify: `web/protocol-inspector.js` (replace console calls)
- Test: Manual verification (check localStorage and console)

**Step 1: Create Logger class**

Create `web/logger.js`:

```javascript
class Logger {
    static levels = { debug: 0, info: 1, warn: 2, error: 3, none: 4 };

    constructor(component, options = {}) {
        this.component = component;
        this.level = options.level || Logger.getDefaultLevel();
    }

    static getDefaultLevel() {
        const stored = localStorage.getItem('ourocodus.logLevel');
        if (stored && stored in Logger.levels) {
            return stored;
        }
        return 'info';
    }

    static setLevel(level) {
        if (level in Logger.levels) {
            localStorage.setItem('ourocodus.logLevel', level);
        }
    }

    _shouldLog(level) {
        return Logger.levels[level] >= Logger.levels[this.level];
    }

    _log(level, color, msg, ...args) {
        if (this._shouldLog(level)) {
            const timestamp = new Date().toISOString().split('T')[1].slice(0, -1);
            console.log(`%c[${timestamp}] [${this.component}] ${msg}`, `color: ${color}`, ...args);
        }
    }

    debug(msg, ...args) {
        this._log('debug', '#999', msg, ...args);
    }

    info(msg, ...args) {
        this._log('info', '#0066cc', msg, ...args);
    }

    warn(msg, ...args) {
        this._log('warn', '#ff9900', msg, ...args);
    }

    error(msg, ...args) {
        this._log('error', '#cc0000', msg, ...args);
    }
}

// Make Logger available globally
window.Logger = Logger;
```

**Step 2: Add logger script to HTML**

In `web/index.html`, add before other script tags:

```html
<script src="logger.js"></script>
```

**Step 3: Replace console calls in relay.js**

In `web/relay.js`, at the top of RelayConnection class:

```javascript
class RelayConnection {
    constructor() {
        this.logger = new Logger('RelayConnection');
        // ... rest of constructor
    }
```

Replace console calls:
- `console.log(` → `this.logger.debug(`
- `console.error(` → `this.logger.error(`
- Important events (connection established, etc.) → `this.logger.info(`

**Step 4: Replace console calls in protocol-inspector.js**

In `web/protocol-inspector.js`, add logger:

```javascript
class ProtocolInspector {
    constructor() {
        this.logger = new Logger('ProtocolInspector');
        // ... rest of constructor
    }
```

Replace console calls similarly.

**Step 5: Test logger levels**

Open browser console:
```javascript
Logger.setLevel('debug');  // See all logs
Logger.setLevel('info');   // Default, see info/warn/error
Logger.setLevel('error');  // Only errors
```

Expected: Log output changes based on level

**Step 6: Commit**

```bash
git add web/logger.js web/index.html web/relay.js web/protocol-inspector.js
git commit -m "feat: add configurable logging system

Create Logger class with debug/info/warn/error levels.
Store level in localStorage (ourocodus.logLevel).
Replace console.log calls throughout PWA.

Default level: info (cleaner production console)
Set to debug for verbose logging during development.

Fixes #65"
```

---

## Task 3: Add Server Protocol Handlers (#64)

**Files:**
- Modify: `internal/relay/websocket.go` (add message handlers)
- Test: Manual verification (send session:end and agent:terminate)

**Step 1: Find message handler switch**

In `internal/relay/websocket.go`, find the message type switch in `handleClientMessage` or similar.

**Step 2: Add session:end handler**

Add case in switch:

```go
case "session:end":
    h.handleSessionEnd(client, msg)
```

Add handler method:

```go
func (h *WebSocketHandler) handleSessionEnd(client *Client, msg Message) {
    sessionID := msg["sessionId"].(string)

    // Get session
    session, exists := h.sessions[sessionID]
    if !exists {
        h.sendError(client, "SESSION_NOT_FOUND", "Session not found", false)
        return
    }

    // Terminate all agents
    agentCount := 0
    for role := range session.agents {
        if err := h.terminateAgent(session, role); err != nil {
            log.Printf("Error terminating agent %s: %v", role, err)
        } else {
            agentCount++
        }
    }

    // Send confirmation
    response := Message{
        "type":              "session:ended",
        "version":           "1.0",
        "sessionId":         sessionID,
        "agentsTerminated":  agentCount,
        "cleanupStatus":     "complete",
    }

    if err := client.conn.WriteJSON(response); err != nil {
        log.Printf("Error sending session:ended: %v", err)
    }
}
```

**Step 3: Add agent:terminate handler**

Add case in switch:

```go
case "agent:terminate":
    h.handleAgentTerminate(client, msg)
```

Add handler method:

```go
func (h *WebSocketHandler) handleAgentTerminate(client *Client, msg Message) {
    sessionID := msg["sessionId"].(string)
    role := msg["role"].(string)

    // Get session
    session, exists := h.sessions[sessionID]
    if !exists {
        h.sendError(client, "SESSION_NOT_FOUND", "Session not found", false)
        return
    }

    // Check agent exists
    _, exists = session.agents[role]
    if !exists {
        h.sendError(client, "AGENT_NOT_FOUND", fmt.Sprintf("Agent with role %s not found", role), true)
        return
    }

    // Terminate agent
    workspaceCleaned := false
    if err := h.terminateAgent(session, role); err != nil {
        log.Printf("Error terminating agent %s: %v", role, err)
    } else {
        workspaceCleaned = true
    }

    // Send confirmation
    response := Message{
        "type":             "agent:terminated",
        "version":          "1.0",
        "sessionId":        sessionID,
        "role":             role,
        "workspaceCleaned": workspaceCleaned,
    }

    if err := client.conn.WriteJSON(response); err != nil {
        log.Printf("Error sending agent:terminated: %v", err)
    }
}
```

**Step 4: Add terminateAgent helper**

```go
func (h *WebSocketHandler) terminateAgent(session *Session, role string) error {
    agent, exists := session.agents[role]
    if !exists {
        return fmt.Errorf("agent not found")
    }

    // Send SIGTERM for graceful shutdown
    if err := agent.process.Signal(syscall.SIGTERM); err != nil {
        log.Printf("SIGTERM failed: %v, trying SIGKILL", err)
        if err := agent.process.Kill(); err != nil {
            return fmt.Errorf("failed to kill agent: %w", err)
        }
    }

    // Wait for termination with timeout
    done := make(chan error, 1)
    go func() {
        _, err := agent.process.Wait()
        done <- err
    }()

    select {
    case <-done:
        // Process exited
    case <-time.After(5 * time.Second):
        // Timeout, force kill
        agent.process.Kill()
    }

    // Clean up workspace
    if agent.workspace != "" {
        if err := os.RemoveAll(agent.workspace); err != nil {
            log.Printf("Failed to clean workspace %s: %v", agent.workspace, err)
        }
    }

    // Remove from session
    delete(session.agents, role)

    return nil
}
```

**Step 5: Test session:end**

From browser console:
```javascript
relay.ws.send(JSON.stringify({
    type: "session:end",
    version: "1.0",
    sessionId: relay.sessionId
}));
```

Expected: Receive `session:ended` response, no errors

**Step 6: Test agent:terminate**

Spawn an agent, then:
```javascript
relay.ws.send(JSON.stringify({
    type: "agent:terminate",
    version: "1.0",
    sessionId: relay.sessionId,
    role: "echo"
}));
```

Expected: Receive `agent:terminated` response, agent process killed

**Step 7: Commit**

```bash
git add internal/relay/websocket.go
git commit -m "feat: add server-side session:end and agent:terminate handlers

Implement graceful agent termination:
- SIGTERM with 5s timeout, then SIGKILL
- Workspace cleanup
- Confirmation messages

Fixes #64"
```

---

## Task 4: Create Agent Cards UI (#11)

**Files:**
- Modify: `web/index.html` (update structure)
- Modify: `web/styles.css` (add agent card styles)
- Modify: `web/relay.js` (refactor to Map-based state)
- Test: Manual verification (spawn multiple agents, see cards)

**Step 1: Refactor state to Map in relay.js**

In `web/relay.js`, find state variables and replace:

```javascript
// Before:
let currentAgentRole = null;
let messages = [];

// After:
const agents = new Map(); // role → { role, status, messages, workspace }
```

Update spawnAgent to add to Map:

```javascript
spawnAgent(role, workspace) {
    // ... existing spawn logic ...

    // After agent:ready received:
    agents.set(role, {
        role: role,
        status: 'ready',
        messages: [],
        workspace: workspace
    });

    this.renderAgentCards();
}
```

**Step 2: Add agent cards container to HTML**

In `web/index.html`, after agent spawn section:

```html
<div id="agentCards" class="agent-cards-container">
    <!-- Agent cards will be rendered here -->
</div>
```

**Step 3: Add agent card styles**

In `web/styles.css`:

```css
.agent-cards-container {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    margin-top: 1rem;
}

.agent-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
    transition: border-color 0.2s;
}

.agent-card:hover {
    border-color: var(--accent-primary);
}

.agent-card-info {
    display: flex;
    align-items: center;
    gap: 1rem;
}

.agent-card-role {
    font-weight: 600;
    color: var(--text-primary);
}

.agent-card-status {
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.875rem;
}

.agent-card-status.ready {
    background: #10b981;
    color: white;
}

.agent-card-status.busy {
    background: #f59e0b;
    color: white;
}

.agent-card-status.error {
    background: #ef4444;
    color: white;
}

.agent-card-actions button {
    padding: 0.5rem 1rem;
    background: var(--error);
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
}

.agent-card-actions button:hover {
    opacity: 0.8;
}
```

**Step 4: Add renderAgentCards method**

In `web/relay.js`:

```javascript
renderAgentCards() {
    const container = document.getElementById('agentCards');
    if (!container) return;

    container.innerHTML = '';

    agents.forEach((agent, role) => {
        const card = document.createElement('div');
        card.className = 'agent-card';
        card.innerHTML = `
            <div class="agent-card-info">
                <span class="agent-card-role">Agent: ${role}</span>
                <span class="agent-card-status ${agent.status}">${agent.status}</span>
                <span>Messages: ${agent.messages.length}</span>
            </div>
            <div class="agent-card-actions">
                <button onclick="relay.terminateAgent('${role}')">×</button>
            </div>
        `;

        card.addEventListener('click', (e) => {
            if (!e.target.closest('button')) {
                this.showAgentChat(role);
            }
        });

        container.appendChild(card);
    });
}
```

**Step 5: Add terminateAgent method**

```javascript
terminateAgent(role) {
    this.logger.info(`Terminating agent: ${role}`);

    this.ws.send(JSON.stringify({
        type: 'agent:terminate',
        version: '1.0',
        sessionId: this.sessionId,
        role: role
    }));
}
```

Handle agent:terminated response:

```javascript
// In message handler
if (msg.type === 'agent:terminated') {
    agents.delete(msg.role);
    this.renderAgentCards();
}
```

**Step 6: Test with multiple agents**

Spawn 2-3 agents with different roles.

Expected: See multiple agent cards, each showing role, status, message count, and terminate button

**Step 7: Commit**

```bash
git add web/index.html web/styles.css web/relay.js
git commit -m "feat: add agent cards UI

Refactor state to Map-based structure supporting multiple agents.
Add agent card component showing role, status, message count.
Keep spawn section always visible.

Fixes #11"
```

---

## Task 5: Add Chat Interface (#12)

**Files:**
- Modify: `web/index.html` (add chat container)
- Modify: `web/styles.css` (add chat styles)
- Modify: `web/relay.js` (add chat logic)
- Test: Manual verification (send messages, see history)

**Step 1: Add chat container to HTML**

In `web/index.html`, after agent cards:

```html
<div id="chatContainer" class="chat-container" style="display: none;">
    <div class="chat-header">
        <span id="chatAgentName"></span>
        <button onclick="relay.closeChat()">×</button>
    </div>
    <div id="chatMessages" class="chat-messages"></div>
    <div class="chat-input">
        <input type="text" id="chatInput" placeholder="Type a message...">
        <button onclick="relay.sendMessage()">Send</button>
    </div>
</div>
```

**Step 2: Add chat styles**

In `web/styles.css`:

```css
.chat-container {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    margin-top: 1rem;
    display: flex;
    flex-direction: column;
    height: 400px;
}

.chat-header {
    padding: 1rem;
    border-bottom: 1px solid var(--border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-weight: 600;
}

.chat-header button {
    background: none;
    border: none;
    font-size: 1.5rem;
    cursor: pointer;
    color: var(--text-secondary);
}

.chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.chat-message {
    padding: 0.75rem;
    border-radius: 8px;
    max-width: 70%;
}

.chat-message.user {
    background: var(--accent-primary);
    color: white;
    align-self: flex-end;
    margin-left: auto;
}

.chat-message.agent {
    background: var(--bg-tertiary);
    color: var(--text-primary);
    align-self: flex-start;
}

.chat-input {
    display: flex;
    gap: 0.5rem;
    padding: 1rem;
    border-top: 1px solid var(--border);
}

.chat-input input {
    flex: 1;
    padding: 0.5rem;
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
}

.chat-input button {
    padding: 0.5rem 1rem;
    background: var(--accent-primary);
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
}

.chat-input button:hover {
    opacity: 0.8;
}
```

**Step 3: Add chat methods to relay.js**

```javascript
showAgentChat(role) {
    this.currentChatRole = role;
    const agent = agents.get(role);
    if (!agent) return;

    document.getElementById('chatAgentName').textContent = `Agent: ${role}`;
    document.getElementById('chatContainer').style.display = 'flex';

    this.renderChatMessages(role);
    document.getElementById('chatInput').focus();
}

closeChat() {
    document.getElementById('chatContainer').style.display = 'none';
    this.currentChatRole = null;
}

renderChatMessages(role) {
    const agent = agents.get(role);
    if (!agent) return;

    const container = document.getElementById('chatMessages');
    container.innerHTML = '';

    agent.messages.forEach(msg => {
        const div = document.createElement('div');
        div.className = `chat-message ${msg.sender}`;
        div.textContent = msg.content;
        container.appendChild(div);
    });

    // Auto-scroll to bottom
    container.scrollTop = container.scrollHeight;
}

sendMessage() {
    const input = document.getElementById('chatInput');
    const content = input.value.trim();
    if (!content || !this.currentChatRole) return;

    // Add to messages
    const agent = agents.get(this.currentChatRole);
    if (agent) {
        agent.messages.push({
            sender: 'user',
            content: content,
            timestamp: new Date()
        });
    }

    // Send to server
    this.ws.send(JSON.stringify({
        type: 'agent:message',
        version: '1.0',
        sessionId: this.sessionId,
        role: this.currentChatRole,
        content: content
    }));

    // Clear input and re-render
    input.value = '';
    this.renderChatMessages(this.currentChatRole);
    this.renderAgentCards(); // Update message count
}
```

**Step 4: Handle agent responses**

In message handler:

```javascript
if (msg.type === 'agent:response') {
    const agent = agents.get(msg.role);
    if (agent) {
        agent.messages.push({
            sender: 'agent',
            content: msg.content,
            timestamp: new Date()
        });

        if (this.currentChatRole === msg.role) {
            this.renderChatMessages(msg.role);
        }
        this.renderAgentCards(); // Update message count
    }
}
```

**Step 5: Add Enter key support**

```javascript
document.getElementById('chatInput').addEventListener('keypress', (e) => {
    if (e.key === 'Enter') {
        relay.sendMessage();
    }
});
```

**Step 6: Test chat**

Spawn agent, click card, send message.

Expected: See user message appear (blue, right-aligned), receive agent response (gray, left-aligned), auto-scroll

**Step 7: Commit**

```bash
git add web/index.html web/styles.css web/relay.js
git commit -m "feat: add chat interface

Add chat UI that expands from agent card.
Support sending messages to agents and displaying responses.
Auto-scroll to latest message.
Enter key sends message.

Fixes #12"
```

---

## Task 6: Enable Multi-Agent Support (#63)

**Files:**
- Modify: `web/relay.js` (remove single-agent restrictions)
- Test: Manual verification (run 3+ agents simultaneously)

**Step 1: Remove single-agent UI logic**

In `web/relay.js`, find and remove any code that:
- Hides spawn section after agent spawns
- Prevents spawning multiple agents
- Assumes only one agent exists

Ensure spawnAgent always:
- Adds to agents Map (don't replace)
- Calls renderAgentCards() (shows all cards)
- Keeps spawn section visible

**Step 2: Add bulk terminate button**

In `web/index.html`, add after agent cards container:

```html
<button id="terminateAll" style="display: none;" onclick="relay.terminateAll()">
    Terminate All Agents
</button>
```

In `web/relay.js`:

```javascript
renderAgentCards() {
    // ... existing code ...

    // Show/hide terminate all button
    const terminateAllBtn = document.getElementById('terminateAll');
    if (terminateAllBtn) {
        terminateAllBtn.style.display = agents.size > 1 ? 'block' : 'none';
    }
}

terminateAll() {
    if (!confirm(`Terminate all ${agents.size} agents?`)) return;

    this.ws.send(JSON.stringify({
        type: 'session:end',
        version: '1.0',
        sessionId: this.sessionId
    }));
}
```

Handle session:ended:

```javascript
if (msg.type === 'session:ended') {
    agents.clear();
    this.renderAgentCards();
    this.closeChat();
}
```

**Step 3: Test with multiple agents**

Spawn 3 agents with different roles (echo, coder, helper).

Expected:
- All 3 agents show separate cards
- Each agent has independent message history
- Can chat with each agent separately
- Can terminate individual agents or all at once
- Spawn section always visible

**Step 4: Test independent operation**

- Send message to agent A
- Switch to agent B, send different message
- Switch back to agent A
- Expected: Each agent's chat history is preserved independently

**Step 5: Commit**

```bash
git add web/index.html web/relay.js
git commit -m "feat: enable multi-agent support

Remove single-agent restrictions from UI.
Add bulk terminate button for multiple agents.
Each agent operates independently with isolated state.

Fixes #63"
```

---

## Task 7: Integration Testing

**Files:**
- None (testing only)

**Step 1: Build and run server**

```bash
make build
./bin/relay
```

Expected: Server starts, PWA loads from embedded assets

**Step 2: Test full workflow**

1. Open http://localhost:8080/
2. Create session
3. Spawn 3 agents (roles: echo, coder, helper)
4. Verify: 3 agent cards appear
5. Click echo agent card
6. Send message "hello echo"
7. Verify: Message appears in chat
8. Click coder agent card
9. Send message "hello coder"
10. Verify: Independent chat history
11. Click terminate on echo agent
12. Verify: Echo card disappears, coder/helper remain
13. Click "Terminate All"
14. Verify: All agents terminate, cards disappear

**Step 3: Test logging levels**

Browser console:
```javascript
Logger.setLevel('debug');
// Refresh page
// Expected: See debug logs

Logger.setLevel('error');
// Refresh page
// Expected: Only errors visible
```

**Step 4: Verify go:embed**

```bash
cd /tmp
/path/to/ourocodus/bin/relay
curl http://localhost:8080/
```

Expected: PWA loads from embedded assets, no file not found errors

**Step 5: Final commit**

```bash
git add .
git commit -m "chore: milestone 3 integration testing complete

All 6 issues verified:
- #46: go:embed working from any directory
- #65: logging system with configurable levels
- #64: session:end and agent:terminate handlers
- #11: agent cards UI
- #12: chat interface
- #63: multi-agent support

PWA now supports full multi-agent interaction."
```

---

## Verification Checklist

- [ ] Binary runs from any directory (go:embed)
- [ ] Log level controls console output
- [ ] session:end terminates all agents
- [ ] agent:terminate terminates specific agent
- [ ] Multiple agent cards display
- [ ] Can spawn 3+ agents simultaneously
- [ ] Each agent has independent chat
- [ ] Can switch between agent chats
- [ ] Message history preserved per agent
- [ ] Can terminate individual agents
- [ ] Can terminate all agents at once
- [ ] Spawn section always visible
- [ ] No errors in console (at info level)
- [ ] WebSocket protocol working
- [ ] All commits follow conventional format

## Success Criteria

✅ All 6 issues (#46, #65, #64, #11, #12, #63) complete
✅ Multi-agent platform functional
✅ Infrastructure polish applied
✅ Tests passing
✅ Documentation updated
✅ Clean commit history

## Next Steps After Completion

1. Test with real ACP clients
2. Gather user feedback
3. Begin Milestone 4 planning
4. Consider polish issues (#60-#62, #66) based on priority
