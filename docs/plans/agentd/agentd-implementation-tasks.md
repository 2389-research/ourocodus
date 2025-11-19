# agentd MVP - Implementation Task Breakdown

**Sprint Duration**: 5 days (Monday → Friday demo)
**Design Document**: [agentd-mvp.md](./agentd-mvp.md)
**Target**: MVP demo-ready by Friday

## Day 1 (Monday): Foundation - 8 hours

### Task 1.1: Project Setup (1 hour)

**Create directory structure**:
```bash
mkdir -p cmd/agentd
touch cmd/agentd/main.go
touch cmd/agentd/labels.go
touch cmd/agentd/cmd_doctor.go
```

**Add dependencies**:
```bash
go get github.com/spf13/cobra@latest
```

**Acceptance Criteria**:
- [ ] Directory structure created
- [ ] Cobra dependency added to go.mod
- [ ] Basic main.go compiles

---

### Task 1.2: Label Schema Constants (30 min)

**File**: `cmd/agentd/labels.go`

**Implementation**:
```go
package main

const (
    // LabelNamespace identifies agentd-managed containers
    LabelNamespace = "org.ourocodus.agentd"

    // LabelAgentID stores the agent identifier
    LabelAgentID = "agentd.id"

    // LabelRepoHash stores the repository hash
    LabelRepoHash = "agentd.repo"

    // LabelWorktreePath stores the worktree path
    LabelWorktreePath = "agentd.worktree"

    // LabelVersion stores the agentd version
    LabelVersion = "agentd.version"

    // Version is the current agentd version
    Version = "0.1.0"
)

// BuildLabels creates the label map for a container
func BuildLabels(agentID, repoHash, worktreePath string) map[string]string {
    return map[string]string{
        LabelNamespace:    "true",
        LabelAgentID:      agentID,
        LabelRepoHash:     repoHash,
        LabelWorktreePath: worktreePath,
        LabelVersion:      Version,
    }
}
```

**Acceptance Criteria**:
- [ ] All label constants defined
- [ ] BuildLabels() helper function implemented
- [ ] File compiles without errors

---

### Task 1.3: Cobra CLI Scaffold (1 hour)

**File**: `cmd/agentd/main.go`

**Implementation**:
```go
package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "agentd",
    Short: "agentd - Multi-agent isolation orchestrator",
    Long: `agentd demonstrates Ourocodus's three-layer isolation architecture:
    - Git worktrees isolate code
    - Docker containers isolate processes
    - Credential volumes isolate access

Multiple agents work concurrently without conflicts.`,
    Version: Version,
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func init() {
    rootCmd.AddCommand(doctorCmd)
    // Future commands will be added here
}
```

**Acceptance Criteria**:
- [ ] Root command defined with help text
- [ ] Version flag works (`agentd --version`)
- [ ] Help text displays (`agentd --help`)
- [ ] Binary compiles: `go build -o bin/agentd ./cmd/agentd`

---

### Task 1.4: Validate pkg/ API (2 hours)

**Goal**: Verify AgentContainerLauncher API meets our needs

**File**: `cmd/agentd/pkg_validation.go` (temporary test file)

**Validation Checklist**:
```go
// Read pkg/agent/container/launcher.go and verify:

// 1. Check Spawn() signature
func (l *AgentContainerLauncher) Spawn(ctx context.Context, config SpawnConfig) (*AgentContainerHandle, error)
// ✓ Returns handle we can use
// ✓ Takes SpawnConfig we can populate

// 2. Check SpawnConfig fields
type SpawnConfig struct {
    AgentID      string
    ImageName    string
    Command      []string
    Entrypoint   []string
    Env          map[string]string
    GitSSHKey    string
    GitHubToken  string
}
// ✓ All fields we need are present

// 3. Check Handle methods
type AgentContainerHandle struct {
    // Methods we need:
    // - ContainerID() string
    // - WorktreePath() string
    // - Status information
}

// 4. Check Stop() signature
func (l *AgentContainerLauncher) Stop(ctx context.Context, agentID string) error
// ✓ Cleanup is handled

// 5. Check GetHandle() signature
func (l *AgentContainerLauncher) GetHandle(agentID string) *AgentContainerHandle
// ✓ Can retrieve existing handles

// 6. Check ListHandles() signature
func (l *AgentContainerLauncher) ListHandles() []*AgentContainerHandle
// ✓ Can list all agents
```

**Action Items**:
- [ ] Read `pkg/agent/container/launcher.go:1-312`
- [ ] Verify all methods we need exist
- [ ] Identify any missing functionality (adapter layer needed?)
- [ ] Document any API mismatches

**Acceptance Criteria**:
- [ ] API validation complete
- [ ] No showstopper mismatches found (or adapter layer identified)
- [ ] Documented how to wire AgentContainerLauncher

---

### Task 1.5: Doctor Command - Core Checks (3.5 hours)

**File**: `cmd/agentd/cmd_doctor.go`

**Implementation Structure**:
```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
    Use:   "doctor",
    Short: "Validate environment for running agents",
    Long:  "Checks Docker, git, disk space, and performs spawn smoke test",
    RunE:  runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    checks := []Check{
        checkDockerDaemon,
        checkDockerVersion,
        checkFileSharingMacOS,
        checkImagePresence,
        checkGitWorktreeSupport,
        checkDiskSpace,
        checkSpawnSmokeTest,
    }

    allPassed := true
    for _, check := range checks {
        if err := check(ctx); err != nil {
            fmt.Fprintf(os.Stderr, "✗ %s\n", err)
            allPassed = false
        }
    }

    if allPassed {
        fmt.Println("Environment ready!")
        return nil
    }

    return fmt.Errorf("environment validation failed")
}

type Check func(context.Context) error

// Implement each check function:

func checkDockerDaemon(ctx context.Context) error {
    // Use Docker SDK to ping daemon
    // Return error with actionable message if fails
}

func checkDockerVersion(ctx context.Context) error {
    // Get Docker version
    // Ensure >= 20.10 (required for features we use)
}

func checkFileSharingMacOS(ctx context.Context) error {
    // On macOS, check Docker Desktop file sharing settings
    // Verify current directory is in shared paths
}

func checkImagePresence(ctx context.Context) error {
    // Check if ourocodus/agent:latest exists
    // Offer to pull if missing
}

func checkGitWorktreeSupport(ctx context.Context) error {
    // Run: git worktree list
    // Verify git supports worktrees (git >= 2.5)
}

func checkDiskSpace(ctx context.Context) error {
    // Check available disk space
    // Ensure >= 1GB free
}

func checkSpawnSmokeTest(ctx context.Context) error {
    // Actually spawn a test container
    // Verify it starts and stops cleanly
    // This catches issues with mounts, permissions, etc.
}
```

**Acceptance Criteria**:
- [ ] All 7 checks implemented
- [ ] Each check prints clear success/failure message
- [ ] Actionable error messages (e.g., "Docker daemon not running. Start Docker Desktop and retry.")
- [ ] Spawn smoke test actually creates/destroys container
- [ ] `agentd doctor` exits with code 0 on success, 1 on failure

**Test Manually**:
```bash
# With Docker running
$ agentd doctor
✓ Docker daemon running (v27.4.1)
✓ Docker version supported (>= 20.10)
✓ File sharing enabled: /Users/clint/code
✓ Image present: ourocodus/agent:latest
✓ Git worktree support confirmed
✓ Disk space: 5.2GB available
✓ Spawn smoke test passed
Environment ready!

# With Docker stopped
$ agentd doctor
✗ Docker daemon not running. Start Docker Desktop and retry.
✗ Image check skipped (Docker unavailable)
...
environment validation failed
```

---

## Day 2 (Tuesday): Spawn Command - 8 hours

### Task 2.1: Spawn Command Structure (1 hour)

**File**: `cmd/agentd/cmd_spawn.go`

**Implementation Skeleton**:
```go
package main

import (
    "context"
    "fmt"

    "github.com/spf13/cobra"
    "github.com/2389-research/ourocodus/pkg/agent/container"
)

var (
    spawnWorkspace string
    spawnImage     string
    spawnEnv       []string
)

var spawnCmd = &cobra.Command{
    Use:   "spawn <agent-id>",
    Short: "Spawn an isolated agent",
    Args:  cobra.MaximumNArgs(1),
    RunE:  runSpawn,
}

func init() {
    spawnCmd.Flags().StringVar(&spawnWorkspace, "workspace", "", "Custom worktree path")
    spawnCmd.Flags().StringVar(&spawnImage, "image", "ourocodus/agent:latest", "Docker image")
    spawnCmd.Flags().StringArrayVar(&spawnEnv, "env", nil, "Environment variables (KEY=VALUE)")

    rootCmd.AddCommand(spawnCmd)
}

func runSpawn(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    // Get or generate agent ID
    agentID := generateAgentID(args)

    // Create launcher (wiring pkg/ components)
    launcher := createLauncher()

    // Build spawn config
    config := buildSpawnConfig(agentID)

    // Spawn agent
    handle, err := launcher.Spawn(ctx, config)
    if err != nil {
        return fmt.Errorf("spawn failed: %w", err)
    }

    // Print success
    printSpawnSuccess(handle)

    return nil
}
```

**Acceptance Criteria**:
- [ ] Command structure defined
- [ ] Flags wired up
- [ ] Compiles (even with TODO stubs)

---

### Task 2.2: Wire AgentContainerLauncher (2 hours)

**Implementation**:
```go
func createLauncher() *container.AgentContainerLauncher {
    // Instantiate dependencies
    containerMgr := createContainerManager()
    worktreeMgr := createWorktreeManager()
    credMounter := createCredentialMounter()

    // Create launcher
    launcher := container.NewAgentContainerLauncher(
        containerMgr,
        worktreeMgr,
        credMounter,
        ".agentd/worktrees", // baseDir
    )

    return launcher
}

func createContainerManager() *containersession.Manager {
    // Use existing package
    // Refer to how relay creates ContainerSession.Manager
}

func createWorktreeManager() *worktree.AgentWorktreeManager {
    // Use existing package
    // Refer to existing usage in relay
}

func createCredentialMounter() *container.AgentCredentialMounter {
    // Use existing package
}
```

**Acceptance Criteria**:
- [ ] All pkg/ components wired correctly
- [ ] No compilation errors
- [ ] Launcher instantiates successfully

---

### Task 2.3: Spawn Config Builder (1 hour)

**Implementation**:
```go
func generateAgentID(args []string) string {
    if len(args) > 0 {
        return args[0]
    }

    // Generate: agent-<shortid>
    return fmt.Sprintf("agent-%s", generateShortID())
}

func generateShortID() string {
    // 6-character random alphanumeric
    // e.g., "a1b2c3"
}

func buildSpawnConfig(agentID string) container.SpawnConfig {
    return container.SpawnConfig{
        AgentID:    agentID,
        ImageName:  spawnImage,
        Command:    []string{"server"},
        Entrypoint: []string{"/usr/local/bin/claude-code-acp"},
        Env:        parseEnvFlags(spawnEnv),
        // GitSSHKey and GitHubToken handled by credential mounter
    }
}

func parseEnvFlags(envFlags []string) map[string]string {
    env := make(map[string]string)
    for _, flag := range envFlags {
        // Parse KEY=VALUE format
    }
    return env
}
```

**Acceptance Criteria**:
- [ ] Agent ID generation works
- [ ] SpawnConfig builds correctly
- [ ] Env flags parse correctly

---

### Task 2.4: Spawn Execution & Output (2 hours)

**Implementation**:
```go
func printSpawnSuccess(handle *container.AgentContainerHandle) {
    fmt.Printf("✓ Created worktree: %s (branch: %s)\n",
        handle.WorktreePath(),
        handle.Branch())

    fmt.Printf("✓ Container started: %s\n",
        handle.ContainerID())

    if handle.HasCredentials() {
        fmt.Printf("✓ Credentials: mounted at /root/.creds (read-only)\n")
    }

    fmt.Printf("✓ Agent ready: %s\n", handle.AgentID())
}
```

**Error Handling**:
```go
// In runSpawn():
handle, err := launcher.Spawn(ctx, config)
if err != nil {
    // Check error type for actionable messages
    if errors.Is(err, container.ErrAgentAlreadyExists) {
        return fmt.Errorf("agent '%s' already exists. Use 'agentd list' to see active agents", agentID)
    }
    if errors.Is(err, container.ErrWorktreeSetupFailed) {
        return fmt.Errorf("worktree creation failed: %w\nEnsure git repository is clean", err)
    }
    return fmt.Errorf("spawn failed: %w", err)
}
```

**Acceptance Criteria**:
- [ ] Success output matches design spec
- [ ] Error messages are actionable
- [ ] Cleanup happens on failure

---

### Task 2.5: macOS Docker Mount Testing (2 hours)

**Manual Testing**:
```bash
# Test spawn with various workspace paths
$ agentd spawn alice
$ agentd spawn bob --workspace /tmp/bob-workspace

# Verify mounts work
$ docker exec agentd-a1b2c3 ls -la /workspace
$ docker exec agentd-a1b2c3 cat /root/.creds/config

# Test credential isolation
$ docker exec agentd-a1b2c3 env | grep ANTHROPIC
# Should NOT see host env vars
```

**Debug Issues**:
- File sharing permissions
- Volume mount paths
- Credential mount read-only enforcement

**Acceptance Criteria**:
- [ ] Workspaces mount correctly on macOS
- [ ] Credentials mount read-only
- [ ] No permission errors
- [ ] Containers can read/write workspace

---

## Day 3 (Wednesday): List & Stop - 6 hours

### Task 3.1: List Command (2 hours)

**File**: `cmd/agentd/cmd_list.go`

**Implementation**:
```go
package main

import (
    "context"
    "fmt"
    "os"
    "text/tabwriter"
    "time"

    "github.com/spf13/cobra"
)

var listFormat string

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List all active agents",
    RunE:  runList,
}

func init() {
    listCmd.Flags().StringVar(&listFormat, "format", "table", "Output format (table|json)")
    rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    // Create launcher
    launcher := createLauncher()

    // Get all handles
    handles := launcher.ListHandles()

    if len(handles) == 0 {
        fmt.Println("No agents running.")
        return nil
    }

    // Print based on format
    if listFormat == "json" {
        return printListJSON(handles)
    }

    return printListTable(handles)
}

func printListTable(handles []*container.AgentContainerHandle) error {
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "AGENT\tSTATUS\tWORKSPACE\tCONTAINER\tCREATED")

    for _, handle := range handles {
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
            handle.AgentID(),
            handle.Status(),
            handle.WorktreePath(),
            handle.ContainerID(),
            formatDuration(time.Since(handle.CreatedAt())),
        )
    }

    return w.Flush()
}

func formatDuration(d time.Duration) string {
    // Format like "2m ago", "30s ago"
}
```

**Acceptance Criteria**:
- [ ] Lists all spawned agents
- [ ] Table format displays correctly
- [ ] JSON format works (optional)
- [ ] Shows "No agents running" when empty

---

### Task 3.2: Stop Command (2 hours)

**File**: `cmd/agentd/cmd_stop.go`

**Implementation**:
```go
package main

import (
    "context"
    "fmt"

    "github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
    Use:   "stop <agent-id> [agent-id...]",
    Short: "Stop agent(s) and cleanup resources",
    Args:  cobra.MinimumNArgs(1),
    RunE:  runStop,
}

func init() {
    rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    launcher := createLauncher()

    // Stop each agent
    for _, agentID := range args {
        if err := stopAgent(ctx, launcher, agentID); err != nil {
            fmt.Fprintf(os.Stderr, "Failed to stop %s: %v\n", agentID, err)
            continue
        }

        fmt.Printf("✓ Stopped container\n")
        fmt.Printf("✓ Removed worktree\n")
        fmt.Printf("✓ Cleaned credentials\n")
    }

    return nil
}

func stopAgent(ctx context.Context, launcher *container.AgentContainerLauncher, agentID string) error {
    // Get handle first (for logging)
    handle := launcher.GetHandle(agentID)
    if handle == nil {
        return fmt.Errorf("agent not found: %s", agentID)
    }

    // Stop via launcher (handles all cleanup)
    if err := launcher.Stop(ctx, agentID); err != nil {
        return err
    }

    return nil
}
```

**Acceptance Criteria**:
- [ ] Stops single agent
- [ ] Stops multiple agents (variadic args)
- [ ] Idempotent (no error if agent doesn't exist)
- [ ] Cleanup verified: container gone, worktree removed
- [ ] Prints success for each step

---

### Task 3.3: End-to-End MVP Testing (2 hours)

**Test Script**:
```bash
#!/bin/bash
# scripts/test-mvp.sh

set -e

echo "=== agentd MVP Test Suite ==="

# Clean slate
echo "Cleaning previous state..."
agentd stop alice bob charlie 2>/dev/null || true
rm -rf .agentd/worktrees

# Test 1: Doctor
echo -e "\n[Test 1] Doctor check..."
agentd doctor

# Test 2: Spawn single agent
echo -e "\n[Test 2] Spawn single agent..."
agentd spawn alice
agentd list | grep -q alice

# Test 3: Spawn multiple agents
echo -e "\n[Test 3] Spawn multiple agents..."
agentd spawn bob
agentd spawn charlie
COUNT=$(agentd list | grep -c running)
[ "$COUNT" -eq 3 ] || (echo "Expected 3 agents, got $COUNT"; exit 1)

# Test 4: Verify isolation
echo -e "\n[Test 4] Verify isolation..."
[ -d .agentd/worktrees/alice ] || (echo "Alice worktree missing"; exit 1)
[ -d .agentd/worktrees/bob ] || (echo "Bob worktree missing"; exit 1)
docker ps | grep -q agentd-

# Test 5: Stop single agent
echo -e "\n[Test 5] Stop single agent..."
agentd stop alice
! agentd list | grep -q alice || (echo "Alice still running"; exit 1)

# Test 6: Stop remaining agents
echo -e "\n[Test 6] Stop remaining agents..."
agentd stop bob charlie
agentd list | grep -q "No agents running"

echo -e "\n✓ All MVP tests passed!"
```

**Acceptance Criteria**:
- [ ] Full lifecycle works end-to-end
- [ ] spawn → list → stop → list works
- [ ] Multiple agents work concurrently
- [ ] Cleanup verified
- [ ] Test script passes

---

## Day 4 (Thursday): Optional Features & Visual Polish - 6 hours

### Task 4.1: Logs Command (3 hours) OR Config Support (3 hours)

**Option A: Logs Command**

**File**: `cmd/agentd/cmd_logs.go`

```go
package main

import (
    "context"
    "fmt"
    "io"
    "os"

    "github.com/spf13/cobra"
)

var (
    logsFollow bool
    logsTail   int
)

var logsCmd = &cobra.Command{
    Use:   "logs <agent-id>",
    Short: "Stream agent container logs",
    Args:  cobra.ExactArgs(1),
    RunE:  runLogs,
}

func init() {
    logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", true, "Follow log output")
    logsCmd.Flags().IntVar(&logsTail, "tail", 0, "Show last N lines (0 = all)")
    rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
    ctx := context.Background()
    agentID := args[0]

    launcher := createLauncher()
    handle := launcher.GetHandle(agentID)
    if handle == nil {
        return fmt.Errorf("agent not found: %s", agentID)
    }

    // Stream logs from container
    return streamContainerLogs(ctx, handle.ContainerID())
}

func streamContainerLogs(ctx context.Context, containerID string) error {
    // Use Docker SDK to stream logs
    // io.Copy from container logs to stdout
    // Prefix each line with [agent-id]
}
```

**Acceptance Criteria**:
- [ ] Streams logs in real-time
- [ ] --follow works
- [ ] --tail N works
- [ ] Prefixes lines with agent ID
- [ ] Ctrl-C stops streaming cleanly

**Option B: Config Support**

**File**: `cmd/agentd/config.go`

```go
package main

import (
    "os"
    "path/filepath"

    "github.com/spf13/viper"
)

type Config struct {
    Image         string            `yaml:"image"`
    WorkspaceBase string            `yaml:"workspace_base"`
    Entrypoint    []string          `yaml:"entrypoint"`
    Command       []string          `yaml:"command"`
    Env           map[string]string `yaml:"env"`
}

func loadConfig() (*Config, error) {
    viper.SetConfigName(".agentd")
    viper.SetConfigType("yaml")

    // Check current directory
    viper.AddConfigPath(".")

    // Check home directory
    home, err := os.UserHomeDir()
    if err == nil {
        viper.AddConfigPath(home)
    }

    // Set defaults
    viper.SetDefault("image", "ourocodus/agent:latest")
    viper.SetDefault("workspace_base", ".agentd/worktrees")
    viper.SetDefault("entrypoint", []string{"/usr/local/bin/claude-code-acp"})
    viper.SetDefault("command", []string{"server"})

    // Read config (if exists)
    if err := viper.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); ok {
            // Config file not found; use defaults
        } else {
            return nil, err
        }
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}
```

**Acceptance Criteria**:
- [ ] Reads .agentd.yml from current dir or home
- [ ] Uses defaults if config missing
- [ ] CLI flags override config values
- [ ] Config validated on load

**Choose based on progress**: If ahead of schedule, do logs. If behind, defer logs and do config for smoother demo.

---

### Task 4.2: Demo Setup Script (1.5 hours)

**File**: `scripts/demo-setup.sh`

```bash
#!/bin/bash
# Demo setup script - pre-create everything for smooth demo

set -e

echo "=== agentd Demo Setup ==="

# Check prerequisites
command -v docker >/dev/null || { echo "Docker not found"; exit 1; }
command -v git >/dev/null || { echo "Git not found"; exit 1; }

# Build agentd
echo "Building agentd..."
make build

# Run doctor
echo "Running doctor..."
./bin/agentd doctor

# Pull image (avoid live pull during demo)
echo "Pre-pulling image..."
docker pull ourocodus/agent:latest

# Clean any previous demo state
echo "Cleaning previous demo state..."
./bin/agentd stop alice bob 2>/dev/null || true
rm -rf .agentd/worktrees

# Pre-create demo agents (optional)
echo "Pre-spawning demo agents..."
./bin/agentd spawn alice
./bin/agentd spawn bob

# Verify
echo -e "\nDemo state ready:"
./bin/agentd list

echo -e "\n✓ Demo setup complete!"
echo "Run: ./bin/agentd list"
```

**Acceptance Criteria**:
- [ ] Script builds agentd
- [ ] Pre-pulls image
- [ ] Cleans previous state
- [ ] Optionally pre-spawns agents
- [ ] Verifies everything works

---

### Task 4.3: Visual Polish & Subtle Delight (1.5 hours)

**Design Philosophy**: "Useful over all but sprinkling in some joy"

**Visual Design Language**:
- **3-5 total emoji** (teaching the three isolation layers):
  - 🌳 Worktree (code layer)
  - 📦 Container (process layer)
  - 🔑 Credentials (access layer)
  - ✓ Success
  - × Failure
- **Color for semantic meaning** (via `github.com/fatih/color`):
  - Green for success/running
  - Red for errors
  - Yellow for warnings
  - Cyan for info
- **Clean typography**: ✓ × for outcomes, clear hierarchy
- **NO themed characters** (no elves, gnomes, crystals)
- **NO ASCII art or heavy borders**

**Implementation**:

```go
// Add dependency
go get github.com/fatih/color@latest

// In cmd/agentd/output.go
package main

import "github.com/fatih/color"

var (
    successColor = color.New(color.FgGreen)
    errorColor   = color.New(color.FgRed)
    warningColor = color.New(color.FgYellow)
    infoColor    = color.New(color.FgCyan)
)

func printSuccess(msg string) {
    successColor.Print("✓ ")
    fmt.Println(msg)
}

func printError(msg string) {
    errorColor.Print("× ")
    fmt.Println(msg)
}

// Update spawn output
func printSpawnSuccess(handle *container.AgentContainerHandle) {
    fmt.Println("Creating isolated agent...")
    fmt.Println()
    infoColor.Print("🌳 ")
    fmt.Printf("Worktree: %s (branch: %s)\n", handle.WorktreePath(), handle.Branch())
    infoColor.Print("📦 ")
    fmt.Printf("Container: %s (running)\n", handle.ContainerID())
    infoColor.Print("🔑 ")
    fmt.Println("Credentials: mounted at /root/.creds (read-only)")
    fmt.Println()
    printSuccess(fmt.Sprintf("Agent %s ready", handle.AgentID()))
}
```

**Updated Output Examples**:

```bash
# Doctor
$ agentd doctor
Checking environment...

✓ Docker daemon running (v27.4.1)
✓ File sharing enabled
✓ Image present: ourocodus/agent:latest
✓ Git worktree support confirmed
✓ Disk space sufficient (5.2GB)

Environment ready

# Spawn
$ agentd spawn alice
Creating isolated agent 'alice'...

🌳 Worktree: .agentd/worktrees/alice (branch: agentd/alice-a1b2c3)
📦 Container: agentd-a1b2c3 (running)
🔑 Credentials: mounted at /root/.creds (read-only)

✓ Agent alice ready

# List
$ agentd list

AGENT  STATUS   WORKTREE                   CONTAINER      CREATED
alice  running  .agentd/worktrees/alice    agentd-a1b2c3  2m ago
bob    running  .agentd/worktrees/bob      agentd-d4e5f6  1m ago

# Error example
$ agentd spawn alice
× Agent 'alice' already exists
  Use 'agentd list' to see active agents or 'agentd stop alice' to remove it
```

**Checklist**:
- [ ] Add `github.com/fatih/color` dependency
- [ ] Create output helpers (printSuccess, printError, etc.)
- [ ] Update all command outputs with emoji + color
- [ ] Keep it minimal (3-5 emoji total, only where meaningful)
- [ ] Test color output in terminal
- [ ] Verify it's still scannable and professional

---

## Day 5 (Friday): Demo Prep - 4 hours

### Task 5.1: Help Text & Usage Polish (1 hour)

**Improve all help text**:

```bash
$ agentd --help
agentd - Multi-agent isolation orchestrator

Demonstrates Ourocodus's three-layer isolation architecture:
  - Git worktrees isolate code
  - Docker containers isolate processes
  - Credential volumes isolate access

Multiple agents work concurrently without conflicts.

Usage:
  agentd [command]

Available Commands:
  doctor      Validate environment
  spawn       Spawn an isolated agent
  list        List all active agents
  stop        Stop agent(s) and cleanup resources
  logs        Stream agent container logs
  help        Help about any command

Flags:
  -h, --help      help for agentd
      --version   version for agentd

Use "agentd [command] --help" for more information about a command.
```

**Acceptance Criteria**:
- [ ] All commands have clear descriptions
- [ ] Examples in help text
- [ ] Consistent formatting
- [ ] No typos or grammar issues

---

### Task 5.2: Demo Script Finalization (1 hour)

**File**: `scripts/demo-script.sh`

```bash
#!/bin/bash
# 4-minute demo script
# Run this for the Friday demo

set -e

# Setup
clear
echo "=== agentd Demo - Multi-Agent Isolation ==="
echo ""
sleep 2

# Act 1: Doctor (30s)
echo "$ agentd doctor"
./bin/agentd doctor
echo ""
sleep 3

# Act 2: Spawn agents (1m)
echo "$ agentd spawn alice"
./bin/agentd spawn alice
echo ""
sleep 2

echo "$ agentd spawn bob"
./bin/agentd spawn bob
echo ""
sleep 2

# Act 3: Show isolation (1m30s)
echo "$ agentd list"
./bin/agentd list
echo ""
sleep 3

echo "$ agentd logs alice --tail 5"
./bin/agentd logs alice --tail 5 &
LOGS_PID=$!
sleep 5
kill $LOGS_PID 2>/dev/null || true
echo ""

echo "$ git worktree list"
git worktree list
echo ""
sleep 3

# Act 4: Cleanup (1m)
echo "$ agentd stop alice"
./bin/agentd stop alice
echo ""
sleep 2

echo "$ agentd list"
./bin/agentd list
echo ""
sleep 2

echo "$ agentd stop bob"
./bin/agentd stop bob
echo ""

echo "$ agentd list"
./bin/agentd list
echo ""

# Closing
echo ""
echo "=== Demo Complete ==="
echo "agentd demonstrates three-layer isolation:"
echo "  ✓ Git worktrees isolate code"
echo "  ✓ Docker containers isolate processes"
echo "  ✓ Credentials isolate access"
echo ""
echo "Multiple agents work concurrently without conflicts."
```

**Acceptance Criteria**:
- [ ] Script runs in <4 minutes
- [ ] Clear narration points
- [ ] Timing feels natural
- [ ] No errors during run

---

### Task 5.3: Record Fallback Screencast (1.5 hours)

**Using asciinema or similar**:

```bash
# Record the demo
asciinema rec demo.cast

# Run demo script
./scripts/demo-script.sh

# Stop recording (Ctrl-D)

# Play back to verify
asciinema play demo.cast

# Upload or convert to GIF
```

**Acceptance Criteria**:
- [ ] Screencast recorded successfully
- [ ] Runs in <4 minutes
- [ ] All commands visible
- [ ] No errors in recording
- [ ] Backup saved (in case live demo fails)

---

### Task 5.4: Final Rehearsal & Contingency Check (30 min)

**Checklist**:
- [ ] Run demo script 3 times (catch any flakiness)
- [ ] Verify Docker is running
- [ ] Image is pulled
- [ ] No orphaned containers/worktrees
- [ ] Fallback screencast works
- [ ] All commands in PATH
- [ ] Terminal configured (font size, colors)

**Contingency Plan**:
- If Docker Desktop acts up → Use fallback screencast
- If live typing fails → Use pre-scripted commands
- If agent spawn hangs → Show pre-created agents

---

## Success Criteria

### MVP Complete Checklist

**Core Functionality**:
- [ ] `agentd doctor` validates environment
- [ ] `agentd spawn <id>` creates isolated agent
- [ ] `agentd list` shows all agents
- [ ] `agentd stop <id>` cleans up resources
- [ ] `agentd logs <id>` streams output (optional)

**Quality Bars**:
- [ ] All commands have --help text
- [ ] Error messages are actionable
- [ ] Idempotent operations
- [ ] Clean exit codes (0=success, 1=error)
- [ ] No panics or raw errors

**Demo Ready**:
- [ ] 4-minute demo script works
- [ ] Fallback screencast recorded
- [ ] Demo setup script automates prep
- [ ] All checks green on demo machine

**Documentation**:
- [ ] Design doc complete (`docs/plans/agentd-mvp.md`)
- [ ] Implementation tasks documented (this file)
- [ ] Demo script commented
- [ ] README updated with agentd usage

---

## Risk Tracking

| Risk | Status | Mitigation |
|------|--------|------------|
| macOS Docker mount issues | ACTIVE | Day 2 Task 2.5 - dedicated testing |
| pkg/ API mismatch | ACTIVE | Day 1 Task 1.4 - early validation |
| Timeline overrun | MONITORED | Hard MVP cutoff at Day 3 |
| Demo failure | MITIGATED | Fallback screencast (Day 5) |
| Log streaming complexity | MONITORED | Optional Day 4 feature |

---

## Daily Standup Format

**End of Each Day - Report**:
1. What was completed today?
2. What's blocked or at risk?
3. What's the plan for tomorrow?

**Example (End of Day 1)**:
> ✅ Completed: Cobra scaffold, label schema, doctor command (all checks)
> ✅ Completed: pkg/ API validation - no showstoppers found
> ⚠️ Risk: Docker file-sharing check needs macOS testing
> 📋 Tomorrow: Implement spawn command, test macOS mounts

---

*Document Version: 1.0*
*Last Updated: 2025-01-18*
*Estimated Total Effort: 32 hours over 5 days*
