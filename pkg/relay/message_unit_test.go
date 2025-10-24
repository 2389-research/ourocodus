package relay

import (
	"testing"
)

// Unit tests for decomposed validation functions

func TestParseMessage_ValidJSON(t *testing.T) {
	data := []byte(`{"version":"1.0","type":"test:echo","message":"hello"}`)

	base, err := parseMessage(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if base.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", base.Version)
	}
	if base.Type != "test:echo" {
		t.Errorf("expected type test:echo, got %s", base.Type)
	}
}

func TestParseMessage_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json}`)

	_, err := parseMessage(data)

	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatal("expected ValidationError")
	}
	if verr.Code != "INVALID_MESSAGE" {
		t.Errorf("expected code INVALID_MESSAGE, got %s", verr.Code)
	}
	if !verr.Recoverable {
		t.Error("expected recoverable=true for invalid JSON")
	}
}

func TestParseMessage_EmptyJSON(t *testing.T) {
	data := []byte(`{}`)

	base, err := parseMessage(data)
	if err != nil {
		t.Fatalf("expected no error for empty JSON, got: %v", err)
	}
	if base.Version != "" {
		t.Errorf("expected empty version, got %s", base.Version)
	}
	if base.Type != "" {
		t.Errorf("expected empty type, got %s", base.Type)
	}
}

func TestValidateRequiredFields_AllPresent(t *testing.T) {
	base := BaseMessage{
		Version: "1.0",
		Type:    "test:echo",
	}

	err := validateRequiredFields(base)
	if err != nil {
		t.Errorf("expected no error when all fields present, got: %v", err)
	}
}

func TestValidateRequiredFields_MissingVersion(t *testing.T) {
	base := BaseMessage{
		Version: "",
		Type:    "test:echo",
	}

	err := validateRequiredFields(base)

	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}

	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatal("expected ValidationError")
	}
	if verr.Code != "INVALID_MESSAGE" {
		t.Errorf("expected code INVALID_MESSAGE, got %s", verr.Code)
	}
	if verr.Message != "Missing required field: version" {
		t.Errorf("unexpected error message: %s", verr.Message)
	}
	if !verr.Recoverable {
		t.Error("expected recoverable=true for missing field")
	}
}

func TestValidateRequiredFields_MissingType(t *testing.T) {
	base := BaseMessage{
		Version: "1.0",
		Type:    "",
	}

	err := validateRequiredFields(base)

	if err == nil {
		t.Fatal("expected error for missing type, got nil")
	}

	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatal("expected ValidationError")
	}
	if verr.Code != "INVALID_MESSAGE" {
		t.Errorf("expected code INVALID_MESSAGE, got %s", verr.Code)
	}
	if verr.Message != "Missing required field: type" {
		t.Errorf("unexpected error message: %s", verr.Message)
	}
	if !verr.Recoverable {
		t.Error("expected recoverable=true for missing field")
	}
}

func TestValidateRequiredFields_BothMissing(t *testing.T) {
	base := BaseMessage{
		Version: "",
		Type:    "",
	}

	err := validateRequiredFields(base)

	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}

	// Should return error for first missing field (version)
	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatal("expected ValidationError")
	}
	if verr.Message != "Missing required field: version" {
		t.Errorf("expected version error first, got: %s", verr.Message)
	}
}

func TestValidateVersion_ValidVersion(t *testing.T) {
	err := validateVersion("1.0")
	if err != nil {
		t.Errorf("expected no error for valid version, got: %v", err)
	}
}

func TestValidateVersion_WrongVersion(t *testing.T) {
	err := validateVersion("2.0")

	if err == nil {
		t.Fatal("expected error for wrong version, got nil")
	}

	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatal("expected ValidationError")
	}
	if verr.Code != "VERSION_MISMATCH" {
		t.Errorf("expected code VERSION_MISMATCH, got %s", verr.Code)
	}
	if verr.Recoverable {
		t.Error("expected recoverable=false for version mismatch")
	}
}

func TestValidateVersion_EmptyVersion(t *testing.T) {
	err := validateVersion("")

	if err == nil {
		t.Fatal("expected error for empty version, got nil")
	}

	verr, ok := err.(ValidationError)
	if !ok {
		t.Fatal("expected ValidationError")
	}
	if verr.Code != "VERSION_MISMATCH" {
		t.Errorf("expected code VERSION_MISMATCH, got %s", verr.Code)
	}
}

func TestNewConnectionEstablished_Pure(t *testing.T) {
	serverID := "test-server-123"
	timestamp := "2025-10-23T12:00:00Z"

	msg := NewConnectionEstablished(serverID, timestamp)

	if msg.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", msg.Version)
	}
	if msg.Type != "connection:established" {
		t.Errorf("expected type connection:established, got %s", msg.Type)
	}
	if msg.ServerID != serverID {
		t.Errorf("expected serverID %s, got %s", serverID, msg.ServerID)
	}
	if msg.Timestamp != timestamp {
		t.Errorf("expected timestamp %s, got %s", timestamp, msg.Timestamp)
	}
}

func TestNewConnectionEstablished_Deterministic(t *testing.T) {
	// Test that same inputs always produce same output (pure function)
	serverID := "test-server"
	timestamp := "2025-10-23T12:00:00Z"

	msg1 := NewConnectionEstablished(serverID, timestamp)
	msg2 := NewConnectionEstablished(serverID, timestamp)

	if msg1.ServerID != msg2.ServerID {
		t.Error("function should be deterministic with same inputs")
	}
	if msg1.Timestamp != msg2.Timestamp {
		t.Error("function should be deterministic with same inputs")
	}
}

func TestNewErrorMessage_Structure(t *testing.T) {
	code := "TEST_ERROR"
	message := "Test error message"
	recoverable := true

	errorMsg := NewErrorMessage(code, message, recoverable)

	if errorMsg.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", errorMsg.Version)
	}
	if errorMsg.Type != "error" {
		t.Errorf("expected type error, got %s", errorMsg.Type)
	}
	if errorMsg.Error.Code != code {
		t.Errorf("expected code %s, got %s", code, errorMsg.Error.Code)
	}
	if errorMsg.Error.Message != message {
		t.Errorf("expected message %s, got %s", message, errorMsg.Error.Message)
	}
	if errorMsg.Error.Recoverable != recoverable {
		t.Errorf("expected recoverable %v, got %v", recoverable, errorMsg.Error.Recoverable)
	}
}
