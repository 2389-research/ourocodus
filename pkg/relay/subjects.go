package relay

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUserSessionIDTooLong = errors.New("user session ID too long for NATS subject (max 200 chars after sanitization)")

// SessionCreated returns the NATS subject for session.created events.
// Returns an error if the user session ID is too long.
func SessionCreated(userSessionID string) (string, error) {
	sanitized, err := sanitizeID(userSessionID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sessions.%s.session.created", sanitized), nil
}

// SessionTerminated returns the NATS subject for session.terminated events.
// Returns an error if the user session ID is too long.
func SessionTerminated(userSessionID string) (string, error) {
	sanitized, err := sanitizeID(userSessionID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sessions.%s.session.terminated", sanitized), nil
}

// AgentSpawned returns the NATS subject for agent.spawned events.
// Returns an error if the user session ID is too long.
func AgentSpawned(userSessionID string) (string, error) {
	sanitized, err := sanitizeID(userSessionID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sessions.%s.agent.spawned", sanitized), nil
}

// AgentTerminated returns the NATS subject for agent.terminated events.
// Returns an error if the user session ID is too long.
func AgentTerminated(userSessionID string) (string, error) {
	sanitized, err := sanitizeID(userSessionID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sessions.%s.agent.terminated", sanitized), nil
}

// sanitizeID sanitizes a user session ID for use in NATS subjects.
// NATS uses dots as subject delimiters, so we replace them with underscores.
// Returns an error if the sanitized ID exceeds 200 characters.
func sanitizeID(id string) (string, error) {
	// Replace dots with underscores
	sanitized := strings.ReplaceAll(id, ".", "_")

	// Validate length (NATS subjects have ~1KB limit, be conservative)
	if len(sanitized) > 200 {
		return "", fmt.Errorf("%w: %d chars", ErrUserSessionIDTooLong, len(sanitized))
	}

	return sanitized, nil
}
