// Package session provides heartbeat monitoring for agent liveness detection.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	// HeartbeatSubjectPattern is the NATS subject pattern for subscribing to all agent heartbeats
	HeartbeatSubjectPattern = "agent.heartbeat.*"

	// ReaperInterval is how often the reaper checks for expired leases
	ReaperInterval = 1 * time.Minute
)

// HeartbeatMonitor monitors agent heartbeats and manages lease lifecycle
type HeartbeatMonitor struct {
	nats     *nats.Conn
	sub      *nats.Subscription
	lastSeen map[string]time.Time
	mu       sync.RWMutex
	logger   *log.Logger
}

// heartbeatMessage represents the structure of a heartbeat message from an agent
type heartbeatMessage struct {
	AgentID   string    `json:"agentId"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

// NewHeartbeatMonitor creates a new heartbeat monitor.
//
// Parameters:
//   - natsURL: The NATS server URL (e.g., "nats://localhost:4222")
//
// Returns an error if the NATS connection fails.
func NewHeartbeatMonitor(natsURL string) (*HeartbeatMonitor, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &HeartbeatMonitor{
		nats:     nc,
		lastSeen: make(map[string]time.Time),
		logger:   log.Default(),
	}, nil
}

// Start begins monitoring heartbeats and automatically renewing leases.
// This method spawns goroutines for:
//  1. Subscribing to heartbeat messages and renewing leases
//  2. Reaping expired leases periodically
//
// The monitor continues until the context is cancelled.
func (h *HeartbeatMonitor) Start(ctx context.Context) error {
	// Subscribe to all agent heartbeats
	sub, err := h.nats.Subscribe(HeartbeatSubjectPattern, func(msg *nats.Msg) {
		var hb heartbeatMessage

		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			h.logger.Printf("Failed to unmarshal heartbeat: %v", err)
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
// Errors are logged but do not interrupt monitoring.
func (h *HeartbeatMonitor) renewLeaseIfAttached(agentID string) {
	// Check if lease exists (agent is attached)
	_, err := ReadLease(agentID)
	if err != nil {
		// No lease = agent is detached, nothing to renew
		return
	}

	// Renew lease to extend expiration
	if err := RenewLease(agentID); err != nil {
		h.logger.Printf("Failed to renew lease for agent %s: %v", agentID, err)
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
func (h *HeartbeatMonitor) removeExpiredLeases() {
	leases, err := ListLeases()
	if err != nil {
		h.logger.Printf("Failed to list leases: %v", err)
		return
	}

	for _, lease := range leases {
		if IsLeaseExpired(lease) {
			h.logger.Printf("Reaping expired lease for agent %s (attached to session %s)",
				lease.AgentID, lease.UserSessionID)

			if err := ReleaseLease(lease.AgentID); err != nil {
				h.logger.Printf("Failed to release expired lease for agent %s: %v",
					lease.AgentID, err)
			}
		}
	}
}

// Stop gracefully shuts down the heartbeat monitor.
// It unsubscribes from NATS and closes the connection.
func (h *HeartbeatMonitor) Stop() {
	if h.sub != nil {
		if err := h.sub.Unsubscribe(); err != nil {
			h.logger.Printf("Failed to unsubscribe from heartbeats: %v", err)
		}
	}
	if h.nats != nil {
		h.nats.Close()
	}
}

// SetLogger sets a custom logger for the heartbeat monitor.
// This is useful for testing or custom logging configurations.
func (h *HeartbeatMonitor) SetLogger(logger *log.Logger) {
	h.logger = logger
}
