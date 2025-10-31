// Package agent provides abstractions for agent lifecycle management.
// This package defines framework-agnostic interfaces that can be implemented
// using different container runtimes (Docker, Kubernetes, etc.) or process
// managers.
package agent

import (
	"context"
	"io"
)

// AgentLauncher manages the lifecycle of agent instances.
// Implementations are responsible for spawning, attaching to, and stopping agents.
// This interface is designed to be framework-agnostic, supporting different
// container runtimes or process management systems.
type AgentLauncher interface {
	// Spawn creates and starts a new agent instance with the given configuration.
	// Returns an AgentHandle for interacting with the spawned agent, or an error
	// if the spawn operation fails.
	//
	// The context can be used to cancel the spawn operation if it takes too long.
	// Canceling the context does not stop the agent if it has already been created.
	Spawn(ctx context.Context, config *SpawnConfig) (AgentHandle, error)

	// Attach reconnects to an existing agent instance by its ID.
	// Returns an AgentHandle for the existing agent, or an error if the agent
	// cannot be found or attached.
	//
	// This is useful for recovering from crashes or reconnecting to long-running agents.
	Attach(ctx context.Context, id string) (AgentHandle, error)

	// Stop terminates an agent instance gracefully.
	// The implementation should attempt a graceful shutdown first, falling back
	// to forceful termination if necessary.
	//
	// Returns an error if the agent cannot be stopped.
	Stop(ctx context.Context, handle AgentHandle) error
}

// AgentHandle represents a running or attached agent instance.
// It provides methods for inspecting the agent's state and communicating with it.
type AgentHandle interface {
	// ID returns the unique identifier for this agent instance.
	ID() string

	// Workspace returns the filesystem path to the agent's workspace directory.
	// For containerized agents, this is typically a git worktree.
	Workspace() string

	// ContainerID returns the container ID if the agent is running in a container.
	// Returns an empty string if the agent is not containerized.
	ContainerID() string

	// Stdin returns a writer for sending input to the agent's standard input.
	Stdin() io.WriteCloser

	// Stdout returns a reader for receiving output from the agent's standard output.
	Stdout() io.ReadCloser

	// Stderr returns a reader for receiving output from the agent's standard error.
	Stderr() io.ReadCloser

	// Wait blocks until the agent process exits and returns its exit status.
	// Returns an error if the agent exited with a non-zero status or if
	// the wait operation failed.
	Wait(ctx context.Context) error

	// Close releases any resources associated with this handle.
	// This does not stop the agent; use AgentLauncher.Stop for that.
	Close() error
}

// SpawnConfig contains the configuration for spawning a new agent instance.
type SpawnConfig struct {
	// Role is the agent's role identifier (e.g., "echo", "coder", "reviewer").
	Role string

	// Workspace is the filesystem path to the agent's workspace directory.
	// If empty, the launcher will create a temporary workspace.
	Workspace string

	// Image is the container image to use for containerized agents (e.g., "ourocodus/echo-agent:latest").
	// Optional for non-containerized agents.
	Image string

	// Command is the command to execute inside the container or as a process.
	// Optional; may use image's default entrypoint if not specified.
	Command []string

	// Args are command-line arguments to pass to the agent.
	Args []string

	// Environment contains environment variables to set for the agent.
	// Keys are variable names, values are variable values.
	Environment map[string]string

	// Credentials contains credential configurations for the agent.
	// Keys are credential types (e.g., "github", "aws"), values are credential paths or configs.
	// The launcher is responsible for mounting or providing these credentials securely.
	Credentials map[string]string

	// Labels are arbitrary key-value pairs for tagging the agent instance.
	// Useful for filtering and organizing agents.
	Labels map[string]string
}
