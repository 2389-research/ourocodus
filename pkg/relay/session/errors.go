package session

import "errors"

var (
	// ErrSessionNotFound indicates the requested session does not exist
	ErrSessionNotFound = errors.New("session not found")

	// ErrAgentNotFound indicates the requested agent does not exist
	ErrAgentNotFound = errors.New("agent not found")

	// ErrMissingAnthropicAPIKey is returned when ANTHROPIC_API_KEY environment variable is not set
	ErrMissingAnthropicAPIKey = errors.New("ANTHROPIC_API_KEY environment variable not set")

	// ErrSessionNil is returned when a nil session is passed to Store.Create
	ErrSessionNil = errors.New("session cannot be nil")

	// ErrSessionDuplicate is returned when attempting to create a session with an ID that already exists
	ErrSessionDuplicate = errors.New("session with this ID already exists")

	// ErrEmptySessionID is returned when an empty or whitespace-only session ID is provided
	ErrEmptySessionID = errors.New("session ID cannot be empty or whitespace-only")

	// ErrWebSocketNil is returned when a nil websocket connection is provided
	ErrWebSocketNil = errors.New("websocket connection cannot be nil")

	// ErrEmptyRole is returned when an empty or whitespace-only role is provided
	ErrEmptyRole = errors.New("role cannot be empty or whitespace-only")

	// ErrEmptyWorkspace is returned when an empty or whitespace-only workspace is provided
	ErrEmptyWorkspace = errors.New("workspace cannot be empty or whitespace-only")
)
