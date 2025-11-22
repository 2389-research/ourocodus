package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

// TestLog tests basic event logging
func TestLog(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	event := Event{
		Type:    EventAgentAttach,
		UserID:  "user-123",
		AgentID: "agent-456",
		Success: true,
	}

	Log(event)

	output := buf.String()
	if !strings.Contains(output, "[AUDIT]") {
		t.Error("Expected [AUDIT] prefix in log output")
	}

	// Verify JSON structure
	if !strings.Contains(output, `"type":"agent:attach"`) {
		t.Error("Expected type field in JSON")
	}
	if !strings.Contains(output, `"userId":"user-123"`) {
		t.Error("Expected userId field in JSON")
	}
	if !strings.Contains(output, `"agentId":"agent-456"`) {
		t.Error("Expected agentId field in JSON")
	}
	if !strings.Contains(output, `"success":true`) {
		t.Error("Expected success field in JSON")
	}
}

// TestLog_AutomaticTimestamp tests that timestamp is set automatically
func TestLog_AutomaticTimestamp(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	event := Event{
		Type:    EventAgentAttach,
		UserID:  "user-123",
		AgentID: "agent-456",
		Success: true,
		// No timestamp set
	}

	before := time.Now()
	Log(event)
	after := time.Now()

	// Parse JSON from log output
	output := buf.String()
	start := strings.Index(output, "{")
	if start == -1 {
		t.Fatal("No JSON found in output")
	}
	jsonStr := output[start:]

	var logged Event
	if err := json.Unmarshal([]byte(jsonStr), &logged); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify timestamp was set
	if logged.Timestamp.IsZero() {
		t.Error("Timestamp should be automatically set")
	}
	if logged.Timestamp.Before(before) || logged.Timestamp.After(after) {
		t.Error("Timestamp should be between before and after times")
	}
}

// TestLogAgentAttach tests LogAgentAttach helper
func TestLogAgentAttach(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	// Test successful attach
	LogAgentAttach("user-123", "agent-456", true, nil)

	output := buf.String()
	if !strings.Contains(output, `"type":"agent:attach"`) {
		t.Error("Expected agent:attach type")
	}
	if !strings.Contains(output, `"success":true`) {
		t.Error("Expected success:true")
	}

	// Test failed attach with error
	buf.Reset()
	testErr := errors.New("test error")
	LogAgentAttach("user-123", "agent-456", false, testErr)

	output = buf.String()
	if !strings.Contains(output, `"success":false`) {
		t.Error("Expected success:false")
	}
	if !strings.Contains(output, `"error":"test error"`) {
		t.Error("Expected error message in output")
	}
}

// TestLogAgentDetach tests LogAgentDetach helper
func TestLogAgentDetach(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	// Test successful detach
	LogAgentDetach("user-123", "agent-456", true, nil)

	output := buf.String()
	if !strings.Contains(output, `"type":"agent:detach"`) {
		t.Error("Expected agent:detach type")
	}
	if !strings.Contains(output, `"success":true`) {
		t.Error("Expected success:true")
	}

	// Test failed detach with error
	buf.Reset()
	testErr := errors.New("detach failed")
	LogAgentDetach("user-123", "agent-456", false, testErr)

	output = buf.String()
	if !strings.Contains(output, `"success":false`) {
		t.Error("Expected success:false")
	}
	if !strings.Contains(output, `"error":"detach failed"`) {
		t.Error("Expected error message in output")
	}
}

// TestLogAuthFailure tests LogAuthFailure helper
func TestLogAuthFailure(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())

	metadata := map[string]string{
		"operation": "attach",
		"reason":    "invalid_token",
	}

	LogAuthFailure("user-123", "agent-456", "Invalid token", metadata)

	output := buf.String()
	if !strings.Contains(output, `"type":"auth:failure"`) {
		t.Error("Expected auth:failure type")
	}
	if !strings.Contains(output, `"success":false`) {
		t.Error("Expected success:false")
	}
	if !strings.Contains(output, `"error":"Invalid token"`) {
		t.Error("Expected error message in output")
	}
	if !strings.Contains(output, `"metadata"`) {
		t.Error("Expected metadata in output")
	}
}

// TestEvent_JSONMarshaling tests that Event marshals correctly to JSON
func TestEvent_JSONMarshaling(t *testing.T) {
	timestamp := time.Date(2025, 11, 22, 15, 30, 0, 0, time.UTC)

	event := Event{
		Timestamp: timestamp,
		Type:      EventAgentAttach,
		UserID:    "user-123",
		AgentID:   "agent-456",
		Success:   true,
		Error:     "test error",
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	// Verify all fields are present
	jsonStr := string(data)
	expectedFields := []string{
		`"timestamp"`,
		`"type":"agent:attach"`,
		`"userId":"user-123"`,
		`"agentId":"agent-456"`,
		`"success":true`,
		`"error":"test error"`,
		`"metadata"`,
		`"key1":"value1"`,
		`"key2":"value2"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("Expected field %s in JSON: %s", field, jsonStr)
		}
	}

	// Verify we can unmarshal back
	var unmarshaled Event
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	if unmarshaled.Type != event.Type {
		t.Errorf("Expected Type %s, got %s", event.Type, unmarshaled.Type)
	}
	if unmarshaled.UserID != event.UserID {
		t.Errorf("Expected UserID %s, got %s", event.UserID, unmarshaled.UserID)
	}
	if unmarshaled.AgentID != event.AgentID {
		t.Errorf("Expected AgentID %s, got %s", event.AgentID, unmarshaled.AgentID)
	}
	if unmarshaled.Success != event.Success {
		t.Errorf("Expected Success %v, got %v", event.Success, unmarshaled.Success)
	}
}

// TestEventType_Constants tests that event type constants are defined correctly
func TestEventType_Constants(t *testing.T) {
	if EventAgentAttach != "agent:attach" {
		t.Errorf("Expected EventAgentAttach to be 'agent:attach', got %s", EventAgentAttach)
	}
	if EventAgentDetach != "agent:detach" {
		t.Errorf("Expected EventAgentDetach to be 'agent:detach', got %s", EventAgentDetach)
	}
	if EventAuthFailure != "auth:failure" {
		t.Errorf("Expected EventAuthFailure to be 'auth:failure', got %s", EventAuthFailure)
	}
}
