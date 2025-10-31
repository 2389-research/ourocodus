# Container Race Demo Enhancements

**Date:** 2025-10-31
**Status:** Design Approved
**Target Audience:** Engineering team (mixed Docker/Git expertise)

## Problem Statement

The Container Race demo showcases PackNplay's capabilities but fails to make clear what happens technically. Engineers watching the demo cannot discern:

- What PackNplay does
- What work the containers perform
- Whether cleanup occurs
- Where artifacts (worktrees, containers) exist

## Goals

Transform the demo from visual entertainment into educational demonstration. Engineers should understand:

1. PackNplay's purpose and value proposition
2. The container lifecycle (spawn → run → stop → cleanup)
3. Git worktree isolation per container
4. Real-time I/O streaming from multiple containers
5. Complete cleanup with no orphaned resources

## Design

### Five-Phase Structure

Organize the demo into distinct phases with visual milestone markers:

**Phase 1: Introduction (NEW)**
- Brief PackNplay explanation (2-3 lines)
- Preview what the demo demonstrates
- Set context before the action

**Phase 2: Spawning**
- Visual marker: `🏗️ SPAWNING CONTAINERS & WORKTREES`
- Show 5 racers being prepared
- Explain: "Each container gets its own isolated git worktree"
- Keep existing countdown (3-2-1-GO!)

**Phase 3: Race**
- Visual marker: `🏁 RACE IN PROGRESS`
- Keep existing colored lap output
- Add brief note: "Each container works independently in parallel"

**Phase 4: Results**
- Keep existing podium and winner display
- Show finish times

**Phase 5: Cleanup (ENHANCED)**
- Visual marker: `🧹 CLEANUP PHASE`
- Show what PackNplay created (counts and paths)
- Display step-by-step container stops with checkmarks
- Show final cleanup summary

### Introduction Section

```
🏁 OUROCODUS CONTAINER RACE 🏁
═══════════════════════════════════════════════════════════

📦 What is PackNplay?
PackNplay spawns containerized agents with isolated git worktrees.
Each agent gets its own Docker container + its own git workspace.
Perfect for parallel execution without conflicts!

🎯 This Demo Shows:
  • Spawning 5 containers in parallel
  • Each with isolated git worktree
  • Real-time I/O streaming from all containers
  • Complete lifecycle: spawn → run → stop → cleanup

🏎️ The Race:
Each container counts through "laps" at different speeds.
First to finish wins!
```

### Enhanced Cleanup Phase

```
🧹 CLEANUP PHASE
═══════════════════════════════════════════════════════════

📊 What PackNplay Created:
  ✓ 5 Docker containers (one per racer)
  ✓ 5 Git worktrees at ~/.local/share/packnplay/worktrees/
  ✓ Independent workspaces for parallel execution

📝 Worktree Details:
  🔴 RedRocket → /Users/you/.local/share/packnplay/worktrees/racer-0-abc123
  🟢 GreenMachine → /Users/you/.local/share/packnplay/worktrees/racer-1-def456
  🟡 YellowFlash → /Users/you/.local/share/packnplay/worktrees/racer-2-ghi789
  🔵 BlueBlaze → /Users/you/.local/share/packnplay/worktrees/racer-3-jkl012
  🟣 PurplePower → /Users/you/.local/share/packnplay/worktrees/racer-4-mno345

🛑 Now Stopping Containers...
  ✓ Stopped container: racer-0-abc123
  ✓ Stopped container: racer-1-def456
  ✓ Stopped container: racer-2-ghi789
  ✓ Stopped container: racer-3-jkl012
  ✓ Stopped container: racer-4-mno345

🗑️ Containers Removed: 5
📁 Worktrees Cleaned Up: 5

✨ All clean! No orphaned containers or worktrees left behind.
```

## Implementation Notes

### Code Changes

1. **Add intro section** before existing startup code
2. **Add phase markers** before each major section
3. **Enhance cleanup** to show:
   - Count of created resources
   - Actual worktree paths (use `handle.Workspace()`)
   - Step-by-step stop confirmations
   - Final cleanup counts

### What Stays the Same

- Colored output and racer names
- Parallel spawning logic
- Countdown timer
- Race visualization
- Podium display
- Worktree display at end (move to cleanup phase)

### Technical Details

- Container IDs available from `handle` (implementation detail)
- Worktree paths from `handle.Workspace()`
- Stop operations already exist at lines 181-187
- Add output between stop calls to show progress

## Success Criteria

Engineers watching the demo can answer:

- What does PackNplay do? (Containerized agents + isolated worktrees)
- What work did containers perform? (Counted to different numbers)
- Did cleanup occur? (Yes - saw 5 stops, 5 worktrees removed)
- Where were artifacts? (Saw actual paths to worktrees)

## Non-Goals

- Change the race mechanics or timing
- Add verifiable commands (docker ps, git worktree list)
- Make the work "realistic" (counting is fine with context)
- Support different Docker implementations (Colima detection already exists)
