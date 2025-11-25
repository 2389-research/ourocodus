package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	spawnWorkspace string
	spawnImage     string
	spawnEnv       []string
	spawnAPIKey    string
	spawnJSON      bool
	spawnPlain     bool
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
  agentd spawn charlie --env "DEBUG=1" --env "LOG_LEVEL=trace"

  # JSON output for scripting
  agentd spawn --json

  # Plain text output (no colors)
  agentd spawn --plain`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSpawn,
}

func init() {
	spawnCmd.Flags().StringVar(&spawnWorkspace, "workspace", "", "Custom worktree path (default: .agentd/worktrees/<id>)")
	spawnCmd.Flags().StringVar(&spawnImage, "image", "ourocodus/agent:latest", "Docker image")
	spawnCmd.Flags().StringArrayVar(&spawnEnv, "env", nil, "Environment variables (KEY=VALUE)")
	spawnCmd.Flags().StringVar(&spawnAPIKey, "api-key", "", "Anthropic API key (or set ANTHROPIC_API_KEY env var)")
	spawnCmd.Flags().BoolVar(&spawnJSON, "json", false, "Output in JSON format")
	spawnCmd.Flags().BoolVar(&spawnPlain, "plain", false, "Output in plain text (no colors)")
}

func runSpawn(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Detect output mode
	shouldPlain := detect.ShouldUsePlainMode(spawnJSON, spawnPlain, os.Environ)
	mode := output.DetectMode(spawnJSON, spawnPlain, shouldPlain)

	// Get or generate agent ID
	agentID := generateAgentID(args)

	// Check if agent already exists in Docker
	existingContainerID, err := findAgentContainerID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to check for existing agent: %w", err)
	}
	if existingContainerID != "" {
		return fmt.Errorf("agent '%s' already exists\nUse 'agentd list' to see active agents or 'agentd stop %s' to remove it", agentID, agentID)
	}

	// Only print progress for rich/plain mode, not JSON
	if !mode.IsJSON() {
		if mode.IsRich() {
			th := theme.NewRetroTheme(theme.PaletteCGA)
			headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Primary))).Bold(true)
			fmt.Println(headerStyle.Render(fmt.Sprintf("✨ Creating isolated agent '%s'...", agentID)))
			fmt.Println()
		} else {
			fmt.Printf("Creating agent '%s'...\n", agentID)
		}
	}

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

	// Generate attach token (Phase 4: Security Hardening)
	token, err := generateAttachToken(agentID)
	if err != nil {
		// Non-fatal: agent is running, just warn about token (only for non-JSON)
		if !mode.IsJSON() {
			if mode.IsRich() {
				th := theme.NewRetroTheme(theme.PaletteCGA)
				warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Warning)))
				fmt.Println(warnStyle.Render(fmt.Sprintf("⚠️  Warning: Failed to generate attach token: %v", err)))
				fmt.Println(warnStyle.Render("   Agent is running but attachments will not be secured"))
			} else {
				fmt.Printf("Warning: Failed to generate attach token: %v\n", err)
			}
		}
	}

	// Print success based on output mode
	switch {
	case mode.IsJSON():
		printSpawnSuccessJSON(handle, token)
	case mode.IsPlain():
		printSpawnSuccessPlain(handle, token)
	default:
		printSpawnSuccessRich(handle, token)
	}

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

	// Add agent ID and NATS URL to environment for heartbeat publishing
	// Use NATS_URL from environment if set, otherwise default to host.docker.internal
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://host.docker.internal:4222"
	}
	env = append(env,
		fmt.Sprintf("AGENT_ID=%s", agentID),
		fmt.Sprintf("NATS_URL=%s", natsURL),
	)

	config := container.SpawnConfig{
		AgentID:    agentID,
		ImageName:  spawnImage,
		Command:    []string{"--workspace", "/workspace"},
		Entrypoint: []string{"/usr/local/bin/acp"},
		Env:        env,
		APIKey:     apiKey,
		// GitSSHKey and GitHubToken handled by credential mounter
		Labels: map[string]string{
			LabelSpawnSource: "cli",
		},
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

// generateAttachToken generates a cryptographically secure 256-bit attach token.
// The token is stored in .agentd/session/{agent-id}.token with 0600 permissions.
// Returns the base64url-encoded token string.
func generateAttachToken(agentID string) (string, error) {
	// Generate 32 random bytes (256 bits)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode as base64url (URL-safe, no padding)
	tokenStr := base64.RawURLEncoding.EncodeToString(tokenBytes)

	// Ensure session directory exists with secure permissions
	sessionDir := filepath.Join(".agentd", "session")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}

	// Write token to file with 0600 permissions (owner read/write only)
	tokenPath := filepath.Join(sessionDir, agentID+".token")
	if err := os.WriteFile(tokenPath, []byte(tokenStr), 0o600); err != nil {
		return "", fmt.Errorf("failed to write token file: %w", err)
	}

	return tokenStr, nil
}

// SpawnResult represents the output of a successful spawn for JSON output
type SpawnResult struct {
	AgentID         string `json:"agentId"`
	ContainerID     string `json:"containerId"`
	WorkspacePath   string `json:"workspacePath"`
	BranchName      string `json:"branchName"`
	CredentialsPath string `json:"credentialsPath,omitempty"`
	AttachToken     string `json:"attachToken,omitempty"`
	Status          string `json:"status"`
}

// printSpawnSuccessJSON prints the successful spawn output as JSON
func printSpawnSuccessJSON(handle *container.AgentContainerHandle, attachToken string) {
	result := SpawnResult{
		AgentID:       handle.AgentID(),
		ContainerID:   handle.ContainerID(),
		WorkspacePath: handle.WorkspacePath(),
		BranchName:    handle.BranchName(),
		AttachToken:   attachToken,
		Status:        "running",
	}

	credPath := handle.CredentialsPath()
	if credPath != "" && hasCredentialFiles(credPath) {
		result.CredentialsPath = credPath
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(result)
}

// printSpawnSuccessPlain prints the successful spawn output in plain text
func printSpawnSuccessPlain(handle *container.AgentContainerHandle, attachToken string) {
	fmt.Printf("Agent: %s\n", handle.AgentID())
	fmt.Printf("Container: %s (running)\n", handle.ContainerID())
	fmt.Printf("Worktree: %s (branch: %s)\n", handle.WorkspacePath(), handle.BranchName())

	credPath := handle.CredentialsPath()
	if credPath != "" && hasCredentialFiles(credPath) {
		fmt.Printf("Credentials: mounted at /root/.creds (read-only)\n")
	} else {
		fmt.Printf("Credentials: none\n")
	}

	if attachToken != "" {
		fmt.Printf("\nAttach Token: %s\n", attachToken)
	}

	fmt.Printf("\nAgent %s ready\n", handle.AgentID())
}

// printSpawnSuccessRich prints the successful spawn output with colors and styling
func printSpawnSuccessRich(handle *container.AgentContainerHandle, attachToken string) {
	th := theme.NewRetroTheme(theme.PaletteCGA)

	// Create styles
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Primary))).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Accent)))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Muted)))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(th.Success)))

	// Worktree
	fmt.Print("🌳 ")
	fmt.Print(labelStyle.Render("Worktree: "))
	fmt.Print(valueStyle.Render(handle.WorkspacePath()) + " ")
	fmt.Println(mutedStyle.Render(fmt.Sprintf("(branch: %s)", handle.BranchName())))

	// Container
	fmt.Print("📦 ")
	fmt.Print(labelStyle.Render("Container: "))
	fmt.Print(valueStyle.Render(handle.ContainerID()) + " ")
	fmt.Println(successStyle.Render("(running)"))

	// Credentials
	fmt.Print("🔑 ")
	fmt.Print(labelStyle.Render("Credentials: "))
	credPath := handle.CredentialsPath()
	if credPath != "" && hasCredentialFiles(credPath) {
		fmt.Print(valueStyle.Render("mounted at /root/.creds "))
		fmt.Println(mutedStyle.Render("(read-only)"))
	} else {
		fmt.Println(mutedStyle.Render("(none)"))
	}

	// Attach token (Phase 4: Security Hardening)
	if attachToken != "" {
		fmt.Println()
		fmt.Print("🔐 ")
		tokenLabel := lipgloss.NewStyle().Foreground(lipgloss.Color("#00F6FF")).Bold(true)
		fmt.Println(tokenLabel.Render("Attach Token:"))
		fmt.Printf("   %s\n", valueStyle.Render(attachToken))
		fmt.Println()
		fmt.Println(mutedStyle.Render("   Use this token when attaching from PWA or relay:"))
		fmt.Println(mutedStyle.Render(fmt.Sprintf("   → agent:attach {\"agentId\": \"%s\", \"token\": \"<token>\"}", handle.AgentID())))
	}

	fmt.Println()

	// Success message with box
	successBox := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(th.Success))).
		Bold(true).
		Render(fmt.Sprintf("✓ Agent %s ready", handle.AgentID()))
	fmt.Println(successBox)
}
