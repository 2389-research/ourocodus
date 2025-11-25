# CLI↔Web Interoperability Demo

**Date:** 2025-11-24
**Duration:** ~10 minutes
**Prerequisites:** NATS server running, relay running, Docker available

This demo showcases Ourocodus's ability to spawn agents from CLI and adopt them from the web interface, demonstrating seamless interoperability between terminal and browser workflows.

## Demo Overview

```
┌─────────────────┐        ┌─────────────────┐        ┌─────────────────┐
│   CLI (agentd)  │───────▶│   NATS + Relay  │◀───────│   Web (PWA)     │
│                 │        │                 │        │                 │
│  spawn alice    │        │  heartbeats     │        │  adopt alice    │
│  watch alice    │        │  lease events   │        │  send messages  │
│  stop alice     │        │  agent messages │        │  see responses  │
└─────────────────┘        └─────────────────┘        └─────────────────┘
```

## Setup

### Terminal 1: Start NATS
```bash
nats-server
```

### Terminal 2: Start Relay
```bash
go run ./cmd/relay
```

You should see the beautiful startup banner:
```
╔══════════════════════════════════════╗
║                                      ║
║        ▄█▀▀█▄ █  █ █▀▀█ ▄█▀▀█▄       ║
║        █    █ █  █ █▄▄▀ █    █       ║
║        ▀█▄▄█▀ ▀▄▄▀ █  █ ▀█▄▄█▀       ║
║                                      ║
║          OUROCODUS RELAY             ║
╚══════════════════════════════════════╝

✓ Server ready
  → HTTP:      http://localhost:8080
  → WebSocket: ws://localhost:8080/ws
  → NATS:      connected
```

### Terminal 3: CLI Operations
This is where we'll run agentd commands.

## Part 1: Spawn Agent from CLI

### 1.1 Validate Environment
```bash
agentd doctor
```

Expected output:
```
🩺 Environment Health Check

Docker
  ✓ Docker daemon accessible
  ✓ Can run containers

Git
  ✓ Git available (2.x.x)
  ✓ In a git repository

NATS
  ✓ Connected to nats://localhost:4222

✓ All checks passed
```

### 1.2 Spawn an Agent
```bash
agentd spawn alice
```

Expected output (rich mode):
```
✨ Creating isolated agent 'alice'...

🌳 Worktree: .agentd/worktrees/alice (branch: agent-alice-20251124-...)
📦 Container: abc123... (running)
🔑 Credentials: mounted at /root/.creds (read-only)

🔐 Attach Token:
   Wm9tYmllX2F0dGFjaF90b2tlbi4uLg==

   Use this token when attaching from PWA or relay:
   → agent:attach {"agentId": "alice", "token": "<token>"}

✓ Agent alice ready
```

### 1.3 List Agents
```bash
agentd list
```

Expected output:
```
╭───────────┬───────────────┬───────────┬─────────────────╮
│ AGENT ID  │ CONTAINER     │ STATUS    │ UPTIME          │
├───────────┼───────────────┼───────────┼─────────────────┤
│ alice     │ abc123...     │ running   │ 10s             │
╰───────────┴───────────────┴───────────┴─────────────────╯
```

### 1.4 Watch Agent Events
```bash
agentd watch alice
```

Expected output:
```
👁️  Watching agent: alice
Press Ctrl+C to stop...

✓ Subscribed to: agent.heartbeat.alice

[14:30:05] 💓 Heartbeat received (lag=12ms, status=idle)
[14:30:10] 💓 Heartbeat received (lag=8ms, status=idle)
```

## Part 2: Adopt from Web

### 2.1 Open Web Interface
Navigate to: `http://localhost:8080`

### 2.2 Connect WebSocket
The PWA should auto-connect. Status indicator should show:
- 🟢 Connected

### 2.3 Adopt the Agent
In the PWA command input, type:
```
agent:adopt {"agentId": "alice"}
```

### 2.4 Observe CLI Watch Output
Back in Terminal 3 (watch), you should see lease events:
```
[14:30:45] 🔐 Lease renewed (expires in 30s, session=ws-abc123...)
```

### 2.5 Send a Message
In the PWA, type a message:
```
Hello from the web! Can you tell me what files are in /workspace?
```

The agent should respond through the PWA interface.

## Part 3: Output Modes

### 3.1 JSON Mode (for scripting)
```bash
agentd spawn bob --json
```

Output:
```json
{
  "agentId": "bob",
  "containerId": "def456...",
  "workspacePath": ".agentd/worktrees/bob",
  "branchName": "agent-bob-20251124-...",
  "attachToken": "...",
  "status": "running"
}
```

```bash
agentd list --json
```

Output:
```json
{
  "agents": [
    {
      "agentId": "alice",
      "containerId": "abc123...",
      "status": "running",
      "uptime": 120
    },
    {
      "agentId": "bob",
      "containerId": "def456...",
      "status": "running",
      "uptime": 5
    }
  ],
  "count": 2
}
```

### 3.2 Plain Mode (for logs/CI)
```bash
agentd list --plain
```

Output:
```
AGENT ID    CONTAINER       STATUS    UPTIME
alice       abc123...       running   2m
bob         def456...       running   30s
```

## Part 4: Cleanup

### 4.1 Stop Agents
```bash
agentd stop alice bob
```

Expected output:
```
🛑 Stopping agent 'alice'...

✓ Stopped container abc123...
✓ Removed worktree .agentd/worktrees/alice
✓ Agent alice stopped and cleaned up

🛑 Stopping agent 'bob'...

✓ Stopped container def456...
✓ Removed worktree .agentd/worktrees/bob
✓ Agent bob stopped and cleaned up
```

### 4.2 Verify Cleanup
```bash
agentd list
```

Expected output:
```
No agents running
```

## Key Takeaways

1. **Three-Layer Isolation**: Each agent runs in its own:
   - Git worktree (isolated code)
   - Docker container (isolated process)
   - Credentials mount (secure secrets)

2. **Real-time Monitoring**: The `watch` command shows:
   - Heartbeats from the agent container
   - Lease events when sessions attach/detach

3. **Flexible Output**: All commands support:
   - Rich mode (default): Beautiful TUI with colors
   - Plain mode (`--plain`): Clean text for logs
   - JSON mode (`--json`): Machine-readable for scripting

4. **CLI↔Web Interop**: Agents spawned from CLI can be:
   - Monitored in real-time via `agentd watch`
   - Adopted and controlled from the web PWA
   - Messages flow seamlessly between interfaces

## Troubleshooting

### Agent not appearing in list
```bash
docker ps --filter "label=ourocodus.agent=true"
```

### NATS connection issues
```bash
agentd doctor
```

### Container logs
```bash
docker logs $(docker ps -q --filter "label=ourocodus.agent-id=alice")
```
