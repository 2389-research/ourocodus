package session

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// IDGenerator abstracts unique ID generation
type IDGenerator interface {
	Generate() string
}

// Clock abstracts time operations for deterministic testing
type Clock interface {
	Now() time.Time
}

// Cleaner abstracts cleanup operations for session termination
type Cleaner interface {
	// Cleanup performs idempotent cleanup of user session resources
	// Called during session termination
	Cleanup(ctx context.Context, session *UserSession) error
}

// Logger abstracts logging operations
type Logger interface {
	Printf(format string, v ...interface{})
}

// Manager coordinates user session and agent lifecycle with dependency injection
// Composes Store + ClientFactory + Cleaner for testable orchestration
type Manager struct {
	store         Store
	idGen         IDGenerator
	clock         Clock
	cleaner       Cleaner
	logger        Logger
	clientFactory ClientFactory
}

// NewManager creates a session manager with injected dependencies.
//
// All dependencies are required and must be non-nil. This constructor panics on
// nil collaborators because missing dependencies indicate programmer configuration
// bugs, not runtime failures.
func NewManager(store Store, idGen IDGenerator, clock Clock, cleaner Cleaner, logger Logger, clientFactory ClientFactory) *Manager {
	if store == nil {
		panic("store cannot be nil")
	}
	if idGen == nil {
		panic("idGen cannot be nil")
	}
	if clock == nil {
		panic("clock cannot be nil")
	}
	if cleaner == nil {
		panic("cleaner cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}
	if clientFactory == nil {
		panic("clientFactory cannot be nil")
	}

	return &Manager{
		store:         store,
		idGen:         idGen,
		clock:         clock,
		cleaner:       cleaner,
		logger:        logger,
		clientFactory: clientFactory,
	}
}

// CreateUserSession creates a new user session in ACTIVE state with no agents
// Session starts empty - agents are spawned separately via SpawnAgent
func (m *Manager) CreateUserSession(ctx context.Context, ws WebSocketConn) (*UserSession, error) {
	// Validate input
	if ws == nil {
		return nil, fmt.Errorf("websocket connection cannot be nil")
	}

	// Generate unique ID and create session
	sessionID := m.idGen.Generate()
	now := m.clock.Now()
	session := NewUserSession(sessionID, ws, now)

	// Store session
	if err := m.store.Create(session); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	m.logger.Printf("User session created: id=%s state=ACTIVE agents=0", sessionID)
	return session, nil
}

// Get retrieves a user session by ID
func (m *Manager) Get(id string) *UserSession {
	return m.store.Get(id)
}

// List returns all user sessions matching the filter
func (m *Manager) List(filter *SessionFilter) []*UserSession {
	return m.store.List(filter)
}

// SpawnAgent spawns ONE agent into an existing user session
// Creates workspace directory if needed, spawns ACP client, adds to session
// Returns error if spawn fails, but user session stays ACTIVE
func (m *Manager) SpawnAgent(ctx context.Context, sessionID, role, workspace string) error {
	// Validate inputs
	if role == "" {
		return fmt.Errorf("role cannot be empty")
	}
	if workspace == "" {
		return fmt.Errorf("workspace cannot be empty")
	}

	// Get user session
	session := m.store.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Check if session is active
	if session.GetState() != StateActive {
		return fmt.Errorf("session %s is not active (state=%s)", sessionID, session.GetState())
	}

	// Check if agent already exists
	if session.GetAgent(role) != nil {
		return fmt.Errorf("agent %s already exists in session %s", role, sessionID)
	}

	m.logger.Printf("Spawning agent: session=%s role=%s workspace=%s", sessionID, role, workspace)

	// Create workspace directory if needed
	if err := os.MkdirAll(workspace, 0o750); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create agent session in SPAWNING state
	now := m.clock.Now()
	agent := NewAgentSession(role, workspace, now)

	// Add agent to session (in SPAWNING state)
	session.mu.Lock()
	session.addAgent(agent)
	session.setLastActive(now)
	session.mu.Unlock()

	// Spawn ACP client
	acpClient, err := m.clientFactory.NewClient(workspace)
	if err != nil {
		// Mark agent as FAILED
		agent.mu.Lock()
		agent.setAgentState(AgentFailed)
		agent.setError(err.Error())
		agent.mu.Unlock()

		m.logger.Printf("Agent spawn failed: session=%s role=%s error=%v", sessionID, role, err)
		return fmt.Errorf("failed to spawn ACP client: %w", err)
	}

	// Transition agent to ACTIVE
	agent.mu.Lock()
	agent.setACPClient(acpClient)
	agent.setAgentState(AgentActive)
	agent.setAgentLastActive(m.clock.Now())
	agent.mu.Unlock()

	m.logger.Printf("Agent spawned: session=%s role=%s state=ACTIVE", sessionID, role)
	return nil
}

// GetAgent returns the agent session for a given role within a user session
func (m *Manager) GetAgent(sessionID, role string) (*AgentSession, error) {
	session := m.store.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	agent := session.GetAgent(role)
	if agent == nil {
		return nil, fmt.Errorf("agent %s not found in session %s", role, sessionID)
	}

	return agent, nil
}

// ListAgents returns all agents in a user session
func (m *Manager) ListAgents(sessionID string) (map[string]*AgentSession, error) {
	session := m.store.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session.ListAgents(), nil
}

// TerminateAgent terminates ONE agent in a user session
// User session stays ACTIVE, other agents unaffected
func (m *Manager) TerminateAgent(ctx context.Context, sessionID, role string) error {
	session := m.store.Get(sessionID)
	if session == nil {
		// Already cleaned up - idempotent
		m.logger.Printf("Session not found during agent termination: %s (already cleaned?)", sessionID)
		return nil
	}

	agent := session.GetAgent(role)
	if agent == nil {
		// Already removed - idempotent
		m.logger.Printf("Agent not found during termination: session=%s role=%s (already terminated?)", sessionID, role)
		return nil
	}

	m.logger.Printf("Terminating agent: session=%s role=%s", sessionID, role)

	// Close ACP client if present
	if acpClient := agent.GetACPClient(); acpClient != nil {
		if err := acpClient.Close(); err != nil {
			m.logger.Printf("Error closing ACP client: session=%s role=%s error=%v", sessionID, role, err)
			// Continue with cleanup even if close fails
		}
	}

	// Mark agent as terminated and remove from session
	agent.mu.Lock()
	agent.setAgentState(AgentTerminated)
	agent.mu.Unlock()

	session.mu.Lock()
	session.removeAgent(role)
	session.setLastActive(m.clock.Now())
	session.mu.Unlock()

	m.logger.Printf("Agent terminated: session=%s role=%s", sessionID, role)
	return nil
}

// TerminateUserSession terminates ALL agents in parallel, then terminates the session
// Idempotent - safe to call multiple times
func (m *Manager) TerminateUserSession(ctx context.Context, sessionID string) error {
	session := m.store.Get(sessionID)
	if session == nil {
		// Already cleaned up - idempotent
		m.logger.Printf("Session not found during termination: %s (already cleaned?)", sessionID)
		return nil
	}

	m.logger.Printf("Terminating user session: id=%s", sessionID)

	// Get all agents
	agents := session.ListAgents()

	// Terminate all agents in parallel with timeout
	if len(agents) > 0 {
		m.logger.Printf("Terminating %d agents in parallel: session=%s", len(agents), sessionID)

		var wg sync.WaitGroup
		agentTimeout := 5 * time.Second

		for role, agent := range agents {
			wg.Add(1)
			go func(r string, a *AgentSession) {
				defer wg.Done()

				// Create context with timeout for this agent
				agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
				defer cancel()

				// Close ACP client
				if acpClient := a.GetACPClient(); acpClient != nil {
					done := make(chan error, 1)
					go func() {
						done <- acpClient.Close()
					}()

					select {
					case err := <-done:
						if err != nil {
							m.logger.Printf("Error closing agent: session=%s role=%s error=%v", sessionID, r, err)
						}
					case <-agentCtx.Done():
						m.logger.Printf("Agent close timeout: session=%s role=%s", sessionID, r)
					}
				}

				// Mark agent as terminated
				a.mu.Lock()
				a.setAgentState(AgentTerminated)
				a.mu.Unlock()
			}(role, agent)
		}

		// Wait for all agents to terminate (with timeout)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			m.logger.Printf("All agents terminated: session=%s", sessionID)
		case <-ctx.Done():
			m.logger.Printf("Session termination timeout: session=%s", sessionID)
		}
	}

	// Run cleanup hook
	if err := m.cleaner.Cleanup(ctx, session); err != nil {
		m.logger.Printf("Cleanup error for session %s: %v", sessionID, err)
		// Continue with termination even if hook fails
	}

	// Mark session as terminated
	session.mu.Lock()
	session.setState(StateTerminated)
	session.mu.Unlock()

	// Remove from store
	m.store.Delete(sessionID)

	m.logger.Printf("User session terminated: id=%s", sessionID)
	return nil
}

// RecordHeartbeat updates the last activity timestamp for a user session
func (m *Manager) RecordHeartbeat(ctx context.Context, sessionID string) error {
	session := m.store.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.setLastActive(m.clock.Now())
	session.mu.Unlock()

	return nil
}

// Count returns total number of user sessions
func (m *Manager) Count() int {
	return m.store.Count()
}
