package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
)

// TestTerminateAgent_CloseError tests agent termination when Close() fails
func TestTerminateAgent_CloseError(t *testing.T) {
	// Create isolated lease directory
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	// Create manager with factory that returns agents that fail to close
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test-session-id"}
	clock := &mockClock{now: time.Date(2025, 10, 23, 12, 0, 0, 0, time.UTC)}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}
	failingFactory := &mockClientFactory{
		clientFunc: func(workspace string) (ACPClient, error) {
			return &mockACPClient{
				closeFunc: func(ctx context.Context) error {
					return fmt.Errorf("close failed")
				},
			}, nil
		},
	}
	mockFactory := agent.NewMockLauncherFactory()                                                      // NEW
	manager := NewManager(store, idGen, clock, cleaner, logger, failingFactory, ".", nil, mockFactory) // nil publisher for tests

	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session with agent that fails to close
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if err := manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth"); err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	// Terminate should log error but continue
	err = manager.TerminateAgent(ctx, session.GetID(), "auth")
	if err != nil {
		t.Fatalf("Expected no error (close error logged), got: %v", err)
	}

	// Check logger recorded the error
	if logger.MessageCount() < 3 {
		t.Error("Expected log message for close error")
	}
}

// TestTerminateUserSession_CleanerError tests that session terminates even if cleaner fails
func TestTerminateUserSession_CleanerError(t *testing.T) {
	manager, _, _, cleaner, logger, _ := setupManager()
	ctx := context.Background()

	// Make cleaner fail
	cleaner.shouldErr = true

	session, err := manager.CreateUserSession(ctx, &mockWebSocket{})
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	sessionID := session.GetID()

	summary, err := manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Expected no error (cleaner error logged), got: %v", err)
	}

	if summary.CleanupStatus != CleanupStatusPartial {
		t.Fatalf("Expected cleanup status partial, got %s", summary.CleanupStatus)
	}

	// Session should still be removed
	if manager.Get(sessionID) != nil {
		t.Error("Expected session to be removed despite cleaner error")
	}

	// Check logger recorded the error
	if logger.MessageCount() < 2 {
		t.Error("Expected log message for cleaner error")
	}
}

// TestTerminateAgent_CredentialCleanup tests that .creds directory is removed on termination
func TestTerminateAgent_CredentialCleanup(t *testing.T) {
	// Create isolated lease directory
	tmpDir := t.TempDir()
	oldLeaseDir := LeaseDir
	LeaseDir = tmpDir
	defer func() { LeaseDir = oldLeaseDir }()

	// Create a workspace with .creds directory
	workspaceDir := t.TempDir()
	credsDir := filepath.Join(workspaceDir, ".creds")
	if err := os.MkdirAll(credsDir, 0o700); err != nil {
		t.Fatalf("Failed to create .creds directory: %v", err)
	}
	envFile := filepath.Join(credsDir, ".env")
	if err := os.WriteFile(envFile, []byte("ANTHROPIC_API_KEY=test-key\n"), 0o600); err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}

	// Verify .creds exists before termination
	if _, err := os.Stat(credsDir); os.IsNotExist(err) {
		t.Fatal("Expected .creds directory to exist before termination")
	}

	// Create manager
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test-session-id"}
	clock := &mockClock{now: time.Date(2025, 10, 23, 12, 0, 0, 0, time.UTC)}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}
	clientFactory := &mockClientFactory{
		clientFunc: func(workspace string) (ACPClient, error) {
			return &mockACPClient{}, nil
		},
	}
	mockFactory := agent.NewMockLauncherFactory()
	manager := NewManager(store, idGen, clock, cleaner, logger, clientFactory, ".", nil, mockFactory)

	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session
	session, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Manually add an agent with the workspace (bypass spawn to use temp workspace)
	agent := &AgentSession{
		AgentID:   "test-agent",
		Workspace: workspaceDir,
		state:     AgentActive,
	}
	session.mu.Lock()
	session.agents["test-agent"] = agent
	session.mu.Unlock()

	// Acquire lease manually to match what AttachAgent does
	_, err = AcquireLease("test-agent", session.GetID())
	if err != nil {
		t.Fatalf("Failed to acquire lease: %v", err)
	}

	// Terminate the agent
	err = manager.TerminateAgent(ctx, session.GetID(), "test-agent")
	if err != nil {
		t.Fatalf("Expected no error terminating agent, got: %v", err)
	}

	// Verify .creds directory was removed
	if _, err := os.Stat(credsDir); !os.IsNotExist(err) {
		t.Error("Expected .creds directory to be removed after termination")
	}

	// Check logger recorded security cleanup
	var foundSecurityLog bool
	for _, msg := range logger.messages {
		if strings.Contains(msg, "[SECURITY]") && strings.Contains(msg, ".creds") {
			foundSecurityLog = true
			break
		}
	}
	if !foundSecurityLog {
		t.Error("Expected [SECURITY] log message for credential cleanup")
	}
}
