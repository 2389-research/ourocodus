package relay

import (
	"fmt"
	"strings"
)

// SessionCreated returns the NATS subject for session.created events.
func SessionCreated(sessionID string) string {
	return fmt.Sprintf("sessions.%s.session.created", sanitizeID(sessionID))
}

// SessionTerminated returns the NATS subject for session.terminated events.
func SessionTerminated(sessionID string) string {
	return fmt.Sprintf("sessions.%s.session.terminated", sanitizeID(sessionID))
}

// AgentSpawned returns the NATS subject for agent.spawned events.
func AgentSpawned(sessionID string) string {
	return fmt.Sprintf("sessions.%s.agent.spawned", sanitizeID(sessionID))
}

// AgentTerminated returns the NATS subject for agent.terminated events.
func AgentTerminated(sessionID string) string {
	return fmt.Sprintf("sessions.%s.agent.terminated", sanitizeID(sessionID))
}

// sanitizeID sanitizes a session ID for use in NATS subjects.
// NATS uses dots as subject delimiters, so we replace them with underscores.
func sanitizeID(id string) string {
	// Replace dots with underscores
	sanitized := strings.ReplaceAll(id, ".", "_")

	// Validate length (NATS subjects have ~1KB limit, be conservative)
	if len(sanitized) > 200 {
		panic(fmt.Sprintf("session ID too long for NATS subject: %d chars", len(sanitized)))
	}

	return sanitized
}
