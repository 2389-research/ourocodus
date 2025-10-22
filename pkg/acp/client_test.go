package acp_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/2389-research/ourocodus/pkg/acp"
)

// getEchoAgentPath returns the path to the echo-agent binary for testing
func getEchoAgentPath(t *testing.T) string {
	t.Helper()

	// Skip on Windows - echo-agent and bash scripts require Unix-like environment
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: echo-agent and bash scripts require a Unix-like shell")
	}

	binPath, err := filepath.Abs("../../bin/echo-agent")
	if err != nil {
		t.Fatalf("Failed to get echo-agent path: %v", err)
	}

	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("echo-agent binary not found, run 'make build' first")
	}

	return binPath
}

func TestNewClient_Success(t *testing.T) {
	t.Parallel()
	echoAgent := getEchoAgentPath(t)
	tmpDir := t.TempDir()

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(echoAgent))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Verify client was created successfully
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
}

func TestNewClient_InvalidWorkspace(t *testing.T) {
	t.Parallel()
	_, err := acp.NewClient("", "test-api-key")
	if err == nil {
		t.Error("Expected error for empty workspace, got nil")
	}

	expectedMsg := "workspace path is required"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewClient_InvalidAPIKey(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	_, err := acp.NewClient(tmpDir, "")
	if err == nil {
		t.Error("Expected error for empty API key, got nil")
	}

	expectedMsg := "API key is required"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing %q, got %q", expectedMsg, err.Error())
	}
}

func TestSendMessage_ValidRequest(t *testing.T) {
	t.Parallel()
	echoAgent := getEchoAgentPath(t)
	tmpDir := t.TempDir()

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(echoAgent))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Send a message
	msg, err := client.SendMessage("Hello, world!")
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Verify response
	if msg.Type != "text" {
		t.Errorf("Expected message type 'text', got %q", msg.Type)
	}

	expected := "Echo: Hello, world!"
	if msg.Content != expected {
		t.Errorf("Expected content %q, got %q", expected, msg.Content)
	}
}

func TestSendMessage_MultipleSequential(t *testing.T) {
	t.Parallel()
	echoAgent := getEchoAgentPath(t)
	tmpDir := t.TempDir()

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(echoAgent))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Send multiple messages and verify they all work
	messages := []string{"First message", "Second message", "Third message"}

	for i, content := range messages {
		msg, err := client.SendMessage(content)
		if err != nil {
			t.Fatalf("Failed to send message %d: %v", i+1, err)
		}

		expected := "Echo: " + content
		if msg.Content != expected {
			t.Errorf("Message %d: expected %q, got %q", i+1, expected, msg.Content)
		}

		if msg.Type != "text" {
			t.Errorf("Message %d: expected type 'text', got %q", i+1, msg.Type)
		}
	}
}

func TestClose_TerminatesCleanly(t *testing.T) {
	t.Parallel()
	echoAgent := getEchoAgentPath(t)
	tmpDir := t.TempDir()

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(echoAgent))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close the client
	err = client.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Verify we can call Close() multiple times without error
	err = client.Close()
	if err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}
}

func TestSendMessage_AfterClose(t *testing.T) {
	t.Parallel()
	echoAgent := getEchoAgentPath(t)
	tmpDir := t.TempDir()

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(echoAgent))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close the client
	err = client.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Try to send a message after closing
	_, err = client.SendMessage("Should fail")
	if err == nil {
		t.Error("Expected error when sending message after Close(), got nil")
	}

	expectedMsg := "client is closed"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewClient_InvalidCommand(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Try to create client with non-existent command
	_, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand("/nonexistent/command"))
	if err == nil {
		t.Fatal("Expected error for non-existent command, got nil")
	}

	// Error should mention the command failed to start
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestSendMessage_ProcessCrash(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a mock process that exits immediately
	mockScript := filepath.Join(tmpDir, "crash-agent.sh")
	scriptContent := "#!/bin/bash\nexit 1\n"
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create crash script: %v", err)
	}
	// Sync to ensure file is fully written before execution
	if f, err := os.Open(mockScript); err == nil {
		_ = f.Sync()
		f.Close()
	}

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(mockScript))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Try to send a message - should fail because process crashed
	_, err = client.SendMessage("Hello")
	if err == nil {
		t.Error("Expected error when process crashes, got nil")
	}
}

func TestSendMessage_InvalidJSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a mock process that returns invalid JSON
	mockScript := filepath.Join(tmpDir, "invalid-json-agent.sh")
	scriptContent := "#!/bin/bash\nwhile read line; do\n  echo 'not valid json'\ndone\n"
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create invalid-json script: %v", err)
	}
	// Sync to ensure file is fully written before execution
	if f, err := os.Open(mockScript); err == nil {
		_ = f.Sync()
		f.Close()
	}

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(mockScript))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Try to send a message - should fail due to invalid JSON response
	_, err = client.SendMessage("Hello")
	if err == nil {
		t.Error("Expected error when response is invalid JSON, got nil")
	}

	// Error should mention JSON parsing failure
	if err.Error() == "" {
		t.Error("Expected non-empty error message for invalid JSON")
	}
}
