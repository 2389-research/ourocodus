package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/gorilla/websocket"
)

// integrationMockACPClient simulates a successful ACP agent for integration testing
// Separate from mockACPClient in server_unit_test.go to avoid conflicts
type integrationMockACPClient struct {
	sendFunc  func(context.Context, string) (*acp.AgentMessage, error)
	closeFunc func(context.Context) error
	mu        sync.Mutex
	messages  []string // Track messages sent
}

func (m *integrationMockACPClient) SendMessage(ctx context.Context, content string) (*acp.AgentMessage, error) {
	m.mu.Lock()
	m.messages = append(m.messages, content)
	m.mu.Unlock()

	if m.sendFunc != nil {
		return m.sendFunc(ctx, content)
	}
	// Default: Return a successful agent response
	return &acp.AgentMessage{
		Content: fmt.Sprintf("Agent response to: %s", content),
	}, nil
}

func (m *integrationMockACPClient) InitializeACP(ctx context.Context) (*acp.InitializeResult, error) {
	return &acp.InitializeResult{ProtocolVersion: 1}, nil
}

func (m *integrationMockACPClient) CreateSession(ctx context.Context, cwd string) (string, error) {
	return "integration-test-session", nil
}

func (m *integrationMockACPClient) SendPrompt(ctx context.Context, sessionID, prompt string) error {
	return nil
}

func (m *integrationMockACPClient) Stream(ctx context.Context) <-chan acp.Event {
	ch := make(chan acp.Event)
	close(ch)
	return ch
}

func (m *integrationMockACPClient) Close(ctx context.Context) error {
	if m.closeFunc != nil {
		return m.closeFunc(ctx)
	}
	return nil
}

func (m *integrationMockACPClient) GetMessages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.messages...)
}

// integrationMockClientFactory creates mock ACP clients for integration testing
type integrationMockClientFactory struct {
	clientFunc  func(workspace string) (session.ACPClient, error)
	runtimeFunc func(ctx context.Context, runtime *session.AgentRuntimeContext) (session.ACPClient, error)
	callCount   int
	mu          sync.Mutex
}

func (m *integrationMockClientFactory) NewClient(ctx context.Context, runtime *session.AgentRuntimeContext) (session.ACPClient, error) {
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
	return &integrationMockACPClient{}, nil
}

func (m *integrationMockClientFactory) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// integrationMockIDGenerator generates predictable IDs for testing
type integrationMockIDGenerator struct {
	counter int
	mu      sync.Mutex
}

func (m *integrationMockIDGenerator) Generate() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counter++
	return fmt.Sprintf("test-id-%d", m.counter)
}

func (m *integrationMockIDGenerator) GetCounter() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counter
}

// integrationMockClock provides fixed time for testing (returns RFC3339 string)
type integrationMockClock struct {
	timestamp string
	mu        sync.RWMutex
}

func (m *integrationMockClock) Now() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.timestamp
}

func (m *integrationMockClock) SetNow(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timestamp = t.Format(time.RFC3339)
}

// integrationMockSessionClock adapts string clock to time.Time for session package
type integrationMockSessionClock struct {
	baseTime time.Time
	mu       sync.RWMutex
}

func (m *integrationMockSessionClock) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.baseTime
}

func (m *integrationMockSessionClock) SetNow(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseTime = t
}

// integrationMockCleaner tracks cleanup calls
type integrationMockCleaner struct {
	cleanupCalled int
	mu            sync.Mutex
}

func (m *integrationMockCleaner) Cleanup(ctx context.Context, sess *session.UserSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupCalled++
	return nil
}

func (m *integrationMockCleaner) CleanupCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cleanupCalled
}

// integrationMockLogger captures log output for testing
type integrationMockLogger struct {
	logs []string
	mu   sync.Mutex
}

func (m *integrationMockLogger) Printf(format string, v ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, fmt.Sprintf(format, v...))
}

func (m *integrationMockLogger) GetLogs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.logs...)
}

// setupIntegrationServer creates a fully configured test server with real session.Manager
// Returns server, clientFactory, and tempDir for constructing workspace paths
func setupIntegrationServer(t *testing.T) (*Server, *integrationMockClientFactory, string) {
	t.Helper()
	// Create relay-layer mocks
	relayIDGen := &integrationMockIDGenerator{}
	relayClock := &integrationMockClock{}
	relayClock.SetNow(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))
	relayLogger := &integrationMockLogger{}

	// Create session-layer mocks
	sessionIDGen := &integrationMockIDGenerator{}
	sessionClock := &integrationMockSessionClock{}
	sessionClock.SetNow(time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))
	sessionCleaner := &integrationMockCleaner{}
	sessionLogger := &integrationMockLogger{}
	clientFactory := &integrationMockClientFactory{}

	// Create temporary directory for test workspaces
	tempDir := t.TempDir()

	// Create real session.Manager with injected mocks
	store := session.NewMemoryStore()
	mockFactory := agent.NewMockLauncherFactory()                                                                                                    // NEW
	sessionManager := session.NewManager(store, sessionIDGen, sessionClock, sessionCleaner, sessionLogger, clientFactory, tempDir, nil, mockFactory) // nil publisher for integration tests

	// Create relay server with real session manager
	server := NewServer(
		relayIDGen,
		relayLogger,
		relayClock,
		NewGorillaUpgrader(func(r *http.Request) bool { return true }),
		sessionManager,
	)

	return server, clientFactory, tempDir
}

// Test_FullFlow_CreateSession_SpawnAgent_SendMessage tests the complete integration flow
// This is the PRIMARY test for Issue #30 - validates the entire PWA → Relay → ACP → PWA flow
func Test_FullFlow_CreateSession_SpawnAgent_SendMessage(t *testing.T) {
	// Setup server with real session.Manager
	server, clientFactory, tempDir := setupIntegrationServer(t)

	// Configure mock client factory to return successful mock client
	var testClient *integrationMockACPClient
	clientFactory.clientFunc = func(workspace string) (session.ACPClient, error) {
		testClient = &integrationMockACPClient{
			sendFunc: func(ctx context.Context, content string) (*acp.AgentMessage, error) {
				return &acp.AgentMessage{
					Content: fmt.Sprintf("Mock agent says: %s", content),
				}, nil
			},
		}
		return testClient, nil
	}

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// 1. Read connection:established handshake
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read handshake: %v", err)
	}

	var handshake BaseMessage
	err = json.Unmarshal(message, &handshake)
	if err != nil {
		t.Fatalf("Failed to parse handshake: %v", err)
	}
	if handshake.Type != "connection:established" {
		t.Errorf("Expected connection:established, got %s", handshake.Type)
	}

	// 2. Send session:create
	sessionCreate := map[string]interface{}{
		"version": ProtocolVersion,
		"type":    "session:create",
	}
	err = conn.WriteJSON(sessionCreate)
	if err != nil {
		t.Fatalf("Failed to send session:create: %v", err)
	}

	// Read session:created response
	_, message, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read session:created: %v", err)
	}

	var sessionCreated map[string]interface{}
	err = json.Unmarshal(message, &sessionCreated)
	if err != nil {
		t.Fatalf("Failed to parse session:created: %v", err)
	}
	if sessionCreated["type"] != "session:created" {
		t.Errorf("Expected session:created, got %v", sessionCreated["type"])
	}

	userSessionID, ok := sessionCreated["userSessionId"].(string)
	if !ok || userSessionID == "" {
		t.Fatalf("Missing or invalid userSessionId in response")
	}

	// 3. Send agent:spawn
	agentSpawn := map[string]interface{}{
		"version":       ProtocolVersion,
		"type":          "agent:spawn",
		"userSessionId": userSessionID,
		"agentId":       "test-agent",
		"workspace":     fmt.Sprintf("%s/test-agent", tempDir),
	}
	err = conn.WriteJSON(agentSpawn)
	if err != nil {
		t.Fatalf("Failed to send agent:spawn: %v", err)
	}

	// Read agent:ready response
	_, message, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read agent:ready: %v", err)
	}

	var agentReady map[string]interface{}
	err = json.Unmarshal(message, &agentReady)
	if err != nil {
		t.Fatalf("Failed to parse agent:ready: %v", err)
	}
	if agentReady["type"] != "agent:ready" {
		t.Errorf("Expected agent:ready, got %v", agentReady["type"])
	}
	if agentReady["agentId"] != "test-agent" {
		t.Errorf("Expected role test-agent, got %v", agentReady["agentId"])
	}

	// 4. Send agent:message
	agentMessage := map[string]interface{}{
		"version":       ProtocolVersion,
		"type":          "agent:message",
		"userSessionId": userSessionID,
		"agentId":       "test-agent",
		"content":       "Hello, agent!",
	}
	err = conn.WriteJSON(agentMessage)
	if err != nil {
		t.Fatalf("Failed to send agent:message: %v", err)
	}

	// Read agent:response
	_, message, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read agent:response: %v", err)
	}

	var agentResponse map[string]interface{}
	err = json.Unmarshal(message, &agentResponse)
	if err != nil {
		t.Fatalf("Failed to parse agent:response: %v", err)
	}

	// Verify response
	if agentResponse["type"] != "agent:response" {
		t.Errorf("Expected agent:response, got %v", agentResponse["type"])
	}
	if agentResponse["userSessionId"] != userSessionID {
		t.Errorf("Expected userSessionId %s, got %v", userSessionID, agentResponse["userSessionId"])
	}
	if agentResponse["agentId"] != "test-agent" {
		t.Errorf("Expected role test-agent, got %v", agentResponse["agentId"])
	}

	content, ok := agentResponse["content"].(string)
	if !ok {
		t.Fatalf("Missing or invalid content in response")
	}
	if !strings.Contains(content, "Mock agent says: Hello, agent!") {
		t.Errorf("Expected response to contain 'Mock agent says: Hello, agent!', got: %s", content)
	}

	// 5. Verify ACP client received message
	messages := testClient.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message to ACP client, got %d", len(messages))
	}
	if len(messages) > 0 && messages[0] != "Hello, agent!" {
		t.Errorf("Expected message 'Hello, agent!', got '%s'", messages[0])
	}

	// 6. Verify client factory was called once
	if count := clientFactory.CallCount(); count != 1 {
		t.Errorf("Expected client factory called once, got %d", count)
	}

	// 7. Verify conversation history was persisted
	sessionMgr, ok := server.sessionManager.(*session.Manager)
	if !ok {
		t.Fatal("Expected session manager to be *session.Manager")
	}
	history, err := sessionMgr.GetAgentHistory(userSessionID, "test-agent")
	if err != nil {
		t.Fatalf("Failed to get agent history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("Expected 2 history entries (user + agent), got %d", len(history))
	}
	if history[0].From != "user" || history[0].Content != "Hello, agent!" {
		t.Errorf("Unexpected first history entry: %+v", history[0])
	}
	expectedAgentResponse := "Mock agent says: Hello, agent!"
	if history[1].From != "agent" || history[1].Content != expectedAgentResponse {
		t.Errorf("Unexpected second history entry: %+v", history[1])
	}
}

// Test_HandleSessionCreate_Success tests session creation success path
func Test_HandleSessionCreate_Success(t *testing.T) {
	server, _, _ := setupIntegrationServer(t)

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read and discard handshake
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read handshake: %v", err)
	}

	// Send session:create
	sessionCreate := map[string]interface{}{
		"version": ProtocolVersion,
		"type":    "session:create",
	}
	err = conn.WriteJSON(sessionCreate)
	if err != nil {
		t.Fatalf("Failed to send session:create: %v", err)
	}

	// Read response
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response
	if response["version"] != ProtocolVersion {
		t.Errorf("Expected version %s, got %v", ProtocolVersion, response["version"])
	}
	if response["type"] != "session:created" {
		t.Errorf("Expected type session:created, got %v", response["type"])
	}

	userSessionID, ok := response["userSessionId"].(string)
	if !ok || userSessionID == "" {
		t.Fatal("Missing or invalid sessionId")
	}

	// Verify session ID format (avoid coupling to internal mock counter)
	if !strings.HasPrefix(userSessionID, "test-id-") {
		t.Errorf("Unexpected sessionId format: %s", userSessionID)
	}

	// Verify timestamp exists
	if _, ok := response["timestamp"]; !ok {
		t.Error("Response missing timestamp")
	}
}

// Test_HandleAgentSpawn_Success tests agent spawning success path
func Test_HandleAgentSpawn_Success(t *testing.T) {
	server, clientFactory, tempDir := setupIntegrationServer(t)

	// Configure mock client factory
	clientFactory.clientFunc = func(workspace string) (session.ACPClient, error) {
		return &integrationMockACPClient{}, nil
	}

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read handshake
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read handshake: %v", err)
	}

	// Create session first
	sessionCreate := map[string]interface{}{
		"version": ProtocolVersion,
		"type":    "session:create",
	}
	err = conn.WriteJSON(sessionCreate)
	if err != nil {
		t.Fatalf("Failed to send session:create: %v", err)
	}

	// Read session:created
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read session:created: %v", err)
	}

	var sessionCreated map[string]interface{}
	err = json.Unmarshal(message, &sessionCreated)
	if err != nil {
		t.Fatalf("Failed to parse session:created: %v", err)
	}
	userSessionID, ok := sessionCreated["userSessionId"].(string)
	if !ok || userSessionID == "" {
		t.Fatal("Missing or invalid sessionId in session:created response")
	}

	// Send agent:spawn
	agentSpawn := map[string]interface{}{
		"version":       ProtocolVersion,
		"type":          "agent:spawn",
		"userSessionId": userSessionID,
		"agentId":       "test-agent",
		"workspace":     fmt.Sprintf("%s/spawn-test", tempDir),
	}
	err = conn.WriteJSON(agentSpawn)
	if err != nil {
		t.Fatalf("Failed to send agent:spawn: %v", err)
	}

	// Read agent:ready response
	_, message, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read agent:ready: %v", err)
	}

	var agentReady map[string]interface{}
	if err := json.Unmarshal(message, &agentReady); err != nil {
		t.Fatalf("Failed to parse agent:ready: %v", err)
	}

	// Verify response
	if agentReady["version"] != ProtocolVersion {
		t.Errorf("Expected version %s, got %v", ProtocolVersion, agentReady["version"])
	}
	if agentReady["type"] != "agent:ready" {
		t.Errorf("Expected type agent:ready, got %v", agentReady["type"])
	}
	if agentReady["userSessionId"] != userSessionID {
		t.Errorf("Expected userSessionId %s, got %v", userSessionID, agentReady["userSessionId"])
	}
	if agentReady["agentId"] != "test-agent" {
		t.Errorf("Expected role test-agent, got %v", agentReady["agentId"])
	}

	// Verify client factory was called
	if count := clientFactory.CallCount(); count != 1 {
		t.Errorf("Expected client factory called once, got %d", count)
	}
}

// Test_HandleAgentMessage_Success_FullFlow tests agent message handling success path
func Test_HandleAgentMessage_Success_FullFlow(t *testing.T) {
	server, clientFactory, tempDir := setupIntegrationServer(t)

	// Configure mock client to return specific response
	var capturedContent string
	clientFactory.clientFunc = func(workspace string) (session.ACPClient, error) {
		return &integrationMockACPClient{
			sendFunc: func(ctx context.Context, content string) (*acp.AgentMessage, error) {
				capturedContent = content
				return &acp.AgentMessage{
					Content: "Agent processed: " + content,
				}, nil
			},
		}, nil
	}

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read handshake
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read handshake: %v", err)
	}

	// Create session
	sessionCreate := map[string]interface{}{
		"version": ProtocolVersion,
		"type":    "session:create",
	}
	err = conn.WriteJSON(sessionCreate)
	if err != nil {
		t.Fatalf("Failed to send session:create: %v", err)
	}

	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read session:created: %v", err)
	}
	var sessionCreated map[string]interface{}
	err = json.Unmarshal(message, &sessionCreated)
	if err != nil {
		t.Fatalf("Failed to parse session:created: %v", err)
	}
	userSessionID, ok := sessionCreated["userSessionId"].(string)
	if !ok || userSessionID == "" {
		t.Fatal("Missing or invalid sessionId in session:created response")
	}

	// Spawn agent
	agentSpawn := map[string]interface{}{
		"version":       ProtocolVersion,
		"type":          "agent:spawn",
		"userSessionId": userSessionID,
		"agentId":       "test-agent",
		"workspace":     fmt.Sprintf("%s/message-test", tempDir),
	}
	err = conn.WriteJSON(agentSpawn)
	if err != nil {
		t.Fatalf("Failed to send agent:spawn: %v", err)
	}

	// Read agent:ready
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read agent:ready: %v", err)
	}

	// Send agent:message
	testContent := "Test message for agent"
	agentMessage := map[string]interface{}{
		"version":       ProtocolVersion,
		"type":          "agent:message",
		"userSessionId": userSessionID,
		"agentId":       "test-agent",
		"content":       testContent,
	}
	err = conn.WriteJSON(agentMessage)
	if err != nil {
		t.Fatalf("Failed to send agent:message: %v", err)
	}

	// Read agent:response
	_, message, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read agent:response: %v", err)
	}

	var agentResponse map[string]interface{}
	err = json.Unmarshal(message, &agentResponse)
	if err != nil {
		t.Fatalf("Failed to parse agent:response: %v", err)
	}

	// Verify response structure
	if agentResponse["version"] != ProtocolVersion {
		t.Errorf("Expected version %s, got %v", ProtocolVersion, agentResponse["version"])
	}
	if agentResponse["type"] != "agent:response" {
		t.Errorf("Expected type agent:response, got %v", agentResponse["type"])
	}
	if agentResponse["userSessionId"] != userSessionID {
		t.Errorf("Expected userSessionId %s, got %v", userSessionID, agentResponse["userSessionId"])
	}
	if agentResponse["agentId"] != "test-agent" {
		t.Errorf("Expected role test-agent, got %v", agentResponse["agentId"])
	}

	// Verify content
	content, ok := agentResponse["content"].(string)
	if !ok {
		t.Fatal("Missing or invalid content in response")
	}
	expectedContent := "Agent processed: " + testContent
	if content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, content)
	}

	// Verify timestamp exists
	if _, ok := agentResponse["timestamp"]; !ok {
		t.Error("Response missing timestamp")
	}

	// Verify ACP client received correct message
	if capturedContent != testContent {
		t.Errorf("Expected ACP client to receive '%s', got '%s'", testContent, capturedContent)
	}
}

// Test_SessionWebSocketAdapter_WriteJSON tests WriteJSON method coverage
func Test_SessionWebSocketAdapter_WriteJSON(t *testing.T) {
	// Create a mock websocket connection
	mockConn := &mockWebSocketConn{}

	// Create adapter
	adapter := &SessionWebSocketAdapter{conn: mockConn}

	// Test data
	testData := map[string]interface{}{
		"version": ProtocolVersion,
		"type":    "test",
		"data":    "test value",
	}

	// Call WriteJSON
	err := adapter.WriteJSON(testData)
	if err != nil {
		t.Errorf("WriteJSON failed: %v", err)
	}

	// Verify mock connection received the data
	if len(mockConn.written) != 1 {
		t.Errorf("Expected 1 write, got %d", len(mockConn.written))
	}
}

// Test_SessionWebSocketAdapter_WriteJSON_Error tests WriteJSON error handling
func Test_SessionWebSocketAdapter_WriteJSON_Error(t *testing.T) {
	// Create a mock websocket connection with error
	mockConn := &mockWebSocketConn{
		writeError: fmt.Errorf("write error"),
	}

	// Create adapter
	adapter := &SessionWebSocketAdapter{conn: mockConn}

	// Test data
	testData := map[string]interface{}{
		"version": ProtocolVersion,
		"type":    "test",
	}

	// Call WriteJSON - should return error
	err := adapter.WriteJSON(testData)
	if err == nil {
		t.Error("Expected error from WriteJSON, got nil")
	}
	if err.Error() != "write error" {
		t.Errorf("Expected 'write error', got '%v'", err)
	}
}

// Test_SessionWebSocketAdapter_Close tests Close method coverage
func Test_SessionWebSocketAdapter_Close(t *testing.T) {
	// Create a mock websocket connection
	mockConn := &mockWebSocketConn{}

	// Create adapter
	adapter := &SessionWebSocketAdapter{conn: mockConn}

	// Call Close
	err := adapter.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify mock connection was closed
	if !mockConn.closed {
		t.Error("Expected connection to be closed")
	}
}
