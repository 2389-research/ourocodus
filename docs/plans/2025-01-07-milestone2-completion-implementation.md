# Milestone 2 Completion - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete Milestone 2 by documenting static file serving approach and agent runtime architecture with MVP depth.

**Architecture:** Single PR with three documentation files using Mermaid diagrams for visual clarity. Focuses on internal developer audience with clear session type definitions, execution flow, and troubleshooting.

**Tech Stack:** Markdown, Mermaid diagrams, Git

---

## Task 1: Create Branch and Setup

**Files:**
- Create branch: `docs/milestone-2-completion`

**Step 1: Create feature branch**

```bash
git checkout -b docs/milestone-2-completion
```

**Step 2: Verify clean working directory**

Run: `git status`
Expected: "On branch docs/milestone-2-completion" with no uncommitted changes

**Step 3: Commit checkpoint**

Already on branch, ready to proceed.

---

## Task 2: Create Static File Serving Documentation

**Files:**
- Create: `docs/STATIC_FILE_SERVING.md`

**Step 1: Create STATIC_FILE_SERVING.md**

```markdown
# Static File Serving

## Current Approach

Ourocodus uses Go's standard `http.FileServer` to serve static files for the PWA frontend.

**Implementation:** `cmd/relay/main.go`
- Serves files from `web/` directory
- Simple, zero-dependency solution
- Suitable for development and small-scale deployments

**Characteristics:**
- No caching headers (browser default caching)
- No compression (relies on reverse proxy if needed)
- Single-instance serving (no load balancing built-in)

## When to Scale

The current approach is sufficient for:
- Development environments
- Internal tools with <100 concurrent users
- Single-instance deployments

## Future Scalability Options

When traffic increases or global distribution is needed:

### 1. CDN (Recommended for production)
- Serve static assets from CloudFlare, Fastly, or AWS CloudFront
- Keep relay for WebSocket connections only
- Benefits: global distribution, caching, DDoS protection

### 2. Separate Static Server
- nginx or Apache for static files
- Relay handles only WebSocket and API
- Benefits: better caching control, compression, tuned configs

### 3. Object Storage
- S3, GCS, or similar with public access
- Direct browser access to assets
- Benefits: infinite scalability, no server load

## Decision Timeline

- **Now:** http.FileServer is appropriate
- **Milestone 3-4:** Evaluate if traffic patterns require change
- **Production:** Implement CDN before public launch

## References

- Implementation: `cmd/relay/main.go`
- Related discussion: Issue #49
```

**Step 2: Verify file created**

Run: `cat docs/STATIC_FILE_SERVING.md | head -5`
Expected: First 5 lines show "# Static File Serving"

**Step 3: Commit static file serving docs**

```bash
git add docs/STATIC_FILE_SERVING.md
git commit -m "docs: Add static file serving documentation

Documents current http.FileServer approach and future scalability options.

Closes #49"
```

---

## Task 3: Create Agent Runtime Documentation (Part 1 - Structure and Definitions)

**Files:**
- Create: `docs/AGENT_RUNTIME.md`

**Step 1: Create AGENT_RUNTIME.md with header and definitions**

```markdown
# Agent Runtime Architecture

## Overview

The agent runtime layer manages the lifecycle and execution environment for AI coding agents. This layer provides isolation, resource management, and workspace coordination through Docker containers.

**Key Components:**
- Session Manager: Tracks UserSessions and spawns AgentSessions
- ContainerSession Manager: Creates and manages Docker containers for agent execution
- Agent Launcher: Coordinates agent spawning and ACP protocol initialization

## Session Type Definitions

These are the authoritative definitions for session types in Ourocodus:

### UserSession

**Definition:** A WebSocket connection from the PWA (browser) to the Relay server.

**Characteristics:**
- Represents a single user's connection to the system
- Contains 0 to N AgentSessions
- Lifecycle: Established on WebSocket connect, terminated on disconnect
- Tracked by: `pkg/relay/session.Store`

**Example:**
- User opens PWA in browser → UserSession created
- User spawns 3 agents → UserSession contains 3 AgentSessions
- User closes browser → UserSession terminated, all AgentSessions cleaned up

### AgentSession

**Definition:** An individual AI agent process with its own workspace and execution state.

**Characteristics:**
- One per agent spawned by the user
- Has dedicated workspace directory (git worktree)
- Communicates via ACP (Agent Client Protocol)
- Runs inside a ContainerSession for isolation
- Tracked by: `pkg/relay/session.Manager`

**Lifecycle:**
1. User requests agent spawn
2. AgentSession created with unique ID
3. ContainerSession created for execution environment
4. Agent process launched inside container
5. Agent processes tasks via ACP messages
6. User terminates agent → AgentSession and ContainerSession cleaned up

### ContainerSession

**Definition:** A Docker container managed by `pkg/containersession` that provides the runtime environment for an AgentSession.

**Characteristics:**
- **Lifecycle Policy:** One ContainerSession per AgentSession (1:1 mapping)
- Provides process isolation and resource management
- Mounts workspace directory from host
- Receives credentials via environment variables
- Automatically cleaned up when AgentSession terminates
- Managed by: `pkg/containersession.Manager`

**Container Configuration:**
- Image: Configurable (typically contains Claude Code ACP binary)
- Workspace: Bind-mounted from host filesystem
- Network: Default Docker bridge network
- Credentials: Injected via environment variables
- Lifecycle: Created on agent spawn, destroyed on agent termination
```

**Step 2: Verify definitions section**

Run: `grep -A 5 "## Session Type Definitions" docs/AGENT_RUNTIME.md`
Expected: Shows the section header and following lines

**Step 3: Commit definitions section**

```bash
git add docs/AGENT_RUNTIME.md
git commit -m "docs: Add agent runtime session type definitions

Defines UserSession, AgentSession, and ContainerSession with authoritative
descriptions and lifecycle policies.

Part of #110"
```

---

## Task 4: Add Component Architecture Diagram

**Files:**
- Modify: `docs/AGENT_RUNTIME.md`

**Step 1: Add component diagram section**

Append to `docs/AGENT_RUNTIME.md`:

```markdown

## Component Architecture

The following diagram shows the runtime layer components and their relationships:

```mermaid
graph TB
    subgraph "Browser"
        PWA[PWA Frontend]
    end

    subgraph "Relay Server"
        WS[WebSocket Handler]
        SM[Session Manager]
        AL[Agent Launcher]
    end

    subgraph "Container Runtime"
        CM[ContainerSession Manager]
        Docker[Docker Engine]
    end

    subgraph "Agent Execution"
        Agent[Claude Code ACP]
        Workspace[Git Worktree]
    end

    PWA -->|WebSocket| WS
    WS --> SM
    SM --> AL
    AL -->|uses| CM
    CM -->|creates/manages| Docker
    Docker -->|runs| Agent
    Agent -->|reads/writes| Workspace
```

**Components Explained:**

- **PWA Frontend:** Browser-based user interface for interacting with agents
- **WebSocket Handler:** Manages UserSession connections and message routing
- **Session Manager:** Tracks active UserSessions and their AgentSessions
- **Agent Launcher:** Coordinates agent spawning and initialization
- **ContainerSession Manager:** Creates and manages Docker containers (`pkg/containersession`)
- **Docker Engine:** Provides container runtime and isolation
- **Claude Code ACP:** AI agent process running inside container
- **Git Worktree:** Isolated workspace directory for agent operations

**Key Interactions:**

1. PWA establishes WebSocket connection → UserSession created
2. User requests agent spawn → Session Manager coordinates with Agent Launcher
3. Agent Launcher uses ContainerSession Manager to create container
4. Container runs Claude Code ACP process with mounted workspace
5. Agent communicates back through Relay to PWA
```

**Step 2: Verify Mermaid diagram syntax**

Run: `grep -A 20 "graph TB" docs/AGENT_RUNTIME.md`
Expected: Shows complete Mermaid diagram with proper syntax

**Step 3: Commit component diagram**

```bash
git add docs/AGENT_RUNTIME.md
git commit -m "docs: Add agent runtime component diagram

Mermaid diagram showing runtime layer components and their relationships.

Part of #110"
```

---

## Task 5: Add Execution Flow Sequence Diagram

**Files:**
- Modify: `docs/AGENT_RUNTIME.md`

**Step 1: Add sequence diagram section**

Append to `docs/AGENT_RUNTIME.md`:

```markdown

## Execution Flow

The following sequence diagram shows the complete lifecycle of an agent session from spawn to termination:

```mermaid
sequenceDiagram
    participant PWA as PWA (Browser)
    participant Relay as Relay Server
    participant CSM as ContainerSession Manager
    participant Docker as Docker Engine
    participant Agent as Claude Code Agent

    PWA->>Relay: WebSocket Connect (UserSession)
    Relay->>PWA: Connected

    PWA->>Relay: Spawn Agent Request
    Relay->>CSM: CreateContainerSession(config)
    CSM->>Docker: Create Container with Env (ANTHROPIC_API_KEY)
    Docker-->>CSM: Container ID
    CSM->>Docker: Start Container
    Docker->>Agent: Launch ACP Process
    CSM-->>Relay: ContainerSession + AgentSession
    Relay->>Agent: ACP Initialize
    Agent-->>Relay: Ready
    Relay-->>PWA: Agent Ready

    PWA->>Relay: Task Request
    Relay->>Agent: ACP Task Message
    Agent->>Agent: Execute in Workspace
    Agent-->>Relay: Task Result
    Relay-->>PWA: Display Result

    PWA->>Relay: Terminate Agent
    Relay->>CSM: Stop(sessionID)
    CSM->>Docker: Stop & Remove Container
    Docker-->>CSM: Stopped
    CSM-->>Relay: Terminated
    Relay-->>PWA: Agent Terminated
```

**Flow Phases:**

### 1. Connection Phase
- User opens PWA → WebSocket connection established
- UserSession created and tracked by Relay

### 2. Agent Spawn Phase
- User requests agent → Relay creates AgentSession
- ContainerSession Manager creates Docker container with configuration
- Container receives ANTHROPIC_API_KEY via environment variables
- Claude Code ACP process launches inside container
- ACP initialization completes → Agent reports ready

### 3. Task Execution Phase
- User sends task via PWA
- Relay routes ACP message to agent
- Agent executes task in mounted workspace
- Results returned through Relay to PWA

### 4. Termination Phase
- User terminates agent (or UserSession disconnects)
- ContainerSession Manager stops and removes container
- AgentSession cleaned up from Session Manager
- Workspace persists on host filesystem
```

**Step 2: Verify sequence diagram**

Run: `grep -A 30 "sequenceDiagram" docs/AGENT_RUNTIME.md`
Expected: Shows complete sequence diagram with all participants

**Step 3: Commit sequence diagram**

```bash
git add docs/AGENT_RUNTIME.md
git commit -m "docs: Add agent runtime execution flow diagram

Sequence diagram showing complete lifecycle from spawn to termination.

Part of #110"
```

---

## Task 6: Add Configuration and Credentials Documentation

**Files:**
- Modify: `docs/AGENT_RUNTIME.md`

**Step 1: Add configuration section**

Append to `docs/AGENT_RUNTIME.md`:

```markdown

## Configuration & Credentials

### Environment Variables

The following environment variables control agent runtime behavior:

| Variable | Required | Purpose | Example |
|----------|----------|---------|---------|
| `ANTHROPIC_API_KEY` | Yes | API key for Claude Code agents | `sk-ant-api03-...` |
| `OUROCODUS_ACP_BINARY` | No | Custom ACP binary path (for testing) | `/path/to/echo-agent` |
| `DOCKER_HOST` | No | Docker daemon connection | `unix:///var/run/docker.sock` |

**Setting Environment Variables:**

```bash
# macOS/Linux
export ANTHROPIC_API_KEY="sk-ant-api03-..."
./cmd/relay/relay

# Or inline
ANTHROPIC_API_KEY="sk-ant-..." ./cmd/relay/relay
```

### Credential Injection Flow

Credentials are injected into containers through the following process:

1. **Host Environment:** Relay reads `ANTHROPIC_API_KEY` from its environment
   - Source: `pkg/relay/session/client_factory.go:NewACPClientFactory()`
   - Validation: Returns `ErrMissingAnthropicAPIKey` if not set

2. **Container Configuration:** ContainerSession Manager receives config with `Env` field
   - Source: `pkg/containersession/config.go:CreateConfig.Env`
   - Format: `[]string{"ANTHROPIC_API_KEY=sk-ant-..."}`

3. **Container Creation:** Docker creates container with environment variables
   - Source: `pkg/containersession/manager.go:CreateContainerSessionWithConfig()`
   - Container receives full environment including credentials

4. **Agent Access:** Agent process inside container accesses credentials via environment
   - Source: `pkg/acp/client.go:NewClient()` sets `cmd.Env`
   - Agent uses credentials to authenticate with Anthropic API

**Diagram:**

```
Host Environment (ANTHROPIC_API_KEY)
    ↓
Relay reads on startup
    ↓
ContainerSession Manager receives in CreateConfig
    ↓
Docker container created with Env variables
    ↓
Agent process accesses via os.Getenv()
```

### Current Limitations

**Explicitly documented for MVP:**

- **Single Credential:** One `ANTHROPIC_API_KEY` per relay instance (global scope)
- **No Per-User Scoping:** All agents use the same credential
- **No Per-Project Scoping:** Cannot configure different keys for different projects
- **No Rotation Support:** Changing credentials requires relay restart
- **Single Provider:** Only Anthropic/Claude supported currently

**Future Enhancements:**

- Mount credential files via `CustomMounts` for multi-provider support
- Per-user credential scoping via UserSession metadata
- Credential rotation via hot-reload mechanism
- Support for OpenAI, Hugging Face, and other providers

### Container Configuration

**Image:**
- Configurable via `CreateConfig.ImageName`
- Must contain ACP-compatible agent binary
- Example: Custom image with Claude Code ACP pre-installed

**Workspace:**
- Bind-mounted from host via `CustomMounts`
- Typically a git worktree for isolated development
- Persists on host after container termination
- Agent has read/write access

**Network:**
- Default Docker bridge network
- Allows outbound connections to Anthropic API
- No inbound connections required (I/O via Relay)

**Resource Limits:**
- **Status:** Not enforced yet
- **Future:** CPU and memory limits via `CreateConfig`
- **Current:** Containers use host default limits

### Code References

- Session Factory: `pkg/relay/session/client_factory.go`
- ContainerSession Config: `pkg/containersession/config.go`
- Container Manager: `pkg/containersession/manager.go`
- ACP Client: `pkg/acp/client.go`
```

**Step 2: Verify configuration table**

Run: `grep -A 10 "| Variable | Required |" docs/AGENT_RUNTIME.md`
Expected: Shows environment variables table with all three variables

**Step 3: Commit configuration section**

```bash
git add docs/AGENT_RUNTIME.md
git commit -m "docs: Add agent runtime configuration and credentials

Documents environment variables, credential injection flow, current limitations,
and container configuration options.

Part of #110"
```

---

## Task 7: Add Troubleshooting Guide

**Files:**
- Modify: `docs/AGENT_RUNTIME.md`

**Step 1: Add troubleshooting section**

Append to `docs/AGENT_RUNTIME.md`:

```markdown

## Troubleshooting

Common issues when working with the agent runtime and their solutions.

### 1. Missing API Key

**Symptom:**
```
Error: ANTHROPIC_API_KEY environment variable not set
```

**Cause:** The relay server cannot find the required API key in its environment.

**Solution:**

```bash
# Set the environment variable before starting relay
export ANTHROPIC_API_KEY="sk-ant-api03-..."
./cmd/relay/relay

# Verify it's set
echo $ANTHROPIC_API_KEY
```

**Prevention:** Add to your shell profile (`.bashrc`, `.zshrc`) for persistence:
```bash
echo 'export ANTHROPIC_API_KEY="sk-ant-..."' >> ~/.zshrc
```

---

### 2. Docker Connection Failed

**Symptom:**
```
Error: Cannot connect to Docker daemon at unix:///var/run/docker.sock
```

**Cause:** Docker daemon is not running or not accessible.

**Solution by Platform:**

**macOS (Docker Desktop):**
```bash
# Check if Docker is running
docker ps

# If not running, start Docker Desktop application
open -a Docker

# Wait for Docker to start, then verify
docker ps
```

**macOS (Colima):**
```bash
# Check Colima status
colima status

# Start if not running
colima start

# Verify connection
docker ps
```

**Linux:**
```bash
# Check if Docker daemon is running
sudo systemctl status docker

# Start if not running
sudo systemctl start docker

# Ensure your user is in docker group
sudo usermod -aG docker $USER
# Log out and back in for group change to take effect

# Verify connection (no sudo needed after group membership)
docker ps
```

**Custom DOCKER_HOST:**
```bash
# If using custom Docker socket location
export DOCKER_HOST="unix:///path/to/custom/docker.sock"
```

---

### 3. Container Image Pull Failure

**Symptom:**
```
Error: failed to pull image "ourocodus/agent:latest": pull access denied
```

**Cause:** Cannot pull the Docker image from registry.

**Solutions:**

**Check Internet Connection:**
```bash
# Test connectivity
ping docker.io
```

**Verify Image Name:**
```bash
# Test manual pull
docker pull ourocodus/agent:latest

# Check available images
docker images
```

**Docker Hub Rate Limits:**
```bash
# If hitting rate limits, authenticate
docker login

# Or use a mirror registry
```

**Image Doesn't Exist:**
```bash
# Build image locally if not in registry
cd /path/to/ourocodus
docker build -t ourocodus/agent:latest -f containers/agent/Dockerfile .
```

**Firewall/Proxy Issues:**
```bash
# Configure Docker to use proxy if behind corporate firewall
# Edit /etc/docker/daemon.json or Docker Desktop settings
```

---

### 4. Permission Denied on Workspace

**Symptom:**
```
Error: permission denied: /workspace/file.txt
```

**Cause:** Container user lacks permissions to access mounted workspace.

**Solution:**

**Check Mount Permissions:**
```bash
# Verify workspace directory is accessible
ls -la /path/to/workspace

# Ensure directory is readable and writable
chmod 755 /path/to/workspace
```

**Linux SELinux:**
```bash
# If using SELinux, add :z or :Z to mount options
# This is handled in pkg/containersession mount configuration
```

---

### 5. Agent Timeout or Unresponsive

**Symptom:**
- Agent spawn request hangs
- No response from agent after several minutes

**Cause:** Container startup issues or network problems.

**Diagnosis:**

```bash
# List running containers
docker ps

# Check specific container logs
docker logs <container-id>

# Inspect container
docker inspect <container-id>

# Check container resource usage
docker stats <container-id>
```

**Common Causes:**

1. **Image pull taking too long:** Wait for pull to complete or pull manually first
2. **Container startup script hanging:** Check container logs for errors
3. **Network issues:** Verify container can reach Anthropic API
4. **Resource exhaustion:** Check host CPU/memory availability

**Solution:**
```bash
# Stop hanging container
docker stop <container-id>

# Remove and retry
docker rm <container-id>

# Try spawning agent again through PWA
```

---

### Additional Diagnostics

**View Relay Logs:**
```bash
# Relay logs show session and agent lifecycle events
./cmd/relay/relay 2>&1 | tee relay.log
```

**Inspect ContainerSession State:**
```bash
# List all containers (including stopped)
docker ps -a

# Filter by ourocodus session labels
docker ps -a --filter "label=ourocodus.session.id"
```

**Test ACP Communication:**
```bash
# Use echo-agent for testing without API key
export OUROCODUS_ACP_BINARY="./cmd/echo-agent/echo-agent"
./cmd/relay/relay
```

### Getting Help

If issues persist:

1. Check GitHub Issues: [github.com/2389-research/ourocodus/issues](https://github.com/2389-research/ourocodus/issues)
2. Review relevant documentation:
   - `docs/ARCHITECTURE.md` - System overview
   - `docs/SESSION_LIFECYCLE.md` - Session management
   - `pkg/containersession/doc.go` - Container session details
3. Include diagnostic information when reporting issues:
   - Relay logs
   - Docker logs for container
   - `docker info` output
   - Operating system and Docker version
```

**Step 2: Verify troubleshooting scenarios**

Run: `grep -c "### [0-9]" docs/AGENT_RUNTIME.md`
Expected: Shows "5" (five troubleshooting scenarios)

**Step 3: Commit troubleshooting section**

```bash
git add docs/AGENT_RUNTIME.md
git commit -m "docs: Add agent runtime troubleshooting guide

Five common scenarios with diagnosis steps and solutions:
- Missing API key
- Docker connection failed
- Image pull failure
- Permission denied on workspace
- Agent timeout/unresponsive

Part of #110"
```

---

## Task 8: Add Future Scalability Section

**Files:**
- Modify: `docs/AGENT_RUNTIME.md`

**Step 1: Add scalability section**

Append to `docs/AGENT_RUNTIME.md`:

```markdown

## Future: Kubernetes Migration Path

### Current Architecture (Single Host)

- **Docker Engine:** Direct connection via Docker socket
- **Isolation:** Process isolation via Docker containers
- **Networking:** Docker bridge network
- **Storage:** Bind mounts from host filesystem
- **Lifecycle:** Managed by ContainerSession Manager

**Scaling Characteristics:**
- Scales to ~50-100 concurrent agents on single host (depending on resources)
- Simple deployment and debugging
- No distributed systems complexity
- Suitable for development and small-scale production

### Near-Term Scaling (Single Host)

The current architecture supports horizontal scaling on a single host:

- **More Containers:** Current design handles N containers efficiently
- **Resource Limits:** Add CPU/memory limits via `CreateConfig`
- **Container Pooling:** Reuse containers for sequential agent sessions (future)
- **Workspace Optimization:** Shared read-only base + per-agent overlay

**When to migrate:** When consistently running >50 concurrent agents or need multi-host.

### Long-Term: Kubernetes Migration

**Primitive Mappings:**

| Current | Kubernetes Equivalent | Notes |
|---------|----------------------|-------|
| ContainerSession | Job or Pod | Ephemeral workload for agent execution |
| Docker container | Container in Pod | Same isolation model |
| Environment variables (credentials) | Secrets | Native K8s secret management |
| Bind mount (workspace) | PersistentVolumeClaim | Durable storage across pod restarts |
| Docker bridge network | NetworkPolicy | Control traffic between pods |
| ContainerSession Manager | Kubernetes API | API-driven lifecycle management |

**Architecture Changes:**

```
Current:
Relay → ContainerSession Manager → Docker API → Container

Kubernetes:
Relay → Kubernetes API → Job/Pod → Container
```

**What Stays Identical:**
- Agent code (Claude Code ACP binary)
- ACP protocol and message flow
- Workspace structure and git worktrees
- Session type abstractions (UserSession, AgentSession, ContainerSession)
- Relay WebSocket handling

**What Changes:**
- `pkg/containersession` replaced with K8s client library
- Container lifecycle managed by K8s instead of direct Docker API
- Workspaces stored in PVCs instead of host bind mounts
- Credentials managed via K8s Secrets API
- Load balancing and scheduling via K8s scheduler

**Migration Complexity:** Low-to-Medium

- Clean abstraction boundaries make swap straightforward
- ContainerSession interface remains conceptually the same
- Main effort: implementing K8s client wrapper matching ContainerSession API

**When to Migrate:**

- **Don't migrate yet:** Current architecture sufficient for M2-M4
- **Consider:** When running >100 concurrent agents consistently
- **Migrate:** When need multi-host distribution or K8s-native deployment

### Kubernetes Example (Future Reference)

**Minimal Job Spec:**

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: agent-session-abc123
  labels:
    ourocodus.session.id: abc123
    ourocodus.user.id: user-xyz
spec:
  template:
    spec:
      containers:
      - name: agent
        image: ourocodus/agent:latest
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: anthropic-credentials
              key: api-key
        volumeMounts:
        - name: workspace
          mountPath: /workspace
      volumes:
      - name: workspace
        persistentVolumeClaim:
          claimName: agent-workspace-abc123
      restartPolicy: Never
```

**Benefits of K8s Migration:**

- Multi-host distribution for massive scale
- Native secret management and rotation
- Better resource utilization via scheduler
- High availability and fault tolerance
- Standard deployment tooling (Helm, Kustomize)

**Trade-offs:**

- Increased operational complexity
- Requires K8s cluster management
- More difficult local development setup
- Additional abstractions to understand

### Design Principle

The ContainerSession abstraction exists specifically to make this migration straightforward when needed. The current Docker-based implementation and future K8s implementation can share the same interface, minimizing changes to the relay and session management layers.
```

**Step 2: Verify scalability section**

Run: `grep -A 5 "## Future: Kubernetes Migration Path" docs/AGENT_RUNTIME.md`
Expected: Shows section header and opening paragraphs

**Step 3: Commit scalability section**

```bash
git add docs/AGENT_RUNTIME.md
git commit -m "docs: Add agent runtime Kubernetes migration path

Documents current single-host architecture, near-term scaling options,
and long-term K8s migration strategy with primitive mappings.

Part of #110"
```

---

## Task 9: Update ARCHITECTURE.md with Agent Runtime Layer

**Files:**
- Modify: `docs/ARCHITECTURE.md`

**Step 1: Read current ARCHITECTURE.md to find insertion point**

Run: `cat docs/ARCHITECTURE.md | grep -n "##" | head -10`
Expected: Shows section headers with line numbers

**Step 2: Add Agent Runtime Layer section**

After the main architecture overview (determine exact line based on Step 1), add:

```markdown
## Agent Runtime Layer

The agent runtime layer manages the lifecycle and execution environment for AI coding agents. This layer provides isolation, resource management, and workspace coordination.

**Key Components:**

- **Session Manager:** Tracks UserSessions (WebSocket connections) and spawns AgentSessions
- **ContainerSession Manager:** Creates and manages Docker containers for agent execution
- **Agent Launcher:** Coordinates agent spawning and ACP protocol initialization

**Session Types:**

- **UserSession:** WebSocket connection from PWA (browser) to Relay
- **AgentSession:** Individual AI agent process with workspace and state
- **ContainerSession:** Docker container runtime (one per AgentSession)

**Architecture Overview:**

```
PWA (Browser)
    ↓ WebSocket
Relay Server (Session Management)
    ↓ Docker API
ContainerSession Manager
    ↓ Container Lifecycle
Docker Engine
    ↓ Runs
Claude Code Agent (ACP)
    ↓ Reads/Writes
Git Worktree (Workspace)
```

**For Complete Details:** See [Agent Runtime Architecture](./AGENT_RUNTIME.md) for:
- Detailed session type definitions and lifecycle policies
- Component and sequence diagrams (Mermaid)
- Configuration and credential management
- Troubleshooting common container issues
- Future Kubernetes migration path

**Session Lifecycle:**

1. User connects via PWA → UserSession created
2. User spawns agent → AgentSession + ContainerSession created
3. Agent executes tasks in isolated container
4. User terminates agent → ContainerSession and AgentSession cleaned up
5. Workspace persists on host after termination

**Implementation:**

- Session tracking: `pkg/relay/session/`
- Container management: `pkg/containersession/`
- ACP client: `pkg/acp/`
```

**Step 3: Verify insertion**

Run: `grep -A 10 "## Agent Runtime Layer" docs/ARCHITECTURE.md`
Expected: Shows the new section with content

**Step 4: Commit ARCHITECTURE.md update**

```bash
git add docs/ARCHITECTURE.md
git commit -m "docs: Add Agent Runtime Layer to architecture overview

Links to detailed AGENT_RUNTIME.md documentation with session types,
component overview, and lifecycle summary.

Part of #110"
```

---

## Task 10: Final Verification and PR Creation

**Files:**
- All documentation files created

**Step 1: Verify all files exist**

```bash
ls -la docs/STATIC_FILE_SERVING.md
ls -la docs/AGENT_RUNTIME.md
git diff origin/main docs/ARCHITECTURE.md
```

Expected: All files exist and ARCHITECTURE.md shows changes

**Step 2: Test Mermaid diagrams in GitHub**

Create a test view to verify Mermaid rendering:

```bash
# Open GitHub's markdown preview for AGENT_RUNTIME.md
# Method 1: Push branch and view in GitHub UI
git push origin docs/milestone-2-completion

# Method 2: Use GitHub CLI to preview locally (if available)
gh markdown-preview docs/AGENT_RUNTIME.md
```

Expected: Diagrams render correctly (component diagram, sequence diagram visible)

**Step 3: Create Pull Request**

```bash
gh pr create \
  --title "docs: Complete Milestone 2 - Agent Runtime and Static Serving" \
  --body "$(cat <<'EOF'
## Summary

Completes Milestone 2 by documenting the remaining two open issues:
- **Issue #49:** Static file serving approach and future scalability options
- **Issue #110:** Agent runtime architecture (MVP documentation)

## Changes

### New Files

1. **`docs/STATIC_FILE_SERVING.md`**
   - Documents current http.FileServer approach
   - When current approach is sufficient
   - Future scalability options (CDN, separate server, object storage)
   - Decision timeline for scaling

2. **`docs/AGENT_RUNTIME.md`**
   - Authoritative session type definitions (UserSession, AgentSession, ContainerSession)
   - Component architecture diagram (Mermaid)
   - Execution flow sequence diagram (Mermaid)
   - Configuration and credential injection documentation
   - Current limitations explicitly documented
   - Troubleshooting guide (5 common scenarios)
   - Kubernetes migration path for future scaling

### Modified Files

3. **`docs/ARCHITECTURE.md`**
   - Added "Agent Runtime Layer" section
   - Links to detailed AGENT_RUNTIME.md
   - Session lifecycle summary

## Documentation Quality

- **Audience:** Internal developers
- **Diagrams:** Mermaid (renders natively in GitHub)
- **Scope:** MVP depth - establishes foundation for future expansion
- **Testing:** All Mermaid diagrams verified to render correctly

## Closes

- Closes #49
- Closes #110

## Follow-up Issues

After this PR merges, we should create follow-up issues for:
- Extended troubleshooting scenarios (10+ scenarios)
- Lifecycle state diagram for ContainerSession
- Detailed Kubernetes migration implementation guide
- Operational runbook (logs, metrics, debugging)
- Multi-provider credential design

## Review Notes

This is MVP documentation focused on closing the milestone. The content provides:
- Clear definitions for session types (eliminates ambiguity)
- Visual diagrams for architecture understanding
- Practical troubleshooting for common issues
- Forward-looking scalability notes

The foundation is solid for expanding documentation based on real usage patterns and team feedback.
EOF
)"
```

Expected: PR created successfully with PR number returned

**Step 4: Verify PR and close issues**

```bash
# View the PR
gh pr view

# After PR is merged, close the issues
gh issue close 49 --comment "Closed by PR #<number>"
gh issue close 110 --comment "Closed by PR #<number>"
```

---

## Task 11: Clean Up and Final Commit

**Step 1: Verify git status is clean**

Run: `git status`
Expected: "On branch docs/milestone-2-completion", "Your branch is up to date", no uncommitted changes

**Step 2: Review commit history**

```bash
git log --oneline origin/main..HEAD
```

Expected: Shows all commits from this implementation:
- docs: Add static file serving documentation
- docs: Add agent runtime session type definitions
- docs: Add agent runtime component diagram
- docs: Add agent runtime execution flow diagram
- docs: Add agent runtime configuration and credentials
- docs: Add agent runtime troubleshooting guide
- docs: Add agent runtime Kubernetes migration path
- docs: Add Agent Runtime Layer to architecture overview

**Step 3: Final summary**

All tasks complete. Ready for PR review and merge.

---

## Success Criteria

- [x] Issue #49 addressed with STATIC_FILE_SERVING.md
- [x] Issue #110 addressed with AGENT_RUNTIME.md (MVP scope)
- [x] ARCHITECTURE.md updated with Agent Runtime Layer reference
- [x] Two Mermaid diagrams render correctly in GitHub (component + sequence)
- [x] Session types authoritatively defined (UserSession, AgentSession, ContainerSession)
- [x] Credential injection flow documented
- [x] Five troubleshooting scenarios documented
- [x] Kubernetes migration path documented
- [x] All commits follow conventional commits format
- [x] PR created with clear description
- [x] Ready to close both issues after merge

## Estimated Time

- **Total:** ~3-4 hours
- **Task 1-2:** 30 min (branch setup, static file serving)
- **Task 3-5:** 90 min (agent runtime core content with diagrams)
- **Task 6-8:** 60 min (config, troubleshooting, scalability)
- **Task 9-11:** 30 min (architecture update, PR creation, verification)

## Notes for Executor

- This is documentation work, so "tests" are replaced with verification steps (file exists, diagrams render, etc.)
- Mermaid syntax is sensitive - verify diagrams render before committing
- Use GitHub preview or push branch to verify Mermaid rendering
- Keep commits small and focused (one section per commit for easy review)
- PR description is comprehensive - adjust as needed for your team's style
