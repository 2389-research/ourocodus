package session

import "context"

// NoOpCleaner implements Cleaner interface with no-op behavior
// Used in Phase 1 where cleanup logic is handled elsewhere
// Future phases can provide real implementations that close connections,
// kill processes, and remove worktrees
type NoOpCleaner struct{}

// NewNoOpCleaner creates a no-op cleaner
func NewNoOpCleaner() *NoOpCleaner {
	return &NoOpCleaner{}
}

// Cleanup does nothing (no-op implementation)
// Always returns nil to indicate success
// Idempotent - safe to call multiple times
func (c *NoOpCleaner) Cleanup(ctx context.Context, session *UserSession) error {
	// Phase 1: No cleanup operations needed
	// Future: Close WebSocket, terminate all ACP processes
	return nil
}
