# Ourocodus Architecture: Phase 1 vs Long-term

## Phase 1: Proof of Concept (Current)

**Goal:** Validate multi-agent communication and concurrent work

```
┌────────────────────────────────┐
│ PWA (Browser)                  │
│ - User Session View            │
│ - 0-N agent chat UIs           │
└────────────┬───────────────────┘
             │ WebSocket
             │
┌────────────▼───────────────────┐
│ Relay (Go)                     │
│ - UserSession (container)      │
│ - Routes messages              │
│ - Spawns agent processes       │
│ - In-memory state              │
└─┬────────┬────────┬────────────┘
  │ stdio  │ stdio  │ stdio (0-N)
  │        │        │
┌─▼────┐ ┌─▼────┐ ┌─▼────┐
│Agent │ │Agent │ │Agent │
│auth  │ │db    │ │test  │
│Claude│ │Claude│ │Claude│
│Code  │ │Code  │ │Code  │
│ACP   │ │ACP   │ │ACP   │
│(proc)│ │(proc)│ │(proc)│
└──────┘ └──────┘ └──────┘

Note: Roles are dynamic, not hardcoded
      Agent failure doesn't terminate session
      Agents can be spawned/terminated independently
```

**Key Characteristics:**
- No NATS (direct WebSocket + stdio)
- No Coordinator (user drives manually)
- Processes not containers
- In-memory session state
- Variable agent count (0-N agents per session)
- Dynamic roles (user-specified, not hardcoded)
- Independent agent lifecycles (agents can fail without affecting session)

**Limitations:**
- Not fault-tolerant (process crash = lost state)
- Not scalable (in-memory only)
- No workflow automation
- Manual git operations
- No approval gates

---

## Long-term: Production Architecture

**Goal:** Autonomous multi-agent workflow system

```
┌──────────────────────┐
│ PWA (Browser)        │
└──────────┬───────────┘
           │ WebSocket
           │
┌──────────▼───────────┐
│ API Server (Go)      │
└──────────┬───────────┘
           │
┌──────────▼───────────┐
│ NATS Message Bus     │
└─┬────────┬───────────┬┘
  │        │           │
┌─▼────────▼──┐  ┌────▼──────┐
│ Coordinator │  │ Relay     │
│ (Go)        │  │ (Go)      │
│ - Graph     │  │ - ACP     │
│ - Workflow  │  │   adapter │
│ - Approvals │  └────┬──────┘
└─────────────┘       │
                      │ WebSocket/stdio
                      │
             ┌────────▼────────┐
             │ Claude Code     │
             │ (containers)    │
             └─────────────────┘
```

**Key Characteristics:**
- NATS for all backend communication
- Coordinator drives workflow
- Relay is protocol adapter only
- Containers for isolation
- SQLite event store
- Sequential or parallel (graph-driven)
- Dynamic workflow generation

**Additions:**
- Fault tolerance (event sourcing)
- Horizontal scaling (NATS clustering)
- Workflow automation (coordinator)
- Approval gates
- Git merge automation
- PRD generation

---

## Migration Path

### Phase 1 → Phase 2: Add NATS
- Keep relay + ACP integration
- Add NATS between PWA and relay
- Relay subscribes to NATS topics
- Still no coordinator

### Phase 2 → Phase 3: Add Coordinator
- Coordinator reads graph
- Coordinator publishes work to NATS
- Relay stays as ACP adapter
- Add approval gate service

### Phase 3 → Phase 4: Production-ize
- SQLite event store
- Container isolation
- Error recovery
- Monitoring/observability

---

## Why This Phased Approach?

**Phase 1 validates the hard part:**
- Can we route messages to multiple ACP instances?
- Can agents work concurrently on same codebase?
- Does the UX model (PWA with multiple chats) work?

**Later phases add infrastructure:**
- Once routing works, add NATS for scalability
- Once manual works, add coordinator for automation
- Once POC works, add production features

**Don't build infrastructure before proving the concept.**
