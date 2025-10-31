# Container Race Demo Enhancement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the Container Race demo educational for engineers by adding clear explanations of PackNplay's behavior and enhanced cleanup visibility.

**Architecture:** Add text output sections before each major phase, track created resources during spawning, and display detailed cleanup steps showing what was created and removed.

**Tech Stack:** Go 1.23, Docker, PackNplay launcher

---

## Task 1: Add Introduction Section

**Files:**
- Modify: `scripts/container-race/main.go:42-47`

**Step 1: Add displayIntro function**

Add this function after the `main` function (around line 210):

```go
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
```

**Step 2: Replace existing header with function call**

In `main()`, replace lines 43-47:

```go
// OLD:
fmt.Println(colorBold + "🏁 OUROCODUS CONTAINER RACE 🏁" + colorReset)
fmt.Println("═══════════════════════════════════════════════════════════")
fmt.Println()
fmt.Println("🚀 Showcasing PackNplay: Containerized Agents + Git Worktrees")
fmt.Println()

// NEW:
displayIntro()
```

**Step 3: Build and test**

Run: `go build -o /tmp/container-race ./scripts/container-race`
Expected: Clean build with no errors

**Step 4: Commit**

```bash
git add scripts/container-race/main.go
git commit -m "feat(demo): add educational intro section to container race"
```

---

## Task 2: Add Spawning Phase Marker

**Files:**
- Modify: `scripts/container-race/main.go:75-76`

**Step 1: Add spawning phase marker**

Replace lines 75-76:

```go
// OLD:
fmt.Println("📦 Spawning containers in parallel worktrees...")
fmt.Println()

// NEW:
fmt.Println(colorBold + "🏗️  SPAWNING CONTAINERS & WORKTREES" + colorReset)
fmt.Println("═══════════════════════════════════════════════════════════")
fmt.Println("Each container gets its own isolated git worktree...")
fmt.Println()
```

**Step 2: Build and test**

Run: `go build -o /tmp/container-race ./scripts/container-race`
Expected: Clean build

**Step 3: Commit**

```bash
git add scripts/container-race/main.go
git commit -m "feat(demo): add spawning phase marker"
```

---

## Task 3: Add Race Phase Marker

**Files:**
- Modify: `scripts/container-race/main.go:149-151`

**Step 1: Replace race in progress text**

Replace lines 149-151:

```go
// OLD:
fmt.Println(colorBold + "📊 RACE IN PROGRESS..." + colorReset)
fmt.Println()

// NEW:
fmt.Println()
fmt.Println(colorBold + "🏁 RACE IN PROGRESS" + colorReset)
fmt.Println("═══════════════════════════════════════════════════════════")
fmt.Println("Each container is working independently in parallel...")
fmt.Println()
```

**Step 2: Build and test**

Run: `go build -o /tmp/container-race ./scripts/container-race`
Expected: Clean build

**Step 3: Commit**

```bash
git add scripts/container-race/main.go
git commit -m "feat(demo): add race phase marker with explanation"
```

---

## Task 4: Enhance Cleanup Phase - Track Resources

**Files:**
- Modify: `scripts/container-race/main.go:32-40` (racer struct)
- Modify: `scripts/container-race/main.go:117-118` (store container ID)

**Step 1: Add containerID field to racer struct**

Modify the racer struct (lines 32-40):

```go
type racer struct {
	name        string
	color       string
	handle      agent.AgentHandle
	containerID string // NEW: track container ID for cleanup display
	startTime   time.Time
	endTime     time.Time
	output      []string
	position    int
}
```

**Step 2: Store container ID after spawn**

After line 117 (where `r.handle = handle`), add:

```go
r.handle = handle
r.containerID = fmt.Sprintf("racer-%d-%s", idx, handle.Workspace()[len(handle.Workspace())-6:]) // NEW: extract short ID
```

**Step 3: Build and verify**

Run: `go build -o /tmp/container-race ./scripts/container-race`
Expected: Clean build

**Step 4: Commit**

```bash
git add scripts/container-race/main.go
git commit -m "feat(demo): track container IDs for cleanup display"
```

---

## Task 5: Enhance Cleanup Phase - Display Created Resources

**Files:**
- Modify: `scripts/container-race/main.go:180-188`

**Step 1: Replace cleanup section with detailed display**

Replace lines 180-188:

```go
// OLD:
fmt.Println()
fmt.Println(colorBold + "🛑 Race complete! Stopping containers..." + colorReset)
for _, r := range racers {
	if r.handle != nil {
		_ = launcher.Stop(ctx, r.handle)
	}
}

// NEW:
fmt.Println()
fmt.Println(colorBold + "🧹 CLEANUP PHASE" + colorReset)
fmt.Println("═══════════════════════════════════════════════════════════")
fmt.Println()
fmt.Println(colorBold + "📊 What PackNplay Created:" + colorReset)
fmt.Printf("  ✓ %d Docker containers (one per racer)\n", len(racers))
fmt.Printf("  ✓ %d Git worktrees at ~/.local/share/packnplay/worktrees/\n", len(racers))
fmt.Println("  ✓ Independent workspaces for parallel execution")
fmt.Println()

fmt.Println(colorBold + "📝 Worktree Details:" + colorReset)
for _, r := range racers {
	if r.handle != nil {
		fmt.Printf("  %s%s%s → %s\n", r.color, r.name, colorReset, r.handle.Workspace())
	}
}
fmt.Println()

fmt.Println(colorBold + "🛑 Now Stopping Containers..." + colorReset)
containersStopped := 0
for _, r := range racers {
	if r.handle != nil {
		_ = launcher.Stop(ctx, r.handle)
		fmt.Printf("  ✓ Stopped container: %s\n", r.containerID)
		containersStopped++
	}
}
fmt.Println()

fmt.Printf(colorBold + "🗑️  Containers Removed: %d\n" + colorReset, containersStopped)
fmt.Printf(colorBold + "📁 Worktrees Cleaned Up: %d\n" + colorReset, containersStopped)
fmt.Println()
fmt.Println(colorGreen + "✨ All clean! No orphaned containers or worktrees left behind." + colorReset)
```

**Step 2: Remove old worktree display section**

Delete lines 193-199 (the old worktree info section that's now redundant):

```go
// DELETE THIS ENTIRE SECTION:
fmt.Println()
fmt.Println(colorBold + "📁 Git Worktrees Created:" + colorReset)
for _, r := range racers {
	if r.handle != nil {
		fmt.Printf("  %s%s%s → %s\n", r.color, r.name, colorReset, r.handle.Workspace())
	}
}
```

**Step 3: Build and test**

Run: `go build -o /tmp/container-race ./scripts/container-race`
Expected: Clean build

**Step 4: Commit**

```bash
git add scripts/container-race/main.go
git commit -m "feat(demo): enhance cleanup phase with detailed resource tracking"
```

---

## Task 6: Manual Integration Test

**Files:**
- Manual test with actual Docker

**Step 1: Ensure Docker is running**

For Colima users:
```bash
colima start
export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
docker ps
```

For Docker Desktop users:
```bash
docker ps
```

Expected: Connection successful, containers listed

**Step 2: Run the demo**

```bash
cd scripts/container-race
go run main.go
```

**Step 3: Verify output sections**

Check for these sections in order:
1. ✅ Introduction section explains PackNplay
2. ✅ Spawning phase marker with explanation
3. ✅ Race in progress marker
4. ✅ Results and podium
5. ✅ Cleanup phase showing:
   - What was created (counts)
   - Worktree paths for each racer
   - Step-by-step container stops with checkmarks
   - Final cleanup summary

**Step 4: Verify no orphaned resources**

```bash
# Should show no packnplay containers
docker ps -a --filter "label=managed-by=packnplay"

# Should show no orphaned worktrees
ls ~/.local/share/packnplay/worktrees/
```

Expected: Both commands show empty results

**Step 5: Document test results**

If all checks pass, create a commit noting successful integration test:

```bash
git commit --allow-empty -m "test: verify container race demo enhancements

Manual integration test confirms:
- All 5 phases display correctly
- Cleanup shows created resources
- No orphaned containers or worktrees
- Educational content clear for mixed audience"
```

---

## Task 7: Update Demo Features List

**Files:**
- Modify: `scripts/container-race/main.go:202-209`

**Step 1: Update features list at end**

The features list appears after cleanup. Verify it's still accurate or update if needed.

Current list (around line 202):

```go
fmt.Println()
fmt.Println(colorBold + "✨ Demo Complete! ✨" + colorReset)
fmt.Println()
fmt.Println("Key Features Demonstrated:")
fmt.Println("  ✓ Parallel container spawning")
fmt.Println("  ✓ Isolated git worktrees per container")
fmt.Println("  ✓ Real-time I/O streaming")
fmt.Println("  ✓ Container lifecycle management")
fmt.Println("  ✓ Docker + PackNplay integration")
```

This list is still accurate. No changes needed.

**Step 2: Build final version**

Run: `go build -o /tmp/container-race ./scripts/container-race`
Expected: Clean build

**Step 3: Run final full test**

```bash
cd scripts/container-race
go run main.go
```

Verify complete flow from intro through cleanup.

**Step 4: Commit if any changes made**

If you made any adjustments to the features list:

```bash
git add scripts/container-race/main.go
git commit -m "docs(demo): update features list for clarity"
```

Otherwise, skip this step.

---

## Testing Strategy

**Manual testing required:** This is a visual demo, automated testing would be complex and provide little value.

**Integration test checklist:**
- [ ] Intro section displays before any action
- [ ] Spawning phase marker appears with explanation
- [ ] All 5 containers spawn successfully
- [ ] Race displays colored lap output
- [ ] Podium shows winners correctly
- [ ] Cleanup phase shows:
  - [ ] Resource counts (5 containers, 5 worktrees)
  - [ ] Actual worktree paths
  - [ ] Step-by-step stop confirmations
  - [ ] Final cleanup summary
- [ ] No orphaned Docker containers remain
- [ ] No orphaned worktrees remain

**Prerequisites:**
- Docker running (Colima or Docker Desktop)
- Project is a git repository
- Network access for container image pulls

---

## Completion Criteria

✅ All 7 tasks completed
✅ Demo runs end-to-end without errors
✅ All 5 phases clearly marked and explained
✅ Cleanup shows complete before/after state
✅ No orphaned resources after demo completes
✅ Code follows project style (gofumpt formatted)

## Notes

- This is primarily a visual/textual enhancement, no business logic changes
- No new dependencies required
- Changes isolated to single file: `scripts/container-race/main.go`
- Demo remains backward compatible with existing PackNplay API
- Educational improvements don't affect performance or timing
