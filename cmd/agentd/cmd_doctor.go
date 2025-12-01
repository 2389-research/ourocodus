package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	doctortui "github.com/2389-research/ourocodus/cmd/agentd/internal/tui/doctor"
	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "🩺 Validate environment for running agents",
	Long: `Validates your environment is ready to run agents:
  ✓ Docker daemon connectivity and version
  ✓ File sharing permissions (macOS)
  ✓ Agent image availability
  ✓ Git worktree support
  ✓ Disk space requirements
  ✓ Container spawn smoke test

Run this before spawning agents to catch configuration issues early.`,
	Example: `  # Validate environment
  agentd doctor

  # Doctor checks are helpful when:
  # - First time using agentd
  # - After Docker Desktop updates
  # - Debugging spawn failures`,
	RunE: runDoctor,
}

type Check struct {
	Name string
	Run  func(context.Context) (string, error) // Returns (message, error)
}

// doctorTheme provides consistent styling for doctor output
var doctorTheme = theme.Default()

func printSuccess(msg string) {
	style := lipgloss.NewStyle().Foreground(doctorTheme.Success)
	fmt.Print(style.Render("✓ "))
	fmt.Println(msg)
}

func printError(msg string) {
	style := lipgloss.NewStyle().Foreground(doctorTheme.Error)
	fmt.Print(style.Render("× "))
	fmt.Println(msg)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	checks := []Check{
		{"Docker daemon", checkDockerDaemon},
		{"Docker version", checkDockerVersion},
		{"File sharing", checkFileSharingMacOS},
		{"Image presence", checkImagePresence},
		{"Git worktree support", checkGitWorktreeSupport},
		{"Disk space", checkDiskSpace},
		{"Spawn smoke test", checkSpawnSmokeTest},
	}

	// Get mode from AppContext (set by cli.App wrapper)
	appCtx := cli.FromContext(ctx)
	if appCtx == nil {
		// Fallback for tests or direct cobra execution
		return runDoctorLegacy(ctx, checks, cli.ModePlain)
	}

	// Use TUI for rich mode, legacy for others
	if appCtx.Mode.IsRich() {
		return runDoctorTUI(ctx, checks)
	}
	return runDoctorLegacy(ctx, checks, appCtx.Mode)
}

// runDoctorTUI runs the doctor command with a Bubble Tea TUI.
func runDoctorTUI(ctx context.Context, checks []Check) error {
	checkNames := make([]string, len(checks))
	for i, c := range checks {
		checkNames[i] = c.Name
	}

	m := doctortui.New(checkNames)
	p := tea.NewProgram(m)

	// Channel to receive final result
	resultCh := make(chan bool, 1)

	// Run checks in background
	go func() {
		allPassed := true

		for i, check := range checks {
			p.Send(doctortui.CheckStartMsg{Index: i})
			time.Sleep(50 * time.Millisecond)

			msg, err := check.Run(ctx)
			if err != nil {
				// Check if it's a skip (error message contains "skipped")
				errStr := err.Error()
				if strings.Contains(strings.ToLower(errStr), "skip") || strings.Contains(errStr, "skipped") {
					p.Send(doctortui.CheckSkipMsg{Index: i, Message: msg})
				} else {
					p.Send(doctortui.CheckFailMsg{Index: i, Error: errStr})
					allPassed = false
				}
			} else {
				p.Send(doctortui.CheckPassMsg{Index: i, Message: msg})
			}
		}

		p.Send(doctortui.AllChecksCompleteMsg{AllPassed: allPassed})
		resultCh <- allPassed
	}()

	// Run TUI
	if _, err := p.Run(); err != nil {
		return err
	}

	// Get result
	allPassed := <-resultCh
	if !allPassed {
		return fmt.Errorf("environment validation failed")
	}

	return nil
}

// DoctorResult represents a single check result for JSON output.
type DoctorResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "passed", "skipped", "failed"
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// DoctorResults represents all doctor results for JSON output.
type DoctorResults struct {
	Results []DoctorResult `json:"results"`
	Success bool           `json:"success"`
}

// runDoctorLegacy runs doctor checks without TUI (for JSON/plain mode).
func runDoctorLegacy(ctx context.Context, checks []Check, mode cli.Mode) error {
	results := make([]DoctorResult, 0, len(checks))
	allPassed := true

	for _, check := range checks {
		result := DoctorResult{Name: check.Name}

		msg, err := check.Run(ctx)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(strings.ToLower(errStr), "skip") {
				result.Status = "skipped"
				result.Message = msg
				if !mode.IsJSON() {
					fmt.Printf("⊘ %s (%s)\n", check.Name, msg)
				}
			} else {
				result.Status = "failed"
				result.Error = errStr
				allPassed = false
				if !mode.IsJSON() {
					printError(fmt.Sprintf("%s: %s", check.Name, errStr))
				}
			}
		} else {
			result.Status = "passed"
			result.Message = msg
			if !mode.IsJSON() {
				if msg != "" {
					printSuccess(fmt.Sprintf("%s (%s)", check.Name, msg))
				} else {
					printSuccess(check.Name)
				}
			}
		}
		results = append(results, result)
	}

	if mode.IsJSON() {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(DoctorResults{
			Results: results,
			Success: allPassed,
		})
	} else {
		fmt.Println()
		if allPassed {
			fmt.Println("Environment ready! All systems go for spawning agents.")
		}
	}

	if !allPassed {
		return fmt.Errorf("environment validation failed")
	}

	return nil
}

func checkDockerDaemon(ctx context.Context) (string, error) {
	dockerCli, err := newDockerClient()
	if err != nil {
		return "", fmt.Errorf("docker client creation failed: %w", err)
	}
	defer func() { _ = dockerCli.Close() }()

	if _, err := dockerCli.Ping(ctx); err != nil {
		return "", fmt.Errorf("docker daemon not running: start Docker Desktop and retry")
	}

	// Get version for display
	version, err := dockerCli.ServerVersion(ctx)
	if err != nil {
		return "running", nil
	}

	return fmt.Sprintf("v%s", version.Version), nil
}

func checkDockerVersion(ctx context.Context) (string, error) {
	dockerCli, err := newDockerClient()
	if err != nil {
		return "", err
	}
	defer func() { _ = dockerCli.Close() }()

	version, err := dockerCli.ServerVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get Docker version: %w", err)
	}

	// Check if version >= 20.10 (numeric comparison for correct ordering)
	versionParts := strings.Split(version.Version, ".")
	if len(versionParts) < 2 {
		return "", fmt.Errorf("unexpected version format: %s", version.Version)
	}

	// Parse major version number
	major := versionParts[0]
	majorInt, err := strconv.Atoi(major)
	if err != nil || majorInt < 20 {
		return "", fmt.Errorf("docker version %s is too old (need >= 20.10)", version.Version)
	}

	return ">= 20.10", nil
}

func checkFileSharingMacOS(_ context.Context) (string, error) {
	// Only check on macOS
	if runtime.GOOS != "darwin" {
		return "skipped on non-macOS", fmt.Errorf("skipped")
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// On macOS, we'll just warn if the path isn't in typical shared locations
	// Docker Desktop for Mac typically shares /Users, /tmp, /private by default
	sharedPrefixes := []string{"/Users/", "/tmp/", "/private/"}
	isShared := false
	for _, prefix := range sharedPrefixes {
		if strings.HasPrefix(cwd, prefix) {
			isShared = true
			break
		}
	}

	if !isShared {
		return "", fmt.Errorf("current directory may not be shared with Docker.\nAdd %s to Docker Desktop file sharing settings", cwd)
	}

	return "enabled", nil
}

func checkImagePresence(ctx context.Context) (string, error) {
	dockerCli, err := newDockerClient()
	if err != nil {
		return "", err
	}
	defer func() { _ = dockerCli.Close() }()

	imageName := "ourocodus/agent:latest"

	// Try to inspect the image
	_, err = dockerCli.ImageInspect(ctx, imageName)
	if err != nil {
		// Image not present - offer guidance
		return "", fmt.Errorf("image not present (run: docker pull %s)", imageName)
	}

	return imageName, nil
}

func checkGitWorktreeSupport(ctx context.Context) (string, error) {
	// Check if git command exists
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git not found in PATH")
	}

	// Run git worktree list to verify support
	cmd := exec.CommandContext(ctx, "git", "worktree", "list")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree not supported (need git >= 2.5)")
	}

	return "supported", nil
}

func checkDiskSpace(_ context.Context) (string, error) {
	// Skip on Windows - syscall.Statfs doesn't exist there
	if runtime.GOOS == "windows" {
		return "skipped on Windows", fmt.Errorf("skipped")
	}

	// Get disk space for current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(cwd, &stat); err != nil {
		return "", fmt.Errorf("failed to check disk space: %w", err)
	}

	// Validate block size before conversion (gosec G115)
	// On some platforms stat.Bsize may be signed
	if stat.Bsize <= 0 {
		return "", fmt.Errorf("invalid block size reported by filesystem: %d", stat.Bsize)
	}

	// Available space in bytes
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	availableGB := float64(availableBytes) / (1024 * 1024 * 1024)

	// Require at least 1GB free
	minGB := 1.0
	if availableGB < minGB {
		return "", fmt.Errorf("insufficient disk space: %.1fGB available (need >= %.1fGB)", availableGB, minGB)
	}

	return fmt.Sprintf("%.1fGB available", availableGB), nil
}

func checkSpawnSmokeTest(ctx context.Context) (string, error) {
	dockerCli, err := newDockerClient()
	if err != nil {
		return "", err
	}
	defer func() { _ = dockerCli.Close() }()

	// Create a simple test container
	// Use alpine since it's tiny and likely to be cached
	testImage := "alpine:latest"

	// Pull image if needed
	_, err = dockerCli.ImageInspect(ctx, testImage)
	if err != nil {
		// Image not present, skip smoke test
		return "alpine:latest not available", fmt.Errorf("skipped")
	}

	// Create container with auto-generated name (empty string) to prevent name collision
	// from crashed prior runs. Cleanup uses container ID, not name.
	resp, err := dockerCli.ContainerCreate(ctx, &container.Config{
		Image: testImage,
		Cmd:   []string{"echo", "test"},
		Labels: map[string]string{
			"agentd.smoke-test": "true",
		},
	}, nil, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create test container: %w", err)
	}

	// Clean up the container
	defer func() {
		// Use a fresh context with timeout for cleanup
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = dockerCli.ContainerRemove(cleanupCtx, resp.ID, container.RemoveOptions{Force: true})
	}()

	return "passed", nil
}
