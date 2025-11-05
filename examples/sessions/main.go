package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
	bold        = "\033[1m"
)

// Session hierarchy types
type UserSessionState string

const (
	UserActive      UserSessionState = "ACTIVE"
	UserTerminated  UserSessionState = "TERMINATED"
)

type AgentState string

const (
	AgentSpawning    AgentState = "SPAWNING"
	AgentActive      AgentState = "ACTIVE"
	AgentFailed      AgentState = "FAILED"
	AgentTerminated  AgentState = "TERMINATED"
)

// UserSession represents a user's workspace (backed by a container)
type UserSession struct {
	ID               string
	ContainerSession *containersession.ContainerSession
	Agents           map[string]*AgentSession
	State            UserSessionState
	CreatedAt        time.Time
}

// AgentSession represents an agent within a user session
type AgentSession struct {
	AgentID    string
	UserSessID string
	State      AgentState
	Workspace  string
	CreatedAt  time.Time
	ErrorMsg   string
}

// Simple implementations
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
	log.Printf("%s[%s]%s %s", colorCyan, l.prefix, colorReset, msg)
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
	fmt.Printf("  %s%-20s%s %s%s%s\n", colorYellow, label+":", colorReset, colorWhite, value, colorReset)
}

func printSuccess(message string) {
	fmt.Printf("%s✓%s %s\n", colorGreen, colorReset, message)
}

func printError(message string) {
	fmt.Printf("%s✗%s %s\n", colorRed, colorReset, message)
}

func printAgent(agent *AgentSession) {
	stateColor := colorGreen
	if agent.State == AgentFailed {
		stateColor = colorRed
	} else if agent.State == AgentSpawning {
		stateColor = colorYellow
	}

	fmt.Printf("    %s├─%s Agent: %s%s%s (State: %s%s%s)\n",
		colorCyan, colorReset,
		colorPurple, agent.AgentID, colorReset,
		stateColor, agent.State, colorReset)
}

func waitForUser() {
	fmt.Printf("\n%s[Press Enter to continue]%s ", colorPurple, colorReset)
	fmt.Scanln()
}

const colorWhite = "\033[37m"

func main() {
	ctx := context.Background()

	printHeader("Session Hierarchy Demo - Three-Tier Architecture")
	fmt.Println("This demo shows how UserSessions and AgentSessions build on ContainerSessions.")
	fmt.Println("")
	fmt.Println("Architecture:")
	fmt.Println("  UserSession    (User's workspace container)")
	fmt.Println("    └─> AgentSession(s)  (Individual agents)")
	fmt.Println("          └─> ContainerSession  (Docker backing)")
	waitForUser()

	// Setup Docker
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		printError(fmt.Sprintf("Failed to create Docker client: %v", err))
		os.Exit(1)
	}
	defer dockerClient.Close()

	containerMgr := containersession.NewManager(
		dockerClient,
		&uuidGenerator{},
		&systemClock{},
		&demoLogger{prefix: "ContainerMgr"},
		"./demo-workspaces",
	)

	// Scenario 1: User Session Creation
	if err := runScenario1(ctx, containerMgr); err != nil {
		printError(fmt.Sprintf("Scenario 1 failed: %v", err))
	}

	// Scenario 2: Multi-Agent Coordination
	if err := runScenario2(ctx, containerMgr); err != nil {
		printError(fmt.Sprintf("Scenario 2 failed: %v", err))
	}

	// Scenario 3: Crash Recovery
	if err := runScenario3(ctx, containerMgr); err != nil {
		printError(fmt.Sprintf("Scenario 3 failed: %v", err))
	}

	// Scenario 4: Agent Lifecycle
	if err := runScenario4(ctx, containerMgr); err != nil {
		printError(fmt.Sprintf("Scenario 4 failed: %v", err))
	}

	printHeader("Demo Complete!")
	fmt.Println("Key Takeaways:")
	fmt.Println("  • UserSessions provide user workspace isolation")
	fmt.Println("  • AgentSessions allow multiple agents per user")
	fmt.Println("  • ContainerSessions enable crash recovery and reuse")
	fmt.Println("  • Three-tier architecture enables resilient multi-agent systems")
}

func runScenario1(ctx context.Context, containerMgr *containersession.Manager) error {
	printHeader("Scenario 1: User Session Creation")

	printStep(1, "Create UserSession with backing ContainerSession")

	// Create container session
	containerSess, err := containerMgr.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	if err := containerMgr.StartContainerSession(ctx, containerSess.ID()); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Create user session
	userSess := &UserSession{
		ID:               uuid.New().String(),
		ContainerSession: containerSess,
		Agents:           make(map[string]*AgentSession),
		State:            UserActive,
		CreatedAt:        time.Now(),
	}

	printInfo("UserSession ID", userSess.ID)
	printInfo("State", string(userSess.State))
	printInfo("Container ID", containerSess.ContainerID()[:12])
	printInfo("Container State", string(containerSess.State()))
	printInfo("Workspace", containerSess.WorkspacePath())
	printSuccess("Three-tier session hierarchy established")

	waitForUser()

	printStep(2, "Verify session architecture")
	fmt.Println("")
	fmt.Printf("  %sUserSession%s: %s\n", colorGreen, colorReset, userSess.ID[:8])
	fmt.Printf("    %s└─> ContainerSession%s: %s\n", colorCyan, colorReset, containerSess.ID()[:8])
	fmt.Printf("         %s└─> Docker Container%s: %s\n", colorPurple, colorReset, containerSess.ContainerID()[:12])

	printSuccess("Session hierarchy verified")

	// Cleanup
	fmt.Println("\nCleaning up...")
	containerMgr.StopContainerSession(ctx, containerSess.ID())
	time.Sleep(500 * time.Millisecond)

	waitForUser()
	return nil
}

func runScenario2(ctx context.Context, containerMgr *containersession.Manager) error {
	printHeader("Scenario 2: Multi-Agent Coordination")

	printStep(1, "Create UserSession")

	containerSess, err := containerMgr.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return err
	}

	if err := containerMgr.StartContainerSession(ctx, containerSess.ID()); err != nil {
		return err
	}

	userSess := &UserSession{
		ID:               uuid.New().String(),
		ContainerSession: containerSess,
		Agents:           make(map[string]*AgentSession),
		State:            UserActive,
		CreatedAt:        time.Now(),
	}

	printInfo("UserSession ID", userSess.ID[:8])
	printInfo("Agent Count", "0")
	printSuccess("UserSession created")

	waitForUser()

	printStep(2, "Spawn Agent1: 'coder'")
	agent1 := &AgentSession{
		AgentID:    "coder",
		UserSessID: userSess.ID,
		State:      AgentSpawning,
		Workspace:  containerSess.WorkspacePath(),
		CreatedAt:  time.Now(),
	}
	userSess.Agents[agent1.AgentID] = agent1

	// Simulate spawn process
	time.Sleep(300 * time.Millisecond)
	agent1.State = AgentActive

	printInfo("Agent ID", agent1.AgentID)
	printInfo("State", string(agent1.State))
	printSuccess("Agent1 spawned and active")

	waitForUser()

	printStep(3, "Spawn Agent2: 'reviewer'")
	agent2 := &AgentSession{
		AgentID:    "reviewer",
		UserSessID: userSess.ID,
		State:      AgentSpawning,
		Workspace:  containerSess.WorkspacePath(),
		CreatedAt:  time.Now(),
	}
	userSess.Agents[agent2.AgentID] = agent2

	time.Sleep(300 * time.Millisecond)
	agent2.State = AgentActive

	printInfo("Agent ID", agent2.AgentID)
	printInfo("State", string(agent2.State))
	printSuccess("Agent2 spawned and active")

	waitForUser()

	printStep(4, "Show multi-agent architecture")
	fmt.Println("")
	fmt.Printf("  %sUserSession%s: %s (State: %s)\n",
		colorGreen, colorReset, userSess.ID[:8], userSess.State)
	printAgent(agent1)
	printAgent(agent2)
	fmt.Printf("    %s└─> ContainerSession%s: %s (SHARED)\n",
		colorCyan, colorReset, containerSess.ID()[:8])

	printSuccess("Two agents coordinating in shared container")
	fmt.Println("\n  KEY POINT: Multiple agents, one container - resource efficient!")

	// Cleanup
	fmt.Println("\nCleaning up...")
	containerMgr.StopContainerSession(ctx, containerSess.ID())
	time.Sleep(500 * time.Millisecond)

	waitForUser()
	return nil
}

func runScenario3(ctx context.Context, containerMgr *containersession.Manager) error {
	printHeader("Scenario 3: Crash Recovery with Container Reuse")

	printStep(1, "Create UserSession with Agent")

	containerSess, err := containerMgr.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return err
	}

	if err := containerMgr.StartContainerSession(ctx, containerSess.ID()); err != nil {
		return err
	}

	userSess := &UserSession{
		ID:               uuid.New().String(),
		ContainerSession: containerSess,
		Agents:           make(map[string]*AgentSession),
		State:            UserActive,
		CreatedAt:        time.Now(),
	}

	agent := &AgentSession{
		AgentID:    "analyzer",
		UserSessID: userSess.ID,
		State:      AgentActive,
		Workspace:  containerSess.WorkspacePath(),
		CreatedAt:  time.Now(),
	}
	userSess.Agents[agent.AgentID] = agent

	printInfo("UserSession ID", userSess.ID[:8])
	printInfo("Agent ID", agent.AgentID)
	printInfo("Container ID", containerSess.ContainerID()[:12])
	printSuccess("User session with agent running")

	waitForUser()

	printStep(2, "Simulate relay server crash")
	fmt.Println("  💥 Relay server crashes...")
	fmt.Println("  ⚠️  UserSession state lost from memory...")
	time.Sleep(1 * time.Second)
	printError("Relay server DOWN")

	waitForUser()

	printStep(3, "New relay server starts and reconnects")
	fmt.Println("  🔄 New relay process starting...")
	fmt.Println("  📋 Loading persisted session ID from database...")
	printInfo("Saved Session ID", containerSess.ID()[:8])

	fmt.Println("  🔌 Calling AttachContainerSession()...")
	// Note: We reuse the same containerMgr to simulate a new process
	// In reality, this would be a completely new Manager instance
	recoveredSess, err := containerMgr.AttachContainerSession(ctx, containerSess.ID())
	if err != nil {
		return fmt.Errorf("failed to reattach: %w", err)
	}

	printSuccess("Successfully reattached to existing container!")
	printInfo("Container ID", recoveredSess.ContainerID()[:12])
	printInfo("State", string(recoveredSess.State()))

	waitForUser()

	printStep(4, "Reconstruct UserSession and Agent state")
	// In production, we'd load this from database/NATS
	recoveredUserSess := &UserSession{
		ID:               userSess.ID,
		ContainerSession: recoveredSess,
		Agents:           make(map[string]*AgentSession),
		State:            UserActive,
		CreatedAt:        userSess.CreatedAt,
	}

	recoveredAgent := &AgentSession{
		AgentID:    agent.AgentID,
		UserSessID: recoveredUserSess.ID,
		State:      AgentActive,
		Workspace:  recoveredSess.WorkspacePath(),
		CreatedAt:  agent.CreatedAt,
	}
	recoveredUserSess.Agents[recoveredAgent.AgentID] = recoveredAgent

	printSuccess("UserSession and Agent state reconstructed")
	fmt.Println("")
	fmt.Printf("  %sUserSession%s: %s (State: %sRECOVERED%s)\n",
		colorGreen, colorReset, recoveredUserSess.ID[:8], colorGreen, colorReset)
	printAgent(recoveredAgent)
	fmt.Printf("    %s└─> ContainerSession%s: %s (REUSED)\n",
		colorCyan, colorReset, recoveredSess.ID()[:8])

	fmt.Println("\n  KEY POINT: Container reuse enables crash recovery!")

	// Cleanup
	fmt.Println("\nCleaning up...")
	containerMgr.StopContainerSession(ctx, recoveredSess.ID())
	time.Sleep(500 * time.Millisecond)

	waitForUser()
	return nil
}

func runScenario4(ctx context.Context, containerMgr *containersession.Manager) error {
	printHeader("Scenario 4: Agent Lifecycle Management")

	printStep(1, "Create UserSession with Agent")

	containerSess, err := containerMgr.CreateContainerSession(ctx, "ubuntu:latest", []string{"sleep", "300"})
	if err != nil {
		return err
	}

	if err := containerMgr.StartContainerSession(ctx, containerSess.ID()); err != nil {
		return err
	}

	userSess := &UserSession{
		ID:               uuid.New().String(),
		ContainerSession: containerSess,
		Agents:           make(map[string]*AgentSession),
		State:            UserActive,
		CreatedAt:        time.Now(),
	}

	agent := &AgentSession{
		AgentID:    "debugger",
		UserSessID: userSess.ID,
		State:      AgentActive,
		Workspace:  containerSess.WorkspacePath(),
		CreatedAt:  time.Now(),
	}
	userSess.Agents[agent.AgentID] = agent

	printInfo("UserSession ID", userSess.ID[:8])
	printAgent(agent)
	printSuccess("Agent active")

	waitForUser()

	printStep(2, "Terminate Agent (user decides they don't need it)")
	agent.State = AgentTerminated
	printInfo("Agent State", string(agent.State))
	printInfo("UserSession State", string(userSess.State))
	printSuccess("Agent terminated, UserSession remains ACTIVE")

	waitForUser()

	printStep(3, "Spawn new Agent in same UserSession")
	newAgent := &AgentSession{
		AgentID:    "optimizer",
		UserSessID: userSess.ID,
		State:      AgentSpawning,
		Workspace:  containerSess.WorkspacePath(),
		CreatedAt:  time.Now(),
	}
	userSess.Agents[newAgent.AgentID] = newAgent

	time.Sleep(300 * time.Millisecond)
	newAgent.State = AgentActive

	printSuccess("New agent spawned in existing UserSession")

	waitForUser()

	printStep(4, "Show independent agent lifecycles")
	fmt.Println("")
	fmt.Printf("  %sUserSession%s: %s (State: %s)\n",
		colorGreen, colorReset, userSess.ID[:8], userSess.State)
	fmt.Printf("    %s├─%s Agent: %s%s%s (State: %s%s%s) [OLD]\n",
		colorCyan, colorReset,
		colorPurple, agent.AgentID, colorReset,
		colorRed, agent.State, colorReset)
	fmt.Printf("    %s├─%s Agent: %s%s%s (State: %s%s%s) [NEW]\n",
		colorCyan, colorReset,
		colorPurple, newAgent.AgentID, colorReset,
		colorGreen, newAgent.State, colorReset)
	fmt.Printf("    %s└─> ContainerSession%s: %s (SHARED, PERSISTENT)\n",
		colorCyan, colorReset, containerSess.ID()[:8])

	fmt.Println("\n  KEY POINT: Agents can be created/terminated without affecting container!")

	// Cleanup
	fmt.Println("\nCleaning up...")
	containerMgr.StopContainerSession(ctx, containerSess.ID())
	time.Sleep(500 * time.Millisecond)

	waitForUser()
	return nil
}
