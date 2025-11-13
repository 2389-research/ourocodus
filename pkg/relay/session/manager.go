package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/runtime"
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

	// NEW: Launcher management
	launcherFactory agent.LauncherFactory
	launchers       map[string]agent.AgentLauncher // "sessionID:agentID" → launcher
	handles         map[string]agent.AgentHandle   // "sessionID:agentID" → handle
	launchersMu     sync.RWMutex                   // protects launchers/handles
}

// NewManager creates a session manager with injected dependencies.
//
// All dependencies are required and must be non-nil (except publisher and launcherFactory).
// This constructor panics on nil collaborators because missing dependencies indicate
// programmer configuration bugs, not runtime failures.
//
// baseWorkspaceDir specifies the base directory under which all workspace paths
// must be constrained. If empty, defaults to "./workspaces".
//
// publisher is optional and can be nil. If nil, event publishing is disabled.
// launcherFactory is optional and can be nil. If nil, container spawning is disabled.
func NewManager(store Store, idGen IDGenerator, clock Clock, cleaner Cleaner, logger Logger, clientFactory ClientFactory, baseWorkspaceDir string, publisher EventPublisher, launcherFactory agent.LauncherFactory) *Manager {
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
		launcherFactory:  launcherFactory,                      // NEW
		launchers:        make(map[string]agent.AgentLauncher), // NEW
		handles:          make(map[string]agent.AgentHandle),   // NEW
	}
}

// launcherKey generates a composite key for session-scoped launcher/handle maps.
// This prevents collisions when multiple sessions use the same agentID.
func launcherKey(sessionID, agentID string) string {
	return sessionID + ":" + agentID
}

// isContainerModeEnabled checks if container mode is enabled and launcher factory is available.
func (m *Manager) isContainerModeEnabled() bool {
	return m.launcherFactory != nil && runtime.IsContainerMode()
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

	// Store userSession
	if err := m.store.Create(userSession); err != nil {
		return nil, fmt.Errorf("failed to store user session: %w", err)
	}

	// Publish session.created event (synchronous, errors logged but non-fatal)
	if m.publisher != nil {
		if err := m.publisher.PublishSessionCreated(ctx, userSessionID); err != nil {
			m.logger.Printf("WARN: Failed to publish session.created event: %v", err)
		}
	}

	m.logger.Printf("[SESSION] User session created: id=%s state=ACTIVE agents=0", userSessionID)
	return userSession, nil
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
	if userSession.state != StateActive {
		userSession.mu.Unlock()
		return fmt.Errorf("session %s is not active (state=%s)", userSessionID, userSession.state)
	}
	if userSession.agents[agentID] != nil {
		userSession.mu.Unlock()
		return fmt.Errorf("agent %s already exists in session %s", agentID, userSessionID)
	}
	userSession.mu.Unlock()

	m.logger.Printf("[SESSION] Starting agent spawn process: session=%s agent=%s", userSessionID, agentID)
	m.logger.Printf("[SESSION] ├─ Workspace path: %s", absPath)

	// Create workspace directory if needed (I/O - no lock held)
	// Use 0o700 for strict workspace isolation (owner-only access)
	err = os.MkdirAll(absPath, 0o700)
	if err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create agent session in SPAWNING state
	now := m.clock.Now()
	agentSession := NewAgentSession(agentID, absPath, now)

	// Re-check and add agent atomically (prevents TOCTOU race)
	userSession.mu.Lock()

	// Re-check state (session could have been terminated during I/O)
	if userSession.state != StateActive {
		userSession.mu.Unlock()
		return fmt.Errorf("session %s is not active (state=%s)", userSessionID, userSession.state)
	}

	// Re-check agent doesn't exist (another goroutine could have added it)
	if userSession.agents[agentID] != nil {
		userSession.mu.Unlock()
		return fmt.Errorf("agent %s already exists in session %s", agentID, userSessionID)
	}

	// Add agent to session in SPAWNING state
	userSession.addAgent(agentSession)
	userSession.setLastActive(now)
	userSession.mu.Unlock()

	// Create launcher ONLY in container mode
	var handle agent.AgentHandle
	if m.isContainerModeEnabled() {
		// Container mode: Use image default (ENTRYPOINT ["/usr/local/bin/acp"])
		// ContainerAttachProcessLauncher will attach to the container's stdio
		// where ACP runs as the main process. The CMD from image provides default args.
		command := []string{"--workspace", "/workspace"} // Args for ACP
		m.logger.Printf("[SESSION] ├─ Runtime mode: CONTAINER (ACP runs as main process, stdio attached)")
		m.logger.Printf("[SESSION] ├─ Container command (ACP args): %v", command)

		// Get API key from client factory for container environment
		var anthropicKey string
		if acpFactory, ok := m.clientFactory.(*ACPClientFactory); ok {
			anthropicKey = acpFactory.GetAPIKey()
		}

		launcherConfig := agent.LauncherConfig{
			AgentID:      agentID,
			ImageName:    "ourocodus/agent:latest", // TODO: make configurable
			Command:      command,
			Workspace:    absPath,
			AnthropicKey: anthropicKey,
			// Git credentials will be added in future task
		}

		m.logger.Printf("[SESSION] ├─ Creating container launcher (image: %s)", launcherConfig.ImageName)

		launcher, err := m.launcherFactory.CreateLauncher(ctx, agentID, launcherConfig)
		if err != nil {
			// Mark agent as FAILED and cleanup
			agentSession.mu.Lock()
			agentSession.setAgentState(AgentFailed)
			agentSession.setError(err.Error())
			agentSession.mu.Unlock()

			m.logger.Printf("[SESSION] ✗ Failed to create launcher: %v", err)
			return fmt.Errorf("failed to create launcher: %w", err)
		}
		m.logger.Printf("[SESSION] ✓ Launcher created successfully")

		// Spawn agent container
		m.logger.Printf("[SESSION] ├─ Spawning agent container...")
		spawnConfig := &agent.SpawnConfig{
			Role:      agentID,
			Image:     launcherConfig.ImageName,
			Command:   launcherConfig.Command,
			Workspace: absPath,
		}

		handle, err = launcher.Spawn(ctx, spawnConfig)
		if err != nil {
			// Mark agent as FAILED and cleanup
			agentSession.mu.Lock()
			agentSession.setAgentState(AgentFailed)
			agentSession.setError(err.Error())
			agentSession.mu.Unlock()

			m.logger.Printf("[SESSION] ✗ Container spawn failed: %v", err)
			return fmt.Errorf("failed to spawn agent: %w", err)
		}
		m.logger.Printf("[SESSION] ✓ Container spawned (id: %s)", handle.ContainerID())

		// Store launcher and handle
		key := launcherKey(userSessionID, agentID)
		m.launchersMu.Lock()
		m.launchers[key] = launcher
		m.handles[key] = handle
		m.launchersMu.Unlock()
	} else {
		// Host mode: ACP will run directly on host via os/exec
		m.logger.Printf("[SESSION] ├─ Runtime mode: HOST (ACP will run directly on host)")
	}

	runtimeCtx := &AgentRuntimeContext{
		SessionID: userSessionID,
		AgentID:   agentID,
		Workspace: absPath,
	}
	if handle != nil {
		runtimeCtx.ContainerID = handle.ContainerID()
		m.logger.Printf("[SESSION] ├─ Creating ACP client for container %s", handle.ContainerID())
	} else {
		m.logger.Printf("[SESSION] ├─ Creating ACP client for host process")
	}

	// Spawn ACP client (I/O - no lock held)
	acpClient, err := m.clientFactory.NewClient(ctx, runtimeCtx)
	if err != nil {
		m.logger.Printf("[SESSION] ✗ Failed to create ACP client: %v", err)

		// Cleanup launcher on client creation failure
		if m.isContainerModeEnabled() {
			key := launcherKey(userSessionID, agentID)
			m.launchersMu.Lock()
			launcher := m.launchers[key]
			handle := m.handles[key]
			delete(m.launchers, key)
			delete(m.handles, key)
			m.launchersMu.Unlock()

			if launcher != nil && handle != nil {
				m.logger.Printf("[SESSION] ├─ Cleaning up container after ACP client failure")
				_ = launcher.Stop(ctx, handle)
			}
		}

		// Mark agent as FAILED
		agentSession.mu.Lock()
		agentSession.setAgentState(AgentFailed)
		agentSession.setError(err.Error())
		agentSession.mu.Unlock()

		return fmt.Errorf("failed to spawn ACP client: %w", err)
	}
	m.logger.Printf("[SESSION] ✓ ACP client created successfully")

	// Transition agent to ACTIVE
	agentSession.mu.Lock()
	agentSession.setACPClient(acpClient)
	agentSession.setAgentState(AgentActive)
	agentSession.setAgentLastActive(m.clock.Now())
	agentSession.mu.Unlock()

	m.logger.Printf("[SESSION] ✓ Agent '%s' is now ACTIVE (session: %s)", agentID, userSessionID)

	// Publish agent.spawned event (synchronous, errors logged but non-fatal)
	if m.publisher != nil {
		if err := m.publisher.PublishAgentSpawned(ctx, userSessionID, agentID, absPath); err != nil {
			m.logger.Printf("WARN: Failed to publish agent.spawned event: %v", err)
		}
	}

	m.logger.Printf("[SESSION] Agent spawned: session=%s agentID=%s state=ACTIVE", userSessionID, agentID)
	return nil
}

// GetAgent returns the agent session for a given agentID within a user session
func (m *Manager) GetAgent(userSessionID, agentID string) (*AgentSession, error) {
	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, userSessionID)
	}

	agent := userSession.GetAgent(agentID)
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

	return userSession.ListAgents(), nil
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

	agent := userSession.GetAgent(agentID)
	if agent == nil {
		// Already removed - idempotent
		m.logger.Printf("Agent not found during termination: session=%s agentID=%s (already terminated?)", userSessionID, agentID)
		return nil
	}

	m.logger.Printf("[SESSION] Terminating agent: session=%s agentID=%s", userSessionID, agentID)

	// Stop container if launcher exists (container mode only)
	// Use atomic take-and-delete pattern to prevent double-stop race (Issue #210)
	if m.isContainerModeEnabled() {
		key := launcherKey(userSessionID, agentID)

		// Atomic take-and-delete under Lock
		m.launchersMu.Lock()
		launcher := m.launchers[key]
		handle := m.handles[key]
		delete(m.launchers, key) // Delete BEFORE releasing lock
		delete(m.handles, key)
		m.launchersMu.Unlock()

		// Now safe - only one goroutine has these pointers
		if launcher != nil && handle != nil {
			if err := launcher.Stop(ctx, handle); err != nil {
				m.logger.Printf("WARN: Failed to stop container for agent %s: %v", agentID, err)
				// Continue cleanup despite error
			}
		}
	}

	// Close ACP client if present (with double-close protection)
	agent.mu.Lock()
	acpClient := agent.acpClient
	if acpClient != nil {
		agent.acpClient = nil // Clear before Close to prevent double-close
	}
	agent.setAgentState(AgentTerminated)
	agent.mu.Unlock()

	// Close outside the lock with context
	if acpClient != nil {
		if err := acpClient.Close(ctx); err != nil {
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

	m.logger.Printf("[SESSION] Agent terminated: session=%s agentID=%s", userSessionID, agentID)
	return nil
}

// TerminateUserSession terminates ALL agents in parallel, then terminates the session
// Idempotent - safe to call multiple times

func (m *Manager) TerminateUserSession(ctx context.Context, userSessionID string) (TerminationSummary, error) {
	summary := TerminationSummary{CleanupStatus: CleanupStatusComplete}

	userSession := m.store.Get(userSessionID)
	if userSession == nil {
		// Already cleaned up - idempotent
		m.logger.Printf("Session not found during termination: %s (already cleaned?)", userSessionID)
		return summary, nil
	}

	// Set state to TERMINATED immediately to prevent new agent spawns during termination
	userSession.mu.Lock()
	userSession.setState(StateTerminated)
	userSession.mu.Unlock()

	m.logger.Printf("[SESSION] Terminating user session: id=%s", userSessionID)

	// Get all agents
	agents := userSession.ListAgents()

	// Terminate all agents in parallel with timeout
	if len(agents) > 0 {
		m.logger.Printf("[SESSION] Terminating %d agents in parallel: session=%s", len(agents), userSessionID)

		var wg sync.WaitGroup
		agentTimeout := DefaultAgentTerminationTimeout
		var agentSuccesses int32
		var agentFailures int32

		for agentID, agent := range agents {
			wg.Add(1)
			go func(id string, a *AgentSession) {
				defer wg.Done()
				failed := false

				// Create dedicated shutdown context independent of request context
				// This ensures cleanup completes even if HTTP request times out
				shutdownCtx, cancel := context.WithTimeout(context.Background(), agentTimeout)
				defer cancel()

				// Close ACP client (with double-close protection)
				a.mu.Lock()
				acpClient := a.acpClient
				if acpClient != nil {
					a.acpClient = nil // Clear before Close to prevent double-close
				}
				a.setAgentState(AgentTerminated)
				a.mu.Unlock()

				// Close outside the lock (no goroutine wrapper needed - Close respects context)
				if acpClient != nil {
					if err := acpClient.Close(shutdownCtx); err != nil {
						m.logger.Printf("Error closing agent: userSession=%s agentID=%s error=%v", userSessionID, id, err)
						failed = true
					}
				}

				// Stop container if launcher exists (container mode only)
				// Use atomic take-and-delete pattern to prevent double-stop race (Issue #210)
				if m.isContainerModeEnabled() {
					key := launcherKey(userSessionID, id)

					// Atomic take-and-delete under Lock
					m.launchersMu.Lock()
					launcher := m.launchers[key]
					handle := m.handles[key]
					delete(m.launchers, key) // Delete BEFORE releasing lock
					delete(m.handles, key)
					m.launchersMu.Unlock()

					// Now safe - only one goroutine has these pointers
					if launcher != nil && handle != nil {
						if err := launcher.Stop(shutdownCtx, handle); err != nil {
							m.logger.Printf("WARN: Failed to stop container for agent %s: %v", id, err)
							failed = true
							// Continue cleanup despite error
						}
					}
				}
				if failed {
					atomic.AddInt32(&agentFailures, 1)
				} else {
					atomic.AddInt32(&agentSuccesses, 1)
				}
			}(agentID, agent)
		}

		// Wait for all agents to terminate (with timeout)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			m.logger.Printf("[SESSION] All agents terminated: session=%s", userSessionID)
		case <-ctx.Done():
			m.logger.Printf("[SESSION] Session termination timeout: session=%s", userSessionID)
			summary.CleanupStatus = CleanupStatusPartial
			summary.addError("session termination timeout before all agents completed")
		}

		summary.AgentsTerminated = int(atomic.LoadInt32(&agentSuccesses))
		summary.AgentFailures = int(atomic.LoadInt32(&agentFailures))
		if summary.AgentFailures > 0 {
			if summary.AgentsTerminated == 0 {
				summary.CleanupStatus = CleanupStatusFailed
			} else if summary.CleanupStatus == CleanupStatusComplete {
				summary.CleanupStatus = CleanupStatusPartial
			}
		}
	}

	// Run cleanup hook
	if err := m.cleaner.Cleanup(ctx, userSession); err != nil {
		m.logger.Printf("Cleanup error for user session %s: %v", userSessionID, err)
		summary.addError(err.Error())
		if summary.CleanupStatus == CleanupStatusComplete {
			summary.CleanupStatus = CleanupStatusPartial
		}
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

	m.logger.Printf("[SESSION] User session terminated: id=%s", userSessionID)
	return summary, nil
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
