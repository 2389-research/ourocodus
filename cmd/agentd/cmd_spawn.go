package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	spawnWorkspace string
	spawnImage     string
	spawnEnv       []string
	spawnAPIKey    string
)

var spawnCmd = &cobra.Command{
	Use:   "spawn [agent-id]",
	Short: "✨ Spawn an isolated agent",
	Long: `Spawn creates an isolated agent environment with three-layer isolation:
  🌳 Git worktree - Isolated workspace on a dedicated branch
  📦 Docker container - Isolated process with resource limits
  🔑 Credentials - Mounted read-only for security

If no agent-id is provided, one will be generated automatically.`,
	Example: `  # Spawn agent with auto-generated ID
  agentd spawn

  # Spawn agent with custom ID
  agentd spawn alice

  # Spawn with custom image
  agentd spawn bob --image ourocodus/agent:dev

  # Spawn with environment variables
  agentd spawn charlie --env "DEBUG=1" --env "LOG_LEVEL=trace"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSpawn,
}

func init() {
	spawnCmd.Flags().StringVar(&spawnWorkspace, "workspace", "", "Custom worktree path (default: .agentd/worktrees/<id>)")
	spawnCmd.Flags().StringVar(&spawnImage, "image", "ourocodus/agent:latest", "Docker image")
	spawnCmd.Flags().StringArrayVar(&spawnEnv, "env", nil, "Environment variables (KEY=VALUE)")
	spawnCmd.Flags().StringVar(&spawnAPIKey, "api-key", "", "Anthropic API key (or set ANTHROPIC_API_KEY env var)")
}

func runSpawn(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get or generate agent ID
	agentID := generateAgentID(args)

	_, _ = color.New(color.FgCyan, color.Bold).Printf("✨ Creating isolated agent '%s'...\n\n", agentID)

	// Create launcher (wiring pkg/ components)
	launcher, err := createLauncher()
	if err != nil {
		return fmt.Errorf("failed to create launcher: %w", err)
	}

	// Build spawn config
	config, err := buildSpawnConfig(agentID)
	if err != nil {
		return fmt.Errorf("failed to build spawn config: %w", err)
	}

	// Spawn agent (launcher will write credentials after creating worktree)
	handle, err := launcher.Spawn(ctx, config)
	if err != nil {
		// Check for specific error types
		if err == container.ErrAgentAlreadyExists {
			return fmt.Errorf("agent '%s' already exists\nUse 'agentd list' to see active agents or 'agentd stop %s' to remove it", agentID, agentID)
		}
		if strings.Contains(err.Error(), "worktree setup failed") {
			return fmt.Errorf("worktree creation failed: %w\nEnsure git repository is clean", err)
		}
		return fmt.Errorf("spawn failed: %w", err)
	}

	// Print success
	printSpawnSuccess(handle)

	return nil
}

// generateAgentID returns the provided agent ID or generates one
func generateAgentID(args []string) string {
	if len(args) > 0 && args[0] != "" {
		return args[0]
	}

	// Generate: agent-<shortid>
	return fmt.Sprintf("agent-%s", generateShortID())
}

// generateShortID generates a 6-character random alphanumeric ID
func generateShortID() string {
	bytes := make([]byte, 3) // 3 bytes = 6 hex chars
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("%06x", os.Getpid()%1000000)
	}
	return hex.EncodeToString(bytes)
}

// buildSpawnConfig creates the SpawnConfig from flags and defaults
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
		APIKey:     apiKey,
		// GitSSHKey and GitHubToken handled by credential mounter
	}

	return config, nil
}

// parseEnvFlags parses KEY=VALUE environment variable flags
func parseEnvFlags(envFlags []string) ([]string, error) {
	for _, flag := range envFlags {
		if !strings.Contains(flag, "=") {
			return nil, fmt.Errorf("invalid format '%s', expected KEY=VALUE", flag)
		}
	}
	return envFlags, nil
}

// hasCredentialFiles checks if a credentials directory contains any files
func hasCredentialFiles(credPath string) bool {
	entries, err := os.ReadDir(credPath)
	if err != nil {
		return false
	}
	// Check if there are any non-hidden files
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			return true
		}
	}
	return false
}

// printSpawnSuccess prints the successful spawn output with visual hierarchy
func printSpawnSuccess(handle *container.AgentContainerHandle) {
	// Worktree
	fmt.Print("🌳 ")
	_, _ = infoColor.Printf("Worktree: ")
	fmt.Printf("%s ", handle.WorkspacePath())
	_, _ = color.New(color.FgHiBlack).Printf("(branch: %s)\n", handle.BranchName())

	// Container
	fmt.Print("📦 ")
	_, _ = infoColor.Printf("Container: ")
	fmt.Printf("%s ", handle.ContainerID())
	_, _ = successColor.Printf("(running)\n")

	// Credentials
	fmt.Print("🔑 ")
	_, _ = infoColor.Printf("Credentials: ")
	credPath := handle.CredentialsPath()
	if credPath != "" && hasCredentialFiles(credPath) {
		fmt.Printf("mounted at /root/.creds ")
		_, _ = color.New(color.FgHiBlack).Printf("(read-only)\n")
	} else {
		_, _ = color.New(color.FgHiBlack).Printf("(none)\n")
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Agent %s ready", handle.AgentID()))
}
