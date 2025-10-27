package session

import (
	"context"
	"testing"
)

// TestCreateUserSession_NilWebSocket tests error handling for nil websocket
func TestCreateUserSession_NilWebSocket(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	_, err := manager.CreateUserSession(ctx, nil)
	if err == nil {
		t.Fatal("Expected error for nil websocket, got nil")
	}
	if err.Error() != "websocket connection cannot be nil" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

// TestSpawnAgent_EmptyRole tests error handling for empty role
func TestSpawnAgent_EmptyRole(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	session, _ := manager.CreateUserSession(ctx, &mockWebSocket{})
	err := manager.SpawnAgent(ctx, session.GetID(), "", "workspace")
	if err == nil {
		t.Fatal("Expected error for empty role, got nil")
	}
}

// TestSpawnAgent_EmptyWorkspace tests error handling for empty workspace
func TestSpawnAgent_EmptyWorkspace(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	session, _ := manager.CreateUserSession(ctx, &mockWebSocket{})
	err := manager.SpawnAgent(ctx, session.GetID(), "auth", "")
	if err == nil {
		t.Fatal("Expected error for empty workspace, got nil")
	}
}

// TestSpawnAgent_TerminatedSession tests spawning agent on terminated session
func TestSpawnAgent_TerminatedSession(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	session, _ := manager.CreateUserSession(ctx, &mockWebSocket{})
	sessionID := session.GetID()
	manager.TerminateUserSession(ctx, sessionID)

	// Try to spawn agent on terminated session (session removed from store)
	err := manager.SpawnAgent(ctx, sessionID, "auth", "testdata/agent/auth")
	if err == nil {
		t.Fatal("Expected error spawning agent on terminated session, got nil")
	}
}

// TestGetAgent_SessionNotFound tests error handling when session not found
func TestGetAgent_SessionNotFound(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()

	_, err := manager.GetAgent("nonexistent", "auth")
	if err == nil {
		t.Fatal("Expected error for nonexistent session, got nil")
	}
}

// TestGetAgent_AgentNotFound tests error handling when agent not found
func TestGetAgent_AgentNotFound(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	session, _ := manager.CreateUserSession(ctx, &mockWebSocket{})
	_, err := manager.GetAgent(session.GetID(), "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent agent, got nil")
	}
}

// TestListAgents_SessionNotFound tests error handling when session not found
func TestListAgents_SessionNotFound(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()

	_, err := manager.ListAgents("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent session, got nil")
	}
}

// TestRecordHeartbeat_SessionNotFound tests error handling
func TestRecordHeartbeat_SessionNotFound(t *testing.T) {
	manager, _, _, _, _, _ := setupManager()
	ctx := context.Background()

	err := manager.RecordHeartbeat(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent session, got nil")
	}
}
