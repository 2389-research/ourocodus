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
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/gorilla/websocket"
)

// integrationMockACPClient simulates a successful ACP agent for integration testing
// Separate from mockACPClient in server_unit_test.go to avoid conflicts
type integrationMockACPClient struct {
	sendFunc  func(string) (interface{}, error)
	closeFunc func() error
	mu        sync.Mutex
	messages  []string // Track messages sent
}

func (m *integrationMockACPClient) SendMessage(content string) (interface{}, error) {
	m.mu.Lock()
	m.messages = append(m.messages, content)
	m.mu.Unlock()

	if m.sendFunc != nil {
		return m.sendFunc(content)
	}
	// Default: Return a successful agent response
	return &acp.AgentMessage{
		Content: fmt.Sprintf("Agent response to: %s", content),
	}, nil
}

func (m *integrationMockACPClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
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
	clientFunc func(workspace string) (session.ACPClient, error)
	callCount  int
	mu         sync.Mutex
}

func (m *integrationMockClientFactory) NewClient(workspace string) (session.ACPClient, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.clientFunc != nil {
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
func setupIntegrationServer() (*Server, *integrationMockIDGenerator, *integrationMockClientFactory) {
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

	// Create real session.Manager with injected mocks
	store := session.NewMemoryStore()
	sessionManager := session.NewManager(store, sessionIDGen, sessionClock, sessionCleaner, sessionLogger, clientFactory, "./test-workspaces")

	// Create relay server with real session manager
	server := NewServer(
		relayIDGen,
		relayLogger,
		relayClock,
		NewGorillaUpgrader(func(r *http.Request) bool { return true }),
		sessionManager,
	)

	return server, relayIDGen, clientFactory
}

// Test_FullFlow_CreateSession_SpawnAgent_SendMessage tests the complete integration flow
// This is the PRIMARY test for Issue #30 - validates the entire PWA → Relay → ACP → PWA flow
func Test_FullFlow_CreateSession_SpawnAgent_SendMessage(t *testing.T) {
	// Setup server with real session.Manager
	server, _, clientFactory := setupIntegrationServer()

	// Configure mock client factory to return successful mock client
	var testClient *integrationMockACPClient
	clientFactory.clientFunc = func(workspace string) (session.ACPClient, error) {
		testClient = &integrationMockACPClient{
			sendFunc: func(content string) (interface{}, error) {
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
		"version": "1.0",
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

	sessionID, ok := sessionCreated["sessionId"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("Missing or invalid sessionId in response")
	}

	// 3. Send agent:spawn
	agentSpawn := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:spawn",
		"sessionId": sessionID,
		"role":      "test-agent",
		"workspace": "./test-workspaces/test-agent",
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
	if agentReady["role"] != "test-agent" {
		t.Errorf("Expected role test-agent, got %v", agentReady["role"])
	}

	// 4. Send agent:message
	agentMessage := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:message",
		"sessionId": sessionID,
		"role":      "test-agent",
		"content":   "Hello, agent!",
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
	if err := json.Unmarshal(message, &agentResponse); err != nil {
		t.Fatalf("Failed to parse agent:response: %v", err)
	}

	// Verify response
	if agentResponse["type"] != "agent:response" {
		t.Errorf("Expected agent:response, got %v", agentResponse["type"])
	}
	if agentResponse["sessionId"] != sessionID {
		t.Errorf("Expected sessionId %s, got %v", sessionID, agentResponse["sessionId"])
	}
	if agentResponse["role"] != "test-agent" {
		t.Errorf("Expected role test-agent, got %v", agentResponse["role"])
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
}

// Test_HandleSessionCreate_Success tests session creation success path
func Test_HandleSessionCreate_Success(t *testing.T) {
	server, idGen, _ := setupIntegrationServer()

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
		"version": "1.0",
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
	if response["version"] != "1.0" {
		t.Errorf("Expected version 1.0, got %v", response["version"])
	}
	if response["type"] != "session:created" {
		t.Errorf("Expected type session:created, got %v", response["type"])
	}

	sessionID, ok := response["sessionId"].(string)
	if !ok || sessionID == "" {
		t.Fatal("Missing or invalid sessionId")
	}

	// Verify session ID follows expected pattern from mockIDGenerator
	expectedID := fmt.Sprintf("test-id-%d", idGen.counter)
	if sessionID != expectedID {
		t.Errorf("Expected sessionId %s, got %s", expectedID, sessionID)
	}

	// Verify timestamp exists
	if _, ok := response["timestamp"]; !ok {
		t.Error("Response missing timestamp")
	}
}

// Test_HandleAgentSpawn_Success tests agent spawning success path
func Test_HandleAgentSpawn_Success(t *testing.T) {
	server, _, clientFactory := setupIntegrationServer()

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
		"version": "1.0",
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
	sessionID := sessionCreated["sessionId"].(string)

	// Send agent:spawn
	agentSpawn := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:spawn",
		"sessionId": sessionID,
		"role":      "test-agent",
		"workspace": "./test-workspaces/spawn-test",
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
	if agentReady["version"] != "1.0" {
		t.Errorf("Expected version 1.0, got %v", agentReady["version"])
	}
	if agentReady["type"] != "agent:ready" {
		t.Errorf("Expected type agent:ready, got %v", agentReady["type"])
	}
	if agentReady["sessionId"] != sessionID {
		t.Errorf("Expected sessionId %s, got %v", sessionID, agentReady["sessionId"])
	}
	if agentReady["role"] != "test-agent" {
		t.Errorf("Expected role test-agent, got %v", agentReady["role"])
	}

	// Verify client factory was called
	if count := clientFactory.CallCount(); count != 1 {
		t.Errorf("Expected client factory called once, got %d", count)
	}
}

// Test_HandleAgentMessage_Success_FullFlow tests agent message handling success path
func Test_HandleAgentMessage_Success_FullFlow(t *testing.T) {
	server, _, clientFactory := setupIntegrationServer()

	// Configure mock client to return specific response
	var capturedContent string
	clientFactory.clientFunc = func(workspace string) (session.ACPClient, error) {
		return &integrationMockACPClient{
			sendFunc: func(content string) (interface{}, error) {
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
		"version": "1.0",
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
	json.Unmarshal(message, &sessionCreated)
	sessionID := sessionCreated["sessionId"].(string)

	// Spawn agent
	agentSpawn := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:spawn",
		"sessionId": sessionID,
		"role":      "test-agent",
		"workspace": "./test-workspaces/message-test",
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
		"version":   "1.0",
		"type":      "agent:message",
		"sessionId": sessionID,
		"role":      "test-agent",
		"content":   testContent,
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
	if err := json.Unmarshal(message, &agentResponse); err != nil {
		t.Fatalf("Failed to parse agent:response: %v", err)
	}

	// Verify response structure
	if agentResponse["version"] != "1.0" {
		t.Errorf("Expected version 1.0, got %v", agentResponse["version"])
	}
	if agentResponse["type"] != "agent:response" {
		t.Errorf("Expected type agent:response, got %v", agentResponse["type"])
	}
	if agentResponse["sessionId"] != sessionID {
		t.Errorf("Expected sessionId %s, got %v", sessionID, agentResponse["sessionId"])
	}
	if agentResponse["role"] != "test-agent" {
		t.Errorf("Expected role test-agent, got %v", agentResponse["role"])
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
		"version": "1.0",
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
		"version": "1.0",
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
