// Package agent provides agent lifecycle management including heartbeat publishing.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	// HeartbeatInterval is the interval between heartbeat publishes
	HeartbeatInterval = 30 * time.Second

	// HeartbeatSubject is the NATS subject pattern for agent heartbeats
	// The %s will be replaced with the agent ID
	HeartbeatSubject = "agent.heartbeat.%s"
)

// Heartbeat represents a heartbeat message published by an agent
type Heartbeat struct {
	AgentID   string    `json:"agentId"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

// HeartbeatPublisher publishes periodic heartbeats to NATS for liveness detection
type HeartbeatPublisher struct {
	agentID string
	nats    *nats.Conn
	cancel  context.CancelFunc
	logger  *log.Logger
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
		logger:  log.Default(),
	}

	// Connect with automatic reconnection support
	// Note: We don't use RetryOnFailedConnect to fail fast on initial connection errors
	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(-1), // Unlimited reconnection attempts after initial connection
		nats.ReconnectWait(2*time.Second),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			pub.logger.Printf("Reconnected to NATS server")
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				pub.logger.Printf("Disconnected from NATS: %v", err)
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
// This method blocks until the context is cancelled or an error occurs.
// It publishes an immediate heartbeat on start, then subsequent heartbeats
// every HeartbeatInterval (30 seconds).
//
// The heartbeat publisher is designed to be resilient - publish failures
// are logged but do not stop the publisher or crash the agent.
func (h *HeartbeatPublisher) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel

	subject := fmt.Sprintf(HeartbeatSubject, h.agentID)
	ticker := time.NewTicker(HeartbeatInterval)
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
	hb := Heartbeat{
		AgentID:   h.agentID,
		Timestamp: time.Now(),
		Status:    "active",
	}

	data, err := json.Marshal(hb)
	if err != nil {
		h.logger.Printf("Failed to marshal heartbeat: %v", err)
		return
	}

	if err := h.nats.Publish(subject, data); err != nil {
		h.logger.Printf("Failed to publish heartbeat: %v", err)
		return
	}
}

// Stop gracefully shuts down the heartbeat publisher.
// It cancels the publishing loop and closes the NATS connection.
func (h *HeartbeatPublisher) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	if h.nats != nil {
		h.nats.Close()
	}
}

// SetLogger sets a custom logger for the heartbeat publisher.
// This is useful for testing or custom logging configurations.
func (h *HeartbeatPublisher) SetLogger(logger *log.Logger) {
	h.logger = logger
}
