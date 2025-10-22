# Ourocodus Implementation Phases

## Philosophy

**Build the foundation first.** Validate that messages can route, containers can communicate, and the system is observable before adding workflow orchestration.

## Phase 1: Foundation (Week 1-2)

**Goal:** Validate infrastructure works - messages route, agents connect, system is observable

**No workflow logic. No AI yet. Just plumbing.**

### What We Build

1. **NATS Setup**
   - Run NATS server locally
   - Test pub/sub with CLI

2. **Relay (Minimal)**
   - WebSocket server accepts connections
   - Subscribes to NATS topics
   - Forwards messages: NATS → WebSocket
   - Forwards messages: WebSocket → NATS
   - **No ACP translation yet** - just forward JSON as-is

3. **Test Agent (Echo Service)**
   - Simple Go program that:
     - Connects to relay via WebSocket
     - Receives message
     - Echoes it back
   - Run in Docker container
   - **Not Claude Code yet** - just validates container → relay communication

4. **CLI (Manual Control)**
   - `ourocodus send` - Publish message to NATS topic
   - `ourocodus tail` - Subscribe and print messages
   - `ourocodus agent start` - Launch echo agent container
   - `ourocodus agent stop` - Stop container

5. **API Server (Basic)**
   - `GET /health` - System health
   - `GET /agents` - List running containers
   - `GET /events` - Tail event stream (SSE)

### Success Criteria

✅ Can publish message via CLI → appears on NATS topic
✅ Relay forwards NATS message → Echo agent receives it
✅ Echo agent responds → message appears back on NATS
✅ Web UI shows live event stream
✅ Can launch/stop agent containers via CLI
✅ All components log to structured JSON

### What We Learn

- NATS topic structure works
- WebSocket connections are stable
- Docker networking is correct
- Event logging captures everything
- CLI is usable for manual testing

### Explicitly Deferred

- ACP protocol translation (relay just forwards JSON)
- Real AI agents (use echo service)
- Workflow automation (manual via CLI)
- Approval gates
- Graph engine
- Error recovery

## Phase 2: ACP Integration (Week 3)

**Goal:** Replace echo agent with real Claude Code, implement ACP protocol translation

### What We Build

1. **ACP Protocol Translation (Relay)**
   - Translate internal JSON → ACP sampling requests
   - Parse ACP responses → internal JSON
   - Handle multi-turn conversations (tool use → tool result)

2. **Claude Code Container**
   - Dockerfile with Claude Code installed
   - Git configured
   - Volume mount for worktree
   - Connects to relay on startup

3. **Worktree Management**
   - CLI command: `ourocodus worktree create`
   - Creates `agent/{id}` branch
   - Mounts to container

4. **Real Coding Task**
   - Send work message: "Create a simple Go HTTP server"
   - Claude Code receives via ACP
   - Writes code, commits
   - Responds with results

### Success Criteria

✅ Claude Code container connects to relay
✅ Relay sends valid ACP sampling request
✅ Claude Code can use tools (bash, edit_file, etc.)
✅ Relay handles tool_use → tool_result cycle
✅ Claude Code commits code to worktree branch
✅ Full conversation is logged

### What We Learn

- ACP protocol details (quirks, edge cases)
- Claude Code container requirements
- Tool execution patterns
- Conversation flow management

## Phase 3: Workflow Automation (Week 4)

**Goal:** Automated workflow via Coordinator

### What We Build

1. **Graph Engine**
   - Parse graph.yaml
   - Extract chunks and dependencies
   - Validate graph structure

2. **Coordinator Service**
   - Read graph
   - Send work messages sequentially
   - Wait for results
   - Progress through chunks

3. **Approval Gates**
   - Coordinator blocks on approval request
   - CLI command: `ourocodus approve {gate_id}`
   - Or: Web UI approval button

4. **Full Workflow**
   - Define 2-chunk workflow in YAML
   - Run: `ourocodus run graph.yaml`
   - System completes both chunks end-to-end

### Success Criteria

✅ Coordinator reads and validates graph
✅ Sends work for chunk 1 → waits for result
✅ Approval gate blocks progression
✅ After approval, sends work for chunk 2
✅ Full workflow completes without manual intervention (except approvals)

### What We Learn

- Workflow state management
- Approval gate patterns
- Multi-chunk coordination
- Error handling strategies

## Phase 4: Robustness (Week 5+)

**Goal:** Make it production-ready

### What We Build

1. **Error Recovery**
   - Agent crashes → restart and retry
   - Coordinator crashes → rebuild state from events
   - Network failures → exponential backoff

2. **Multiple Agent Types**
   - Coding agent
   - Testing agent
   - Review agent
   - Each with specialized prompts

3. **Event Store (SQLite)**
   - Replace JSON log files
   - Queryable history
   - Snapshot state for fast startup

4. **Web UI Enhancements**
   - Session management
   - Workflow visualization
   - Approval UI
   - Live progress tracking

## Phase 5: Advanced Features (Future)

- Parallel chunk execution
- Multiple agents per chunk
- PRD decomposition engine
- GitHub integration (automated PRs)
- Cost tracking and limits
- Performance evaluation agents

## Why This Order?

**Phase 1 is critical.** If messages don't route correctly, nothing else matters. We validate the entire communication stack before adding AI complexity.

**Phase 2 adds AI but keeps workflow manual.** We learn ACP protocol details without fighting coordinator bugs simultaneously.

**Phase 3 automates what we've already proven works.** Coordinator just orchestrates known-good components.

## Anti-Patterns to Avoid

❌ Building coordinator before relay works
❌ Trying to run Claude Code before echo agent works
❌ Adding approval gates before basic workflow runs
❌ Implementing error recovery before happy path works
❌ Building web UI before CLI works

## Validation Gates

Each phase ends with a demo:

**Phase 1 Demo:** "Watch this message go CLI → NATS → Relay → Echo Agent → back to CLI"

**Phase 2 Demo:** "Claude Code writes a working HTTP server from a prompt"

**Phase 3 Demo:** "Two-chunk workflow completes end-to-end with approval gates"

Only proceed to next phase when current phase demo works reliably.

## Resource Estimates

| Phase | Time | Complexity | Risk |
|-------|------|------------|------|
| 1: Foundation | 2 weeks | Low | Low |
| 2: ACP | 1 week | Medium | Medium (protocol quirks) |
| 3: Workflow | 1 week | Medium | Low (building on proven base) |
| 4: Robustness | 2+ weeks | High | Medium (many edge cases) |

**Total for MVP (Phases 1-3): ~4 weeks**

## Getting Started

Start here:

```bash
# Phase 1, Step 1: Get NATS running
docker run -p 4222:4222 nats:latest

# Phase 1, Step 2: Test with CLI
go run cmd/cli/main.go send --topic test --message "hello"
go run cmd/cli/main.go tail --topic test
```

If those two commands work, you have a functioning message bus. Everything else builds on that.
