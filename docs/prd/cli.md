# CLI - Product Requirements Document

## Purpose

Command-line interface for interacting with Ourocodus system. Used for manual testing in Phase 1 and user control in later phases.

## Responsibilities

1. Start/stop system components
2. Send messages to agents (Phase 1 manual workflow)
3. View event logs
4. Query system status
5. Approve workflow gates

## Technical Approach

**Language:** Go
**Framework:** `cobra` for CLI structure
**Dependencies:** NATS client

## Phase 1 Commands (Foundation)

### System Management

#### Start System
```bash
ourocodus start
# Starts: NATS, Relay, API server
```

#### Stop System
```bash
ourocodus stop
```

#### Status
```bash
ourocodus status
# Output:
#   NATS:    running (localhost:4222)
#   Relay:   running (localhost:8080)
#   API:     running (localhost:9000)
#   Agents:  1 active
```

### Agent Management

#### Launch Agent
```bash
ourocodus agent start --session sess_123 --role coding
# Output: Agent agent_abc started in container docker_xyz
```

#### Stop Agent
```bash
ourocodus agent stop agent_abc
```

#### List Agents
```bash
ourocodus agent list
# Output:
#   agent_abc  sess_123  coding   running  2m ago
#   agent_def  sess_123  testing  running  1m ago
```

### Manual Workflow (Phase 1)

#### Send Work
```bash
ourocodus send work \
  --session sess_123 \
  --role coding \
  --task "Implement user authentication" \
  --requirements "Use bcrypt, JWT tokens"
```

#### Tail Results
```bash
ourocodus tail results --session sess_123
# Blocks and prints results as they arrive
```

### Observability

#### Tail Events
```bash
ourocodus logs --session sess_123 --follow
```

#### Show Session
```bash
ourocodus session show sess_123
```

## Phase 2 Commands (Automation)

### Workflow Management

#### Start Workflow
```bash
ourocodus run --graph config/graph.yaml
```

#### Approve Gate
```bash
ourocodus approve gate_123
```

#### Reject Gate
```bash
ourocodus reject gate_123 --reason "Missing tests"
```

## Configuration

```bash
# ~/.ourocodus/config.yaml
nats_url: nats://localhost:4222
api_url: http://localhost:9000
relay_url: ws://localhost:8080
```

## Implementation

```
cmd/cli/
  main.go
  cmd/
    start.go    # System management
    stop.go
    status.go
    agent.go    # Agent commands
    send.go     # Send messages
    tail.go     # Tail logs/results
    session.go  # Session management
```

## Success Criteria (Phase 1)

1. Can start/stop system
2. Can launch agent container
3. Can send work message via NATS
4. Can tail results
5. Can view logs
6. Can query status
