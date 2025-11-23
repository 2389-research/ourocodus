// Demo program for Phase 3: ACP Bridge for CLI Agent Adoption
//
// This demo showcases the new ACPBridge functionality that enables
// bidirectional communication with CLI-spawned agents via Docker attach.
//
// Usage:
//
//	go run cmd/demo-phase3/main.go
//
// Prerequisites:
//   - Docker daemon running
//   - bin/agentd binary built
//   - ANTHROPIC_API_KEY set (or sk-test-dummy for demo)
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/session"
)

const (
	demoAgentID = "demo-phase3-agent"
	timeout     = 60 * time.Second
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Phase 3 Demo: ACP Bridge for CLI Agent Adoption          ║")
	fmt.Println("║  Demonstrates Docker-based agent discovery and ACP comms   ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	if err := runDemo(); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Demo failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Demo completed successfully!")
}

func runDemo() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Step 1: Prerequisites check
	fmt.Println("📋 Step 1: Checking prerequisites...")
	if err := checkPrerequisites(); err != nil {
		return fmt.Errorf("prerequisites check failed: %w", err)
	}
	fmt.Println("   ✓ Docker daemon accessible")
	fmt.Println("   ✓ agentd binary found")
	fmt.Println()

	// Step 2: Spawn CLI agent
	fmt.Printf("🚀 Step 2: Spawning CLI agent (ID: %s)...\n", demoAgentID)
	if err := spawnAgent(ctx, demoAgentID); err != nil {
		return fmt.Errorf("failed to spawn agent: %w", err)
	}
	fmt.Println("   ✓ Agent spawned successfully")
	fmt.Println()

	// Ensure cleanup happens
	defer func() {
		fmt.Println("\n🧹 Cleanup: Terminating demo agent...")
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := terminateAgent(cleanupCtx, demoAgentID); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  Warning: Failed to cleanup agent: %v\n", err)
		} else {
			fmt.Println("   ✓ Agent terminated")
		}
	}()

	// Step 3: Discover agent container using Docker labels
	fmt.Println("🔍 Step 3: Discovering agent container via Docker labels...")
	containerID, workspace, err := session.FindAgentContainerIDForTesting(ctx, demoAgentID)
	if err != nil {
		return fmt.Errorf("failed to discover agent container: %w", err)
	}
	shortID := containerID
	if len(containerID) > 12 {
		shortID = containerID[:12]
	}
	fmt.Printf("   ✓ Found container: %s\n", shortID)
	fmt.Printf("   ✓ Workspace: %s\n", workspace)
	fmt.Println()

	// Step 4: Create ACP Bridge
	fmt.Println("🌉 Step 4: Creating ACP Bridge...")
	bridge, err := session.NewACPBridge(ctx, containerID, demoAgentID)
	if err != nil {
		return fmt.Errorf("failed to create ACP bridge: %w", err)
	}
	defer func() {
		fmt.Println("\n🔌 Closing ACP Bridge...")
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer closeCancel()
		if err := bridge.Close(closeCtx); err != nil {
			fmt.Fprintf(os.Stderr, "   ⚠️  Warning: Failed to close bridge: %v\n", err)
		} else {
			fmt.Println("   ✓ Bridge closed cleanly")
		}
	}()
	fmt.Println("   ✓ Bridge established")
	fmt.Println()

	// Step 5: Send test messages through the bridge
	fmt.Println("💬 Step 5: Testing ACP communication...")
	fmt.Println()

	// Test message 1: Simple greeting
	if err := sendTestMessage(ctx, bridge, "Hello! Can you confirm you're receiving this message?"); err != nil {
		return fmt.Errorf("test message 1 failed: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Test message 2: Ask about workspace
	if err := sendTestMessage(ctx, bridge, "What files are in your current workspace?"); err != nil {
		return fmt.Errorf("test message 2 failed: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Test message 3: Simple coding task
	if err := sendTestMessage(ctx, bridge, "Write a simple hello world function in Go."); err != nil {
		return fmt.Errorf("test message 3 failed: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ All messages sent and received successfully!")
	fmt.Println()
	fmt.Println("🎉 Key achievements demonstrated:")
	fmt.Println("   • Docker label-based agent discovery")
	fmt.Println("   • ACPBridge creation and attachment")
	fmt.Println("   • Bidirectional JSON-RPC communication")
	fmt.Println("   • Clean shutdown and resource cleanup")

	return nil
}

func checkPrerequisites() error {
	// Check Docker
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker daemon not accessible: %w", err)
	}

	// Check agentd binary
	if _, err := os.Stat("bin/agentd"); err != nil {
		return fmt.Errorf("bin/agentd not found (run 'make build' first): %w", err)
	}

	return nil
}

func spawnAgent(ctx context.Context, agentID string) error {
	// Set dummy API key if not set
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		if err := os.Setenv("ANTHROPIC_API_KEY", "sk-test-dummy"); err != nil {
			return fmt.Errorf("failed to set ANTHROPIC_API_KEY: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "./bin/agentd", "spawn", agentID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ() // Inherit all environment variables including DOCKER_HOST

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agentd spawn failed: %w", err)
	}

	// Give agent a moment to fully initialize
	time.Sleep(2 * time.Second)

	return nil
}

func terminateAgent(ctx context.Context, agentID string) error {
	cmd := exec.CommandContext(ctx, "./bin/agentd", "stop", agentID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ() // Inherit all environment variables including DOCKER_HOST
	return cmd.Run()
}

func sendTestMessage(ctx context.Context, bridge *session.ACPBridge, content string) error {
	fmt.Printf("   📤 Sending: %q\n", content)

	msgCtx, msgCancel := context.WithTimeout(ctx, 10*time.Second)
	defer msgCancel()

	response, err := bridge.SendMessage(msgCtx, content)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Parse response
	respMap, ok := response.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected response type: %T", response)
	}

	fmt.Printf("   📥 Response type: %v\n", respMap["type"])

	// Show response content (truncate if too long)
	if text, ok := respMap["text"].(string); ok && text != "" {
		if len(text) > 100 {
			fmt.Printf("   📝 Content: %s... (truncated)\n", text[:100])
		} else {
			fmt.Printf("   📝 Content: %s\n", text)
		}
	} else if content, ok := respMap["content"].(string); ok && content != "" {
		if len(content) > 100 {
			fmt.Printf("   📝 Content: %s... (truncated)\n", content[:100])
		} else {
			fmt.Printf("   📝 Content: %s\n", content)
		}
	}

	fmt.Println()
	return nil
}
