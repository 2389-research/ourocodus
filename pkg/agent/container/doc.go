// Package container provides Docker container management for AgentSessions.
//
// The container package orchestrates Docker containers for AgentSession execution,
// combining workspace isolation (via worktrees) with credential mounting and container
// lifecycle management.
//
// # Core Concepts
//
// An AgentSession requires:
//   - Isolated workspace (git worktree from pkg/worktree)
//   - Docker container for execution environment
//   - Credentials mounted read-only for git/API access
//   - I/O streams connected for agent interaction
//
// AgentContainerLauncher handles the complete lifecycle:
//   - Spawning containers with workspace and credentials
//   - Attaching to existing containers (cross-process)
//   - Stopping containers gracefully
//
// # Architecture
//
// The package consists of three main components:
//
// 1. AgentContainerLauncher - Main orchestrator
//   - Coordinates worktree, credentials, and container
//   - Manages AgentContainerHandle instances
//   - Provides Spawn(), Attach(), and Stop() operations
//
// 2. AgentContainerHandle - Running container reference
//   - Holds container session and worktree references
//   - Provides access to workspace, credentials, and I/O
//   - Tracks container state and metadata
//
// 3. AgentCredentialMounter - Credential management
//   - Creates read-only credential files
//   - Mounts credentials as read-only volumes in containers
//   - Cleans up credentials on container stop
//
// # Example Usage
//
//	// Create launcher with dependencies
//	launcher := container.NewAgentContainerLauncher(
//	    dockerClient,
//	    worktreeManager,
//	    credMounter,
//	    idGen,
//	    clock,
//	    logger,
//	    "/workspaces",
//	)
//
//	// Spawn new agent container
//	handle, err := launcher.Spawn(ctx, container.SpawnConfig{
//	    AgentID:     "coder-abc123",
//	    ImageName:   "ourocodus/agent:latest",
//	    Command:     []string{"/bin/bash"},
//	    GitSSHKey:   sshKeyData,
//	    GitHubToken: ghTokenData,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer launcher.Stop(ctx, handle.AgentID())
//
//	// Agent works in isolated container with:
//	// - Workspace: handle.WorkspacePath()
//	// - Credentials: handle.CredentialsPath()
//	// - Container: handle.ContainerID()
//
// # Relationship to Domain Model
//
//	UserSession (relay WebSocket)
//	    ↓ spawns
//	AgentSession (agent process)
//	    ↓ requires
//	AgentContainerHandle (this package)
//	    ├─ AgentWorktree (git isolation)
//	    ├─ ContainerSession (Docker runtime)
//	    └─ Credentials (read-only mounts)
//	    ↓ filesystem
//	/workspaces/agent-{id}/        (git worktree)
//	/credentials/agent-{id}/       (read-only credentials)
//	/workspace/                    (container mount point)
//	/root/.ssh/id_ed25519         (container credential mount)
//	/root/.github-token           (container credential mount)
//
// Each AgentSession gets an AgentContainerHandle that coordinates all isolation layers,
// enabling concurrent agents to work safely on the same repository.
//
// # Security
//
// - Credentials are mounted read-only to prevent tampering
// - Workspace paths are validated to prevent traversal
// - Each container runs with its own isolated filesystem
// - Git worktrees provide branch isolation
// - Containers are labeled for tracking and cleanup
//
// # Thread Safety
//
// AgentContainerLauncher is safe for concurrent use. Multiple goroutines can safely
// call Spawn(), Attach(), and Stop() simultaneously.
package container
