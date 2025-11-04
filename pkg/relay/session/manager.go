package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultAgentTerminationTimeout is the maximum time to wait for a single agent to close
	// during parallel termination. Can be tuned based on deployment needs.
	DefaultAgentTerminationTimeout = 5 * time.Second
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
	store            Store
	idGen            IDGenerator
	clock            Clock
	cleaner          Cleaner
	logger           Logger
	clientFactory    ClientFactory
	baseWorkspaceDir string
	publisher        EventPublisher // Optional: publishes lifecycle events to NATS
}

// NewManager creates a session manager with injected dependencies.
//
// All dependencies are required and must be non-nil (except publisher). This constructor
// panics on nil collaborators because missing dependencies indicate programmer configuration
// bugs, not runtime failures.
//
// baseWorkspaceDir specifies the base directory under which all workspace paths
// must be constrained. If empty, defaults to "./workspaces".
//
// publisher is optional and can be nil. If nil, event publishing is disabled.
func NewManager(store Store, idGen IDGenerator, clock Clock, cleaner Cleaner, logger Logger, clientFactory ClientFactory, baseWorkspaceDir string, publisher EventPublisher) *Manager {
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

	if baseWorkspaceDir == "" {
		baseWorkspaceDir = "./workspaces"
	}

	return &Manager{
		store:            store,
		idGen:            idGen,
		clock:            clock,
		cleaner:          cleaner,
		logger:           logger,
		clientFactory:    clientFactory,
		baseWorkspaceDir: baseWorkspaceDir,
		publisher:        publisher,
	}
}

// CreateUserSession creates a new user session in ACTIVE state with no agents
// Session starts empty - agents are spawned separately via SpawnAgent
func (m *Manager) CreateUserSession(ctx context.Context, ws WebSocketConn) (*UserSession, error) {
	// Validate input
	if ws == nil {
		return nil, ErrWebSocketNil
	}

	// Generate unique ID and create session
	userSessionID := m.idGen.Generate()
	now := m.clock.Now()
	userSession := NewUserSession(userSessionID, ws, now)

	// Store session
	if err := m.store.Create(session); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	// Publish session.created event (synchronous, errors logged but non-fatal)
	if m.publisher != nil {
		if err := m.publisher.PublishSessionCreated(ctx, userSessionID); err != nil {
			m.logger.Printf("WARN: Failed to publish session.created event: %v", err)
		}
	}

	m.logger.Printf("User session created: id=%s state=ACTIVE agents=0", userSessionID)
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
//
//nolint:gocyclo // Complexity required for TOCTOU prevention and proper error handling
func (m *Manager) SpawnAgent(ctx context.Context, userSessionID, agentID, workspace string) error {
	// Validate inputs
	if strings.TrimSpace(agentID) == "" {
		return ErrEmptyAgentID
	}
	if strings.TrimSpace(workspace) == "" {
		return ErrEmptyWorkspace
	}

	// Validate and constrain workspace path under base directory
	cleanPath := filepath.Clean(workspace)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid workspace path: %w", err)
	}

	baseAbs, err := filepath.Abs(m.baseWorkspaceDir)
	if err != nil {
		return fmt.Errorf("invalid base workspace directory: %w", err)
	}

	// Defense-in-depth: Check prefix with separator to prevent directory name bypass
	if absPath != baseAbs && !strings.HasPrefix(absPath, baseAbs+string(os.PathSeparator)) {
		return fmt.Errorf("workspace path must be under base directory %s", m.baseWorkspaceDir)
	}

	// Use filepath.Rel to prevent directory traversal with ".."
	relPath, err := filepath.Rel(baseAbs, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") || relPath == ".." || filepath.IsAbs(relPath) {
		return fmt.Errorf("workspace path must be under base directory %s", m.baseWorkspaceDir)
	}

	// Get user session
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, userSessionID)
	}

	// Initial validation with lock (check-lock-check pattern to prevent TOCTOU)
	userSession.mu.Lock()
	if session.state != StateActive {
		userSession.mu.Unlock()
		return fmt.Errorf("session %s is not active (state=%s)", userSessionID, session.state)
	}
	if session.agents[agentID] != nil {
		userSession.mu.Unlock()
		return fmt.Errorf("agent %s already exists in session %s", agentID, userSessionID)
	}
	userSession.mu.Unlock()

	m.logger.Printf("Spawning agent: session=%s agentID=%s workspace=%s", userSessionID, agentID, absPath)

	// Create workspace directory if needed (I/O - no lock held)
	// Use 0o700 for strict workspace isolation (owner-only access)
	err = os.MkdirAll(absPath, 0o700)
	if err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create agent session in SPAWNING state
	now := m.clock.Now()
	agent := NewAgentSession(agentID, absPath, now)

	// Re-check and add agent atomically (prevents TOCTOU race)
	userSession.mu.Lock()

	// Re-check state (session could have been terminated during I/O)
	if session.state != StateActive {
		userSession.mu.Unlock()
		return fmt.Errorf("session %s is not active (state=%s)", userSessionID, session.state)
	}

	// Re-check agent doesn't exist (another goroutine could have added it)
	if session.agents[agentID] != nil {
		userSession.mu.Unlock()
		return fmt.Errorf("agent %s already exists in session %s", agentID, userSessionID)
	}

	// Add agent to session in SPAWNING state
	userSession.addAgent(agent)
	userSession.setLastActive(now)
	userSession.mu.Unlock()

	// Spawn ACP client (I/O - no lock held)
	acpClient, err := m.clientFactory.NewClient(absPath)
	if err != nil {
		// Mark agent as FAILED
		agent.mu.Lock()
		agent.setAgentState(AgentFailed)
		agent.setError(err.Error())
		agent.mu.Unlock()

		m.logger.Printf("Agent spawn failed: session=%s agentID=%s error=%v", userSessionID, agentID, err)
		return fmt.Errorf("failed to spawn ACP client: %w", err)
	}

	// Transition agent to ACTIVE
	agent.mu.Lock()
	agent.setACPClient(acpClient)
	agent.setAgentState(AgentActive)
	agent.setAgentLastActive(m.clock.Now())
	agent.mu.Unlock()

	// Publish agent.spawned event (synchronous, errors logged but non-fatal)
	if m.publisher != nil {
		if err := m.publisher.PublishAgentSpawned(ctx, userSessionID, agentID, absPath); err != nil {
			m.logger.Printf("WARN: Failed to publish agent.spawned event: %v", err)
		}
	}

	m.logger.Printf("Agent spawned: session=%s agentID=%s state=ACTIVE", userSessionID, agentID)
	return nil
}

// GetAgent returns the agent session for a given agentID within a user session
func (m *Manager) GetAgent(userSessionID, agentID string) (*AgentSession, error) {
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, userSessionID)
	}

	agent := session.GetAgent(agentID)
	if agent == nil {
		return nil, fmt.Errorf("%w: agentID=%s session=%s", ErrAgentNotFound, agentID, userSessionID)
	}

	return agent, nil
}

// ListAgents returns all agents in a user session
func (m *Manager) ListAgents(userSessionID string) (map[string]*AgentSession, error) {
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, userSessionID)
	}

	return session.ListAgents(), nil
}

// GetAgentHistory returns the conversation history for a specific agent
func (m *Manager) GetAgentHistory(userSessionID, agentID string) ([]Message, error) {
	agent, err := m.GetAgent(userSessionID, agentID)
	if err != nil {
		return nil, err
	}

	return agent.GetHistory(), nil
}

// TerminateAgent terminates ONE agent in a user session
// User session stays ACTIVE, other agents unaffected
func (m *Manager) TerminateAgent(ctx context.Context, userSessionID, agentID string) error {
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		// Already cleaned up - idempotent
		m.logger.Printf("Session not found during agent termination: %s (already cleaned?)", userSessionID)
		return nil
	}

	agent := session.GetAgent(agentID)
	if agent == nil {
		// Already removed - idempotent
		m.logger.Printf("Agent not found during termination: session=%s agentID=%s (already terminated?)", userSessionID, agentID)
		return nil
	}

	m.logger.Printf("Terminating agent: session=%s agentID=%s", userSessionID, agentID)

	// Close ACP client if present (with double-close protection)
	agent.mu.Lock()
	acpClient := agent.acpClient
	if acpClient != nil {
		agent.acpClient = nil // Clear before Close to prevent double-close
	}
	agent.setAgentState(AgentTerminated)
	agent.mu.Unlock()

	// Close outside the lock
	if acpClient != nil {
		if err := acpClient.Close(); err != nil {
			m.logger.Printf("Error closing ACP client: session=%s agentID=%s error=%v", userSessionID, agentID, err)
			// Continue with cleanup even if close fails
		}
	}

	userSession.mu.Lock()
	userSession.removeAgent(agentID)
	userSession.setLastActive(m.clock.Now())
	userSession.mu.Unlock()

	// Publish agent.terminated event (synchronous, errors logged but non-fatal)
	// TODO: Capture actual exit code when available from ACP client
	if m.publisher != nil {
		if err := m.publisher.PublishAgentTerminated(ctx, userSessionID, agentID, 0); err != nil {
			m.logger.Printf("WARN: Failed to publish agent.terminated event: %v", err)
		}
	}

	m.logger.Printf("Agent terminated: session=%s agentID=%s", userSessionID, agentID)
	return nil
}

// TerminateUserSession terminates ALL agents in parallel, then terminates the session
// Idempotent - safe to call multiple times
func (m *Manager) TerminateUserSession(ctx context.Context, userSessionID string) error {
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		// Already cleaned up - idempotent
		m.logger.Printf("Session not found during termination: %s (already cleaned?)", userSessionID)
		return nil
	}

	// Set state to TERMINATED immediately to prevent new agent spawns during termination
	userSession.mu.Lock()
	userSession.setState(StateTerminated)
	userSession.mu.Unlock()

	m.logger.Printf("Terminating user session: id=%s", userSessionID)

	// Get all agents
	agents := session.ListAgents()

	// Terminate all agents in parallel with timeout
	if len(agents) > 0 {
		m.logger.Printf("Terminating %d agents in parallel: session=%s", len(agents), userSessionID)

		var wg sync.WaitGroup
		agentTimeout := DefaultAgentTerminationTimeout

		for role, agent := range agents {
			wg.Add(1)
			go func(r string, a *AgentSession) {
				defer wg.Done()

				// Create context with timeout for this agent
				agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
				defer cancel()

				// Close ACP client (with double-close protection)
				a.mu.Lock()
				acpClient := a.acpClient
				if acpClient != nil {
					a.acpClient = nil // Clear before Close to prevent double-close
				}
				a.setAgentState(AgentTerminated)
				a.mu.Unlock()

				// Close outside the lock
				if acpClient != nil {
					done := make(chan error, 1)
					go func() {
						done <- acpClient.Close()
					}()

					select {
					case err := <-done:
						if err != nil {
							m.logger.Printf("Error closing agent: session=%s role=%s error=%v", userSessionID, r, err)
						}
					case <-agentCtx.Done():
						m.logger.Printf("Agent close timeout: session=%s role=%s", userSessionID, r)
					}
				}
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
			m.logger.Printf("All agents terminated: session=%s", userSessionID)
		case <-ctx.Done():
			m.logger.Printf("Session termination timeout: session=%s", userSessionID)
		}
	}

	// Run cleanup hook
	if err := m.cleaner.Cleanup(ctx, session); err != nil {
		m.logger.Printf("Cleanup error for session %s: %v", userSessionID, err)
		// Continue with termination even if hook fails
	}

	// Note: WebSocket ownership belongs to server layer
	// Server is responsible for closing the connection, not the session manager

	// Remove from store (state already set to TERMINATED above)
	m.store.Delete(userSessionID)

	// Publish session.terminated event (synchronous, errors logged but non-fatal)
	if m.publisher != nil {
		if err := m.publisher.PublishSessionTerminated(ctx, userSessionID); err != nil {
			m.logger.Printf("WARN: Failed to publish session.terminated event: %v", err)
		}
	}

	m.logger.Printf("User session terminated: id=%s", userSessionID)
	return nil
}

// RecordHeartbeat updates the last activity timestamp for a user session
func (m *Manager) RecordHeartbeat(ctx context.Context, userSessionID string) error {
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, userSessionID)
	}

	userSession.mu.Lock()
	userSession.setLastActive(m.clock.Now())
	userSession.mu.Unlock()

	return nil
}

// Count returns total number of user sessions
func (m *Manager) Count() int {
	return m.store.Count()
}
