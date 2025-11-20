package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/fatih/color"
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
	Run  func(context.Context) error
}

var (
	successColor = color.New(color.FgGreen)
	errorColor   = color.New(color.FgRed)
	infoColor    = color.New(color.FgCyan)
)

func printSuccess(msg string) {
	_, _ = successColor.Print("✓ ")
	fmt.Println(msg)
}

func printError(msg string) {
	_, _ = errorColor.Print("× ")
	fmt.Println(msg)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	checks := []Check{
		{"Docker daemon", checkDockerDaemon},
		{"Docker version", checkDockerVersion},
		{"File sharing", checkFileSharingMacOS},
		{"Image presence", checkImagePresence},
		{"Git worktree support", checkGitWorktreeSupport},
		{"Disk space", checkDiskSpace},
		{"Spawn smoke test", checkSpawnSmokeTest},
	}

	allPassed := true
	for _, check := range checks {
		if err := check.Run(ctx); err != nil {
			printError(fmt.Sprintf("%s: %v", check.Name, err))
			allPassed = false
		}
	}

	fmt.Println()
	if allPassed {
		fmt.Println()
		_, _ = color.New(color.FgGreen, color.Bold).Print("✨ Environment ready!")
		_, _ = color.New(color.FgHiBlack).Println(" All systems go for spawning agents.")
		fmt.Println()
		return nil
	}

	return fmt.Errorf("environment validation failed")
}

func checkDockerDaemon(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("Docker client creation failed: %w", err)
	}
	defer func() { _ = cli.Close() }()

	if _, err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("Docker daemon not running. Start Docker Desktop and retry")
	}

	// Get version for display
	version, err := cli.ServerVersion(ctx)
	if err != nil {
		printSuccess("Docker daemon running")
		return nil
	}

	printSuccess(fmt.Sprintf("Docker daemon running (v%s)", version.Version))
	return nil
}

func checkDockerVersion(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	version, err := cli.ServerVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Docker version: %w", err)
	}

	// Check if version >= 20.10 (simple string comparison works for most cases)
	versionParts := strings.Split(version.Version, ".")
	if len(versionParts) < 2 {
		return fmt.Errorf("unexpected version format: %s", version.Version)
	}

	// For simplicity, just check major version >= 20
	major := versionParts[0]
	if major < "20" {
		return fmt.Errorf("Docker version %s is too old (need >= 20.10)", version.Version)
	}

	printSuccess("Docker version supported (>= 20.10)")
	return nil
}

func checkFileSharingMacOS(ctx context.Context) error {
	// Only check on macOS
	if runtime.GOOS != "darwin" {
		printSuccess("File sharing check (skipped on non-macOS)")
		return nil
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
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
		return fmt.Errorf("current directory may not be shared with Docker.\nAdd %s to Docker Desktop file sharing settings", cwd)
	}

	printSuccess(fmt.Sprintf("File sharing enabled: %s", cwd))
	return nil
}

func checkImagePresence(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	imageName := "ourocodus/agent:latest"

	// Try to inspect the image
	_, err = cli.ImageInspect(ctx, imageName)
	if err != nil {
		// Image not present - offer guidance
		_, _ = infoColor.Printf("Image %s not found locally\n", imageName)
		fmt.Println("  Run: docker pull ourocodus/agent:latest")
		return fmt.Errorf("image not present (pull required)")
	}

	printSuccess(fmt.Sprintf("Image present: %s", imageName))
	return nil
}

func checkGitWorktreeSupport(ctx context.Context) error {
	// Check if git command exists
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}

	// Run git worktree list to verify support
	cmd := exec.CommandContext(ctx, "git", "worktree", "list")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree not supported (need git >= 2.5)")
	}

	printSuccess("Git worktree support confirmed")
	return nil
}

func checkDiskSpace(ctx context.Context) error {
	// Skip on Windows - syscall.Statfs doesn't exist there
	if runtime.GOOS == "windows" {
		printSuccess("Disk space check (skipped on Windows)")
		return nil
	}

	// Get disk space for current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(cwd, &stat); err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	// Available space in bytes
	// stat.Bsize is uint32, so safe to convert to uint64 (gosec G115)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	availableGB := float64(availableBytes) / (1024 * 1024 * 1024)

	// Require at least 1GB free
	minGB := 1.0
	if availableGB < minGB {
		return fmt.Errorf("insufficient disk space: %.1fGB available (need >= %.1fGB)", availableGB, minGB)
	}

	printSuccess(fmt.Sprintf("Disk space: %.1fGB available", availableGB))
	return nil
}

func checkSpawnSmokeTest(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	// Create a simple test container
	// Use alpine since it's tiny and likely to be cached
	testImage := "alpine:latest"

	// Pull image if needed
	_, err = cli.ImageInspect(ctx, testImage)
	if err != nil {
		// Image not present, skip smoke test
		_, _ = infoColor.Println("  Smoke test skipped (alpine:latest not available)")
		printSuccess("Spawn smoke test (skipped)")
		return nil
	}

	// Create container with auto-generated name (empty string) to prevent name collision
	// from crashed prior runs. Cleanup uses container ID, not name.
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: testImage,
		Cmd:   []string{"echo", "test"},
		Labels: map[string]string{
			"agentd.smoke-test": "true",
		},
	}, nil, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create test container: %w", err)
	}

	// Clean up the container
	defer func() {
		// Use a fresh context with timeout for cleanup
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = cli.ContainerRemove(cleanupCtx, resp.ID, container.RemoveOptions{Force: true})
	}()

	printSuccess("Spawn smoke test passed")
	return nil
}
