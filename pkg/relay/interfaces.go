package relay

import (
	"context"

	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/gorilla/websocket"
)

// Logger abstracts logging operations
type Logger interface {
	Printf(format string, v ...interface{})
}

// Clock abstracts time operations
type Clock interface {
	Now() string // Returns RFC3339 formatted timestamp
}

// IDGenerator abstracts unique ID generation
type IDGenerator interface {
	Generate() string
}

// WebSocketConn abstracts websocket connection operations
type WebSocketConn interface {
	WriteJSON(v interface{}) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

// Upgrader abstracts WebSocket upgrade operations
type Upgrader interface {
	Upgrade(w interface{}, r interface{}, responseHeader interface{}) (WebSocketConn, error)
}

// SessionManagerInterface abstracts session management operations
// Allows mocking for testing while maintaining dependency injection
type SessionManagerInterface interface {
	// CreateUserSession creates a new user session with WebSocket connection
	CreateUserSession(ctx context.Context, ws session.WebSocketConn) (*session.UserSession, error)

	// SpawnAgent spawns an agent in an existing user session
	SpawnAgent(ctx context.Context, userSessionID, agentID, workspace string) error

	// GetAgent retrieves an agent by user session ID and agent ID
	GetAgent(userSessionID, agentID string) (*session.AgentSession, error)

	// Get retrieves a user session by ID
	Get(userSessionID string) *session.UserSession

	// TerminateAgent terminates a single agent in a user session
	TerminateAgent(ctx context.Context, userSessionID, agentID string) error

	// TerminateUserSession terminates all agents and the session
	TerminateUserSession(ctx context.Context, userSessionID string) error
}

// Ensure gorilla websocket implements our interface
var _ WebSocketConn = (*websocket.Conn)(nil)
