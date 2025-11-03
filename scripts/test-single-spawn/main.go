package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/agent/packnplay"
)

func main() {
	ctx := context.Background()

	// Setup demo repo path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fatal("Failed to get home directory: %v", err)
	}
	projectPath := filepath.Join(homeDir, ".local", "share", "packnplay", "demo-repo")

	fmt.Println("🧪 MINIMAL SPAWN TEST")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Using fork: v1.0.3-0.20251031201910-406e910bc750\n")
	fmt.Printf("Demo repo: %s\n\n", projectPath)

	// Create launcher
	fmt.Println("Creating launcher...")
	dockerHost := fmt.Sprintf("unix://%s/.colima/default/docker.sock", homeDir)
	launcher, err := packnplay.NewLauncher(
		packnplay.WithProjectPath(projectPath),
		packnplay.WithDockerHost(dockerHost),
		packnplay.WithVerbose(true),
	)
	if err != nil {
		fatal("Failed to create launcher: %v", err)
	}
	fmt.Println("✓ Launcher created\n")

	// Spawn single container
	fmt.Println("Spawning SINGLE container...")
	fmt.Println("If this fails → Fork fixes don't work")
	fmt.Println("If this succeeds → Race condition issue\n")

	cfg := &agent.SpawnConfig{
		Role:    "test-single",
		Image:   "busybox:latest",
		Command: []string{"sh", "-c", "echo '✓ Container started successfully' && sleep 5"},
	}

	startTime := time.Now()
	handle, err := launcher.Spawn(ctx, cfg)
	if err != nil {
		fatal("❌ SINGLE SPAWN FAILED: %v\n\nConclusion: Fork fixes DON'T WORK. Need to try Option B (CLI) or C (Docker SDK).", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n✓ SINGLE SPAWN SUCCEEDED in %.2fs\n", elapsed.Seconds())
	fmt.Printf("Container: %s\n", handle.Workspace())
	fmt.Println("\nConclusion: Fork fixes WORK. The issue is likely:")
	fmt.Println("  • Race condition during parallel spawning")
	fmt.Println("  • Shared resource contention")
	fmt.Println("  • Environment state change between spawns")
	fmt.Println("\nRecommendation: Add delays between spawns in demo")

	// Wait for container output
	fmt.Println("\nWaiting 3 seconds for container output...")
	time.Sleep(3 * time.Second)

	// Cleanup
	fmt.Println("\nCleaning up...")
	if err := launcher.Stop(ctx, handle); err != nil {
		fmt.Printf("Warning: cleanup failed: %v\n", err)
	} else {
		fmt.Println("✓ Cleanup complete")
	}

	fmt.Println("\n✅ TEST COMPLETE - FORK WORKS!")
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
