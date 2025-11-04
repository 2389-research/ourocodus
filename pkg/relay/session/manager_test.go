package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- Test Mocks ---

type mockIDGenerator struct {
	nextID string
	count  int
	mu     sync.Mutex
}

func (m *mockIDGenerator) Generate() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++
	return fmt.Sprintf("%s%d", m.nextID, m.count)
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

type mockWebSocket struct {
	mu          sync.Mutex
	closeCalled bool
}

func (m *mockWebSocket) WriteJSON(v interface{}) error     { return nil }
func (m *mockWebSocket) ReadMessage() (int, []byte, error) { return 0, nil, nil }
func (m *mockWebSocket) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled = true
	return nil
}

func (m *mockWebSocket) WasClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCalled
}

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

	// Use current directory as base for tests (allows testdata/ paths)
	manager := NewManager(store, idGen, clock, cleaner, logger, clientFactory, ".", nil) // nil publisher for tests
	return manager, idGen, clock, cleaner, logger, clientFactory
}

// --- Tests ---

func TestCreateUserSession_EmptySession(t *testing.T) {
	manager, _, _, _, logger, _ := setupManager()
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
	// Check that ID starts with the prefix (now has counter suffix)
	expectedID := "test-session-id1"
	if session.GetID() != expectedID {
		t.Errorf("Expected session ID %s, got %s", expectedID, session.GetID())
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
	if agent.GetAgentID() != "auth" {
		t.Errorf("Expected role 'auth', got %s", agent.GetAgentID())
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
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	if err := manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth"); err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	// Try to spawn agent with same role
	err = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth2")
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
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	if err := manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth"); err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	// Terminate the agent
	err = manager.TerminateAgent(ctx, session.GetID(), "auth")
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
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	if err := manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth"); err != nil {
		t.Fatalf("Failed to spawn auth agent: %v", err)
	}
	if err := manager.SpawnAgent(ctx, session.GetID(), "db", "testdata/agent/db"); err != nil {
		t.Fatalf("Failed to spawn db agent: %v", err)
	}
	if err := manager.SpawnAgent(ctx, session.GetID(), "tests", "testdata/agent/tests"); err != nil {
		t.Fatalf("Failed to spawn tests agent: %v", err)
	}

	// Terminate one agent
	err = manager.TerminateAgent(ctx, session.GetID(), "db")
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
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()
	if err := manager.SpawnAgent(ctx, sessionID, "auth", "testdata/agent/auth"); err != nil {
		t.Fatalf("Failed to spawn auth agent: %v", err)
	}
	if err := manager.SpawnAgent(ctx, sessionID, "db", "testdata/agent/db"); err != nil {
		t.Fatalf("Failed to spawn db agent: %v", err)
	}
	if err := manager.SpawnAgent(ctx, sessionID, "tests", "testdata/agent/tests"); err != nil {
		t.Fatalf("Failed to spawn tests agent: %v", err)
	}

	// Terminate user session
	err = manager.TerminateUserSession(ctx, sessionID)
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
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()
	if err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		t.Fatalf("Failed to terminate session: %v", err)
	}

	// Terminate again (should not panic or error)
	err = manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Expected idempotent termination, got error: %v", err)
	}
}

func TestTerminateUserSession_DoesNotCloseWebSocket(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()

	// Verify WebSocket not closed yet
	if ws.WasClosed() {
		t.Fatal("WebSocket should not be closed before termination")
	}

	// Terminate session
	if err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		t.Fatalf("Failed to terminate session: %v", err)
	}

	// Verify WebSocket was NOT closed (server layer owns WebSocket closing)
	if ws.WasClosed() {
		t.Fatal("WebSocket.Close() should NOT be called by session manager (server owns closing)")
	}

	// Verify session was removed from store
	if manager.Get(sessionID) != nil {
		t.Fatal("Session should be removed from store after termination")
	}
}

func TestTerminateAgent_Idempotent(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session, spawn agent, terminate it
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()
	if err := manager.SpawnAgent(ctx, sessionID, "auth", "testdata/agent/auth"); err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}
	if err := manager.TerminateAgent(ctx, sessionID, "auth"); err != nil {
		t.Fatalf("Failed to terminate agent: %v", err)
	}

	// Terminate again (should not panic or error)
	err = manager.TerminateAgent(ctx, sessionID, "auth")
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
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	if err := manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth"); err != nil {
		t.Fatalf("Failed to spawn auth agent: %v", err)
	}
	if err := manager.SpawnAgent(ctx, session.GetID(), "db", "testdata/agent/db"); err != nil {
		t.Fatalf("Failed to spawn db agent: %v", err)
	}

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
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	if err := manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth"); err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	// Get agent
	agent, err := manager.GetAgent(session.GetID(), "auth")
	if err != nil {
		t.Fatalf("Expected no error getting agent, got: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected agent, got nil")
	}
	if agent.GetAgentID() != "auth" {
		t.Errorf("Expected role 'auth', got %s", agent.GetAgentID())
	}
}

func TestRecordHeartbeat(t *testing.T) {
	manager, _, clock, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	oldLastActive := session.GetLastActive()

	// Advance time and record heartbeat
	clock.now = clock.now.Add(5 * time.Second)
	err = manager.RecordHeartbeat(ctx, session.GetID())
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
	if _, err := manager.CreateUserSession(ctx, &mockWebSocket{}); err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}
	if _, err := manager.CreateUserSession(ctx, &mockWebSocket{}); err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}

	// Should have 2 sessions (unique IDs now generated)
	count := manager.Count()
	if count != 2 {
		t.Errorf("Expected 2 sessions after creating sessions, got %d", count)
	}
}

// --- Security Tests ---

func TestSpawnAgent_RejectsPathTraversal(t *testing.T) {
	// Use specific base directory to test prefix bypass
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test-session"}
	clock := &mockClock{now: time.Date(2025, 10, 23, 12, 0, 0, 0, time.UTC)}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}
	clientFactory := &mockClientFactory{}

	// Use "./workspaces" as base to test directory name bypass
	manager := NewManager(store, idGen, clock, cleaner, logger, clientFactory, "./workspaces", nil) // nil publisher for tests

	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()

	tests := []struct {
		name      string
		workspace string
	}{
		{"parent directory traversal", "../../../etc/passwd"},
		{"absolute path outside base", "/tmp/evil"},
		{"traversal within path", "workspace/../../escape"},
		{"traversal with dots", "./workspaces/../../../etc"},
		{"directory name prefix bypass - workspaces2", "workspaces2/hack"},
		{"directory name prefix bypass - workspaces-backup", "workspaces-backup/data"},
		{"directory name prefix bypass - workspaces_evil", "workspaces_evil/../etc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.SpawnAgent(ctx, sessionID, "testRole", tt.workspace)
			if err == nil {
				t.Errorf("Expected error for workspace %q, but got nil", tt.workspace)
			}
			if !strings.Contains(err.Error(), "must be under base directory") {
				t.Errorf("Expected path traversal error, got: %v", err)
			}
		})
	}
}

func TestSpawnAgent_AcceptsValidPaths(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()

	tests := []struct {
		name      string
		workspace string
	}{
		{"relative path under base", "./workspaces/test1"},
		{"another relative path", "workspaces/test2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.SpawnAgent(ctx, sessionID, tt.name, tt.workspace)
			// Should not error due to path validation
			// (May still error due to ACP client creation, but that's expected)
			if err != nil && strings.Contains(err.Error(), "must be under base directory") {
				t.Errorf("Valid workspace path %q was rejected: %v", tt.workspace, err)
			}
		})
	}
}

func TestGetAgentHistory(t *testing.T) {
	manager, _, clock, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session and spawn agent
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()

	err = manager.SpawnAgent(ctx, sessionID, "auth", "testdata/agent/auth")
	if err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	// Get agent and add some messages
	agent, err := manager.GetAgent(sessionID, "auth")
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	now := clock.Now()
	agent.AddMessage("user", "Hello", now)
	agent.AddMessage("agent", "Hi there", now)

	// Get history via Manager API
	history, err := manager.GetAgentHistory(sessionID, "auth")
	if err != nil {
		t.Fatalf("Failed to get agent history: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(history))
	}

	if history[0].From != "user" || history[0].Content != "Hello" {
		t.Error("First message incorrect")
	}

	if history[1].From != "agent" || history[1].Content != "Hi there" {
		t.Error("Second message incorrect")
	}
}

func TestGetAgentHistory_SessionNotFound(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()

	_, err := manager.GetAgentHistory("nonexistent", "auth")
	if err == nil {
		t.Fatal("Expected error for nonexistent session")
	}
}

func TestGetAgentHistory_AgentNotFound(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session without spawning agent
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	_, err = manager.GetAgentHistory(session.GetID(), "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent agent")
	}
}
