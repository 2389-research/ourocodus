package session

import (
	"context"
	"fmt"
	"time"
)

// IDGenerator abstracts unique ID generation
type IDGenerator interface {
	Generate() string
}

// Clock abstracts time operations for deterministic testing
type Clock interface {
	Now() time.Time
}

// Cleaner abstracts cleanup operations for session termination
// Allows pluggable strategies (no-op for Phase 1, real cleanup later)
type Cleaner interface {
	// Cleanup performs idempotent cleanup of session resources
	// Called during session termination
	Cleanup(ctx context.Context, session *Session) error
}

// Logger abstracts logging operations
type Logger interface {
	Printf(format string, v ...interface{})
}

// Manager coordinates session lifecycle with dependency injection
// Composes Store + StateMachine + Cleaner for testable orchestration
type Manager struct {
	store   Store
	idGen   IDGenerator
	clock   Clock
	cleaner Cleaner
	logger  Logger
}

// NewManager creates a session manager with injected dependencies
func NewManager(store Store, idGen IDGenerator, clock Clock, cleaner Cleaner, logger Logger) *Manager {
	if store == nil {
		panic("store cannot be nil")
	}
	if idGen == nil {
		panic("idGen cannot be nil")
	}
	if clock == nil {
		panic("clock cannot be nil")
	}
	if cleaner == nil {
		panic("cleaner cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}

	return &Manager{
		store:   store,
		idGen:   idGen,
		clock:   clock,
		cleaner: cleaner,
		logger:  logger,
	}
}

// Create creates a new session in CREATED state
// Returns error if session for this agent role already exists
func (m *Manager) Create(ctx context.Context, agentID string, ws WebSocketConn) (*Session, error) {
	// Validate inputs
	if agentID == "" {
		return nil, fmt.Errorf("agentID cannot be empty")
	}
	if ws == nil {
		return nil, fmt.Errorf("websocket connection cannot be nil")
	}

	// Check if session already exists for this role
	if existing := m.store.GetByRole(agentID); existing != nil {
		return nil, fmt.Errorf("session already exists for agent %s (session_id=%s)",
			agentID, existing.GetID())
	}

	// Generate unique ID and create session
	sessionID := m.idGen.Generate()
	now := m.clock.Now()
	session := NewSession(sessionID, agentID, now)

	// Create handle with WebSocket connection
	handle := &Handle{
		WebSocket: ws,
	}

	// Attach handle to session
	session.mu.Lock()
	session.setHandle(handle)
	session.mu.Unlock()

	// Store session
	if err := m.store.Create(session); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	m.logger.Printf("Session created: id=%s agent=%s", sessionID, agentID)
	return session, nil
}

// Get retrieves a session by ID
func (m *Manager) Get(id string) *Session {
	return m.store.Get(id)
}

// GetByRole retrieves a session by agent role
func (m *Manager) GetByRole(agentID string) *Session {
	return m.store.GetByRole(agentID)
}

// List returns all sessions matching the filter
func (m *Manager) List(filter *SessionFilter) []*Session {
	return m.store.List(filter)
}

// BeginSpawn transitions session from CREATED to SPAWNING
// Caller is responsible for actually spawning the ACP process
func (m *Manager) BeginSpawn(ctx context.Context, sessionID string) error {
	session := m.store.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return m.transition(session, EventSpawn, "begin spawn")
}

// AttachAgent attaches ACP client and transitions to ACTIVE
// Called after ACP process successfully spawned
func (m *Manager) AttachAgent(ctx context.Context, sessionID, worktreeDir string, acpClient ACPClient) error {
	session := m.store.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Validate inputs
	if worktreeDir == "" {
		return fmt.Errorf("worktreeDir cannot be empty")
	}
	if acpClient == nil {
		return fmt.Errorf("acpClient cannot be nil")
	}

	// Update session with ACP client and worktree
	session.mu.Lock()
	handle := session.handle
	if handle == nil {
		session.mu.Unlock()
		return fmt.Errorf("session has no handle")
	}
	handle.ACPClient = acpClient
	session.setWorktreeDir(worktreeDir)
	session.setLastActive(m.clock.Now())
	session.mu.Unlock()

	// Transition to ACTIVE
	if err := m.transition(session, EventActivate, "attach agent"); err != nil {
		return err
	}

	m.logger.Printf("Agent attached: session=%s worktree=%s", sessionID, worktreeDir)
	return nil
}

// RecordHeartbeat updates the last activity timestamp
// Used to track session liveness
func (m *Manager) RecordHeartbeat(ctx context.Context, sessionID string) error {
	session := m.store.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.setLastActive(m.clock.Now())
	session.mu.Unlock()

	return nil
}

// IncrementMessageCount increments the message counter
func (m *Manager) IncrementMessageCount(ctx context.Context, sessionID string) error {
	session := m.store.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.incrementMessageCount()
	session.setLastActive(m.clock.Now())
	session.mu.Unlock()

	return nil
}

// MarkTerminating transitions session to TERMINATING state
// Idempotent - safe to call multiple times
func (m *Manager) MarkTerminating(ctx context.Context, sessionID string, reason string) error {
	session := m.store.Get(sessionID)
	if session == nil {
		// Session already cleaned up - this is OK
		m.logger.Printf("Session not found during termination: %s (already cleaned?)", sessionID)
		return nil
	}

	m.logger.Printf("Terminating session: id=%s reason=%s", sessionID, reason)

	// Transition to TERMINATING (idempotent)
	if err := m.transition(session, EventTerminate, reason); err != nil {
		// Log but don't fail - termination should be best-effort
		m.logger.Printf("Transition error during termination: %v", err)
	}

	return nil
}

// CompleteCleanup performs cleanup and transitions to CLEANED state
// Removes session from store after cleanup completes
// Idempotent - safe to call multiple times
func (m *Manager) CompleteCleanup(ctx context.Context, sessionID string) error {
	session := m.store.Get(sessionID)
	if session == nil {
		// Already cleaned up
		return nil
	}

	// Run cleanup hook
	if err := m.cleaner.Cleanup(ctx, session); err != nil {
		m.logger.Printf("Cleanup error for session %s: %v", sessionID, err)
		// Continue with cleanup even if hook fails
	}

	// Transition to CLEANED
	if err := m.transition(session, EventClean, "cleanup complete"); err != nil {
		m.logger.Printf("Transition error during cleanup: %v", err)
		return fmt.Errorf("failed to transition to CLEANED state: %w", err)
	}

	// Remove from store only after successful transition
	m.store.Delete(sessionID)

	m.logger.Printf("Session cleaned: id=%s", sessionID)
	return nil
}

// transition performs a state transition using the pure state machine
// Coordinates mutex access with state machine logic
func (m *Manager) transition(session *Session, event Event, reason string) error {
	session.mu.Lock()
	defer session.mu.Unlock()

	currentState := session.state

	// Compute next state using pure state machine
	nextState, err := NextState(currentState, event)
	if err != nil {
		return fmt.Errorf("transition failed: %w", err)
	}

	// Apply state change
	session.setState(nextState)

	m.logger.Printf("Session transition: id=%s %s → %s (event=%s reason=%s)",
		session.ID, currentState, nextState, event, reason)

	return nil
}

// Count returns total number of sessions
func (m *Manager) Count() int {
	return m.store.Count()
}
