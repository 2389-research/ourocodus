# Milestone 3 Implementation Design

**Date:** 2025-01-07
**Status:** Approved
**Goal:** Complete core PWA features and key infrastructure polish

## Overview

Milestone 3 delivers six features that complete the core PWA functionality: agent cards, chat interface, multi-agent support, server protocol handlers, configurable logging, and embedded file serving. These features transform the PWA from a basic prototype into a functional multi-agent platform.

## Scope

### In Scope (6 Issues)
- #46: go:embed file serving
- #65: Configurable logging system
- #64: Server protocol handlers (session:end, agent:terminate)
- #11: Agent cards UI
- #12: Chat interface
- #63: Multi-agent support

### Out of Scope (6 Issues)
Deferred to later milestones:
- #66: Theme toggle (UX polish)
- #60, #61, #62: Progress indicators, retry buttons, inspector features (UX polish)
- #47, #48: Accessibility, PWA manifest (research/nice-to-have)

## Implementation Sequence

We implement in dependency order to build a solid foundation:

1. **#46 - go:embed File Serving** (Infrastructure foundation)
2. **#65 - Configurable Logging** (Development experience)
3. **#64 - Server Protocol Handlers** (Protocol completeness)
4. **#11 - Agent Cards UI** (User interaction foundation)
5. **#12 - Chat Interface** (Core feature)
6. **#63 - Multi-Agent Support** (Advanced feature)

## Technical Design

### #46: go:embed File Serving

**Problem:** The relay serves PWA files from `./web`, which requires running the binary from the repository root. Directory listing is enabled by default.

**Solution:** Embed PWA assets at build time using `//go:embed web/*` directive.

**Changes:**
- Add `//go:embed web/*` to main.go
- Replace `http.FileServer(http.Dir("./web"))` with `http.FileServer(http.FS(embeddedFS))`
- No code changes needed in web/ directory

**Benefits:**
- Self-contained binary eliminates deployment complexity
- No working directory dependency
- Directory listing prevented automatically
- Build size increases ~50KB

### #65: Configurable Logging System

**Problem:** PWA uses `console.log()` everywhere with no way to control output. Production console is noisy, debugging requires code changes.

**Solution:** Create logger with configurable levels (debug/info/warn/error) stored in localStorage.

**Implementation:**
```javascript
class Logger {
    constructor(component, options = {}) {
        this.component = component;
        this.level = options.level || Logger.getDefaultLevel();
    }

    static levels = { debug: 0, info: 1, warn: 2, error: 3, none: 4 };

    static getDefaultLevel() {
        return localStorage.getItem('ourocodus.logLevel') || 'info';
    }

    debug(msg, ...args) {
        if (this._shouldLog('debug')) {
            console.log(`[${this.component}] ${msg}`, ...args);
        }
    }

    // Similar for info, warn, error
}
```

**Migration:**
- Replace `console.log` → `logger.debug` or `logger.info`
- Replace `console.error` → `logger.error`
- Create logger instances per component

**Benefits:**
- Clean production console (default level: info)
- Easy debugging (set `localStorage.ourocodus.logLevel = 'debug'`)
- Per-component control
- No performance impact when disabled

### #64: Server Protocol Handlers

**Problem:** Client sends `session:end` and `agent:terminate` messages but server returns `UNKNOWN_MESSAGE_TYPE` errors.

**Solution:** Add handlers in relay's WebSocket message dispatcher.

**session:end Handler:**
- Terminates all agents in session (SIGTERM, wait, SIGKILL if needed)
- Cleans agent workspaces
- Returns `session:ended` response with agent count and cleanup status
- Keeps WebSocket open (client closes when ready)

**agent:terminate Handler:**
- Terminates specific agent by role
- Cleans agent workspace
- Returns `agent:terminated` response with cleanup confirmation
- Session remains active

**Error Handling:**
- Session not found → return error with `recoverable: false`
- Agent not found → return error with `recoverable: true`
- Termination timeout → force kill after 5 seconds
- Cleanup failure → log warning, return success (process is dead)

**Benefits:**
- Proper resource cleanup
- Graceful shutdown
- Protocol completeness
- Enables session management features

### #11: Agent Cards UI

**Problem:** PWA replaces spawn section with single agent card. No support for multiple agents.

**Solution:** Refactor state to Map-based structure, keep spawn section always visible.

**State Changes:**
```javascript
// Before: single agent
let currentAgentRole = null;
let messages = [];

// After: multiple agents
const agents = new Map(); // role → AgentState
// AgentState: { role, status, messages, workspace }
```

**UI Structure:**
```
┌─────────────────────────────────┐
│  Spawn Agent                    │
│  Role: [____] Workspace: [____] │
│  [Spawn Agent]                  │
└─────────────────────────────────┘

┌─────────────────────────────────┐
│  Agent: echo          [Ready]   │
│  Messages: 5          [×]       │
└─────────────────────────────────┘
```

**Component:**
- AgentCard shows role badge, status indicator, message count, terminate button
- Cards in scrollable list
- Click card to expand/show chat

**Benefits:**
- Foundation for multi-agent
- Always-visible spawn controls
- Clear status per agent

### #12: Chat Interface

**Problem:** No way to communicate with agents.

**Solution:** Add chat interface that expands from agent card.

**UI:**
- Message list (scrollable, auto-scroll to bottom)
- Input field with send button (Enter submits)
- User messages right-aligned (blue)
- Agent responses left-aligned (gray)

**Protocol:**
```javascript
// Send message
ws.send({
    type: "agent:message",
    sessionId: currentSessionId,
    role: agentRole,
    content: messageText
});

// Receive response
// Message routed to agents.get(role).messages
```

**State:**
- Messages stored per-agent: `agents.get(role).messages[]`
- Each message: `{sender: 'user'|'agent', content: string, timestamp: Date}`

**Benefits:**
- Complete agent interaction
- Persistent history per agent
- Clean messenger-style UI

### #63: Multi-Agent Support

**Problem:** UI needs to handle multiple concurrent agents.

**Solution:** Build on #11's Map-based state. No single-agent restrictions.

**Changes:**
- Remove logic that hides spawn section after first agent
- Protocol routing: use `role` field to route messages to correct agent
- UI: show all active agents simultaneously
- Add "Terminate All" button for bulk operations

**Independent Operation:**
- Each agent has isolated message history
- Each agent has independent status
- Each agent's chat expands/collapses independently

**Benefits:**
- Demonstrates full platform capability
- Realistic use cases
- Better testing of agent interactions
- Natural extension of #11 and #12

## Testing Strategy

Each issue includes verification steps:

1. **#46:** Build binary, run from any directory, verify PWA loads
2. **#65:** Set log level to debug/info/error, verify console output
3. **#64:** Send session:end and agent:terminate, verify responses and cleanup
4. **#11:** Spawn multiple agents, verify cards appear
5. **#12:** Send messages, verify chat history
6. **#63:** Run 3+ agents simultaneously, verify independent operation

## Success Criteria

- Binary runs from any directory (go:embed working)
- Log level controls console output (logger working)
- session:end and agent:terminate return success (protocol complete)
- Multiple agent cards display simultaneously (UI foundation)
- Chat interface sends/receives messages (interaction working)
- Multiple agents operate independently (multi-agent working)

## Timeline

With AI agents implementing:
- Day 1-2: Infrastructure (#46, #65, #64)
- Day 3-4: UI Foundation (#11, #12)
- Day 5: Multi-Agent (#63)
- Day 6-7: Testing and integration

## Risks and Mitigations

**Risk:** go:embed breaks development workflow
**Mitigation:** Keep `./web` directory structure unchanged, test both dev and production modes

**Risk:** State refactoring breaks existing features
**Mitigation:** Implement #11 incrementally, test before proceeding to #12

**Risk:** Protocol handlers conflict with existing cleanup
**Mitigation:** Review existing cleanup code before implementing #64

## Dependencies

- #11 must complete before #12 (chat needs agent cards)
- #11 must complete before #63 (multi-agent extends cards)
- No other hard dependencies (can parallelize infrastructure track)

## References

- Issue #46: https://github.com/2389-research/ourocodus/issues/46
- Issue #65: https://github.com/2389-research/ourocodus/issues/65
- Issue #64: https://github.com/2389-research/ourocodus/issues/64
- Issue #11: https://github.com/2389-research/ourocodus/issues/11
- Issue #12: https://github.com/2389-research/ourocodus/issues/12
- Issue #63: https://github.com/2389-research/ourocodus/issues/63
