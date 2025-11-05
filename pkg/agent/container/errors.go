package container

import "errors"

// Sentinel errors for common failure cases.
var (
	// ErrAgentNotFound indicates the specified agent container does not exist.
	ErrAgentNotFound = errors.New("agent container not found")

	// ErrAgentAlreadyExists indicates an agent container already exists for this agent ID.
	ErrAgentAlreadyExists = errors.New("agent container already exists")

	// ErrInvalidAgentID indicates the agentID is empty or invalid.
	ErrInvalidAgentID = errors.New("invalid agent ID")

	// ErrInvalidImageName indicates the image name is empty or invalid.
	ErrInvalidImageName = errors.New("invalid image name")

	// ErrInvalidCommand indicates the command is empty or invalid.
	ErrInvalidCommand = errors.New("invalid command")

	// ErrCredentialSetupFailed indicates credential mounting failed.
	ErrCredentialSetupFailed = errors.New("credential setup failed")

	// ErrWorktreeSetupFailed indicates worktree creation failed.
	ErrWorktreeSetupFailed = errors.New("worktree setup failed")

	// ErrContainerSetupFailed indicates container session creation failed.
	ErrContainerSetupFailed = errors.New("container setup failed")

	// ErrInvalidState indicates the operation is not allowed in the current state.
	ErrInvalidState = errors.New("invalid state")
)
