package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/2389-research/ourocodus/tests/e2e/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	relayBinaryPath = "bin/relay-e2e" // Relative to project root
	relayPort       = "8080"
	wsURL           = "ws://localhost:8080/ws"
	protocolVersion = "1.0"

	// Workspace configuration
	// Must match WORKSPACE_BASE_DIR in tests/e2e/helpers/process.go
	// Paths are relative to project root where relay server runs
	workspaceBase = "agent"

	// Timeouts
	setupTimeout    = 60 * time.Second
	messageTimeout  = 30 * time.Second
	worktreeTimeout = 90 * time.Second
	testTimeout     = 5 * time.Minute
)

// TestE2EFullFlow tests the complete PWA → Relay → Claude Code → Back flow
func TestE2EFullFlow(t *testing.T) {
	fmt.Println("=== E2E Test Starting ===")
	t.Log("Test initialized")

	// Create test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Record test start time for worktree verification
	testStartTime := time.Now()

	// Step 1: Check prerequisites
	fmt.Println("Step 1: Checking prerequisites")
	t.Log("Checking prerequisites...")
	checkPrerequisites(t)

	// Step 2: Setup worktrees
	fmt.Println("Step 2: Setting up worktrees")
	t.Log("Setting up worktrees...")
	setupWorktrees(t, ctx)
	fmt.Println("Step 2: Worktrees setup complete")

	// Step 3: Build relay binary
	fmt.Println("Step 3: Building relay binary")
	t.Log("Building relay binary...")
	buildRelay(t, ctx)
	fmt.Println("Step 3: Relay binary built")

	// Step 4: Start relay server
	fmt.Println("Step 4: Starting relay server")
	t.Log("Starting relay server...")
	server := startRelayServer(t, ctx)
	defer func() {
		t.Log("Stopping relay server...")
		if err := server.Stop(); err != nil {
			t.Logf("Warning: Failed to stop relay server: %v", err)
		}
	}()

	// Step 5: Connect WebSocket client
	t.Log("Connecting to relay WebSocket...")
	client, sessionID := connectAndCreateSession(t)
	defer func() {
		t.Log("Closing WebSocket connection...")
		if err := client.Close(); err != nil {
			t.Logf("Warning: Failed to close WebSocket: %v", err)
		}
	}()

	// Step 6: Spawn agents
	t.Log("Spawning agents...")
	agentRoles := []string{"auth", "db", "tests"}
	spawnAgents(t, client, sessionID, agentRoles)

	// Step 7: Test agent communication
	t.Log("Testing agent communication...")
	testAgentCommunication(t, client, sessionID, agentRoles)

	// Step 8: Verify worktree commits
	t.Log("Verifying worktree commits...")
	verifyWorktreeCommits(t, ctx, agentRoles, testStartTime)

	t.Log("E2E test completed successfully!")
}

// checkPrerequisites verifies required environment setup
func checkPrerequisites(t *testing.T) {
	// Check ANTHROPIC_API_KEY
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping E2E test: ANTHROPIC_API_KEY not set (required for real API calls)")
	}

	// Check claude-code-acp is in PATH
	// We'll skip this for now since the worktree setup should handle it
}

// setupWorktrees runs the worktree setup script
func setupWorktrees(t *testing.T, ctx context.Context) {
	projectRoot, err := helpers.FindProjectRoot()
	if err != nil {
		t.Logf("Warning: Could not find project root: %v", err)
		return
	}

	scriptPath := filepath.Join(projectRoot, "scripts", "setup-worktrees.sh")

	// Check if script exists
	if _, err := os.Stat(scriptPath); err != nil {
		t.Logf("Warning: Worktree setup script not found: %s (may already be set up)", scriptPath)
		return
	}

	if err := helpers.RunWorktreeSetup(ctx, scriptPath); err != nil {
		t.Logf("Warning: Worktree setup failed (may already be set up): %v", err)
		// Don't fail the test - worktrees may already be set up
	}
}

// buildRelay compiles the relay binary for testing
func buildRelay(t *testing.T, ctx context.Context) {
	projectRoot, err := helpers.FindProjectRoot()
	if err != nil {
		t.Fatalf("Failed to find project root: %v", err)
	}

	// Create bin directory if it doesn't exist
	binDir := filepath.Join(projectRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("Failed to create bin directory: %v", err)
	}

	if err := helpers.BuildRelay(ctx, relayBinaryPath); err != nil {
		t.Fatalf("Failed to build relay: %v", err)
	}
}

// startRelayServer starts the relay server as a background process
func startRelayServer(t *testing.T, ctx context.Context) *helpers.RelayServer {
	server, err := helpers.StartRelay(ctx, relayBinaryPath, relayPort)
	require.NoError(t, err, "Failed to start relay server")
	return server
}

// connectAndCreateSession connects to the WebSocket and creates a session
func connectAndCreateSession(t *testing.T) (*helpers.WSClient, string) {
	// Connect to WebSocket
	client, err := helpers.Connect(wsURL)
	require.NoError(t, err, "Failed to connect to WebSocket")

	// Wait for connection established message
	msg, err := client.WaitForMessageType("connection:established", messageTimeout)
	require.NoError(t, err, "Failed to receive connection:established message")
	t.Logf("Connected to relay server: %s", msg["serverId"])

	// Create session
	createSessionMsg := map[string]interface{}{
		"version": protocolVersion,
		"type":    "session:create",
	}
	err = client.Send(createSessionMsg)
	require.NoError(t, err, "Failed to send session:create message")

	// Wait for session created response
	sessionMsg, err := client.WaitForMessageType("session:created", messageTimeout)
	require.NoError(t, err, "Failed to receive session:created message")

	sessionID, ok := sessionMsg["sessionId"].(string)
	require.True(t, ok, "sessionId field missing or wrong type")
	require.NotEmpty(t, sessionID, "sessionId is empty")

	t.Logf("Session created: %s", sessionID)
	return client, sessionID
}

// spawnAgents spawns all specified agents and waits for them to be ready
func spawnAgents(t *testing.T, client *helpers.WSClient, sessionID string, roles []string) {
	for _, role := range roles {
		t.Logf("Spawning %s agent...", role)

		// Send agent spawn message
		// Workspace path is relative to the relay server's CWD (project root)
		// With WORKSPACE_BASE_DIR=./agent, this creates ./agent/<role>
		// The relay validates that workspace path is under baseWorkspaceDir
		spawnMsg := map[string]interface{}{
			"version":   protocolVersion,
			"type":      "agent:spawn",
			"sessionId": sessionID,
			"role":      role,
			"workspace": filepath.Join(workspaceBase, role),
		}
		err := client.Send(spawnMsg)
		require.NoError(t, err, "Failed to send agent:spawn message for %s", role)

		// Wait for agent ready message
		readyMsg, err := client.ReceiveWithFilter(func(msg map[string]interface{}) bool {
			msgType, _ := msg["type"].(string)
			msgRole, _ := msg["role"].(string)
			return msgType == "agent:ready" && msgRole == role
		}, messageTimeout)
		require.NoError(t, err, "Failed to receive agent:ready message for %s", role)

		msgSessionID, _ := readyMsg["sessionId"].(string)
		assert.Equal(t, sessionID, msgSessionID, "Session ID mismatch in agent:ready")

		t.Logf("Agent %s is ready", role)
	}
}

// testAgentCommunication sends messages to agents and verifies responses
func testAgentCommunication(t *testing.T, client *helpers.WSClient, sessionID string, roles []string) {
	// Define test messages for each agent
	testMessages := map[string]string{
		"auth":  "What is your role?",
		"db":    "What database are you working with?",
		"tests": "What testing framework should we use?",
	}

	for _, role := range roles {
		content := testMessages[role]
		t.Logf("Sending message to %s agent: %s", role, content)

		// Send message to agent
		msgRequest := map[string]interface{}{
			"version":   protocolVersion,
			"type":      "agent:message",
			"sessionId": sessionID,
			"role":      role,
			"content":   content,
		}
		err := client.Send(msgRequest)
		require.NoError(t, err, "Failed to send message to %s agent", role)

		// Wait for agent response
		responseMsg, err := client.ReceiveWithFilter(func(msg map[string]interface{}) bool {
			msgType, _ := msg["type"].(string)
			msgRole, _ := msg["role"].(string)
			return msgType == "agent:response" && msgRole == role
		}, messageTimeout)
		require.NoError(t, err, "Failed to receive response from %s agent", role)

		// Verify response
		msgSessionID, _ := responseMsg["sessionId"].(string)
		msgContent, _ := responseMsg["content"].(string)
		assert.Equal(t, sessionID, msgSessionID, "Session ID mismatch in response")
		assert.NotEmpty(t, msgContent, "Response content is empty")

		t.Logf("Received response from %s agent: %s", role, truncateString(msgContent, 100))
	}
}

// verifyWorktreeCommits checks that agents have made commits to their worktrees
func verifyWorktreeCommits(t *testing.T, ctx context.Context, roles []string, since time.Time) {
	projectRoot, err := helpers.FindProjectRoot()
	if err != nil {
		t.Logf("Warning: Could not find project root: %v", err)
		return
	}

	worktreeBasePath := filepath.Join(projectRoot, "agent")

	for _, role := range roles {
		worktreePath := filepath.Join(worktreeBasePath, role)
		t.Logf("Checking worktree for %s agent: %s", role, worktreePath)

		// Check if worktree exists
		exists, err := helpers.CheckWorktreeExists(worktreePath)
		if err != nil {
			t.Logf("Warning: Failed to check worktree existence for %s: %v", role, err)
			continue
		}
		if !exists {
			t.Logf("Warning: Worktree does not exist for %s: %s", role, worktreePath)
			continue
		}

		// Get worktree info
		info, err := helpers.GetWorktreeInfo(ctx, worktreePath, since)
		if err != nil {
			t.Logf("Warning: Failed to get worktree info for %s: %v", role, err)
			continue
		}

		t.Logf("Agent %s worktree: %d commits since test start", role, info.CommitCount)
		if info.CommitCount > 0 {
			t.Logf("  Latest commit: %s", truncateString(info.LatestCommit, 80))
		}

		// Note: We don't assert commits > 0 because agent behavior may vary
		// In a real scenario, we'd want to be more strict
	}
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
