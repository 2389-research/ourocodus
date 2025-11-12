# Afternoon Work Plan - 2025-11-03 (Starting at Noon)

## Available Work Options from Milestone 2

### Option 1: NATS Foundation Track (Critical Path)
**Focus:** Set up NATS infrastructure and client package

**Issues:**
1. **#35: NATS Server Setup** (MEDIUM, no dependencies)
   - Create docker-compose.yml with NATS
   - Enable JetStream
   - Add make targets
   - Document setup

2. **#37: NATS Client Package** (HIGH, depends on #35 for integration tests)
   - Create pkg/nats/ package
   - Connection management with reconnection
   - Publish/subscribe helpers
   - Structured message types
   - Unit + integration tests

**Why this track:**
- HIGH priority item (#37)
- Unblocks other NATS work (#38, #39, #40)
- Foundation for Milestone 3
- Clear deliverables

**What you'll have by end of day:**
- NATS server running locally
- Client package with connection management
- Tests passing
- Foundation for relay integration

---

### Option 2: PWA Visual Track (User-Facing)
**Focus:** Build out the visual PWA interface

**Issues:**
1. **#11: PWA Agent Cards** (MEDIUM, depends on #10 which is done)
   - Display 3 agent cards (auth, db, tests)
   - Connection status
   - Clickable cards
   - Responsive layout

2. **#12: PWA Chat Interface** (MEDIUM, depends on #11)
   - Chat container per agent
   - Message input/display
   - WebSocket integration
   - Message history

**Why this track:**
- Tangible visual progress
- User-facing features
- Good for demos
- No backend dependencies (uses existing relay)

**What you'll have by end of day:**
- Visual agent cards in PWA
- Working chat interface
- Can send/receive messages to agents
- Looks like a real product

---

### Option 3: Container Session Management (New Work, M5)
**Focus:** Start building the container management foundation

**Issues:**
1. **#101: Phase 1 - Core Container Session Package**
   - Create pkg/containersession/ structure
   - Manager struct and NewManager()
   - CreateSession() with Docker SDK
   - StopSession() with graceful shutdown
   - ListSessions() with labels
   - Unit tests

**Why this track:**
- Fresh start on new component
- Solves the TTY issues we debugged
- Foundation for containerized agents
- High impact for agent reliability

**What you'll have by end of day:**
- pkg/containersession/ package working
- Can create/stop/list container sessions
- Unit tests passing
- Docker SDK integration complete

---

## Recommendation

**Start with Option 1 (NATS Foundation)** because:
1. **#37 is HIGH priority** - only high-priority item in M2
2. **Unblocks downstream work** - NATS is needed for M3
3. **Well-scoped** - NATS setup is straightforward, client package is clear
4. **Critical path** - M2 → M3 progression depends on this

**Alternative: Option 3 (Container Session)** if:
- Want to close out the containerization investigation
- Prefer to work on isolated component
- Want to build on fresh insights from PackNplay/ACP-Relay analysis

**Not recommended today: Option 2 (PWA)** because:
- Lower priority than NATS
- More UI work (slower feedback loop)
- Can be done anytime (not blocking anything)

---

## Today's Execution Plan (Option 1: NATS Foundation)

### Phase 1: NATS Server Setup (~1-2 hours)
- [ ] Create docker-compose.yml with NATS configuration
- [ ] Test NATS startup (`docker-compose up nats`)
- [ ] Add make targets (`make nats-start`, `make nats-stop`)
- [ ] Document in README
- [ ] Close #35

### Phase 2: NATS Client Package (~3-4 hours)
- [ ] Create pkg/nats/ package structure
- [ ] Implement Manager with connection logic
- [ ] Implement Publish() and Subscribe() methods
- [ ] Add correlation ID generation
- [ ] Write unit tests (mock NATS)
- [ ] Write integration tests (real NATS)
- [ ] Document API with examples
- [ ] Close #37

### Phase 3: Quick Win (if time)
- [ ] Start #38: Relay NATS Event Publisher (non-breaking)
- [ ] Or start #101: Container Session Management Phase 1

---

## Current Time: Noon
**Expected completion:** 5-6 PM (5-6 hours of work)
**Deliverables:** NATS foundation complete, unblocking M3 work
