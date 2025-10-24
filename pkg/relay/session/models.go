package session

import (
	"sync"
	"time"
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

// AgentSession represents ONE claude-code-acp process within a user session
// Immutable after creation except for state transitions through Manager
type AgentSession struct {
	// Immutable fields (set at creation)
	Role      string // "auth", "db", "tests", etc.
	Workspace string // Path to agent workspace directory

	// Mutable fields (protected by mu)
	state      AgentState
	acpClient  ACPClient
	createdAt  time.Time
	lastActive time.Time
	errorMsg   string // Error message if state is FAILED

	mu sync.RWMutex
}

// UserSession represents a user's workspace container (0-N agents)
// Immutable after creation except for state transitions through Manager
type UserSession struct {
	// Immutable fields (set at creation)
	ID string // UUID v4

	// Mutable fields (protected by mu)
	state      UserSessionState
	webSocket  WebSocketConn
	agents     map[string]*AgentSession // role → agent instance
	createdAt  time.Time
	lastActive time.Time

	mu sync.RWMutex
}

// WebSocketConn abstracts WebSocket operations
// Matches existing relay.WebSocketConn interface for compatibility
type WebSocketConn interface {
	WriteJSON(v interface{}) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

// ACPClient abstracts ACP process operations
// Implemented by pkg/acp.Client
// Note: AgentMessage type is defined in pkg/acp
type ACPClient interface {
	SendMessage(content string) (interface{}, error) // Returns *acp.AgentMessage
	Close() error
}

// ClientFactory abstracts ACP client creation for testing
type ClientFactory interface {
	NewClient(workspace string) (ACPClient, error)
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
func NewAgentSession(role, workspace string, createdAt time.Time) *AgentSession {
	return &AgentSession{
		Role:       role,
		Workspace:  workspace,
		state:      AgentSpawning,
		createdAt:  createdAt,
		lastActive: createdAt,
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

// GetAgent returns the agent session for the given role (may be nil)
func (u *UserSession) GetAgent(role string) *AgentSession {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.agents[role]
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

// GetRole returns the agent role (immutable, no lock needed)
func (a *AgentSession) GetRole() string {
	return a.Role
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

// GetLastActive returns the last activity timestamp
func (a *AgentSession) GetLastActive() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastActive
}

// --- Package-private mutators (called only by Manager) ---

// setState updates the user session state (must hold lock)
func (u *UserSession) setState(state UserSessionState) {
	u.state = state
}

// addAgent adds an agent to the session (must hold lock)
func (u *UserSession) addAgent(agent *AgentSession) {
	u.agents[agent.Role] = agent
}

// removeAgent removes an agent from the session (must hold lock)
func (u *UserSession) removeAgent(role string) {
	delete(u.agents, role)
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
