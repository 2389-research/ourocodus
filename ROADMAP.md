# Ourocodus Development Roadmap

**Last Updated:** 2025-10-29

---

## Timeline Overview

```
┌─────────────────┬─────────────────┬─────────────────┬─────────────────┐
│   Milestone 1   │   Milestone 2   │   Milestone 3   │   Milestone 4   │
│   (COMPLETE)    │  (4-6 weeks)    │  (8-10 weeks)   │  (6-8 weeks)    │
└─────────────────┴─────────────────┴─────────────────┴─────────────────┘
  Oct 1 - Oct 29    Nov 1 - Dec 13    Dec 14 - Feb 28   Mar 1 - Apr 30
```

---

## Milestone 1: Foundation ✅ COMPLETE

**Status:** 9/13 issues closed (70% complete)
**Timeline:** Oct 1 - Oct 29, 2025
**GitHub:** [Milestone 1](https://github.com/2389-research/ourocodus/milestone/1)

### Completed Deliverables
- [x] Project initialization and tooling
- [x] Git worktree management for isolated agent workspaces
- [x] ACP client package with comprehensive tests
- [x] Relay WebSocket server
- [x] Session management (in-memory)
- [x] ACP process spawning and integration
- [x] Bidirectional message routing (WebSocket ↔ ACP)
- [x] Integration and smoke test suites

### Key Achievements
- **Backend infrastructure production-ready**
- **Can spawn N concurrent Claude Code agents**
- **Full test coverage (unit + integration + smoke)**
- **Demo scripts working (interactive REPL + automated)**

### In Progress (moving to Milestone 2)
- [ ] #10 - PWA WebSocket connection
- [ ] #11 - PWA agent cards UI
- [ ] #12 - PWA chat interface
- [ ] #13 - End-to-end integration tests

---

## Milestone 2: Complete + Prepare to Scale 🚧 CURRENT

**Status:** 0/9 issues complete (just started)
**Timeline:** Nov 1 - Dec 13, 2025 (6 weeks)
**GitHub:** [Milestone 2](https://github.com/2389-research/ourocodus/milestone/4)

### Track A: Phase 1 Completion (User-Facing)
**Goal:** Working PWA demo
**Issues:** #10, #11, #12, #13
**Estimate:** 3-4 weeks

Deliverables:
- [ ] Browser-based UI for spawning agents
- [ ] Chat interface for each agent
- [ ] Real-time agent state updates
- [ ] E2E automated tests

**Demo capability:** Users can open browser, spawn 3 agents, chat with each independently

### Track B: NATS Foundation (Technical)
**Goal:** Prove horizontal scaling
**Issues:** #35, #37, #38, #39, #40
**Estimate:** 4-5 weeks

Deliverables:
- [ ] NATS server configured with JetStream
- [ ] NATS client package (Go)
- [ ] Relay publishes events to NATS (non-breaking)
- [ ] Event logger service (observability)
- [ ] Load balancing proof with 2+ relay instances

**Technical proof:** System can scale beyond single relay instance

### Why This Matters
- **User value:** Working demo for stakeholders/investors
- **Technical value:** De-risks Phase 2 architecture
- **Strategic value:** Shows we're thinking ahead

---

## Milestone 3: Production Architecture 📅 PLANNED

**Status:** Not started
**Timeline:** Dec 14, 2025 - Feb 28, 2026 (10 weeks)
**GitHub:** TBD

### Phase 2: NATS Migration
**Goal:** Full migration to NATS message bus

Components:
- [ ] Coordinator service (workflow orchestration)
- [ ] API Server (decouple PWA from relay)
- [ ] Full NATS integration (all communication via message bus)
- [ ] Approval gate service
- [ ] Workflow automation (graph-based task dependencies)

### Production Features
**Goal:** Production-ready system

Features:
- [ ] Container isolation for agents (Docker/Podman)
- [ ] SQLite event store (persistence)
- [ ] Monitoring and observability (Prometheus)
- [ ] Error recovery and replay
- [ ] Authentication (JWT)
- [ ] Rate limiting

### Technical Debt
- [ ] Refactor session management (event sourcing)
- [ ] Add graceful shutdown logic
- [ ] Improve error handling consistency
- [ ] Performance optimization

---

## Milestone 4: Advanced Features 📅 PLANNED

**Status:** Not started
**Timeline:** Mar 1 - Apr 30, 2026 (8 weeks)
**GitHub:** TBD

### Multi-Project Support
- [ ] Multiple concurrent projects
- [ ] Project isolation
- [ ] Project templates

### Collaboration Features
- [ ] Real-time collaboration (multiple users)
- [ ] Agent sharing across users
- [ ] Approval gate UI (review agent work)

### Advanced Orchestration
- [ ] Dynamic workflow generation
- [ ] PRD → task graph automation
- [ ] Merge conflict resolution
- [ ] Git PR automation

### Scalability
- [ ] NATS clustering (HA)
- [ ] Relay horizontal scaling (production)
- [ ] Database migration (SQLite → PostgreSQL)
- [ ] CDN for PWA assets

---

## Success Metrics by Milestone

### Milestone 2 (Dec 13, 2025)
- [ ] Can demo PWA → 3 agents working concurrently
- [ ] E2E tests passing in CI
- [ ] 2+ relay instances load-balancing proven
- [ ] Event logger capturing all events
- [ ] NATS migration path documented

### Milestone 3 (Feb 28, 2026)
- [ ] Coordinator orchestrates full coding workflow
- [ ] Approval gates working (user reviews agent work)
- [ ] Zero message loss during normal operation
- [ ] Graceful degradation during failures
- [ ] Prometheus dashboards show healthy metrics

### Milestone 4 (Apr 30, 2026)
- [ ] 10+ concurrent users supported
- [ ] Agent work auto-merged to main branch
- [ ] PRD → working code in <2 hours
- [ ] 99.9% uptime
- [ ] Complete documentation for self-hosting

---

## Architecture Evolution

### Phase 1 (Current - Milestone 1-2)
```
PWA → WebSocket → Relay → stdio → N× Claude Code processes
```
- Direct connections
- In-memory state
- Manual orchestration

### Phase 2 (Milestone 3)
```
PWA → WebSocket → API Server → NATS → Relay → N× Claude Code containers
                                 ↓
                            Coordinator
```
- Message bus for all communication
- Autonomous workflow orchestration
- Horizontal scaling

### Phase 3 (Milestone 4)
```
PWA (CDN) → API Server (HA) → NATS Cluster → Relay Pool → Agent Containers
                                ↓
                           Coordinator + Event Store (PostgreSQL)
```
- High availability
- Multi-tenant
- Full production deployment

---

## Decision Points

### After Milestone 2 (Dec 2025)
**Question:** Continue with NATS migration or pivot?
**Decide based on:**
- Did NATS proof-of-concept succeed?
- Is horizontal scaling necessary yet?
- User feedback on PWA demo

**Options:**
- A) Proceed to Milestone 3 (full NATS migration)
- B) Add more Phase 1 features (defer scaling)
- C) Focus on user acquisition (marketing)

### After Milestone 3 (Feb 2026)
**Question:** Self-hosted vs managed service?
**Decide based on:**
- User demand for self-hosting
- Operational complexity
- Revenue model

**Options:**
- A) Open-source self-hosted (docs + Docker Compose)
- B) Managed SaaS offering
- C) Hybrid (free self-hosted + paid managed)

---

## Dependencies and Risks

### External Dependencies
- **Anthropic API:** Claude Code requires API key
  - Risk: Rate limits, API changes
  - Mitigation: Support multiple AI providers
- **NATS Server:** Phase 2+ requires NATS
  - Risk: Operational complexity
  - Mitigation: Comprehensive NATS.md documentation
- **Browser Support:** PWA requires modern browsers
  - Risk: Legacy browser support
  - Mitigation: Document browser requirements

### Technical Risks
1. **Message Ordering (NATS)**
   - Risk: Out-of-order messages break workflows
   - Mitigation: JetStream with per-session consumers
2. **Agent Resource Usage**
   - Risk: Agents consume too much memory/CPU
   - Mitigation: Container limits, monitoring
3. **Git Merge Conflicts**
   - Risk: Concurrent agents create conflicts
   - Mitigation: Git worktrees + automated resolution

### Resource Risks
1. **Development Capacity**
   - Risk: Single developer bottleneck
   - Mitigation: Clear documentation, modular design
2. **Funding**
   - Risk: Running out of runway before revenue
   - Mitigation: Focus on MVP, defer nice-to-haves

---

## Key Documents

- **[ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md)** - Phase 1 vs long-term architecture
- **[NATS.md](docs/architecture/NATS.md)** - Comprehensive NATS integration plan
- **[ISSUES.md](docs/history/ISSUES.md)** - Historical issue dependency graph (archived)
- **[Milestone History](docs/history/README.md)** - Completed milestone implementation logs

---

## Contact

**Questions about roadmap?** Open a GitHub issue with label `question`

**Want to contribute?** See [CONTRIBUTING.md](CONTRIBUTING.md)

**Need technical clarification?** See architecture docs linked above

---

**Maintained by:** Ourocodus Core Team
**Roadmap Version:** 1.0
**Next Review:** After Milestone 2 completion (Dec 2025)
