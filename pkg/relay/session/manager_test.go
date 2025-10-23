package session

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- Test Mocks ---

type mockIDGenerator struct {
	nextID string
}

func (m *mockIDGenerator) Generate() string {
	return m.nextID
}

type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

type mockCleaner struct {
	called    int
	mu        sync.Mutex
	shouldErr bool
}

func (m *mockCleaner) Cleanup(ctx context.Context, session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called++
	if m.shouldErr {
		return fmt.Errorf("cleanup error")
	}
	return nil
}

func (m *mockCleaner) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

type mockLogger struct {
	messages []string
	mu       sync.Mutex
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, fmt.Sprintf(format, v...))
}

func (m *mockLogger) MessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

type mockWebSocket struct{}

func (m *mockWebSocket) WriteJSON(v interface{}) error     { return nil }
func (m *mockWebSocket) ReadMessage() (int, []byte, error) { return 0, nil, nil }
func (m *mockWebSocket) Close() error                      { return nil }

type mockACPClient struct{}

func (m *mockACPClient) SendMessage(content string) error { return nil }
func (m *mockACPClient) Close() error                     { return nil }

// --- Test Setup ---

func setupManager() (*Manager, *mockIDGenerator, *mockClock, *mockCleaner, *mockLogger) {
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test-session-id"}
	clock := &mockClock{now: time.Date(2025, 10, 23, 12, 0, 0, 0, time.UTC)}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}

	manager := NewManager(store, idGen, clock, cleaner, logger)
	return manager, idGen, clock, cleaner, logger
}

// --- Tests ---

func TestNewManager_RequiresDependencies(t *testing.T) {
	store := NewMemoryStore()
	idGen := &mockIDGenerator{}
	clock := &mockClock{}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}

	tests := []struct {
		name    string
		store   Store
		idGen   IDGenerator
		clock   Clock
		cleaner Cleaner
		logger  Logger
		panics  bool
	}{
		{"all deps provided", store, idGen, clock, cleaner, logger, false},
		{"nil store", nil, idGen, clock, cleaner, logger, true},
		{"nil idGen", store, nil, clock, cleaner, logger, true},
		{"nil clock", store, idGen, nil, cleaner, logger, true},
		{"nil cleaner", store, idGen, clock, nil, logger, true},
		{"nil logger", store, idGen, clock, cleaner, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.panics && r == nil {
					t.Error("expected panic, got none")
				}
				if !tt.panics && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			NewManager(tt.store, tt.idGen, tt.clock, tt.cleaner, tt.logger)
		})
	}
}

func TestManager_Create(t *testing.T) {
	ctx := context.Background()
	manager, idGen, clock, _, logger := setupManager()

	ws := &mockWebSocket{}
	session, err := manager.Create(ctx, "auth", ws)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify session properties
	if session.GetID() != idGen.nextID {
		t.Errorf("expected ID %s, got %s", idGen.nextID, session.GetID())
	}
	if session.GetAgentID() != "auth" {
		t.Errorf("expected agentID auth, got %s", session.GetAgentID())
	}
	if session.GetState() != StateCreated {
		t.Errorf("expected state CREATED, got %s", session.GetState())
	}
	if !session.GetCreatedAt().Equal(clock.now) {
		t.Errorf("expected createdAt %v, got %v", clock.now, session.GetCreatedAt())
	}

	// Verify handle attached
	handle := session.GetHandle()
	if handle == nil {
		t.Fatal("expected handle to be attached")
	}
	if handle.WebSocket != ws {
		t.Error("expected WebSocket to be set in handle")
	}

	// Verify stored
	if manager.Count() != 1 {
		t.Errorf("expected 1 session in store, got %d", manager.Count())
	}

	// Verify logging
	if logger.MessageCount() == 0 {
		t.Error("expected log message")
	}
}

func TestManager_Create_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	tests := []struct {
		name    string
		agentID string
		ws      WebSocketConn
		errMsg  string
	}{
		{"empty agentID", "", &mockWebSocket{}, "agentID cannot be empty"},
		{"nil websocket", "auth", nil, "websocket connection cannot be nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.Create(ctx, tt.agentID, tt.ws)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.errMsg {
				t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

func TestManager_Create_DuplicateRole(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	ws := &mockWebSocket{}

	// Create first session
	_, err := manager.Create(ctx, "auth", ws)
	if err != nil {
		t.Fatalf("failed to create first session: %v", err)
	}

	// Try to create second session with same role
	_, err = manager.Create(ctx, "auth", ws)
	if err == nil {
		t.Fatal("expected error for duplicate role, got nil")
	}
}

func TestManager_GetAndGetByRole(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)

	// Get by ID
	retrieved := manager.Get(session.GetID())
	if retrieved == nil {
		t.Fatal("expected to retrieve session by ID")
	}
	if retrieved.GetID() != session.GetID() {
		t.Error("retrieved wrong session by ID")
	}

	// Get by role
	retrieved = manager.GetByRole("auth")
	if retrieved == nil {
		t.Fatal("expected to retrieve session by role")
	}
	if retrieved.GetAgentID() != "auth" {
		t.Error("retrieved wrong session by role")
	}

	// Get non-existent
	if manager.Get("nonexistent") != nil {
		t.Error("expected nil for non-existent session")
	}
	if manager.GetByRole("nonexistent") != nil {
		t.Error("expected nil for non-existent role")
	}
}

func TestManager_BeginSpawn(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)

	// Begin spawn
	err := manager.BeginSpawn(ctx, session.GetID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify state transition
	if session.GetState() != StateSpawning {
		t.Errorf("expected state SPAWNING, got %s", session.GetState())
	}
}

func TestManager_AttachAgent(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)
	manager.BeginSpawn(ctx, session.GetID())

	// Attach agent
	acpClient := &mockACPClient{}
	worktree := "/path/to/worktree"
	err := manager.AttachAgent(ctx, session.GetID(), worktree, acpClient)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify state transition
	if session.GetState() != StateActive {
		t.Errorf("expected state ACTIVE, got %s", session.GetState())
	}

	// Verify worktree set
	if session.GetWorktreeDir() != worktree {
		t.Errorf("expected worktree %s, got %s", worktree, session.GetWorktreeDir())
	}

	// Verify ACP client set
	handle := session.GetHandle()
	if handle.ACPClient != acpClient {
		t.Error("expected ACP client to be set in handle")
	}
}

func TestManager_AttachAgent_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)
	manager.BeginSpawn(ctx, session.GetID())

	tests := []struct {
		name      string
		sessionID string
		worktree  string
		client    ACPClient
	}{
		{"nonexistent session", "nonexistent", "/path", &mockACPClient{}},
		{"empty worktree", session.GetID(), "", &mockACPClient{}},
		{"nil client", session.GetID(), "/path", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.AttachAgent(ctx, tt.sessionID, tt.worktree, tt.client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestManager_RecordHeartbeat(t *testing.T) {
	ctx := context.Background()
	manager, _, clock, _, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)

	originalTime := session.GetLastActive()

	// Advance clock
	clock.now = clock.now.Add(5 * time.Second)

	// Record heartbeat
	err := manager.RecordHeartbeat(ctx, session.GetID())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify last active updated
	newTime := session.GetLastActive()
	if !newTime.After(originalTime) {
		t.Error("expected last active to be updated")
	}
	if !newTime.Equal(clock.now) {
		t.Errorf("expected last active %v, got %v", clock.now, newTime)
	}
}

func TestManager_IncrementMessageCount(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)

	// Initial count should be 0
	if session.GetMessageCount() != 0 {
		t.Errorf("expected initial count 0, got %d", session.GetMessageCount())
	}

	// Increment
	manager.IncrementMessageCount(ctx, session.GetID())
	if session.GetMessageCount() != 1 {
		t.Errorf("expected count 1, got %d", session.GetMessageCount())
	}

	// Increment again
	manager.IncrementMessageCount(ctx, session.GetID())
	if session.GetMessageCount() != 2 {
		t.Errorf("expected count 2, got %d", session.GetMessageCount())
	}
}

func TestManager_MarkTerminating(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)
	manager.BeginSpawn(ctx, session.GetID())
	manager.AttachAgent(ctx, session.GetID(), "/path", &mockACPClient{})

	// Mark terminating
	err := manager.MarkTerminating(ctx, session.GetID(), "user requested")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify state transition
	if session.GetState() != StateTerminating {
		t.Errorf("expected state TERMINATING, got %s", session.GetState())
	}
}

func TestManager_MarkTerminating_Idempotent(t *testing.T) {
	ctx := context.Background()
	manager, _, _, _, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)
	manager.BeginSpawn(ctx, session.GetID())
	manager.AttachAgent(ctx, session.GetID(), "/path", &mockACPClient{})

	// Mark terminating multiple times
	manager.MarkTerminating(ctx, session.GetID(), "first call")
	manager.MarkTerminating(ctx, session.GetID(), "second call")
	manager.MarkTerminating(ctx, session.GetID(), "third call")

	// Should still be TERMINATING
	if session.GetState() != StateTerminating {
		t.Errorf("expected state TERMINATING, got %s", session.GetState())
	}
}

func TestManager_CompleteCleanup(t *testing.T) {
	ctx := context.Background()
	manager, _, _, cleaner, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)
	sessionID := session.GetID()

	manager.BeginSpawn(ctx, sessionID)
	manager.AttachAgent(ctx, sessionID, "/path", &mockACPClient{})
	manager.MarkTerminating(ctx, sessionID, "test")

	// Complete cleanup
	err := manager.CompleteCleanup(ctx, sessionID)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify cleaner called
	if cleaner.CallCount() != 1 {
		t.Errorf("expected cleaner called once, got %d", cleaner.CallCount())
	}

	// Verify session removed from store
	if manager.Get(sessionID) != nil {
		t.Error("expected session to be removed from store")
	}
	if manager.Count() != 0 {
		t.Errorf("expected 0 sessions, got %d", manager.Count())
	}
}

func TestManager_CompleteCleanup_Idempotent(t *testing.T) {
	ctx := context.Background()
	manager, _, _, cleaner, _ := setupManager()

	ws := &mockWebSocket{}
	session, _ := manager.Create(ctx, "auth", ws)
	sessionID := session.GetID()

	manager.BeginSpawn(ctx, sessionID)
	manager.AttachAgent(ctx, sessionID, "/path", &mockACPClient{})
	manager.MarkTerminating(ctx, sessionID, "test")

	// Call cleanup multiple times
	manager.CompleteCleanup(ctx, sessionID)
	manager.CompleteCleanup(ctx, sessionID)
	manager.CompleteCleanup(ctx, sessionID)

	// Should only call cleaner once (during first cleanup)
	if cleaner.CallCount() != 1 {
		t.Errorf("expected cleaner called once, got %d", cleaner.CallCount())
	}
}

func TestManager_List(t *testing.T) {
	ctx := context.Background()
	manager, idGen, _, _, _ := setupManager()

	ws := &mockWebSocket{}

	// Create multiple sessions
	idGen.nextID = "session-1"
	session1, _ := manager.Create(ctx, "auth", ws)
	manager.BeginSpawn(ctx, session1.GetID())
	manager.AttachAgent(ctx, session1.GetID(), "/path1", &mockACPClient{})

	idGen.nextID = "session-2"
	_, _ = manager.Create(ctx, "db", ws)

	// List all
	all := manager.List(nil)
	if len(all) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(all))
	}

	// List by state
	activeState := StateActive
	activeFilter := &SessionFilter{State: &activeState}
	active := manager.List(activeFilter)
	if len(active) != 1 {
		t.Errorf("expected 1 active session, got %d", len(active))
	}

	// List by agent ID
	authID := "auth"
	authFilter := &SessionFilter{AgentID: &authID}
	authSessions := manager.List(authFilter)
	if len(authSessions) != 1 {
		t.Errorf("expected 1 auth session, got %d", len(authSessions))
	}
}

func TestManager_ConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	manager, idGen, _, _, _ := setupManager()
	var wg sync.WaitGroup
	ws := &mockWebSocket{}

	// Pre-create sessions sequentially to avoid race on mock ID generator
	roles := []string{"auth", "db", "tests"}
	for i, role := range roles {
		idGen.nextID = fmt.Sprintf("session-%d", i)
		_, _ = manager.Create(ctx, role, ws)
	}

	// Verify all created
	if manager.Count() != 3 {
		t.Errorf("expected 3 sessions, got %d", manager.Count())
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.List(nil)
			manager.Get("session-0")
			manager.GetByRole("auth")
		}()
	}

	wg.Wait()
}
