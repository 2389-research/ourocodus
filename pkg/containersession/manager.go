package containersession

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// Manager coordinates container session lifecycle with dependency injection
type Manager struct {
	dockerClient     DockerClient
	idGen            IDGenerator
	clock            Clock
	logger           Logger
	baseWorkspaceDir string

	// In-memory session tracking
	sessions map[string]*ContainerSession
	mu       sync.RWMutex
}

// NewManager creates a container session manager with injected dependencies
//
// All dependencies are required and must be non-nil. This constructor
// panics on nil collaborators because missing dependencies indicate programmer
// configuration bugs, not runtime failures.
//
// baseWorkspaceDir specifies the base directory under which all workspace paths
// must be constrained. If empty, defaults to "./workspaces".
func NewManager(dockerClient DockerClient, idGen IDGenerator, clock Clock, logger Logger, baseWorkspaceDir string) *Manager {
	if dockerClient == nil {
		panic("dockerClient cannot be nil")
	}
	if idGen == nil {
		panic("idGen cannot be nil")
	}
	if clock == nil {
		panic("clock cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}

	if baseWorkspaceDir == "" {
		baseWorkspaceDir = "./workspaces"
	}

	return &Manager{
		dockerClient:     dockerClient,
		idGen:            idGen,
		clock:            clock,
		logger:           logger,
		baseWorkspaceDir: baseWorkspaceDir,
		sessions:         make(map[string]*ContainerSession),
	}
}

// CreateSession creates a new container session with workspace and Docker container
func (m *Manager) CreateSession(ctx context.Context, imageName string, cmd []string) (*ContainerSession, error) {
	// Generate unique session ID
	sessionID := m.idGen.Generate()
	now := m.clock.Now()

	// Build labels
	labels := BuildLabels(sessionID, now)

	// Prepare workspace directory
	workspacePath, err := PrepareWorkspace(m.baseWorkspaceDir, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare workspace: %w", err)
	}

	// Create session in PENDING state
	session := NewContainerSession(sessionID, workspacePath, labels, now)

	// Store session (with TOCTOU prevention)
	m.mu.Lock()
	if _, exists := m.sessions[sessionID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrSessionAlreadyExists, sessionID)
	}
	m.sessions[sessionID] = session
	m.mu.Unlock()

	// Create Docker container
	containerConfig := &container.Config{
		Image:  imageName,
		Cmd:    cmd,
		Labels: labels,
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/workspace", workspacePath),
		},
	}

	resp, err := m.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		// Remove session from map on failure
		m.mu.Lock()
		delete(m.sessions, sessionID)
		m.mu.Unlock()

		// Clean up workspace directory
		if cleanupErr := CleanupWorkspace(workspacePath, m.logger); cleanupErr != nil {
			m.logger.Printf("Workspace cleanup failed: session=%s path=%s error=%v",
				sessionID, workspacePath, cleanupErr)
		}

		session.SetError(err.Error())
		m.logger.Printf("Container creation failed: session=%s error=%v", sessionID, err)
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	session.SetContainerID(resp.ID)
	m.logger.Printf("Container session created: id=%s container=%s state=PENDING", sessionID, resp.ID)

	return session, nil
}

// StartSession starts a container and attaches I/O streams
func (m *Manager) StartSession(ctx context.Context, sessionID string) error {
	session := m.GetSession(sessionID)
	if session == nil {
		return fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	// Validate state
	if session.State() != StatePending {
		return fmt.Errorf("%w: cannot start session in state %s", ErrInvalidState, session.State())
	}

	containerID := session.ContainerID()
	if containerID == "" {
		return fmt.Errorf("session has no container ID")
	}

	// Start container
	err := m.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		session.SetError(err.Error())
		m.logger.Printf("Container start failed: session=%s container=%s error=%v", sessionID, containerID, err)
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Attach to container I/O
	attachResp, err := m.dockerClient.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		m.logger.Printf("Container attach failed: session=%s container=%s error=%v", sessionID, containerID, err)
		// Continue even if attach fails - container is still running
	} else {
		// Start goroutines to demux stdout/stderr
		go m.handleContainerOutput(sessionID, containerID, attachResp.Reader)
	}

	session.MarkStarted(m.clock.Now())
	m.logger.Printf("Container session started: id=%s container=%s state=RUNNING", sessionID, containerID)

	return nil
}

// handleContainerOutput demultiplexes Docker container output streams
func (m *Manager) handleContainerOutput(sessionID, containerID string, reader io.Reader) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Printf("Panic in output handler: session=%s container=%s panic=%v", sessionID, containerID, r)
		}
	}()

	// Use stdcopy to demux stdout/stderr
	// For MVP, we log output - future versions can stream to clients
	_, err := stdcopy.StdCopy(
		&logWriter{logger: m.logger, prefix: fmt.Sprintf("[%s:stdout]", sessionID)},
		&logWriter{logger: m.logger, prefix: fmt.Sprintf("[%s:stderr]", sessionID)},
		reader,
	)

	if err != nil && err != io.EOF {
		m.logger.Printf("Output stream error: session=%s error=%v", sessionID, err)
	}
}

// logWriter adapts Logger to io.Writer interface
type logWriter struct {
	logger Logger
	prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.Printf("%s %s", w.prefix, string(p))
	return len(p), nil
}

// StopSession stops a running container gracefully
func (m *Manager) StopSession(ctx context.Context, sessionID string) error {
	session := m.GetSession(sessionID)
	if session == nil {
		// Idempotent - already removed
		m.logger.Printf("Session not found during stop: %s (already removed?)", sessionID)
		return nil
	}

	// Idempotent - already stopped
	state := session.State()
	if state == StateStopped || state == StateFailed {
		m.logger.Printf("Session already stopped: id=%s state=%s", sessionID, state)
		return nil
	}

	containerID := session.ContainerID()
	if containerID == "" {
		m.logger.Printf("Session has no container: id=%s", sessionID)
		return nil
	}

	// Stop container with timeout (graceful shutdown)
	timeout := 30 // seconds
	err := m.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		session.SetError(err.Error())
		m.logger.Printf("Container stop failed: session=%s container=%s error=%v", sessionID, containerID, err)
		return fmt.Errorf("failed to stop container: %w", err)
	}

	session.MarkStopped(m.clock.Now())
	m.logger.Printf("Container session stopped: id=%s container=%s state=STOPPED", sessionID, containerID)

	return nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) *ContainerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// ListSessions returns all tracked sessions
func (m *Manager) ListSessions() []*ContainerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*ContainerSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}
