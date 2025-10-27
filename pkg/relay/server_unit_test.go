package relay

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// Mock implementations for unit testing

type mockLogger struct {
	logs []string
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	// Store logs for verification
	m.logs = append(m.logs, format)
}

type mockClock struct {
	timestamp string
}

func (m *mockClock) Now() string {
	return m.timestamp
}

type mockIDGenerator struct {
	id string
}

func (m *mockIDGenerator) Generate() string {
	return m.id
}

type mockUpgrader struct {
	conn  WebSocketConn
	error error
}

func (m *mockUpgrader) Upgrade(w interface{}, r interface{}, responseHeader interface{}) (WebSocketConn, error) {
	return m.conn, m.error
}

type mockWebSocketConn struct {
	written       []interface{}
	messageToRead []byte
	readError     error
	writeError    error
	closed        bool
}

func (m *mockWebSocketConn) WriteJSON(v interface{}) error {
	if m.writeError != nil {
		return m.writeError
	}
	m.written = append(m.written, v)
	return nil
}

func (m *mockWebSocketConn) ReadMessage() (int, []byte, error) {
	return 1, m.messageToRead, m.readError
}

func (m *mockWebSocketConn) Close() error {
	m.closed = true
	return nil
}

// Unit tests for server methods

func TestAddTimestamp(t *testing.T) {
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	server := &Server{clock: clock}

	msg := map[string]interface{}{
		"version": "1.0",
		"type":    "test:echo",
	}

	server.addTimestamp(msg)

	if msg["timestamp"] != "2025-10-23T12:00:00Z" {
		t.Errorf("expected timestamp 2025-10-23T12:00:00Z, got %v", msg["timestamp"])
	}
}

func TestAddTimestamp_PreservesExistingFields(t *testing.T) {
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	server := &Server{clock: clock}

	msg := map[string]interface{}{
		"version": "1.0",
		"type":    "test:echo",
		"data":    "important data",
	}

	server.addTimestamp(msg)

	if msg["data"] != "important data" {
		t.Error("addTimestamp should preserve existing fields")
	}
	if msg["version"] != "1.0" {
		t.Error("addTimestamp should preserve existing fields")
	}
}

func TestSendHandshake_Success(t *testing.T) {
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	conn := &mockWebSocketConn{}
	server := &Server{
		serverID: "test-server-123",
		logger:   logger,
		clock:    clock,
	}

	err := server.sendHandshake(conn)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(conn.written) != 1 {
		t.Fatalf("expected 1 message written, got %d", len(conn.written))
	}

	handshake, ok := conn.written[0].(ConnectionEstablishedMessage)
	if !ok {
		t.Fatal("expected ConnectionEstablishedMessage")
	}

	if handshake.ServerID != "test-server-123" {
		t.Errorf("expected serverID test-server-123, got %s", handshake.ServerID)
	}
	if handshake.Timestamp != "2025-10-23T12:00:00Z" {
		t.Errorf("expected timestamp from clock, got %s", handshake.Timestamp)
	}
}

func TestSendHandshake_WriteError(t *testing.T) {
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	conn := &mockWebSocketConn{
		writeError: errors.New("write failed"),
	}
	server := &Server{
		serverID: "test-server-123",
		logger:   logger,
		clock:    clock,
	}

	err := server.sendHandshake(conn)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "write failed" {
		t.Errorf("expected 'write failed', got %v", err)
	}

	// Verify logger was called
	if len(logger.logs) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(logger.logs))
	}
}

func TestHandleValidationError_Recoverable(t *testing.T) {
	logger := &mockLogger{}
	conn := &mockWebSocketConn{}
	server := &Server{logger: logger}

	validationErr := ValidationError{
		Code:        "INVALID_MESSAGE",
		Message:     "Missing field",
		Recoverable: true,
	}

	shouldClose := server.handleValidationError(conn, validationErr)

	if shouldClose {
		t.Error("expected shouldClose=false for recoverable error")
	}

	if len(conn.written) != 1 {
		t.Fatalf("expected 1 error message written, got %d", len(conn.written))
	}

	errorMsg, ok := conn.written[0].(ErrorMessage)
	if !ok {
		t.Fatal("expected ErrorMessage")
	}

	if errorMsg.Error.Code != "INVALID_MESSAGE" {
		t.Errorf("expected code INVALID_MESSAGE, got %s", errorMsg.Error.Code)
	}
	if !errorMsg.Error.Recoverable {
		t.Error("expected recoverable=true in error message")
	}
}

func TestHandleValidationError_NonRecoverable(t *testing.T) {
	logger := &mockLogger{}
	conn := &mockWebSocketConn{}
	server := &Server{logger: logger}

	validationErr := ValidationError{
		Code:        "VERSION_MISMATCH",
		Message:     "Wrong version",
		Recoverable: false,
	}

	shouldClose := server.handleValidationError(conn, validationErr)

	if !shouldClose {
		t.Error("expected shouldClose=true for non-recoverable error")
	}

	if len(conn.written) != 1 {
		t.Fatalf("expected 1 error message written, got %d", len(conn.written))
	}

	errorMsg, ok := conn.written[0].(ErrorMessage)
	if !ok {
		t.Fatal("expected ErrorMessage")
	}

	if errorMsg.Error.Code != "VERSION_MISMATCH" {
		t.Errorf("expected code VERSION_MISMATCH, got %s", errorMsg.Error.Code)
	}
	if errorMsg.Error.Recoverable {
		t.Error("expected recoverable=false in error message")
	}
}

func TestHandleValidationError_NonValidationError(t *testing.T) {
	logger := &mockLogger{}
	conn := &mockWebSocketConn{}
	server := &Server{logger: logger}

	// Generic error should be treated as recoverable INVALID_MESSAGE
	genericErr := errors.New("some random error")

	shouldClose := server.handleValidationError(conn, genericErr)

	if shouldClose {
		t.Error("expected shouldClose=false for generic error (fallback to recoverable)")
	}

	if len(conn.written) != 1 {
		t.Fatalf("expected 1 error message written, got %d", len(conn.written))
	}

	errorMsg, ok := conn.written[0].(ErrorMessage)
	if !ok {
		t.Fatal("expected ErrorMessage")
	}

	if errorMsg.Error.Code != "INVALID_MESSAGE" {
		t.Errorf("expected fallback code INVALID_MESSAGE, got %s", errorMsg.Error.Code)
	}
	if !errorMsg.Error.Recoverable {
		t.Error("expected fallback to recoverable=true")
	}
}

func TestHandleEcho_Success(t *testing.T) {
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	conn := &mockWebSocketConn{}
	server := &Server{
		logger: logger,
		clock:  clock,
	}

	rawMessage := []byte(`{"version":"1.0","type":"test:echo","message":"hello"}`)

	shouldClose := server.handleEcho(conn, rawMessage)
	if shouldClose {
		t.Error("expected shouldClose=false for successful echo")
	}

	if len(conn.written) != 1 {
		t.Fatalf("expected 1 message written, got %d", len(conn.written))
	}

	echoMsg, ok := conn.written[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}

	if echoMsg["message"] != "hello" {
		t.Errorf("expected message 'hello', got %v", echoMsg["message"])
	}
	if echoMsg["timestamp"] != "2025-10-23T12:00:00Z" {
		t.Errorf("expected timestamp added, got %v", echoMsg["timestamp"])
	}
}

func TestHandleEcho_InvalidJSON(t *testing.T) {
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	conn := &mockWebSocketConn{}
	server := &Server{
		logger: logger,
		clock:  clock,
	}

	rawMessage := []byte(`{invalid json}`)

	shouldClose := server.handleEcho(conn, rawMessage)

	if !shouldClose {
		t.Error("expected shouldClose=true for invalid JSON")
	}

	// Should log error
	if len(logger.logs) == 0 {
		t.Error("expected error to be logged")
	}

	// Should not write anything
	if len(conn.written) != 0 {
		t.Errorf("expected nothing written on error, got %d messages", len(conn.written))
	}
}

func TestRouteMessage_ValidEchoMessage(t *testing.T) {
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	conn := &mockWebSocketConn{}
	server := &Server{
		logger: logger,
		clock:  clock,
	}

	ctx := context.Background()
	rawMessage := []byte(`{"version":"1.0","type":"test:echo","message":"hello"}`)

	shouldClose := server.routeMessage(ctx, conn, rawMessage)

	if shouldClose {
		t.Error("expected shouldClose=false for valid message")
	}

	// Should write echo
	if len(conn.written) != 1 {
		t.Fatalf("expected 1 message written, got %d", len(conn.written))
	}
}

func TestRouteMessage_ValidationError(t *testing.T) {
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	conn := &mockWebSocketConn{}
	server := &Server{
		logger: logger,
		clock:  clock,
	}

	ctx := context.Background()
	// Missing version field
	rawMessage := []byte(`{"type":"test:echo","message":"hello"}`)

	shouldClose := server.routeMessage(ctx, conn, rawMessage)

	if shouldClose {
		t.Error("expected shouldClose=false for recoverable validation error")
	}

	// Should write error message
	if len(conn.written) != 1 {
		t.Fatalf("expected 1 error message written, got %d", len(conn.written))
	}

	errorMsg, ok := conn.written[0].(ErrorMessage)
	if !ok {
		t.Fatal("expected ErrorMessage")
	}

	if errorMsg.Type != "error" {
		t.Errorf("expected type 'error', got %s", errorMsg.Type)
	}
}

func TestRouteMessage_VersionMismatch(t *testing.T) {
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	conn := &mockWebSocketConn{}
	server := &Server{
		logger: logger,
		clock:  clock,
	}

	ctx := context.Background()
	// Wrong version - non-recoverable
	rawMessage := []byte(`{"version":"2.0","type":"test:echo"}`)

	shouldClose := server.routeMessage(ctx, conn, rawMessage)

	if !shouldClose {
		t.Error("expected shouldClose=true for version mismatch")
	}

	// Should write error message
	if len(conn.written) != 1 {
		t.Fatalf("expected 1 error message written, got %d", len(conn.written))
	}
}

func TestNewServer_UsesIDGenerator(t *testing.T) {
	idGen := &mockIDGenerator{id: "test-server-123"}
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	upgrader := &mockUpgrader{}

	// sessionManager not used in this test, pass nil
	server := NewServer(idGen, logger, clock, upgrader, nil)

	if server.serverID != "test-server-123" {
		t.Errorf("expected serverID test-server-123, got %s", server.serverID)
	}
}

func TestNewServer_InjectsDependencies(t *testing.T) {
	idGen := &mockIDGenerator{id: "test-id"}
	logger := &mockLogger{}
	clock := &mockClock{timestamp: "2025-10-23T12:00:00Z"}
	upgrader := &mockUpgrader{}

	// Test with nil sessionManager - just verify the field is properly injected
	server := NewServer(idGen, logger, clock, upgrader, nil)

	// Verify all dependencies are set
	if server.logger == nil {
		t.Error("expected logger to be set")
	}
	if server.clock == nil {
		t.Error("expected clock to be set")
	}
	if server.upgrader == nil {
		t.Error("expected upgrader to be set")
	}
	// sessionManager can be nil in this test, we're just verifying the field exists
}

func TestMapError_GenericError(t *testing.T) {
	server := &Server{logger: &mockLogger{}}

	// Test generic error that's not a session sentinel
	genericErr := errors.New("some random error")

	code, _, recoverable := server.mapError(genericErr)

	// Should map to INTERNAL_ERROR (recoverable fallback)
	if code != "INTERNAL_ERROR" {
		t.Errorf("expected code INTERNAL_ERROR for non-sentinel error, got %s", code)
	}
	if !recoverable {
		t.Error("expected recoverable=true for unknown error")
	}
}

func TestMapError_ValidationError(t *testing.T) {
	server := &Server{logger: &mockLogger{}}

	validationErr := ValidationError{
		Code:        "MISSING_FIELD",
		Message:     "Missing required field: foo",
		Recoverable: false,
	}

	code, message, recoverable := server.mapError(validationErr)

	if code != "MISSING_FIELD" {
		t.Errorf("expected code MISSING_FIELD, got %s", code)
	}
	if message != "Missing required field: foo" {
		t.Errorf("expected original message, got %s", message)
	}
	if recoverable {
		t.Error("expected recoverable=false from ValidationError")
	}
}

func TestMapError_UnknownError(t *testing.T) {
	server := &Server{logger: &mockLogger{}}

	unknownErr := errors.New("something unexpected happened")

	code, message, recoverable := server.mapError(unknownErr)

	if code != "INTERNAL_ERROR" {
		t.Errorf("expected code INTERNAL_ERROR, got %s", code)
	}
	if message != "something unexpected happened" {
		t.Errorf("expected error message, got %s", message)
	}
	if !recoverable {
		t.Error("expected recoverable=true for unknown errors")
	}
}

// Integration tests with actual session errors

func TestMapError_SessionNotFoundSentinel(t *testing.T) {
	server := &Server{logger: &mockLogger{}}

	// Wrap session.ErrSessionNotFound as GetAgent would
	err := fmt.Errorf("%w: test-session", session.ErrSessionNotFound)

	code, _, recoverable := server.mapError(err)

	if code != "SESSION_NOT_FOUND" {
		t.Errorf("expected code SESSION_NOT_FOUND, got %s", code)
	}
	if recoverable {
		t.Error("expected recoverable=false for session not found")
	}
}

func TestMapError_AgentNotFoundSentinel(t *testing.T) {
	server := &Server{logger: &mockLogger{}}

	// Wrap session.ErrAgentNotFound as GetAgent would
	err := fmt.Errorf("%w: role=auth session=test-session", session.ErrAgentNotFound)

	code, _, recoverable := server.mapError(err)

	if code != "AGENT_NOT_FOUND" {
		t.Errorf("expected code AGENT_NOT_FOUND, got %s", code)
	}
	if recoverable {
		t.Error("expected recoverable=false for agent not found")
	}
}

// TODO: Add handler tests for handleSessionCreate, handleAgentSpawn, handleAgentMessage
// Requires creating a SessionManager interface to allow mocking
// Deferred to follow-up work
