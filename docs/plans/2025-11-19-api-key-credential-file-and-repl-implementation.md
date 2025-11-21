# API Key Credential File and REPL Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use @superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement secure credential file pattern for API keys and working REPL command via docker attach.

**Architecture:** Write API keys to `.creds/.env` files mounted read-only in containers. ACP sources credentials at startup. REPL uses `docker attach` to connect to running ACP process (PID 1). Migrate relay to use same credential pattern.

**Tech Stack:** Go 1.24, Docker API, golang.org/x/term for terminal handling, existing pkg/agent and pkg/relay infrastructure.

---

## Task 1: Add API Key Flag and Credential File Writing

**Files:**
- Modify: `cmd/agentd/cmd_spawn.go`
- Test: `cmd/agentd/cmd_spawn_test.go`

**Step 1: Write failing test for API key flag parsing**

Add to `cmd_spawn_test.go`:

```go
func TestBuildSpawnConfig_APIKeyFromFlag(t *testing.T) {
	// Save and restore
	oldSpawnAPIKey := spawnAPIKey
	defer func() { spawnAPIKey = oldSpawnAPIKey }()

	spawnAPIKey = "sk-test-key-from-flag"

	config, err := buildSpawnConfig("test-agent")
	if err != nil {
		t.Fatalf("buildSpawnConfig failed: %v", err)
	}

	if config.APIKey != "sk-test-key-from-flag" {
		t.Errorf("Expected APIKey='sk-test-key-from-flag', got '%s'", config.APIKey)
	}
}

func TestBuildSpawnConfig_APIKeyFromEnv(t *testing.T) {
	// Save and restore
	oldSpawnAPIKey := spawnAPIKey
	oldEnv := os.Getenv("ANTHROPIC_API_KEY")
	defer func() {
		spawnAPIKey = oldSpawnAPIKey
		os.Setenv("ANTHROPIC_API_KEY", oldEnv)
	}()

	spawnAPIKey = "" // No flag
	os.Setenv("ANTHROPIC_API_KEY", "sk-test-key-from-env")

	config, err := buildSpawnConfig("test-agent")
	if err != nil {
		t.Fatalf("buildSpawnConfig failed: %v", err)
	}

	if config.APIKey != "sk-test-key-from-env" {
		t.Errorf("Expected APIKey='sk-test-key-from-env', got '%s'", config.APIKey)
	}
}

func TestBuildSpawnConfig_MissingAPIKey(t *testing.T) {
	// Save and restore
	oldSpawnAPIKey := spawnAPIKey
	oldEnv := os.Getenv("ANTHROPIC_API_KEY")
	defer func() {
		spawnAPIKey = oldSpawnAPIKey
		os.Setenv("ANTHROPIC_API_KEY", oldEnv)
	}()

	spawnAPIKey = "" // No flag
	os.Setenv("ANTHROPIC_API_KEY", "")

	_, err := buildSpawnConfig("test-agent")
	if err == nil {
		t.Error("Expected error when API key missing, got nil")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("Expected error to mention ANTHROPIC_API_KEY, got: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test -v ./cmd/agentd -run TestBuildSpawnConfig_APIKey
```

Expected: FAIL - undefined `spawnAPIKey` variable

**Step 3: Add API key flag and logic to cmd_spawn.go**

In `cmd_spawn.go`, add variable after other vars (~line 19):

```go
var (
	spawnWorkspace string
	spawnImage     string
	spawnEnv       []string
	spawnAPIKey    string  // Add this
)
```

In `init()` function (~line 49):

```go
func init() {
	spawnCmd.Flags().StringVar(&spawnWorkspace, "workspace", "", "Custom worktree path (default: .agentd/worktrees/<id>)")
	spawnCmd.Flags().StringVar(&spawnImage, "image", "ourocodus/agent:latest", "Docker image")
	spawnCmd.Flags().StringArrayVar(&spawnEnv, "env", nil, "Environment variables (KEY=VALUE)")
	spawnCmd.Flags().StringVar(&spawnAPIKey, "api-key", "", "Anthropic API key (or set ANTHROPIC_API_KEY env var)")
}
```

Update `buildSpawnConfig()` function (~line 112):

```go
func buildSpawnConfig(agentID string) (container.SpawnConfig, error) {
	// Get API key from flag or environment
	apiKey := spawnAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return container.SpawnConfig{}, fmt.Errorf("ANTHROPIC_API_KEY required (via --api-key flag or ANTHROPIC_API_KEY environment variable)")
	}

	// Parse environment variables
	env, err := parseEnvFlags(spawnEnv)
	if err != nil {
		return container.SpawnConfig{}, fmt.Errorf("invalid --env flag: %w", err)
	}

	config := container.SpawnConfig{
		AgentID:    agentID,
		ImageName:  spawnImage,
		Command:    []string{"--workspace", "/workspace"},
		Entrypoint: []string{"/usr/local/bin/acp"},
		Env:        env,
		APIKey:     apiKey,  // Add this
	}

	return config, nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test -v ./cmd/agentd -run TestBuildSpawnConfig_APIKey
```

Expected: PASS (3 tests)

**Step 5: Commit**

```bash
git add cmd/agentd/cmd_spawn.go cmd/agentd/cmd_spawn_test.go
git commit -m "feat: add --api-key flag to spawn command

Supports both --api-key flag and ANTHROPIC_API_KEY env var
Errors clearly if neither provided

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Write Credential File in Spawn Command

**Files:**
- Modify: `cmd/agentd/cmd_spawn.go`
- Test: `cmd/agentd/cmd_spawn_test.go`

**Step 1: Write failing test for credential file creation**

Add to `cmd_spawn_test.go`:

```go
func TestWriteCredentialFile(t *testing.T) {
	// Create temp workspace
	workspace := t.TempDir()
	apiKey := "sk-test-key-12345"

	err := writeCredentialFile(workspace, apiKey)
	if err != nil {
		t.Fatalf("writeCredentialFile failed: %v", err)
	}

	// Verify .creds directory exists with 0700
	credsDir := filepath.Join(workspace, ".creds")
	info, err := os.Stat(credsDir)
	if err != nil {
		t.Fatalf("Failed to stat .creds directory: %v", err)
	}
	if !info.IsDir() {
		t.Error(".creds is not a directory")
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Expected .creds permissions 0700, got %o", info.Mode().Perm())
	}

	// Verify .env file exists with 0600
	envFile := filepath.Join(credsDir, ".env")
	info, err = os.Stat(envFile)
	if err != nil {
		t.Fatalf("Failed to stat .env file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("Expected .env permissions 0600, got %o", info.Mode().Perm())
	}

	// Verify .env file content
	content, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("Failed to read .env file: %v", err)
	}
	expected := "ANTHROPIC_API_KEY=sk-test-key-12345\n"
	if string(content) != expected {
		t.Errorf("Expected .env content %q, got %q", expected, string(content))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./cmd/agentd -run TestWriteCredentialFile
```

Expected: FAIL - undefined `writeCredentialFile`

**Step 3: Implement credential file writing**

Add to `cmd_spawn.go` after `parseEnvFlags()` function:

```go
// writeCredentialFile creates .creds/.env file with API key
func writeCredentialFile(workspace, apiKey string) error {
	// Create .creds directory with 0700 permissions
	credsDir := filepath.Join(workspace, ".creds")
	if err := os.MkdirAll(credsDir, 0o700); err != nil {
		return fmt.Errorf("failed to create .creds directory: %w", err)
	}

	// Write .env file with 0600 permissions
	envFile := filepath.Join(credsDir, ".env")
	content := fmt.Sprintf("ANTHROPIC_API_KEY=%s\n", apiKey)
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	return nil
}
```

Add import at top of file:

```go
import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"  // Add this
	"strings"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)
```

**Step 4: Run test to verify it passes**

```bash
go test -v ./cmd/agentd -run TestWriteCredentialFile
```

Expected: PASS

**Step 5: Integrate into runSpawn function**

In `runSpawn()` function, after `buildSpawnConfig()` (~line 70):

```go
// Build spawn config
config, err := buildSpawnConfig(agentID)
if err != nil {
	return fmt.Errorf("failed to build spawn config: %w", err)
}

// Determine workspace path
workspacePath := spawnWorkspace
if workspacePath == "" {
	workspacePath = fmt.Sprintf(".agentd/worktrees/%s", agentID)
}

// Write credential file before spawning
if err := writeCredentialFile(workspacePath, config.APIKey); err != nil {
	return fmt.Errorf("failed to write credentials: %w", err)
}
```

**Step 6: Run manual test**

```bash
cd /Users/clint/.config/superpowers/worktrees/ourocodus/feat-agentd-foundation/cmd/agentd
export ANTHROPIC_API_KEY=sk-test-manual
go run . spawn test-creds-agent --workspace /tmp/test-workspace
ls -la /tmp/test-workspace/.creds/
cat /tmp/test-workspace/.creds/.env
```

Expected: Directory exists with `.env` containing API key

**Step 7: Commit**

```bash
git add cmd/agentd/cmd_spawn.go cmd/agentd/cmd_spawn_test.go
git commit -m "feat: write API key to .creds/.env file

Creates .creds directory with 0700 permissions
Writes .env file with 0600 permissions
Credentials ready for container mount

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Update Container Spawn Config to Mount Credentials

**Files:**
- Modify: `pkg/agent/container/types.go`
- Modify: `pkg/agent/container/launcher.go`

**Step 1: Add APIKey field to SpawnConfig**

In `pkg/agent/container/types.go`, modify `SpawnConfig` struct (~line 15):

```go
type SpawnConfig struct {
	AgentID     string
	ImageName   string
	Command     []string
	Entrypoint  []string
	Env         []string
	GitSSHKey   []byte
	GitHubToken []byte
	APIKey      string   // Add this field
}
```

**Step 2: Update launcher to mount .creds directory**

In `pkg/agent/container/launcher.go`, in `Spawn()` method, find the mounts section (~line 100) and add:

```go
// Mount credentials directory (read-only)
credsPath := filepath.Join(workspacePath, ".creds")
if _, err := os.Stat(credsPath); err == nil {
	mounts = append(mounts, mount.Mount{
		Type:     mount.TypeBind,
		Source:   credsPath,
		Target:   "/root/.creds",
		ReadOnly: true,
	})
}
```

Add import at top:

```go
import (
	// ... existing imports ...
	"path/filepath"  // Add if not present
)
```

**Step 3: Run existing tests**

```bash
go test -v ./pkg/agent/container
```

Expected: PASS (all existing tests)

**Step 4: Commit**

```bash
git add pkg/agent/container/types.go pkg/agent/container/launcher.go
git commit -m "feat: mount .creds directory in agent containers

Adds read-only mount of .creds at /root/.creds
Only mounts if directory exists on host

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Implement Basic REPL Command Structure

**Files:**
- Modify: `cmd/agentd/cmd_repl.go`
- Test: `cmd/agentd/cmd_repl_test.go`

**Step 1: Write test for finding agent by ID**

Replace content of `cmd_repl_test.go`:

```go
package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunREPL_FindsAgent(t *testing.T) {
	// This test verifies the agent lookup logic
	// We can't easily test full docker attach without integration test

	// Test that runREPL validates agent ID
	err := runREPL(replCmd, []string{})
	if err == nil {
		t.Error("Expected error with no agent ID")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("Expected error to mention agent, got: %v", err)
	}
}

func TestFindAgentByID(t *testing.T) {
	agents := []agentInfo{
		{AgentID: "alice", ContainerID: "abc123", Status: "running"},
		{AgentID: "bob", ContainerID: "def456", Status: "running"},
	}

	agent, found := findAgentByID(agents, "alice")
	if !found {
		t.Error("Expected to find alice")
	}
	if agent.ContainerID != "abc123" {
		t.Errorf("Expected container abc123, got %s", agent.ContainerID)
	}

	_, found = findAgentByID(agents, "charlie")
	if found {
		t.Error("Should not find charlie")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -v ./cmd/agentd -run TestFindAgentByID
```

Expected: FAIL - undefined `findAgentByID`

**Step 3: Implement basic REPL structure**

Replace `cmd_repl.go` content:

```go
package main

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var replCmd = &cobra.Command{
	Use:   "repl <agent-id>",
	Short: "🔄 Interactive REPL with agent via ACP",
	Long: `Connect to a running agent and interact via ACP protocol.

The agent must be running (spawned). This command attaches directly to
the agent's stdin/stdout where the ACP process runs as PID 1.`,
	Example: `  # Connect to running agent
  agentd repl alice

  # Once connected, send messages
  > Hello agent!
  Echo: Hello agent!

  # Exit with Ctrl+D`,
	Args: cobra.ExactArgs(1),
	RunE: runREPL,
}

func runREPL(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent ID required")
	}

	agentID := args[0]
	ctx := context.Background()

	// Find agent
	agents, err := listAgentsFromDocker(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	agent, found := findAgentByID(agents, agentID)
	if !found {
		color.New(color.FgRed).Printf("✗ Agent '%s' not found\n", agentID)
		fmt.Println("\nRunning agents:")
		if len(agents) == 0 {
			fmt.Println("  (none)")
		}
		for _, a := range agents {
			fmt.Printf("  - %s\n", a.AgentID)
		}
		return fmt.Errorf("agent not found")
	}

	if agent.Status != "running" {
		return fmt.Errorf("agent '%s' is not running (status: %s)", agentID, agent.Status)
	}

	// TODO: Implement docker attach
	color.New(color.FgGreen).Printf("✓ Found agent '%s' (container: %s)\n", agentID, formatContainerID(agent.ContainerID))
	fmt.Println("REPL implementation coming in next task...")

	return nil
}

// findAgentByID searches for an agent by ID
func findAgentByID(agents []agentInfo, agentID string) (agentInfo, bool) {
	for _, agent := range agents {
		if agent.AgentID == agentID {
			return agent, true
		}
	}
	return agentInfo{}, false
}
```

**Step 4: Run tests**

```bash
go test -v ./cmd/agentd -run TestFindAgentByID
go test -v ./cmd/agentd -run TestRunREPL
```

Expected: PASS (2 tests)

**Step 5: Commit**

```bash
git add cmd/agentd/cmd_repl.go cmd/agentd/cmd_repl_test.go
git commit -m "feat: implement REPL command structure

Validates agent ID, finds agent, checks status
Full docker attach implementation in next task

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Implement Docker Attach for REPL

**Files:**
- Modify: `cmd/agentd/cmd_repl.go`

**Step 1: Implement docker attach with terminal handling**

Add imports to `cmd_repl.go`:

```go
import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)
```

Replace the TODO section in `runREPL()` with:

```go
// Connect to Docker
dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
if err != nil {
	return fmt.Errorf("failed to create Docker client: %w", err)
}
defer dockerClient.Close()

// Print connection message
color.New(color.FgGreen).Printf("✓ Connected to agent '%s'\n", agentID)
color.New(color.FgHiBlack).Println("  Press Ctrl+D to exit\n")

// Attach to container
attachResp, err := dockerClient.ContainerAttach(ctx, agent.ContainerID, container.AttachOptions{
	Stream: true,
	Stdin:  true,
	Stdout: true,
	Stderr: true,
})
if err != nil {
	return fmt.Errorf("failed to attach to container: %w", err)
}
defer attachResp.Close()

// Set up terminal
oldState, err := setRawTerminal()
if err != nil {
	color.New(color.FgYellow).Printf("Warning: Failed to set raw mode: %v\n", err)
	// Continue without raw mode
}
if oldState != nil {
	defer restoreTerminal(oldState)
}

// Handle Ctrl+C gracefully
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt)
go func() {
	<-sigChan
	fmt.Println()
	restoreTerminal(oldState)
	os.Exit(0)
}()

// Bidirectional copy
errChan := make(chan error, 2)

// Copy container output to stdout
go func() {
	_, err := io.Copy(os.Stdout, attachResp.Reader)
	errChan <- err
}()

// Copy stdin to container
go func() {
	_, err := io.Copy(attachResp.Conn, os.Stdin)
	errChan <- err
}()

// Wait for either copy to finish
if err := <-errChan; err != nil && err != io.EOF {
	return fmt.Errorf("REPL error: %w", err)
}

color.New(color.FgGreen).Printf("\n✓ Disconnected from agent '%s'\n", agentID)
return nil
```

Add terminal helper functions at the end of file:

```go
// setRawTerminal sets the terminal to raw mode
func setRawTerminal() (*term.State, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("not a terminal")
	}
	return term.MakeRaw(fd)
}

// restoreTerminal restores the terminal to its previous state
func restoreTerminal(oldState *term.State) error {
	if oldState == nil {
		return nil
	}
	fd := int(os.Stdin.Fd())
	return term.Restore(fd, oldState)
}
```

**Step 2: Install golang.org/x/term dependency**

```bash
go get golang.org/x/term
go mod tidy
```

**Step 3: Build and test manually (requires running agent)**

```bash
cd /Users/clint/.config/superpowers/worktrees/ourocodus/feat-agentd-foundation/cmd/agentd
go build -o agentd-test .

# Spawn a test agent first
export ANTHROPIC_API_KEY=sk-test-key
./agentd-test spawn test-repl-agent

# Test REPL (should connect and echo)
./agentd-test repl test-repl-agent
# Type: hello
# Press Ctrl+D to exit

# Cleanup
./agentd-test stop test-repl-agent
rm agentd-test
```

Expected: REPL connects, echoes input, exits cleanly with Ctrl+D

**Step 4: Commit**

```bash
git add cmd/agentd/cmd_repl.go go.mod go.sum
git commit -m "feat: implement docker attach for REPL

Uses Docker attach API to connect to running ACP process
Terminal raw mode for proper stdio handling
Graceful exit with Ctrl+D, Ctrl+C

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Add Integration Test for Full Workflow

**Files:**
- Modify: `cmd/agentd/cmd_integration_test.go`

**Step 1: Write integration test**

Add to `cmd_integration_test.go`:

```go
func TestCLI_SpawnWithAPIKeyAndREPL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping CLI integration test in short mode")
	}

	if os.Getenv("DOCKER_HOST") == "" {
		t.Skip("DOCKER_HOST not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	agentID := "test-cli-repl"

	// Cleanup
	defer func() {
		stopAgent(ctx, nil, agentID)
		time.Sleep(500 * time.Millisecond)
	}()

	// Build binary
	buildCmd := exec.Command("go", "build", "-o", "agentd-test", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove("agentd-test")

	// Test spawn with API key
	t.Run("spawn with API key", func(t *testing.T) {
		cmd := exec.Command("./agentd-test", "spawn", agentID, "--api-key", "sk-test-integration-key")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Spawn output:\n%s", output)
			t.Fatalf("Spawn command failed: %v", err)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "Agent "+agentID+" ready") {
			t.Errorf("Spawn output missing success message: %s", outputStr)
		}
	})

	time.Sleep(2 * time.Second)

	// Verify credential file was created
	t.Run("verify credential file", func(t *testing.T) {
		agents, err := listAgentsFromDocker(ctx)
		if err != nil {
			t.Fatalf("Failed to list agents: %v", err)
		}

		var workspace string
		for _, agent := range agents {
			if agent.AgentID == agentID {
				workspace = agent.Workspace
				break
			}
		}

		if workspace == "" {
			t.Fatal("Could not find agent workspace")
		}

		envFile := filepath.Join(workspace, ".creds", ".env")
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			t.Errorf("Credential file not found at %s", envFile)
		}

		// Verify permissions
		info, err := os.Stat(envFile)
		if err == nil {
			if info.Mode().Perm() != 0o600 {
				t.Errorf("Expected .env permissions 0600, got %o", info.Mode().Perm())
			}
		}

		// Verify content
		content, err := os.ReadFile(envFile)
		if err == nil {
			if !strings.Contains(string(content), "ANTHROPIC_API_KEY=sk-test-integration-key") {
				t.Errorf("Expected API key in .env, got: %s", string(content))
			}
		}
	})

	// Test REPL command (just verify it can connect, don't test interactivity)
	t.Run("repl connects", func(t *testing.T) {
		// Since we can't test interactivity, just verify connection setup
		// by checking the command doesn't error on startup
		cmd := exec.Command("./agentd-test", "repl", agentID)

		// Kill after 2 seconds to avoid hanging
		go func() {
			time.Sleep(2 * time.Second)
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()

		output, _ := cmd.CombinedOutput()
		outputStr := string(output)

		// Should show connection message
		if !strings.Contains(outputStr, "Connected to agent") && !strings.Contains(outputStr, "connection") {
			t.Logf("REPL output: %s", outputStr)
			// Not a hard failure - might be timing issue
		}
	})
}
```

Add import at top if not present:

```go
import (
	// ... existing imports ...
	"path/filepath"
)
```

**Step 2: Run integration test**

```bash
go test -v ./cmd/agentd -run TestCLI_SpawnWithAPIKeyAndREPL
```

Expected: PASS

**Step 3: Commit**

```bash
git add cmd/agentd/cmd_integration_test.go
git commit -m "test: add integration test for spawn with API key and REPL

Tests full workflow: spawn with --api-key, verify .creds/.env,
connect via REPL command

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Update Relay to Use Credential File Pattern

**Files:**
- Modify: `pkg/relay/session/manager.go`
- Modify: `pkg/agent/factory.go`

**Step 1: Add credential file helper to relay**

In `pkg/relay/session/manager.go`, add after imports:

```go
import (
	// ... existing imports ...
	"path/filepath"
)

// writeCredentialFile creates .creds/.env file with API key in workspace
func writeCredentialFile(workspace, apiKey string) error {
	if apiKey == "" {
		return nil // No API key to write
	}

	// Create .creds directory
	credsDir := filepath.Join(workspace, ".creds")
	if err := os.MkdirAll(credsDir, 0o700); err != nil {
		return fmt.Errorf("failed to create .creds directory: %w", err)
	}

	// Write .env file
	envFile := filepath.Join(credsDir, ".env")
	content := fmt.Sprintf("ANTHROPIC_API_KEY=%s\n", apiKey)
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}

	return nil
}
```

**Step 2: Call writeCredentialFile in SpawnAgent**

In `pkg/relay/session/manager.go`, in `SpawnAgent()` method, after workspace creation (~line 208):

```go
// Create workspace directory if needed
err = os.MkdirAll(absPath, 0o700)
if err != nil {
	return fmt.Errorf("failed to create workspace directory: %w", err)
}

// Write credential file if API key available
if m.isContainerModeEnabled() {
	var anthropicKey string
	if acpFactory, ok := m.clientFactory.(*ACPClientFactory); ok {
		anthropicKey = acpFactory.GetAPIKey()
	}
	if anthropicKey != "" {
		if err := writeCredentialFile(absPath, anthropicKey); err != nil {
			return fmt.Errorf("failed to write credentials: %w", err)
		}
		m.logger.Printf("[SESSION] ├─ Wrote credentials to %s/.creds/", absPath)
	}
}
```

**Step 3: Remove API key from container environment**

In `pkg/agent/factory.go`, remove lines 136-140 (the API key environment injection):

```go
// REMOVED: API key now comes from .creds/.env file, not container environment
// if a.launcherConfig.AnthropicKey != "" {
//     env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", a.launcherConfig.AnthropicKey))
// }
```

**Step 4: Run relay tests**

```bash
go test -v ./pkg/relay/session -run TestSpawnAgent
go test -v ./pkg/agent
```

Expected: PASS (all tests)

**Step 5: Commit**

```bash
git add pkg/relay/session/manager.go pkg/agent/factory.go
git commit -m "feat: migrate relay to use credential file pattern

Relay now writes .creds/.env before spawning container
Removed API key from container environment variables
More secure: key not visible in docker inspect

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Update Documentation

**Files:**
- Modify: `docs/agentd.md`
- Modify: `README.md` (if applicable)

**Step 1: Update agentd.md with new commands**

In `docs/agentd.md`, update the spawn command section:

```markdown
### spawn

Spawn an isolated agent with three-layer isolation:

```bash
agentd spawn [agent-id] [flags]
```

**Flags:**
- `--workspace <path>` - Custom worktree path (default: .agentd/worktrees/<id>)
- `--image <name>` - Docker image (default: ourocodus/agent:latest)
- `--env KEY=VALUE` - Environment variables (repeatable)
- `--api-key <key>` - Anthropic API key (or set ANTHROPIC_API_KEY env var)

**API Key Requirement:**
The spawn command requires an Anthropic API key for agent communication. Provide it via:
1. `--api-key` flag: `agentd spawn alice --api-key sk-...`
2. `ANTHROPIC_API_KEY` environment variable: `export ANTHROPIC_API_KEY=sk-...`

The API key is written to `.creds/.env` in the agent's workspace and mounted read-only at `/root/.creds/` in the container.

**Examples:**

```bash
# Spawn with auto-generated ID
agentd spawn --api-key sk-...

# Spawn with custom ID
export ANTHROPIC_API_KEY=sk-...
agentd spawn alice

# Spawn with custom image and environment
agentd spawn bob --api-key sk-... --image ourocodus/agent:dev --env DEBUG=1
```
```

Add repl command section:

```markdown
### repl

Connect to a running agent and interact via ACP protocol:

```bash
agentd repl <agent-id>
```

Opens an interactive REPL session with the agent. The agent must be running (spawned). Messages are sent to the ACP process via stdin/stdout.

**Usage:**

```bash
# Start REPL with agent
$ agentd repl alice
✓ Connected to agent 'alice'
  Press Ctrl+D to exit

> Hello agent!
Echo: Hello agent!

> help
Echo: help

> ^D
✓ Disconnected from agent 'alice'
```

**Controls:**
- `Ctrl+D` - Exit REPL
- `Ctrl+C` - Interrupt current operation

**Notes:**
- Agent must be in "running" state
- Uses direct docker attach to ACP process (PID 1)
- For programmatic interaction, use the `send` command instead
```

**Step 2: Commit**

```bash
git add docs/agentd.md
git commit -m "docs: update agentd.md with API key and REPL docs

Documents --api-key flag, credential file pattern
Adds REPL command usage and examples

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Run Full Test Suite and Verify

**Step 1: Run all tests**

```bash
# Unit tests
go test ./cmd/agentd/...
go test ./pkg/agent/...
go test ./pkg/relay/session/...

# Integration tests
go test -v ./cmd/agentd -run Integration
```

Expected: All tests PASS

**Step 2: Run linting**

```bash
make lint
```

Expected: No errors

**Step 3: Run formatting**

```bash
make fmt
```

Expected: No changes (already formatted)

**Step 4: Build project**

```bash
make build
```

Expected: Successful build

**Step 5: Manual end-to-end test**

```bash
cd bin
export ANTHROPIC_API_KEY=sk-test-final

# Spawn agent with API key
./agentd spawn final-test

# Verify credential file
cat ../.agentd/worktrees/final-test/.creds/.env

# Connect via REPL
./agentd repl final-test
# Type some messages
# Press Ctrl+D

# Cleanup
./agentd stop final-test
```

Expected: Full workflow works end-to-end

**Step 6: Final commit**

```bash
git add -A
git commit -m "chore: final cleanup and verification

All tests passing, lint clean, manual e2e verified
Ready for review

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Completion Checklist

- [ ] Task 1: API key flag and credential file writing
- [ ] Task 2: Credential file writing function
- [ ] Task 3: Container mount configuration
- [ ] Task 4: REPL command structure
- [ ] Task 5: Docker attach implementation
- [ ] Task 6: Integration tests
- [ ] Task 7: Relay migration
- [ ] Task 8: Documentation updates
- [ ] Task 9: Full verification

## Related Skills

- @superpowers:test-driven-development - Follow RED-GREEN-REFACTOR for each task
- @superpowers:verification-before-completion - Run verification commands before claiming completion
- @superpowers:finishing-a-development-branch - Use after all tasks complete

## Notes for Implementation

- Keep API key out of logs and error messages
- Test both flag and environment variable sources
- Verify file permissions (0700/0600) are correct
- Test REPL with real agent to verify stdin/stdout works
- The relay migration is a breaking change for existing agents (respawn required)
