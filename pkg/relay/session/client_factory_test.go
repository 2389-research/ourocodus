package session

import (
	"testing"
)

// TestACPClientFactory_MissingAPIKey tests error when ANTHROPIC_API_KEY not set
func TestACPClientFactory_MissingAPIKey(t *testing.T) {
	// Unset the key (t.Setenv automatically restores original value)
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := NewACPClientFactory()
	if err == nil {
		t.Fatal("Expected error when ANTHROPIC_API_KEY not set, got nil")
	}
	if err != ErrMissingAnthropicAPIKey {
		t.Errorf("Expected ErrMissingAnthropicAPIKey, got: %v", err)
	}
}

// TestACPClientFactory_WithAPIKey tests successful factory creation
func TestACPClientFactory_WithAPIKey(t *testing.T) {
	// Set API key (t.Setenv automatically restores original value)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	factory, err := NewACPClientFactory()
	if err != nil {
		t.Fatalf("Expected no error with API key set, got: %v", err)
	}
	if factory == nil {
		t.Fatal("Expected factory, got nil")
	}
}

// TestFakeClientFactory tests the fake factory
func TestFakeClientFactory(t *testing.T) {
	called := false
	factory := NewFakeClientFactory(func(workspace string) (ACPClient, error) {
		called = true
		return &mockACPClient{}, nil
	})

	client, err := factory.NewClient("test-workspace")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if client == nil {
		t.Fatal("Expected client, got nil")
	}
	if !called {
		t.Error("Expected clientFunc to be called")
	}
}

// TestACPClientFactory_CustomBinary tests that custom ACP binary path is respected
func TestACPClientFactory_CustomBinary(t *testing.T) {
	// Set environment variables (t.Setenv automatically restores original values)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_BINARY", "/path/to/echo-agent")

	factory, err := NewACPClientFactory()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if factory.acpBinaryPath != "/path/to/echo-agent" {
		t.Errorf("Expected acpBinaryPath='/path/to/echo-agent', got '%s'", factory.acpBinaryPath)
	}
}

// TestACPClientFactory_DefaultBinary tests that binary path defaults to empty when env var not set
func TestACPClientFactory_DefaultBinary(t *testing.T) {
	// Set API key but unset custom binary (t.Setenv automatically restores original values)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_BINARY", "")

	factory, err := NewACPClientFactory()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if factory.acpBinaryPath != "" {
		t.Errorf("Expected acpBinaryPath='', got '%s'", factory.acpBinaryPath)
	}
}
