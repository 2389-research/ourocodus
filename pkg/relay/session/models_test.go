package session

import (
	"testing"
)

// TestUserSession_GetWebSocket tests the WebSocket accessor
func TestUserSession_GetWebSocket(t *testing.T) {
	ws := &mockWebSocket{}
	session := NewUserSession("test-id", ws, testTime())

	if session.GetWebSocket() != ws {
		t.Error("Expected WebSocket to match")
	}
}

// TestAgentSession_Accessors tests all agent session accessors
func TestAgentSession_Accessors(t *testing.T) {
	agent := NewAgentSession("auth", "workspace/auth", testTime())

	if agent.GetRole() != "auth" {
		t.Errorf("Expected role 'auth', got %s", agent.GetRole())
	}
	if agent.GetWorkspace() != "workspace/auth" {
		t.Errorf("Expected workspace 'workspace/auth', got %s", agent.GetWorkspace())
	}
	if agent.GetState() != AgentSpawning {
		t.Errorf("Expected state SPAWNING, got %s", agent.GetState())
	}
	if agent.GetACPClient() != nil {
		t.Error("Expected nil ACP client initially")
	}
	if agent.GetError() != "" {
		t.Error("Expected empty error initially")
	}

	// Test after setting error
	agent.mu.Lock()
	agent.setError("test error")
	agent.mu.Unlock()

	if agent.GetError() != "test error" {
		t.Errorf("Expected error 'test error', got %s", agent.GetError())
	}
}

// TestUserSession_GetCreatedAt tests the CreatedAt accessor
func TestUserSession_GetCreatedAt(t *testing.T) {
	now := testTime()
	session := NewUserSession("test-id", &mockWebSocket{}, now)

	if !session.GetCreatedAt().Equal(now) {
		t.Error("Expected CreatedAt to match")
	}
}

// TestAgentSession_GetCreatedAtAndLastActive tests time accessors
func TestAgentSession_GetCreatedAtAndLastActive(t *testing.T) {
	now := testTime()
	agent := NewAgentSession("auth", "workspace", now)

	if !agent.GetCreatedAt().Equal(now) {
		t.Error("Expected CreatedAt to match")
	}
	if !agent.GetLastActive().Equal(now) {
		t.Error("Expected LastActive to equal CreatedAt initially")
	}
}
