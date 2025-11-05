// Package worktree provides git worktree management for AgentSession workspace isolation.
//
// The worktree package enables each AgentSession to work in an isolated git worktree with
// its own branch, preventing conflicts between concurrent agents working on the same repository.
//
// # Core Concepts
//
// An AgentSession requires:
//   - Isolated filesystem workspace (git worktree)
//   - Unique git branch for changes
//   - Clean separation from other AgentSessions
//
// AgentWorktreeManager handles the complete lifecycle:
//   - Creating worktrees with unique branches
//   - Listing active worktrees
//   - Removing worktrees and branches on cleanup
//
// # Example Usage
//
//	repo, _ := git.PlainOpen("/path/to/repo")
//	manager := worktree.NewAgentWorktreeManager(repo)
//
//	// Create worktree for agent session
//	wt, err := manager.Create(ctx, "agent-coder-abc123", "/workspaces")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer manager.Remove(ctx, wt.Path())
//
//	// Agent works in isolated workspace
//	log.Printf("Agent workspace: %s (branch: %s)", wt.Path(), wt.BranchName())
//
// # Relationship to Domain Model
//
//	UserSession (relay WebSocket)
//	    ↓ spawns
//	AgentSession (individual agent process)
//	    ↓ requires
//	AgentWorktree (git worktree + branch)
//	    ↓ filesystem
//	/workspaces/agent-{id}/
//
// Each AgentSession gets its own AgentWorktree, enabling concurrent agents to work
// on the same repository without conflicts.
package worktree
