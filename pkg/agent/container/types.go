package container

import (
	"time"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
)

// SpawnConfig contains configuration for spawning an agent container.
type SpawnConfig struct {
	// AgentID is the unique identifier for this agent (e.g., "coder-abc123", "reviewer-xyz789")
	AgentID string

	// ImageName is the Docker image to use (e.g., "ourocodus/agent:latest")
	ImageName string

	// Command is the command to run in the container (e.g., []string{"/bin/bash"})
	Command []string

	// Entrypoint overrides the Docker image ENTRYPOINT (optional)
	// If nil, uses the image's ENTRYPOINT. If empty slice, clears the ENTRYPOINT.
	Entrypoint []string

	// GitSSHKey is the SSH private key data for git operations (optional)
	GitSSHKey []byte

	// GitHubToken is the GitHub personal access token for API operations (optional)
	GitHubToken []byte

	// Env is additional environment variables to set in the container (optional)
	Env []string

	// APIKey is the Anthropic API key for agent communication
	APIKey string

	// Labels are custom Docker labels to add to the container (optional)
	// These are merged with default labels (ourocodus.agent=true, agent-id=<id>)
	Labels map[string]string
}

// AgentContainerHandle represents a running agent container with workspace and credentials.
//
// AgentContainerHandle combines:
//   - ContainerSession (Docker container runtime)
//   - AgentWorktree (git workspace isolation)
//   - Credentials (mounted read-only)
//
// Thread-safety: AgentContainerHandle is safe for concurrent read access.
type AgentContainerHandle struct {
	agentID         string
	containerSess   *containersession.ContainerSession
	worktree        *worktree.AgentWorktree
	credentialsPath string
	createdAt       time.Time
}

// AgentID returns the unique identifier for this agent.
func (h *AgentContainerHandle) AgentID() string {
	return h.agentID
}

// ContainerID returns the Docker container ID.
func (h *AgentContainerHandle) ContainerID() string {
	if h.containerSess == nil {
		return ""
	}
	return h.containerSess.ContainerID()
}

// WorkspacePath returns the filesystem path to the git worktree.
func (h *AgentContainerHandle) WorkspacePath() string {
	if h.worktree == nil {
		return ""
	}
	return h.worktree.Path()
}

// BranchName returns the git branch name for this agent's worktree.
func (h *AgentContainerHandle) BranchName() string {
	if h.worktree == nil {
		return ""
	}
	return h.worktree.BranchName()
}

// CredentialsPath returns the filesystem path to the credentials directory.
func (h *AgentContainerHandle) CredentialsPath() string {
	return h.credentialsPath
}

// State returns the current container state.
func (h *AgentContainerHandle) State() containersession.SessionState {
	if h.containerSess == nil {
		return containersession.StateFailed
	}
	return h.containerSess.State()
}

// CreatedAt returns when this handle was created.
func (h *AgentContainerHandle) CreatedAt() time.Time {
	return h.createdAt
}

// ContainerSession returns the underlying container session (for I/O access).
func (h *AgentContainerHandle) ContainerSession() *containersession.ContainerSession {
	return h.containerSess
}

// Worktree returns the underlying agent worktree (for git operations).
func (h *AgentContainerHandle) Worktree() *worktree.AgentWorktree {
	return h.worktree
}
