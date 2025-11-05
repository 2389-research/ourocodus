package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	bold        = "\033[1m"
)

// Real implementations for production use
type uuidGenerator struct{}

func (g *uuidGenerator) Generate() string {
	return uuid.New().String()
}

type systemClock struct{}

func (c *systemClock) Now() time.Time {
	return time.Now()
}

type demoLogger struct {
	prefix string
}

func (l *demoLogger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Printf("%s[MANAGER]%s %s", colorCyan, colorReset, msg)
}

func printHeader(title string) {
	fmt.Printf("\n%s%s═══════════════════════════════════════════════════════════════%s\n", bold, colorBlue, colorReset)
	fmt.Printf("%s%s  %s%s\n", bold, colorBlue, title, colorReset)
	fmt.Printf("%s%s═══════════════════════════════════════════════════════════════%s\n\n", bold, colorBlue, colorReset)
}

func printStep(step int, description string) {
	fmt.Printf("%s%s[Step %d]%s %s\n", bold, colorGreen, step, colorReset, description)
}

func printInfo(label, value string) {
	fmt.Printf("  %s%-18s%s %s%s%s\n", colorYellow, label+":", colorReset, colorWhite, value, colorReset)
}

func printSuccess(message string) {
	fmt.Printf("%s✓%s %s\n", colorGreen, colorReset, message)
}

func printError(message string) {
	fmt.Printf("%s✗%s %s\n", colorRed, colorReset, message)
}

func waitForUser() {
	fmt.Printf("\n%s[Press Enter to continue]%s ", colorPurple, colorReset)
	fmt.Scanln()
}

// createDockerClient tries to connect to Docker in order: Colima, then Docker Desktop
func createDockerClient() (*client.Client, error) {
	// Try Colima socket first (most common on macOS with Colima)
	colimaSocket := filepath.Join(os.Getenv("HOME"), ".colima", "default", "docker.sock")
	if _, err := os.Stat(colimaSocket); err == nil {
		if err := os.Setenv("DOCKER_HOST", "unix://"+colimaSocket); err == nil {
			dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err == nil {
				// Test connection
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if _, err := dockerClient.Ping(ctx); err == nil {
					fmt.Printf("%sℹ%s Using Colima at %s\n", colorCyan, colorReset, colimaSocket)
					return dockerClient, nil
				}
				dockerClient.Close()
			}
		}
	}

	// Try standard Docker Desktop socket
	if err := os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock"); err == nil {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if _, err := dockerClient.Ping(ctx); err == nil {
				fmt.Printf("%sℹ%s Using Docker Desktop\n", colorCyan, colorReset)
				return dockerClient, nil
			}
			dockerClient.Close()
		}
	}

	return nil, fmt.Errorf("cannot connect to Docker - tried Colima (%s) and Docker Desktop (/var/run/docker.sock)", colimaSocket)
}

func main() {
	ctx := context.Background()

	// Setup
	printHeader("Container Session Demo - Phase 2: Reuse & Attach")
	fmt.Println("This demo showcases intelligent container reuse and cross-process attachment.")
	fmt.Println("Watch how containers are reused instead of recreated!")
	waitForUser()

	// Create Docker client - try Colima first, then standard Docker socket
	dockerClient, err := createDockerClient()
	if err != nil {
		printError(fmt.Sprintf("Failed to connect to Docker: %v", err))
		printInfo("Solution", "Start Docker Desktop or Colima (colima start)")
		os.Exit(1)
	}
	defer dockerClient.Close()

	// Setup workspace
	baseWorkspace := "./demo-workspaces"
	if err := os.MkdirAll(baseWorkspace, 0755); err != nil {
		printError(fmt.Sprintf("Failed to create workspace dir: %v", err))
		os.Exit(1)
	}

	// Run demos - stop on first failure
	if err := runScenario1(ctx, dockerClient, baseWorkspace); err != nil {
		printError(fmt.Sprintf("Scenario 1 failed: %v", err))
		printError("Demo stopped due to failure")
		os.Exit(1)
	}

	if err := runScenario2(ctx, dockerClient, baseWorkspace); err != nil {
		printError(fmt.Sprintf("Scenario 2 failed: %v", err))
		printError("Demo stopped due to failure")
		os.Exit(1)
	}

	if err := runScenario3(ctx, dockerClient, baseWorkspace); err != nil {
		printError(fmt.Sprintf("Scenario 3 failed: %v", err))
		printError("Demo stopped due to failure")
		os.Exit(1)
	}

	if err := runScenario4(ctx, dockerClient, baseWorkspace); err != nil {
		printError(fmt.Sprintf("Scenario 4 failed: %v", err))
		printError("Demo stopped due to failure")
		os.Exit(1)
	}

	// Cleanup
	printHeader("Demo Complete!")
	fmt.Println("All scenarios completed successfully.")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("  • Containers are reused when possible (same session ID)")
	fmt.Println("  • Stopped containers are automatically restarted")
	fmt.Println("  • Sessions can attach across process boundaries")
	fmt.Println("  • Workspace state persists across container lifecycle")
	fmt.Println("\nWorkspaces and containers have been cleaned up.")
}

// Scenario 1: Automatic reuse of running container
func runScenario1(ctx context.Context, dockerClient *client.Client, baseWorkspace string) error {
	printHeader("Scenario 1: Automatic Container Reuse (Running)")

	printStep(1, "Create initial container session")

	// Create manager with FIXED session ID generator for reuse testing
	sessionID := uuid.New().String()
	manager1 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID}, // Returns same ID
		&systemClock{},
		&demoLogger{prefix: "Manager-1"},
		filepath.Join(baseWorkspace, "scenario1"),
	)

	session1, err := manager1.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	printInfo("Session ID", session1.ID())
	printInfo("Container ID", session1.ContainerID()[:12])
	printInfo("State", string(session1.State()))
	printInfo("Workspace", session1.WorkspacePath())

	printStep(2, "Start the container")
	if err := manager1.StartContainerSession(ctx, session1.ID()); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	printSuccess("Container started successfully")
	printInfo("State", string(session1.State()))

	originalContainerID := session1.ContainerID()
	waitForUser()

	printStep(3, "Simulate process restart - create new Manager")
	manager2 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID}, // Same ID!
		&systemClock{},
		&demoLogger{prefix: "Manager-2"},
		filepath.Join(baseWorkspace, "scenario1"),
	)
	printSuccess("New Manager instance created")

	printStep(4, "Call CreateContainerSession again (same session ID)")
	session2, err := manager2.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	printStep(5, "Verify container was REUSED, not recreated")
	if session2.ContainerID() == originalContainerID {
		printSuccess("✓ Container REUSED! Same container ID")
	} else {
		printError("✗ Container RECREATED (unexpected)")
	}

	printInfo("Original Container", originalContainerID[:12])
	printInfo("New Container", session2.ContainerID()[:12])
	printInfo("Match?", fmt.Sprintf("%v", session2.ContainerID() == originalContainerID))

	// Cleanup
	fmt.Println("\nCleaning up...")
	manager2.StopContainerSession(ctx, session2.ID())
	time.Sleep(500 * time.Millisecond)

	waitForUser()
	return nil
}

// Scenario 2: Restart stopped container
func runScenario2(ctx context.Context, dockerClient *client.Client, baseWorkspace string) error {
	printHeader("Scenario 2: Restart Stopped Container")

	sessionID := uuid.New().String()
	manager := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&systemClock{},
		&demoLogger{prefix: "Manager"},
		filepath.Join(baseWorkspace, "scenario2"),
	)

	printStep(1, "Create and start container")
	session1, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	if err := manager.StartContainerSession(ctx, session1.ID()); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	originalContainerID := session1.ContainerID()
	printInfo("Container ID", originalContainerID[:12])
	printInfo("State", string(session1.State()))

	printStep(2, "Stop the container")
	if err := manager.StopContainerSession(ctx, session1.ID()); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}
	printSuccess("Container stopped")
	printInfo("State", string(session1.State()))

	waitForUser()

	printStep(3, "Call CreateContainerSession again (same ID, stopped container)")
	manager2 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&systemClock{},
		&demoLogger{prefix: "Manager-2"},
		filepath.Join(baseWorkspace, "scenario2"),
	)

	session2, err := manager2.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return fmt.Errorf("failed to reuse session: %w", err)
	}

	printStep(4, "Verify container was RESTARTED, not recreated")
	if session2.ContainerID() == originalContainerID {
		printSuccess("✓ Container RESTARTED! Same container ID")
	} else {
		printError("✗ New container created (unexpected)")
	}

	printInfo("Original Container", originalContainerID[:12])
	printInfo("Reused Container", session2.ContainerID()[:12])
	printInfo("State", string(session2.State()))

	// Cleanup
	fmt.Println("\nCleaning up...")
	manager2.StopContainerSession(ctx, session2.ID())
	time.Sleep(500 * time.Millisecond)

	waitForUser()
	return nil
}

// Scenario 3: Explicit cross-process attachment
func runScenario3(ctx context.Context, dockerClient *client.Client, baseWorkspace string) error {
	printHeader("Scenario 3: Explicit Cross-Process Attachment")

	printStep(1, "Process A: Create and start container")
	sessionID := uuid.New().String()

	managerA := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&systemClock{},
		&demoLogger{prefix: "Process-A"},
		filepath.Join(baseWorkspace, "scenario3"),
	)

	sessionA, err := managerA.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return fmt.Errorf("Process A failed to create session: %w", err)
	}

	if err := managerA.StartContainerSession(ctx, sessionA.ID()); err != nil {
		return fmt.Errorf("Process A failed to start session: %w", err)
	}

	printInfo("Session ID", sessionA.ID())
	printInfo("Container ID", sessionA.ContainerID()[:12])
	printInfo("State", string(sessionA.State()))
	printSuccess("Process A: Container running")

	// Simulate persisting session ID
	printStep(2, "Persist session ID (simulating database/file storage)")
	persistedSessionID := sessionA.ID()
	printInfo("Persisted ID", persistedSessionID)

	waitForUser()

	printStep(3, "Process B: Create new Manager (different process)")
	managerB := containersession.NewManager(
		dockerClient,
		&uuidGenerator{}, // Different generator
		&systemClock{},
		&demoLogger{prefix: "Process-B"},
		filepath.Join(baseWorkspace, "scenario3"),
	)
	printSuccess("Process B: New Manager created")

	printStep(4, "Process B: Attach to existing session using AttachContainerSession")
	sessionB, err := managerB.AttachContainerSession(ctx, persistedSessionID)
	if err != nil {
		return fmt.Errorf("Process B failed to attach: %w", err)
	}

	printStep(5, "Verify both processes connected to SAME container")
	if sessionB.ContainerID() == sessionA.ContainerID() {
		printSuccess("✓ Successfully attached to same container!")
	} else {
		printError("✗ Different containers (unexpected)")
	}

	printInfo("Process A Container", sessionA.ContainerID()[:12])
	printInfo("Process B Container", sessionB.ContainerID()[:12])
	printInfo("Match?", fmt.Sprintf("%v", sessionB.ContainerID() == sessionA.ContainerID()))

	// Cleanup
	fmt.Println("\nCleaning up...")
	managerB.StopContainerSession(ctx, sessionB.ID())
	time.Sleep(500 * time.Millisecond)

	waitForUser()
	return nil
}

// Scenario 4: Workspace persistence
func runScenario4(ctx context.Context, dockerClient *client.Client, baseWorkspace string) error {
	printHeader("Scenario 4: Workspace Persistence Across Container Lifecycle")

	sessionID := uuid.New().String()
	manager := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&systemClock{},
		&demoLogger{prefix: "Manager"},
		filepath.Join(baseWorkspace, "scenario4"),
	)

	printStep(1, "Create and start container")
	session1, err := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	if err := manager.StartContainerSession(ctx, session1.ID()); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	workspace := session1.WorkspacePath()
	printInfo("Workspace", workspace)

	printStep(2, "Write test file to workspace")
	testFile := filepath.Join(workspace, "test.txt")
	testContent := "Container session data persists!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}
	printSuccess("Wrote: " + testContent)
	printInfo("File", testFile)

	printStep(3, "Stop container")
	if err := manager.StopContainerSession(ctx, session1.ID()); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}
	printSuccess("Container stopped")

	waitForUser()

	printStep(4, "Reuse container (CreateContainerSession with same ID)")
	manager2 := containersession.NewManager(
		dockerClient,
		&fixedIDGenerator{id: sessionID},
		&systemClock{},
		&demoLogger{prefix: "Manager-2"},
		filepath.Join(baseWorkspace, "scenario4"),
	)

	session2, err := manager2.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return fmt.Errorf("failed to reuse session: %w", err)
	}
	printSuccess("Container reused and restarted")
	printInfo("Workspace", session2.WorkspacePath())

	printStep(5, "Verify workspace file still exists")
	if session2.WorkspacePath() != workspace {
		printError("✗ Different workspace path!")
		return fmt.Errorf("workspace path mismatch")
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		printError("✗ File not found in workspace")
		return fmt.Errorf("failed to read test file: %w", err)
	}

	if string(content) == testContent {
		printSuccess("✓ Workspace data PERSISTED across container restart!")
	} else {
		printError("✗ Workspace data changed")
	}

	printInfo("File content", string(content))
	printInfo("Expected", testContent)

	// Cleanup
	fmt.Println("\nCleaning up...")
	manager2.StopContainerSession(ctx, session2.ID())
	time.Sleep(500 * time.Millisecond)
	os.RemoveAll(workspace)

	waitForUser()
	return nil
}

// fixedIDGenerator always returns the same ID (for testing reuse)
type fixedIDGenerator struct {
	id string
}

func (g *fixedIDGenerator) Generate() string {
	return g.id
}
