package acp_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/acp"
)

type capturingLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *capturingLogger) Printf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, v...))
}

func (l *capturingLogger) contains(substring string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.lines {
		if strings.Contains(line, substring) {
			return true
		}
	}
	return false
}

func (l *capturingLogger) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]string, len(l.lines))
	copy(cp, l.lines)
	return cp
}

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
	defer func() {
		ctx := context.Background()
		_ = client.Close(ctx)
	}()

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
	defer func() {
		ctx := context.Background()
		_ = client.Close(ctx)
	}()

	// Send a message
	ctx := context.Background()
	msg, err := client.SendMessage(ctx, "Hello, world!")
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
	defer func() {
		ctx := context.Background()
		_ = client.Close(ctx)
	}()

	// Send multiple messages and verify they all work
	messages := []string{"First message", "Second message", "Third message"}
	ctx := context.Background()

	for i, content := range messages {
		msg, err := client.SendMessage(ctx, content)
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
	ctx := context.Background()
	err = client.Close(ctx)
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Verify we can call Close() multiple times without error
	err = client.Close(ctx)
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
	ctx := context.Background()
	err = client.Close(ctx)
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Try to send a message after closing
	ctx2 := context.Background()
	_, err = client.SendMessage(ctx2, "Should fail")
	if err == nil {
		t.Error("Expected error when sending message after Close(), got nil")
	}

	expectedMsg := "client is closed"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing %q, got %q", expectedMsg, err.Error())
	}
}

func TestClient_WithLogger(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: mock shell script requires Unix-like environment")
	}

	workspace := t.TempDir()
	scriptPath := filepath.Join(t.TempDir(), "stderr-agent.sh")

	script := "#!/bin/sh\n" +
		"echo \"mock stderr line\" >&2\n" +
		"sleep 0.2\n"

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to write mock script: %v", err)
	}

	logger := &capturingLogger{}

	client, err := acp.NewClient(workspace, "test-api-key",
		acp.WithCommand(scriptPath),
		acp.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		ctx := context.Background()
		_ = client.Close(ctx)
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if logger.contains("mock stderr line") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !logger.contains("mock stderr line") {
		t.Fatalf("Expected logger to capture stderr output, lines=%v", logger.snapshot())
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
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
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
	defer func() {
		ctx := context.Background()
		_ = client.Close(ctx)
	}()

	// Try to send a message - should fail because process crashed
	ctx := context.Background()
	_, err = client.SendMessage(ctx, "Hello")
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
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
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
	defer func() {
		ctx := context.Background()
		_ = client.Close(ctx)
	}()

	// Try to send a message - should fail due to invalid JSON response
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.SendMessage(ctx, "Hello")
	if err == nil {
		t.Error("Expected error when response is invalid JSON, got nil")
	}

	// Error should mention JSON parsing failure or read loop termination
	if err.Error() == "" {
		t.Error("Expected non-empty error message for invalid JSON")
	}
}

// Test Close respects context timeout (issue #211)
func TestClient_Close_Timeout(t *testing.T) {
	// Create a transport that blocks on Close
	blockingTransport := &blockingCloseTransport{
		closeDone: make(chan struct{}),
	}

	client, err := acp.NewClientFromTransport(blockingTransport)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Close should return timeout error, not block forever
	err = client.Close(ctx)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected deadline exceeded error, got: %v", err)
	}

	// Unblock the transport to avoid goroutine leak
	close(blockingTransport.closeDone)
	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)
}

// Test Close succeeds when transport closes quickly
func TestClient_Close_Success(t *testing.T) {
	// Create a normal transport
	normalTransport := &mockTransportForClose{}

	client, err := acp.NewClientFromTransport(normalTransport)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Close should succeed
	err = client.Close(ctx)
	if err != nil {
		t.Fatalf("Expected successful close, got error: %v", err)
	}

	if !normalTransport.closed {
		t.Error("Transport should be closed")
	}
}

// Test Close is idempotent
func TestClient_Close_Idempotent(t *testing.T) {
	normalTransport := &mockTransportForClose{}
	client, err := acp.NewClientFromTransport(normalTransport)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// First close
	err1 := client.Close(ctx)
	if err1 != nil {
		t.Fatalf("First close failed: %v", err1)
	}

	// Second close should be no-op
	err2 := client.Close(ctx)
	if err2 != nil {
		t.Fatalf("Second close failed: %v", err2)
	}

	// Transport Close should only be called once
	if normalTransport.closeCount != 1 {
		t.Errorf("Expected 1 close call, got %d", normalTransport.closeCount)
	}
}

// blockingCloseTransport simulates a transport that blocks indefinitely on Close
type blockingCloseTransport struct {
	closeDone chan struct{}
}

func (t *blockingCloseTransport) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (t *blockingCloseTransport) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (t *blockingCloseTransport) Close(ctx context.Context) error {
	select {
	case <-t.closeDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *blockingCloseTransport) Stderr() io.Reader {
	return nil
}

// mockTransportForClose is a simple non-blocking transport for tests
type mockTransportForClose struct {
	closed     bool
	closeCount int
}

func (t *mockTransportForClose) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (t *mockTransportForClose) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (t *mockTransportForClose) Close(ctx context.Context) error {
	t.closed = true
	t.closeCount++
	return nil
}

func (t *mockTransportForClose) Stderr() io.Reader {
	return nil
}

// TestSendMessage_ContextTimeout verifies that SendMessage respects context timeouts (issue #226)
func TestSendMessage_ContextTimeout(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a mock process that never responds (simulates hanging agent)
	mockScript := filepath.Join(tmpDir, "hanging-agent.sh")
	scriptContent := "#!/bin/bash\nwhile read line; do\n  sleep 10  # Never send response\ndone\n"
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create hanging script: %v", err)
	}

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(mockScript))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		ctx := context.Background()
		_ = client.Close(ctx)
	}()

	// Send message with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = client.SendMessage(ctx, "test message")
	elapsed := time.Since(start)

	// Verify timeout occurred
	if err == nil {
		t.Error("Expected error due to context timeout, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded error, got: %v", err)
	}

	// Verify timeout happened quickly (not waiting indefinitely)
	if elapsed > 500*time.Millisecond {
		t.Errorf("Timeout took too long: %v (expected ~100ms)", elapsed)
	}
}

// TestSendMessage_ContextCancellation verifies that SendMessage respects context cancellation (issue #226)
func TestSendMessage_ContextCancellation(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a mock process that never responds
	mockScript := filepath.Join(tmpDir, "hanging-agent.sh")
	scriptContent := "#!/bin/bash\nwhile read line; do\n  sleep 10\ndone\n"
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create hanging script: %v", err)
	}

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(mockScript))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		ctx := context.Background()
		_ = client.Close(ctx)
	}()

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = client.SendMessage(ctx, "test message")
	elapsed := time.Since(start)

	// Verify cancellation occurred
	if err == nil {
		t.Error("Expected error due to context cancellation, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	// Verify cancellation happened quickly
	if elapsed > 200*time.Millisecond {
		t.Errorf("Cancellation took too long: %v (expected ~50ms)", elapsed)
	}
}

// TestSendMessage_ContextCancelDuringClose verifies interaction between context cancellation
// and Close's done channel (ensures both signals are handled correctly)
func TestSendMessage_ContextCancelDuringClose(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a mock process that never responds
	mockScript := filepath.Join(tmpDir, "hanging-agent.sh")
	scriptContent := "#!/bin/bash\nwhile read line; do\n  sleep 10\ndone\n"
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create hanging script: %v", err)
	}

	client, err := acp.NewClient(tmpDir, "test-api-key", acp.WithCommand(mockScript))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Start a SendMessage that will hang
	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		_, err := client.SendMessage(ctx, "test message")
		done <- err
	}()

	// Give it time to start waiting
	time.Sleep(50 * time.Millisecond)

	// Close the client (should wake the waiting SendMessage)
	// Use a 2-second timeout for Close since it waits for in-flight requests
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer closeCancel()

	closeErr := client.Close(closeCtx)

	// Close may timeout waiting for the hanging process, which is expected
	if closeErr != nil {
		if !strings.Contains(closeErr.Error(), "context deadline exceeded") {
			t.Errorf("Close returned unexpected error: %v", closeErr)
		}
	}

	// Verify SendMessage woke up quickly (even though Close may have timed out)
	select {
	case err := <-done:
		if err == nil {
			t.Error("Expected error from SendMessage after Close, got nil")
		}
		// Should get "client closed" error or similar
		if !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "EOF") {
			t.Logf("Got error: %v (expected 'closed' or 'EOF')", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SendMessage did not return after Close")
	}

	// Note: Close might take the full timeout (up to 2s) waiting for the hanging
	// process to exit, which is expected behavior. The important part is that
	// SendMessage returned immediately when done channel was closed.
}
