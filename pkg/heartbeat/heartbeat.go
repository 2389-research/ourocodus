// Package heartbeat provides shared constants and types for the agent heartbeat system.
//
// This package serves as the single source of truth for heartbeat configuration
// to prevent drift between agent and relay implementations.
package heartbeat

import "time"

const (
	// Interval is the duration between heartbeat publishes.
	// Agents publish heartbeats at this interval to signal liveness.
	Interval = 30 * time.Second

	// SubjectPrefix is the NATS subject prefix for agent heartbeats.
	// Individual agent subjects are: "agent.heartbeat.<agentID>"
	SubjectPrefix = "agent.heartbeat"

	// SubjectPattern is the NATS wildcard pattern for subscribing to all agent heartbeats.
	SubjectPattern = "agent.heartbeat.*"
)

// Message represents a heartbeat message published by an agent.
// This structure is shared between agent (publisher) and relay (subscriber)
// to ensure compatibility.
type Message struct {
	AgentID   string    `json:"agentId"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}
