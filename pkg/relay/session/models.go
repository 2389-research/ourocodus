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
	AgentID   string    // User-chosen agent identifier (e.g., "coder-1", "analyzer")
	Workspace string    // Path to agent workspace directory
	createdAt time.Time // Agent creation timestamp

	// Mutable fields (protected by mu)
	state      AgentState
	acpClient  ACPClient
	lastActive time.Time
	errorMsg   string    // Error message if state is FAILED
	history    []Message // Conversation history

	mu sync.RWMutex
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
