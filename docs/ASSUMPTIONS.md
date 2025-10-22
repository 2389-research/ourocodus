# Phase 1 Assumptions and Unknowns

## Critical Assumptions (Need Validation)

### 1. ACP Process Behavior

**Assumption:** `claude-code-acp` auto-commits changes to the current git branch

**Status:** ⚠️ **UNVERIFIED**

**Validation needed:**
```bash
# Test this before Issue #3:
cd test-worktree
claude-code-acp --workspace .
# Send message: "create test.txt with hello world"
# Check: git log (is there a commit?)
```

**If FALSE:** We need to add git commit logic to relay after each agent response

---

**Assumption:** `claude-code-acp` is installed globally via npm

**Status:** ⚠️ **UNVERIFIED**

**Validation:**
```bash
npm install -g @zed-industries/claude-code-acp
which claude-code-acp  # Should return path
```

**If FALSE:** Update installation docs and path resolution logic

---

**Assumption:** Tool approvals/permissions are handled by claude-code-acp internally

**Status:** ⚠️ **UNVERIFIED**

**Impact:** If FALSE, we need UI for approval flow in PWA

---

### 2. Git Worktree Concurrency

**Assumption:** Multiple ACP processes can safely work on different worktrees simultaneously

**Status:** ⚠️ **UNVERIFIED**

**Known risks:**
- Shared `.git/refs` directory - concurrent updates may conflict
- Shared `.git/index` - lock file contention
- Shared `.gitignore` - all worktrees use same ignore rules

**Mitigation (if problems occur):**
- Serialize agent operations (queue)
- Use git worktree locking
- Monitor for `.git/index.lock` conflicts

---

**Assumption:** Worktrees can start empty (no base commit needed)

**Status:** ⚠️ **DECISION NEEDED**

**Options:**
- Start empty (agents create files from scratch)
- Initialize with base files (README, .gitignore, etc.)

**Decision:** Start empty for Phase 1, add base template in Phase 2 if needed

---

### 3. WebSocket Protocol Sufficiency

**Assumption:** WebSocket is sufficient for PWA ↔ Relay communication

**Status:** ✅ **ACCEPTABLE FOR PHASE 1**

**Limitations:**
- No automatic reconnection (must refresh page)
- No offline support
- No request/response correlation built-in

**Future:** Consider SSE for one-way updates, keep WS for bidirectional

---

### 4. Session Lifecycle

**Assumption:** Sessions are ephemeral (no persistence across relay restarts)

**Status:** ✅ **ACCEPTABLE FOR PHASE 1**

**Implications:**
- Relay restart = all sessions lost
- No reconnection to existing sessions
- Conversation history lost on disconnect

**Future:** Add SQLite event store (Phase 4)

---

### 5. Agent Roles

**Assumption:** Agent roles (auth, db, tests) are meaningful for user context

**Status:** ✅ **ACCEPTABLE FOR PHASE 1**

**Reality:** Roles are just labels. Agents can do any work regardless of role name.

**Future:** Make agent count and roles configurable via UI

---

### 6. ACP JSON-RPC Protocol

**Assumption:** ACP uses standard JSON-RPC 2.0 over stdio

**Status:** ✅ **VERIFIED** (from npm package documentation)

**Example:**
```json
// Request (stdin)
{"jsonrpc":"2.0","id":1,"method":"agent/sendMessage","params":{"content":"hello"}}

// Response (stdout)
{"jsonrpc":"2.0","id":1,"result":{"type":"text","content":"Hello!"}}
```

---

## Open Questions

### Must Answer Before Implementation

1. **Does claude-code-acp require a config file?**
   - Check: `claude-code-acp --help`
   - Look for: --config, --session, --state flags

2. **How does claude-code-acp handle tool permissions?**
   - Does it prompt on stdio?
   - Does it use a permissions file?
   - Can we pre-approve all tools?

3. **What's the timeout for ACP responses?**
   - Long operations (installing packages) may take minutes
   - Need to set realistic timeout (5 minutes?)

4. **Does ACP maintain session state across restarts?**
   - If yes, where is it stored?
   - Do we need to clean up state files?

5. **How do we detect ACP process health?**
   - Ping/pong in JSON-RPC?
   - Read stderr for errors?
   - Process exit code?

---

## Testing Checklist

Before starting Issue #3, validate these assumptions:

- [ ] Install claude-code-acp globally
- [ ] Test spawning with --workspace flag
- [ ] Send test message via stdin
- [ ] Verify response on stdout
- [ ] Check if git commit was created
- [ ] Test with multiple concurrent processes
- [ ] Check for .git/index.lock conflicts
- [ ] Verify ANTHROPIC_API_KEY is respected
- [ ] Test error scenarios (invalid JSON, bad key, etc.)

---

## Assumptions Registry

| ID | Assumption | Status | Risk | Mitigation |
|----|-----------|--------|------|----------|
| A1 | ACP auto-commits | ⚠️ Unverified | High | Add manual git commit logic |
| A2 | Global npm install | ⚠️ Unverified | Low | Document alternative paths |
| A3 | Tool approval handling | ⚠️ Unverified | Medium | Add approval UI if needed |
| A4 | Worktree concurrency safe | ⚠️ Unverified | Medium | Add locking mechanism |
| A5 | Empty worktree OK | ⚠️ Decision needed | Low | Start empty, adjust later |
| A6 | WebSocket sufficient | ✅ Acceptable | Low | Document limitations |
| A7 | Ephemeral sessions OK | ✅ Acceptable | Low | Phase 1 only |
| A8 | Agent roles meaningful | ✅ Acceptable | Low | Make configurable later |
| A9 | JSON-RPC 2.0 format | ✅ Verified | Low | None |

---

## When Assumptions Fail

**If A1 fails (no auto-commit):**
- Add git commit logic to relay after each agent response
- Issue to create: "Add git commit automation"

**If A3 fails (need approval UI):**
- Add tool approval modal to PWA
- Issue to create: "PWA tool approval interface"

**If A4 fails (worktree conflicts):**
- Implement operation queue (serialize agent work)
- Issue to create: "Add agent operation queue"

**Always:** Document actual behavior in this file after testing
