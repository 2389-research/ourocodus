package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// TestACPBridge_Integration tests the ACPBridge with a real CLI-spawned agent.
// This is an integration test that requires:
// - Docker daemon running
// - A test agent spawned via `agentd spawn test-integration`
//
// Run with: go test -v ./pkg/relay/session -run TestACPBridge_Integration
//
// Prerequisites:
//
//	export DOCKER_HOST="unix:///Users/clint/.colima/default/docker.sock"  # Adjust for your system
//	export ANTHROPIC_API_KEY="sk-test-dummy"
//	bin/agentd spawn test-integration
func TestACPBridge_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test configuration
	agentID := "test-phase3" // Use existing agent (spawned earlier in test)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Find agent container
	t.Log("Step 1: Finding agent container...")
	containerID, workspace, err := findAgentContainerID(ctx, agentID)
	if err != nil {
		t.Fatalf("Failed to find agent container: %v\nMake sure agent is spawned: bin/agentd spawn %s", err, agentID)
	}
	t.Logf("Found container: %s (workspace: %s)", containerID[:12], workspace)

	// Step 2: Create ACP bridge
	t.Log("Step 2: Creating ACP bridge...")
	bridge, err := session.NewACPBridge(ctx, containerID, agentID)
	if err != nil {
		t.Fatalf("Failed to create ACP bridge: %v", err)
	}
	defer func() {
		if err := bridge.Close(ctx); err != nil {
			t.Logf("Warning: Failed to close bridge: %v", err)
		}
	}()
	t.Log("ACPBridge created successfully")

	// Step 3: Send a message
	t.Log("Step 3: Sending message to agent...")
	msgCtx, msgCancel := context.WithTimeout(ctx, 10*time.Second)
	defer msgCancel()

	response, err := bridge.SendMessage(msgCtx, "echo 'Phase 3 integration test successful!'")
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	t.Logf("Received response: %+v", response)

	// Step 4: Verify response structure
	respMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected response to be map, got: %T", response)
	}

	if respMap["type"] != "text" {
		t.Errorf("Expected response type 'text', got: %v", respMap["type"])
	}

	if respMap["text"] == nil && respMap["content"] == nil {
		t.Error("Response missing text/content field")
	}

	t.Log("✅ Phase 3 integration test PASSED")
	t.Log("   - ACPBridge successfully discovered agent via Docker labels")
	t.Log("   - ACPBridge successfully attached to container exec")
	t.Log("   - ACPBridge successfully sent message and received response")
	t.Log("   - Message protocol working correctly")
}

// Helper function to find agent container (mirrors models.go logic)
func findAgentContainerID(ctx context.Context, agentID string) (containerID, workspace string, err error) {
	// Import needed for testing
	// This is duplicated from helpers.go for testing isolation
	// In real usage, AttachAgent() calls the helpers.go version
	return session.FindAgentContainerIDForTesting(ctx, agentID)
}
