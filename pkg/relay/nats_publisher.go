package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/2389-research/ourocodus/pkg/nats"
	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const correlationIDKey contextKey = "correlationId"

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// NATSEventPublisher publishes session lifecycle events to NATS.
// It is safe for concurrent use.
type NATSEventPublisher struct {
	client     nats.Client
	idGen      IDGenerator
	clock      Clock
	logger     Logger
	eventIndex sync.Map // map[userSessionID]*atomic.Int64
}

// NewNATSEventPublisher creates a new NATS event publisher.
func NewNATSEventPublisher(client nats.Client, idGen IDGenerator, clock Clock, logger Logger) *NATSEventPublisher {
	return &NATSEventPublisher{
		client: client,
		idGen:  idGen,
		clock:  clock,
		logger: logger,
	}
}

// PublishSessionCreated publishes a session.created event.
func (p *NATSEventPublisher) PublishSessionCreated(ctx context.Context, userSessionID string) error {
	subject, err := SessionCreated(userSessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID for NATS subject: %w", err)
	}

	payload := map[string]interface{}{
		"userSessionId": userSessionID,
		"createdAt":     p.clock.Now(),
	}

	return p.publish(ctx, subject, "session.created", userSessionID, payload)
}

// PublishSessionTerminated publishes a session.terminated event.
// Automatically cleans up the event index counter for the session to prevent memory leaks.
func (p *NATSEventPublisher) PublishSessionTerminated(ctx context.Context, userSessionID string) error {
	subject, err := SessionTerminated(userSessionID)
	if err != nil {
		return fmt.Errorf("invalid user session ID for NATS subject: %w", err)
	}

	payload := map[string]interface{}{
		"userSessionId": userSessionID,
		"terminatedAt":  p.clock.Now(),
	}

	err = p.publish(ctx, subject, "session.terminated", userSessionID, payload)

	// Clean up event index counter after publishing (even if publish failed)
	// This prevents memory leaks in long-running services
	p.CleanupSession(userSessionID)

	return err
}

// PublishAgentSpawned publishes an agent.spawned event.
func (p *NATSEventPublisher) PublishAgentSpawned(ctx context.Context, userSessionID, agentID, workspace string) error {
	subject, err := AgentSpawned(userSessionID)
	if err != nil {
		return fmt.Errorf("invalid user session ID for NATS subject: %w", err)
	}

	payload := map[string]interface{}{
		"userSessionId": userSessionID,
		"agentId":       agentID,
		"workspace":     workspace,
		"spawnedAt":     p.clock.Now(),
	}

	return p.publish(ctx, subject, "agent.spawned", userSessionID, payload)
}

// PublishAgentTerminated publishes an agent.terminated event.
func (p *NATSEventPublisher) PublishAgentTerminated(ctx context.Context, userSessionID, agentID string, exitCode int) error {
	subject, err := AgentTerminated(userSessionID)
	if err != nil {
		return fmt.Errorf("invalid user session ID for NATS subject: %w", err)
	}

	payload := map[string]interface{}{
		"userSessionId": userSessionID,
		"agentId":       agentID,
		"terminatedAt":  p.clock.Now(),
		"exitCode":      exitCode,
	}

	return p.publish(ctx, subject, "agent.terminated", userSessionID, payload)
}

// publish publishes an event to NATS.
func (p *NATSEventPublisher) publish(ctx context.Context, subject, eventType, userSessionID string, payload interface{}) error {
	now := p.clock.Now()

	event := map[string]interface{}{
		"version":     "1.0",
		"messageId":   p.idGen.Generate(),
		"eventIndex":  p.nextEventIndex(userSessionID),
		"occurredAt":  now,
		"publishedAt": now,
		"type":        eventType,
		"payload":     payload,
	}

	// Add correlationId from context if present
	if correlationID := ctx.Value(correlationIDKey); correlationID != nil {
		event["correlationId"] = correlationID
	}

	data, err := json.Marshal(event)
	if err != nil {
		p.logger.Printf("ERROR: Failed to marshal %s event: %v", eventType, err)
		return fmt.Errorf("marshal event: %w", err)
	}

	// Publish to NATS (pkg/nats.Client handles retry/backoff/metrics)
	if err := p.client.Publish(ctx, subject, data); err != nil {
		p.logger.Printf("ERROR: Failed to publish %s event for userSession %s: %v", eventType, userSessionID, err)
		return err
	}

	return nil
}

// nextEventIndex returns the next event index for a user session.
// Event indices are 0-based and monotonically increasing per user session.
func (p *NATSEventPublisher) nextEventIndex(userSessionID string) int64 {
	// Load or create atomic counter for this user session
	val, _ := p.eventIndex.LoadOrStore(userSessionID, &atomic.Int64{})
	counter := val.(*atomic.Int64)

	// Increment and return (0-based)
	return counter.Add(1) - 1
}

// CleanupSession removes the event index counter for a terminated user session.
// This should be called when a user session is terminated to prevent memory leaks.
// It is safe to call this method even if no counter exists for the user session.
func (p *NATSEventPublisher) CleanupSession(userSessionID string) {
	p.eventIndex.Delete(userSessionID)
}

// Compile-time verification that NATSEventPublisher implements session.EventPublisher
var _ session.EventPublisher = (*NATSEventPublisher)(nil)
