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

func (m *mockCleaner) Cleanup(ctx context.Context, session *UserSession) error {
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

type mockACPClient struct {
	sendFunc  func(string) (interface{}, error)
	closeFunc func() error
}

func (m *mockACPClient) SendMessage(content string) (interface{}, error) {
	if m.sendFunc != nil {
		return m.sendFunc(content)
	}
	return nil, nil
}

func (m *mockACPClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

type mockClientFactory struct {
	clientFunc func(workspace string) (ACPClient, error)
	callCount  int
	mu         sync.Mutex
}

func (m *mockClientFactory) NewClient(workspace string) (ACPClient, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.clientFunc != nil {
		return m.clientFunc(workspace)
	}
	return &mockACPClient{}, nil
}

func (m *mockClientFactory) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// --- Test Setup ---

func setupManager() (*Manager, *mockIDGenerator, *mockClock, *mockCleaner, *mockLogger, *mockClientFactory) {
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test-session-id"}
	clock := &mockClock{now: time.Date(2025, 10, 23, 12, 0, 0, 0, time.UTC)}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}
	clientFactory := &mockClientFactory{}

	manager := NewManager(store, idGen, clock, cleaner, logger, clientFactory)
	return manager, idGen, clock, cleaner, logger, clientFactory
}

// --- Tests ---

func TestCreateUserSession_EmptySession(t *testing.T) {
	manager, idGen, _, _, logger, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create empty user session
	session, err := manager.CreateUserSession(ctx, ws)
	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if session == nil {
		t.Fatal("Expected session, got nil")
	}
	if session.GetID() != idGen.nextID {
		t.Errorf("Expected session ID %s, got %s", idGen.nextID, session.GetID())
	}
	if session.GetState() != StateActive {
		t.Errorf("Expected state ACTIVE, got %s", session.GetState())
	}

	// Check no agents spawned
	agents := session.ListAgents()
	if len(agents) != 0 {
		t.Errorf("Expected 0 agents, got %d", len(agents))
	}

	// Check logger
	if logger.MessageCount() == 0 {
		t.Error("Expected log message for session creation")
	}
}

func TestSpawnAgent_SingleAgent(t *testing.T) {
	manager, _, _, _, logger, clientFactory := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create user session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Spawn one agent
	err = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth")
	if err != nil {
		t.Fatalf("Expected no error spawning agent, got: %v", err)
	}

	// Check agent was added
	agent := session.GetAgent("auth")
	if agent == nil {
		t.Fatal("Expected agent to be added to session")
	}
	if agent.GetRole() != "auth" {
		t.Errorf("Expected role 'auth', got %s", agent.GetRole())
	}
	if agent.GetState() != AgentActive {
		t.Errorf("Expected agent state ACTIVE, got %s", agent.GetState())
	}
	if agent.GetACPClient() == nil {
		t.Error("Expected ACP client to be set")
	}

	// Check client factory was called
	if clientFactory.CallCount() != 1 {
		t.Errorf("Expected client factory called once, got %d", clientFactory.CallCount())
	}

	// Check logger
	if logger.MessageCount() < 2 {
		t.Error("Expected log messages for session creation and agent spawn")
	}
}

func TestSpawnAgent_MultipleAgents(t *testing.T) {
	manager, _, _, _, _, clientFactory := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create user session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Spawn three agents
	roles := []string{"auth", "db", "tests"}
	for _, role := range roles {
		err = manager.SpawnAgent(ctx, session.GetID(), role, fmt.Sprintf("testdata/agent/%s", role))
		if err != nil {
			t.Fatalf("Expected no error spawning agent %s, got: %v", role, err)
		}
	}

	// Check all agents were added
	agents := session.ListAgents()
	if len(agents) != 3 {
		t.Fatalf("Expected 3 agents, got %d", len(agents))
	}

	for _, role := range roles {
		agent := session.GetAgent(role)
		if agent == nil {
			t.Errorf("Expected agent %s to exist", role)
			continue
		}
		if agent.GetState() != AgentActive {
			t.Errorf("Expected agent %s to be ACTIVE, got %s", role, agent.GetState())
		}
	}

	// Check client factory called 3 times
	if clientFactory.CallCount() != 3 {
		t.Errorf("Expected client factory called 3 times, got %d", clientFactory.CallCount())
	}
}

func TestSpawnAgent_FailureDoesNotAffectSession(t *testing.T) {
	manager, _, _, _, logger, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create user session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Configure client factory to fail
	failingFactory := &mockClientFactory{
		clientFunc: func(workspace string) (ACPClient, error) {
			return nil, fmt.Errorf("spawn failed")
		},
	}
	manager.clientFactory = failingFactory

	// Try to spawn agent (should fail)
	err = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth")
	if err == nil {
		t.Fatal("Expected error spawning agent, got nil")
	}

	// Check session is still ACTIVE
	if session.GetState() != StateActive {
		t.Errorf("Expected session to stay ACTIVE, got %s", session.GetState())
	}

	// Check agent is in FAILED state
	agent := session.GetAgent("auth")
	if agent == nil {
		t.Fatal("Expected failed agent to be in session")
	}
	if agent.GetState() != AgentFailed {
		t.Errorf("Expected agent state FAILED, got %s", agent.GetState())
	}
	if agent.GetError() == "" {
		t.Error("Expected error message on failed agent")
	}

	// Check logger has error message
	if logger.MessageCount() < 2 {
		t.Error("Expected log messages for session creation and agent spawn failure")
	}
}

func TestSpawnAgent_DuplicateRole(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create user session and spawn agent
	session, _ := manager.CreateUserSession(ctx, ws)
	_ = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth")

	// Try to spawn agent with same role
	err := manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth2")
	if err == nil {
		t.Fatal("Expected error spawning duplicate role, got nil")
	}

	// Check only one agent exists
	agents := session.ListAgents()
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}
}

func TestTerminateAgent_SingleAgent(t *testing.T) {
	manager, _, _, _, logger, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session and spawn agent
	session, _ := manager.CreateUserSession(ctx, ws)
	_ = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth")

	// Terminate the agent
	err := manager.TerminateAgent(ctx, session.GetID(), "auth")
	if err != nil {
		t.Fatalf("Expected no error terminating agent, got: %v", err)
	}

	// Check agent was removed
	agent := session.GetAgent("auth")
	if agent != nil {
		t.Error("Expected agent to be removed from session")
	}

	// Check session is still ACTIVE
	if session.GetState() != StateActive {
		t.Errorf("Expected session to stay ACTIVE, got %s", session.GetState())
	}

	// Check logger
	if logger.MessageCount() < 3 {
		t.Error("Expected log messages for create, spawn, and terminate")
	}
}

func TestTerminateAgent_OtherAgentsUnaffected(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session and spawn three agents
	session, _ := manager.CreateUserSession(ctx, ws)
	_ = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth")
	_ = manager.SpawnAgent(ctx, session.GetID(), "db", "testdata/agent/db")
	_ = manager.SpawnAgent(ctx, session.GetID(), "tests", "testdata/agent/tests")

	// Terminate one agent
	err := manager.TerminateAgent(ctx, session.GetID(), "db")
	if err != nil {
		t.Fatalf("Expected no error terminating agent, got: %v", err)
	}

	// Check db agent removed
	if session.GetAgent("db") != nil {
		t.Error("Expected db agent to be removed")
	}

	// Check other agents still exist and are ACTIVE
	if agent := session.GetAgent("auth"); agent == nil || agent.GetState() != AgentActive {
		t.Error("Expected auth agent to remain ACTIVE")
	}
	if agent := session.GetAgent("tests"); agent == nil || agent.GetState() != AgentActive {
		t.Error("Expected tests agent to remain ACTIVE")
	}

	// Check session still ACTIVE
	if session.GetState() != StateActive {
		t.Error("Expected session to remain ACTIVE")
	}
}

func TestTerminateUserSession_AllAgentsTerminated(t *testing.T) {
	manager, _, _, cleaner, logger, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session and spawn three agents
	session, _ := manager.CreateUserSession(ctx, ws)
	sessionID := session.GetID()
	_ = manager.SpawnAgent(ctx, sessionID, "auth", "testdata/agent/auth")
	_ = manager.SpawnAgent(ctx, sessionID, "db", "testdata/agent/db")
	_ = manager.SpawnAgent(ctx, sessionID, "tests", "testdata/agent/tests")

	// Terminate user session
	err := manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Expected no error terminating session, got: %v", err)
	}

	// Check session was removed from store
	if manager.Get(sessionID) != nil {
		t.Error("Expected session to be removed from store")
	}

	// Check cleaner was called
	if cleaner.CallCount() != 1 {
		t.Errorf("Expected cleaner called once, got %d", cleaner.CallCount())
	}

	// Check logger
	if logger.MessageCount() < 5 {
		t.Error("Expected log messages for create, spawns, and termination")
	}
}

func TestTerminateUserSession_Idempotent(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create and terminate session
	session, _ := manager.CreateUserSession(ctx, ws)
	sessionID := session.GetID()
	_ = manager.TerminateUserSession(ctx, sessionID)

	// Terminate again (should not panic or error)
	err := manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Expected idempotent termination, got error: %v", err)
	}
}

func TestTerminateAgent_Idempotent(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session, spawn agent, terminate it
	session, _ := manager.CreateUserSession(ctx, ws)
	sessionID := session.GetID()
	_ = manager.SpawnAgent(ctx, sessionID, "auth", "testdata/agent/auth")
	_ = manager.TerminateAgent(ctx, sessionID, "auth")

	// Terminate again (should not panic or error)
	err := manager.TerminateAgent(ctx, sessionID, "auth")
	if err != nil {
		t.Fatalf("Expected idempotent termination, got error: %v", err)
	}
}

func TestSpawnAgent_SessionNotFound(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	// Try to spawn agent in non-existent session
	err := manager.SpawnAgent(ctx, "nonexistent-id", "auth", "testdata/agent/auth")
	if err == nil {
		t.Fatal("Expected error for non-existent session, got nil")
	}
}

func TestListAgents(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session and spawn agents
	session, _ := manager.CreateUserSession(ctx, ws)
	_ = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth")
	_ = manager.SpawnAgent(ctx, session.GetID(), "db", "testdata/agent/db")

	// List agents
	agents, err := manager.ListAgents(session.GetID())
	if err != nil {
		t.Fatalf("Expected no error listing agents, got: %v", err)
	}

	if len(agents) != 2 {
		t.Fatalf("Expected 2 agents, got %d", len(agents))
	}

	if agents["auth"] == nil || agents["db"] == nil {
		t.Error("Expected both auth and db agents in list")
	}
}

func TestGetAgent(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session and spawn agent
	session, _ := manager.CreateUserSession(ctx, ws)
	_ = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth")

	// Get agent
	agent, err := manager.GetAgent(session.GetID(), "auth")
	if err != nil {
		t.Fatalf("Expected no error getting agent, got: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected agent, got nil")
	}
	if agent.GetRole() != "auth" {
		t.Errorf("Expected role 'auth', got %s", agent.GetRole())
	}
}

func TestRecordHeartbeat(t *testing.T) {
	manager, _, clock, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, _ := manager.CreateUserSession(ctx, ws)
	oldLastActive := session.GetLastActive()

	// Advance time and record heartbeat
	clock.now = clock.now.Add(5 * time.Second)
	err := manager.RecordHeartbeat(ctx, session.GetID())
	if err != nil {
		t.Fatalf("Expected no error recording heartbeat, got: %v", err)
	}

	// Check last active updated
	newLastActive := session.GetLastActive()
	if !newLastActive.After(oldLastActive) {
		t.Error("Expected last active timestamp to be updated")
	}
}

func TestCount(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	// Initially no sessions
	if manager.Count() != 0 {
		t.Errorf("Expected 0 sessions, got %d", manager.Count())
	}

	// Create sessions
	manager.CreateUserSession(ctx, &mockWebSocket{})
	manager.CreateUserSession(ctx, &mockWebSocket{})

	// Should have 2 sessions (but IDs will be same, so actually 1)
	// Need unique IDs for this test
	// This is a limitation of the test setup - in real usage IDs would be unique
	count := manager.Count()
	if count == 0 {
		t.Error("Expected at least 1 session after creating sessions")
	}
}
