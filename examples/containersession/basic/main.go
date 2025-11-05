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

// UUIDGenerator generates unique IDs using Google's UUID library
type UUIDGenerator struct{}

func (g *UUIDGenerator) Generate() string {
	return uuid.New().String()
}

// SystemClock provides real time
type SystemClock struct{}

func (c *SystemClock) Now() time.Time {
	return time.Now()
}

// StdLogger wraps standard logger to implement containersession.Logger interface
type StdLogger struct {
	*log.Logger
}

func (l *StdLogger) Printf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// createDockerClient tries to connect to Docker: Colima first, then Docker Desktop.
// Note: This function assumes a Unix-like environment (macOS/Linux) and will not
// work on Windows without modification to use named pipes.
func createDockerClient() (*client.Client, error) {
	// Try Colima socket first
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

	// Try Docker Desktop
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

	return nil, fmt.Errorf("cannot connect to Docker - tried Colima (%s) and Docker Desktop", colimaSocket)
}

func main() {
	ctx := context.Background()

	fmt.Println("=== ContainerSession Basic Example ===")
	fmt.Println("This demonstrates the simplest container session lifecycle:")

	// 1. Create Docker client
	fmt.Println("Step 1: Connecting to Docker...")
	dockerClient, err := createDockerClient()
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v\n", err)
	}
	defer dockerClient.Close()
	fmt.Println("✓ Connected to Docker")

	// 2. Create Manager
	fmt.Println("Step 2: Creating Manager...")
	baseWorkspace := "./workspaces/basic-example"
	manager := containersession.NewManager(
		dockerClient,
		&UUIDGenerator{},
		&SystemClock{},
		&StdLogger{Logger: log.New(os.Stdout, "[manager] ", 0)},
		baseWorkspace,
	)
	fmt.Println("✓ Manager created")

	// 3. Create container session
	fmt.Println("Step 3: Creating container session...")
	session, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "60"})
	if err != nil {
		log.Fatalf("Failed to create session: %v\n", err)
	}
	fmt.Printf("✓ Session created\n")
	fmt.Printf("  Session ID: %s\n", session.ID())
	fmt.Printf("  Container ID: %s\n", session.ContainerID()[:12])
	fmt.Printf("  State: %s\n", session.State())
	fmt.Printf("  Workspace: %s\n\n", session.WorkspacePath())

	// 4. Start the session
	fmt.Println("Step 4: Starting container...")
	if err := manager.StartContainerSession(ctx, session.ID()); err != nil {
		log.Fatalf("Failed to start session: %v\n", err)
	}
	fmt.Printf("✓ Container started\n")
	fmt.Printf("  State: %s\n\n", session.State())

	// 5. Write a file to the workspace
	fmt.Println("Step 5: Writing data to workspace...")
	testFile := filepath.Join(session.WorkspacePath(), "example.txt")
	testContent := "Hello from containersession basic example!"
	if err := os.WriteFile(testFile, []byte(testContent), 0o644); err != nil {
		log.Fatalf("Failed to write to workspace: %v\n", err)
	}
	fmt.Printf("✓ Wrote file: %s\n", testFile)
	fmt.Printf("  Content: %s\n\n", testContent)

	// 6. Read it back to verify
	fmt.Println("Step 6: Reading data from workspace...")
	readContent, err := os.ReadFile(testFile)
	if err != nil {
		log.Fatalf("Failed to read from workspace: %v\n", err)
	}
	fmt.Printf("✓ Read file: %s\n", testFile)
	fmt.Printf("  Content: %s\n\n", string(readContent))

	// 7. Stop the session
	fmt.Println("Step 7: Stopping container...")
	if err := manager.StopContainerSession(ctx, session.ID()); err != nil {
		log.Fatalf("Failed to stop session: %v\n", err)
	}
	fmt.Printf("✓ Container stopped\n")
	fmt.Printf("  State: %s\n\n", session.State())

	// 8. Remove the container
	fmt.Println("Step 8: Removing container...")
	if err := dockerClient.ContainerRemove(ctx, session.ContainerID(), container.RemoveOptions{}); err != nil {
		log.Printf("Warning: Failed to remove container: %v\n", err)
	} else {
		fmt.Println("✓ Container removed")
	}

	// 9. Cleanup workspace
	fmt.Println("Step 9: Cleaning up workspace...")
	if err := os.RemoveAll(session.WorkspacePath()); err != nil {
		log.Printf("Warning: Failed to cleanup workspace: %v\n", err)
	} else {
		fmt.Println("✓ Workspace cleaned up")
	}

	fmt.Println("=== Example Complete ===")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("  • Container sessions provide isolated execution environments")
	fmt.Println("  • Workspace directories persist data between host and container")
	fmt.Println("  • Session lifecycle: PENDING → RUNNING → STOPPED")
	fmt.Println("  • Manager handles all Docker operations transparently")
}
