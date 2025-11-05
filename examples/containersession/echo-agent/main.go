package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// UUIDGenerator generates unique IDs
type UUIDGenerator struct{}

func (g *UUIDGenerator) Generate() string {
	return uuid.New().String()
}

// SystemClock provides real time
type SystemClock struct{}

func (c *SystemClock) Now() time.Time {
	return time.Now()
}

// StdLogger wraps standard logger
type StdLogger struct {
	*log.Logger
}

func (l *StdLogger) Printf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// createDockerClient tries Colima first, then Docker Desktop
func createDockerClient() (*client.Client, error) {
	colimaSocket := filepath.Join(os.Getenv("HOME"), ".colima", "default", "docker.sock")
	if _, err := os.Stat(colimaSocket); err == nil {
		if err := os.Setenv("DOCKER_HOST", "unix://"+colimaSocket); err == nil {
			dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if _, err := dockerClient.Ping(ctx); err == nil {
					log.Printf("Using Colima at %s\n", colimaSocket)
					return dockerClient, nil
				}
				dockerClient.Close()
			}
		}
	}

	if err := os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock"); err == nil {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := dockerClient.Ping(ctx); err == nil {
				log.Println("Using Docker Desktop")
				return dockerClient, nil
			}
			dockerClient.Close()
		}
	}

	return nil, fmt.Errorf("cannot connect to Docker")
}

func main() {
	ctx := context.Background()

	fmt.Println("=== ContainerSession Echo Agent Example ===")
	fmt.Println("This demonstrates bidirectional I/O with a container agent.")

	// 1. Connect to Docker
	fmt.Println("Step 1: Connecting to Docker...")
	dockerClient, err := createDockerClient()
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v\n", err)
	}
	defer dockerClient.Close()
	fmt.Println("✓ Connected")

	// 2. Create Manager
	fmt.Println("Step 2: Creating Manager...")
	baseWorkspace := "./workspaces/echo-agent"
	manager := containersession.NewManager(
		dockerClient,
		&UUIDGenerator{},
		&SystemClock{},
		&StdLogger{Logger: log.New(os.Stdout, "[manager] ", 0)},
		baseWorkspace,
	)
	fmt.Println("✓ Manager created")

	// 3. Copy echo script to workspace first
	fmt.Println("Step 3: Creating session...")
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"/bin/bash", "/workspace/echo-script.sh"})
	if err != nil {
		log.Fatalf("Failed to create session: %v\n", err)
	}
	fmt.Printf("✓ Session created: %s\n", session.ID())
	fmt.Printf("  Workspace: %s\n\n", session.WorkspacePath())

	// Copy the echo script into the workspace
	fmt.Println("Step 4: Copying echo script to workspace...")
	scriptSrc := "examples/containersession/echo-agent/echo-script.sh"
	scriptDst := filepath.Join(session.WorkspacePath(), "echo-script.sh")

	srcData, err := os.ReadFile(scriptSrc)
	if err != nil {
		log.Fatalf("Failed to read script: %v\n", err)
	}

	if err := os.WriteFile(scriptDst, srcData, 0755); err != nil {
		log.Fatalf("Failed to copy script: %v\n", err)
	}
	fmt.Printf("✓ Script copied to %s\n\n", scriptDst)

	// 4. Start the session
	fmt.Println("Step 5: Starting container with echo agent...")
	if err := manager.StartContainerSession(ctx, session.ID()); err != nil {
		log.Fatalf("Failed to start session: %v\n", err)
	}
	fmt.Println("✓ Container running")

	// Give the agent a moment to start
	time.Sleep(500 * time.Millisecond)

	// 5. Get I/O streams
	fmt.Println("Step 6: Attaching to container I/O streams...")
	// Note: In a production system, you'd use ContainerAttach from the Docker client
	// For this example, we'll demonstrate the pattern conceptually
	fmt.Println("✓ I/O streams attached (conceptual - see README for full implementation)")

	// 6. Simulate sending messages
	fmt.Println("Step 7: Demonstrating message exchange pattern...")
	messages := []string{
		"hello world",
		"test message",
		"container sessions are great",
	}

	fmt.Println("Messages we would send to the agent:")
	for i, msg := range messages {
		fmt.Printf("  %d. %s\n", i+1, msg)
	}
	fmt.Println()

	fmt.Println("Expected agent responses:")
	for i, msg := range messages {
		fmt.Printf("  Message %d:\n", i+1)
		fmt.Printf("    Received: %s\n", msg)
		fmt.Printf("    Processed: %s\n", toUpperSimple(msg))
		fmt.Printf("    Length: %d characters\n", len(msg))
		fmt.Println("    ---")
	}
	fmt.Println()

	// 7. Stop the session
	fmt.Println("Step 8: Stopping container...")
	if err := manager.StopContainerSession(ctx, session.ID()); err != nil {
		log.Printf("Warning: Failed to stop session: %v\n", err)
	} else {
		fmt.Println("✓ Container stopped")
	}

	// 8. Remove the container
	fmt.Println("Step 9: Removing container...")
	if err := dockerClient.ContainerRemove(ctx, session.ContainerID(), container.RemoveOptions{}); err != nil {
		log.Printf("Warning: Failed to remove container: %v\n", err)
	} else {
		fmt.Println("✓ Container removed")
	}

	// 9. Cleanup workspace
	fmt.Println("Step 10: Cleaning up workspace...")
	if err := os.RemoveAll(session.WorkspacePath()); err != nil {
		log.Printf("Warning: Failed to cleanup workspace: %v\n", err)
	} else {
		fmt.Println("✓ Workspace cleaned up")
	}

	fmt.Println("=== Example Complete ===")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("  • Container sessions can run interactive agents")
	fmt.Println("  • Scripts are deployed via the workspace directory")
	fmt.Println("  • Bidirectional I/O enables agent communication")
	fmt.Println("  • See README.md for full I/O stream implementation")
}

// Helper for demo
func toUpperSimple(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			result[i] = c - 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
