# 🏁 Container Race Demo

A fun, visual demonstration of PackNplay's containerized agent capabilities!

## What It Shows

This demo showcases the key features of the PackNplay integration:

- **🚀 Parallel Container Spawning** - Multiple Docker containers launch simultaneously
- **📁 Isolated Git Worktrees** - Each container gets its own isolated workspace
- **📊 Real-Time I/O Streaming** - Live output streaming from all containers with color-coded display
- **⏱️  Container Lifecycle** - Complete spawn → execute → stop lifecycle
- **🎯 Race Visualization** - Fun racing metaphor with timing stats and podium display

## Requirements

- **Docker** running and accessible (Docker Desktop or Colima)
- **Go 1.24+** installed
- **busybox:latest** Docker image (will auto-pull on first run)

### For Colima Users

```bash
# Start Colima
colima start

# Set Docker socket (add to your ~/.zshrc or ~/.bashrc)
export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"

# Verify
docker ps
```

## Quick Start

```bash
# Option 1: Via Makefile (recommended)
make container-race

# Option 2: Direct execution
go run scripts/container-race/main.go

# Option 3: Build and run
go build -o bin/container-race scripts/container-race/main.go
./bin/container-race
```

## Demo Output

The demo creates a visual racing experience:

```
🏁 OUROCODUS CONTAINER RACE 🏁
═══════════════════════════════════════════════════════════

🚀 Showcasing PackNplay: Containerized Agents + Git Worktrees

📦 Spawning containers in parallel worktrees...

🏁 STARTING POSITIONS:
  Lane 1: 🔴 RedRocket
  Lane 2: 🟢 GreenMachine
  Lane 3: 🟡 YellowFlash
  Lane 4: 🔵 BlueBlaze
  Lane 5: 🟣 PurplePower

🚦 Starting in...
   3...
   2...
   1...
   GO! 🏁

📊 RACE IN PROGRESS...

[🔴 RedRocket] 🏎️  RedRocket starting engine...
[🔴 RedRocket] 🏁 Lap 1/15
[🟢 GreenMachine] 🏎️  GreenMachine starting engine...
...
[🔴 RedRocket] 🏆 RedRocket crossed the finish line!

🏁 RedRocket finished! Position #1 (4.23s)
...

🏆 FINAL RESULTS 🏆
═══════════════════════════════════════════════════════════
🥇 1. 🔴 RedRocket - 4.23s
🥈 2. 🟢 GreenMachine - 5.12s
🥉 3. 🟡 YellowFlash - 6.45s
   4. 🔵 BlueBlaze - 7.89s
   5. 🟣 PurplePower - 8.34s

🎉 RedRocket WINS! 🎉

                    🏆 PODIUM 🏆

                  ╔═══════════╗
                  ║  1st      ║
                  ║ ───────── ║
     ╔═══════════╗╚═══════════╝╔═══════════╗
     ║  2nd      ║              ║  3rd      ║
     ║ ───────── ║              ║ ───────── ║
     ╚═══════════╝              ╚═══════════╝

📁 Git Worktrees Created:
  🔴 RedRocket → /path/to/project/.git/worktrees/agent-01HJ...
  🟢 GreenMachine → /path/to/project/.git/worktrees/agent-01HJ...
  ...

✨ Demo Complete! ✨

Key Features Demonstrated:
  ✓ Parallel container spawning
  ✓ Isolated git worktrees per container
  ✓ Real-time I/O streaming
  ✓ Container lifecycle management
  ✓ Docker + PackNplay integration
```

## How It Works

1. **Setup**: Creates a PackNplay launcher with project path
2. **Spawn**: Launches 5 busybox containers in parallel, each with:
   - Unique role identifier
   - Isolated git worktree
   - Custom racing script that simulates work
3. **Stream**: Captures and displays real-time stdout from all containers with color coding
4. **Track**: Records start/end times for each container
5. **Cleanup**: Gracefully stops all containers
6. **Display**: Shows race results, timing stats, and worktree locations

## Customization

Want to modify the race? Edit `main.go`:

```go
// Change number of racers
racerNames := []string{"🔴 RedRocket", "🟢 GreenMachine", ...}

// Adjust race difficulty
countTo := 15 + (idx * 3)  // Higher = longer race

// Modify container image
Image: "alpine:latest",  // Use different base image

// Custom racing script
script := `your custom bash script here`
```

## Troubleshooting

**"Failed to create launcher: permission denied"**
- Ensure Docker daemon is running
- For Colima users: `export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"`

**"Failed to spawn: cannot connect to Docker daemon"**
- Verify Docker is running: `docker ps`
- Check DOCKER_HOST environment variable

**Containers don't clean up**
```bash
# List PackNplay containers
docker ps -a --filter "label=managed-by=packnplay"

# Clean up manually
docker ps -a --filter "label=managed-by=packnplay" -q | xargs docker rm -f
```

## Performance Tips

For even faster demos:
- Pre-pull the busybox image: `docker pull busybox:latest`
- Use faster storage (SSD)
- Reduce sleep intervals in the racing script

## For Presentations

This demo is perfect for:
- Team demos showing PackNplay integration
- PR walkthroughs
- Architecture presentations
- Testing container orchestration

Run time: ~20-30 seconds
Visual impact: High 🎉
