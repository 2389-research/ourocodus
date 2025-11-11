# ACP Launcher Selection Architecture

This document describes the runtime launcher selection mechanism for ACP (Agent Client Protocol) execution in Ourocodus.

## Overview

The system supports two execution modes for ACP processes:
1. **Host mode** (default) - ACP runs as a host process via `os/exec`
2. **Container mode** (opt-in) - ACP runs inside agent containers via `docker exec`

The launcher is selected at runtime based on environment configuration and runtime context.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                            main.go                                  │
│                                                                     │
│  initializeAgentInfrastructure()                                   │
│    ├─> dockerClient                                                │
│    ├─> launcherFactory (spawns containers)                        │
│    └─> containerManager (docker exec service)                     │
│                          │                                         │
│                          ▼                                         │
│  NewSessionManager(containerManager)                              │
└─────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      pkg/relay/session_adapter.go                   │
│                                                                     │
│  NewSessionManager(..., containerManager)                          │
│    └─> NewACPClientFactory(containerManager, logger)              │
│                          │                                         │
│                          ▼                                         │
│        ACPClientFactory {                                          │
│            apiKey: "sk-..."                                        │
│            acpBinaryPath: "/path/to/acp"  // optional            │
│            containerSessionMgr: containerManager                   │
│            logger: logger                                          │
│        }                                                           │
└─────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                pkg/relay/session/client_factory.go                  │
│                                                                     │
│  factory.NewClient(ctx, runtime)                                   │
│    │                                                               │
│    ├─> runtime validation                                         │
│    │     ├─> nil check                                            │
│    │     └─> workspace check                                      │
│    │                                                               │
│    └─> selectLauncher(runtime)                                    │
│          │                                                         │
│          ├─> getRuntimeMode()                                     │
│          │     ├─> Read OUROCODUS_ACP_RUNTIME                     │
│          │     ├─> "" or "host" → "host"                          │
│          │     ├─> "container" → "container"                      │
│          │     └─> other → error                                  │
│          │                                                         │
│          ├─> switch (mode) {                                      │
│          │     case "host":                                       │
│          │       └─> createHostLauncher(runtime)                 │
│          │             └─> return HostProcessLauncher{}           │
│          │                                                         │
│          │     case "container":                                  │
│          │       ├─> validateContainerPrerequisites()            │
│          │       │     ├─> runtime.HasContainer()               │
│          │       │     └─> containerSessionMgr != nil           │
│          │       │                                                │
│          │       └─> createContainerLauncher(runtime)            │
│          │             └─> NewContainerExecProcessLauncher()     │
│          │                   .WithWorkspacePath("/workspace")    │
│          │   }                                                    │
│          │                                                         │
│          └─> launcher                                             │
└─────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      pkg/acp/client.go                              │
│                                                                     │
│  acp.NewClient(workspace, apiKey,                                  │
│      acp.WithProcessLauncher(launcher))                           │
│    │                                                               │
│    └─> launcher.Start(ctx, launchCfg)                            │
│          │                                                         │
│          ├─ HOST MODE ───────────────────────────────────┐       │
│          │                                                │       │
│          │  HostProcessLauncher.Start()                  │       │
│          │    └─> exec.CommandContext(...)              │       │
│          │          └─> claude-code-acp                  │       │
│          │                (runs on host)                 │       │
│          │                     │                          │       │
│          │                     ▼                          │       │
│          │              stdin/stdout pipes ────────────────┘      │
│          │                                                         │
│          └─ CONTAINER MODE ──────────────────────────────┐       │
│                                                            │       │
│            ContainerExecProcessLauncher.Start()          │       │
│              └─> containerSessionMgr.ExecInContainer()   │       │
│                    └─> dockerClient.ContainerExecCreate()│       │
│                          └─> docker exec container_id    │       │
│                                /workspace/claude-code-acp│       │
│                                     │                     │       │
│                                     ▼                     │       │
│                         multiplexed stdout/stderr ────────┘       │
│                                                                     │
│  return Transport {                                                │
│      Read() → stdout                                               │
│      Write() → stdin                                               │
│      Stderr() → stderr                                             │
│      Close() → cleanup                                             │
│  }                                                                 │
└─────────────────────────────────────────────────────────────────────┘
```

## Runtime Context Flow

```
AgentRuntimeContext {
    SessionID: "session-abc123"
    AgentID: "coder"
    Workspace: "/Users/dev/workspaces/session-abc123"
    ContainerID: "container-xyz789"  // empty string for host mode
}
              │
              ▼
    runtime.HasContainer()
              │
              ├─ true ──> Container mode eligible
              │            (if OUROCODUS_ACP_RUNTIME=container)
              │
              └─ false ─> Host mode only
                          (ContainerID empty)
```

## Configuration Matrix

| OUROCODUS_ACP_RUNTIME | HasContainer() | ContainerMgr | Result                          |
|-----------------------|----------------|--------------|----------------------------------|
| "" (default)          | false          | any          | ✅ Host mode                     |
| "" (default)          | true           | any          | ✅ Host mode                     |
| "host"                | false          | any          | ✅ Host mode                     |
| "host"                | true           | any          | ✅ Host mode                     |
| "container"           | false          | any          | ❌ Error: no container ID        |
| "container"           | true           | nil          | ❌ Error: no manager             |
| "container"           | true           | available    | ✅ Container mode                |
| "kubernetes"          | any            | any          | ❌ Error: invalid runtime value  |

## Decision Tree

```
                    getRuntimeMode()
                         │
        ┌────────────────┼────────────────┐
        │                │                │
     mode=""          mode="host"    mode="container"
        │                │                │
        ▼                ▼                ▼
    Host Mode        Host Mode    validatePrerequisites()
                                         │
                        ┌────────────────┼───────────────┐
                        │                │               │
                  HasContainer()?  Manager != nil?  (both true)
                        │                │               │
                    ┌───┴───┐        ┌───┴───┐          ▼
                   No      Yes      No      Yes    Container Mode
                    │        │        │        │
                    ▼        ▼        ▼        ▼
                 ERROR    ERROR    ERROR       ✅
```

## Component Dependencies

```
┌──────────────────┐
│   main.go        │
│  - Docker client │◄─────────────┐
│  - Launcher      │               │
│  - Container mgr │               │
└────────┬─────────┘               │
         │                         │
         │ passes containerMgr     │
         ▼                         │
┌──────────────────┐               │
│ session_adapter  │               │
│  - Wires deps    │               │
└────────┬─────────┘               │
         │                         │
         │ injects                 │
         ▼                         │
┌──────────────────┐               │
│ ACPClientFactory │               │
│  - Stores mgr    │               │
│  - Creates ACP   │               │
└────────┬─────────┘               │
         │                         │
         │ uses when mode=container│
         ▼                         │
┌──────────────────┐               │
│ContainerExecLauncher├────────────┘
│  - Calls docker  │
│  - Returns transport
└──────────────────┘
```

## Error Handling Paths

### Validation Errors (Before Launching)

```
NewClient(ctx, runtime)
    │
    ├─> runtime == nil
    │     └─> "runtime context is required"
    │
    ├─> runtime.Workspace == ""
    │     └─> "workspace is required"
    │
    └─> selectLauncher(runtime)
          │
          ├─> getRuntimeMode() error
          │     └─> "invalid OUROCODUS_ACP_RUNTIME value: %q"
          │
          └─> validateContainerPrerequisites() error
                │
                ├─> !HasContainer()
                │     └─> "no container ID in runtime context (session=%s agent=%s)"
                │
                └─> containerSessionMgr == nil
                      └─> "container session manager not available (session=%s agent=%s)"
```

### Runtime Errors (During Launch)

```
launcher.Start(ctx, cfg)
    │
    ├─ HOST MODE
    │    └─> exec.CommandContext() error
    │          └─> "failed to start ACP process: %w"
    │
    └─ CONTAINER MODE
         └─> ExecInContainer() error
               └─> "failed to exec ACP command %q in container %s: %w"
```

## Testability

The decomposed architecture enables focused unit testing:

### Unit Test Targets

```
getRuntimeMode()
  ├─> TestGetRuntimeMode_Default
  ├─> TestGetRuntimeMode_Host
  ├─> TestGetRuntimeMode_Container
  └─> TestGetRuntimeMode_Invalid (table-driven)

validateContainerPrerequisites()
  ├─> TestValidateContainerPrerequisites_Success
  ├─> TestValidateContainerPrerequisites_MissingContainerID
  ├─> TestValidateContainerPrerequisites_NilManager
  └─> TestValidateContainerPrerequisites_NilRuntime

createHostLauncher()
  ├─> TestCreateHostLauncher_WithoutLogger
  └─> TestCreateHostLauncher_WithLogger

createContainerLauncher()
  ├─> TestCreateContainerLauncher_WithoutLogger
  └─> TestCreateContainerLauncher_WithLogger

selectLauncher()
  ├─> TestSelectLauncher_HostMode_Default
  ├─> TestSelectLauncher_HostMode_Explicit
  ├─> TestSelectLauncher_ContainerMode_Success
  ├─> TestSelectLauncher_ContainerMode_MissingContainerID
  ├─> TestSelectLauncher_ContainerMode_MissingManager
  └─> TestSelectLauncher_InvalidMode
```

### Integration Test Targets

```
NewClient()
  ├─> TestNewClient_Integration_HostMode
  ├─> TestNewClient_Integration_ContainerMode_MissingPrerequisites
  ├─> TestNewClient_Integration_ContainerMode_MissingContainerID
  ├─> TestNewClient_Integration_NilRuntime
  └─> TestNewClient_Integration_EmptyWorkspace
```

### Smoke Test Targets (requires Docker)

```
ContainerExecProcessLauncher
  ├─> TestContainerExecProcessLauncher_SmokeTest
  │     └─> Verifies commands execute in containers
  │
  └─> TestContainerExecProcessLauncher_WithEchoAgent
        └─> Verifies echo-agent binary runs in containers
```

## Future Enhancements

### Planned Improvements

1. **KubernetesExecProcessLauncher** - Run ACP in k8s pods
2. **RemoteSSHProcessLauncher** - Run ACP on remote machines
3. **WebSocketProcessLauncher** - ACP connects to relay (Phase 2)
4. **Metrics & Observability** - Track launcher selection and execution times
5. **Dynamic Launcher Selection** - Choose launcher based on workload characteristics

### Extension Pattern

```go
// Future: Kubernetes support
type KubernetesExecProcessLauncher struct {
    clientset *kubernetes.Clientset
    namespace string
    podName   string
}

func (l *KubernetesExecProcessLauncher) Start(ctx context.Context, cfg acp.ProcessLaunchConfig) (acp.Transport, error) {
    // kubectl exec equivalent
}

// Registration
func (f *ACPClientFactory) selectLauncher(runtime *AgentRuntimeContext) (acp.ProcessLauncher, error) {
    mode, err := getRuntimeMode()
    // ...

    switch mode {
    case "host":
        return f.createHostLauncher(runtime), nil
    case "container":
        return f.createContainerLauncher(runtime), nil
    case "kubernetes":  // New
        return f.createKubernetesLauncher(runtime), nil
    }
}
```

## References

- Implementation: `pkg/relay/session/client_factory.go`
- Transport abstraction: `pkg/acp/transport.go`
- Container execution: `pkg/relay/session/container_exec_process_launcher.go`
- Tests: `pkg/relay/session/client_factory_test.go`
- Smoke tests: `tests/e2e/acp_container_exec_test.go`
