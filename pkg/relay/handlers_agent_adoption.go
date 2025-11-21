package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const (
	LabelNamespace   = "ourocodus.agent"
	LabelAgentID     = "agent-id"
	LabelSpawnSource = "ourocodus.agent/spawn-source"
)

// AgentStatus represents the attachment status of an agent
type AgentStatus string

const (
	StatusDetached AgentStatus = "detached"
	StatusAttached AgentStatus = "attached"
)

// AgentInfo contains information about a discovered agent
type AgentInfo struct {
	AgentID      string      `json:"agentId"`
	ContainerID  string      `json:"containerId"`
	Workspace    string      `json:"workspace"`
	Status       AgentStatus `json:"status"`
	SpawnSource  string      `json:"spawnSource"`
	AttachedTo   string      `json:"attachedTo,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
}

// AgentDiscoverRequest represents the agent:discover message
type AgentDiscoverRequest struct {
	Type string `json:"type"` // "agent:discover"
}

// AgentDiscoverResponse represents the agent:discovered message
type AgentDiscoverResponse struct {
	Type   string      `json:"type"` // "agent:discovered"
	Agents []AgentInfo `json:"agents"`
}

// handleAgentDiscover handles agent:discover messages
// Returns true if connection should be closed
func (s *Server) handleAgentDiscover(ctx context.Context, conn WebSocketConn, rawMessage []byte) bool {
	var req AgentDiscoverRequest
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		s.logger.Printf("Failed to parse agent:discover message: %v", err)
		return s.handleValidationError(conn, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Failed to parse agent:discover message",
			Recoverable: true,
		})
	}

	// Discover agents
	agents, err := s.discoverAgents(ctx)
	if err != nil {
		s.logger.Printf("Failed to discover agents: %v", err)
		errorMsg := NewErrorMessage("AGENT_DISCOVERY_FAILED", fmt.Sprintf("Failed to discover agents: %v", err), true)
		if err := conn.WriteJSON(errorMsg); err != nil {
			s.logger.Printf("Failed to send error response: %v", err)
			return true
		}
		return false
	}

	// Send response
	resp := AgentDiscoverResponse{
		Type:   "agent:discovered",
		Agents: agents,
	}

	if err := conn.WriteJSON(resp); err != nil {
		s.logger.Printf("Failed to send agent:discovered response: %v", err)
		return true
	}

	return false
}

// discoverAgents queries Docker for running agents and checks their attachment status
func (s *Server) discoverAgents(ctx context.Context) ([]AgentInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Filter for agent containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=true", LabelNamespace))

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     false, // Only running containers
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Get all leases to determine attached status
	leases, err := session.ListLeases()
	if err != nil {
		return nil, fmt.Errorf("failed to list leases: %w", err)
	}

	// Build lease map for quick lookup
	leaseMap := make(map[string]*session.Lease)
	for _, lease := range leases {
		if !session.IsLeaseExpired(lease) {
			leaseMap[lease.AgentID] = lease
		}
	}

	agents := make([]AgentInfo, 0, len(containers))
	for _, c := range containers {
		agentID := c.Labels[LabelAgentID]
		if agentID == "" {
			continue // Skip containers without agent-id label
		}

		// Extract workspace from mounts
		workspace := ""
		for _, mnt := range c.Mounts {
			if mnt.Destination == "/workspace" {
				workspace = mnt.Source
				break
			}
		}

		// Determine status from lease
		status := StatusDetached
		attachedTo := ""
		if lease, ok := leaseMap[agentID]; ok {
			status = StatusAttached
			attachedTo = lease.UserSessionID
		}

		agents = append(agents, AgentInfo{
			AgentID:     agentID,
			ContainerID: c.ID,
			Workspace:   workspace,
			Status:      status,
			SpawnSource: c.Labels[LabelSpawnSource],
			AttachedTo:  attachedTo,
			CreatedAt:   time.Unix(c.Created, 0),
		})
	}

	return agents, nil
}
