# Agent Client Protocol (ACP) Guide

## Overview

The Agent Client Protocol (ACP) is a standardized communication framework that enables code editors and IDEs to interact with AI-powered coding agents. It uses JSON-RPC 2.0 over stdio, similar to the Language Server Protocol (LSP).

### Key Concepts

- **Agents**: Autonomous AI programs that can analyze and modify code
- **Clients**: User-facing interfaces like IDEs or text editors that invoke agents
- **Sessions**: Independent conversation threads with isolated context and history
- **Bidirectional Communication**: Both agents and clients can initiate actions
- **Markdown-First**: Default format for user-readable text with rich formatting support

## Architecture

```
┌─────────────────┐         JSON-RPC 2.0          ┌─────────────────┐
│                 │◄──────────over stdio──────────►│                 │
│   IDE/Editor    │                                 │  Claude Code    │
│    (Client)     │   Requests & Notifications      │    (Agent)      │
│                 │                                 │                 │
└─────────────────┘                                 └─────────────────┘
        │                                                    │
        │                                                    │
        ▼                                                    ▼
  User Prompts                                       AI Responses
  Permissions                                        Tool Execution
  File Operations                                    Session State
```

## Message Types

ACP implements two fundamental message patterns based on JSON-RPC 2.0:

### 1. Methods (Request/Response)

Methods are request-response pairs that expect a result or error. They include an `id` field for correlation.

**Request Format:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "method_name",
  "params": { /* parameters */ }
}
```

**Success Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": { /* result data */ }
}
```

**Error Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request"
  }
}
```

### 2. Notifications (One-Way)

Notifications are one-way messages that do not expect a response. They have no `id` field.

**Notification Format:**
```json
{
  "jsonrpc": "2.0",
  "method": "notification_name",
  "params": { /* parameters */ }
}
```

## Complete Message Flow Analysis

Let's analyze the session output step by step:

### Step 1: Initialize (Request/Response)

**Client sends:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": 1,
    "clientInfo": {
      "name": "test",
      "version": "1.0"
    },
    "capabilities": {}
  }
}
```

**What this does:**
- Establishes protocol compatibility between client and agent
- Negotiates capabilities (features both sides support)
- Shares client information for logging/telemetry

**Agent responds:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": 1,
    "agentCapabilities": {
      "promptCapabilities": {
        "image": true,
        "embeddedContext": true
      },
      "mcpCapabilities": {
        "http": true,
        "sse": true
      }
    },
    "agentInfo": {
      "name": "@zed-industries/claude-code-acp",
      "title": "Claude Code",
      "version": "0.10.10"
    },
    "authMethods": [
      {
        "description": "Run `/login` in the terminal",
        "name": "Log in with Claude Code",
        "id": "claude-login"
      }
    ]
  }
}
```

**What the response means:**
- **protocolVersion**: Confirms protocol version 1
- **agentCapabilities**: Agent supports images in prompts, embedded context, HTTP and SSE for MCP servers
- **agentInfo**: Identifies the agent (Claude Code v0.10.10)
- **authMethods**: Available authentication methods (requires `/login` command)

### Step 2: Create Session (Request/Response)

**Client sends:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "session/new",
  "params": {
    "cwd": "/workspace",
    "mcpServers": []
  }
}
```

**What this does:**
- Creates a new conversation session
- Sets working directory to `/workspace`
- Specifies MCP servers to connect (none in this case)

**Agent responds:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "sessionId": "019ae541-51d4-7698-a83e-369928a36485",
    "models": {
      "availableModels": [
        {
          "modelId": "default",
          "name": "Default (recommended)",
          "description": "Use the default model (currently Sonnet 4.5) · $3/$15 per Mtok"
        },
        {
          "modelId": "opus",
          "name": "Opus",
          "description": "Opus 4.5 · Most capable for complex work · $5/$25 per Mtok"
        },
        {
          "modelId": "haiku",
          "name": "Haiku",
          "description": "Haiku 4.5 · Fastest for quick answers · $1/$5 per Mtok"
        }
      ],
      "currentModelId": "default"
    },
    "modes": {
      "currentModeId": "default",
      "availableModes": [
        {
          "id": "default",
          "name": "Always Ask",
          "description": "Prompts for permission on first use of each tool"
        },
        {
          "id": "acceptEdits",
          "name": "Accept Edits",
          "description": "Automatically accepts file edit permissions for the session"
        },
        {
          "id": "plan",
          "name": "Plan Mode",
          "description": "Claude can analyze but not modify files or execute commands"
        },
        {
          "id": "bypassPermissions",
          "name": "Bypass Permissions",
          "description": "Skips all permission prompts"
        }
      ]
    }
  }
}
```

**What the response means:**
- **sessionId**: Unique identifier for this conversation (`019ae541-51d4-7698-a83e-369928a36485`)
- **models**: Available AI models and their pricing/capabilities
- **modes**: Permission modes that control how the agent requests user approval:
  - `default`: Ask for permission on first use of each tool
  - `acceptEdits`: Auto-approve file edits
  - `plan`: Read-only mode (no modifications)
  - `bypassPermissions`: Skip all permission prompts

### Step 3: Available Commands Update (Notification)

**Agent sends:**
```json
{
  "jsonrpc": "2.0",
  "method": "session/update",
  "params": {
    "sessionId": "019ae541-51d4-7698-a83e-369928a36485",
    "update": {
      "sessionUpdate": "available_commands_update",
      "availableCommands": [
        {
          "name": "compact",
          "description": "Clear conversation history but keep a summary in context. Optional: /compact [instructions for summarization]",
          "input": {
            "hint": "<optional custom summarization instructions>"
          }
        },
        {
          "name": "init",
          "description": "Initialize a new CLAUDE.md file with codebase documentation",
          "input": null
        },
        {
          "name": "pr-comments",
          "description": "Get comments from a GitHub pull request",
          "input": null
        },
        {
          "name": "review",
          "description": "Review a pull request",
          "input": null
        },
        {
          "name": "security-review",
          "description": "Complete a security review of the pending changes on the current branch",
          "input": null
        }
      ]
    }
  }
}
```

**What this means:**
- Agent notifies client about available slash commands
- These are special commands users can invoke (e.g., `/compact`, `/review`)
- Client can display these in autocomplete or help menus
- No response expected (notification)

### Step 4: Send Prompt (Request/Response)

**Client sends:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "session/prompt",
  "params": {
    "sessionId": "019ae541-51d4-7698-a83e-369928a36485",
    "prompt": [
      {
        "type": "text",
        "text": "Say hello in one sentence"
      }
    ]
  }
}
```

**What this does:**
- Sends a user prompt to the agent
- Prompt is an array of content blocks (can include text, images, etc.)
- Session ID links this prompt to the conversation

### Step 5: Streaming Response (Multiple Notifications)

The agent sends multiple `session/update` notifications to stream the response:

**5a. Plan Update:**
```json
{
  "jsonrpc": "2.0",
  "method": "session/update",
  "params": {
    "sessionId": "019ae541-51d4-7698-a83e-369928a36485",
    "update": {
      "sessionUpdate": "plan_update",
      "planUpdate": {
        "entries": []
      }
    }
  }
}
```

**What this means:**
- Agent reports its intended plan (empty in this case)
- For complex tasks, would show steps like "Read file X", "Modify function Y"

**5b. Start Text Block:**
```json
{
  "jsonrpc": "2.0",
  "method": "session/update",
  "params": {
    "sessionId": "019ae541-51d4-7698-a83e-369928a36485",
    "update": {
      "sessionUpdate": "assistant_update",
      "messageId": "019ae541-6527-7f35-8f76-fdce62d9e92a",
      "contentBlockIndex": 0,
      "update": {
        "blockUpdate": "start",
        "block": {
          "type": "text",
          "text": ""
        }
      }
    }
  }
}
```

**What this means:**
- Agent starts a new content block (text)
- `messageId`: Unique ID for this assistant message
- `contentBlockIndex`: Index of this content block (0-based)
- `blockUpdate: "start"`: Beginning of a new block

**5c. Text Delta (Streaming):**
```json
{
  "jsonrpc": "2.0",
  "method": "session/update",
  "params": {
    "sessionId": "019ae541-51d4-7698-a83e-369928a36485",
    "update": {
      "sessionUpdate": "assistant_update",
      "messageId": "019ae541-6527-7f35-8f76-fdce62d9e92a",
      "contentBlockIndex": 0,
      "update": {
        "blockUpdate": "delta",
        "delta": "Hello"
      }
    }
  }
}
```

**What this means:**
- Agent streams text incrementally
- `blockUpdate: "delta"`: Incremental text addition
- `delta`: Text to append to the current block
- Multiple deltas can be sent (next one adds "!")

**5d. Stop Block:**
```json
{
  "jsonrpc": "2.0",
  "method": "session/update",
  "params": {
    "sessionId": "019ae541-51d4-7698-a83e-369928a36485",
    "update": {
      "sessionUpdate": "assistant_update",
      "messageId": "019ae541-6527-7f35-8f76-fdce62d9e92a",
      "contentBlockIndex": 0,
      "update": {
        "blockUpdate": "stop"
      }
    }
  }
}
```

**What this means:**
- Agent finished streaming this content block
- No more deltas will be sent for block index 0

### Step 6: Prompt Response (Request Response)

**Agent responds:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "stopReason": "end_turn"
  }
}
```

**What this means:**
- Agent completed the prompt turn successfully
- `stopReason: "end_turn"`: Normal completion

**Other possible stop reasons:**
- `max_tokens`: Token limit reached
- `max_turn_requests`: Request limit exceeded
- `refusal`: Agent declined to continue
- `cancelled`: Client cancelled the operation

## Handling Streaming Responses

Clients must handle streaming responses by accumulating deltas:

```typescript
interface ContentBlock {
  type: string;
  text: string;
}

// Map to store content blocks by message ID and block index
const contentBlocks = new Map<string, Map<number, ContentBlock>>();

function handleSessionUpdate(update: SessionUpdate) {
  if (update.sessionUpdate === "assistant_update") {
    const { messageId, contentBlockIndex, update: blockUpdate } = update;

    // Get or create message's blocks
    if (!contentBlocks.has(messageId)) {
      contentBlocks.set(messageId, new Map());
    }
    const blocks = contentBlocks.get(messageId)!;

    switch (blockUpdate.blockUpdate) {
      case "start":
        // Initialize new block
        blocks.set(contentBlockIndex, blockUpdate.block);
        displayBlock(messageId, contentBlockIndex, blockUpdate.block);
        break;

      case "delta":
        // Append delta to existing block
        const block = blocks.get(contentBlockIndex)!;
        block.text += blockUpdate.delta;
        updateBlock(messageId, contentBlockIndex, block);
        break;

      case "stop":
        // Block is complete
        finalizeBlock(messageId, contentBlockIndex);
        break;
    }
  }
}
```

### Streaming Flow Diagram

```
start → delta → delta → ... → delta → stop
  │       │       │             │       │
  ▼       ▼       ▼             ▼       ▼
  ""    "H"   "Hello"     "Hello!"   [done]
```

## Permission Handling

### Permission Request Flow

When an agent needs to perform a sensitive operation (e.g., file edit, command execution), it may request permission:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "session/request_permission",
  "params": {
    "sessionId": "019ae541-51d4-7698-a83e-369928a36485",
    "toolCall": {
      "toolCallId": "toolu_123",
      "title": "Edit file: main.go",
      "description": "Add error handling to main function"
    },
    "options": [
      {
        "optionId": "allow_once",
        "name": "Allow Once",
        "kind": "allow_once"
      },
      {
        "optionId": "allow_always",
        "name": "Always Allow File Edits",
        "kind": "allow_always"
      },
      {
        "optionId": "reject_once",
        "name": "Reject",
        "kind": "reject_once"
      },
      {
        "optionId": "reject_always",
        "name": "Never Allow File Edits",
        "kind": "reject_always"
      }
    ]
  }
}
```

### Client Response

The client responds with the user's decision:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "selectedOptionId": "allow_once"
  }
}
```

Or if cancelled:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "cancelled": true
  }
}
```

### Permission Modes

In the session output, we saw four permission modes:

#### 1. Default Mode (Always Ask)
```json
{
  "id": "default",
  "name": "Always Ask",
  "description": "Prompts for permission on first use of each tool"
}
```

- Agent requests permission on first use of each tool type
- Provides maximum user control
- Recommended for sensitive operations

#### 2. Accept Edits Mode
```json
{
  "id": "acceptEdits",
  "name": "Accept Edits",
  "description": "Automatically accepts file edit permissions for the session"
}
```

- Auto-approves file edit operations
- Still prompts for other operations (command execution, etc.)
- Good for trusted agents doing code refactoring

#### 3. Plan Mode (Read-Only)
```json
{
  "id": "plan",
  "name": "Plan Mode",
  "description": "Claude can analyze but not modify files or execute commands"
}
```

- Agent can read files and analyze code
- Cannot modify files or execute commands
- Safe mode for code review/analysis

#### 4. Bypass Permissions Mode
```json
{
  "id": "bypassPermissions",
  "name": "Bypass Permissions",
  "description": "Skips all permission prompts"
}
```

- **Skips all permission prompts**
- Agent can perform any operation without user approval
- Use with caution: only in trusted environments
- Ideal for automation/CI scenarios

### Setting Permission Mode

To set the permission mode, clients should send it during session creation or use a mode-switching method (implementation-specific).

**Example: Session with bypass permissions**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "session/new",
  "params": {
    "cwd": "/workspace",
    "mcpServers": [],
    "mode": "bypassPermissions"
  }
}
```

### Automatic Permission Handling

Clients may automatically allow or reject permission requests based on:
- User settings/preferences
- Current permission mode
- Security policies
- Tool type or risk level

## Session Management

### Session Lifecycle

```
┌──────────────┐
│  Initialize  │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Session/New  │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Prompts    │◄──┐
│  & Updates   │   │
└──────┬───────┘   │
       │           │
       └───────────┘
       │
       ▼
┌──────────────┐
│  Session End │
└──────────────┘
```

### Session Properties

- **Independent Context**: Each session maintains isolated conversation history
- **Working Directory**: Anchors file operations to a specific path
- **MCP Servers**: Connected external tools/data sources
- **Session ID**: Unique identifier for routing messages
- **Model Selection**: Can change AI model during session
- **Permission Mode**: Controls approval workflow

### Multiple Sessions

Clients can create multiple concurrent sessions:

```json
// Session 1: Code review
{
  "id": 1,
  "method": "session/new",
  "params": {
    "cwd": "/workspace",
    "mode": "plan"
  }
}

// Session 2: Active development
{
  "id": 2,
  "method": "session/new",
  "params": {
    "cwd": "/workspace",
    "mode": "acceptEdits"
  }
}
```

## Error Handling

### Standard JSON-RPC Errors

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request",
    "data": {
      "details": "Missing required parameter: sessionId"
    }
  }
}
```

### Common Error Codes

- `-32700`: Parse error (invalid JSON)
- `-32600`: Invalid request (malformed structure)
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error

### Application-Specific Errors

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "error": {
    "code": 1001,
    "message": "Session not found",
    "data": {
      "sessionId": "invalid-session-id"
    }
  }
}
```

### Handling Connection Loss

If the agent process terminates unexpectedly:

1. Detect stdio stream closure
2. Clean up session state
3. Notify user of disconnection
4. Offer to restart agent and create new session

### Timeout Handling

Long-running operations should include timeout detection:

```typescript
async function sendPromptWithTimeout(
  sessionId: string,
  prompt: Prompt,
  timeoutMs: number = 120000
): Promise<PromptResult> {
  const timeoutPromise = new Promise((_, reject) =>
    setTimeout(() => reject(new Error("Prompt timeout")), timeoutMs)
  );

  const promptPromise = sendPrompt(sessionId, prompt);

  return Promise.race([promptPromise, timeoutPromise]);
}
```

## Issuing ACP Messages to claude-code-acp

### Process Spawning

From the session output, we see the agent spawns as a subprocess:

```bash
/usr/local/bin/node \
  /usr/local/lib/node_modules/@zed-industries/claude-code-acp/node_modules/@anthropic-ai/claude-agent-sdk/cli.js \
  --output-format stream-json \
  --verbose \
  --input-format stream-json \
  --permission-prompt-tool stdio \
  --mcp-config '{"mcpServers":{"acp":{"type":"sdk","name":"acp"}}}' \
  --setting-sources user,project,local \
  --permission-mode default \
  --allow-dangerously-skip-permissions \
  --include-partial-messages
```

### Key Command-Line Arguments

- `--output-format stream-json`: Agent sends newline-delimited JSON
- `--input-format stream-json`: Agent expects newline-delimited JSON
- `--permission-prompt-tool stdio`: Permission prompts via stdio
- `--permission-mode default`: Initial permission mode
- `--allow-dangerously-skip-permissions`: Enables bypass mode
- `--include-partial-messages`: Enables streaming updates

### Sending Messages

Write JSON-RPC messages to the agent's stdin, one per line:

```typescript
import { spawn } from 'child_process';
import { v4 as uuidv4 } from 'uuid';

const agent = spawn('node', [
  '/usr/local/lib/node_modules/@zed-industries/claude-code-acp/node_modules/@anthropic-ai/claude-agent-sdk/cli.js',
  '--output-format', 'stream-json',
  '--input-format', 'stream-json',
  '--permission-mode', 'bypassPermissions',
  '--allow-dangerously-skip-permissions'
]);

let requestId = 1;
let sessionId: string | null = null;

function sendRequest(method: string, params: any): Promise<any> {
  return new Promise((resolve, reject) => {
    const id = requestId++;
    const request = {
      jsonrpc: "2.0",
      id,
      method,
      params
    };

    // Write to stdin (newline-delimited)
    agent.stdin.write(JSON.stringify(request) + '\n');

    // Set up response handler
    const handler = (response: any) => {
      if (response.id === id) {
        if (response.error) {
          reject(response.error);
        } else {
          resolve(response.result);
        }
        // Remove this handler
        responseHandlers.delete(id);
      }
    };

    responseHandlers.set(id, handler);
  });
}

// Initialize
const initResult = await sendRequest('initialize', {
  protocolVersion: 1,
  clientInfo: { name: "my-client", version: "1.0" },
  capabilities: {}
});

// Create session
const sessionResult = await sendRequest('session/new', {
  cwd: "/workspace",
  mcpServers: []
});
sessionId = sessionResult.sessionId;

// Send prompt
const promptResult = await sendRequest('session/prompt', {
  sessionId,
  prompt: [{ type: "text", text: "Say hello" }]
});
```

### Receiving Messages

Read from agent's stdout, parsing newline-delimited JSON:

```typescript
import { createInterface } from 'readline';

const readline = createInterface({
  input: agent.stdout,
  crlfDelay: Infinity
});

const responseHandlers = new Map<number, (response: any) => void>();
const notificationHandlers: ((notification: any) => void)[] = [];

readline.on('line', (line) => {
  try {
    const message = JSON.parse(line);

    if (message.method) {
      // Notification (no id)
      notificationHandlers.forEach(handler => handler(message));

      if (message.method === 'session/update') {
        handleSessionUpdate(message.params);
      }
    } else if (message.id !== undefined) {
      // Response to a request
      const handler = responseHandlers.get(message.id);
      if (handler) {
        handler(message);
      }
    }
  } catch (err) {
    console.error('Failed to parse message:', err);
  }
});

readline.on('close', () => {
  console.log('Agent connection closed');
});
```

## Best Practices

### 1. Always Initialize First

```typescript
// Correct order
await initialize();
await createSession();
await sendPrompt();

// Wrong: skipping initialization
await createSession(); // Will fail
```

### 2. Handle All Session Updates

```typescript
function handleSessionUpdate(params: SessionUpdateParams) {
  switch (params.update.sessionUpdate) {
    case 'plan_update':
      updatePlanDisplay(params.update.planUpdate);
      break;
    case 'assistant_update':
      updateAssistantMessage(params.update);
      break;
    case 'tool_call_update':
      updateToolCall(params.update);
      break;
    case 'available_commands_update':
      updateCommandPalette(params.update.availableCommands);
      break;
  }
}
```

### 3. Use Appropriate Permission Modes

```typescript
// Code review (read-only)
await createSession({
  cwd: "/workspace",
  mode: "plan"
});

// Active development (auto-approve edits)
await createSession({
  cwd: "/workspace",
  mode: "acceptEdits"
});

// Automation/CI (skip all prompts)
await createSession({
  cwd: "/workspace",
  mode: "bypassPermissions"
});
```

### 4. Handle Streaming Gracefully

```typescript
// Accumulate text in real-time
const display = new Map<string, string>();

function handleDelta(messageId: string, blockIndex: number, delta: string) {
  const key = `${messageId}:${blockIndex}`;
  const current = display.get(key) || '';
  const updated = current + delta;
  display.set(key, updated);

  // Update UI incrementally
  updateDisplay(messageId, blockIndex, updated);
}
```

### 5. Clean Up Resources

```typescript
// Close session when done
agent.stdin.end();
agent.kill();

// Clear state
contentBlocks.clear();
responseHandlers.clear();
```

## Example: Complete Integration

Here's a complete example of integrating claude-code-acp:

```typescript
import { spawn, ChildProcess } from 'child_process';
import { createInterface, Interface } from 'readline';

class ClaudeCodeClient {
  private agent: ChildProcess;
  private readline: Interface;
  private requestId = 1;
  private responseHandlers = new Map<number, (response: any) => void>();
  private sessionId: string | null = null;

  constructor(private cwd: string, private permissionMode: string = 'default') {
    this.agent = spawn('node', [
      '/usr/local/lib/node_modules/@zed-industries/claude-code-acp/node_modules/@anthropic-ai/claude-agent-sdk/cli.js',
      '--output-format', 'stream-json',
      '--input-format', 'stream-json',
      '--permission-mode', permissionMode,
      '--allow-dangerously-skip-permissions'
    ]);

    this.readline = createInterface({
      input: this.agent.stdout,
      crlfDelay: Infinity
    });

    this.readline.on('line', this.handleMessage.bind(this));
  }

  private handleMessage(line: string) {
    const message = JSON.parse(line);

    if (message.method === 'session/update') {
      this.handleSessionUpdate(message.params);
    } else if (message.id !== undefined) {
      const handler = this.responseHandlers.get(message.id);
      if (handler) {
        handler(message);
      }
    }
  }

  private handleSessionUpdate(params: any) {
    // Handle streaming updates
    console.log('Session update:', params.update.sessionUpdate);
  }

  private sendRequest(method: string, params: any): Promise<any> {
    return new Promise((resolve, reject) => {
      const id = this.requestId++;
      const request = { jsonrpc: "2.0", id, method, params };

      this.agent.stdin.write(JSON.stringify(request) + '\n');

      this.responseHandlers.set(id, (response) => {
        this.responseHandlers.delete(id);
        if (response.error) {
          reject(response.error);
        } else {
          resolve(response.result);
        }
      });
    });
  }

  async initialize() {
    const result = await this.sendRequest('initialize', {
      protocolVersion: 1,
      clientInfo: { name: "my-client", version: "1.0" },
      capabilities: {}
    });
    console.log('Initialized:', result.agentInfo.name);
    return result;
  }

  async createSession() {
    const result = await this.sendRequest('session/new', {
      cwd: this.cwd,
      mcpServers: []
    });
    this.sessionId = result.sessionId;
    console.log('Session created:', this.sessionId);
    return result;
  }

  async sendPrompt(text: string) {
    if (!this.sessionId) {
      throw new Error('No active session');
    }

    const result = await this.sendRequest('session/prompt', {
      sessionId: this.sessionId,
      prompt: [{ type: "text", text }]
    });

    console.log('Prompt completed:', result.stopReason);
    return result;
  }

  close() {
    this.agent.stdin.end();
    this.agent.kill();
  }
}

// Usage
async function main() {
  const client = new ClaudeCodeClient('/workspace', 'bypassPermissions');

  try {
    await client.initialize();
    await client.createSession();
    await client.sendPrompt('Say hello in one sentence');
  } finally {
    client.close();
  }
}
```

## Summary

The Agent Client Protocol provides a standardized way for IDEs to communicate with AI coding agents:

1. **Initialization**: Negotiate capabilities and authenticate
2. **Session Creation**: Establish conversation context with working directory and permissions
3. **Prompt Turns**: Send user prompts and receive streaming responses
4. **Permission Handling**: Request user approval for sensitive operations (or bypass)
5. **Streaming**: Real-time updates via deltas for responsive UX
6. **Error Handling**: Standard JSON-RPC error responses

Key takeaways:
- Use `bypassPermissions` mode to skip all permission prompts
- Handle streaming with start/delta/stop flow
- Accumulate deltas incrementally for real-time display
- Each session is independent with isolated context
- Notifications (no id) don't expect responses
- Methods (with id) always receive responses

## References

- [Agent Client Protocol Specification](https://agentclientprotocol.com)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [Claude Code Documentation](https://docs.anthropic.com/claude-code)
