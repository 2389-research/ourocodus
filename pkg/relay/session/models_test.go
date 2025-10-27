package session

import (
	"fmt"
	"testing"
	"time"
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

// TestAgentSession_AddMessage tests adding messages to history
func TestAgentSession_AddMessage(t *testing.T) {
	now := testTime()
	agent := NewAgentSession("auth", "workspace", now)

	// Initially empty
	history := agent.GetHistory()
	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d messages", len(history))
	}

	// Add user message
	agent.AddMessage("user", "Hello", now)
	history = agent.GetHistory()
	if len(history) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(history))
	}
	if history[0].From != "user" {
		t.Errorf("Expected From='user', got %s", history[0].From)
	}
	if history[0].Content != "Hello" {
		t.Errorf("Expected Content='Hello', got %s", history[0].Content)
	}
	if !history[0].Timestamp.Equal(now) {
		t.Error("Expected timestamp to match")
	}

	// Add agent message
	later := now.Add(time.Second)
	agent.AddMessage("agent", "Hi there", later)
	history = agent.GetHistory()
	if len(history) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(history))
	}
	if history[1].From != "agent" {
		t.Errorf("Expected From='agent', got %s", history[1].From)
	}
	if history[1].Content != "Hi there" {
		t.Errorf("Expected Content='Hi there', got %s", history[1].Content)
	}
}

// TestAgentSession_GetHistoryReturnsCopy tests that GetHistory returns a copy
func TestAgentSession_GetHistoryReturnsCopy(t *testing.T) {
	now := testTime()
	agent := NewAgentSession("auth", "workspace", now)

	agent.AddMessage("user", "Test", now)
	history1 := agent.GetHistory()
	history2 := agent.GetHistory()

	// Modify first copy
	history1[0].Content = "Modified"

	// Second copy should be unchanged
	if history2[0].Content != "Test" {
		t.Error("GetHistory should return a copy, not a reference")
	}

	// Original should be unchanged
	history3 := agent.GetHistory()
	if history3[0].Content != "Test" {
		t.Error("Original history should not be modified")
	}
}

// TestAgentSession_ConcurrentMessageAccess tests thread-safety
func TestAgentSession_ConcurrentMessageAccess(t *testing.T) {
	now := testTime()
	agent := NewAgentSession("auth", "workspace", now)

	// Concurrent writes
	const numGoroutines = 10
	const messagesPerGoroutine = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < messagesPerGoroutine; j++ {
				agent.AddMessage("user", fmt.Sprintf("msg-%d-%d", id, j), now)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check total count
	history := agent.GetHistory()
	expected := numGoroutines * messagesPerGoroutine
	if len(history) != expected {
		t.Errorf("Expected %d messages, got %d", expected, len(history))
	}
}

// TestAgentSession_ConcurrentReadWrite tests concurrent reads and writes
func TestAgentSession_ConcurrentReadWrite(t *testing.T) {
	now := testTime()
	agent := NewAgentSession("auth", "workspace", now)

	// Add some initial messages
	for i := 0; i < 5; i++ {
		agent.AddMessage("user", fmt.Sprintf("initial-%d", i), now)
	}

	done := make(chan bool, 10)

	// Writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 5; j++ {
				agent.AddMessage("user", fmt.Sprintf("write-%d-%d", id, j), now)
			}
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = agent.GetHistory()
			}
			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have correct total
	history := agent.GetHistory()
	if len(history) < 5 {
		t.Error("Expected at least 5 messages")
	}
}
