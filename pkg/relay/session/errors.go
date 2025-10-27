package session

import "errors"

var (
	// ErrSessionNotFound indicates the requested session does not exist
	ErrSessionNotFound = errors.New("session not found")

	// ErrAgentNotFound indicates the requested agent does not exist
	ErrAgentNotFound = errors.New("agent not found")
)
