package session

import (
	"fmt"
	"sync"
)

// Store defines the interface for session storage
// Implementations can be in-memory (Phase 1) or persistent (future phases)
type Store interface {
	// Create adds a new session to storage
	// Returns error if session with same ID already exists
	Create(session *Session) error

	// Get retrieves a session by ID
	// Returns nil if not found
	Get(id string) *Session

	// GetByRole retrieves a session by agent role
	// Returns nil if no session found for that role
	// Phase 1: only one session per role allowed
	GetByRole(agentID string) *Session

	// List returns all sessions, optionally filtered by state
	// Pass nil filter to get all sessions
	List(filter *SessionFilter) []*Session

	// Delete removes a session from storage
	// Idempotent - no error if session doesn't exist
	Delete(id string)

	// Count returns total number of stored sessions
	Count() int
}

// SessionFilter defines criteria for filtering sessions
type SessionFilter struct {
	State   *SessionState // Filter by state (nil = no filter)
	AgentID *string       // Filter by agent role (nil = no filter)
}

// MemoryStore implements Store interface with in-memory storage
// Thread-safe using sync.RWMutex
type MemoryStore struct {
	sessions map[string]*Session // session_id → session
	byRole   map[string]*Session // agent_role → session
	mu       sync.RWMutex
}

// NewMemoryStore creates a new in-memory session store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
		byRole:   make(map[string]*Session),
	}
}

// Create adds a new session to storage
func (m *MemoryStore) Create(session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate session ID
	if _, exists := m.sessions[session.ID]; exists {
		return fmt.Errorf("session with ID %s already exists", session.ID)
	}

	// Check for duplicate agent role
	if existing, hasRole := m.byRole[session.AgentID]; hasRole {
		return fmt.Errorf("session for agent %s already exists (session_id=%s)",
			session.AgentID, existing.ID)
	}

	// Store in both maps
	m.sessions[session.ID] = session
	m.byRole[session.AgentID] = session

	return nil
}

// Get retrieves a session by ID
func (m *MemoryStore) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.sessions[id]
}

// GetByRole retrieves a session by agent role
func (m *MemoryStore) GetByRole(agentID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.byRole[agentID]
}

// List returns all sessions matching the filter
func (m *MemoryStore) List(filter *SessionFilter) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Session

	for _, session := range m.sessions {
		if m.matchesFilter(session, filter) {
			result = append(result, session)
		}
	}

	return result
}

// matchesFilter checks if session matches filter criteria
// Pure function - no locks needed (caller holds lock)
func (m *MemoryStore) matchesFilter(session *Session, filter *SessionFilter) bool {
	if filter == nil {
		return true
	}

	// Filter by state
	if filter.State != nil {
		sessionState := session.GetState()
		if sessionState != *filter.State {
			return false
		}
	}

	// Filter by agent ID
	if filter.AgentID != nil {
		if session.AgentID != *filter.AgentID {
			return false
		}
	}

	return true
}

// Delete removes a session from storage
func (m *MemoryStore) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get session before deleting to clean up byRole index
	session, exists := m.sessions[id]
	if !exists {
		return // Idempotent - already deleted
	}

	// Remove from both maps
	delete(m.sessions, id)
	delete(m.byRole, session.AgentID)
}

// Count returns total number of stored sessions
func (m *MemoryStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.sessions)
}
