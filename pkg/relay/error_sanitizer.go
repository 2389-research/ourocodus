package relay

import (
	"context"
	"errors"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// sanitizeError converts internal errors to user-safe messages that don't leak
// implementation details. Full error details should be logged server-side.
//
// Security considerations:
// - Never expose file paths, function names, or stack traces
// - Never expose build commands or Docker image names
// - Never expose internal configuration or environment variables
// - Provide actionable guidance when possible
func sanitizeError(err error) string {
	if err == nil {
		return "An error occurred"
	}

	// Container-related errors
	switch {
	case errors.Is(err, container.ErrContainerSetupFailed):
		return "Agent container unavailable. Please ensure the system is properly configured."

	case errors.Is(err, container.ErrAgentNotFound):
		return "Agent container not found. The agent may have been terminated."

	case errors.Is(err, container.ErrAgentAlreadyExists):
		return "An agent is already running for this session."

	case errors.Is(err, container.ErrInvalidAgentID):
		return "Invalid agent identifier provided."

	case errors.Is(err, container.ErrInvalidImageName):
		return "Agent container configuration error. Please contact support."

	case errors.Is(err, container.ErrInvalidCommand):
		return "Invalid agent command. Please check your request."

	case errors.Is(err, container.ErrCredentialSetupFailed):
		return "Failed to configure agent credentials. Please try again."

	case errors.Is(err, container.ErrWorktreeSetupFailed):
		return "Failed to prepare workspace. Please try again."

	case errors.Is(err, container.ErrInvalidState):
		return "Operation not allowed in current state. Please try again."

	// Session-related errors
	case errors.Is(err, session.ErrSessionNotFound):
		return "Session not found. Please create a new session."

	case errors.Is(err, session.ErrAgentNotFound):
		return "Agent not found in this session."

	case errors.Is(err, session.ErrMissingAnthropicAPIKey):
		return "API configuration error. Please contact your administrator."

	case errors.Is(err, session.ErrSessionDuplicate):
		return "A session with this ID already exists."

	case errors.Is(err, session.ErrEmptySessionID):
		return "Session ID cannot be empty."

	case errors.Is(err, session.ErrWebSocketNil):
		return "WebSocket connection error. Please reconnect."

	case errors.Is(err, session.ErrEmptyAgentID):
		return "Agent ID cannot be empty."

	case errors.Is(err, session.ErrEmptyWorkspace):
		return "Workspace path cannot be empty."

	case errors.Is(err, session.ErrSessionNil):
		return "Invalid session data provided."

	// Context errors
	case errors.Is(err, context.DeadlineExceeded):
		return "Operation timed out. Please try again."

	case errors.Is(err, context.Canceled):
		return "Operation was canceled. Please try again."

	// Relay-specific errors
	case errors.Is(err, ErrUserSessionIDTooLong):
		return "Session ID is too long. Please use a shorter identifier."

	// Generic fallback - never expose raw error details
	default:
		return "An internal error occurred. Please contact support if this persists."
	}
}
