package worktree

import "errors"

// Sentinel errors for common failure cases.
var (
	// ErrWorktreeNotFound indicates the specified worktree does not exist.
	ErrWorktreeNotFound = errors.New("worktree not found")

	// ErrWorktreeAlreadyExists indicates a worktree already exists at the specified path.
	ErrWorktreeAlreadyExists = errors.New("worktree already exists")

	// ErrBranchAlreadyExists indicates a branch with the same name already exists.
	ErrBranchAlreadyExists = errors.New("branch already exists")

	// ErrInvalidRepository indicates the repository is nil or invalid.
	ErrInvalidRepository = errors.New("invalid repository")

	// ErrInvalidAgentID indicates the agentID is empty or invalid.
	ErrInvalidAgentID = errors.New("invalid agent ID")

	// ErrInvalidPath indicates the path is empty or invalid.
	ErrInvalidPath = errors.New("invalid path")
)
