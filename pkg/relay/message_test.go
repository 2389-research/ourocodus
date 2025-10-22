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

	expectedMsg := "missing required field: version"
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

	expectedMsg := "protocol version 2.0 not supported (server supports 1.0)"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
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
