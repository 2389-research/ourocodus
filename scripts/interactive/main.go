package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	relayAddr     = "localhost:8080"
	websocketPath = "/ws"
)

type replState struct {
	conn      *websocket.Conn
	sessionID string
	agents    map[string]bool // Track spawned agents
}

func main() {
	fmt.Println("🎮 Ourocodus Interactive Demo - REPL")
	fmt.Println("====================================")
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
	relayCmd := exec.Command(relayPath)
	relayCmd.Env = append(os.Environ(),
		"OUROCODUS_ACP_BINARY="+echoAgentPath,
		"ANTHROPIC_API_KEY=interactive-demo-key",
	)
	if err := relayCmd.Start(); err != nil {
		log.Fatalf("❌ Failed to start relay: %v", err)
	}

	// Cleanup on exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\n\n🛑 Shutting down...")
		_ = relayCmd.Process.Kill()
		_ = relayCmd.Wait()
		os.Exit(0)
	}()

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
	defer conn.Close()

	// Receive handshake
	var handshake map[string]interface{}
	if err := conn.ReadJSON(&handshake); err != nil {
		log.Fatalf("❌ Failed to read handshake: %v", err)
	}
	fmt.Printf("✅ Connected! Server ID: %s\n\n", handshake["serverId"])

	// Initialize REPL state
	state := &replState{
		conn:   conn,
		agents: make(map[string]bool),
	}

	// Show help
	printHelp()

	// Start REPL
	scanner := bufio.NewScanner(os.Stdin)
	for {
		// Show prompt
		if state.sessionID != "" {
			fmt.Printf("\n[session:%s] > ", state.sessionID[:8])
		} else {
			fmt.Print("\n[no session] > ")
		}

		// Read input
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse and execute command
		parts := strings.Fields(line)
		cmd := parts[0]

		switch cmd {
		case "help", "h", "?":
			printHelp()

		case "create":
			handleCreate(state)

		case "spawn":
			if len(parts) < 2 {
				fmt.Println("❌ Usage: spawn <role> [workspace]")
				fmt.Println("   Example: spawn assistant ./workspaces/demo")
				continue
			}
			role := parts[1]
			workspace := "./workspaces/interactive"
			if len(parts) >= 3 {
				workspace = parts[2]
			}
			handleSpawn(state, role, workspace)

		case "msg", "message", "send":
			if len(parts) < 3 {
				fmt.Println("❌ Usage: msg <role> <message>")
				fmt.Println("   Example: msg assistant hello there!")
				continue
			}
			role := parts[1]
			message := strings.Join(parts[2:], " ")
			handleMessage(state, role, message)

		case "agents":
			handleListAgents(state)

		case "quit", "exit", "q":
			fmt.Println("👋 Goodbye!")
			return

		default:
			fmt.Printf("❌ Unknown command: %s\n", cmd)
			fmt.Println("   Type 'help' for available commands")
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
	}
}

func printHelp() {
	fmt.Println("📋 Available Commands:")
	fmt.Println("  create                      - Create a new session")
	fmt.Println("  spawn <role> [workspace]    - Spawn an agent (default: ./workspaces/interactive)")
	fmt.Println("  msg <role> <message>        - Send message to agent")
	fmt.Println("  agents                      - List spawned agents")
	fmt.Println("  help                        - Show this help")
	fmt.Println("  quit                        - Exit the REPL")
	fmt.Println()
	fmt.Println("💡 Quick Start:")
	fmt.Println("  1. create")
	fmt.Println("  2. spawn assistant")
	fmt.Println("  3. msg assistant hello!")
}

func handleCreate(state *replState) {
	msg := map[string]interface{}{
		"version": "1.0",
		"type":    "session:create",
	}
	if err := state.conn.WriteJSON(msg); err != nil {
		fmt.Printf("❌ Failed to send: %v\n", err)
		return
	}

	var resp map[string]interface{}
	if err := state.conn.ReadJSON(&resp); err != nil {
		fmt.Printf("❌ Failed to read response: %v\n", err)
		return
	}

	if resp["type"] == "error" {
		errorDetail := resp["error"].(map[string]interface{})
		fmt.Printf("❌ Error: %s - %s\n", errorDetail["code"], errorDetail["message"])
		return
	}

	sessionID, ok := resp["sessionId"].(string)
	if !ok {
		fmt.Printf("❌ Invalid response: %v\n", resp)
		return
	}

	state.sessionID = sessionID
	state.agents = make(map[string]bool) // Reset agents for new session
	fmt.Printf("✅ Session created: %s\n", sessionID)
}

func handleSpawn(state *replState, role, workspace string) {
	if state.sessionID == "" {
		fmt.Println("❌ No session. Run 'create' first.")
		return
	}

	msg := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:spawn",
		"sessionId": state.sessionID,
		"role":      role,
		"workspace": workspace,
	}
	if err := state.conn.WriteJSON(msg); err != nil {
		fmt.Printf("❌ Failed to send: %v\n", err)
		return
	}

	var resp map[string]interface{}
	if err := state.conn.ReadJSON(&resp); err != nil {
		fmt.Printf("❌ Failed to read response: %v\n", err)
		return
	}

	if resp["type"] == "error" {
		errorDetail := resp["error"].(map[string]interface{})
		fmt.Printf("❌ Error: %s - %s\n", errorDetail["code"], errorDetail["message"])
		return
	}

	state.agents[role] = true
	fmt.Printf("✅ Agent '%s' spawned in %s\n", role, workspace)
}

func handleMessage(state *replState, role, content string) {
	if state.sessionID == "" {
		fmt.Println("❌ No session. Run 'create' first.")
		return
	}

	if !state.agents[role] {
		fmt.Printf("⚠️  Agent '%s' not spawned yet. Messages may fail.\n", role)
	}

	msg := map[string]interface{}{
		"version":   "1.0",
		"type":      "agent:message",
		"sessionId": state.sessionID,
		"role":      role,
		"content":   content,
	}
	if err := state.conn.WriteJSON(msg); err != nil {
		fmt.Printf("❌ Failed to send: %v\n", err)
		return
	}

	var resp map[string]interface{}
	if err := state.conn.ReadJSON(&resp); err != nil {
		fmt.Printf("❌ Failed to read response: %v\n", err)
		return
	}

	if resp["type"] == "error" {
		errorDetail := resp["error"].(map[string]interface{})
		recoverable := errorDetail["recoverable"].(bool)
		recoverableStr := "recoverable"
		if !recoverable {
			recoverableStr = "non-recoverable"
		}
		fmt.Printf("❌ Error (%s): %s - %s\n", recoverableStr, errorDetail["code"], errorDetail["message"])
		return
	}

	responseContent, ok := resp["content"].(string)
	if !ok {
		fmt.Printf("❌ Invalid response: %v\n", resp)
		return
	}

	fmt.Printf("🤖 %s: %s\n", role, responseContent)
}

func handleListAgents(state *replState) {
	if state.sessionID == "" {
		fmt.Println("❌ No session. Run 'create' first.")
		return
	}

	if len(state.agents) == 0 {
		fmt.Println("No agents spawned yet. Use 'spawn <role>' to spawn one.")
		return
	}

	fmt.Println("🤖 Spawned Agents:")
	for role := range state.agents {
		fmt.Printf("  - %s\n", role)
	}
}

func dialRelay() (*websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: relayAddr, Path: websocketPath}
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, _, err := dialer.Dial(u.String(), nil)
	return conn, err
}

func findRepoRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		log.Fatal("Failed to find git repository root")
	}
	return string(output[:len(output)-1])
}
