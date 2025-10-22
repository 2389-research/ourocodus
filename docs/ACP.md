# ACP (Agent Client Protocol) Integration

**Note:** ACP is the "Agent Client Protocol" by Zed Industries/Google, not Anthropic. Anthropic has MCP (Model Context Protocol). We use ACP because Claude Code supports it.

## Overview

Ourocodus does NOT implement custom AI agents. Instead, it orchestrates **existing ACP-compatible servers** (Claude Code, OpenAI Codex, etc.). The relay routes ACP messages between the PWA/coordinator and these agent processes/containers.

## Core Principle

**We are ACP clients, not ACP servers.**

The "agents" are Claude Code, OpenAI Codex, or other tools that:
- Speak ACP protocol
- Have access to git, code editing, terminal
- Can be run in containers

Ourocodus provides:
1. Container orchestration (launch, stop, monitor)
2. Message routing (NATS → ACP containers)
3. Workflow coordination (sequential execution, approval gates)
4. Observability (logging, status, events)

## Architecture

```
┌─────────────┐
│ Coordinator │
│    (Go)     │
└──────┬──────┘
       │ Send ACP sampling request via NATS
       │
┌──────▼──────────────────────────────────────┐
│  ACP Relay                                   │
│  - Receives NATS messages                    │
│  - Translates to ACP WebSocket/HTTP         │
│  - Routes to correct agent container         │
└──────┬──────────────────────────────────────┘
       │ ACP over WebSocket
       │
┌──────▼──────┐   ┌─────────────┐   ┌────────────┐
│ Claude Code │   │ OpenAI Codex│   │   Future   │
│ Container   │   │  Container  │   │  ACP Tools │
│             │   │             │   │            │
│ - git clone │   │ - git clone │   │ - git ops  │
│ - edit files│   │ - edit files│   │ - terminal │
│ - run tests │   │ - run tests │   │ - etc.     │
└─────────────┘   └─────────────┘   └────────────┘
       │
       └─────► Shared worktree volume (git branch per container)
```

## ACP Protocol Basics

### Message Flow

1. **Coordinator → Agent (Sampling Request)**
```json
{
  "id": "msg_123",
  "method": "sampling.request",
  "params": {
    "messages": [
      {
        "role": "user",
        "content": "Implement user authentication using bcrypt"
      }
    ],
    "model": "claude-sonnet-4",
    "max_tokens": 4096,
    "tools": [
      {
        "name": "bash",
        "description": "Execute bash commands",
        "input_schema": {...}
      },
      {
        "name": "edit_file",
        "description": "Edit file contents",
        "input_schema": {...}
      }
    ]
  }
}
```

2. **Agent → Coordinator (Tool Use)**
```json
{
  "id": "msg_123",
  "result": {
    "type": "tool_use",
    "name": "bash",
    "input": {
      "command": "git checkout -b auth-feature"
    }
  }
}
```

3. **Coordinator → Agent (Tool Result)**
```json
{
  "id": "msg_124",
  "method": "tool_result",
  "params": {
    "tool_use_id": "tool_123",
    "result": "Switched to a new branch 'auth-feature'"
  }
}
```

4. **Agent → Coordinator (Final Response)**
```json
{
  "id": "msg_125",
  "result": {
    "type": "text",
    "content": "I've implemented user authentication in src/auth.go with bcrypt password hashing."
  }
}
```

## Container Requirements

Each agent container must:

1. **Run an ACP server** (Claude Code, Codex, etc.)
2. **Expose ACP endpoint** (WebSocket or HTTP/SSE)
3. **Have git access** (clone repo, create branches, commit)
4. **Mount worktree** (isolated git branch per agent)
5. **Connect to relay** (via env var: `ACP_RELAY_URL`)

### Example: Claude Code Container

```dockerfile
FROM ubuntu:22.04

# Install Claude Code
RUN curl -fsSL https://install.claude.com | bash

# Install git, build tools
RUN apt-get update && apt-get install -y \
    git \
    build-essential \
    curl

# Worktree will be mounted at /workspace
VOLUME /workspace

# Relay URL passed as env var
ENV ACP_RELAY_URL=ws://relay:8080/acp

# Start Claude Code in server mode
CMD ["claude-code", "--server", "--workspace", "/workspace"]
```

### Example: Agent Config

```yaml
# config/agent-config.yaml
agents:
  - name: coding
    image: ourocodus/claude-code:latest
    capabilities:
      - code_editing
      - git_operations
      - terminal_access
    tools:
      - bash
      - edit_file
      - read_file
      - write_file
      - search_files

  - name: testing
    image: ourocodus/claude-code:latest
    capabilities:
      - code_editing
      - test_execution
    tools:
      - bash
      - edit_file
      - read_file
      - pytest
```

## Relay Implementation

The ACP relay bridges NATS (internal) and ACP (containers):

### Relay Responsibilities

1. **Session management** - Track which agent belongs to which session
2. **Protocol translation** - NATS JSON → ACP WebSocket
3. **Connection pooling** - Maintain WebSocket connections to agents
4. **Message routing** - Route based on session_id + agent_role
5. **Error handling** - Reconnect on disconnection, report failures

### Relay API (Internal - NATS)

**Subscribe to:**
```
sessions.{session_id}.work.{role}   # Work for specific agent role
```

**Publish to:**
```
sessions.{session_id}.results.{role}  # Results from agent
sessions.{session_id}.events          # Status events
```

### Relay API (External - ACP Containers)

**WebSocket endpoint:**
```
ws://relay:8080/acp/{session_id}/{role}
```

**Headers:**
```
X-Agent-ID: agent_abc123
X-Session-ID: sess_xyz789
X-Agent-Role: coding
```

## Workflow Example

### Scenario: Implement authentication feature

1. **Coordinator reads graph:**
```yaml
# graph.yaml
chunks:
  - id: auth-implementation
    phases:
      - coding
      - testing
      - review
```

2. **Coordinator launches coding agent:**
```
POST /api/agents
{
  "session_id": "sess_123",
  "role": "coding",
  "worktree": "agent/auth-implementation"
}
```

3. **Relay connects to agent container:**
- Container starts with Claude Code
- Claude Code connects to relay WebSocket
- Relay registers: `sess_123` + `coding` → `ws://container_ip:5000`

4. **Coordinator sends work via NATS:**
```
Topic: sessions.sess_123.work.coding
Message: {
  "type": "work.coding",
  "payload": {
    "task": "Implement user authentication with bcrypt",
    "requirements": [...]
  }
}
```

5. **Relay translates to ACP:**
```json
{
  "method": "sampling.request",
  "params": {
    "messages": [{
      "role": "user",
      "content": "Implement user authentication with bcrypt. Requirements: [...]"
    }],
    "tools": ["bash", "edit_file", "read_file"]
  }
}
```

6. **Claude Code executes:**
- Checkouts branch: `git checkout -b agent/auth-implementation`
- Creates files: `src/auth.go`
- Writes tests: `src/auth_test.go`
- Runs tests: `go test ./...`
- Commits: `git commit -m "Add authentication"`

7. **Relay receives result:**
```json
{
  "result": {
    "type": "text",
    "content": "Implemented authentication with bcrypt. Tests passing."
  }
}
```

8. **Relay publishes to NATS:**
```
Topic: sessions.sess_123.results.coding
Message: {
  "type": "result.success",
  "payload": {
    "summary": "Implemented authentication with bcrypt. Tests passing.",
    "files_changed": ["src/auth.go", "src/auth_test.go"],
    "commit_sha": "abc123"
  }
}
```

9. **Coordinator receives result, requests approval:**
```
Topic: sessions.sess_123.approvals
Message: {
  "type": "approval.request",
  "payload": {
    "phase": "post-coding",
    "summary": "Review changes before proceeding to testing?"
  }
}
```

10. **Human approves → Coordinator continues to testing phase**

## Tool Availability

Different agents may have different tools available:

**Claude Code tools:**
- `bash` - Execute shell commands
- `edit_file` - Edit file with search/replace
- `write_file` - Write new file
- `read_file` - Read file contents
- `search_files` - Grep/find files

**Custom tools (future):**
- `run_tests` - Run test suite
- `lint` - Run linter
- `format` - Format code
- `git_commit` - Atomic git operations

## Error Handling

### Agent Container Crashes

**Detection:** WebSocket disconnect or container exit

**Response:**
1. Relay publishes event: `event.agent.crashed`
2. Coordinator marks chunk as failed
3. Manual intervention required (POC)
4. Future: Auto-restart with state recovery

### ACP Protocol Errors

**Scenario:** Malformed ACP message

**Response:**
1. Log error with full message
2. Send error response to coordinator
3. Continue processing (don't crash relay)

### Tool Execution Failures

**Scenario:** `bash` tool returns non-zero exit

**Response:**
- Agent decides how to handle (ACP server responsibility)
- Coordinator treats as normal result
- Agent may retry, ask for help, or fail gracefully

## Security Considerations

### POC Assumptions

- Containers run on localhost
- No authentication between components
- Agents have full filesystem access to worktree
- No rate limiting or resource constraints

### Post-POC Requirements

1. **Sandboxing** - Restrict agent filesystem/network access
2. **Authentication** - Verify coordinator identity
3. **Encryption** - TLS for relay ↔ agent communication
4. **Resource limits** - CPU/memory/token limits per agent
5. **Audit logging** - All agent actions logged

## Implementation Checklist

- [ ] ACP protocol parser (Go pkg)
- [ ] WebSocket server (relay → agents)
- [ ] NATS ↔ ACP translation layer
- [ ] Claude Code Dockerfile
- [ ] Agent launcher (start container with volume mounts)
- [ ] Session → Agent mapping (relay state)
- [ ] Error handling (reconnection, timeouts)
- [ ] Event logging (all ACP messages)

## Testing Strategy

### Unit Tests
- ACP message parsing/formatting
- Protocol translation (NATS ↔ ACP)

### Integration Tests
- Real Claude Code container
- Send work, receive results
- Tool execution (bash, edit_file)
- Multi-turn conversations

### E2E Tests
- Full workflow: coding → approval → testing
- Multiple agents sequentially
- Error scenarios (crash, timeout)

## References

- [ACP Specification](https://github.com/anthropics/anthropic-sdk-python/blob/main/acp/README.md)
- [Claude Code Documentation](https://docs.claude.com/claude-code)
- [NATS Documentation](https://docs.nats.io/)
- [Docker SDK for Go](https://pkg.go.dev/github.com/docker/docker)
