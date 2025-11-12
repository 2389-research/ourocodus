# Ourocodus Terminology

This document defines the core entities, concepts, and naming conventions used throughout the Ourocodus codebase. Clear terminology is critical for understanding the architecture and relationships between components.

## Table of Contents

- [Core Entities](#core-entities)
- [Isolation Layers](#isolation-layers)
- [Infrastructure Components](#infrastructure-components)
- [Naming Conventions](#naming-conventions)
- [Relationship Summary](#relationship-summary)

## Core Entities

### User

A human developer or team member using the Ourocodus system. Users interact with the system through a Progressive Web Application (PWA) that connects to the relay server.

**Key Properties:**
- Has authentication credentials
- Can create multiple UserSessions
- Owns repositories and projects

### UserSession

A single WebSocket connection from a User's PWA to the relay server. Represents an active user interaction session.

**Key Properties:**
- Managed by the relay server
- Bidirectional communication channel (user ↔ relay)
- Can spawn multiple AgentSessions
- Lifetime tied to WebSocket connection

**Example:** When a user opens the Ourocodus PWA and connects, a UserSession is established.

### Agent

An autonomous AI-powered worker that performs coding tasks (e.g., implementing features, reviewing code, running tests). Agents are the "workers" in the Ourocodus system.

**Types of Agents:**
- **Coder Agent**: Implements features and writes code
- **Reviewer Agent**: Reviews code and provides feedback
- **Tester Agent**: Runs tests and validates changes
- **Custom Agents**: User-defined specialized agents

**Key Characteristics:**
- Has a specific role/purpose
- Operates autonomously within constraints
- Requires isolated workspace and execution environment

### AgentSession

A running instance of an Agent working on a specific task. Represents the complete execution environment for one agent's work.

**Key Properties:**
- Spawned by a UserSession
- Has unique identifier (agentID)
- Requires isolation from other AgentSessions
- Comprises: AgentWorktree + ContainerSession + Credentials
- Lifetime managed by AgentContainerLauncher

**Example:** When a user asks to "implement feature X", the relay spawns an AgentSession for a coder agent to work on that feature.

### Container

A Docker container providing the runtime environment for an agent. Containers provide process isolation, filesystem isolation, and resource constraints.

**Key Properties:**
- Runs agent code/tools
- Isolated filesystem namespace
- Network isolation (optional)
- Resource limits (CPU, memory)

### ContainerSession

A managed Docker container lifecycle for an AgentSession. Wraps a Container with lifecycle management and I/O handling.

**Managed by:** `pkg/containersession` package

**Key Properties:**
- Session ID for tracking
- State machine: PENDING → RUNNING → STOPPED/FAILED
- Workspace directory mounted at `/workspace`
- I/O streams for agent communication
- Labels for discovery and tracking

**Relationship:** Each AgentSession has exactly one ContainerSession.

## Isolation Layers

### AgentWorktree

An isolated git worktree with a unique branch for an AgentSession's code changes. Provides filesystem and git isolation so multiple agents can work on the same repository concurrently without conflicts.

**Managed by:** `pkg/worktree.AgentWorktreeManager`

**Key Properties:**
- Unique branch name: `agent-{agentID}-{timestamp}`
- Isolated working directory: `/workspaces/agent-{agentID}/`
- Git operations scoped to worktree branch
- Cleanup removes both worktree and branch

**Benefits:**
- Multiple agents can work on same repository simultaneously
- Changes are isolated to agent's branch
- No merge conflicts during development
- Clean separation for code review

**Example Structure:**
```
/workspaces/
├── agent-coder-abc123/          (AgentWorktree for coder agent)
│   ├── .git                     (git metadata, linked to main repo)
│   ├── src/                     (code being modified)
│   └── ...
└── agent-reviewer-xyz789/       (AgentWorktree for reviewer agent)
    ├── .git
    ├── src/
    └── ...
```

### AgentCredentialMounter

A component that securely mounts credentials (SSH keys, API tokens) into agent containers as read-only files.

**Managed by:** `pkg/agent/container.AgentCredentialMounter`

**Key Properties:**
- Creates credential files with 0600 permissions
- Mounts read-only into containers
- Cleanup on container stop
- Supports: SSH keys, GitHub tokens, custom credentials

**Security:**
- Read-only mounts prevent tampering
- Restricted file permissions
- Isolated per agent (no sharing)
- Automatic cleanup

**Container Mount Points:**
- SSH key: `/root/.ssh/id_ed25519` (read-only)
- GitHub token: `/root/.github-token` (read-only)

### AgentContainerHandle

A reference to a running AgentSession's complete environment (worktree + container + credentials). Provides unified access to all isolation layers.

**Managed by:** `pkg/agent/container.AgentContainerLauncher`

**Key Properties:**
- AgentID (unique identifier)
- ContainerSession (Docker runtime)
- AgentWorktree (git isolation)
- Credentials path
- State tracking

**Purpose:** Single object representing everything needed for an agent to work.

## Infrastructure Components

### AgentWorktreeManager

Manages the lifecycle of git worktrees for AgentSessions.

**Package:** `pkg/worktree`

**Operations:**
- `Create()`: Create new worktree with unique branch
- `Remove()`: Delete worktree and branch
- `List()`: List all agent worktrees

**Implementation:** Uses shell git commands (`git worktree add`, `git worktree remove`) for reliability.

### AgentContainerLauncher

Orchestrates the complete AgentSession lifecycle by coordinating worktrees, containers, and credentials.

**Package:** `pkg/agent/container`

**Operations:**
- `Spawn()`: Create new AgentSession (worktree + container + credentials)
- `Stop()`: Cleanup AgentSession (stop container, remove worktree, delete credentials)
- `Attach()`: Reconnect to existing AgentSession (TODO: Phase 3)
- `GetHandle()`: Retrieve handle for active agent
- `ListHandles()`: List all active agents

**Responsibilities:**
1. Create isolated worktree
2. Setup credential files
3. Create and start container with mounts
4. Track active AgentSessions
5. Cleanup all resources on stop

### Relay

The WebSocket server that manages UserSessions and coordinates AgentSessions. Acts as the central hub for user-agent communication.

**Key Responsibilities:**
- Accept WebSocket connections from PWA
- Manage UserSession lifecycle
- Spawn AgentSessions based on user requests
- Route messages between users and agents
- Coordinate multi-agent workflows

## Naming Conventions

### Prefixes

All components related to AgentSessions use the **`Agent`** prefix to make relationships explicit:

- `AgentSession` - The complete agent environment
- `AgentWorktree` - Worktree for an agent
- `AgentWorktreeManager` - Manages agent worktrees
- `AgentContainerHandle` - Handle to agent's environment
- `AgentContainerLauncher` - Launches agent containers
- `AgentCredentialMounter` - Mounts agent credentials

**Rationale:** The `Agent` prefix makes it immediately clear that these components are part of the agent isolation system and manage AgentSessions, not generic resources.

### Suffixes

- **`Manager`**: Lifecycle management and operations (Create, Remove, List)
  - Examples: `AgentWorktreeManager`, `ContainerSession.Manager`

- **`Launcher`**: Orchestrates multiple components to create/start something
  - Examples: `AgentContainerLauncher`

- **`Handle`**: Reference to a running resource with metadata
  - Examples: `AgentContainerHandle`

- **`Mounter`**: Manages mounting/unmounting resources
  - Examples: `AgentCredentialMounter`

- **`Session`**: Represents a time-bounded interaction or runtime
  - Examples: `UserSession`, `AgentSession`, `ContainerSession`

### Package Names

Packages are organized by functional domain:

- `pkg/worktree` - Git worktree operations
- `pkg/containersession` - Docker container lifecycle
- `pkg/agent/container` - Agent-specific container orchestration
- `cmd/relay` - Relay server implementation

## Relationship Summary

```
User (human)
  ↓ creates (via PWA)
UserSession (WebSocket connection)
  ↓ spawns
AgentSession (agent task instance)
  ↓ requires
AgentContainerHandle (this layer)
  ├─ AgentWorktree (git isolation)
  │   ├─ Branch: agent-{agentID}-{timestamp}
  │   └─ Path: /workspaces/agent-{agentID}/
  ├─ ContainerSession (Docker runtime)
  │   ├─ Container ID
  │   ├─ State: PENDING → RUNNING → STOPPED
  │   └─ Workspace mount: /workspace
  └─ Credentials (read-only mounts)
      ├─ SSH key: /root/.ssh/id_ed25519
      └─ GitHub token: /root/.github-token
```

### Component Interactions

```
AgentContainerLauncher.Spawn()
  │
  ├─> AgentWorktreeManager.Create()
  │     └─> git worktree add -b agent-{id}-{timestamp}
  │
  ├─> AgentCredentialMounter.Setup()
  │     ├─> Create credentials directory (0700)
  │     ├─> Write SSH key (0600)
  │     └─> Write GitHub token (0600)
  │
  └─> ContainerSession.Manager.CreateContainerSession()
        ├─> Docker container create
        ├─> Mount /workspace (worktree path)
        ├─> Mount /root/.ssh/id_ed25519 (read-only)
        ├─> Mount /root/.github-token (read-only)
        └─> Docker container start

AgentContainerLauncher.Stop()
  │
  ├─> ContainerSession.Manager.StopContainerSession()
  │     └─> Docker container stop
  │
  ├─> AgentWorktreeManager.Remove()
  │     ├─> git worktree remove
  │     └─> git branch -D agent-{id}-{timestamp}
  │
  └─> AgentCredentialMounter.Cleanup()
        └─> rm -rf /credentials/agent-{id}/
```

## State Machines

### ContainerSession States

```
PENDING ──StartContainerSession──> RUNNING ──StopContainerSession──> STOPPED
   │                                   │
   └──────────────(error)──────────────┴─────────────────────────────> FAILED
```

### AgentSession Lifecycle

```
1. User requests task via UserSession
2. Relay spawns AgentSession
3. AgentContainerLauncher.Spawn() creates:
   - AgentWorktree (git isolation)
   - Credentials (read-only files)
   - ContainerSession (Docker runtime)
4. Agent performs work in isolated environment
5. AgentContainerLauncher.Stop() cleans up:
   - Stops container
   - Removes worktree and branch
   - Deletes credentials
6. Results returned to UserSession
```

## Examples

### Example: Coder Agent Workflow

1. **User Action**: User clicks "Implement feature X" in PWA
2. **UserSession**: Sends request to relay over WebSocket
3. **Relay**: Spawns AgentSession for coder agent
4. **Spawn**:
   - Creates worktree: `/workspaces/agent-coder-abc123/` on branch `agent-coder-abc123-20250105-143022`
   - Creates credentials: `/credentials/agent-coder-abc123/` with SSH key and GitHub token
   - Creates container with mounts:
     - `/workspace` → `/workspaces/agent-coder-abc123/` (read-write)
     - `/root/.ssh/id_ed25519` → `/credentials/agent-coder-abc123/id_ed25519` (read-only)
     - `/root/.github-token` → `/credentials/agent-coder-abc123/github-token` (read-only)
   - Starts container (state: RUNNING)
5. **Agent Execution**: Coder agent modifies code in `/workspace`, commits to its branch
6. **Stop**:
   - Stops container (state: STOPPED)
   - Removes worktree and deletes branch
   - Deletes credentials directory
7. **Result**: Changes are on branch `agent-coder-abc123-20250105-143022` in main repository

### Example: Concurrent Agents

Two agents working simultaneously:

```
Repository: /repo
├─ Worktrees:
│  ├─ agent-coder-abc123/       (Coder implementing feature A)
│  └─ agent-reviewer-xyz789/    (Reviewer reviewing feature B)
├─ Credentials:
│  ├─ agent-coder-abc123/       (Coder's credentials)
│  └─ agent-reviewer-xyz789/    (Reviewer's credentials)
└─ Containers:
   ├─ container-abc123          (Coder's runtime)
   └─ container-xyz789          (Reviewer's runtime)
```

Both agents work in complete isolation:
- Separate git branches (no conflicts)
- Separate filesystems (via worktrees)
- Separate containers (process isolation)
- Separate credentials (security isolation)

## Glossary

- **AgentID**: Unique identifier for an agent instance (e.g., "coder-abc123")
- **Session ID**: Unique identifier for a session (UserSession, AgentSession, ContainerSession)
- **Worktree**: Git working directory separate from main repository
- **Branch**: Git branch for isolated code changes
- **Mount**: Docker volume or bind mount for sharing host files into container
- **Credentials**: Authentication tokens/keys for git/API access
- **Handle**: Reference object providing access to managed resource
- **Launcher**: Component that orchestrates creation of complex resources
- **Manager**: Component that manages lifecycle of a resource type

## References

- [Architecture Documentation](../architecture/ARCHITECTURE.md)
- [ContainerSession Package README](../../pkg/containersession/README.md)
- [Git Worktree Documentation](https://git-scm.com/docs/git-worktree)
- [Docker Volumes and Mounts](https://docs.docker.com/storage/volumes/)
