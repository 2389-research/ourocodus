package session

import "context"

// EventPublisher publishes session lifecycle events to external systems.
// Implementations must be safe for concurrent use and should not block.
type EventPublisher interface {
	// PublishSessionCreated publishes a session.created event.
	PublishSessionCreated(ctx context.Context, sessionID string) error

	// PublishSessionTerminated publishes a session.terminated event.
	PublishSessionTerminated(ctx context.Context, sessionID string) error

	// PublishAgentSpawned publishes an agent.spawned event.
	PublishAgentSpawned(ctx context.Context, sessionID, role, workspace string) error

	// PublishAgentTerminated publishes an agent.terminated event.
	PublishAgentTerminated(ctx context.Context, sessionID, role string, exitCode int) error
}

// NoOpPublisher is an EventPublisher that does nothing.
// Used when event publishing is disabled or unavailable.
type NoOpPublisher struct{}

// NewNoOpPublisher creates a new no-op event publisher.
func NewNoOpPublisher() *NoOpPublisher {
	return &NoOpPublisher{}
}

// PublishSessionCreated does nothing.
func (n *NoOpPublisher) PublishSessionCreated(ctx context.Context, sessionID string) error {
	return nil
}

// PublishSessionTerminated does nothing.
func (n *NoOpPublisher) PublishSessionTerminated(ctx context.Context, sessionID string) error {
	return nil
}

// PublishAgentSpawned does nothing.
func (n *NoOpPublisher) PublishAgentSpawned(ctx context.Context, sessionID, role, workspace string) error {
	return nil
}

// PublishAgentTerminated does nothing.
func (n *NoOpPublisher) PublishAgentTerminated(ctx context.Context, sessionID, role string, exitCode int) error {
	return nil
}
