// Package session implements agent session management including lease-based attachment.
//
// # Lease System Architecture
//
// The lease system provides exclusive agent attachment using file-based locking with O_EXCL.
// This ensures atomic lease acquisition and prevents race conditions when multiple sessions
// attempt to attach to the same agent.
//
// # Single-Host Limitation
//
// IMPORTANT: This lease implementation is designed for single-host deployments only.
// The file-based locking mechanism relies on the operating system's atomic file creation
// guarantees (O_EXCL flag), which only work within a single filesystem namespace.
//
// Limitations:
//   - Does NOT work across multiple relay servers (distributed deployment)
//   - Does NOT work with network file systems (NFS, CIFS, etc.) - these filesystems
//     may not properly support O_EXCL semantics
//   - Does NOT provide distributed consensus
//
// For multi-host deployments, you will need to replace this implementation with a
// distributed coordination system such as:
//   - etcd with distributed locks
//   - Redis with RedLock algorithm
//   - ZooKeeper with ephemeral nodes
//   - Database-backed pessimistic locking
//
// The current implementation is intentionally simple and suitable for Phase 1 (single-host)
// deployments. Migration to distributed locking is planned for Phase 2.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Logger interface for optional lease operation logging
type LeaseLogger interface {
	Printf(format string, v ...interface{})
}

// DefaultLogger can be set to enable logging for lease operations
// If nil, logging is disabled
var DefaultLogger LeaseLogger

// LeaseDir is the directory where lease files are stored
// Can be overridden via OUROCODUS_LEASE_DIR environment variable
var LeaseDir = getLeaseDir()

// getLeaseDir returns the lease directory path, checking environment variable first
func getLeaseDir() string {
	if dir := os.Getenv("OUROCODUS_LEASE_DIR"); dir != "" {
		return dir
	}
	// Default to user's home directory
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".agentd", "session")
	}
	// Fallback to relative path (for tests or when HOME not set)
	return ".agentd/session"
}

const (
	// LeaseTTL is the duration before a lease expires
	LeaseTTL = 5 * time.Minute

	// HeartbeatInterval is how often agents should send heartbeats
	HeartbeatInterval = 30 * time.Second

	// MaxLeaseRetries is the maximum number of times to retry acquiring a lease
	// when encountering expired leases
	MaxLeaseRetries = 3
)

var (
	// ErrAlreadyAttached is returned when trying to acquire a lease for an already-attached agent
	ErrAlreadyAttached = fmt.Errorf("agent already attached to another session")

	// ErrLeaseNotFound is returned when reading a lease that doesn't exist
	ErrLeaseNotFound = fmt.Errorf("lease not found")

	// ErrLeaseExpired is returned when a lease has expired
	ErrLeaseExpired = fmt.Errorf("lease has expired")

	// ErrInvalidAgentID is returned when agentID contains invalid characters
	ErrInvalidAgentID = fmt.Errorf("invalid agent ID")
)

// Lease represents an agent attachment lease.
type Lease struct {
	AgentID           string    `json:"agentId"`
	UserSessionID     string    `json:"userSessionId"`
	AttachedAt        time.Time `json:"attachedAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
	HeartbeatInterval string    `json:"heartbeatInterval"`
}

// validateAgentID checks if agentID is safe to use in file paths
// Rejects: empty strings, path traversal attempts, absolute paths, special characters
func validateAgentID(agentID string) error {
	if agentID == "" {
		return ErrInvalidAgentID
	}

	// Reject path traversal attempts
	if filepath.Base(agentID) != agentID {
		return ErrInvalidAgentID
	}

	// Reject absolute paths
	if filepath.IsAbs(agentID) {
		return ErrInvalidAgentID
	}

	// Additional check: ensure the cleaned path equals the original
	cleaned := filepath.Clean(agentID)
	if cleaned != agentID {
		return ErrInvalidAgentID
	}

	return nil
}

// AcquireLease atomically creates a lease file for the given agent.
// Returns ErrAlreadyAttached if lease already exists and is not expired.
func AcquireLease(agentID, userSessionID string) (*Lease, error) {
	return acquireLeaseWithRetry(agentID, userSessionID, 0)
}

// acquireLeaseWithRetry is the internal implementation with retry counting
func acquireLeaseWithRetry(agentID, userSessionID string, retryCount int) (*Lease, error) {
	// Validate agentID to prevent path traversal
	if err := validateAgentID(agentID); err != nil {
		return nil, err
	}

	// Ensure lease directory exists
	if err := os.MkdirAll(LeaseDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create lease directory: %w", err)
	}

	leasePath := filepath.Join(LeaseDir, agentID+".lease")

	// O_EXCL ensures atomic creation (fails if exists)
	f, err := os.OpenFile(leasePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			// Check if existing lease is expired
			if existing, err := ReadLease(agentID); err == nil {
				if IsLeaseExpired(existing) {
					// Prevent infinite retry loop
					if retryCount >= MaxLeaseRetries {
						if DefaultLogger != nil {
							DefaultLogger.Printf("[LEASE] Max retries (%d) exceeded acquiring lease for agent %s", MaxLeaseRetries, agentID)
						}
						return nil, fmt.Errorf("max retries exceeded acquiring expired lease for agent %s", agentID)
					}

					// Log lease takeover
					if DefaultLogger != nil {
						DefaultLogger.Printf("[LEASE] Reclaiming expired lease for agent %s (was owned by session %s, expired at %s)",
							agentID, existing.UserSessionID, existing.ExpiresAt.Format(time.RFC3339))
					}

					// Expired lease, force release and retry with backoff
					_ = ReleaseLease(agentID)
					time.Sleep(time.Duration(retryCount+1) * 50 * time.Millisecond) // Exponential backoff
					return acquireLeaseWithRetry(agentID, userSessionID, retryCount+1)
				}
				// Lease is still valid
				if DefaultLogger != nil {
					DefaultLogger.Printf("[LEASE] Agent %s already attached to session %s (expires at %s)",
						agentID, existing.UserSessionID, existing.ExpiresAt.Format(time.RFC3339))
				}
				return nil, ErrAlreadyAttached
			}
			return nil, ErrAlreadyAttached
		}
		return nil, fmt.Errorf("failed to create lease file: %w", err)
	}
	defer func() { _ = f.Close() }()

	lease := &Lease{
		AgentID:           agentID,
		UserSessionID:     userSessionID,
		AttachedAt:        time.Now(),
		ExpiresAt:         time.Now().Add(LeaseTTL),
		HeartbeatInterval: HeartbeatInterval.String(),
	}

	if err := json.NewEncoder(f).Encode(lease); err != nil {
		_ = os.Remove(leasePath) // Cleanup on error
		return nil, fmt.Errorf("failed to write lease: %w", err)
	}

	if DefaultLogger != nil {
		DefaultLogger.Printf("[LEASE] Acquired lease for agent %s by session %s (expires at %s)",
			agentID, userSessionID, lease.ExpiresAt.Format(time.RFC3339))
	}

	return lease, nil
}

// ReleaseLease deletes the lease file for the given agent.
// This operation is idempotent - it succeeds even if the lease doesn't exist.
func ReleaseLease(agentID string) error {
	// Validate agentID to prevent path traversal
	if err := validateAgentID(agentID); err != nil {
		return err
	}

	leasePath := filepath.Join(LeaseDir, agentID+".lease")
	if err := os.Remove(leasePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already released, idempotent
		}
		return fmt.Errorf("failed to remove lease: %w", err)
	}
	return nil
}

// ReadLease reads the lease file for the given agent.
func ReadLease(agentID string) (*Lease, error) {
	// Validate agentID to prevent path traversal
	if err := validateAgentID(agentID); err != nil {
		return nil, err
	}

	leasePath := filepath.Join(LeaseDir, agentID+".lease")
	data, err := os.ReadFile(leasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrLeaseNotFound
		}
		return nil, fmt.Errorf("failed to read lease: %w", err)
	}

	var lease Lease
	if err := json.Unmarshal(data, &lease); err != nil {
		return nil, fmt.Errorf("failed to parse lease: %w", err)
	}

	return &lease, nil
}

// RenewLease extends the expiry time of an existing lease.
func RenewLease(agentID string) error {
	// Validate agentID to prevent path traversal
	if err := validateAgentID(agentID); err != nil {
		return err
	}

	lease, err := ReadLease(agentID)
	if err != nil {
		return err
	}

	// Extend expiry
	lease.ExpiresAt = time.Now().Add(LeaseTTL)

	leasePath := filepath.Join(LeaseDir, agentID+".lease")
	data, err := json.Marshal(lease)
	if err != nil {
		return fmt.Errorf("failed to marshal lease: %w", err)
	}

	if err := os.WriteFile(leasePath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write lease: %w", err)
	}

	return nil
}

// IsLeaseExpired checks if a lease has expired.
func IsLeaseExpired(lease *Lease) bool {
	return time.Now().After(lease.ExpiresAt)
}

// ListLeases returns all lease files in the lease directory.
// Invalid or unreadable leases are skipped.
func ListLeases() ([]*Lease, error) {
	entries, err := os.ReadDir(LeaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Lease{}, nil // No leases yet
		}
		return nil, fmt.Errorf("failed to read lease directory: %w", err)
	}

	var leases []*Lease
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Extract agent ID from filename (remove .lease extension)
		agentID := entry.Name()
		if len(agentID) > 6 && agentID[len(agentID)-6:] == ".lease" {
			agentID = agentID[:len(agentID)-6]
		} else {
			continue // Skip files without .lease extension
		}

		lease, err := ReadLease(agentID)
		if err != nil {
			continue // Skip invalid leases
		}
		leases = append(leases, lease)
	}

	return leases, nil
}

// CleanupExpiredLeases removes all expired lease files from the lease directory.
// Returns the number of leases cleaned up and any error encountered.
// This should be called on server startup to prevent orphaned leases.
func CleanupExpiredLeases() (int, error) {
	leases, err := ListLeases()
	if err != nil {
		return 0, fmt.Errorf("failed to list leases: %w", err)
	}

	cleaned := 0
	for _, lease := range leases {
		if IsLeaseExpired(lease) {
			if err := ReleaseLease(lease.AgentID); err != nil {
				// Log but continue cleaning others
				continue
			}
			cleaned++
		}
	}

	return cleaned, nil
}
