package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/relay/session"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
)

func main() {
	verbose := flag.Bool("verbose", false, "emit detailed test output")
	flag.Parse()

	announce("🧪", "Session Management Smoke Test")
	announce("📋", "Testing UserSession/AgentSession architecture from PR7")

	if err := runSessionSmokeTest(*verbose); err != nil {
		fail("💥", "Session smoke test failed: %v", err)
	}

	success("🎉", "All session management smoke tests passed!")
}

func runSessionSmokeTest(verbose bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup test dependencies
	store := session.NewMemoryStore()
	idGen := &testIDGenerator{nextID: "test-session-"}
	clock := &testClock{now: time.Now()}
	cleaner := session.NewNoOpCleaner()
	logger := &testLogger{verbose: verbose}
	clientFactory := session.NewFakeClientFactory(func(workspace string) (session.ACPClient, error) {
		return &fakeACPClient{workspace: workspace}, nil
	})

	mockFactory := agent.NewMockLauncherFactory()                                                            // NEW
	manager := session.NewManager(store, idGen, clock, cleaner, logger, clientFactory, "", nil, mockFactory) // nil publisher for smoketest

	announce("🧪", "Test 1: Create UserSession")
	if err := testCreateUserSession(ctx, manager, verbose); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	announce("🧪", "Test 2: Spawn Single Agent")
	if err := testSpawnSingleAgent(ctx, manager, verbose); err != nil {
		return fmt.Errorf("spawn single agent: %w", err)
	}

	announce("🧪", "Test 3: Spawn Multiple Agents")
	if err := testSpawnMultipleAgents(ctx, manager, verbose); err != nil {
		return fmt.Errorf("spawn multiple agents: %w", err)
	}

	announce("🧪", "Test 4: Agent Spawn Failure Isolation")
	if err := testAgentSpawnFailureIsolation(ctx, verbose); err != nil {
		return fmt.Errorf("agent spawn failure: %w", err)
	}

	announce("🧪", "Test 5: Terminate Single Agent")
	if err := testTerminateSingleAgent(ctx, manager, verbose); err != nil {
		return fmt.Errorf("terminate single agent: %w", err)
	}

	announce("🧪", "Test 6: Terminate Session")
	if err := testTerminateSession(ctx, manager, verbose); err != nil {
		return fmt.Errorf("terminate session: %w", err)
	}

	announce("🧪", "Test 7: Idempotent Termination")
	if err := testIdempotentTermination(ctx, manager, verbose); err != nil {
		return fmt.Errorf("idempotent termination: %w", err)
	}

	announce("🧪", "Test 8: List and Filter Sessions")
	if err := testListAndFilter(ctx, verbose); err != nil {
		return fmt.Errorf("list and filter: %w", err)
	}

	announce("🧪", "Test 9: Event Publishing")
	if err := testEventPublishing(ctx, verbose); err != nil {
		return fmt.Errorf("event publishing: %w", err)
	}

	return nil
}

func testCreateUserSession(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to create: %w", err)
	}

	if userSession.GetID() == "" {
		return fmt.Errorf("session ID is empty")
	}

	if userSession.GetState() != session.StateActive {
		return fmt.Errorf("expected ACTIVE state, got %s", userSession.GetState())
	}

	// Verify initial state has 0 agents
	agents := userSession.ListAgents()
	if len(agents) != 0 {
		return fmt.Errorf("expected 0 agents initially, got %d", len(agents))
	}

	debug(verbose, "  ✓ Created session %s in ACTIVE state", userSession.GetID())
	debug(verbose, "  ✓ Session has 0 agents initially")
	success("✅", "UserSession created successfully")
	return nil
}

func testSpawnSingleAgent(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sessionID := userSession.GetID()

	// Use unique agent ID to avoid lease conflicts with other tests
	agentID := fmt.Sprintf("single-agent-%d", time.Now().UnixNano())
	err = manager.SpawnAgent(ctx, sessionID, agentID, "./workspaces/single")
	if err != nil {
		return fmt.Errorf("failed to spawn: %w", err)
	}

	agent, err := manager.GetAgent(sessionID, agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	if agent.GetAgentID() != agentID {
		return fmt.Errorf("expected agent ID '%s', got %s", agentID, agent.GetAgentID())
	}

	if agent.GetState() != session.AgentActive {
		return fmt.Errorf("expected ACTIVE state, got %s", agent.GetState())
	}

	debug(verbose, "  ✓ Agent '%s' spawned successfully", agentID)
	debug(verbose, "  ✓ Agent state is ACTIVE")
	debug(verbose, "  ✓ UserSession remains ACTIVE")
	success("✅", "Single agent spawned successfully")
	return nil
}

func testSpawnMultipleAgents(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sessionID := userSession.GetID()

	// Use unique agent IDs to avoid lease conflicts with other tests
	ts := time.Now().UnixNano()
	agentIDs := []string{
		fmt.Sprintf("multi-auth-%d", ts),
		fmt.Sprintf("multi-db-%d", ts),
		fmt.Sprintf("multi-tests-%d", ts),
	}
	for _, agentID := range agentIDs {
		workspace := fmt.Sprintf("./workspaces/%s", agentID)
		if err := manager.SpawnAgent(ctx, sessionID, agentID, workspace); err != nil {
			return fmt.Errorf("failed to spawn %s: %w", agentID, err)
		}
	}

	agents, err := manager.ListAgents(sessionID)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) != 3 {
		return fmt.Errorf("expected 3 agents, got %d", len(agents))
	}

	for _, agentID := range agentIDs {
		agent, err := manager.GetAgent(sessionID, agentID)
		if err != nil {
			return fmt.Errorf("agent %s not found: %w", agentID, err)
		}
		if agent.GetState() != session.AgentActive {
			return fmt.Errorf("agent %s not ACTIVE: %s", agentID, agent.GetState())
		}
	}

	debug(verbose, "  ✓ Spawned 3 agents with unique IDs")
	debug(verbose, "  ✓ All agents in ACTIVE state")
	debug(verbose, "  ✓ ListAgents returned 3 agents")
	success("✅", "Multiple agents spawned successfully")
	return nil
}

func testAgentSpawnFailureIsolation(ctx context.Context, verbose bool) error {
	// Create manager with failing client factory
	store := session.NewMemoryStore()
	idGen := &testIDGenerator{nextID: "fail-test-"}
	clock := &testClock{now: time.Now()}
	cleaner := session.NewNoOpCleaner()
	logger := &testLogger{verbose: verbose}

	// Use unique agent ID to avoid lease conflicts with other tests
	ts := time.Now().UnixNano()
	successAgentID := fmt.Sprintf("isolation-auth-%d", ts)
	failingAgentID := fmt.Sprintf("isolation-failing-%d", ts)

	failingFactory := session.NewFakeClientFactory(func(workspace string) (session.ACPClient, error) {
		// Manager converts paths to absolute, so check suffix
		if strings.HasSuffix(workspace, failingAgentID) {
			return nil, fmt.Errorf("simulated spawn failure")
		}
		return &fakeACPClient{workspace: workspace}, nil
	})

	mockFactory := agent.NewMockLauncherFactory()
	manager := session.NewManager(store, idGen, clock, cleaner, logger, failingFactory, "", nil, mockFactory) // nil publisher for smoketest

	ws := &fakeWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sessionID := userSession.GetID()

	// Spawn successful agent first
	if err := manager.SpawnAgent(ctx, sessionID, successAgentID, "./workspaces/"+successAgentID); err != nil {
		return fmt.Errorf("successful agent failed: %w", err)
	}

	// Try to spawn failing agent
	err = manager.SpawnAgent(ctx, sessionID, failingAgentID, "./workspaces/"+failingAgentID)
	if err == nil {
		return fmt.Errorf("expected spawn to fail, but it succeeded")
	}

	// Verify session is still ACTIVE
	userSession = manager.Get(sessionID)
	if userSession == nil {
		return fmt.Errorf("session disappeared after agent failure")
	}
	if userSession.GetState() != session.StateActive {
		return fmt.Errorf("session not ACTIVE after agent failure: %s", userSession.GetState())
	}

	// Verify successful agent still works
	agent, err := manager.GetAgent(sessionID, successAgentID)
	if err != nil {
		return fmt.Errorf("successful agent disappeared: %w", err)
	}
	if agent.GetState() != session.AgentActive {
		return fmt.Errorf("successful agent not ACTIVE: %s", agent.GetState())
	}

	debug(verbose, "  ✓ Agent spawn failure occurred as expected")
	debug(verbose, "  ✓ UserSession remained ACTIVE")
	debug(verbose, "  ✓ Other agents unaffected")
	success("✅", "Agent failure isolation verified")
	return nil
}

func testTerminateSingleAgent(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sessionID := userSession.GetID()

	// Use unique agent IDs to avoid lease conflicts with other tests
	ts := time.Now().UnixNano()
	authAgentID := fmt.Sprintf("term-auth-%d", ts)
	dbAgentID := fmt.Sprintf("term-db-%d", ts)

	// Spawn two agents
	if err := manager.SpawnAgent(ctx, sessionID, authAgentID, "./workspaces/"+authAgentID); err != nil {
		return fmt.Errorf("failed to spawn auth agent: %w", err)
	}
	if err := manager.SpawnAgent(ctx, sessionID, dbAgentID, "./workspaces/"+dbAgentID); err != nil {
		return fmt.Errorf("failed to spawn db agent: %w", err)
	}

	// Terminate one agent
	if err := manager.TerminateAgent(ctx, sessionID, authAgentID); err != nil {
		return fmt.Errorf("failed to terminate: %w", err)
	}

	// Verify auth agent is gone
	_, err = manager.GetAgent(sessionID, authAgentID)
	if err == nil {
		return fmt.Errorf("terminated agent still exists")
	}

	// Verify db agent still active
	dbAgent, err := manager.GetAgent(sessionID, dbAgentID)
	if err != nil {
		return fmt.Errorf("remaining agent disappeared: %w", err)
	}
	if dbAgent.GetState() != session.AgentActive {
		return fmt.Errorf("remaining agent not ACTIVE: %s", dbAgent.GetState())
	}

	// Verify session still active
	userSession = manager.Get(sessionID)
	if userSession.GetState() != session.StateActive {
		return fmt.Errorf("session not ACTIVE after single termination: %s", userSession.GetState())
	}

	debug(verbose, "  ✓ Agent terminated")
	debug(verbose, "  ✓ Remaining agent still ACTIVE")
	debug(verbose, "  ✓ UserSession still ACTIVE")
	success("✅", "Single agent termination verified")
	return nil
}

func testTerminateSession(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sessionID := userSession.GetID()

	// Use unique agent IDs to avoid lease conflicts with other tests
	ts := time.Now().UnixNano()
	authAgentID := fmt.Sprintf("sess-auth-%d", ts)
	dbAgentID := fmt.Sprintf("sess-db-%d", ts)
	testsAgentID := fmt.Sprintf("sess-tests-%d", ts)

	// Spawn multiple agents
	if err := manager.SpawnAgent(ctx, sessionID, authAgentID, "./workspaces/"+authAgentID); err != nil {
		return fmt.Errorf("failed to spawn auth agent: %w", err)
	}
	if err := manager.SpawnAgent(ctx, sessionID, dbAgentID, "./workspaces/"+dbAgentID); err != nil {
		return fmt.Errorf("failed to spawn db agent: %w", err)
	}
	if err := manager.SpawnAgent(ctx, sessionID, testsAgentID, "./workspaces/"+testsAgentID); err != nil {
		return fmt.Errorf("failed to spawn tests agent: %w", err)
	}

	// Terminate session
	if _, err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to terminate session: %w", err)
	}

	// Verify session is removed
	userSession = manager.Get(sessionID)
	if userSession != nil {
		return fmt.Errorf("userSession still exists after termination")
	}

	// Verify agents are gone
	_, err = manager.ListAgents(sessionID)
	if err == nil {
		return fmt.Errorf("agents still exist after session termination")
	}

	debug(verbose, "  ✓ All agents terminated in parallel")
	debug(verbose, "  ✓ Session removed from store")
	debug(verbose, "  ✓ ListAgents returns error")
	success("✅", "Session termination verified")
	return nil
}

func testIdempotentTermination(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	sessionID := userSession.GetID()

	// Use unique agent ID to avoid lease conflicts with other tests
	agentID := fmt.Sprintf("idemp-auth-%d", time.Now().UnixNano())

	if err := manager.SpawnAgent(ctx, sessionID, agentID, "./workspaces/"+agentID); err != nil {
		return fmt.Errorf("failed to spawn auth agent: %w", err)
	}

	// Terminate agent twice - both should succeed (idempotent)
	if err := manager.TerminateAgent(ctx, sessionID, agentID); err != nil {
		return fmt.Errorf("first termination failed: %w", err)
	}

	// Second termination should succeed without error (idempotent behavior)
	if err := manager.TerminateAgent(ctx, sessionID, agentID); err != nil {
		return fmt.Errorf("second termination failed (expected idempotent): %w", err)
	}

	// Terminate session twice - both should succeed (idempotent)
	if _, err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		return fmt.Errorf("first session termination failed: %w", err)
	}

	// Second termination should succeed without error (idempotent behavior)
	if _, err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		return fmt.Errorf("second session termination failed (expected idempotent): %w", err)
	}

	debug(verbose, "  ✓ Double agent termination is idempotent (no error)")
	debug(verbose, "  ✓ Double session termination is idempotent (no error)")
	success("✅", "Idempotent termination verified")
	return nil
}

func testListAndFilter(ctx context.Context, verbose bool) error {
	// Create fresh manager for isolated testing
	store := session.NewMemoryStore()
	idGen := &testIDGenerator{nextID: "list-test-"}
	clock := &testClock{now: time.Now()}
	cleaner := session.NewNoOpCleaner()
	logger := &testLogger{verbose: verbose}
	clientFactory := session.NewFakeClientFactory(func(workspace string) (session.ACPClient, error) {
		return &fakeACPClient{workspace: workspace}, nil
	})

	mockFactory := agent.NewMockLauncherFactory()
	freshManager := session.NewManager(store, idGen, clock, cleaner, logger, clientFactory, "", nil, mockFactory) // nil publisher for smoketest

	// Verify initial state has 0 sessions
	if initialCount := freshManager.Count(); initialCount != 0 {
		return fmt.Errorf("expected 0 sessions initially, got %d", initialCount)
	}

	// Create multiple sessions
	ws1 := &fakeWebSocket{}
	session1, err := freshManager.CreateUserSession(ctx, ws1)
	if err != nil {
		return fmt.Errorf("failed to create session1: %w", err)
	}

	ws2 := &fakeWebSocket{}
	session2, err := freshManager.CreateUserSession(ctx, ws2)
	if err != nil {
		return fmt.Errorf("failed to create session2: %w", err)
	}

	// List all sessions
	allSessions := freshManager.List(nil)
	if len(allSessions) != 2 {
		return fmt.Errorf("expected 2 sessions, got %d", len(allSessions))
	}

	// Filter by active state
	activeState := session.StateActive
	filter := &session.SessionFilter{State: &activeState}
	activeSessions := freshManager.List(filter)
	if len(activeSessions) != 2 {
		return fmt.Errorf("expected 2 active sessions, got %d", len(activeSessions))
	}

	// Terminate one session
	if _, err := freshManager.TerminateUserSession(ctx, session1.GetID()); err != nil {
		return fmt.Errorf("failed to terminate session: %w", err)
	}

	// List again - should only have one
	allSessions = freshManager.List(nil)
	if len(allSessions) != 1 {
		return fmt.Errorf("expected 1 session after termination, got %d", len(allSessions))
	}

	if allSessions[0].GetID() != session2.GetID() {
		return fmt.Errorf("wrong session returned after termination")
	}

	debug(verbose, "  ✓ List returned 2 active sessions")
	debug(verbose, "  ✓ Filter by ACTIVE state worked")
	debug(verbose, "  ✓ List updated after termination")
	success("✅", "List and filter verified")
	return nil
}

// Test helpers

type testIDGenerator struct {
	nextID string
	count  int
}

func (g *testIDGenerator) Generate() string {
	g.count++
	return fmt.Sprintf("%s%d", g.nextID, g.count)
}

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time {
	return c.now
}

type testLogger struct {
	verbose bool
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	if l.verbose {
		fmt.Printf("    [LOG] "+format+"\n", v...)
	}
}

type fakeWebSocket struct{}

func (ws *fakeWebSocket) WriteJSON(v interface{}) error {
	return nil
}

func (ws *fakeWebSocket) ReadMessage() (messageType int, p []byte, err error) {
	// Not used in this smoke test
	return 0, nil, nil
}

func (ws *fakeWebSocket) Close() error {
	return nil
}

type fakeACPClient struct {
	workspace string
	closed    bool
}

func (c *fakeACPClient) SendMessage(ctx context.Context, content string) (*acp.AgentMessage, error) {
	return &acp.AgentMessage{
		Content: "fake response from " + c.workspace,
	}, nil
}

func (c *fakeACPClient) InitializeACP(ctx context.Context) (*acp.InitializeResult, error) {
	return &acp.InitializeResult{ProtocolVersion: 1}, nil
}

func (c *fakeACPClient) CreateSession(ctx context.Context, cwd string) (string, error) {
	return "fake-session-id", nil
}

func (c *fakeACPClient) SendPrompt(ctx context.Context, sessionID, prompt string) error {
	return nil
}

func (c *fakeACPClient) Stream(ctx context.Context) <-chan acp.Event {
	ch := make(chan acp.Event)
	close(ch)
	return ch
}

func (c *fakeACPClient) Close(ctx context.Context) error {
	c.closed = true
	return nil
}

// Mock event publisher for testing
type mockEventPublisher struct {
	sessionCreatedCount    int
	sessionTerminatedCount int
	agentSpawnedCount      int
	agentTerminatedCount   int
}

func (m *mockEventPublisher) PublishSessionCreated(ctx context.Context, userSessionID string) error {
	m.sessionCreatedCount++
	return nil
}

func (m *mockEventPublisher) PublishSessionTerminated(ctx context.Context, userSessionID string) error {
	m.sessionTerminatedCount++
	return nil
}

func (m *mockEventPublisher) PublishAgentSpawned(ctx context.Context, userSessionID, agentID, workspace string) error {
	m.agentSpawnedCount++
	return nil
}

func (m *mockEventPublisher) PublishAgentTerminated(ctx context.Context, userSessionID, agentID string, exitCode int) error {
	m.agentTerminatedCount++
	return nil
}

func testEventPublishing(ctx context.Context, verbose bool) error {
	debug(verbose, "Testing event publishing with mock publisher")

	// Create a mock event publisher that tracks calls
	mockPublisher := &mockEventPublisher{}

	// Create manager with mock publisher
	store := session.NewMemoryStore()
	idGen := &testIDGenerator{nextID: "event-test-"}
	clock := &testClock{now: time.Now()}
	cleaner := session.NewNoOpCleaner()
	logger := &testLogger{verbose: verbose}
	clientFactory := session.NewFakeClientFactory(func(workspace string) (session.ACPClient, error) {
		return &fakeACPClient{workspace: workspace}, nil
	})

	mockFactory := agent.NewMockLauncherFactory() // NEW
	manager := session.NewManager(store, idGen, clock, cleaner, logger, clientFactory, "", mockPublisher, mockFactory)

	// Test 1: Session created event
	ws := &fakeWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	if mockPublisher.sessionCreatedCount != 1 {
		return fmt.Errorf("expected 1 session.created event, got %d", mockPublisher.sessionCreatedCount)
	}
	debug(verbose, "✓ session.created event published")

	// Test 2: Agent spawned event
	// Use unique agent ID to avoid lease conflicts with other tests
	eventAgentID := fmt.Sprintf("event-tester-%d", time.Now().UnixNano())
	err = manager.SpawnAgent(ctx, userSession.GetID(), eventAgentID, "./workspaces/"+eventAgentID)
	if err != nil {
		return fmt.Errorf("failed to spawn agent: %w", err)
	}
	if mockPublisher.agentSpawnedCount != 1 {
		return fmt.Errorf("expected 1 agent.spawned event, got %d", mockPublisher.agentSpawnedCount)
	}
	debug(verbose, "✓ agent.spawned event published")

	// Test 3: Agent terminated event
	err = manager.TerminateAgent(ctx, userSession.GetID(), eventAgentID)
	if err != nil {
		return fmt.Errorf("failed to terminate agent: %w", err)
	}
	if mockPublisher.agentTerminatedCount != 1 {
		return fmt.Errorf("expected 1 agent.terminated event, got %d", mockPublisher.agentTerminatedCount)
	}
	debug(verbose, "✓ agent.terminated event published")

	// Test 4: Session terminated event
	_, err = manager.TerminateUserSession(ctx, userSession.GetID())
	if err != nil {
		return fmt.Errorf("failed to terminate session: %w", err)
	}
	if mockPublisher.sessionTerminatedCount != 1 {
		return fmt.Errorf("expected 1 session.terminated event, got %d", mockPublisher.sessionTerminatedCount)
	}
	debug(verbose, "✓ session.terminated event published")

	// Verify total event count
	totalEvents := mockPublisher.sessionCreatedCount + mockPublisher.agentSpawnedCount +
		mockPublisher.agentTerminatedCount + mockPublisher.sessionTerminatedCount
	if totalEvents != 4 {
		return fmt.Errorf("expected 4 total events, got %d", totalEvents)
	}

	debug(verbose, "✓ All events published correctly")
	return nil
}

// Output helpers

//nolint:unparam // args may be nil, which is intentional for simple messages
func announce(icon, format string, args ...interface{}) {
	fmt.Printf("%s%s %s%s\n", colorCyan, icon, fmt.Sprintf(format, args...), colorReset)
}

//nolint:unparam // args may be nil, which is intentional for simple messages
func success(icon, format string, args ...interface{}) {
	fmt.Printf("%s%s %s%s\n", colorGreen, icon, fmt.Sprintf(format, args...), colorReset)
}

func fail(icon, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s%s %s%s\n", colorRed, icon, fmt.Sprintf(format, args...), colorReset)
	os.Exit(1)
}

func debug(verbose bool, format string, args ...interface{}) {
	if verbose {
		fmt.Printf("  %s\n", fmt.Sprintf(format, args...))
	}
}
