package relay

import (
	"time"

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
func NewSessionManager(logger Logger, clock Clock, idGen IDGenerator) *session.Manager {
	store := session.NewMemoryStore()

	// Adapt relay dependencies to session interfaces
	sessionClock := &SessionClockAdapter{clock: clock}
	sessionIDGen := &SessionIDGenAdapter{idGen: idGen}
	sessionLogger := &SessionLoggerAdapter{logger: logger}

	// Use no-op cleaner for Phase 1
	// Issue #7 will provide real cleanup implementation
	cleaner := session.NewNoOpCleaner()

	return session.NewManager(store, sessionIDGen, sessionClock, cleaner, sessionLogger)
}
