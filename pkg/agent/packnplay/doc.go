// Package packnplay provides a Packnplay-based implementation of the agent.AgentLauncher interface.
//
// This package implements AgentLauncher using Packnplay's library for Docker containerization,
// git worktree management, and credential mounting.
//
// # Architecture
//
// PacknplayLauncher uses Packnplay's runner.Run() to spawn Docker containers with automatic
// worktree management. Each agent gets a unique ULID-based identifier and worktree.
//
// Container Discovery:
//   - Uses Packnplay labels: managed-by=packnplay, packnplay-worktree=agent-{ULID}
//   - Container name pattern: packnplay-{project}-agent-{ULID}
//
// I/O Streaming:
//   - runner.Run() runs in a goroutine (non-blocking)
//   - Docker Engine API provides live stdin/stdout/stderr via ContainerAttach
//   - Stream demultiplexing with TTY=false for separate stdout/stderr
//
// # Usage
//
//	launcher, err := packnplay.NewLauncher(
//		packnplay.WithProjectPath("/path/to/repo"),
//		packnplay.WithDefaultImage("ubuntu:22.04"),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer launcher.Close()
//
//	// Spawn an agent
//	handle, err := launcher.Spawn(ctx, &agent.SpawnConfig{
//		Role:    "coder",
//		Image:   "ourocodus/agent:latest",
//		Command: []string{"bash"},
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Interact with agent
//	io.Copy(handle.Stdin(), os.Stdin)
//	io.Copy(os.Stdout, handle.Stdout())
//
//	// Stop when done
//	launcher.Stop(ctx, handle)
//
// # Testing
//
// Unit tests run by default and cover configuration and helper functions.
//
// Integration tests require Docker and are gated behind the "integration" build tag:
//
//	go test -tags=integration ./pkg/agent/packnplay/...
//
// Integration tests verify:
//   - Container spawning and lifecycle
//   - Live I/O streaming
//   - Attach to existing containers
//   - Agent discovery
//   - Cleanup and error scenarios
//
// # Implementation Details
//
// This implementation fulfills issue #83 and anticipates:
//   - Issue #84: Credential configuration (GitHub CLI, AWS, Git, API keys)
//   - Issue #85: Relay integration via dependency injection
//   - Issue #86: E2E tests for containerized agent lifecycle
package packnplay
