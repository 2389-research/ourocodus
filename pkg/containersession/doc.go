/*
Package containersession provides container session management using Docker SDK.

This package enables isolated execution environments for agents by managing
Docker container lifecycle, workspace directories, and I/O streams with support
for container reuse and session attachment.

# Basic Usage

	// Create manager with dependencies
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}
	idGen := &UUIDGenerator{}
	clock := &SystemClock{}
	logger := log.New(os.Stdout, "[containersession] ", log.LstdFlags)

	manager := containersession.NewManager(dockerClient, idGen, clock, logger, "./workspaces")

	// Create and start a session
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"/bin/bash"})
	if err != nil {
	    log.Fatal(err)
	}

	err = manager.StartContainerSession(ctx, session.ID())
	if err != nil {
	    log.Fatal(err)
	}

	// Stop the session
	err = manager.StopContainerSession(ctx, session.ID())

# Container Reuse

CreateContainerSession automatically checks for existing containers with the same
session ID before creating a new one. This enables resilient reconnection after
process restarts or network failures.

Reuse behavior by container state:
  - Running: Reattaches I/O streams without restarting
  - Stopped/Exited: Starts the container then attaches
  - Created: Starts the container then attaches
  - Dead/Removing: Removes the bad container and returns error (retry to create new)
  - Paused: Returns error (unpause not yet supported)

Example - Automatic Reuse:

	// First process creates and uses a session
	session1, _ := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"/bin/bash"})
	sessionID := session1.ID()
	// ... do work ...

	// Process restarts or crashes
	// Second process with same Manager config automatically reuses the container
	session2, _ := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"/bin/bash"})
	// session2 reconnects to existing container if still running

Note: CreateContainerSession generates a new session ID each time, so automatic
reuse only occurs when the generated ID matches an existing container (rare).
For intentional reuse across processes, use AttachContainerSession instead.

# Explicit Session Attachment

AttachContainerSession allows explicitly reconnecting to a running container
by session ID. This is useful for:
  - Reconnecting after process restart
  - Attaching from a different process
  - Monitoring or debugging existing sessions

Example - Explicit Attach:

	// Process A creates a session
	session, _ := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"/bin/bash"})
	sessionID := session.ID()
	_ = manager.StartContainerSession(ctx, sessionID)

	// Store sessionID persistently (database, file, etc.)
	saveSessionID(sessionID)

	// Process B reconnects to the same session
	sessionID := loadSessionID()
	manager2, _ := NewManager(...) // New manager instance
	reattached, _ := manager2.AttachContainerSession(ctx, sessionID)
	// reattached is connected to the same running container

AttachContainerSession requirements:
  - Container must exist (returns ErrSessionNotFound if not)
  - Container must be in "running" state (returns error otherwise)
  - Container must have /workspace mount (returns error if missing)

# Label Conventions

Containers are labeled with:
  - com.ourocodus.containersession.id: Unique session ID
  - com.ourocodus.containersession.created: RFC3339 timestamp
  - com.ourocodus.containersession.managed-by: "ourocodus-containersession"

These labels enable discovery and management of containers across restarts.
The package uses label-based filtering to find existing containers efficiently.

# Workspace Security

Workspace paths are validated to prevent directory traversal attacks.
All workspace directories are created with 0700 permissions (owner-only access).
Path validation follows defense-in-depth principles from pkg/relay/session.

When reusing containers, workspace paths are extracted from existing volume mounts
and validated to ensure they're under the configured base directory.

# Session Lifecycle

Sessions progress through states:
  - PENDING: Created but not started
  - RUNNING: Container is running
  - STOPPED: Container stopped gracefully
  - FAILED: Container failed or error occurred

State transitions are thread-safe and logged for observability.

# Thread Safety

All Manager methods are thread-safe and can be called concurrently.
ContainerSession methods use internal locking to ensure safe concurrent access.

The in-memory session map uses RWMutex for efficient concurrent reads with
safe writes during session creation and cleanup.

# Edge Cases and Limitations

Container Reuse:
  - Multiple containers with same session ID: Uses first found, logs warning
  - Container in "removing" state: Treated as not found
  - Paused containers: Returns error (unpause not supported yet)
  - Dead containers: Removed automatically, returns error to retry

Attach Limitations:
  - Can only attach to running containers (not stopped)
  - Requires container to have /workspace mount at expected location
  - I/O attach failures are logged but don't fail the operation (container still usable)

Concurrent Operations:
  - CreateContainerSession with same ID may race (both create containers)
  - AttachContainerSession is idempotent (safe to call multiple times)
  - StopContainerSession is idempotent (safe to call on stopped sessions)

# Error Handling

The package uses structured errors for common failure cases:
  - ErrSessionNotFound: Session ID not found in manager or Docker
  - ErrSessionAlreadyExists: Session ID already exists in manager
  - ErrInvalidState: Operation not allowed in current session state

Docker API errors are wrapped with context using fmt.Errorf with %w.

# I/O Stream Management

Container stdout/stderr are automatically demuxed using Docker's stdcopy package.
Streams are managed in background goroutines that log errors for observability.

When reattaching or attaching to existing containers:
  - Previous stream goroutines continue running (no cleanup needed)
  - New stream goroutines are spawned for new I/O connections
  - Stream attach failures are logged but don't block session operations

# Performance Considerations

Label-based Discovery:
  - Uses Docker API filters for efficient container lookup
  - Lists containers with All=true to include stopped containers
  - Single API call per discovery operation

Memory Usage:
  - In-memory session map grows with active sessions
  - Stopped sessions remain in map (call StopContainerSession to cleanup)
  - Each session holds references to workspace path and labels

Concurrency:
  - Manager uses RWMutex for optimal read concurrency
  - ContainerSession uses Mutex for state protection
  - No blocking operations under locks (fast critical sections)
*/
package containersession
