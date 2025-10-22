//go:build ignore
// +build ignore

// Manual test for ACP client with echo-agent
// Run with: go run test-acp-client.go

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/2389-research/ourocodus/pkg/acp"
)

// TestClient is a wrapper that uses echo-agent instead of claude-code-acp
type TestClient struct {
	*acp.Client
}

// NewTestClient creates a client that uses the echo-agent binary
func NewTestClient(workspace string) (*TestClient, error) {
	// Get absolute path to echo-agent
	binPath, err := filepath.Abs("bin/echo-agent")
	if err != nil {
		return nil, fmt.Errorf("failed to get echo-agent path: %w", err)
	}

	// Check if binary exists
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("echo-agent binary not found at %s, run 'make build' first", binPath)
	}

	// Temporarily override PATH to use echo-agent as claude-code-acp
	tmpDir, err := os.MkdirTemp("", "acp-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Create symlink to echo-agent named claude-code-acp
	symlinkPath := filepath.Join(tmpDir, "claude-code-acp")
	if err := os.Symlink(binPath, symlinkPath); err != nil {
		return nil, fmt.Errorf("failed to create symlink: %w", err)
	}

	// Prepend temp dir to PATH
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)

	// Create client (will now use our echo-agent)
	client, err := acp.NewClient(workspace, "test-api-key")
	if err != nil {
		os.Setenv("PATH", oldPath)
		os.RemoveAll(tmpDir)
		return nil, err
	}

	return &TestClient{Client: client}, nil
}

func main() {
	fmt.Println("=== ACP Client Integration Test ===")
	fmt.Println()

	// Create temporary workspace
	workspace, err := os.MkdirTemp("", "acp-workspace-*")
	if err != nil {
		log.Fatalf("Failed to create workspace: %v", err)
	}
	defer os.RemoveAll(workspace)

	fmt.Printf("Workspace: %s\n", workspace)
	fmt.Println()

	// Check if echo-agent exists
	if _, err := exec.LookPath("./bin/echo-agent"); err != nil {
		log.Fatal("echo-agent not found. Run 'make build' first")
	}

	// Create test client
	fmt.Println("Creating ACP client...")
	client, err := NewTestClient(workspace)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("✓ Client created successfully")
	fmt.Println()

	// Test 1: Simple message
	fmt.Println("Test 1: Sending simple message")
	msg, err := client.SendMessage("Hello, world!")
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	fmt.Printf("  Type: %s\n", msg.Type)
	fmt.Printf("  Content: %s\n", msg.Content)

	if msg.Type != "text" {
		log.Fatalf("Expected type 'text', got %q", msg.Type)
	}

	expected := "Echo: Hello, world!"
	if msg.Content != expected {
		log.Fatalf("Expected content %q, got %q", expected, msg.Content)
	}

	fmt.Println("✓ Test 1 passed")
	fmt.Println()

	// Test 2: Multiple messages
	fmt.Println("Test 2: Sending multiple messages")
	messages := []string{
		"First message",
		"Second message",
		"Third message",
	}

	for i, content := range messages {
		msg, err := client.SendMessage(content)
		if err != nil {
			log.Fatalf("Failed to send message %d: %v", i+1, err)
		}

		expected := fmt.Sprintf("Echo: %s", content)
		if msg.Content != expected {
			log.Fatalf("Message %d: expected %q, got %q", i+1, expected, msg.Content)
		}

		fmt.Printf("  Message %d: ✓\n", i+1)
	}

	fmt.Println("✓ Test 2 passed")
	fmt.Println()

	// Close client
	fmt.Println("Closing client...")
	if err := client.Close(); err != nil {
		log.Fatalf("Failed to close client: %v", err)
	}

	fmt.Println("✓ Client closed successfully")
	fmt.Println()

	fmt.Println("=== All Tests Passed ===")
}
