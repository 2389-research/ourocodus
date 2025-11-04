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

// NATSEventPublisher publishes session lifecycle events to NATS.
// It is safe for concurrent use.
type NATSEventPublisher struct {
	client     nats.Client
	idGen      IDGenerator
	clock      Clock
	logger     Logger
	eventIndex sync.Map // map[sessionID]*atomic.Int64
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
func (p *NATSEventPublisher) PublishSessionCreated(ctx context.Context, sessionID string) error {
	payload := map[string]interface{}{
		"sessionId": sessionID,
		"createdAt": p.clock.Now(),
	}

	return p.publish(ctx, SessionCreated(sessionID), "session.created", sessionID, payload)
}

// PublishSessionTerminated publishes a session.terminated event.
func (p *NATSEventPublisher) PublishSessionTerminated(ctx context.Context, sessionID string) error {
	payload := map[string]interface{}{
		"sessionId":     sessionID,
		"terminatedAt": p.clock.Now(),
	}

	return p.publish(ctx, SessionTerminated(sessionID), "session.terminated", sessionID, payload)
}

// PublishAgentSpawned publishes an agent.spawned event.
func (p *NATSEventPublisher) PublishAgentSpawned(ctx context.Context, sessionID, role, workspace string) error {
	payload := map[string]interface{}{
		"sessionId": sessionID,
		"role":      role,
		"workspace": workspace,
		"spawnedAt": p.clock.Now(),
	}

	return p.publish(ctx, AgentSpawned(sessionID), "agent.spawned", sessionID, payload)
}

// PublishAgentTerminated publishes an agent.terminated event.
func (p *NATSEventPublisher) PublishAgentTerminated(ctx context.Context, sessionID, role string, exitCode int) error {
	payload := map[string]interface{}{
		"sessionId":     sessionID,
		"role":          role,
		"terminatedAt": p.clock.Now(),
		"exitCode":      exitCode,
	}

	return p.publish(ctx, AgentTerminated(sessionID), "agent.terminated", sessionID, payload)
}

// publish publishes an event to NATS.
func (p *NATSEventPublisher) publish(ctx context.Context, subject, eventType, sessionID string, payload interface{}) error {
	now := p.clock.Now()

	event := map[string]interface{}{
		"version":     "1.0",
		"messageId":   p.idGen.Generate(),
		"eventIndex":  p.nextEventIndex(sessionID),
		"occurredAt":  now,
		"publishedAt": now,
		"type":        eventType,
		"payload":     payload,
	}

	// Add correlationId from context if present
	if correlationID := ctx.Value("correlationId"); correlationID != nil {
		event["correlationId"] = correlationID
	}

	data, err := json.Marshal(event)
	if err != nil {
		p.logger.Printf("ERROR: Failed to marshal %s event: %v", eventType, err)
		return fmt.Errorf("marshal event: %w", err)
	}

	// Publish to NATS (pkg/nats.Client handles retry/backoff/metrics)
	if err := p.client.Publish(ctx, subject, data); err != nil {
		p.logger.Printf("ERROR: Failed to publish %s event for session %s: %v", eventType, sessionID, err)
		return err
	}

	return nil
}

// nextEventIndex returns the next event index for a session.
// Event indices are 0-based and monotonically increasing per session.
func (p *NATSEventPublisher) nextEventIndex(sessionID string) int64 {
	// Load or create atomic counter for this session
	val, _ := p.eventIndex.LoadOrStore(sessionID, &atomic.Int64{})
	counter := val.(*atomic.Int64)

	// Increment and return (0-based)
	return counter.Add(1) - 1
}

// Compile-time verification that NATSEventPublisher implements session.EventPublisher
var _ session.EventPublisher = (*NATSEventPublisher)(nil)
