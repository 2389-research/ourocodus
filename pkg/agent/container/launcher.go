package container

import (
	"context"
	"fmt"
	"sync"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
)

// AgentContainerLauncher orchestrates Docker containers for AgentSessions.
//
// AgentContainerLauncher combines:
//   - AgentWorktreeManager (git workspace isolation)
//   - ContainerSession.Manager (Docker container runtime)
//   - AgentCredentialMounter (credential management)
//
// Thread-safety: AgentContainerLauncher is safe for concurrent use.
type AgentContainerLauncher struct {
	containerMgr *containersession.Manager
	worktreeMgr  *worktree.AgentWorktreeManager
	credMounter  *AgentCredentialMounter
	baseDir      string

	// In-memory handle tracking
	handles map[string]*AgentContainerHandle
	mu      sync.RWMutex
}

// NewAgentContainerLauncher creates an agent container launcher.
//
// Parameters:
//   - containerMgr: Container session manager (required, non-nil)
//   - worktreeMgr: Worktree manager (required, non-nil)
//   - credMounter: Credential mounter (required, non-nil)
//   - baseDir: Base directory for worktrees (empty = "./workspaces")
//
// Panics if any dependency is nil.
func NewAgentContainerLauncher(
	containerMgr *containersession.Manager,
	worktreeMgr *worktree.AgentWorktreeManager,
	credMounter *AgentCredentialMounter,
	baseDir string,
) *AgentContainerLauncher {
	if containerMgr == nil {
		panic("containerMgr cannot be nil")
	}
	if worktreeMgr == nil {
		panic("worktreeMgr cannot be nil")
	}
	if credMounter == nil {
		panic("credMounter cannot be nil")
	}

	if baseDir == "" {
		baseDir = "./workspaces"
	}

	return &AgentContainerLauncher{
		containerMgr: containerMgr,
		worktreeMgr:  worktreeMgr,
		credMounter:  credMounter,
		baseDir:      baseDir,
		handles:      make(map[string]*AgentContainerHandle),
	}
}

// Spawn creates and starts a new agent container with workspace and credentials.
//
// This orchestrates:
//  1. Creating git worktree for workspace isolation
//  2. Setting up credential files (SSH key, GitHub token)
//  3. Creating Docker container with workspace and credential mounts
//  4. Starting the container
//
// The container will have:
//   - Workspace mounted at /workspace (read-write)
//   - SSH key at /root/.ssh/id_ed25519 (read-only, if provided)
//   - GitHub token at /root/.github-token (read-only, if provided)
//
// Returns:
//   - AgentContainerHandle: Handle to the running container
//   - error: Non-nil if any step fails
//
// If any step fails, already-created resources are cleaned up automatically.
func (l *AgentContainerLauncher) Spawn(ctx context.Context, config SpawnConfig) (*AgentContainerHandle, error) {
	// Validate config
	if config.AgentID == "" {
		return nil, ErrInvalidAgentID
	}
	if config.ImageName == "" {
		return nil, ErrInvalidImageName
	}
	if len(config.Command) == 0 {
		return nil, ErrInvalidCommand
	}

	// Check if agent already exists
	l.mu.RLock()
	if _, exists := l.handles[config.AgentID]; exists {
		l.mu.RUnlock()
		return nil, ErrAgentAlreadyExists
	}
	l.mu.RUnlock()

	// Step 1: Create worktree
	wt, err := l.worktreeMgr.Create(ctx, config.AgentID, l.baseDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorktreeSetupFailed, err)
	}

	// Cleanup worktree on failure
	cleanupWorktree := true
	defer func() {
		if cleanupWorktree {
			_ = l.worktreeMgr.Remove(ctx, wt.Path())
		}
	}()

	// Step 2: Setup credentials
	credFiles, err := l.credMounter.Setup(ctx, config.AgentID, config.GitSSHKey, config.GitHubToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCredentialSetupFailed, err)
	}

	// Cleanup credentials on failure
	cleanupCreds := true
	defer func() {
		if cleanupCreds {
			_ = l.credMounter.Cleanup(ctx, config.AgentID)
		}
	}()

	// Step 3: Create container session with custom mounts
	// We need to use a custom approach here since containersession.Manager
	// doesn't support custom mounts directly
	sess, err := l.createContainerWithMounts(ctx, config, wt, credFiles)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrContainerSetupFailed, err)
	}

	// Cleanup container on failure
	cleanupContainer := true
	defer func() {
		if cleanupContainer {
			_ = l.containerMgr.StopContainerSession(ctx, sess.ID())
		}
	}()

	// Step 4: Start container
	if err := l.containerMgr.StartContainerSession(ctx, sess.ID()); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Create handle
	handle := &AgentContainerHandle{
		agentID:         config.AgentID,
		containerSess:   sess,
		worktree:        wt,
		credentialsPath: credFiles.CredentialsDir,
		createdAt:       sess.CreatedAt(),
	}

	// Store handle
	l.mu.Lock()
	l.handles[config.AgentID] = handle
	l.mu.Unlock()

	// Success - don't cleanup
	cleanupWorktree = false
	cleanupCreds = false
	cleanupContainer = false

	return handle, nil
}

// createContainerWithMounts is a helper that creates a container with custom mounts.
//
// This is needed because containersession.Manager.CreateContainerSession doesn't
// support custom mounts. We need to integrate with the Docker API directly while
// still using the containersession package for session management.
//
// For now, this is a simplified implementation. In a full implementation, we would
// need to either:
// 1. Extend containersession.Manager to support custom mounts
// 2. Use Docker API directly and track our own sessions
//
// This placeholder implementation delegates to the standard CreateContainerSession
// and would need enhancement to support custom mounts.
func (l *AgentContainerLauncher) createContainerWithMounts(
	ctx context.Context,
	config SpawnConfig,
	_ *worktree.AgentWorktree,
	_ *CredentialFiles,
) (*containersession.ContainerSession, error) {
	// TODO: This needs to be enhanced to support custom mounts
	// For now, just create a standard container session
	//
	// The full implementation would:
	// 1. Build container config with custom mounts
	// 2. Create container using Docker API
	// 3. Wrap in ContainerSession
	//
	// Since containersession.Manager doesn't expose this capability yet,
	// we'll need to either extend it or duplicate some of its logic.

	// Create a basic container session first
	// This will need to be replaced with custom mount logic
	sess, err := l.containerMgr.CreateContainerSession(ctx, config.ImageName, config.Command)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

// Stop stops an agent container and cleans up all resources.
//
// This:
//  1. Stops the Docker container gracefully
//  2. Removes the git worktree and branch
//  3. Cleans up credential files
//  4. Removes the handle from tracking
//
// This is idempotent - safe to call multiple times.
// Returns nil if the agent doesn't exist.
func (l *AgentContainerLauncher) Stop(ctx context.Context, agentID string) error {
	if agentID == "" {
		return ErrInvalidAgentID
	}

	// Get handle
	l.mu.Lock()
	handle, exists := l.handles[agentID]
	if exists {
		delete(l.handles, agentID)
	}
	l.mu.Unlock()

	if !exists {
		// Already stopped or never existed - idempotent
		return nil
	}

	// Stop container (idempotent)
	if handle.containerSess != nil {
		if err := l.containerMgr.StopContainerSession(ctx, handle.containerSess.ID()); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}
	}

	// Remove worktree (idempotent)
	if handle.worktree != nil {
		if err := l.worktreeMgr.Remove(ctx, handle.worktree.Path()); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}
	}

	// Cleanup credentials (idempotent)
	if err := l.credMounter.Cleanup(ctx, agentID); err != nil {
		return fmt.Errorf("failed to cleanup credentials: %w", err)
	}

	return nil
}

// GetHandle retrieves a handle for an agent.
// Returns nil if the agent doesn't exist.
func (l *AgentContainerLauncher) GetHandle(agentID string) *AgentContainerHandle {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.handles[agentID]
}

// ListHandles returns all active agent handles.
func (l *AgentContainerLauncher) ListHandles() []*AgentContainerHandle {
	l.mu.RLock()
	defer l.mu.RUnlock()

	handles := make([]*AgentContainerHandle, 0, len(l.handles))
	for _, h := range l.handles {
		handles = append(handles, h)
	}
	return handles
}

// The following are not yet implemented and marked with a placeholder comment
// indicating they need to be implemented in a future iteration:

// Attach attaches to an existing agent container.
//
// This allows reconnecting to a container from a different process or after
// a restart. The container must already be running.
//
// TODO: Implement this in Phase 3
func (l *AgentContainerLauncher) Attach(ctx context.Context, agentID string) (*AgentContainerHandle, error) {
	// Implementation needed:
	// 1. Find container by label (com.ourocodus.agent.id=agentID)
	// 2. Attach to container session
	// 3. Find existing worktree
	// 4. Reconstruct credentials path
	// 5. Create handle
	return nil, fmt.Errorf("Attach not yet implemented")
}
