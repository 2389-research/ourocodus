package acp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/2389-research/ourocodus/pkg/acp"
)

func TestClient_EchoAgent(t *testing.T) {
	// Get path to echo-agent binary
	binPath, err := filepath.Abs("../../bin/echo-agent")
	if err != nil {
		t.Fatalf("Failed to get echo-agent path: %v", err)
	}

	// Check if binary exists
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("echo-agent binary not found, run 'make build' first")
	}

	// Override the claude-code-acp command to use our echo-agent
	// We'll use a temporary test workspace
	_ = t.TempDir() // Reserved for future use

	// Create a client
	// For testing, we'll modify the NewClient to accept a custom command
	// Since we can't modify it in tests, let's test the real implementation
	// by ensuring the echo-agent is in PATH or using full path

	t.Run("SendMessage", func(t *testing.T) {
		// For now, skip this test until we can properly mock the command
		// or add a test helper that uses echo-agent
		t.Skip("Need to implement command override for testing")

		// Future implementation:
		// client, err := acp.NewClient(tmpDir, "test-api-key")
		// if err != nil {
		// 	t.Fatalf("Failed to create client: %v", err)
		// }
		// defer client.Close()

		// msg, err := client.SendMessage("Hello, world!")
		// if err != nil {
		// 	t.Fatalf("Failed to send message: %v", err)
		// }

		// if msg.Type != "text" {
		// 	t.Errorf("Expected message type 'text', got %q", msg.Type)
		// }

		// expected := "Echo: Hello, world!"
		// if msg.Content != expected {
		// 	t.Errorf("Expected content %q, got %q", expected, msg.Content)
		// }
	})
}

func TestClient_InvalidWorkspace(t *testing.T) {
	_, err := acp.NewClient("", "test-api-key")
	if err == nil {
		t.Error("Expected error for empty workspace, got nil")
	}
}

func TestClient_InvalidAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := acp.NewClient(tmpDir, "")
	if err == nil {
		t.Error("Expected error for empty API key, got nil")
	}
}
