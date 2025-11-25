package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/relay/audit"
)

// UserSessionState represents the lifecycle state of a user session (container)
type UserSessionState string

const (
	// StateActive indicates user session is active (can have 0-N agents)
	StateActive UserSessionState = "ACTIVE"

	// StateTerminated indicates user session and all agents have been terminated
	StateTerminated UserSessionState = "TERMINATED"
)

// String returns the string representation of UserSessionState
func (s UserSessionState) String() string {
	return string(s)
}

// IsValid returns true if the state is a recognized UserSessionState
func (s UserSessionState) IsValid() bool {
	switch s {
	case StateActive, StateTerminated:
		return true
	default:
		return false
	}
}

// AgentState represents the lifecycle state of an individual agent session
type AgentState string

const (
	// AgentSpawning indicates ACP process is being spawned
	AgentSpawning AgentState = "SPAWNING"

	// AgentActive indicates ACP process is running and accepting messages
	AgentActive AgentState = "ACTIVE"

	// AgentFailed indicates ACP process spawn or operation failed
	AgentFailed AgentState = "FAILED"

	// AgentTerminated indicates ACP process has been terminated
	AgentTerminated AgentState = "TERMINATED"
)

// String returns the string representation of AgentState
func (a AgentState) String() string {
	return string(a)
}

// IsValid returns true if the state is a recognized AgentState
func (a AgentState) IsValid() bool {
	switch a {
	case AgentSpawning, AgentActive, AgentFailed, AgentTerminated:
		return true
	default:
		return false
	}
}

// Message represents a single conversation message
type Message struct {
	From      string    `json:"from"`      // "user" or "agent"
	Content   string    `json:"content"`   // Message content
	Timestamp time.Time `json:"timestamp"` // When message was sent
}

// AgentSession represents ONE claude-code-acp process within a user session
// Immutable after creation except for state transitions through Manager
type AgentSession struct {
	// Immutable fields (set at creation, never modified)
	AgentID     string    // User-chosen agent identifier (e.g., "coder-1", "analyzer")
	Workspace   string    // Path to agent workspace directory
	ContainerID string    // Docker container ID (empty for host-mode agents)
	IsAdopted   bool      // True if agent was adopted (vs spawned by relay)
	createdAt   time.Time // Agent creation timestamp
	expiresAt   time.Time // Lease expiration timestamp

	// Mutable fields (protected by mu)
	state      AgentState
	acpClient  ACPClient
	lastActive time.Time
	errorMsg   string    // Error message if state is FAILED
	history    []Message // Conversation history

	mu sync.RWMutex
}

// AgentRuntimeContext describes the runtime environment for an ACP process.
// It is passed to factories launching transports so they can decide whether
// to run ACP on the host or inside a container.
type AgentRuntimeContext struct {
	SessionID   string
	AgentID     string
	Workspace   string
	ContainerID string
}

// HasContainer returns true if ACP should run inside a container runtime.
func (c *AgentRuntimeContext) HasContainer() bool {
	return c != nil && c.ContainerID != ""
}

// UserSession represents a user's workspace container (0-N agents)
// Immutable after creation except for state transitions through Manager
type UserSession struct {
	// Immutable fields (set at creation, never modified)
	ID        string    // UUID v4
	createdAt time.Time // Session creation timestamp

	// Mutable fields (protected by mu)
	state      UserSessionState
	webSocket  WebSocketConn
	agents     map[string]*AgentSession // agentID → agent instance
	lastActive time.Time

	mu sync.RWMutex
}

// WebSocketConn abstracts WebSocket operations for session layer
// Server owns all I/O - session layer only needs WriteJSON and Close
type WebSocketConn interface {
	WriteJSON(v interface{}) error
	Close() error
}

// ACPClient abstracts ACP process operations
// Implemented by pkg/acp.Client
type ACPClient interface {
	SendMessage(ctx context.Context, content string) (*acp.AgentMessage, error)
	Close(ctx context.Context) error
}

// ClientFactory abstracts ACP client creation for testing
type ClientFactory interface {
	NewClient(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error)
}

// NewUserSession creates a new user session in ACTIVE state
// Pure function - no side effects, no I/O
// Session starts empty with no agents
func NewUserSession(id string, ws WebSocketConn, createdAt time.Time) *UserSession {
	return &UserSession{
		ID:         id,
		webSocket:  ws,
		agents:     make(map[string]*AgentSession),
		state:      StateActive,
		createdAt:  createdAt,
		lastActive: createdAt,
	}
}

// NewAgentSession creates a new agent session in SPAWNING state
// Pure function - no side effects, no I/O
func NewAgentSession(agentID, workspace string, createdAt time.Time) *AgentSession {
	return &AgentSession{
		AgentID:    agentID,
		Workspace:  workspace,
		state:      AgentSpawning,
		createdAt:  createdAt,
		lastActive: createdAt,
		history:    make([]Message, 0),
	}
}

// --- UserSession accessors (thread-safe) ---

// GetID returns the user session ID (immutable, no lock needed)
func (u *UserSession) GetID() string {
	return u.ID
}

// GetState returns the current user session state
func (u *UserSession) GetState() UserSessionState {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.state
}

// GetWebSocket returns the WebSocket connection
func (u *UserSession) GetWebSocket() WebSocketConn {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.webSocket
}

// GetAgent returns the agent session for the given agent ID (may be nil)
func (u *UserSession) GetAgent(agentID string) *AgentSession {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.agents[agentID]
}

// ListAgents returns a copy of the agents map
func (u *UserSession) ListAgents() map[string]*AgentSession {
	u.mu.RLock()
	defer u.mu.RUnlock()
	agents := make(map[string]*AgentSession, len(u.agents))
	for k, v := range u.agents {
		agents[k] = v
	}
	return agents
}

// AgentCount returns the number of agents in this session
func (u *UserSession) AgentCount() int {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return len(u.agents)
}

// GetCreatedAt returns the session creation timestamp (immutable, no lock needed)
func (u *UserSession) GetCreatedAt() time.Time {
	return u.createdAt
}

// GetLastActive returns the last activity timestamp
func (u *UserSession) GetLastActive() time.Time {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.lastActive
}

// --- AgentSession accessors (thread-safe) ---

// GetAgentID returns the agent identifier (immutable, no lock needed)
func (a *AgentSession) GetAgentID() string {
	return a.AgentID
}

// GetWorkspace returns the agent workspace path (immutable, no lock needed)
func (a *AgentSession) GetWorkspace() string {
	return a.Workspace
}

// GetState returns the current agent state
func (a *AgentSession) GetState() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// GetACPClient returns the ACP client (may be nil if not spawned)
func (a *AgentSession) GetACPClient() ACPClient {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.acpClient
}

// GetError returns the error message if state is FAILED
func (a *AgentSession) GetError() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.errorMsg
}

// GetCreatedAt returns the agent creation timestamp (immutable, no lock needed)
func (a *AgentSession) GetCreatedAt() time.Time {
	return a.createdAt
}

// GetExpiresAt returns the lease expiration timestamp
func (a *AgentSession) GetExpiresAt() time.Time {
	return a.expiresAt
}

// GetLastActive returns the last activity timestamp
func (a *AgentSession) GetLastActive() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastActive
}

// AddMessage appends a message to the conversation history (thread-safe)
func (a *AgentSession) AddMessage(from, content string, timestamp time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = append(a.history, Message{
		From:      from,
		Content:   content,
		Timestamp: timestamp,
	})
}

// GetHistory returns a copy of the conversation history (thread-safe)
func (a *AgentSession) GetHistory() []Message {
	a.mu.RLock()
	defer a.mu.RUnlock()
	// Return a copy to prevent external modification
	history := make([]Message, len(a.history))
	copy(history, a.history)
	return history
}

// --- CLI Agent Adoption methods (Phase 1) ---

// AttachAgent attaches a CLI-spawned agent to this UserSession.
// Returns the agent session if successful, or error if agent is already attached elsewhere.
// This operation is idempotent - calling it multiple times for the same agent has no effect.
//
// Phase 4: Requires a valid attach token for security.
// The token must match the one generated during agent spawn.
func (u *UserSession) AttachAgent(agentID string, workspace string, attachToken string) (*AgentSession, error) {
	// Phase 1: Quick check under lock for idempotency
	u.mu.Lock()
	if existing, ok := u.agents[agentID]; ok {
		u.mu.Unlock()
		// Audit: Successful idempotent attach
		audit.LogAgentAttach(u.ID, agentID, true, nil)
		return existing, nil
	}
	userID := u.ID // Capture for use outside lock
	u.mu.Unlock()

	// Phase 2: All I/O operations WITHOUT holding the lock
	// This prevents blocking other goroutines during slow operations

	// Verify attach token BEFORE acquiring lease (Phase 4: Security)
	if err := verifyAttachToken(agentID, attachToken); err != nil {
		// Audit: Token verification failure
		audit.LogAuthFailure(userID, agentID, err.Error(), map[string]string{
			"operation": "attach",
		})
		return nil, err
	}

	// Try to acquire lease (this is atomic via O_EXCL)
	lease, err := AcquireLease(agentID, userID)
	if err != nil {
		// Audit: Lease acquisition failure
		audit.LogAgentAttach(userID, agentID, false, err)
		return nil, err // ErrAlreadyAttached if taken by another session
	}

	// Find the agent's container to establish ACP communication (Phase 3)
	containerID, discoveredWorkspace, err := findAgentContainerID(context.Background(), agentID)
	if err != nil {
		// Release lease if we can't find the container
		_ = ReleaseLease(agentID)
		// Audit: Container discovery failure
		audit.LogAgentAttach(userID, agentID, false, fmt.Errorf("failed to find agent container: %w", err))
		return nil, fmt.Errorf("failed to find agent container: %w", err)
	}

	// Use discovered workspace if none was provided
	if workspace == "" {
		workspace = discoveredWorkspace
	}

	// Create ACP bridge for bidirectional communication (Phase 3)
	// Note: logger is nil here; AttachAgent caller should provide logger if needed
	bridge, err := NewACPBridge(context.Background(), containerID, agentID, nil)
	if err != nil {
		// Release lease if bridge creation fails
		_ = ReleaseLease(agentID)
		// Audit: ACP bridge creation failure
		audit.LogAgentAttach(userID, agentID, false, fmt.Errorf("failed to create ACP bridge: %w", err))
		return nil, fmt.Errorf("failed to create ACP bridge: %w", err)
	}

	// Phase 3: Re-acquire lock for state mutation only
	u.mu.Lock()
	defer u.mu.Unlock()

	// Double-check: another goroutine may have attached while we were doing I/O
	if existing, ok := u.agents[agentID]; ok {
		// Clean up our work - another goroutine won the race
		_ = bridge.Close(context.Background())
		_ = ReleaseLease(agentID)
		// Audit: Successful idempotent attach
		audit.LogAgentAttach(userID, agentID, true, nil)
		return existing, nil
	}

	// Create AgentSession for this CLI agent
	agent := &AgentSession{
		AgentID:     agentID,
		Workspace:   workspace,
		ContainerID: containerID, // Track container for termination
		IsAdopted:   true,        // Mark as adopted (not spawned by relay)
		createdAt:   lease.AttachedAt,
		expiresAt:   lease.ExpiresAt,
		state:       AgentActive, // CLI agent is already running
		lastActive:  time.Now(),
		history:     []Message{},
		acpClient:   bridge, // Phase 3: ACP bridge for communication
	}

	u.agents[agentID] = agent
	u.lastActive = time.Now()

	// Audit: Successful agent attachment
	audit.LogAgentAttach(userID, agentID, true, nil)

	return agent, nil
}

// DetachAgent detaches a CLI-spawned agent from this UserSession.
// The agent container continues running but is no longer associated with this session.
// This operation is idempotent - calling it multiple times has no effect.
func (u *UserSession) DetachAgent(agentID string) error {
	// Phase 1: Quick check and extract under lock
	u.mu.Lock()
	agent, ok := u.agents[agentID]
	if !ok {
		userID := u.ID
		u.mu.Unlock()
		// Already detached - idempotent
		// Audit: Successful idempotent detach
		audit.LogAgentDetach(userID, agentID, true, nil)
		return nil
	}
	// Remove from map immediately to prevent double-detach races
	delete(u.agents, agentID)
	u.lastActive = time.Now()
	userID := u.ID
	acpClient := agent.acpClient
	u.mu.Unlock()

	// Phase 2: All I/O operations WITHOUT holding the lock
	// This prevents blocking other goroutines during slow operations

	// Close ACP bridge (Phase 3)
	if acpClient != nil {
		_ = acpClient.Close(context.Background())
	}

	// Release lease (idempotent operation)
	if err := ReleaseLease(agentID); err != nil {
		// Audit: Lease release failure
		// Note: We've already removed from map, so this is a partial failure
		// The agent is effectively detached from this session but lease may be orphaned
		audit.LogAgentDetach(userID, agentID, false, err)
		return err
	}

	// Audit: Successful agent detachment
	audit.LogAgentDetach(userID, agentID, true, nil)

	// Note: Don't terminate the agent container, it continues running detached

	return nil
}

// --- Package-private mutators (called only by Manager) ---

// setState updates the user session state (must hold lock)
func (u *UserSession) setState(state UserSessionState) {
	u.state = state
}

// addAgent adds an agent to the session (must hold lock)
func (u *UserSession) addAgent(agent *AgentSession) {
	u.agents[agent.AgentID] = agent
}

// removeAgent removes an agent from the session (must hold lock)
func (u *UserSession) removeAgent(agentID string) {
	delete(u.agents, agentID)
}

// setLastActive updates the last activity timestamp (must hold lock)
func (u *UserSession) setLastActive(t time.Time) {
	u.lastActive = t
}

// setAgentState updates the agent state (must hold lock)
func (a *AgentSession) setAgentState(state AgentState) {
	a.state = state
}

// setACPClient updates the ACP client (must hold lock)
func (a *AgentSession) setACPClient(client ACPClient) {
	a.acpClient = client
}

// setError updates the error message (must hold lock)
func (a *AgentSession) setError(err string) {
	a.errorMsg = err
}

// setAgentLastActive updates the last activity timestamp (must hold lock)
func (a *AgentSession) setAgentLastActive(t time.Time) {
	a.lastActive = t
}
