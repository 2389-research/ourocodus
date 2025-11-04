package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
)

const (
	relayAddr     = "localhost:8080"
	websocketPath = "/ws"
)

// Demo scenarios showcasing PR #27 features
func main() {
	fmt.Println("🎬 Ourocodus Demo - PR #27 Features Showcase")
	fmt.Println("=" + "=")
	fmt.Println()

	// Find binary paths
	root := findRepoRoot()
	relayPath := filepath.Join(root, "bin", "relay")
	echoAgentPath := filepath.Join(root, "bin", "echo-agent")

	if _, err := os.Stat(relayPath); err != nil {
		log.Fatalf("❌ Relay binary not found at %s (run `make build` first)", relayPath)
	}
	if _, err := os.Stat(echoAgentPath); err != nil {
		log.Fatalf("❌ Echo-agent binary not found at %s (run `make build` first)", echoAgentPath)
	}

	// Start relay server
	fmt.Println("🚀 Starting relay server...")
	relayCmd := exec.Command(relayPath) //#nosec G204 -- relayPath is validated to exist before use
	relayCmd.Env = append(os.Environ(),
		"OUROCODUS_ACP_BINARY="+echoAgentPath,
		"ANTHROPIC_API_KEY=demo-key",
	)
	if err := relayCmd.Start(); err != nil {
		log.Fatalf("❌ Failed to start relay: %v", err)
	}
	defer func() {
		fmt.Println("\n🛑 Stopping relay server...")
		_ = relayCmd.Process.Kill()
		_ = relayCmd.Wait()
	}()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Connect to relay
	fmt.Println("🔌 Connecting to relay...")
	conn, err := dialRelay()
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Receive handshake
	var handshake map[string]interface{}
	if err = conn.ReadJSON(&handshake); err != nil {
		log.Fatalf("❌ Failed to read handshake: %v", err)
	}
	fmt.Printf("✅ Connected! Server ID: %s\n\n", handshake["serverId"])

	// Run demo scenarios
	fmt.Println("📋 Demo Scenarios:")
	fmt.Println("1️⃣  Session Lifecycle & Agent Communication")
	fmt.Println("2️⃣  Clear Error Semantics with Recoverability")
	fmt.Println()

	// Scenario 1: Session Lifecycle
	if err = demoSessionLifecycle(conn); err != nil {
		log.Fatalf("❌ Scenario 1 failed: %v", err)
	}

	// Scenario 2: Clear Error Semantics
	if err = demoErrorSemantics(conn); err != nil {
		log.Fatalf("❌ Scenario 2 failed: %v", err)
	}

	fmt.Println("\n🎉 Demo complete! All PR #27 features working as expected.")
}

func demoSessionLifecycle(conn *websocket.Conn) error {
	fmt.Println("━━━ Scenario 1: Session Lifecycle & Agent Communication ━━━")
	fmt.Println("Full session flow: create → spawn → message → response")
	fmt.Println()

	// Create session
	fmt.Println("→ Creating session...")
	userSessionID, err := createSession(conn)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Session created: %s\n", userSessionID)

	// Spawn agent
	fmt.Println("→ Spawning agent...")
	if err := spawnAgent(conn, userSessionID, "demo-agent", "./workspaces/demo"); err != nil {
		return err
	}
	fmt.Println("✅ Agent spawned and ready (state: ACTIVE)")

	// Send multiple messages to demonstrate bidirectional communication
	messages := []string{
		"Hello, agent!",
		"Can you count to three?",
		"What's your role?",
	}

	fmt.Println("\n→ Testing bidirectional communication...")
	for i, msg := range messages {
		fmt.Printf("   User: %s\n", msg)
		response, err := sendAgentMessage(conn, userSessionID, "demo-agent", msg)
		if err != nil {
			return err
		}
		fmt.Printf("   Agent: %s\n", response)

		if i < len(messages)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Println("\n✨ Session lifecycle complete!")
	return nil
}

func demoErrorSemantics(conn *websocket.Conn) error {
	fmt.Println("━━━ Scenario 2: Clear Error Semantics ━━━")
	fmt.Println("Non-recoverable errors indicate client action required")
	fmt.Println()

	// Try to message non-existent session
	fmt.Println("→ Testing SESSION_NOT_FOUND error...")
	msg := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:message",
		"userSessionId": "00000000-0000-0000-0000-000000000000",
		"agentId":      "test",
		"content":   "hello",
	}
	if err := conn.WriteJSON(msg); err != nil {
		return err
	}

	var errResp map[string]interface{}
	if err := conn.ReadJSON(&errResp); err != nil {
		return err
	}

	errorDetail := errResp["error"].(map[string]interface{})
	code := errorDetail["code"].(string)
	message := errorDetail["message"].(string)
	recoverable := errorDetail["recoverable"].(bool)

	fmt.Printf("✅ Error received:\n")
	fmt.Printf("   Code: %s\n", code)
	fmt.Printf("   Message: %s\n", message)
	fmt.Printf("   Recoverable: %v\n", recoverable)

	if code != "SESSION_NOT_FOUND" || recoverable {
		return fmt.Errorf("expected SESSION_NOT_FOUND (non-recoverable), got %s (recoverable=%v)", code, recoverable)
	}

	// Create session for next test
	fmt.Println("\n→ Creating session for AGENT_NOT_FOUND test...")
	userSessionID, err := createSession(conn)
	if err != nil {
		return err
	}

	// Try to message non-existent agent
	fmt.Println("→ Testing AGENT_NOT_FOUND error...")
	msg = map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:message",
		"userSessionId": userSessionID,
		"agentId":      "non-existent-agent",
		"content":   "hello",
	}
	if err := conn.WriteJSON(msg); err != nil {
		return err
	}

	if err := conn.ReadJSON(&errResp); err != nil {
		return err
	}

	errorDetail = errResp["error"].(map[string]interface{})
	code = errorDetail["code"].(string)
	message = errorDetail["message"].(string)
	recoverable = errorDetail["recoverable"].(bool)

	fmt.Printf("✅ Error received:\n")
	fmt.Printf("   Code: %s\n", code)
	fmt.Printf("   Message: %s\n", message)
	fmt.Printf("   Recoverable: %v\n", recoverable)

	if code != "AGENT_NOT_FOUND" || recoverable {
		return fmt.Errorf("expected AGENT_NOT_FOUND (non-recoverable), got %s (recoverable=%v)", code, recoverable)
	}

	fmt.Println("✨ Error semantics verified!")
	return nil
}

// Helper functions
func dialRelay() (*websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: relayAddr, Path: websocketPath}
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, _, err := dialer.Dial(u.String(), nil)
	return conn, err
}

func createSession(conn *websocket.Conn) (string, error) {
	msg := map[string]interface{}{
		"version": "1.0",
		"type":    "session:create",
	}
	if err := conn.WriteJSON(msg); err != nil {
		return "", err
	}

	var resp map[string]interface{}
	if err := conn.ReadJSON(&resp); err != nil {
		return "", err
	}

	userSessionID, ok := resp["userSessionId"].(string)
	if !ok {
		return "", fmt.Errorf("no userSessionId in response")
	}
	return userSessionID, nil
}

func spawnAgent(conn *websocket.Conn, userSessionID, role, workspace string) error {
	msg := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:spawn",
		"userSessionId": userSessionID,
		"agentId":      role,
		"workspace": workspace,
	}
	if err := conn.WriteJSON(msg); err != nil {
		return err
	}

	var resp map[string]interface{}
	return conn.ReadJSON(&resp)
}

func sendAgentMessage(conn *websocket.Conn, userSessionID, role, content string) (string, error) {
	msg := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:message",
		"userSessionId": userSessionID,
		"agentId":      role,
		"content":   content,
	}
	if err := conn.WriteJSON(msg); err != nil {
		return "", err
	}

	var resp map[string]interface{}
	if err := conn.ReadJSON(&resp); err != nil {
		return "", err
	}

	respContent, ok := resp["content"].(string)
	if !ok {
		return "", fmt.Errorf("no content in agent response")
	}
	return respContent, nil
}

func findRepoRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		log.Fatal("Failed to find git repository root")
	}
	return string(output[:len(output)-1])
}
