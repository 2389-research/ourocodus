package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/2389-research/ourocodus/pkg/labels"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Label constants removed - now using centralized pkg/labels package

// AgentStatus represents the attachment status of an agent
type AgentStatus string

const (
	StatusDetached AgentStatus = "detached"
	StatusAttached AgentStatus = "attached"
)

// AgentInfo contains information about a discovered agent
type AgentInfo struct {
	AgentID     string      `json:"agentId"`
	ContainerID string      `json:"containerId"`
	Workspace   string      `json:"workspace"`
	Status      AgentStatus `json:"status"`
	SpawnSource string      `json:"spawnSource"`
	AttachedTo  string      `json:"attachedTo,omitempty"`
	CreatedAt   time.Time   `json:"createdAt"`
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

// AgentAttachRequest represents the agent:attach message
type AgentAttachRequest struct {
	Type          string `json:"type"` // "agent:attach"
	AgentID       string `json:"agentId"`
	UserSessionID string `json:"userSessionId"`
	Token         string `json:"token"` // Phase 4: Required attach token for security
}

// AgentAttachResponse represents the agent:attached message
type AgentAttachResponse struct {
	Type      string    `json:"type"` // "agent:attached"
	AgentID   string    `json:"agentId"`
	SessionID string    `json:"sessionId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// AgentDetachRequest represents the agent:detach message
type AgentDetachRequest struct {
	Type          string `json:"type"` // "agent:detach"
	AgentID       string `json:"agentId"`
	UserSessionID string `json:"userSessionId"`
}

// AgentDetachResponse represents the agent:detached message
type AgentDetachResponse struct {
	Type    string `json:"type"` // "agent:detached"
	AgentID string `json:"agentId"`
}

// handleAgentDiscover handles agent:discover messages
// Returns true if connection should be closed
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleAgentDiscover(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) bool {
	var req AgentDiscoverRequest
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		s.logger.Printf("Failed to parse agent:discover message: %v", err)
		return s.handleValidationError(adapter, ValidationError{
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
		if err := adapter.WriteJSON(errorMsg); err != nil {
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

	if err := adapter.WriteJSON(resp); err != nil {
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
	defer func() { _ = cli.Close() }()

	// Use Phase 3 labels package to list all agent containers
	filterArgs := labels.ListAgentsFilter()

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
		// Use Phase 3 labels package constants
		agentID := c.Labels[labels.AgentID]
		if agentID == "" {
			continue // Skip containers without agent-id label
		}

		// Try to get workspace from Phase 3 label first
		workspace := c.Labels[labels.Workspace]

		// Fallback: Extract workspace from mounts if label not present
		if workspace == "" {
			for _, mnt := range c.Mounts {
				if mnt.Destination == "/workspace" {
					workspace = mnt.Source
					break
				}
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
			SpawnSource: c.Labels[labels.SpawnSource],
			AttachedTo:  attachedTo,
			CreatedAt:   time.Unix(c.Created, 0),
		})
	}

	return agents, nil
}

// validateAttachRequest validates required fields for agent:attach
// Returns true if connection should be closed
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) validateAttachRequest(adapter *SessionWebSocketAdapter, req *AgentAttachRequest) bool {
	if req.AgentID == "" {
		s.logger.Printf("agent:attach missing agentId")
		errorMsg := NewErrorMessage("MISSING_AGENT_ID", "agentId is required", true)
		if err := adapter.WriteJSON(errorMsg); err != nil {
			s.logger.Printf("Failed to send error response: %v", err)
			return true
		}
		return false
	}
	if req.UserSessionID == "" {
		s.logger.Printf("agent:attach missing userSessionId")
		errorMsg := NewErrorMessage("MISSING_SESSION_ID", "userSessionId is required", true)
		if err := adapter.WriteJSON(errorMsg); err != nil {
			s.logger.Printf("Failed to send error response: %v", err)
			return true
		}
		return false
	}
	return false
}

// handleAlreadyAttachedError handles the case where agent is already attached
// Returns true if connection should be closed
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleAlreadyAttachedError(adapter *SessionWebSocketAdapter, req *AgentAttachRequest) bool {
	existingLease, readErr := session.ReadLease(req.AgentID)
	if readErr == nil && existingLease.UserSessionID == req.UserSessionID {
		// Already attached to this user - return success (idempotent)
		resp := AgentAttachResponse{
			Type:      "agent:attached",
			AgentID:   req.AgentID,
			SessionID: req.UserSessionID,
			ExpiresAt: existingLease.ExpiresAt,
		}
		if err := adapter.WriteJSON(resp); err != nil {
			s.logger.Printf("Failed to send agent:attached response: %v", err)
			return true
		}
		return false
	}
	// Attached to different user
	s.logger.Printf("Agent %s already attached to session %s", req.AgentID, existingLease.UserSessionID)
	errorMsg := NewErrorMessage("AGENT_ALREADY_ATTACHED", "Agent is already attached to another session", true)
	if err := adapter.WriteJSON(errorMsg); err != nil {
		s.logger.Printf("Failed to send error response: %v", err)
		return true
	}
	return false
}

// handleAttachError handles various attach errors and returns true if connection should close
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleAttachError(adapter *SessionWebSocketAdapter, req *AgentAttachRequest, err error) bool {
	// Handle specific error types
	if err == session.ErrAlreadyAttached {
		return s.handleAlreadyAttachedError(adapter, req)
	}
	if err == session.ErrMissingAttachToken {
		s.logger.Printf("Attach token missing for agent %s", req.AgentID)
		return s.sendErrorResponseAdapter(adapter, "MISSING_TOKEN", "Attach token is required")
	}
	if err == session.ErrInvalidAttachToken {
		s.logger.Printf("Invalid attach token for agent %s", req.AgentID)
		return s.sendErrorResponseAdapter(adapter, "INVALID_TOKEN", "Invalid attach token")
	}
	// Generic attach failure
	s.logger.Printf("Failed to attach agent %s to session %s: %v", req.AgentID, req.UserSessionID, err)
	return s.sendErrorResponseAdapter(adapter, "ATTACH_FAILED", fmt.Sprintf("Failed to attach agent: %v", err))
}

// sendErrorResponseAdapter sends a recoverable error message using adapter for thread-safe writes
// Returns true if connection should close
func (s *Server) sendErrorResponseAdapter(adapter *SessionWebSocketAdapter, code, message string) bool {
	errorMsg := NewErrorMessage(code, message, true)
	if err := adapter.WriteJSON(errorMsg); err != nil {
		s.logger.Printf("Failed to send error response: %v", err)
		return true
	}
	return false
}

// handleAgentAttach handles agent:attach messages
// Returns true if connection should be closed
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleAgentAttach(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) bool {
	var req AgentAttachRequest
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		s.logger.Printf("Failed to parse agent:attach message: %v", err)
		return s.handleValidationError(adapter, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Failed to parse agent:attach message",
			Recoverable: true,
		})
	}

	// Validate required fields
	if shouldClose := s.validateAttachRequest(adapter, &req); shouldClose {
		return shouldClose
	}

	// Phase 4: Rate limiting for attach operations
	if !s.rateLimiter.Allow(req.UserSessionID) {
		s.logger.Printf("Rate limit exceeded for user session %s on agent:attach", req.UserSessionID)
		return s.sendErrorResponseAdapter(adapter, "RATE_LIMIT_EXCEEDED", "Too many attach requests. Please wait before trying again.")
	}

	// Get workspace path from Docker
	workspace, err := s.getAgentWorkspace(ctx, req.AgentID)
	if err != nil {
		s.logger.Printf("Failed to get workspace for agent %s: %v", req.AgentID, err)
		return s.sendErrorResponseAdapter(adapter, "AGENT_NOT_FOUND", fmt.Sprintf("Agent %s not found or workspace unavailable", req.AgentID))
	}

	// Get UserSession from session manager
	userSession := s.sessionManager.Get(req.UserSessionID)
	if userSession == nil {
		s.logger.Printf("User session not found: %s", req.UserSessionID)
		return s.sendErrorResponseAdapter(adapter, "SESSION_NOT_FOUND", fmt.Sprintf("User session %s not found", req.UserSessionID))
	}

	// Attach agent to user session (Phase 4: with token verification)
	agentSession, err := userSession.AttachAgent(req.AgentID, workspace, req.Token)
	if err != nil {
		return s.handleAttachError(adapter, &req, err)
	}

	// Send success response
	resp := AgentAttachResponse{
		Type:      "agent:attached",
		AgentID:   agentSession.GetAgentID(),
		SessionID: req.UserSessionID,
		ExpiresAt: agentSession.GetExpiresAt(),
	}

	if err := adapter.WriteJSON(resp); err != nil {
		s.logger.Printf("Failed to send agent:attached response: %v", err)
		// Detach agent since we couldn't send the response
		_ = userSession.DetachAgent(req.AgentID)
		return true
	}

	return false
}

// validateDetachRequest validates required fields for agent:detach
// Returns true if connection should be closed
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) validateDetachRequest(adapter *SessionWebSocketAdapter, req *AgentDetachRequest) bool {
	if req.AgentID == "" {
		s.logger.Printf("agent:detach missing agentId")
		errorMsg := NewErrorMessage("MISSING_AGENT_ID", "agentId is required", true)
		if err := adapter.WriteJSON(errorMsg); err != nil {
			s.logger.Printf("Failed to send error response: %v", err)
			return true
		}
		return false
	}
	if req.UserSessionID == "" {
		s.logger.Printf("agent:detach missing userSessionId")
		errorMsg := NewErrorMessage("MISSING_SESSION_ID", "userSessionId is required", true)
		if err := adapter.WriteJSON(errorMsg); err != nil {
			s.logger.Printf("Failed to send error response: %v", err)
			return true
		}
		return false
	}
	return false
}

// checkDetachOwnership checks if agent can be detached by this session
// Returns (shouldClose, handled) - if handled is true, response was already sent
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) checkDetachOwnership(adapter *SessionWebSocketAdapter, req *AgentDetachRequest, agent *session.AgentSession) (bool, bool) {
	if agent == nil {
		// Check if it's attached to a different session
		lease, err := session.ReadLease(req.AgentID)
		if err == nil && lease.UserSessionID != req.UserSessionID {
			s.logger.Printf("Agent %s is attached to session %s, not %s", req.AgentID, lease.UserSessionID, req.UserSessionID)
			errorMsg := NewErrorMessage("NOT_ATTACHED_TO_YOU", "Agent is not attached to your session", true)
			if err := adapter.WriteJSON(errorMsg); err != nil {
				s.logger.Printf("Failed to send error response: %v", err)
				return true, true
			}
			return false, true
		}
		// Not attached to anyone - idempotent success
		resp := AgentDetachResponse{
			Type:    "agent:detached",
			AgentID: req.AgentID,
		}
		if err := adapter.WriteJSON(resp); err != nil {
			s.logger.Printf("Failed to send agent:detached response: %v", err)
			return true, true
		}
		return false, true
	}
	return false, false
}

// handleAgentDetach handles agent:detach messages
// Returns true if connection should be closed
// Uses adapter for thread-safe writes (issue #213)
//
//nolint:unparam // ctx parameter required by handler interface, may be used in future
func (s *Server) handleAgentDetach(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) bool {
	var req AgentDetachRequest
	if err := json.Unmarshal(rawMessage, &req); err != nil {
		s.logger.Printf("Failed to parse agent:detach message: %v", err)
		return s.handleValidationError(adapter, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Failed to parse agent:detach message",
			Recoverable: true,
		})
	}

	// Validate required fields
	if shouldClose := s.validateDetachRequest(adapter, &req); shouldClose {
		return shouldClose
	}

	// Get UserSession from session manager
	userSession := s.sessionManager.Get(req.UserSessionID)
	if userSession == nil {
		s.logger.Printf("User session not found: %s", req.UserSessionID)
		errorMsg := NewErrorMessage("SESSION_NOT_FOUND", fmt.Sprintf("User session %s not found", req.UserSessionID), true)
		if err := adapter.WriteJSON(errorMsg); err != nil {
			s.logger.Printf("Failed to send error response: %v", err)
			return true
		}
		return false
	}

	// Check if agent is attached to this session before detaching
	agent := userSession.GetAgent(req.AgentID)
	if shouldClose, handled := s.checkDetachOwnership(adapter, &req, agent); handled {
		return shouldClose
	}

	// Detach agent from user session
	if err := userSession.DetachAgent(req.AgentID); err != nil {
		s.logger.Printf("Failed to detach agent %s from session %s: %v", req.AgentID, req.UserSessionID, err)
		errorMsg := NewErrorMessage("DETACH_FAILED", fmt.Sprintf("Failed to detach agent: %v", err), true)
		if err := adapter.WriteJSON(errorMsg); err != nil {
			s.logger.Printf("Failed to send error response: %v", err)
			return true
		}
		return false
	}

	// Send success response
	resp := AgentDetachResponse{
		Type:    "agent:detached",
		AgentID: req.AgentID,
	}

	if err := adapter.WriteJSON(resp); err != nil {
		s.logger.Printf("Failed to send agent:detached response: %v", err)
		return true
	}

	return false
}

// getAgentWorkspace retrieves the workspace path for an agent from Docker mounts
func (s *Server) getAgentWorkspace(ctx context.Context, agentID string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	// Use Phase 3 labels package for consistent querying
	filterArgs := labels.FindAgentFilter(agentID)

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     false, // Only running containers
		Filters: filterArgs,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("agent container not found")
	}

	// Try to get workspace from Phase 3 label first
	ctr := containers[0]
	workspace := ctr.Labels[labels.Workspace]

	// Fallback: Extract workspace from mounts if label not present
	if workspace == "" {
		for _, mnt := range ctr.Mounts {
			if mnt.Destination == "/workspace" {
				workspace = mnt.Source
				break
			}
		}
	}

	if workspace == "" {
		return "", fmt.Errorf("workspace not found (no label or mount)")
	}

	return workspace, nil
}
