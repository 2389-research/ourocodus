package relay

import (
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
