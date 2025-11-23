package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
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

	if agent.GetAgentID() != "auth" {
		t.Errorf("Expected role 'auth', got %s", agent.GetAgentID())
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

// TestUserSession_AttachAgent tests attaching agents to a user session
func TestUserSession_AttachAgent(t *testing.T) {
	// Ensure Docker is available (will fail test if not)
	isDockerAvailable(t)

	// Clean up any existing leases before test
	t.Cleanup(func() {
		_ = ReleaseLease("test-agent-1")
		_ = ReleaseLease("test-agent-2")
	})

	now := testTime()
	session := NewUserSession("test-session-id", &mockWebSocket{}, now)

	// Test: Attach first agent
	t.Run("AttachFirstAgent", func(t *testing.T) {
		// Create test container for agent-1
		createTestContainer(t, "test-agent-1", "/workspace/agent1")

		// Generate test token (Phase 4)
		token1, err := GenerateTestAttachToken("test-agent-1")
		if err != nil {
			t.Fatalf("Failed to generate test token: %v", err)
		}

		agent, err := session.AttachAgent("test-agent-1", "/workspace/agent1", token1)
		if err != nil {
			t.Fatalf("Failed to attach agent: %v", err)
		}
		if agent.GetAgentID() != "test-agent-1" {
			t.Errorf("Expected agentID 'test-agent-1', got %s", agent.GetAgentID())
		}
		if agent.GetWorkspace() != "/workspace/agent1" {
			t.Errorf("Expected workspace '/workspace/agent1', got %s", agent.GetWorkspace())
		}
		if agent.GetState() != AgentActive {
			t.Errorf("Expected state ACTIVE, got %s", agent.GetState())
		}

		// Verify agent is in session's map
		retrieved := session.GetAgent("test-agent-1")
		if retrieved == nil {
			t.Error("Expected agent to be in session's map")
		}
		if retrieved.GetAgentID() != "test-agent-1" {
			t.Errorf("Expected retrieved agent ID 'test-agent-1', got %s", retrieved.GetAgentID())
		}
	})

	// Test: Idempotent attach (same session)
	t.Run("IdempotentAttach", func(t *testing.T) {
		// Reuse same token (token is still valid)
		token1, _ := GenerateTestAttachToken("test-agent-1")

		agent, err := session.AttachAgent("test-agent-1", "/workspace/agent1", token1)
		if err != nil {
			t.Fatalf("Failed idempotent attach: %v", err)
		}
		if agent.GetAgentID() != "test-agent-1" {
			t.Errorf("Expected same agent on idempotent attach")
		}

		// Should still be only one agent in session
		agents := session.ListAgents()
		if len(agents) != 1 {
			t.Errorf("Expected 1 agent after idempotent attach, got %d", len(agents))
		}
	})

	// Test: Attach second agent to same session
	t.Run("AttachSecondAgent", func(t *testing.T) {
		// Create test container for agent-2
		createTestContainer(t, "test-agent-2", "/workspace/agent2")

		// Generate token for second agent (Phase 4)
		token2, err := GenerateTestAttachToken("test-agent-2")
		if err != nil {
			t.Fatalf("Failed to generate test token: %v", err)
		}

		agent, err := session.AttachAgent("test-agent-2", "/workspace/agent2", token2)
		if err != nil {
			t.Fatalf("Failed to attach second agent: %v", err)
		}
		if agent.GetAgentID() != "test-agent-2" {
			t.Errorf("Expected agentID 'test-agent-2', got %s", agent.GetAgentID())
		}

		// Should have two agents now
		agents := session.ListAgents()
		if len(agents) != 2 {
			t.Errorf("Expected 2 agents, got %d", len(agents))
		}
	})

	// Test: Conflict - agent already attached to different session
	t.Run("ConflictAttach", func(t *testing.T) {
		session2 := NewUserSession("test-session-id-2", &mockWebSocket{}, now)
		// Token is valid but agent is already leased to session1
		token1, _ := GenerateTestAttachToken("test-agent-1")

		_, err := session2.AttachAgent("test-agent-1", "/workspace/agent1", token1)
		if err != ErrAlreadyAttached {
			t.Errorf("Expected ErrAlreadyAttached, got %v", err)
		}

		// Verify agent is NOT in session2's map
		retrieved := session2.GetAgent("test-agent-1")
		if retrieved != nil {
			t.Error("Expected agent to NOT be in session2's map")
		}
	})

	// Cleanup
	_ = session.DetachAgent("test-agent-1")
	_ = session.DetachAgent("test-agent-2")
}

// TestUserSession_DetachAgent tests detaching agents from a user session
func TestUserSession_DetachAgent(t *testing.T) {
	// Ensure Docker is available (will fail test if not)
	isDockerAvailable(t)

	// Clean up any existing leases before test
	t.Cleanup(func() {
		_ = ReleaseLease("detach-test-agent")
	})

	now := testTime()
	session := NewUserSession("detach-test-session", &mockWebSocket{}, now)

	// Create test container for detach test
	createTestContainer(t, "detach-test-agent", "/workspace/test")

	// Generate token for detach test (Phase 4)
	token, err := GenerateTestAttachToken("detach-test-agent")
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Attach agent first
	agent, err := session.AttachAgent("detach-test-agent", "/workspace/test", token)
	if err != nil {
		t.Fatalf("Failed to attach agent: %v", err)
	}
	if agent == nil {
		t.Fatal("Expected non-nil agent")
	}

	// Test: Detach agent
	t.Run("DetachAgent", func(t *testing.T) {
		err := session.DetachAgent("detach-test-agent")
		if err != nil {
			t.Fatalf("Failed to detach agent: %v", err)
		}

		// Verify agent is removed from session's map
		retrieved := session.GetAgent("detach-test-agent")
		if retrieved != nil {
			t.Error("Expected agent to be removed from session's map")
		}

		// Verify lease is released
		lease, err := ReadLease("detach-test-agent")
		if err != ErrLeaseNotFound {
			t.Errorf("Expected ErrLeaseNotFound, got %v (lease: %+v)", err, lease)
		}
	})

	// Test: Idempotent detach
	t.Run("IdempotentDetach", func(t *testing.T) {
		err := session.DetachAgent("detach-test-agent")
		if err != nil {
			t.Fatalf("Failed idempotent detach: %v", err)
		}

		// Should still have no agents
		agents := session.ListAgents()
		if len(agents) != 0 {
			t.Errorf("Expected 0 agents after idempotent detach, got %d", len(agents))
		}
	})
}

// TestUserSession_AttachDetach_ConcurrentAccess tests thread-safety
func TestUserSession_AttachDetach_ConcurrentAccess(t *testing.T) {
	// Ensure Docker is available (will fail test if not)
	isDockerAvailable(t)

	// Clean up any existing leases before test
	t.Cleanup(func() {
		for i := 0; i < 10; i++ {
			_ = ReleaseLease(fmt.Sprintf("concurrent-agent-%d", i))
		}
	})

	now := testTime()
	session := NewUserSession("concurrent-test-session", &mockWebSocket{}, now)

	// Create test containers before concurrent access
	for i := 0; i < 10; i++ {
		agentID := fmt.Sprintf("concurrent-agent-%d", i)
		workspace := fmt.Sprintf("/workspace/agent-%d", i)
		createTestContainer(t, agentID, workspace)
	}

	done := make(chan bool, 20)

	// Concurrent attaches
	for i := 0; i < 10; i++ {
		go func(id int) {
			agentID := fmt.Sprintf("concurrent-agent-%d", id)
			workspace := fmt.Sprintf("/workspace/agent-%d", id)

			// Generate token for this agent (Phase 4)
			token, err := GenerateTestAttachToken(agentID)
			if err != nil {
				t.Logf("Failed to generate token for agent %s: %v", agentID, err)
				done <- true
				return
			}

			_, err = session.AttachAgent(agentID, workspace, token)
			if err != nil {
				t.Logf("Failed to attach agent %s: %v", agentID, err)
			}
			done <- true
		}(i)
	}

	// Concurrent detaches (may fail if agent not attached yet, that's ok)
	for i := 0; i < 10; i++ {
		go func(id int) {
			agentID := fmt.Sprintf("concurrent-agent-%d", id)
			_ = session.DetachAgent(agentID)
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < 20; i++ {
		<-done
	}

	// Should not panic and should have consistent state
	agents := session.ListAgents()
	t.Logf("Final agent count: %d", len(agents))

	// Cleanup any remaining agents
	for id := range agents {
		_ = session.DetachAgent(id)
	}
}

// isDockerAvailable checks if Docker is available and fails the test if not
func isDockerAvailable(t *testing.T) bool {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Docker client creation failed: %v. Docker must be available to run these tests.", err)
		return false
	}
	defer func() { _ = cli.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		t.Fatalf("Docker ping failed: %v. Docker daemon must be running to run these tests.", err)
		return false
	}

	return true
}

// createTestContainer creates a test agent container for unit tests
func createTestContainer(t *testing.T, agentID, workspace string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}
	defer func() { _ = cli.Close() }()

	// Create minimal test container with Phase 3 labels
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sleep", "3600"},
		Labels: map[string]string{
			"ourocodus.agent":              "true",
			"ourocodus.agent/agent-id":     agentID,
			"ourocodus.agent/workspace":    workspace,
			"ourocodus.agent/spawn-source": "test",
		},
	}, &container.HostConfig{}, nil, nil, "")
	if err != nil {
		t.Fatalf("Failed to create test container: %v", err)
	}

	// Start container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		t.Fatalf("Failed to start test container: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		// Force remove atomically stops and removes (prevents double-stop hangs)
		if err := cli.ContainerRemove(cleanupCtx, resp.ID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			//nolint:staticcheck // errdefs is Docker SDK's official error handling
			if !errdefs.IsNotFound(err) {
				t.Logf("cleanup warning: remove %s: %v", resp.ID[:12], err)
			}
		}
	})

	return resp.ID
}
