package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// TestAgentAttachDetach_HappyPath tests the full attach/detach flow
func TestAgentAttachDetach_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup: Create a mock agent container
	ctx := context.Background()

	// Set Docker socket for Colima if available
	setupDockerSocket(t)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer cli.Close()

	// Create temporary lease directory
	tempDir := t.TempDir()
	oldLeaseDir := session.LeaseDir
	session.LeaseDir = filepath.Join(tempDir, "leases")
	defer func() { session.LeaseDir = oldLeaseDir }()

	// Create test agent container
	agentID := fmt.Sprintf("test-agent-%d", time.Now().Unix())
	containerID := createTestAgentContainer(t, cli, agentID)
	defer cleanupTestContainer(t, cli, containerID)

	// Create server with mocks
	logger := &integrationMockLogger{}
	clock := &integrationMockClock{}
	clock.SetNow(time.Now())
	idGen := &integrationMockIDGenerator{}

	sessionManager := createMockSessionManager(t)
	server := NewServer(idGen, logger, clock, &mockUpgrader{}, sessionManager)

	// Create mock WebSocket connection
	conn := &testWebSocketConn{
		messages: make([]interface{}, 0),
	}

	// Create a real user session to test against
	userSession, err := sessionManager.CreateUserSession(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to create user session: %v", err)
	}
	userSessionID := userSession.GetID()

	// Test 1: Attach to agent
	attachReq := AgentAttachRequest{
		Type:          "agent:attach",
		AgentID:       agentID,
		UserSessionID: userSessionID,
	}
	attachMsg, _ := json.Marshal(attachReq)

	shouldClose := server.handleAgentAttach(ctx, conn, attachMsg)
	if shouldClose {
		t.Fatal("handleAgentAttach returned true (connection close), expected false")
	}

	// Verify attach response
	if len(conn.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conn.messages))
	}

	var attachResp AgentAttachResponse
	respBytes, _ := json.Marshal(conn.messages[0])
	if err := json.Unmarshal(respBytes, &attachResp); err != nil {
		t.Fatalf("Failed to parse attach response: %v", err)
	}

	if attachResp.Type != "agent:attached" {
		// Print the error message if it's an error response
		if attachResp.Type == "error" {
			// The mock stores ErrorMessage directly, not as []byte
			if errMsg, ok := conn.messages[0].(ErrorMessage); ok {
				t.Errorf("Expected type 'agent:attached', got error: %s - %s", errMsg.Error.Code, errMsg.Error.Message)
			} else {
				t.Errorf("Expected type 'agent:attached', got error (couldn't parse)")
			}
		} else {
			t.Errorf("Expected type 'agent:attached', got '%s'", attachResp.Type)
		}
	}
	if attachResp.AgentID != agentID {
		t.Errorf("Expected agentId '%s', got '%s'", agentID, attachResp.AgentID)
	}
	if attachResp.SessionID != userSessionID {
		t.Errorf("Expected sessionId '%s', got '%s'", userSessionID, attachResp.SessionID)
	}
	if attachResp.ExpiresAt.IsZero() {
		t.Error("Expected non-zero ExpiresAt")
	}

	// Verify lease file exists
	lease, err := session.ReadLease(agentID)
	if err != nil {
		t.Fatalf("Failed to read lease: %v", err)
	}
	if lease.UserSessionID != userSessionID {
		t.Errorf("Expected lease userSessionId '%s', got '%s'", userSessionID, lease.UserSessionID)
	}

	// Test 2: Detach from agent
	conn.messages = make([]interface{}, 0) // Clear messages

	detachReq := AgentDetachRequest{
		Type:          "agent:detach",
		AgentID:       agentID,
		UserSessionID: userSessionID,
	}
	detachMsg, _ := json.Marshal(detachReq)

	shouldClose = server.handleAgentDetach(ctx, conn, detachMsg)
	if shouldClose {
		t.Fatal("handleAgentDetach returned true (connection close), expected false")
	}

	// Verify detach response
	if len(conn.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conn.messages))
	}

	var detachResp AgentDetachResponse
	respBytes, _ = json.Marshal(conn.messages[0])
	if err := json.Unmarshal(respBytes, &detachResp); err != nil {
		t.Fatalf("Failed to parse detach response: %v", err)
	}

	if detachResp.Type != "agent:detached" {
		t.Errorf("Expected type 'agent:detached', got '%s'", detachResp.Type)
	}
	if detachResp.AgentID != agentID {
		t.Errorf("Expected agentId '%s', got '%s'", agentID, detachResp.AgentID)
	}

	// Verify lease is gone
	_, err = session.ReadLease(agentID)
	if err != session.ErrLeaseNotFound {
		t.Errorf("Expected lease to be removed, got error: %v", err)
	}
}

// TestAgentAttach_NonExistentAgent tests attaching to a non-existent agent
func TestAgentAttach_NonExistentAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Verify Docker is available
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer cli.Close()

	// Create temporary lease directory
	tempDir := t.TempDir()
	oldLeaseDir := session.LeaseDir
	session.LeaseDir = filepath.Join(tempDir, "leases")
	defer func() { session.LeaseDir = oldLeaseDir }()

	// Create server
	logger := &integrationMockLogger{}
	clock := &integrationMockClock{}
	clock.SetNow(time.Now())
	idGen := &integrationMockIDGenerator{}

	sessionManager := createMockSessionManager(t)
	server := NewServer(idGen, logger, clock, &mockUpgrader{}, sessionManager)

	conn := &testWebSocketConn{
		messages: make([]interface{}, 0),
	}

	// Try to attach to non-existent agent
	attachReq := AgentAttachRequest{
		Type:          "agent:attach",
		AgentID:       "non-existent-agent-123",
		UserSessionID: "test-session-456",
	}
	attachMsg, _ := json.Marshal(attachReq)

	shouldClose := server.handleAgentAttach(ctx, conn, attachMsg)
	if shouldClose {
		t.Fatal("Expected connection to remain open for recoverable error")
	}

	// Verify error response
	if len(conn.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conn.messages))
	}

	var errorResp ErrorMessage
	respBytes, _ := json.Marshal(conn.messages[0])
	if err := json.Unmarshal(respBytes, &errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errorResp.Type != "error" {
		t.Errorf("Expected type 'error', got '%s'", errorResp.Type)
	}
	if errorResp.Error.Code != "AGENT_NOT_FOUND" {
		t.Errorf("Expected code 'AGENT_NOT_FOUND', got '%s'", errorResp.Error.Code)
	}
}

// TestAgentAttach_AlreadyAttached tests attaching to an already-attached agent
func TestAgentAttach_AlreadyAttached(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Set Docker socket for Colima if available
	setupDockerSocket(t)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer cli.Close()

	// Create temporary lease directory
	tempDir := t.TempDir()
	oldLeaseDir := session.LeaseDir
	session.LeaseDir = filepath.Join(tempDir, "leases")
	defer func() { session.LeaseDir = oldLeaseDir }()

	// Create test agent container
	agentID := fmt.Sprintf("test-agent-%d", time.Now().Unix())
	containerID := createTestAgentContainer(t, cli, agentID)
	defer cleanupTestContainer(t, cli, containerID)

	// Create server
	logger := &integrationMockLogger{}
	clock := &integrationMockClock{}
	clock.SetNow(time.Now())
	idGen := &integrationMockIDGenerator{}

	sessionManager := createMockSessionManager(t)
	server := NewServer(idGen, logger, clock, &mockUpgrader{}, sessionManager)

	conn := &testWebSocketConn{
		messages: make([]interface{}, 0),
	}

	// Create two user sessions
	firstSession, err := sessionManager.CreateUserSession(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to create first user session: %v", err)
	}
	firstSessionID := firstSession.GetID()

	conn2 := &testWebSocketConn{
		messages: make([]interface{}, 0),
	}
	secondSession, err := sessionManager.CreateUserSession(ctx, conn2)
	if err != nil {
		t.Fatalf("Failed to create second user session: %v", err)
	}
	secondSessionID := secondSession.GetID()

	// First session attaches
	attachReq1 := AgentAttachRequest{
		Type:          "agent:attach",
		AgentID:       agentID,
		UserSessionID: firstSessionID,
	}
	attachMsg1, _ := json.Marshal(attachReq1)

	shouldClose := server.handleAgentAttach(ctx, conn, attachMsg1)
	if shouldClose {
		t.Fatal("First attach should succeed")
	}

	// Verify first attach succeeded
	if len(conn.messages) != 1 {
		t.Fatalf("Expected 1 message after first attach, got %d", len(conn.messages))
	}

	// Second session tries to attach
	conn.messages = make([]interface{}, 0) // Clear messages
	attachReq2 := AgentAttachRequest{
		Type:          "agent:attach",
		AgentID:       agentID,
		UserSessionID: secondSessionID,
	}
	attachMsg2, _ := json.Marshal(attachReq2)

	shouldClose = server.handleAgentAttach(ctx, conn, attachMsg2)
	if shouldClose {
		t.Fatal("Expected connection to remain open for recoverable error")
	}

	// Verify conflict error
	if len(conn.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conn.messages))
	}

	var errorResp ErrorMessage
	respBytes, _ := json.Marshal(conn.messages[0])
	if err := json.Unmarshal(respBytes, &errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errorResp.Type != "error" {
		t.Errorf("Expected type 'error', got '%s'", errorResp.Type)
	}
	if errorResp.Error.Code != "AGENT_ALREADY_ATTACHED" {
		t.Errorf("Expected code 'AGENT_ALREADY_ATTACHED', got '%s'", errorResp.Error.Code)
	}
}

// TestAgentDetach_NotAttachedToYou tests detaching an agent owned by another session
func TestAgentDetach_NotAttachedToYou(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Set Docker socket for Colima if available
	setupDockerSocket(t)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer cli.Close()

	// Create temporary lease directory
	tempDir := t.TempDir()
	oldLeaseDir := session.LeaseDir
	session.LeaseDir = filepath.Join(tempDir, "leases")
	defer func() { session.LeaseDir = oldLeaseDir }()

	// Create test agent container
	agentID := fmt.Sprintf("test-agent-%d", time.Now().Unix())
	containerID := createTestAgentContainer(t, cli, agentID)
	defer cleanupTestContainer(t, cli, containerID)

	// Create server
	logger := &integrationMockLogger{}
	clock := &integrationMockClock{}
	clock.SetNow(time.Now())
	idGen := &integrationMockIDGenerator{}

	sessionManager := createMockSessionManager(t)
	server := NewServer(idGen, logger, clock, &mockUpgrader{}, sessionManager)

	conn := &testWebSocketConn{
		messages: make([]interface{}, 0),
	}

	// Create two user sessions
	session1, err := sessionManager.CreateUserSession(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to create first user session: %v", err)
	}
	session1ID := session1.GetID()

	conn2 := &testWebSocketConn{
		messages: make([]interface{}, 0),
	}
	session2, err := sessionManager.CreateUserSession(ctx, conn2)
	if err != nil {
		t.Fatalf("Failed to create second user session: %v", err)
	}
	session2ID := session2.GetID()

	// Session 1 attaches
	attachReq := AgentAttachRequest{
		Type:          "agent:attach",
		AgentID:       agentID,
		UserSessionID: session1ID,
	}
	attachMsg, _ := json.Marshal(attachReq)

	shouldClose := server.handleAgentAttach(ctx, conn, attachMsg)
	if shouldClose {
		t.Fatal("Attach should succeed")
	}

	// Session 2 tries to detach
	conn.messages = make([]interface{}, 0) // Clear messages
	detachReq := AgentDetachRequest{
		Type:          "agent:detach",
		AgentID:       agentID,
		UserSessionID: session2ID,
	}
	detachMsg, _ := json.Marshal(detachReq)

	shouldClose = server.handleAgentDetach(ctx, conn, detachMsg)
	if shouldClose {
		t.Fatal("Expected connection to remain open for recoverable error")
	}

	// Verify ownership error
	if len(conn.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conn.messages))
	}

	var errorResp ErrorMessage
	respBytes, _ := json.Marshal(conn.messages[0])
	if err := json.Unmarshal(respBytes, &errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errorResp.Type != "error" {
		t.Errorf("Expected type 'error', got '%s'", errorResp.Type)
	}
	if errorResp.Error.Code != "NOT_ATTACHED_TO_YOU" {
		t.Errorf("Expected code 'NOT_ATTACHED_TO_YOU', got '%s'", errorResp.Error.Code)
	}

	// Verify lease is still owned by session 1
	lease, err := session.ReadLease(agentID)
	if err != nil {
		t.Fatalf("Failed to read lease: %v", err)
	}
	if lease.UserSessionID != session1ID {
		t.Errorf("Expected lease to still be owned by '%s', got '%s'", session1ID, lease.UserSessionID)
	}
}

// TestAgentAttachDetach_Idempotent tests that attach/detach operations are idempotent
func TestAgentAttachDetach_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Set Docker socket for Colima if available
	setupDockerSocket(t)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
	defer cli.Close()

	// Create temporary lease directory
	tempDir := t.TempDir()
	oldLeaseDir := session.LeaseDir
	session.LeaseDir = filepath.Join(tempDir, "leases")
	defer func() { session.LeaseDir = oldLeaseDir }()

	// Create test agent container
	agentID := fmt.Sprintf("test-agent-%d", time.Now().Unix())
	containerID := createTestAgentContainer(t, cli, agentID)
	defer cleanupTestContainer(t, cli, containerID)

	// Create server
	logger := &integrationMockLogger{}
	clock := &integrationMockClock{}
	clock.SetNow(time.Now())
	idGen := &integrationMockIDGenerator{}

	sessionManager := createMockSessionManager(t)
	server := NewServer(idGen, logger, clock, &mockUpgrader{}, sessionManager)

	conn := &testWebSocketConn{
		messages: make([]interface{}, 0),
	}

	// Create a user session
	userSession, err := sessionManager.CreateUserSession(ctx, conn)
	if err != nil {
		t.Fatalf("Failed to create user session: %v", err)
	}
	userSessionID := userSession.GetID()

	// Test 1: Attach twice (idempotent attach)
	attachReq := AgentAttachRequest{
		Type:          "agent:attach",
		AgentID:       agentID,
		UserSessionID: userSessionID,
	}
	attachMsg, _ := json.Marshal(attachReq)

	// First attach
	shouldClose := server.handleAgentAttach(ctx, conn, attachMsg)
	if shouldClose {
		t.Fatal("First attach should succeed")
	}

	// Second attach (same session)
	conn.messages = make([]interface{}, 0)
	shouldClose = server.handleAgentAttach(ctx, conn, attachMsg)
	if shouldClose {
		t.Fatal("Second attach should be idempotent")
	}

	// Verify second attach returned success (not error)
	if len(conn.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conn.messages))
	}

	var attachResp AgentAttachResponse
	respBytes, _ := json.Marshal(conn.messages[0])
	if err := json.Unmarshal(respBytes, &attachResp); err != nil {
		t.Fatalf("Failed to parse attach response: %v", err)
	}

	if attachResp.Type != "agent:attached" {
		t.Errorf("Expected idempotent attach to return 'agent:attached', got '%s'", attachResp.Type)
	}

	// Detach
	detachReq := AgentDetachRequest{
		Type:          "agent:detach",
		AgentID:       agentID,
		UserSessionID: userSessionID,
	}
	detachMsg, _ := json.Marshal(detachReq)

	conn.messages = make([]interface{}, 0)
	shouldClose = server.handleAgentDetach(ctx, conn, detachMsg)
	if shouldClose {
		t.Fatal("First detach should succeed")
	}

	// Test 2: Detach twice (idempotent detach)
	conn.messages = make([]interface{}, 0)
	shouldClose = server.handleAgentDetach(ctx, conn, detachMsg)
	if shouldClose {
		t.Fatal("Second detach should be idempotent")
	}

	// Verify second detach returned success (not error)
	if len(conn.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(conn.messages))
	}

	var detachResp AgentDetachResponse
	respBytes, _ = json.Marshal(conn.messages[0])
	if err := json.Unmarshal(respBytes, &detachResp); err != nil {
		t.Fatalf("Failed to parse detach response: %v", err)
	}

	if detachResp.Type != "agent:detached" {
		t.Errorf("Expected idempotent detach to return 'agent:detached', got '%s'", detachResp.Type)
	}
}

// Helper functions

// setupDockerSocket configures the Docker socket for Colima if available
func setupDockerSocket(t *testing.T) {
	t.Helper()

	// Check if DOCKER_HOST is already set (don't override user configuration)
	if existingHost := os.Getenv("DOCKER_HOST"); existingHost != "" {
		return
	}

	// Check if Colima is running
	colimaSocket := filepath.Join(os.Getenv("HOME"), ".colima", "default", "docker.sock")
	if _, err := os.Stat(colimaSocket); err == nil {
		// Colima socket exists, use it
		os.Setenv("DOCKER_HOST", "unix://"+colimaSocket)
		t.Logf("Using Colima Docker socket: %s", colimaSocket)
	}
}

// createTestAgentContainer creates a mock agent container for testing
func createTestAgentContainer(t *testing.T, cli *client.Client, agentID string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary workspace directory
	tmpDir, err := os.MkdirTemp("", "agent-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create temp workspace: %v", err)
	}

	// Create a minimal container with agent labels and workspace mount
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sleep", "3600"},
		Labels: map[string]string{
			LabelNamespace: "true",
			LabelAgentID:   agentID,
		},
	}, &container.HostConfig{
		Binds: []string{
			tmpDir + ":/workspace:rw",
		},
	}, nil, nil, "")
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create test container: %v", err)
	}

	// Start container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("Failed to start test container: %v", err)
	}

	return resp.ID
}

// cleanupTestContainer removes a test container and its workspace directory
func cleanupTestContainer(t *testing.T, cli *client.Client, containerID string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get workspace path from container mounts before removing
	inspectResp, err := cli.ContainerInspect(ctx, containerID)
	if err == nil {
		// Find and remove workspace directory
		for _, mnt := range inspectResp.Mounts {
			if mnt.Destination == "/workspace" {
				_ = os.RemoveAll(mnt.Source)
				break
			}
		}
	}

	_ = cli.ContainerStop(ctx, containerID, container.StopOptions{})
	_ = cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

// createMockSessionManager creates a minimal session manager for testing
func createMockSessionManager(t *testing.T) *session.Manager {
	t.Helper()

	// Set ANTHROPIC_API_KEY for session manager creation
	oldKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key-for-relay-tests")
	defer func() {
		if oldKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", oldKey)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	sessionIDGen := &integrationMockIDGenerator{}
	sessionClock := &integrationMockSessionClock{}
	sessionClock.SetNow(time.Now())
	sessionLogger := &integrationMockLogger{}

	sessionManager, err := NewSessionManager(
		sessionLogger,
		&testClockAdapter{clock: sessionClock},
		&testIDGenAdapter{idGen: sessionIDGen},
		nil, // no NATS for these tests
		&testLauncherFactory{},
		&testContainerManager{},
	)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	return sessionManager
}

// testClockAdapter adapts integrationMockSessionClock to Clock interface
type testClockAdapter struct {
	clock *integrationMockSessionClock
}

func (m *testClockAdapter) Now() string {
	return m.clock.Now().Format(time.RFC3339)
}

// testIDGenAdapter adapts integrationMockIDGenerator to IDGenerator interface
type testIDGenAdapter struct {
	idGen *integrationMockIDGenerator
}

func (m *testIDGenAdapter) Generate() string {
	return m.idGen.Generate()
}

// testLauncherFactory is a minimal launcher factory for testing
type testLauncherFactory struct{}

func (m *testLauncherFactory) CreateLauncher(ctx context.Context, agentID string, config agent.LauncherConfig) (agent.AgentLauncher, error) {
	return nil, fmt.Errorf("not implemented")
}

// testContainerManager is a minimal container manager for testing
type testContainerManager struct{}

func (m *testContainerManager) ExecInContainer(ctx context.Context, containerID string, cfg containersession.ExecConfig) (*containersession.ExecAttachment, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *testContainerManager) GetDockerClient() containersession.DockerClient {
	return nil
}

// testWebSocketConn captures messages for testing (distinct from mockWebSocketConn in server_unit_test.go)
type testWebSocketConn struct {
	messages []interface{}
}

func (m *testWebSocketConn) ReadMessage() (int, []byte, error) {
	return 1, nil, nil
}

func (m *testWebSocketConn) WriteJSON(v interface{}) error {
	m.messages = append(m.messages, v)
	return nil
}

func (m *testWebSocketConn) Close() error {
	return nil
}

func (m *testWebSocketConn) SetReadLimit(limit int64) {}

func (m *testWebSocketConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *testWebSocketConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *testWebSocketConn) SetPongHandler(h func(appData string) error) {}

func (m *testWebSocketConn) WriteMessage(messageType int, data []byte) error {
	return nil
}
