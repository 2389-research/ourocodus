package watch

import (
	"time"

	"github.com/nats-io/nats.go"

	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// connectedMsg is sent when NATS connection succeeds
type connectedMsg struct {
	nc *nats.Conn
}

// heartbeatMsg represents a heartbeat event received from NATS
type heartbeatMsg struct {
	AgentID   string
	Timestamp time.Time
	Lag       time.Duration
	Status    string
}

// leaseMsg represents a lease state update
// Lease can be nil to indicate the agent is detached
type leaseMsg struct {
	Lease *session.Lease
}

// errMsg represents an error that occurred during monitoring
type errMsg struct {
	err error
}

// Error implements the error interface for errMsg
func (e errMsg) Error() string {
	return e.err.Error()
}

// tickMsg is sent periodically to trigger state updates
type tickMsg time.Time
