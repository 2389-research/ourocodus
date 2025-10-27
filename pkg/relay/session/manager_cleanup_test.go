package session

import (
	"context"
	"fmt"
	"testing"
)

// TestTerminateAgent_CloseError tests agent termination when Close() fails
func TestTerminateAgent_CloseError(t *testing.T) {
	manager, _, _, _, logger, _ := setupManager()
	ctx := context.Background()
	ws := &mockWebSocket{}

	// Create session with agent that fails to close
	session, _ := manager.CreateUserSession(ctx, ws)

	failingFactory := &mockClientFactory{
		clientFunc: func(workspace string) (ACPClient, error) {
			return &mockACPClient{
				closeFunc: func() error {
					return fmt.Errorf("close failed")
				},
			}, nil
		},
	}
	manager.clientFactory = failingFactory

	_ = manager.SpawnAgent(ctx, session.GetID(), "auth", "testdata/agent/auth")

	// Terminate should log error but continue
	err := manager.TerminateAgent(ctx, session.GetID(), "auth")
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

	session, _ := manager.CreateUserSession(ctx, &mockWebSocket{})
	sessionID := session.GetID()

	err := manager.TerminateUserSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("Expected no error (cleaner error logged), got: %v", err)
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
