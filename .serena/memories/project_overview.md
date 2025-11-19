# Ourocodus Project Overview

## Purpose
Ourocodus is a multi-agent AI coding system that orchestrates Claude Code, OpenAI Codex, and other ACP-compatible agents to collaboratively build software. It enables users to spin up multiple AI coding agents that work concurrently on different aspects of the same codebase through isolated worktrees and containers.

## Core Concept
Users interact through a Progressive Web App (PWA), directing agents through isolated conversations while the system manages git worktrees and coordinates their work. Each agent works in its own isolated environment (git worktree + Docker container) to prevent conflicts.

## Current Status
**Phase 1 - Foundation Implementation** (Proof of Concept)
- Focus: Validate multi-agent communication and concurrent work with proper isolation
- No NATS coordinator yet (direct WebSocket + stdio)
- In-memory session state
- Manual user orchestration (no automated workflow)

## Architecture (Phase 1)
```
PWA (Browser) ←WebSocket→ Relay (Go) ←stdio→ N× Claude Code ACP processes
                                              ↓
                                         Git Worktrees + Docker Containers
```

## Key Features
- **Multi-agent concurrency**: Spawn N agents working on same codebase without conflicts
- **Isolation layers**: Git worktree + Docker container + credential isolation per agent
- **WebSocket communication**: Real-time bidirectional messaging
- **Dynamic agent IDs**: User chooses agent identifiers (not predefined types)
- **Independent lifecycles**: Agent failure doesn't terminate session

## Future Vision (Long-term)
Phase 3+ will add:
- NATS message bus for scalability
- Coordinator service for autonomous workflow automation
- Sequential or parallel execution based on dependency graph
- Approval gates at merge points
- Git merge automation
- Fault tolerance via event sourcing
