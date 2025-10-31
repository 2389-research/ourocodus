# 🏁 Container Race - Team Demo Guide

Quick reference for presenting the Container Race demo to your team!

## 🎯 What This Demonstrates

In **30 seconds**, this demo showcases everything your team accomplished in the PackNplay PR:

1. ✅ **Docker containerization** - Real busybox containers, not mock agents
2. ✅ **Git worktree management** - Each container in isolated workspace
3. ✅ **Parallel execution** - 5 containers spawning simultaneously
4. ✅ **Live I/O streaming** - Real-time output from all containers
5. ✅ **Container lifecycle** - Spawn → Execute → Stop → Cleanup

## 🚀 Running the Demo

### Pre-Demo Checklist

```bash
# 1. Ensure Docker is running
docker ps

# 2. Pre-pull the image (optional, but faster)
docker pull busybox:latest

# 3. Test run (dry run)
make container-race
```

### During Presentation

```bash
# Just run:
make container-race

# Or with mise:
mise run container-race
```

**Pro tip**: Use a large terminal font (18-20pt) for audience visibility!

## 🎬 Presentation Script

### Opening (5 seconds)
"Let me show you what we built with PackNplay integration..."

### Demo Running (20-30 seconds)
Point out as it runs:
- "Five Docker containers spawning in parallel..."
- "Each gets its own isolated git worktree..."
- "Real-time streaming - you're seeing actual container output..."
- "Watch them race to completion..."

### Conclusion (5 seconds)
- Show the final results and podium
- Point to the worktree paths
- "All managed automatically by PackNplay!"

## 🎨 Visual Highlights to Point Out

1. **Color-coded output** - Each racer has unique color
2. **Parallel spawning** - All containers start simultaneously
3. **Real-time streaming** - Output appears as containers execute
4. **Timing stats** - Actual performance measurements
5. **Podium display** - Fun ASCII art visualization
6. **Worktree paths** - Shows actual git worktree locations

## 🛠️ Technical Details (For Q&A)

### Architecture
- Uses `pkg/agent/packnplay` launcher
- Each container runs `busybox:latest` with custom shell script
- Worktrees created in `.git/worktrees/agent-{ULID}`
- Docker labels: `managed-by=packnplay`

### Performance
- ~2-5 seconds for parallel spawn
- ~15-20 seconds total demo time
- Minimal resource usage (busybox is tiny)

### Code Walkthrough
If asked "How does it work?":

```go
// 1. Create launcher
launcher, _ := packnplay.NewLauncher(
    packnplay.WithProjectPath("."),
)

// 2. Spawn containers in parallel
for _, racer := range racers {
    cfg := &agent.SpawnConfig{
        Role:    "racer-1",
        Image:   "busybox:latest",
        Command: []string{"sh", "-c", script},
    }
    handle, _ := launcher.Spawn(ctx, cfg)
}

// 3. Stream output
scanner := bufio.NewScanner(handle.Stdout())
for scanner.Scan() {
    fmt.Println(scanner.Text())
}

// 4. Wait and cleanup
handle.Wait(ctx)
launcher.Stop(ctx, handle.ID())
```

## 🐛 Troubleshooting During Demo

### "Docker not running"
```bash
# Quick fix
colima start  # or start Docker Desktop
export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
```

### "Containers not starting"
- Check Docker disk space: `docker system df`
- Verify busybox image: `docker images | grep busybox`

### "Output looks messy"
- Increase terminal size
- Use a solid-colored terminal background
- Ensure UTF-8 encoding is enabled

## 💡 Demo Variations

### Quick Version (10 seconds)
Edit `main.go` line 95:
```go
racerNames := []string{"🔴 RedRocket", "🟢 GreenMachine"}  // Just 2 racers
countTo := 5 + (idx * 2)  // Shorter race
```

### Extended Version (60 seconds)
Add more racers, longer scripts, or show container logs.

## 🎤 Key Talking Points

1. **"Before PackNplay"**: Mock agents, no real containers, manual worktree management
2. **"After PackNplay"**: Real Docker containers, automatic worktrees, full lifecycle management
3. **"Developer Experience"**: Single `Spawn()` call handles everything
4. **"Production Ready"**: Proper cleanup, error handling, concurrent execution

## 📊 Metrics to Mention

- **Lines of code**: ~250 for full demo (shows API simplicity)
- **Containers**: 5 simultaneous (shows scalability)
- **Startup time**: <3 seconds (shows performance)
- **Cleanup**: Automatic (shows reliability)

## 🎁 Demo Extras

Want to impress even more?

1. **Live code walkthrough**: Show `main.go` structure
2. **Worktree inspection**: `ls -la .git/worktrees/` after demo
3. **Container inspection**: `docker ps --filter label=managed-by=packnplay`
4. **Performance comparison**: Time before/after PackNplay integration

---

**Remember**: This is a fun demo, but it showcases real production code. The same API powers your actual multi-agent system! 🚀
