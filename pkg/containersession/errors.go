package containersession

import "errors"

var (
	// ErrSessionNotFound indicates the requested session does not exist
	ErrSessionNotFound = errors.New("container session not found")

	// ErrSessionAlreadyExists indicates a session with the given ID already exists
	ErrSessionAlreadyExists = errors.New("container session already exists")

	// ErrInvalidSessionID indicates the session ID is empty or invalid
	ErrInvalidSessionID = errors.New("invalid session ID")

	// ErrInvalidWorkspacePath indicates the workspace path is invalid or unsafe
	ErrInvalidWorkspacePath = errors.New("invalid workspace path")

	// ErrContainerNotFound indicates the container does not exist
	ErrContainerNotFound = errors.New("container not found")

	// ErrContainerAlreadyRunning indicates the container is already running
	ErrContainerAlreadyRunning = errors.New("container already running")

	// ErrInvalidState indicates the operation cannot be performed in current state
	ErrInvalidState = errors.New("invalid state for operation")
)
