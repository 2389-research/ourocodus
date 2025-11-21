// Package agent provides agent lifecycle management including heartbeat publishing.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/2389-research/ourocodus/pkg/heartbeat"
	"github.com/nats-io/nats.go"
)

// HeartbeatPublisher publishes periodic heartbeats to NATS for liveness detection
type HeartbeatPublisher struct {
	agentID  string
	nats     *nats.Conn
	logger   atomic.Value // *log.Logger
	stopOnce sync.Once
}

// NewHeartbeatPublisher creates a new heartbeat publisher for the given agent.
// It establishes a connection to NATS and prepares to publish heartbeats.
//
// The connection is configured with automatic reconnection to handle network partitions:
//   - Unlimited reconnection attempts
//   - 2-second wait between reconnect attempts
//   - Logging on successful reconnection
//
// Parameters:
//   - agentID: The unique identifier for this agent
//   - natsURL: The NATS server URL (e.g., "nats://localhost:4222")
//
// Returns an error if the initial NATS connection fails.
func NewHeartbeatPublisher(agentID, natsURL string) (*HeartbeatPublisher, error) {
	// Create publisher instance first to use logger in reconnect handler
	pub := &HeartbeatPublisher{
		agentID: agentID,
	}
	pub.logger.Store(log.Default())

	// Connect with automatic reconnection support
	// Note: We don't use RetryOnFailedConnect to fail fast on initial connection errors
	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(-1), // Unlimited reconnection attempts after initial connection
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			pub.getLogger().Printf("Reconnected to NATS server")
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				pub.getLogger().Printf("Disconnected from NATS: %v", err)
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	pub.nats = nc
	return pub, nil
}

// Start begins publishing heartbeats at regular intervals.
// This method blocks until the context is cancelled.
// It publishes an immediate heartbeat on start, then subsequent heartbeats
// every 30 seconds (heartbeat.Interval).
//
// The heartbeat publisher is designed to be resilient - publish failures
// are logged but do not stop the publisher or crash the agent.
//
// Start should only be called once per HeartbeatPublisher instance.
func (h *HeartbeatPublisher) Start(ctx context.Context) {
	subject := fmt.Sprintf("%s.%s", heartbeat.SubjectPrefix, h.agentID)
	ticker := time.NewTicker(heartbeat.Interval)
	defer ticker.Stop()

	// Send initial heartbeat immediately
	h.publish(subject)

	for {
		select {
		case <-ticker.C:
			h.publish(subject)
		case <-ctx.Done():
			return
		}
	}
}

// publish sends a single heartbeat message to NATS.
// Errors are logged but do not interrupt the heartbeat loop.
func (h *HeartbeatPublisher) publish(subject string) {
	hb := heartbeat.Message{
		AgentID:   h.agentID,
		Timestamp: time.Now(),
		Status:    "active",
	}

	data, err := json.Marshal(hb)
	if err != nil {
		h.getLogger().Printf("Failed to marshal heartbeat: %v", err)
		return
	}

	if err := h.nats.Publish(subject, data); err != nil {
		h.getLogger().Printf("Failed to publish heartbeat: %v", err)
		return
	}
}

// getLogger returns the current logger in a thread-safe manner
func (h *HeartbeatPublisher) getLogger() *log.Logger {
	return h.logger.Load().(*log.Logger)
}

// Stop gracefully shuts down the heartbeat publisher.
// It closes the NATS connection. This method is safe to call multiple times.
//
// The caller should cancel the context passed to Start() to stop the
// publishing loop.
func (h *HeartbeatPublisher) Stop() {
	h.stopOnce.Do(func() {
		if h.nats != nil {
			h.nats.Close()
		}
	})
}

// SetLogger sets a custom logger for the heartbeat publisher.
// This is useful for testing or custom logging configurations.
// This method is safe to call concurrently with Start().
//
// If logger is nil, this method panics.
func (h *HeartbeatPublisher) SetLogger(logger *log.Logger) {
	if logger == nil {
		panic("logger cannot be nil")
	}
	h.logger.Store(logger)
}
