package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// Test helpers for dependency injection
func newTestServer() *Server {
	return NewServer(
		&UUIDGenerator{},
		&StdLogger{},
		&SystemClock{},
		NewGorillaUpgrader(func(r *http.Request) bool { return true }),
	)
}

func TestServer_ConnectionHandshake(t *testing.T) {
	// Create server
	server := newTestServer()

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Convert http://... to ws://...
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	// Connect via WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read connection:established message
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	// Parse message
	var base BaseMessage
	if err := json.Unmarshal(message, &base); err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	// Verify it's a connection:established message
	if base.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", base.Version)
	}
	if base.Type != "connection:established" {
		t.Errorf("expected type connection:established, got %s", base.Type)
	}

	// Parse full message to check for serverId and timestamp
	var fullMsg map[string]interface{}
	if err := json.Unmarshal(message, &fullMsg); err != nil {
		t.Fatalf("Failed to parse full message: %v", err)
	}

	if _, ok := fullMsg["serverId"]; !ok {
		t.Error("message missing serverId field")
	}
	if _, ok := fullMsg["timestamp"]; !ok {
		t.Error("message missing timestamp field")
	}
}

func TestServer_Echo(t *testing.T) {
	// Create server
	server := newTestServer()

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Convert http://... to ws://...
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	// Connect via WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read and discard connection:established message
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read handshake: %v", err)
	}

	// Send test message
	testMsg := map[string]interface{}{
		"version": "1.0",
		"type":    "test:echo",
		"message": "hello",
	}
	err = conn.WriteJSON(testMsg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read echo response
	var response []byte
	_, response, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Parse response
	var echoMsg map[string]interface{}
	err = json.Unmarshal(response, &echoMsg)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify echo has same fields plus timestamp
	if echoMsg["version"] != "1.0" {
		t.Errorf("expected version 1.0, got %v", echoMsg["version"])
	}
	if echoMsg["type"] != "test:echo" {
		t.Errorf("expected type test:echo, got %v", echoMsg["type"])
	}
	if echoMsg["message"] != "hello" {
		t.Errorf("expected message hello, got %v", echoMsg["message"])
	}
	if _, ok := echoMsg["timestamp"]; !ok {
		t.Error("echo message missing timestamp field")
	}
}

func TestServer_VersionMismatch(t *testing.T) {
	// Create server
	server := newTestServer()

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Convert http://... to ws://...
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	// Connect via WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read and discard connection:established message
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read handshake: %v", err)
	}

	// Send message with wrong version
	testMsg := map[string]interface{}{
		"version": "2.0",
		"type":    "test:echo",
	}
	err = conn.WriteJSON(testMsg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read error response
	var response []byte
	_, response, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read error response: %v", err)
	}

	// Parse error message
	var errorMsg ErrorMessage
	err = json.Unmarshal(response, &errorMsg)
	if err != nil {
		t.Fatalf("Failed to parse error message: %v", err)
	}

	// Verify error structure
	if errorMsg.Type != "error" {
		t.Errorf("expected type error, got %s", errorMsg.Type)
	}
	if errorMsg.Error.Code != "VERSION_MISMATCH" {
		t.Errorf("expected code VERSION_MISMATCH, got %s", errorMsg.Error.Code)
	}
	if errorMsg.Error.Recoverable {
		t.Error("expected recoverable=false for version mismatch")
	}

	// Verify connection is closed
	// Try to read again, should get EOF or close error
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected connection to be closed after version mismatch")
	}
}

func TestServer_MissingRequiredField(t *testing.T) {
	// Create server
	server := newTestServer()

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Convert http://... to ws://...
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	// Connect via WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read and discard connection:established message
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read handshake: %v", err)
	}

	// Send message without version
	testMsg := map[string]interface{}{
		"type":    "test:echo",
		"message": "test",
	}
	err = conn.WriteJSON(testMsg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read error response
	var response []byte
	_, response, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read error response: %v", err)
	}

	// Parse error message
	var errorMsg ErrorMessage
	err = json.Unmarshal(response, &errorMsg)
	if err != nil {
		t.Fatalf("Failed to parse error message: %v", err)
	}

	// Verify error structure
	if errorMsg.Type != "error" {
		t.Errorf("expected type error, got %s", errorMsg.Type)
	}
	if errorMsg.Error.Code != "INVALID_MESSAGE" {
		t.Errorf("expected code INVALID_MESSAGE, got %s", errorMsg.Error.Code)
	}
	if !errorMsg.Error.Recoverable {
		t.Error("expected recoverable=true for missing field")
	}

	// Verify connection stays open - send a valid message
	validMsg := map[string]interface{}{
		"version": "1.0",
		"type":    "test:echo",
		"message": "recovered",
	}
	err = conn.WriteJSON(validMsg)
	if err != nil {
		t.Fatalf("Failed to send valid message after error: %v", err)
	}

	// Should receive echo
	_, response, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Connection should still be open after recoverable error: %v", err)
	}

	var echoMsg map[string]interface{}
	err = json.Unmarshal(response, &echoMsg)
	if err != nil {
		t.Fatalf("Failed to parse echo: %v", err)
	}

	if echoMsg["message"] != "recovered" {
		t.Errorf("expected echo of recovered message, got %v", echoMsg["message"])
	}
}

func TestServer_InvalidJSON(t *testing.T) {
	// Create server
	server := newTestServer()

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Convert http://... to ws://...
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	// Connect via WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Read and discard connection:established message
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read handshake: %v", err)
	}

	// Send invalid JSON
	err = conn.WriteMessage(websocket.TextMessage, []byte("{invalid json}"))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read error response
	var response []byte
	_, response, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read error response: %v", err)
	}

	// Parse error message
	var errorMsg ErrorMessage
	err = json.Unmarshal(response, &errorMsg)
	if err != nil {
		t.Fatalf("Failed to parse error message: %v", err)
	}

	// Verify error structure
	if errorMsg.Type != "error" {
		t.Errorf("expected type error, got %s", errorMsg.Type)
	}
	if errorMsg.Error.Code != "INVALID_MESSAGE" {
		t.Errorf("expected code INVALID_MESSAGE, got %s", errorMsg.Error.Code)
	}
	if !errorMsg.Error.Recoverable {
		t.Error("expected recoverable=true for invalid JSON")
	}

	// Verify connection stays open - send a valid message
	validMsg := map[string]interface{}{
		"version": "1.0",
		"type":    "test:echo",
		"message": "recovered",
	}
	err = conn.WriteJSON(validMsg)
	if err != nil {
		t.Fatalf("Failed to send valid message after error: %v", err)
	}

	// Should receive echo
	_, response, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("Connection should still be open after recoverable error: %v", err)
	}

	var echoMsg map[string]interface{}
	err = json.Unmarshal(response, &echoMsg)
	if err != nil {
		t.Fatalf("Failed to parse echo: %v", err)
	}

	if echoMsg["message"] != "recovered" {
		t.Errorf("expected echo of recovered message, got %v", echoMsg["message"])
	}
}
