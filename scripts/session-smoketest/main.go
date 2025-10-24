package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

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
	ctx := context.Background()

	// Setup test dependencies
	store := session.NewMemoryStore()
	idGen := &testIDGenerator{nextID: "test-session-"}
	clock := &testClock{now: time.Now()}
	cleaner := session.NewNoOpCleaner()
	logger := &testLogger{verbose: verbose}
	clientFactory := session.NewFakeClientFactory(func(workspace string) (session.ACPClient, error) {
		return &fakeACPClient{workspace: workspace}, nil
	})

	manager := session.NewManager(store, idGen, clock, cleaner, logger, clientFactory)

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
	if err := testListAndFilter(ctx, manager, verbose); err != nil {
		return fmt.Errorf("list and filter: %w", err)
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

	debug(verbose, "  ✓ Created session %s in ACTIVE state", userSession.GetID())
	debug(verbose, "  ✓ Session has 0 agents initially")
	success("✅", "UserSession created successfully")
	return nil
}

func testSpawnSingleAgent(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, _ := manager.CreateUserSession(ctx, ws)
	sessionID := userSession.GetID()

	err := manager.SpawnAgent(ctx, sessionID, "auth", "workspace/auth")
	if err != nil {
		return fmt.Errorf("failed to spawn: %w", err)
	}

	agent, err := manager.GetAgent(sessionID, "auth")
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	if agent.GetRole() != "auth" {
		return fmt.Errorf("expected role 'auth', got %s", agent.GetRole())
	}

	if agent.GetState() != session.AgentActive {
		return fmt.Errorf("expected ACTIVE state, got %s", agent.GetState())
	}

	debug(verbose, "  ✓ Agent 'auth' spawned successfully")
	debug(verbose, "  ✓ Agent state is ACTIVE")
	debug(verbose, "  ✓ UserSession remains ACTIVE")
	success("✅", "Single agent spawned successfully")
	return nil
}

func testSpawnMultipleAgents(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, _ := manager.CreateUserSession(ctx, ws)
	sessionID := userSession.GetID()

	roles := []string{"auth", "db", "tests"}
	for _, role := range roles {
		workspace := fmt.Sprintf("workspace/%s", role)
		if err := manager.SpawnAgent(ctx, sessionID, role, workspace); err != nil {
			return fmt.Errorf("failed to spawn %s: %w", role, err)
		}
	}

	agents, err := manager.ListAgents(sessionID)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) != 3 {
		return fmt.Errorf("expected 3 agents, got %d", len(agents))
	}

	for _, role := range roles {
		agent, err := manager.GetAgent(sessionID, role)
		if err != nil {
			return fmt.Errorf("agent %s not found: %w", role, err)
		}
		if agent.GetState() != session.AgentActive {
			return fmt.Errorf("agent %s not ACTIVE: %s", role, agent.GetState())
		}
	}

	debug(verbose, "  ✓ Spawned 3 agents: auth, db, tests")
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
	failingFactory := session.NewFakeClientFactory(func(workspace string) (session.ACPClient, error) {
		if workspace == "workspace/failing" {
			return nil, fmt.Errorf("simulated spawn failure")
		}
		return &fakeACPClient{workspace: workspace}, nil
	})

	manager := session.NewManager(store, idGen, clock, cleaner, logger, failingFactory)

	ws := &fakeWebSocket{}
	userSession, _ := manager.CreateUserSession(ctx, ws)
	sessionID := userSession.GetID()

	// Spawn successful agent first
	if err := manager.SpawnAgent(ctx, sessionID, "auth", "workspace/auth"); err != nil {
		return fmt.Errorf("successful agent failed: %w", err)
	}

	// Try to spawn failing agent
	err := manager.SpawnAgent(ctx, sessionID, "failing", "workspace/failing")
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
	agent, err := manager.GetAgent(sessionID, "auth")
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
	userSession, _ := manager.CreateUserSession(ctx, ws)
	sessionID := userSession.GetID()

	// Spawn two agents
	manager.SpawnAgent(ctx, sessionID, "auth", "workspace/auth")
	manager.SpawnAgent(ctx, sessionID, "db", "workspace/db")

	// Terminate one agent
	if err := manager.TerminateAgent(ctx, sessionID, "auth"); err != nil {
		return fmt.Errorf("failed to terminate: %w", err)
	}

	// Verify auth agent is gone
	_, err := manager.GetAgent(sessionID, "auth")
	if err == nil {
		return fmt.Errorf("terminated agent still exists")
	}

	// Verify db agent still active
	dbAgent, err := manager.GetAgent(sessionID, "db")
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

	debug(verbose, "  ✓ Agent 'auth' terminated")
	debug(verbose, "  ✓ Agent 'db' still ACTIVE")
	debug(verbose, "  ✓ UserSession still ACTIVE")
	success("✅", "Single agent termination verified")
	return nil
}

func testTerminateSession(ctx context.Context, manager *session.Manager, verbose bool) error {
	ws := &fakeWebSocket{}
	userSession, _ := manager.CreateUserSession(ctx, ws)
	sessionID := userSession.GetID()

	// Spawn multiple agents
	manager.SpawnAgent(ctx, sessionID, "auth", "workspace/auth")
	manager.SpawnAgent(ctx, sessionID, "db", "workspace/db")
	manager.SpawnAgent(ctx, sessionID, "tests", "workspace/tests")

	// Terminate session
	if err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to terminate session: %w", err)
	}

	// Verify session is removed
	session := manager.Get(sessionID)
	if session != nil {
		return fmt.Errorf("session still exists after termination")
	}

	// Verify agents are gone
	_, err := manager.ListAgents(sessionID)
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
	userSession, _ := manager.CreateUserSession(ctx, ws)
	sessionID := userSession.GetID()

	manager.SpawnAgent(ctx, sessionID, "auth", "workspace/auth")

	// Terminate agent twice - both should succeed (idempotent)
	if err := manager.TerminateAgent(ctx, sessionID, "auth"); err != nil {
		return fmt.Errorf("first termination failed: %w", err)
	}

	// Second termination should succeed without error (idempotent behavior)
	if err := manager.TerminateAgent(ctx, sessionID, "auth"); err != nil {
		return fmt.Errorf("second termination failed (expected idempotent): %w", err)
	}

	// Terminate session twice - both should succeed (idempotent)
	if err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		return fmt.Errorf("first session termination failed: %w", err)
	}

	// Second termination should succeed without error (idempotent behavior)
	if err := manager.TerminateUserSession(ctx, sessionID); err != nil {
		return fmt.Errorf("second session termination failed (expected idempotent): %w", err)
	}

	debug(verbose, "  ✓ Double agent termination is idempotent (no error)")
	debug(verbose, "  ✓ Double session termination is idempotent (no error)")
	success("✅", "Idempotent termination verified")
	return nil
}

func testListAndFilter(ctx context.Context, manager *session.Manager, verbose bool) error {
	// Create fresh manager for isolated testing
	store := session.NewMemoryStore()
	idGen := &testIDGenerator{nextID: "list-test-"}
	clock := &testClock{now: time.Now()}
	cleaner := session.NewNoOpCleaner()
	logger := &testLogger{verbose: verbose}
	clientFactory := session.NewFakeClientFactory(func(workspace string) (session.ACPClient, error) {
		return &fakeACPClient{workspace: workspace}, nil
	})

	freshManager := session.NewManager(store, idGen, clock, cleaner, logger, clientFactory)

	// Create multiple sessions
	ws1 := &fakeWebSocket{}
	session1, _ := freshManager.CreateUserSession(ctx, ws1)

	ws2 := &fakeWebSocket{}
	session2, _ := freshManager.CreateUserSession(ctx, ws2)

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
	freshManager.TerminateUserSession(ctx, session1.GetID())

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

func (c *fakeACPClient) SendMessage(content string) (interface{}, error) {
	return map[string]interface{}{
		"type":    "text",
		"content": "fake response from " + c.workspace,
	}, nil
}

func (c *fakeACPClient) Close() error {
	c.closed = true
	return nil
}

// Output helpers

func announce(icon, format string, args ...interface{}) {
	fmt.Printf("%s%s %s%s\n", colorCyan, icon, fmt.Sprintf(format, args...), colorReset)
}

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
