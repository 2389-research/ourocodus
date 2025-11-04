package relay

import (
	"os"
	"time"

	"github.com/2389-research/ourocodus/pkg/nats"
	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// SessionClockAdapter adapts relay.Clock to session.Clock
// Bridges the two packages until they are unified
//
// Note: This adapter parses RFC3339 timestamps on every call.
// The relay package uses string timestamps for JSON serialization in protocol messages,
// while the session package uses time.Time for internal state management.
// This design keeps the relay protocol layer decoupled from internal time handling.
type SessionClockAdapter struct {
	clock Clock
}

// Now implements session.Clock interface
func (a *SessionClockAdapter) Now() time.Time {
	// relay.Clock returns string, parse it
	timestamp := a.clock.Now()
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		// Fall back to current time if parse fails
		return time.Now()
	}
	return t
}

// SessionIDGenAdapter adapts relay.IDGenerator to session.IDGenerator
type SessionIDGenAdapter struct {
	idGen IDGenerator
}

// Generate implements session.IDGenerator interface
func (a *SessionIDGenAdapter) Generate() string {
	return a.idGen.Generate()
}

// SessionLoggerAdapter adapts relay.Logger to session.Logger
type SessionLoggerAdapter struct {
	logger Logger
}

// Printf implements session.Logger interface
func (a *SessionLoggerAdapter) Printf(format string, v ...interface{}) {
	a.logger.Printf(format, v...)
}

// NewSessionManager creates a session.Manager using relay dependencies
// Example of how to wire session management into the relay server
//
// natsClient is optional - if nil, event publishing is disabled.
// Caller is responsible for managing NATS client lifecycle (including graceful drain on shutdown).
func NewSessionManager(logger Logger, clock Clock, idGen IDGenerator, natsClient nats.Client) (*session.Manager, error) {
	store := session.NewMemoryStore()

	// Adapt relay dependencies to session interfaces
	sessionClock := &SessionClockAdapter{clock: clock}
	sessionIDGen := &SessionIDGenAdapter{idGen: idGen}
	sessionLogger := &SessionLoggerAdapter{logger: logger}

	// Use no-op cleaner for Phase 1
	cleaner := session.NewNoOpCleaner()

	// Create ACP client factory (reads ANTHROPIC_API_KEY from environment)
	clientFactory, err := session.NewACPClientFactory()
	if err != nil {
		return nil, err
	}

	// Use base workspace directory from env or default to "./workspaces"
	baseWorkspaceDir := os.Getenv("WORKSPACE_BASE_DIR")
	if baseWorkspaceDir == "" {
		baseWorkspaceDir = "./workspaces"
	}

	// Create event publisher if NATS client is available
	var publisher session.EventPublisher
	if natsClient != nil {
		// Create adapters for NATS publisher dependencies
		publisherIDGen := &SessionIDGenAdapter{idGen: idGen}
		publisherLogger := &SessionLoggerAdapter{logger: logger}

		// Create clock adapter that returns strings (NATSEventPublisher expects Clock interface with Now() string)
		publisherClock := clock // relay.Clock returns string, which is what NATSEventPublisher needs

		publisher = NewNATSEventPublisher(natsClient, publisherIDGen, publisherClock, publisherLogger)
		logger.Printf("NATS event publisher initialized")
	} else {
		logger.Printf("NATS event publishing disabled (no NATS_URL configured)")
	}

	return session.NewManager(store, sessionIDGen, sessionClock, cleaner, sessionLogger, clientFactory, baseWorkspaceDir, publisher), nil
}
