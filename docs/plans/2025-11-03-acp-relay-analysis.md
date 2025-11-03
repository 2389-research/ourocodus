# ACP-Relay Analysis and Comparison
**Date:** 2025-11-03
**Repository:** https://github.com/2389-research/acp-relay
**Purpose:** Evaluate acp-relay as alternative to PackNplay for ourocodus agent orchestration

## Executive Summary

**ACP-Relay is MUCH more aligned with our needs than PackNplay!**

- **PackNplay**: Git worktree management tool that happens to spawn containers
- **ACP-Relay**: ACP-first relay server designed for agent orchestration
- **Key Insight**: ACP-Relay **already uses Docker SDK** (not CLI!), avoiding all the TTY issues we've been debugging

## What is ACP-Relay?

ACP-Relay is a **relay server** that:
1. Accepts HTTP/WebSocket connections
2. Translates requests to ACP JSON-RPC protocol
3. Spawns isolated agent subprocesses (or containers) per session
4. Manages bidirectional I/O streaming
5. Provides session lifecycle management

It's essentially middleware between clients (like ourocodus) and agent processes, with containerization built in.

## Architecture Comparison

### PackNplay Architecture
```
Your Code
  └─> PackNplay Library
      └─> exec.Command("docker", ...)  ← Docker CLI (TTY issues!)
          └─> Git worktree management
          └─> Container spawning
```

### ACP-Relay Architecture
```
Your Code
  └─> HTTP/WebSocket Client
      └─> ACP-Relay Server
          ├─> Docker SDK (client.Client)  ← Direct API! No TTY issues!
          ├─> Session Manager
          └─> stdio Bridge (channels)
```

### Ourocodus with ACP-Relay
```
Ourocodus
  └─> pkg/agent Interface
      └─> ACPRelayLauncher (new!)
          └─> HTTP Client → ACP-Relay Server
              └─> Docker SDK → Containers
```

## Key Differences

| Feature | PackNplay | ACP-Relay |
|---------|-----------|-----------|
| **Primary Purpose** | Git worktree + container tool | Agent session relay server |
| **Docker Integration** | CLI (`exec.Command`) | SDK (`github.com/docker/docker/client`) |
| **TTY Issues** | Yes (we spent days on this!) | No (SDK doesn't have TTY semantics) |
| **Protocol** | Library API (Go functions) | ACP JSON-RPC (HTTP/WebSocket) |
| **I/O Streaming** | Direct stdio pipes | Bidirectional channels + streaming |
| **Session Management** | Manual | Built-in with lifecycle hooks |
| **Container Reuse** | No | Yes (finds existing containers by label) |
| **Multi-Agent** | Not designed for it | Designed for concurrent sessions |
| **Error Handling** | Basic Go errors | LLM-optimized structured errors |

## Features ACP-Relay Has That We Need

### 1. **Docker SDK Instead of CLI** ✅
**Critical:** Bypasses all the TTY/stdin issues we've been debugging!

```go
// ACP-Relay uses Docker SDK directly
dockerClient, err := client.NewClientWithOpts(
    client.WithHost(cfg.DockerHost),
    client.WithAPIVersionNegotiation(),
)

// Attach to container for I/O
attachResp, err := dockerClient.ContainerAttach(ctx, containerID, ...)
```

No `exec.Command`, no TTY errors, direct API access.

### 2. **Container Reuse** ✅
Finds existing containers by label and reattaches instead of creating new ones:

```go
// Query containers with labels
filters.Add("label", "managed-by=acp-relay")
filters.Add("label", fmt.Sprintf("session-id=%s", sessionID))
```

This solves the parallel spawning issues we were worried about!

### 3. **Bidirectional I/O Streaming** ✅
Proper stdio bridge with goroutines managing channels:

```go
type SessionComponents struct {
    ContainerID string
    Stdin       io.WriteCloser
    Stdout      io.ReadCloser
    Stderr      io.ReadCloser
}
```

Exactly what we need for ACP communication!

### 4. **Session Lifecycle Management** ✅
Built-in tracking of sessions, automatic cleanup, health monitoring:

```go
type Manager struct {
    containers   map[string]*ContainerInfo // sessionID -> container
    mu           sync.RWMutex
}

func (m *Manager) monitorContainer(ctx, containerID, sessionID)
func (m *Manager) StopSession(ctx, sessionID)
```

### 5. **Environment Variable Security** ✅
Allowlist approach (only TERM, LANG, etc.) instead of passing everything:

```go
// Allowlist: only safe terminal and locale vars
allowlist := []string{"TERM", "COLORTERM", "LANG", ...}
```

### 6. **Container Labels for Observability** ✅
Every container tagged for easy debugging:

```yaml
managed-by: acp-relay
session-id: <session-id>
created-at: <timestamp>
```

Can list all managed containers:
```bash
docker ps --filter label=managed-by=acp-relay
```

### 7. **LLM-Optimized Errors** ✅
Error messages designed for AI agents with structured data:

```json
{
  "error_type": "session_not_found",
  "explanation": "The relay server doesn't have...",
  "possible_causes": [...],
  "suggested_actions": [...],
  "relevant_state": {...}
}
```

## What PackNplay Has That ACP-Relay Doesn't

### 1. **Git Worktree Management** ⚠️
PackNplay creates isolated git worktrees per agent.

**But:** ACP-Relay uses workspace directories that we could mount as volumes. We could handle worktree creation separately in ourocodus.

### 2. **Direct Library API** ⚠️
PackNplay is a Go library; ACP-Relay is a server.

**But:** Server architecture is actually **better** for:
- Multi-agent coordination
- Process isolation
- Language independence
- Horizontal scaling

### 3. **Credential File Mounting** ⚠️
PackNplay has special handling for `.claude.json`, etc.

**But:** We can configure volume mounts in ACP-Relay config.

## Integration Strategy for Ourocodus

### Option A: Replace PackNplay with ACP-Relay (Recommended)

**Approach:**
1. Run ACP-Relay as a sidecar service
2. Implement `ACPRelayLauncher` that satisfies `agent.Launcher` interface
3. Use HTTP/WebSocket to communicate with relay server
4. Let relay handle all Docker interactions

**Benefits:**
- No TTY issues (SDK-based!)
- Better multi-agent support
- Container reuse built-in
- Proper I/O streaming
- Session management included
- Future-proof for ACP

**Implementation:**
```go
// pkg/agent/acprelay/launcher.go
type ACPRelayLauncher struct {
    httpClient  *http.Client
    wsConn      *websocket.Conn
    relayURL    string
}

func (l *ACPRelayLauncher) Spawn(ctx, cfg) (Handle, error) {
    // POST to http://relay:8080/session/new
    // Returns session_id
    // Open WebSocket to ws://relay:8081/<session_id>
    // Return handle that wraps WebSocket I/O
}
```

**Effort:** 2-3 days
- Implement ACPRelayLauncher
- Handle WebSocket I/O
- Add worktree management in ourocodus (if needed)
- Update container-race demo

### Option B: Use ACP-Relay for Some Agents, PackNplay for Others

Keep both:
- PackNplay for simple cases (single agent, git-centric)
- ACP-Relay for complex orchestration (multiple agents, streaming I/O)

**Effort:** Minimal initially, but maintains dual implementations.

### Option C: Extract Patterns from ACP-Relay into PackNplay

Port the Docker SDK approach and container reuse logic to PackNplay.

**Effort:** High, and loses many ACP-Relay benefits like HTTP/WS protocol, session management, etc.

## Technical Details

### Docker SDK Usage in ACP-Relay

**Container Creation:**
```go
resp, err := m.dockerClient.ContainerCreate(ctx, &container.Config{
    Image: m.config.Image,
    Cmd:   append([]string{m.agentCommand}, m.agentArgs...),
    Env:   envVars,
    Labels: m.buildContainerLabels(sessionID),
    WorkingDir: m.config.WorkspaceContainerPath,
}, &container.HostConfig{
    Binds: []string{
        fmt.Sprintf("%s:%s", hostWorkspace, m.config.WorkspaceContainerPath),
    },
    AutoRemove: m.config.AutoRemove,
    Resources: container.Resources{
        Memory:   memoryBytes,
        NanoCPUs: cpuNano,
    },
    NetworkMode: container.NetworkMode(m.config.NetworkMode),
}, nil, nil, containerName)
```

**I/O Attachment:**
```go
attachResp, err := m.dockerClient.ContainerAttach(ctx, containerID, container.AttachOptions{
    Stream: true,
    Stdin:  true,
    Stdout: true,
    Stderr: true,
})

// Use stdcopy to demux Docker's multiplexed stream
stdoutReader, stderrReader := demuxStreams(attachResp.Reader)
```

**No TTY flags, no exec.Command, no stdin issues!**

### Dependencies

ACP-Relay uses:
- `github.com/docker/docker` v28.5.1 - **Same as ourocodus!**
- `github.com/gorilla/websocket` - WebSocket support
- `github.com/google/uuid` - Session IDs
- `github.com/spf13/viper` - Config management

All compatible with our stack.

## Recommendations

### Immediate: Use ACP-Relay (Option A)

**Why:**
1. **Solves our TTY problems** - Uses Docker SDK, not CLI
2. **Better architecture** - Server-based, designed for multi-agent
3. **More features** - Session management, container reuse, streaming I/O
4. **ACP-native** - Built for Agent Client Protocol from the start
5. **Active development** - In our own org, can extend as needed

**Migration Path:**
1. **Week 1:** Implement `ACPRelayLauncher` wrapping HTTP/WebSocket client
2. **Week 2:** Test single-agent scenarios, fix integration issues
3. **Week 3:** Test multi-agent scenarios (container-race demo)
4. **Week 4:** Add worktree management if needed, polish

### Future: Extend ACP-Relay

ACP-Relay is in our org (2389-research) and designed to be extended:

**Potential additions:**
- Git worktree integration (mount specific branches/worktrees)
- Agent discovery and coordination (multi-agent communication)
- Resource limits and quotas per session
- Metrics and observability (Prometheus, etc.)
- Authentication and authorization

## PackNplay vs ACP-Relay: Decision Matrix

| Criteria | PackNplay (with fork) | ACP-Relay |
|----------|----------------------|-----------|
| **Solves TTY issues** | Partially (warnings remain) | Completely (no CLI) |
| **Multi-agent ready** | No | Yes |
| **I/O streaming** | Manual pipe management | Built-in channels |
| **Session lifecycle** | Manual | Automatic |
| **Container reuse** | No | Yes |
| **Error handling** | Basic | LLM-optimized |
| **Protocol** | Library calls | HTTP/WebSocket |
| **Horizontal scaling** | No | Yes (multiple servers) |
| **Git integration** | Built-in worktrees | Need to add |
| **Setup complexity** | Library import | Run server + client |
| **Debugging** | Hard (no visibility) | Easy (container labels, logs) |

## Conclusion

**ACP-Relay is the right choice for ourocodus.**

While we've invested time in PackNplay, it's fundamentally a git worktree tool adapted for containerization, not a purpose-built agent orchestration system. ACP-Relay:

1. **Solves our immediate problem** (TTY errors via Docker SDK)
2. **Aligns with our architecture** (ACP-native, multi-agent)
3. **Provides better primitives** (sessions, streaming, lifecycle)
4. **Is extensible** (in our org, active development)

The server architecture is actually a **benefit**: it provides process isolation, allows horizontal scaling, and creates clean separation of concerns. The git worktree functionality can be added to ourocodus or extended in ACP-Relay.

**Recommended Next Step:** Implement `ACPRelayLauncher` as proof-of-concept, replacing PackNplay in container-race demo.

---

## Appendix: Code Snippets

### ACP-Relay Container Creation (Full)

```go
// From internal/container/manager.go:CreateSession
func (m *Manager) CreateSession(ctx context.Context, sessionID, workingDir string) (*SessionComponents, error) {
    // Check for existing container first (reuse!)
    existingID, err := m.findExistingContainer(ctx, sessionID)
    if existingID != "" && containerIsRunning(existingID) {
        // Reattach to existing
        return m.attachToExisting(ctx, existingID, sessionID)
    }

    // Create workspace on host
    hostWorkspace := filepath.Join(m.config.WorkspaceHostBase, sessionID)
    os.MkdirAll(hostWorkspace, 0755)

    // Prepare environment
    envVars := m.buildEnvVars()

    // Create container via Docker SDK
    resp, err := m.dockerClient.ContainerCreate(ctx,
        &container.Config{
            Image:      m.config.Image,
            Cmd:        append([]string{m.agentCommand}, m.agentArgs...),
            Env:        envVars,
            Labels:     m.buildContainerLabels(sessionID),
            WorkingDir: m.config.WorkspaceContainerPath,
        },
        &container.HostConfig{
            Binds: []string{
                fmt.Sprintf("%s:%s", hostWorkspace, m.config.WorkspaceContainerPath),
            },
            Resources: container.Resources{
                Memory:   parseMemory(m.config.MemoryLimit),
                NanoCPUs: parseCPU(m.config.CPULimit),
            },
        },
        nil, nil,
        m.sanitizeContainerName(sessionID))

    // Start container
    m.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{})

    // Attach for I/O
    attachResp, err := m.dockerClient.ContainerAttach(ctx, resp.ID,
        container.AttachOptions{
            Stream: true,
            Stdin:  true,
            Stdout: true,
            Stderr: true,
        })

    // Demux stdout/stderr
    stdoutReader, stderrReader := demuxStreams(attachResp.Reader)

    return &SessionComponents{
        ContainerID: resp.ID,
        Stdin:       attachResp.Conn,
        Stdout:      stdoutReader,
        Stderr:      stderrReader,
    }, nil
}
```

### Proposed ACPRelayLauncher Interface

```go
package acprelay

import (
    "context"
    "net/http"
    "github.com/gorilla/websocket"
    "github.com/2389-research/ourocodus/pkg/agent"
)

type ACPRelayLauncher struct {
    relayHTTPURL string // http://localhost:8080
    relayWSURL   string // ws://localhost:8081
    httpClient   *http.Client
}

func NewLauncher(opts ...Option) (*ACPRelayLauncher, error) {
    // Initialize with relay server URLs
}

func (l *ACPRelayLauncher) Spawn(ctx context.Context, cfg *agent.SpawnConfig) (agent.Handle, error) {
    // 1. POST to /session/new with session ID and config
    sessionResp := l.createSession(ctx, cfg)

    // 2. Open WebSocket to /ws/<session-id>
    wsConn, err := websocket.Dial(l.relayWSURL + "/" + sessionResp.SessionID)

    // 3. Return handle wrapping WebSocket
    return &ACPRelayHandle{
        sessionID: sessionResp.SessionID,
        wsConn:    wsConn,
        workspace: sessionResp.Workspace,
    }, nil
}

func (l *ACPRelayLauncher) Stop(ctx context.Context, handle agent.Handle) error {
    // POST to /session/<id>/stop
    h := handle.(*ACPRelayHandle)
    return l.httpClient.Post(l.relayHTTPURL + "/session/" + h.sessionID + "/stop")
}

type ACPRelayHandle struct {
    sessionID string
    wsConn    *websocket.Conn
    workspace string
}

func (h *ACPRelayHandle) Stdin() io.Writer {
    return &wsWriter{conn: h.wsConn}
}

func (h *ACPRelayHandle) Stdout() io.Reader {
    return &wsReader{conn: h.wsConn, stream: "stdout"}
}

func (h *ACPRelayHandle) Workspace() string {
    return h.workspace
}
```

This would be ~300 lines of code vs forking and maintaining PackNplay!
