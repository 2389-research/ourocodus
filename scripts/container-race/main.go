package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/agent/packnplay"
)

// ANSI color codes for visual flair
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

var racerColors = []string{colorRed, colorGreen, colorYellow, colorBlue, colorPurple, colorCyan}

type racer struct {
	name      string
	color     string
	handle    agent.AgentHandle
	startTime time.Time
	endTime   time.Time
	output    []string
	position  int
}

func main() {
	displayIntro()

	ctx := context.Background()

	// Auto-detect and configure Colima on macOS
	if dockerHost := detectColimaSocket(); dockerHost != "" {
		fmt.Printf("🐳 Detected Colima at %s\n\n", dockerHost)
		os.Setenv("DOCKER_HOST", dockerHost)
	}

	// Create PackNplay launcher
	projectPath, err := filepath.Abs(".")
	if err != nil {
		fatal("Failed to get project path: %v", err)
	}

	launcher, err := packnplay.NewLauncher(
		packnplay.WithProjectPath(projectPath),
		packnplay.WithVerbose(false),
	)
	if err != nil {
		fatal("Failed to create launcher: %v", err)
	}

	// Define our racers
	racerNames := []string{"🔴 RedRocket", "🟢 GreenMachine", "🟡 YellowFlash", "🔵 BlueBlaze", "🟣 PurplePower"}
	racers := make([]*racer, len(racerNames))

	fmt.Println(colorBold + "🏗️  SPAWNING CONTAINERS & WORKTREES" + colorReset)
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Each container gets its own isolated git worktree...")
	fmt.Println()

	// Spawn all racers in parallel
	var wg sync.WaitGroup
	startGate := make(chan struct{})

	for i, name := range racerNames {
		racers[i] = &racer{
			name:  name,
			color: racerColors[i%len(racerColors)],
		}

		wg.Add(1)
		go func(r *racer, idx int) {
			defer wg.Done()

			// Create unique racing task for each container
			// Each racer counts to a random number (simulating work)
			countTo := 15 + (idx * 3) // Stagger finish times for drama
			script := fmt.Sprintf(`
				echo "🏎️  %s starting engine..."
				for i in $(seq 1 %d); do
					echo "🏁 Lap $i/%d"
					sleep 0.$(( RANDOM %% 3 + 1 ))
				done
				echo "🏆 %s crossed the finish line!"
			`, r.name, countTo, countTo, r.name)

			cfg := &agent.SpawnConfig{
				Role:    fmt.Sprintf("racer-%d", idx),
				Image:   "busybox:latest",
				Command: []string{"sh", "-c", script},
			}

			<-startGate // Wait for starting gun
			r.startTime = time.Now()

			handle, err := launcher.Spawn(ctx, cfg)
			if err != nil {
				fmt.Printf("%s❌ %s failed to spawn: %v%s\n", r.color, r.name, err, colorReset)
				return
			}
			r.handle = handle

			// Stream output with colors
			go streamOutput(r)

		}(racers[i], i)
	}

	// Show pre-race info
	time.Sleep(1 * time.Second)
	fmt.Println(colorBold + "🏁 STARTING POSITIONS:" + colorReset)
	for i, r := range racers {
		fmt.Printf("  Lane %d: %s%s%s\n", i+1, r.color, r.name, colorReset)
	}
	fmt.Println()

	// Countdown
	fmt.Println(colorBold + "🚦 Starting in..." + colorReset)
	for i := 3; i > 0; i-- {
		fmt.Printf("   %s%d...%s\n", colorYellow, i, colorReset)
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println(colorGreen + colorBold + "   GO! 🏁" + colorReset)
	fmt.Println()

	// Open starting gate
	close(startGate)

	// Wait for all spawns to complete
	wg.Wait()

	// Display race in progress
	fmt.Println(colorBold + "📊 RACE IN PROGRESS..." + colorReset)
	fmt.Println()

	// Wait for all racers to finish
	finishOrder := make([]*racer, 0, len(racers))
	var finishMu sync.Mutex

	for _, r := range racers {
		if r.handle == nil {
			continue
		}

		go func(racer *racer) {
			_ = racer.handle.Wait(ctx)
			racer.endTime = time.Now()

			finishMu.Lock()
			racer.position = len(finishOrder) + 1
			finishOrder = append(finishOrder, racer)
			finishMu.Unlock()

			duration := racer.endTime.Sub(racer.startTime)
			fmt.Printf("%s%s🏁 %s finished! Position #%d (%.2fs)%s\n",
				colorBold, racer.color, racer.name, racer.position, duration.Seconds(), colorReset)
		}(r)
	}

	// Wait for all to finish
	time.Sleep(20 * time.Second)

	// Stop any remaining containers
	fmt.Println()
	fmt.Println(colorBold + "🛑 Race complete! Stopping containers..." + colorReset)
	for _, r := range racers {
		if r.handle != nil {
			_ = launcher.Stop(ctx, r.handle)
		}
	}

	// Display final results
	displayResults(finishOrder)

	// Show worktree info
	fmt.Println()
	fmt.Println(colorBold + "📁 Git Worktrees Created:" + colorReset)
	for _, r := range racers {
		if r.handle != nil {
			fmt.Printf("  %s%s%s → %s\n", r.color, r.name, colorReset, r.handle.Workspace())
		}
	}

	fmt.Println()
	fmt.Println(colorBold + "✨ Demo Complete! ✨" + colorReset)
	fmt.Println()
	fmt.Println("Key Features Demonstrated:")
	fmt.Println("  ✓ Parallel container spawning")
	fmt.Println("  ✓ Isolated git worktrees per container")
	fmt.Println("  ✓ Real-time I/O streaming")
	fmt.Println("  ✓ Container lifecycle management")
	fmt.Println("  ✓ Docker + PackNplay integration")
}

func displayIntro() {
	fmt.Println(colorBold + "🏁 OUROCODUS CONTAINER RACE 🏁" + colorReset)
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println(colorBold + "📦 What is PackNplay?" + colorReset)
	fmt.Println("PackNplay spawns containerized agents with isolated git worktrees.")
	fmt.Println("Each agent gets its own Docker container + its own git workspace.")
	fmt.Println("Perfect for parallel execution without conflicts!")
	fmt.Println()
	fmt.Println(colorBold + "🎯 This Demo Shows:" + colorReset)
	fmt.Println("  • Spawning 5 containers in parallel")
	fmt.Println("  • Each with isolated git worktree")
	fmt.Println("  • Real-time I/O streaming from all containers")
	fmt.Println("  • Complete lifecycle: spawn → run → stop → cleanup")
	fmt.Println()
	fmt.Println(colorBold + "🏎️  The Race:" + colorReset)
	fmt.Println("Each container counts through \"laps\" at different speeds.")
	fmt.Println("First to finish wins!")
	fmt.Println()
}

func streamOutput(r *racer) {
	scanner := bufio.NewScanner(r.handle.Stdout())
	for scanner.Scan() {
		line := scanner.Text()
		r.output = append(r.output, line)
		fmt.Printf("%s[%s]%s %s\n", r.color, r.name, colorReset, line)
	}
}

func displayResults(finishOrder []*racer) {
	fmt.Println()
	fmt.Println(colorBold + "🏆 FINAL RESULTS 🏆" + colorReset)
	fmt.Println("═══════════════════════════════════════════════════════════")

	if len(finishOrder) == 0 {
		fmt.Println("No racers finished :(")
		return
	}

	for i, r := range finishOrder {
		duration := r.endTime.Sub(r.startTime)
		medal := ""
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		default:
			medal = "  "
		}

		fmt.Printf("%s %d. %s%s%s - %.2fs\n",
			medal, i+1, r.color, r.name, colorReset, duration.Seconds())
	}

	// Winner celebration
	if len(finishOrder) > 0 {
		winner := finishOrder[0]
		fmt.Println()
		fmt.Printf("%s%s🎉 %s WINS! 🎉%s\n",
			colorBold, winner.color, winner.name, colorReset)
		fmt.Println()
		displayPodium(finishOrder)
	}
}

func displayPodium(finishOrder []*racer) {
	if len(finishOrder) < 3 {
		return
	}

	fmt.Println("                    🏆 PODIUM 🏆")
	fmt.Println()
	fmt.Println("                  ╔═══════════╗")
	fmt.Printf("                  ║  %s1st%s     ║\n", colorYellow, colorReset)
	fmt.Printf("                  ║ %s%s%s ║\n", finishOrder[0].color, strings.Repeat("─", 9), colorReset)
	fmt.Println("     ╔═══════════╗╚═══════════╝╔═══════════╗")
	fmt.Printf("     ║  %s2nd%s     ║              ║  %s3rd%s     ║\n",
		colorCyan, colorReset, colorOrange(), colorReset)
	fmt.Printf("     ║ %s%s%s ║              ║ %s%s%s ║\n",
		finishOrder[1].color, strings.Repeat("─", 9), colorReset,
		finishOrder[2].color, strings.Repeat("─", 9), colorReset)
	fmt.Println("     ╚═══════════╝              ╚═══════════╝")
}

func colorOrange() string {
	return "\033[38;5;208m" // 256-color orange
}

func detectColimaSocket() string {
	// Check if DOCKER_HOST is already set
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host
	}

	// Check for Colima default socket on macOS
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	colimaSocket := filepath.Join(homeDir, ".colima", "default", "docker.sock")
	if _, err := os.Stat(colimaSocket); err == nil {
		return "unix://" + colimaSocket
	}

	return ""
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, colorRed+"❌ "+format+colorReset+"\n", args...)
	os.Exit(1)
}
