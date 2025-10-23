package session

import (
	"sync"
	"time"
)

// SessionState represents the lifecycle state of a session
type SessionState string

const (
	// StateCreated indicates session ID generated, WebSocket established
	StateCreated SessionState = "CREATED"

	// StateSpawning indicates git worktree being created, ACP process being spawned
	StateSpawning SessionState = "SPAWNING"

	// StateActive indicates ACP process running, accepting messages
	StateActive SessionState = "ACTIVE"

	// StateTerminating indicates cleanup in progress, resources being freed
	StateTerminating SessionState = "TERMINATING"

	// StateCleaned indicates session removed, no further operations possible
	StateCleaned SessionState = "CLEANED"
)

// String returns the string representation of SessionState
func (s SessionState) String() string {
	return string(s)
}

// IsValid returns true if the state is a recognized SessionState
func (s SessionState) IsValid() bool {
	switch s {
	case StateCreated, StateSpawning, StateActive, StateTerminating, StateCleaned:
		return true
	default:
		return false
	}
}

// Session represents a user conversation with one AI agent
// Immutable after creation except for state transitions through Manager
type Session struct {
	// Immutable fields (set at creation)
	ID      string // UUID v4
	AgentID string // Role: "auth", "db", "tests"

	// Mutable fields (protected by mu)
	state        SessionState
	worktreeDir  string
	handle       *Handle
	createdAt    time.Time
	lastActive   time.Time
	messageCount int

	mu sync.RWMutex
}

// Handle encapsulates runtime resources associated with a session
// Separates connection/process management from session metadata
type Handle struct {
	// WebSocket connection to PWA client
	WebSocket WebSocketConn

	// ACP client wrapper (set during SPAWNING → ACTIVE transition)
	ACPClient ACPClient

	// Context cancel function for graceful shutdown
	CancelFunc func()
}

// WebSocketConn abstracts WebSocket operations
// Matches existing relay.WebSocketConn interface for compatibility
type WebSocketConn interface {
	WriteJSON(v interface{}) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

// ACPClient abstracts ACP process operations
// Will be implemented by pkg/acp.Client
type ACPClient interface {
	SendMessage(content string) error
	Close() error
}

// NewSession creates a new session in CREATED state
// Pure function - no side effects, no I/O
func NewSession(id, agentID string, createdAt time.Time) *Session {
	return &Session{
		ID:         id,
		AgentID:    agentID,
		state:      StateCreated,
		createdAt:  createdAt,
		lastActive: createdAt,
	}
}

// --- Read-only accessors (thread-safe) ---

// GetID returns the session ID (immutable, no lock needed)
func (s *Session) GetID() string {
	return s.ID
}

// GetAgentID returns the agent role (immutable, no lock needed)
func (s *Session) GetAgentID() string {
	return s.AgentID
}

// GetState returns the current session state
func (s *Session) GetState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// GetWorktreeDir returns the git worktree directory path
func (s *Session) GetWorktreeDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.worktreeDir
}

// GetCreatedAt returns the session creation timestamp (immutable)
func (s *Session) GetCreatedAt() time.Time {
	return s.createdAt
}

// GetLastActive returns the last activity timestamp
func (s *Session) GetLastActive() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActive
}

// GetMessageCount returns the total messages sent to agent
func (s *Session) GetMessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messageCount
}

// GetHandle returns the session handle (may be nil if not attached)
func (s *Session) GetHandle() *Handle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handle
}

// --- Package-private mutators (called only by Manager) ---

// setState updates the session state (must hold lock)
func (s *Session) setState(state SessionState) {
	s.state = state
}

// setWorktreeDir updates the worktree directory path (must hold lock)
func (s *Session) setWorktreeDir(dir string) {
	s.worktreeDir = dir
}

// setHandle updates the session handle (must hold lock)
func (s *Session) setHandle(h *Handle) {
	s.handle = h
}

// setLastActive updates the last activity timestamp (must hold lock)
func (s *Session) setLastActive(t time.Time) {
	s.lastActive = t
}

// incrementMessageCount increases message counter (must hold lock)
func (s *Session) incrementMessageCount() {
	s.messageCount++
}
