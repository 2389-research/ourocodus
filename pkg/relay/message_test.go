package relay

import (
	"encoding/json"
	"testing"
)

func TestValidateMessage_MissingVersion(t *testing.T) {
	// Message without version field
	data := []byte(`{"type":"test:echo","message":"hello"}`)

	err := ValidateMessage(data)

	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}

	expectedMsg := "Missing required field: version"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestValidateMessage_VersionMismatch(t *testing.T) {
	// Message with wrong version
	data := []byte(`{"version":"2.0","type":"test:echo"}`)

	err := ValidateMessage(data)

	if err == nil {
		t.Fatal("expected error for version mismatch, got nil")
	}

	expectedMsg := "Protocol version 2.0 not supported (server supports 1.0)"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestValidateMessage_MissingType(t *testing.T) {
	// Message without type field
	data := []byte(`{"version":"1.0","message":"hello"}`)

	err := ValidateMessage(data)

	if err == nil {
		t.Fatal("expected error for missing type, got nil")
	}

	expectedMsg := "Missing required field: type"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}

	// Check it's a recoverable ValidationError
	if verr, ok := err.(ValidationError); ok {
		if verr.Code != "INVALID_MESSAGE" {
			t.Errorf("expected code INVALID_MESSAGE, got %s", verr.Code)
		}
		if !verr.Recoverable {
			t.Error("expected recoverable=true for missing field")
		}
	} else {
		t.Error("expected ValidationError type")
	}
}

func TestValidateMessage_InvalidJSON(t *testing.T) {
	// Invalid JSON
	data := []byte(`{invalid json}`)

	err := ValidateMessage(data)

	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	// Check it's a recoverable ValidationError
	if verr, ok := err.(ValidationError); ok {
		if verr.Code != "INVALID_MESSAGE" {
			t.Errorf("expected code INVALID_MESSAGE, got %s", verr.Code)
		}
		if !verr.Recoverable {
			t.Error("expected recoverable=true for invalid JSON")
		}
	} else {
		t.Error("expected ValidationError type")
	}
}

func TestValidateMessage_VersionMismatchNotRecoverable(t *testing.T) {
	// Message with wrong version should be non-recoverable
	data := []byte(`{"version":"2.0","type":"test:echo"}`)

	err := ValidateMessage(data)

	if err == nil {
		t.Fatal("expected error for version mismatch, got nil")
	}

	// Check it's a non-recoverable ValidationError
	if verr, ok := err.(ValidationError); ok {
		if verr.Code != "VERSION_MISMATCH" {
			t.Errorf("expected code VERSION_MISMATCH, got %s", verr.Code)
		}
		if verr.Recoverable {
			t.Error("expected recoverable=false for version mismatch")
		}
	} else {
		t.Error("expected ValidationError type")
	}
}

func TestValidateMessage_ValidMessage(t *testing.T) {
	// Valid message with correct version and type
	data := []byte(`{"version":"1.0","type":"test:echo","message":"hello"}`)

	err := ValidateMessage(data)
	if err != nil {
		t.Fatalf("expected no error for valid message, got: %v", err)
	}
}

// TestParseSessionCreateMessage tests parsing of session:create messages
func TestParseSessionCreateMessage(t *testing.T) {
	data := []byte(`{"version":"1.0","type":"session:create"}`)

	msg, err := parseSessionCreateMessage(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if msg.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", msg.Version)
	}
	if msg.Type != "session:create" {
		t.Errorf("expected type session:create, got %s", msg.Type)
	}
}

// TestParseSessionCreateMessage_InvalidJSON tests parsing with invalid JSON
func TestParseSessionCreateMessage_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid}`)

	_, err := parseSessionCreateMessage(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	if verr, ok := err.(ValidationError); ok {
		if verr.Code != "INVALID_MESSAGE" {
			t.Errorf("expected code INVALID_MESSAGE, got %s", verr.Code)
		}
	} else {
		t.Error("expected ValidationError type")
	}
}

// TestValidateSessionCreateMessage tests validation (should always pass after base validation)
func TestValidateSessionCreateMessage(t *testing.T) {
	msg := SessionCreateMessage{
		BaseMessage: BaseMessage{Version: "1.0", Type: "session:create"},
	}

	err := validateSessionCreateMessage(msg)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestParseAgentSpawnMessage tests parsing of agent:spawn messages
func TestParseAgentSpawnMessage(t *testing.T) {
	data := []byte(`{"version":"1.0","type":"agent:spawn","userSessionId":"sess123","agentId":"auth","workspace":"/path/to/workspace"}`)

	msg, err := parseAgentSpawnMessage(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if msg.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", msg.Version)
	}
	if msg.UserSessionID != "sess123" {
		t.Errorf("expected userSessionId sess123, got %s", msg.UserSessionID)
	}
	if msg.AgentID != "auth" {
		t.Errorf("expected agentId auth, got %s", msg.AgentID)
	}
	if msg.Workspace != "/path/to/workspace" {
		t.Errorf("expected workspace /path/to/workspace, got %s", msg.Workspace)
	}
}

// TestValidateAgentSpawnMessage_MissingSessionID tests validation with missing userSessionId
func TestValidateAgentSpawnMessage_MissingSessionID(t *testing.T) {
	msg := AgentSpawnMessage{
		BaseMessage: BaseMessage{Version: "1.0", Type: "agent:spawn"},
		AgentID:     "auth",
		Workspace:   "/path",
	}

	err := validateAgentSpawnMessage(msg)
	if err == nil {
		t.Fatal("expected error for missing userSessionId, got nil")
	}

	expectedMsg := "Missing required field: userSessionId"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateAgentSpawnMessage_MissingRole tests validation with missing agentId
func TestValidateAgentSpawnMessage_MissingRole(t *testing.T) {
	msg := AgentSpawnMessage{
		BaseMessage:   BaseMessage{Version: "1.0", Type: "agent:spawn"},
		UserSessionID: "sess123",
		Workspace:     "/path",
	}

	err := validateAgentSpawnMessage(msg)
	if err == nil {
		t.Fatal("expected error for missing agentId, got nil")
	}

	expectedMsg := "Missing required field: agentId"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateAgentSpawnMessage_MissingWorkspace tests validation with missing workspace
func TestValidateAgentSpawnMessage_MissingWorkspace(t *testing.T) {
	msg := AgentSpawnMessage{
		BaseMessage:   BaseMessage{Version: "1.0", Type: "agent:spawn"},
		UserSessionID: "sess123",
		AgentID:       "auth",
	}

	err := validateAgentSpawnMessage(msg)
	if err == nil {
		t.Fatal("expected error for missing workspace, got nil")
	}

	expectedMsg := "Missing required field: workspace"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateAgentSpawnMessage_Valid tests validation with all fields
func TestValidateAgentSpawnMessage_Valid(t *testing.T) {
	msg := AgentSpawnMessage{
		BaseMessage:   BaseMessage{Version: "1.0", Type: "agent:spawn"},
		UserSessionID: "sess123",
		AgentID:       "auth",
		Workspace:     "/path",
	}

	err := validateAgentSpawnMessage(msg)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestParseAgentMessageRequest tests parsing of agent:message messages
func TestParseAgentMessageRequest(t *testing.T) {
	data := []byte(`{"version":"1.0","type":"agent:message","userSessionId":"sess123","agentId":"auth","content":"implement JWT"}`)

	msg, err := parseAgentMessageRequest(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if msg.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", msg.Version)
	}
	if msg.UserSessionID != "sess123" {
		t.Errorf("expected userSessionId sess123, got %s", msg.UserSessionID)
	}
	if msg.AgentID != "auth" {
		t.Errorf("expected agentId auth, got %s", msg.AgentID)
	}
	if msg.Content != "implement JWT" {
		t.Errorf("expected content 'implement JWT', got %s", msg.Content)
	}
}

// TestValidateAgentMessageRequest_MissingSessionID tests validation with missing userSessionId
func TestValidateAgentMessageRequest_MissingSessionID(t *testing.T) {
	msg := AgentMessageRequest{
		BaseMessage: BaseMessage{Version: "1.0", Type: "agent:message"},
		AgentID:     "auth",
		Content:     "test",
	}

	err := validateAgentMessageRequest(msg)
	if err == nil {
		t.Fatal("expected error for missing userSessionId, got nil")
	}

	expectedMsg := "Missing required field: userSessionId"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateAgentMessageRequest_MissingRole tests validation with missing agentId
func TestValidateAgentMessageRequest_MissingRole(t *testing.T) {
	msg := AgentMessageRequest{
		BaseMessage:   BaseMessage{Version: "1.0", Type: "agent:message"},
		UserSessionID: "sess123",
		Content:       "test",
	}

	err := validateAgentMessageRequest(msg)
	if err == nil {
		t.Fatal("expected error for missing agentId, got nil")
	}

	expectedMsg := "Missing required field: agentId"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateAgentMessageRequest_MissingContent tests validation with missing content
func TestValidateAgentMessageRequest_MissingContent(t *testing.T) {
	msg := AgentMessageRequest{
		BaseMessage:   BaseMessage{Version: "1.0", Type: "agent:message"},
		UserSessionID: "sess123",
		AgentID:       "auth",
	}

	err := validateAgentMessageRequest(msg)
	if err == nil {
		t.Fatal("expected error for missing content, got nil")
	}

	expectedMsg := "Missing required field: content"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateAgentMessageRequest_Valid tests validation with all fields
func TestValidateAgentMessageRequest_Valid(t *testing.T) {
	msg := AgentMessageRequest{
		BaseMessage:   BaseMessage{Version: "1.0", Type: "agent:message"},
		UserSessionID: "sess123",
		AgentID:       "auth",
		Content:       "test",
	}

	err := validateAgentMessageRequest(msg)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestNewSessionCreatedMessage tests constructor function
func TestNewSessionCreatedMessage(t *testing.T) {
	msg := NewSessionCreatedMessage("sess123", "2025-10-27T12:00:00Z")

	if msg.Version != ProtocolVersion {
		t.Errorf("expected version %s, got %s", ProtocolVersion, msg.Version)
	}
	if msg.Type != "session:created" {
		t.Errorf("expected type session:created, got %s", msg.Type)
	}
	if msg.UserSessionID != "sess123" {
		t.Errorf("expected userSessionId sess123, got %s", msg.UserSessionID)
	}
	if msg.Timestamp != "2025-10-27T12:00:00Z" {
		t.Errorf("expected timestamp 2025-10-27T12:00:00Z, got %s", msg.Timestamp)
	}
}

// TestNewAgentReadyMessage tests constructor function
func TestNewAgentReadyMessage(t *testing.T) {
	msg := NewAgentReadyMessage("sess123", "auth")

	if msg.Version != ProtocolVersion {
		t.Errorf("expected version %s, got %s", ProtocolVersion, msg.Version)
	}
	if msg.Type != "agent:ready" {
		t.Errorf("expected type agent:ready, got %s", msg.Type)
	}
	if msg.UserSessionID != "sess123" {
		t.Errorf("expected userSessionId sess123, got %s", msg.UserSessionID)
	}
	if msg.AgentID != "auth" {
		t.Errorf("expected agentId auth, got %s", msg.AgentID)
	}
}

// TestNewAgentMessageResponse tests constructor function
func TestNewAgentMessageResponse(t *testing.T) {
	msg := NewAgentMessageResponse("sess123", "auth", "JWT implemented", "2025-10-27T12:00:00Z")

	if msg.Version != ProtocolVersion {
		t.Errorf("expected version %s, got %s", ProtocolVersion, msg.Version)
	}
	if msg.Type != "agent:response" {
		t.Errorf("expected type agent:response, got %s", msg.Type)
	}
	if msg.UserSessionID != "sess123" {
		t.Errorf("expected userSessionId sess123, got %s", msg.UserSessionID)
	}
	if msg.AgentID != "auth" {
		t.Errorf("expected agentId auth, got %s", msg.AgentID)
	}
	if msg.Content != "JWT implemented" {
		t.Errorf("expected content 'JWT implemented', got %s", msg.Content)
	}
	if msg.Timestamp != "2025-10-27T12:00:00Z" {
		t.Errorf("expected timestamp 2025-10-27T12:00:00Z, got %s", msg.Timestamp)
	}
}

// TestAgentStreamDelta_TextDelta tests text delta message
func TestAgentStreamDelta_TextDelta(t *testing.T) {
	msg := NewAgentStreamDelta("sess-123", "auth", "text.delta", "Hello", false)

	if msg.Version != ProtocolVersion {
		t.Errorf("expected version %s, got %s", ProtocolVersion, msg.Version)
	}
	if msg.Type != "agent:stream-delta" {
		t.Errorf("expected type agent:stream-delta, got %s", msg.Type)
	}
	if msg.UserSessionID != "sess-123" {
		t.Errorf("expected userSessionId sess-123, got %s", msg.UserSessionID)
	}
	if msg.AgentID != "auth" {
		t.Errorf("expected agentId auth, got %s", msg.AgentID)
	}
	if msg.EventType != "text.delta" {
		t.Errorf("expected eventType text.delta, got %s", msg.EventType)
	}
	if msg.Delta != "Hello" {
		t.Errorf("expected delta Hello, got %s", msg.Delta)
	}
	if msg.Final != false {
		t.Errorf("expected final false, got %v", msg.Final)
	}
}

// TestAgentStreamDelta_ToolCall tests tool call message
func TestAgentStreamDelta_ToolCall(t *testing.T) {
	tool := StreamToolCall{
		ToolName:  "bash",
		ToolInput: "{\"command\":\"ls -la\"}",
	}
	msg := NewAgentStreamDeltaToolCall("sess-123", "auth", tool)

	if msg.Version != ProtocolVersion {
		t.Errorf("expected version %s, got %s", ProtocolVersion, msg.Version)
	}
	if msg.Type != "agent:stream-delta" {
		t.Errorf("expected type agent:stream-delta, got %s", msg.Type)
	}
	if msg.EventType != "tool.call" {
		t.Errorf("expected eventType tool.call, got %s", msg.EventType)
	}
	if msg.ToolCall == nil {
		t.Fatal("expected toolCall to be set")
	}
	if msg.ToolCall.ToolName != "bash" {
		t.Errorf("expected toolName bash, got %s", msg.ToolCall.ToolName)
	}
	if msg.ToolCall.ToolInput != "{\"command\":\"ls -la\"}" {
		t.Errorf("expected toolInput, got %s", msg.ToolCall.ToolInput)
	}
}

// TestAgentStreamDelta_SessionEnd tests session end message
func TestAgentStreamDelta_SessionEnd(t *testing.T) {
	msg := NewAgentStreamDelta("sess-123", "auth", "session.end", "", true)

	if msg.EventType != "session.end" {
		t.Errorf("expected eventType session.end, got %s", msg.EventType)
	}
	if msg.Final != true {
		t.Errorf("expected final true, got %v", msg.Final)
	}
}

// TestAgentStreamDelta_Marshal tests JSON serialization
func TestAgentStreamDelta_Marshal(t *testing.T) {
	msg := NewAgentStreamDelta("sess-123", "auth", "text.delta", "Hello", false)

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed["version"] != ProtocolVersion {
		t.Errorf("expected version %s, got %v", ProtocolVersion, parsed["version"])
	}
	if parsed["type"] != "agent:stream-delta" {
		t.Errorf("expected type agent:stream-delta, got %v", parsed["type"])
	}
	if parsed["eventType"] != "text.delta" {
		t.Errorf("expected eventType text.delta, got %v", parsed["eventType"])
	}
	if parsed["delta"] != "Hello" {
		t.Errorf("expected delta Hello, got %v", parsed["delta"])
	}
}
