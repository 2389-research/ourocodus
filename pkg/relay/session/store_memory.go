package session

import (
	"fmt"
	"strings"
	"sync"
)

// Store defines the interface for user session storage
// Implementations can be in-memory (Phase 1) or persistent (future phases)
type Store interface {
	// Create adds a new user session to storage
	// Returns error if user session with same ID already exists
	Create(session *UserSession) error

	// Get retrieves a user session by user session ID
	// Returns nil if not found
	Get(userSessionID string) *UserSession

	// List returns all user sessions, optionally filtered by state
	// Pass nil filter to get all sessions
	List(filter *SessionFilter) []*UserSession

	// Delete removes a user session from storage by user session ID
	// Idempotent - no error if user session doesn't exist
	Delete(userSessionID string)

	// Count returns total number of stored user sessions
	Count() int
}

// SessionFilter defines criteria for filtering user sessions
type SessionFilter struct {
	State *UserSessionState // Filter by state (nil = no filter)
}

// MemoryStore implements Store interface with in-memory storage
// Thread-safe using sync.RWMutex
type MemoryStore struct {
	sessions map[string]*UserSession // user_session_id → UserSession
	mu       sync.RWMutex
}

// NewMemoryStore creates a new in-memory session store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*UserSession),
	}
}

// Create adds a new user session to storage
func (m *MemoryStore) Create(session *UserSession) error {
	if session == nil {
		return ErrSessionNil
	}

	// Validate session ID is not empty or whitespace-only
	if len(session.ID) == 0 || len(session.ID) != len(strings.TrimSpace(session.ID)) {
		return ErrEmptySessionID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate session ID
	if _, exists := m.sessions[session.ID]; exists {
		return fmt.Errorf("%w: %s", ErrSessionDuplicate, session.ID)
	}

	// Store session
	m.sessions[session.ID] = session

	return nil
}

// Get retrieves a user session by user session ID
func (m *MemoryStore) Get(userSessionID string) *UserSession {
	// Return nil for empty or whitespace-only user session IDs
	if len(userSessionID) == 0 || len(userSessionID) != len(strings.TrimSpace(userSessionID)) {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.sessions[userSessionID]
}

// List returns all user sessions matching the filter
func (m *MemoryStore) List(filter *SessionFilter) []*UserSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*UserSession, 0, len(m.sessions))

	for _, session := range m.sessions {
		if m.matchesFilter(session, filter) {
			result = append(result, session)
		}
	}

	return result
}

// matchesFilter checks if session matches filter criteria
// Pure function - no locks needed (caller holds lock)
func (m *MemoryStore) matchesFilter(session *UserSession, filter *SessionFilter) bool {
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

	return true
}

// Delete removes a user session from storage by user session ID
func (m *MemoryStore) Delete(userSessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Idempotent - no error if already deleted
	delete(m.sessions, userSessionID)
}

// Count returns total number of stored user sessions
func (m *MemoryStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.sessions)
}
