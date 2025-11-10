package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
)

// TestTerminateAgent_CloseError tests agent termination when Close() fails
func TestTerminateAgent_CloseError(t *testing.T) {
	// Create manager with factory that returns agents that fail to close
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test-session-id"}
	clock := &mockClock{now: time.Date(2025, 10, 23, 12, 0, 0, 0, time.UTC)}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}
	failingFactory := &mockClientFactory{
		clientFunc: func(workspace string) (ACPClient, error) {
			return &mockACPClient{
				closeFunc: func() error {
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
