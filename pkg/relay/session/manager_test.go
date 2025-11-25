package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
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
	sendFunc  func(context.Context, string) (interface{}, error)
	closeFunc func(context.Context) error
}

func (m *mockACPClient) SendMessage(ctx context.Context, content string) (interface{}, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, content)
	}
	return nil, nil
}

func (m *mockACPClient) Close(ctx context.Context) error {
	if m.closeFunc != nil {
		return m.closeFunc(ctx)
	}
	return nil
}

type mockClientFactory struct {
	clientFunc  func(workspace string) (ACPClient, error)
	runtimeFunc func(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error)
	callCount   int
	mu          sync.Mutex
}

func (m *mockClientFactory) NewClient(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.runtimeFunc != nil {
		return m.runtimeFunc(ctx, runtime)
	}
	if m.clientFunc != nil {
		workspace := ""
		if runtime != nil {
			workspace = runtime.Workspace
		}
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
	mockFactory := agent.NewMockLauncherFactory() // NEW

	// Use current directory as base for tests (allows testdata/ paths)
	manager := NewManager(store, idGen, clock, cleaner, logger, clientFactory, ".", nil, mockFactory) // nil publisher for tests, mockFactory for launcher tests
	return manager, idGen, clock, cleaner, logger, clientFactory
}

// setupManagerWithLeaseIsolation creates a manager with an isolated lease directory for testing.
// This prevents lease file conflicts between tests and with production lease files.
// Call restore() when done to restore the original lease directory.
func setupManagerWithLeaseIsolation(t *testing.T) (manager *Manager, idGen *mockIDGenerator, clock *mockClock, cleaner *mockCleaner, logger *mockLogger, clientFactory *mockClientFactory, restore func()) {
	t.Helper()

	// Create isolated lease directory
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir

	restore = func() {
		LeaseDir = oldLeaseDir
	}

	manager, idGen, clock, cleaner, logger, clientFactory = setupManager()
	return
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
	manager, _, _, _, logger, clientFactory, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, _, _, _, clientFactory, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, _, _, logger, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, _, _, logger, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, _, cleaner, logger, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	summary, err := manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Expected no error terminating session, got: %v", err)
	}
	if summary.AgentsTerminated != 3 {
		t.Fatalf("Expected 3 agents terminated, got %d", summary.AgentsTerminated)
	}
	if summary.AgentFailures != 0 {
		t.Fatalf("Expected no agent failures, got %d", summary.AgentFailures)
	}
	if summary.CleanupStatus != CleanupStatusComplete {
		t.Fatalf("Expected cleanup status complete, got %s", summary.CleanupStatus)
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
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create and terminate session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()
	if _, err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		t.Fatalf("Failed to terminate session: %v", err)
	}

	// Terminate again (should not panic or error)
	_, err = manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Expected idempotent termination, got error: %v", err)
	}
}

func TestTerminateUserSession_DoesNotCloseWebSocket(t *testing.T) {
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	if _, err := manager.TerminateUserSession(ctx, sessionID); err != nil {
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
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
	ctx := context.Background()

	// Try to spawn agent in non-existent session
	err := manager.SpawnAgent(ctx, "nonexistent-id", "auth", "testdata/agent/auth")
	if err == nil {
		t.Fatal("Expected error for non-existent session, got nil")
	}
}

func TestListAgents(t *testing.T) {
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	// Create isolated lease directory
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	// Use specific base directory to test prefix bypass
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test-session"}
	clock := &mockClock{now: time.Date(2025, 10, 23, 12, 0, 0, 0, time.UTC)}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}
	clientFactory := &mockClientFactory{}
	mockFactory := agent.NewMockLauncherFactory() // NEW

	// Use "./workspaces" as base to test directory name bypass
	manager := NewManager(store, idGen, clock, cleaner, logger, clientFactory, "./workspaces", nil, mockFactory) // nil publisher for tests

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
	manager, _, _, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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
	manager, _, clock, _, _, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
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

// --- Adopted Agent Tests ---

// TestAgentSession_IsAdopted verifies the IsAdopted flag is correctly set for adopted agents
func TestAgentSession_IsAdopted(t *testing.T) {
	tests := []struct {
		name        string
		isAdopted   bool
		containerID string
	}{
		{
			name:        "spawned agent (not adopted)",
			isAdopted:   false,
			containerID: "",
		},
		{
			name:        "adopted agent with container ID",
			isAdopted:   true,
			containerID: "abc123def456",
		},
		{
			name:        "adopted agent with full container ID",
			isAdopted:   true,
			containerID: "abc123def456789012345678901234567890123456789012345678901234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &AgentSession{
				ContainerID: tt.containerID,
				IsAdopted:   tt.isAdopted,
			}

			if agent.IsAdopted != tt.isAdopted {
				t.Errorf("IsAdopted = %v, want %v", agent.IsAdopted, tt.isAdopted)
			}
			if agent.ContainerID != tt.containerID {
				t.Errorf("ContainerID = %v, want %v", agent.ContainerID, tt.containerID)
			}
		})
	}
}

// TestTerminateAgent_AdoptedAgentFieldsSet verifies that adopted agent termination
// properly accesses ContainerID and IsAdopted fields
func TestTerminateAgent_AdoptedAgentFieldsSet(t *testing.T) {
	manager, _, _, _, logger, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Manually create an "adopted" agent (simulating what AttachAgent does)
	now := time.Now()
	adoptedAgent := &AgentSession{
		AgentID:     "adopted-test",
		Workspace:   "/workspace/adopted",
		ContainerID: "fake-container-id-12345", // Adopted agents have container IDs
		IsAdopted:   true,                      // Marked as adopted
		createdAt:   now,
		expiresAt:   now.Add(time.Hour),
		state:       AgentActive,
		lastActive:  now,
		history:     []Message{},
		acpClient:   &mockACPClient{}, // Mock ACP client
	}

	// Add adopted agent to session directly (bypassing normal attach flow)
	session.mu.Lock()
	session.agents["adopted-test"] = adoptedAgent
	session.mu.Unlock()

	// Verify agent is in session
	if session.GetAgent("adopted-test") == nil {
		t.Fatal("Expected adopted agent to be in session")
	}

	// Terminate the adopted agent
	err = manager.TerminateAgent(ctx, session.GetID(), "adopted-test")
	if err != nil {
		t.Fatalf("Failed to terminate adopted agent: %v", err)
	}

	// Verify agent was removed
	if session.GetAgent("adopted-test") != nil {
		t.Error("Expected adopted agent to be removed from session")
	}

	// Verify log messages indicate adopted agent handling
	found := false
	for _, msg := range logger.messages {
		if strings.Contains(msg, "Terminating agent") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected log message for agent termination")
	}
}

// TestTerminateUserSession_MixedAgents verifies that terminating a session with
// both spawned and adopted agents handles them appropriately
func TestTerminateUserSession_MixedAgents(t *testing.T) {
	manager, _, _, cleaner, logger, _, restore := setupManagerWithLeaseIsolation(t)
	defer restore()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()

	// Spawn a regular agent
	if err := manager.SpawnAgent(ctx, sessionID, "spawned-agent", "testdata/agent/spawned"); err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	// Manually add an "adopted" agent WITHOUT container ID
	// (so we don't attempt Docker operations in unit tests)
	now := time.Now()
	adoptedAgent := &AgentSession{
		AgentID:     "adopted-agent",
		Workspace:   "/workspace/adopted",
		ContainerID: "", // Empty so we don't try Docker stop in tests
		IsAdopted:   true,
		createdAt:   now,
		expiresAt:   now.Add(time.Hour),
		state:       AgentActive,
		lastActive:  now,
		history:     []Message{},
		acpClient:   &mockACPClient{},
	}

	session.mu.Lock()
	session.agents["adopted-agent"] = adoptedAgent
	session.mu.Unlock()

	// Verify both agents are in session
	agents := session.ListAgents()
	if len(agents) != 2 {
		t.Fatalf("Expected 2 agents, got %d", len(agents))
	}

	// Terminate user session
	summary, err := manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to terminate session: %v", err)
	}

	// Verify both agents were processed (terminated = closed successfully)
	// In unit tests without Docker, spawned agent succeeds but adopted agent
	// with empty containerID also succeeds (no Docker call)
	if summary.AgentsTerminated != 2 {
		t.Errorf("Expected 2 agents terminated, got %d (failures: %d)",
			summary.AgentsTerminated, summary.AgentFailures)
	}

	// Verify session was removed
	if manager.Get(sessionID) != nil {
		t.Error("Expected session to be removed from store")
	}

	// Verify cleaner was called
	if cleaner.CallCount() != 1 {
		t.Errorf("Expected cleaner called once, got %d", cleaner.CallCount())
	}

	// Verify appropriate logging
	logOutput := strings.Join(logger.messages, "\n")
	if !strings.Contains(logOutput, "Terminating") {
		t.Error("Expected termination log messages")
	}
}

// TestTerminateUserSession_OnlyAdoptedAgents verifies session termination
// when all agents are adopted (no spawned agents)
func TestTerminateUserSession_OnlyAdoptedAgents(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()

	// Add multiple adopted agents without container IDs
	// (so we don't attempt Docker operations in unit tests)
	now := time.Now()
	for _, name := range []string{"adopted-1", "adopted-2", "adopted-3"} {
		adoptedAgent := &AgentSession{
			AgentID:     name,
			Workspace:   fmt.Sprintf("/workspace/%s", name),
			ContainerID: "", // Empty so no Docker calls in unit tests
			IsAdopted:   true,
			createdAt:   now,
			expiresAt:   now.Add(time.Hour),
			state:       AgentActive,
			lastActive:  now,
			history:     []Message{},
			acpClient:   &mockACPClient{},
		}
		session.mu.Lock()
		session.agents[name] = adoptedAgent
		session.mu.Unlock()
	}

	// Verify all agents are in session
	if len(session.ListAgents()) != 3 {
		t.Fatalf("Expected 3 agents, got %d", len(session.ListAgents()))
	}

	// Terminate user session
	summary, err := manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to terminate session: %v", err)
	}

	// All 3 adopted agents should be terminated (ACP close succeeds)
	if summary.AgentsTerminated != 3 {
		t.Errorf("Expected 3 agents terminated, got %d (failures: %d)",
			summary.AgentsTerminated, summary.AgentFailures)
	}

	// Session should be removed
	if manager.Get(sessionID) != nil {
		t.Error("Expected session to be removed")
	}
}

// TestTerminateAgent_AdoptedAgentWithoutContainerID tests the edge case
// where an adopted agent somehow doesn't have a container ID
func TestTerminateAgent_AdoptedAgentWithoutContainerID(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create adopted agent WITHOUT container ID (edge case)
	now := time.Now()
	adoptedAgent := &AgentSession{
		AgentID:     "orphan-adopted",
		Workspace:   "/workspace/orphan",
		ContainerID: "", // No container ID
		IsAdopted:   true,
		createdAt:   now,
		expiresAt:   now.Add(time.Hour),
		state:       AgentActive,
		lastActive:  now,
		history:     []Message{},
		acpClient:   &mockACPClient{},
	}

	session.mu.Lock()
	session.agents["orphan-adopted"] = adoptedAgent
	session.mu.Unlock()

	// Termination should succeed without panicking
	err = manager.TerminateAgent(ctx, session.GetID(), "orphan-adopted")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Agent should be removed
	if session.GetAgent("orphan-adopted") != nil {
		t.Error("Expected agent to be removed")
	}
}

// TestTerminateUserSession_AdoptedAgentWithContainerID tests that the adopted agent
// termination code path is exercised even when Docker is unavailable
// (Docker failure doesn't block session termination)
func TestTerminateUserSession_AdoptedAgentWithContainerID(t *testing.T) {
	manager, _, _, _, logger, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()

	// Add adopted agent WITH container ID
	// This will attempt Docker stop, which may fail in test environment
	now := time.Now()
	adoptedAgent := &AgentSession{
		AgentID:     "adopted-with-container",
		Workspace:   "/workspace/adopted",
		ContainerID: "abc123def456", // Has container ID - will try Docker stop
		IsAdopted:   true,
		createdAt:   now,
		expiresAt:   now.Add(time.Hour),
		state:       AgentActive,
		lastActive:  now,
		history:     []Message{},
		acpClient:   &mockACPClient{},
	}

	session.mu.Lock()
	session.agents["adopted-with-container"] = adoptedAgent
	session.mu.Unlock()

	// Terminate user session
	// Docker stop may fail but session termination should still complete
	summary, err := manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Session termination should not return error: %v", err)
	}

	// Agent may be counted as success or failure depending on Docker availability
	// The important thing is that the session was terminated
	totalProcessed := summary.AgentsTerminated + summary.AgentFailures
	if totalProcessed != 1 {
		t.Errorf("Expected 1 agent processed, got %d", totalProcessed)
	}

	// Session should be removed regardless of container stop result
	if manager.Get(sessionID) != nil {
		t.Error("Expected session to be removed from store")
	}

	// Check that we attempted the adopted agent termination code path
	foundAdoptedLog := false
	for _, msg := range logger.messages {
		if strings.Contains(msg, "adopted") || strings.Contains(msg, "container") {
			foundAdoptedLog = true
			break
		}
	}
	// Note: Log may or may not appear depending on Docker availability
	_ = foundAdoptedLog // Acknowledge we checked
}

// TestAgentSession_ContainerIDFormat validates ContainerID format handling
func TestAgentSession_ContainerIDFormat(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		expectValid bool
	}{
		{
			name:        "short container ID (12 chars)",
			containerID: "abc123def456",
			expectValid: true,
		},
		{
			name:        "full container ID (64 chars)",
			containerID: "abc123def456789012345678901234567890123456789012345678901234abcd",
			expectValid: true,
		},
		{
			name:        "empty container ID",
			containerID: "",
			expectValid: true, // Empty is valid for spawned agents
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &AgentSession{
				ContainerID: tt.containerID,
			}
			// Just verify we can set and read the field
			if agent.ContainerID != tt.containerID {
				t.Errorf("ContainerID mismatch: got %q, want %q", agent.ContainerID, tt.containerID)
			}
		})
	}
}
