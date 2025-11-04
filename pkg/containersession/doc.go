/*
Package containersession provides container session management using Docker SDK.

This package enables isolated execution environments for agents by managing
Docker container lifecycle, workspace directories, and I/O streams.

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

# Thread Safety

All Manager methods are thread-safe and can be called concurrently.
ContainerSession methods use internal locking to ensure safe concurrent access.

# Label Conventions

Containers are labeled with:
  - com.ourocodus.containersession.id: Unique session ID
  - com.ourocodus.containersession.created: RFC3339 timestamp
  - com.ourocodus.containersession.managed-by: "ourocodus-containersession"

These labels enable discovery and management of containers across restarts.

# Workspace Security

Workspace paths are validated to prevent directory traversal attacks.
All workspace directories are created with 0700 permissions (owner-only access).
Path validation follows defense-in-depth principles from pkg/relay/session.

# Session Lifecycle

Sessions progress through states:
  - PENDING: Created but not started
  - RUNNING: Container is running
  - STOPPED: Container stopped gracefully
  - FAILED: Container failed or error occurred

State transitions are thread-safe and logged for observability.
*/
package containersession
