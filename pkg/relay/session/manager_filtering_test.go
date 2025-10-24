package session

import (
	"context"
	"testing"
)

// TestList tests the List method with filtering
func TestList(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	// Create sessions
	session1, _ := manager.CreateUserSession(ctx, &mockWebSocket{})

	// Terminate one session
	manager.TerminateUserSession(ctx, session1.GetID())

	// Create another active session
	idGen := &mockIDGenerator{nextID: "session-2"}
	manager2 := NewManager(NewMemoryStore(), idGen, &mockClock{}, &mockCleaner{}, &mockLogger{}, &mockClientFactory{})
	session2, _ := manager2.CreateUserSession(ctx, &mockWebSocket{})

	// Test list all
	allSessions := manager2.List(nil)
	if len(allSessions) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(allSessions))
	}

	// Test list with filter
	activeState := StateActive
	filter := &SessionFilter{State: &activeState}
	activeSessions := manager2.List(filter)
	if len(activeSessions) != 1 {
		t.Errorf("Expected 1 active session with filter, got %d", len(activeSessions))
	}
	if activeSessions[0].GetID() != session2.GetID() {
		t.Error("Expected session2 in filtered results")
	}

	// Test list with terminated filter (should be empty)
	terminatedState := StateTerminated
	terminatedFilter := &SessionFilter{State: &terminatedState}
	terminatedSessions := manager2.List(terminatedFilter)
	if len(terminatedSessions) != 0 {
		t.Errorf("Expected 0 terminated sessions (removed from store), got %d", len(terminatedSessions))
	}
}
