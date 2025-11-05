//go:build integration

package session

import (
	"context"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
)

func TestSpawnAgent_WithContainerLauncher(t *testing.T) {
	manager, _, _, _, _, _ := setupManagerWithMockFactory()
	ctx := context.Background()

	// Create user session
	ws := &mockWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("CreateUserSession failed: %v", err)
	}

	// Spawn agent - this should use the launcher factory
	err = manager.SpawnAgent(ctx, userSession.GetID(), "test-agent", "/tmp/test-workspaces/agent1")
	if err != nil {
		t.Fatalf("SpawnAgent failed: %v", err)
	}

	// Verify launcher was created and stored
	// This will fail until we implement the integration
}

func TestTerminateAgent_WithContainerLauncher(t *testing.T) {
	manager, _, _, _, _, _ := setupManagerWithMockFactory()
	ctx := context.Background()

	// Create session and spawn agent
	ws := &mockWebSocket{}
	userSession, err := manager.CreateUserSession(ctx, ws)
	if err != nil {
		t.Fatalf("CreateUserSession failed: %v", err)
	}

	err = manager.SpawnAgent(ctx, userSession.GetID(), "test-agent", "/tmp/test-workspaces/agent1")
	if err != nil {
		t.Fatalf("SpawnAgent failed: %v", err)
	}

	// Terminate agent - should stop container
	err = manager.TerminateAgent(ctx, userSession.GetID(), "test-agent")
	if err != nil {
		t.Fatalf("TerminateAgent failed: %v", err)
	}

	// Verify launcher and handle were cleaned up
	manager.launchersMu.RLock()
	launcher := manager.launchers["test-agent"]
	handle := manager.handles["test-agent"]
	manager.launchersMu.RUnlock()

	if launcher != nil {
		t.Error("Expected launcher to be removed from map")
	}
	if handle != nil {
		t.Error("Expected handle to be removed from map")
	}
}

func setupManagerWithMockFactory() (*Manager, *mockIDGenerator, *mockClock, *mockCleaner, *mockLogger, *mockClientFactory) {
	// Same as setupManager but exposed for this test
	store := NewMemoryStore()
	idGen := &mockIDGenerator{nextID: "test-session-id"}
	clock := &mockClock{now: time.Date(2025, 10, 23, 12, 0, 0, 0, time.UTC)}
	cleaner := &mockCleaner{}
	logger := &mockLogger{}
	clientFactory := &mockClientFactory{}
	mockFactory := agent.NewMockLauncherFactory() // NEW

	// Use /tmp for tests to avoid side effects
	manager := NewManager(store, idGen, clock, cleaner, logger, clientFactory, "/tmp/test-workspaces", nil, mockFactory) // nil publisher, mockFactory for launcher
	return manager, idGen, clock, cleaner, logger, clientFactory
}
