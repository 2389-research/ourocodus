// Package session provides heartbeat monitoring for agent liveness detection.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/2389-research/ourocodus/pkg/heartbeat"
	"github.com/nats-io/nats.go"
)

const (
	// ReaperInterval is how often the reaper checks for expired leases
	ReaperInterval = 1 * time.Minute
)

// HeartbeatMonitor monitors agent heartbeats and manages lease lifecycle
type HeartbeatMonitor struct {
	nats     *nats.Conn
	sub      *nats.Subscription
	lastSeen map[string]time.Time
	mu       sync.RWMutex
	logger   atomic.Value // *log.Logger
	stopOnce sync.Once
}

// NewHeartbeatMonitor creates a new heartbeat monitor.
//
// The connection is configured with automatic reconnection to handle network partitions:
//   - Unlimited reconnection attempts
//   - 2-second wait between reconnect attempts
//   - Logging on successful reconnection
//
// Parameters:
//   - natsURL: The NATS server URL (e.g., "nats://localhost:4222")
//
// Returns an error if the initial NATS connection fails.
func NewHeartbeatMonitor(natsURL string) (*HeartbeatMonitor, error) {
	// Create monitor instance first to use logger in reconnect handler
	monitor := &HeartbeatMonitor{
		lastSeen: make(map[string]time.Time),
	}
	monitor.logger.Store(log.Default())

	// Connect with automatic reconnection support
	// Note: We don't use RetryOnFailedConnect to fail fast on initial connection errors
	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(-1), // Unlimited reconnection attempts after initial connection
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			monitor.getLogger().Printf("Reconnected to NATS server")
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				monitor.getLogger().Printf("Disconnected from NATS: %v", err)
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	monitor.nats = nc
	return monitor, nil
}

// Start begins monitoring heartbeats and automatically renewing leases.
// This method spawns goroutines for:
//  1. Subscribing to heartbeat messages and renewing leases
//  2. Reaping expired leases periodically
//
// The monitor continues until the context is cancelled.
func (h *HeartbeatMonitor) Start(ctx context.Context) error {
	// Subscribe to all agent heartbeats
	sub, err := h.nats.Subscribe(heartbeat.SubjectPattern, func(msg *nats.Msg) {
		var hb heartbeat.Message

		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			h.getLogger().Printf("Failed to unmarshal heartbeat: %v", err)
			return
		}

		h.updateLastSeen(hb.AgentID, hb.Timestamp)
		h.renewLeaseIfAttached(hb.AgentID)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to heartbeats: %w", err)
	}
	h.sub = sub

	// Start background reaper
	go h.reapExpiredLeases(ctx)

	return nil
}

// updateLastSeen records the last time a heartbeat was received from an agent
func (h *HeartbeatMonitor) updateLastSeen(agentID string, timestamp time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSeen[agentID] = timestamp
}

// GetLastSeen returns the last seen timestamp for an agent.
// Returns zero time if the agent has never been seen.
func (h *HeartbeatMonitor) GetLastSeen(agentID string) time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastSeen[agentID]
}

// renewLeaseIfAttached renews the lease for an agent if it has an active lease.
// This is called automatically when a heartbeat is received.
//
// If the agent is not attached (no lease exists), this is a no-op.
// IO errors or malformed lease files are logged.
func (h *HeartbeatMonitor) renewLeaseIfAttached(agentID string) {
	// Check if lease exists (agent is attached)
	_, err := ReadLease(agentID)
	if err != nil {
		if errors.Is(err, ErrLeaseNotFound) {
			// No lease = agent is detached, nothing to renew
			return
		}
		// Log unexpected errors (IO issues, malformed JSON, etc.)
		h.getLogger().Printf("Failed to read lease for agent %s: %v", agentID, err)
		return
	}

	// Renew lease to extend expiration
	if err := RenewLease(agentID); err != nil {
		h.getLogger().Printf("Failed to renew lease for agent %s: %v", agentID, err)
	}
}

// reapExpiredLeases runs periodically to clean up expired leases.
// This goroutine runs until the context is cancelled.
//
// Expired leases indicate that an agent has stopped sending heartbeats
// (likely crashed or terminated) and should be detached.
func (h *HeartbeatMonitor) reapExpiredLeases(ctx context.Context) {
	ticker := time.NewTicker(ReaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.removeExpiredLeases()
		case <-ctx.Done():
			return
		}
	}
}

// removeExpiredLeases scans all leases and removes any that have expired.
// This is called periodically by the reaper goroutine.
//
// In addition to removing expired lease files, this also prunes the lastSeen map to prevent
// memory leaks from agents that no longer exist or have been detached.
func (h *HeartbeatMonitor) removeExpiredLeases() {
	leases, err := ListLeases()
	if err != nil {
		h.getLogger().Printf("Failed to list leases: %v", err)
		return
	}

	// Build set of active agent IDs from current leases
	activeAgents := make(map[string]bool)
	for _, lease := range leases {
		activeAgents[lease.AgentID] = true

		// Remove expired leases
		if IsLeaseExpired(lease) {
			h.getLogger().Printf("Reaping expired lease for agent %s (attached to session %s)",
				lease.AgentID, lease.UserSessionID)

			if err := ReleaseLease(lease.AgentID); err != nil {
				h.getLogger().Printf("Failed to release expired lease for agent %s: %v",
					lease.AgentID, err)
			} else {
				// Successfully released, remove from active set
				delete(activeAgents, lease.AgentID)
			}
		}
	}

	// Prune lastSeen entries for agents that no longer have leases
	// Also prune entries that haven't been seen in 2x the lease TTL (10 minutes)
	h.mu.Lock()
	defer h.mu.Unlock()

	staleThreshold := time.Now().Add(-2 * LeaseTTL) // 10 minutes ago
	for agentID, lastSeen := range h.lastSeen {
		// Remove if agent has no lease or hasn't been seen in 10 minutes
		if !activeAgents[agentID] || lastSeen.Before(staleThreshold) {
			delete(h.lastSeen, agentID)
		}
	}
}

// Stop gracefully shuts down the heartbeat monitor.
// It unsubscribes from NATS and closes the connection.
// This method is safe to call multiple times.
//
// The caller should cancel the context passed to Start() to stop the
// background reaper goroutine.
func (h *HeartbeatMonitor) Stop() {
	h.stopOnce.Do(func() {
		if h.sub != nil {
			if err := h.sub.Unsubscribe(); err != nil {
				h.getLogger().Printf("Failed to unsubscribe from heartbeats: %v", err)
			}
		}
		if h.nats != nil {
			h.nats.Close()
		}
	})
}

// getLogger returns the current logger in a thread-safe manner
func (h *HeartbeatMonitor) getLogger() *log.Logger {
	return h.logger.Load().(*log.Logger)
}

// SetLogger sets a custom logger for the heartbeat monitor.
// This is useful for testing or custom logging configurations.
// This method is safe to call concurrently with Start().
//
// If logger is nil, this method panics.
func (h *HeartbeatMonitor) SetLogger(logger *log.Logger) {
	if logger == nil {
		panic("logger cannot be nil")
	}
	h.logger.Store(logger)
}
