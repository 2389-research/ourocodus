# Coordinator - Product Requirements Document

## Purpose

Orchestrates development workflows by reading graph definitions and driving agent work through sequential phases with approval gates. **Deferred for Phase 2** - Phase 1 focuses on infrastructure validation only.

## Responsibilities

1. Read workflow graph from YAML
2. Send work messages to agents via NATS
3. Wait for results from agents
4. Block on approval gates
5. Progress through workflow sequentially
6. Handle errors and timeouts

## Non-Responsibilities

- Agent lifecycle management (API server handles this)
- Message routing (Relay handles this)
- Protocol translation (Relay handles this)
- Graph execution parallelism (Phase 1 is sequential only)

## Technical Approach

**Language:** Go
**Framework:** Stdlib only
**Dependencies:** NATS client, YAML parser

## High-Level Flow

```
1. Load graph.yaml
2. For each chunk in graph:
   a. Send work.coding message to NATS
   b. Wait for result
   c. Request approval
   d. Wait for approval.granted
   e. Send work.testing message
   f. Wait for result
   g. Request approval
   h. Send work.review message
   i. Wait for result
   j. Mark chunk complete
3. Workflow complete
```

## Graph Definition (YAML)

```yaml
# config/graph.yaml
session:
  name: "my-project"
  repo: "https://github.com/user/repo"

chunks:
  - id: chunk_1
    name: "Authentication"
    description: "Implement user authentication"
    requirements:
      - "Use bcrypt for passwords"
      - "JWT tokens with 24h expiry"
    phases:
      - coding
      - testing
      - review
    depends_on: []

  - id: chunk_2
    name: "Database"
    description: "Set up database layer"
    requirements:
      - "Use PostgreSQL"
      - "Migrations with migrate tool"
    phases:
      - coding
      - testing
    depends_on: [chunk_1]
```

## Message Interaction

### Send Work
```
Topic: sessions.{session_id}.work.coding
Message: {
  "id": "msg_123",
  "type": "work.coding",
  "payload": {
    "chunk_id": "chunk_1",
    "description": "Implement user authentication",
    "requirements": [...]
  }
}
```

### Wait for Result
```
Topic: sessions.{session_id}.results.coding
Subscribe and block until message received
```

### Request Approval
```
Topic: sessions.{session_id}.approvals
Message: {
  "type": "approval.request",
  "payload": {
    "gate_id": "chunk_1_post_coding",
    "summary": "Coding complete. Proceed to testing?"
  }
}
```

### Wait for Approval
```
Topic: sessions.{session_id}.approvals
Block until approval.granted received
```

## Implementation (Deferred to Phase 2)

Coordinator will be implemented after relay infrastructure is validated in Phase 1.

**Phase 1:** Manual workflow via CLI
**Phase 2:** Automated coordinator

## Success Criteria (Phase 2)

1. Can parse graph.yaml
2. Sends work messages sequentially
3. Waits for results correctly
4. Approval gates block progression
5. Handles one complete workflow end-to-end
6. Logs all state transitions
