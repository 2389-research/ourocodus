package containersession

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/stdcopy"
)

// Manager coordinates container session lifecycle with dependency injection
type Manager struct {
	dockerClient     DockerClient
	idGen            IDGenerator
	clock            Clock
	logger           Logger
	baseWorkspaceDir string

	// Timeout configuration (in seconds)
	stopTimeout int

	// In-memory session tracking
	sessions map[string]*ContainerSession
	mu       sync.RWMutex
}

const (
	// DefaultStopTimeout is the default graceful shutdown timeout for containers (seconds)
	DefaultStopTimeout = 30
)

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
		stopTimeout:      DefaultStopTimeout,
		sessions:         make(map[string]*ContainerSession),
	}
}

// SetStopTimeout configures the graceful shutdown timeout for containers (in seconds).
// This timeout is used when stopping containers to allow them to shut down gracefully
// before being forcefully killed. Default is 30 seconds.
func (m *Manager) SetStopTimeout(seconds int) {
	if seconds < 1 {
		seconds = DefaultStopTimeout
	}
	m.stopTimeout = seconds
}

// CreateContainerSession creates a new container session with workspace and Docker container
func (m *Manager) CreateContainerSession(ctx context.Context, imageName string, cmd []string) (*ContainerSession, error) {
	// Generate unique session ID
	sessionID := m.idGen.Generate()
	now := m.clock.Now()

	// Check for existing container with this session ID (for reuse scenarios)
	existingID, state, err := m.findContainer(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing container: %w", err)
	}

	// If container exists, handle it based on state
	if existingID != "" {
		return m.handleExistingContainer(ctx, existingID, state, sessionID)
	}

	// No existing container - proceed with normal creation
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
	m.logger.Printf("[CONTAINER] Session created: id=%s container=%s state=PENDING", sessionID, resp.ID)

	return session, nil
}

// CreateContainerSessionWithConfig creates a new container session with custom configuration.
// This method provides more control than CreateContainerSession, allowing custom mounts,
// environment variables, and workspace paths.
//
// The method automatically:
//   - Generates a unique session ID
//   - Adds default session labels (plus any custom labels from config)
//   - Creates or validates the workspace directory
//   - Adds the workspace mount (unless WorkspaceDir is empty)
//   - Merges custom mounts with the workspace mount
//   - Creates and tracks the container session
//
// Use this method when you need to:
//   - Mount additional volumes (credentials, config files, etc.)
//   - Use a pre-existing workspace directory
//   - Set custom environment variables
//   - Add custom labels beyond the defaults
//
// For simple cases, use CreateContainerSession instead.
func (m *Manager) CreateContainerSessionWithConfig(ctx context.Context, config CreateConfig) (*ContainerSession, error) {
	// Validate required fields
	if config.ImageName == "" {
		return nil, fmt.Errorf("image name is required")
	}
	if len(config.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	// Generate unique session ID
	sessionID := m.idGen.Generate()
	// Use single 'now' for both labels and session creation to ensure
	// consistent timestamps and avoid test flakiness
	now := m.clock.Now()

	// Check for existing container with this session ID (for reuse scenarios)
	existingID, state, err := m.findContainer(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing container: %w", err)
	}

	// If container exists, handle it based on state
	if existingID != "" {
		return m.handleExistingContainer(ctx, existingID, state, sessionID)
	}

	// No existing container - proceed with creation
	// Build labels (merge defaults with custom labels)
	labels := BuildLabels(sessionID, now)
	for k, v := range config.Labels {
		labels[k] = v
	}

	// Determine workspace path
	var workspacePath string
	if config.WorkspaceDir != "" {
		// Use provided workspace path
		// Basic validation: ensure path exists and is accessible
		if _, err := os.Stat(config.WorkspaceDir); err != nil {
			return nil, fmt.Errorf("workspace directory not accessible: %w", err)
		}
		// Resolve symlinks for Docker compatibility (e.g., macOS /tmp -> /private/tmp)
		var err error
		workspacePath, err = filepath.EvalSymlinks(config.WorkspaceDir)
		if err != nil {
			// If symlink resolution fails, use the original path
			workspacePath = config.WorkspaceDir
		}
	} else {
		// Create default workspace
		workspacePath, err = PrepareWorkspace(m.baseWorkspaceDir, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare workspace: %w", err)
		}
	}

	// Create session in PENDING state
	session := NewContainerSession(sessionID, workspacePath, labels, now)
	session.skipOutputLogging = config.SkipOutputLogging

	// Store session (with TOCTOU prevention)
	m.mu.Lock()
	if _, exists := m.sessions[sessionID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrSessionAlreadyExists, sessionID)
	}
	m.sessions[sessionID] = session
	m.mu.Unlock()

	// Build container config
	containerConfig := &container.Config{
		Image:        config.ImageName,
		Cmd:          config.Command,
		Labels:       labels,
		Env:          config.Env,
		OpenStdin:    true,  // Keep stdin open even when not attached
		StdinOnce:    false, // Don't close stdin after first attach
		AttachStdin:  true,  // Enable stdin attachment
		AttachStdout: true,  // Enable stdout attachment
		AttachStderr: true,  // Enable stderr attachment
		Tty:          false, // No TTY (we need raw streams for JSON-RPC)
	}

	// Override entrypoint if specified (nil = use image default, empty slice = clear entrypoint)
	if config.Entrypoint != nil {
		containerConfig.Entrypoint = config.Entrypoint
	}

	// Build host config with mounts
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: workspacePath,
			Target: "/workspace",
		},
	}
	// Add custom mounts
	mounts = append(mounts, config.CustomMounts...)

	hostConfig := &container.HostConfig{
		Mounts: mounts,
	}

	resp, err := m.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		// Remove session from map on failure
		m.mu.Lock()
		delete(m.sessions, sessionID)
		m.mu.Unlock()

		// Clean up workspace directory if we created it
		if config.WorkspaceDir == "" {
			if cleanupErr := CleanupWorkspace(workspacePath, m.logger); cleanupErr != nil {
				m.logger.Printf("Workspace cleanup failed: session=%s path=%s error=%v",
					sessionID, workspacePath, cleanupErr)
			}
		}

		session.SetError(err.Error())
		m.logger.Printf("Container creation failed: session=%s error=%v", sessionID, err)
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	session.SetContainerID(resp.ID)
	m.logger.Printf("[CONTAINER] Session created: id=%s container=%s state=PENDING mounts=%d",
		sessionID, resp.ID, len(mounts))

	return session, nil
}

// findContainer searches for an existing container by session ID using label-based filtering.
//
// Returns:
//   - containerID: Docker container ID if found, empty string if not found
//   - state: Container state ("running", "created", "exited", "paused", "dead")
//   - error: Non-nil if Docker API call fails
//
// If multiple containers are found with the same session ID (which shouldn't happen),
// returns the first one and logs a warning.
//
// This is an internal helper used by CreateContainerSession and AttachContainerSession
// to check for existing containers before creating new ones or validating attach requests.
func (m *Manager) findContainer(ctx context.Context, sessionID string) (string, string, error) {
	// Build label filters for this session
	filterArgs := BuildLabelFilters(sessionID)

	// List containers with matching labels (include stopped containers)
	listOptions := container.ListOptions{
		All:     true, // Include stopped containers
		Filters: filterArgs,
	}

	containers, err := m.dockerClient.ContainerList(ctx, listOptions)
	if err != nil {
		return "", "", fmt.Errorf("failed to list containers: %w", err)
	}

	// No containers found
	if len(containers) == 0 {
		return "", "", nil
	}

	// Multiple containers found - this shouldn't happen but handle gracefully
	if len(containers) > 1 {
		m.logger.Printf("WARNING: Multiple containers found for session %s, using first one", sessionID)
	}

	// Return first container's ID and state
	c := containers[0]
	return c.ID, c.State, nil
}

// extractAndValidateWorkspace extracts the workspace mount from a container
// and validates it's under baseWorkspaceDir.
//
// This prevents directory traversal attacks where a malicious container with
// our session labels mounts an arbitrary host path at /workspace.
func (m *Manager) extractAndValidateWorkspace(inspectData container.InspectResponse, containerID string) (string, error) {
	// Extract workspace path from volume mounts
	workspacePath := ""
	for _, mount := range inspectData.Mounts {
		if mount.Destination == "/workspace" {
			workspacePath = mount.Source
			break
		}
	}
	if workspacePath == "" {
		return "", fmt.Errorf("container %s has no /workspace mount", containerID)
	}

	// Validate workspace path to prevent directory traversal attacks
	if err := ValidateWorkspacePath(m.baseWorkspaceDir, workspacePath); err != nil {
		return "", fmt.Errorf("invalid workspace mount for container %s: %w", containerID, err)
	}

	return workspacePath, nil
}

// handleExistingContainer processes an existing container based on its state.
//
// This is an internal helper used by CreateContainerSession when findContainer()
// discovers an existing container with the same session ID.
//
// Behavior by container state:
//   - "running": Reattaches I/O streams without restarting the container
//   - "created", "exited": Starts the container, then attaches I/O streams
//   - "paused": Returns error (unpause not yet supported)
//   - "dead", "removing": Removes the bad container and returns error (caller should retry)
//
// The method extracts the workspace path from the container's volume mounts,
// validates it, and creates or retrieves the ContainerSession object.
//
// Returns the ContainerSession with proper state, or an error if the container
// cannot be reused.
func (m *Manager) handleExistingContainer(ctx context.Context, containerID, state, sessionID string) (*ContainerSession, error) {
	m.logger.Printf("Found existing container: session=%s container=%s state=%s", sessionID, containerID, state)

	// Inspect container to get full details
	inspectData, err := m.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	// Extract and validate workspace path
	workspacePath, err := m.extractAndValidateWorkspace(inspectData, containerID)
	if err != nil {
		return nil, err
	}

	// Get labels and creation time
	labels := inspectData.Config.Labels
	createdAt := m.clock.Now() // Use current time if we can't parse original
	if createdStr, ok := labels[LabelCreatedAt]; ok {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			createdAt = t
		}
	}

	// Create or retrieve session object
	session := m.getOrCreateSession(sessionID, containerID, workspacePath, labels, createdAt)

	// Handle based on container state
	switch state {
	case "running":
		return m.handleRunningContainer(ctx, session, containerID, sessionID)
	case "created", "exited":
		return m.handleStoppedContainer(ctx, session, containerID, sessionID)
	case "paused":
		return nil, fmt.Errorf("container %s is paused - unpause not yet supported", containerID)
	case "dead", "removing":
		return m.handleBadStateContainer(ctx, containerID, state, sessionID)
	default:
		return nil, fmt.Errorf("unknown container state: %s", state)
	}
}

// getOrCreateSession retrieves or creates a session with the given parameters
func (m *Manager) getOrCreateSession(sessionID, containerID, workspacePath string, labels map[string]string, createdAt time.Time) *ContainerSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		session = NewContainerSession(sessionID, workspacePath, labels, createdAt)
		m.sessions[sessionID] = session
	}
	// Always update container ID (even for existing sessions) to handle container replacement
	session.SetContainerID(containerID)
	return session
}

// attachAndStartOutput attaches to container I/O and starts output handling
func (m *Manager) attachAndStartOutput(ctx context.Context, session *ContainerSession, containerID, sessionID string) error {
	attachResp, err := m.dockerClient.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  false,
		Stdout: true,
		Stderr: true,
		Logs:   true,
	})
	if err != nil {
		m.logger.Printf("Container attach failed: session=%s container=%s error=%v", sessionID, containerID, err)
		return err
	}

	session.mu.Lock()
	// Wait for previous output handler if one exists
	if session.outputDone != nil {
		oldDone := session.outputDone
		session.mu.Unlock()
		select {
		case <-oldDone:
		case <-time.After(2 * time.Second):
			m.logger.Printf("[WARN] Previous output handler did not exit: session=%s", sessionID)
		}
		session.mu.Lock()
	}
	session.outputDone = make(chan struct{})
	outputDone := session.outputDone
	session.mu.Unlock()
	go m.handleContainerOutput(sessionID, containerID, attachResp.Reader, outputDone)
	return nil
}

// handleRunningContainer handles reattaching to an already running container
func (m *Manager) handleRunningContainer(ctx context.Context, session *ContainerSession, containerID, sessionID string) (*ContainerSession, error) {
	// Container is already running - just attach I/O
	// Continue even if attach fails - container is still usable
	_ = m.attachAndStartOutput(ctx, session, containerID, sessionID)
	session.MarkStarted(m.clock.Now())
	m.logger.Printf("Reattached to running container: session=%s container=%s", sessionID, containerID)
	return session, nil
}

// handleStoppedContainer handles starting and attaching to a stopped container
func (m *Manager) handleStoppedContainer(ctx context.Context, session *ContainerSession, containerID, sessionID string) (*ContainerSession, error) {
	// Container exists but is stopped - start it
	err := m.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start existing container: %w", err)
	}

	// Attach to I/O
	_ = m.attachAndStartOutput(ctx, session, containerID, sessionID)
	session.MarkStarted(m.clock.Now())
	m.logger.Printf("Started and attached to existing container: session=%s container=%s", sessionID, containerID)
	return session, nil
}

// handleBadStateContainer handles containers in dead or removing states
func (m *Manager) handleBadStateContainer(ctx context.Context, containerID, state, sessionID string) (*ContainerSession, error) {
	// Container is in a bad state - remove it and let caller create new one
	m.logger.Printf("Container in bad state (%s), removing: session=%s container=%s", state, sessionID, containerID)
	_ = m.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	// Remove from session map so CreateContainerSession can create new one
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	return nil, fmt.Errorf("container %s in bad state (%s), removed - retry CreateContainerSession", containerID, state)
}

// AttachContainerSession reconnects to an existing container session by session ID.
//
// This method is useful for:
//   - Reconnecting to a session after process restart
//   - Attaching from a different process or Manager instance
//   - Monitoring or debugging existing sessions
//
// Requirements:
//   - Container must exist (returns ErrSessionNotFound if not found)
//   - Container must be in "running" state (returns error for stopped/dead containers)
//   - Container must have /workspace mount (returns error if missing)
//
// The method automatically:
//   - Locates the container using label-based filtering
//   - Extracts workspace path from volume mounts
//   - Attaches I/O streams for output monitoring
//   - Creates or updates the in-memory ContainerSession object
//
// I/O attachment failures are logged but do not fail the operation, as the
// container is still accessible even if stream attachment fails.
//
// Example:
//
//	// Process A creates and stores session ID
//	session, _ := manager.CreateContainerSession(ctx, "ubuntu:latest", []string{"/bin/bash"})
//	sessionID := session.ID()
//	saveSessionID(sessionID) // persist to database/file
//
//	// Process B reconnects to the same session
//	sessionID := loadSessionID()
//	manager2 := NewManager(...) // new Manager instance
//	reattached, err := manager2.AttachContainerSession(ctx, sessionID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// reattached is now connected to the same running container
func (m *Manager) AttachContainerSession(ctx context.Context, sessionID string) (*ContainerSession, error) {
	m.logger.Printf("Attempting to attach to session: %s", sessionID)

	// Find the container
	containerID, state, err := m.findContainer(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find container: %w", err)
	}

	if containerID == "" {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	// Only allow attaching to running containers
	if state != "running" {
		return nil, fmt.Errorf("cannot attach to container in state %s (must be running)", state)
	}

	// Inspect container to get full details
	inspectData, err := m.dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	// Extract and validate workspace path
	workspacePath, err := m.extractAndValidateWorkspace(inspectData, containerID)
	if err != nil {
		return nil, err
	}

	// Get labels and creation time
	labels := inspectData.Config.Labels
	createdAt := m.clock.Now()
	if createdStr, ok := labels[LabelCreatedAt]; ok {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			createdAt = t
		}
	}

	// Create or retrieve session object
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if !exists {
		session = NewContainerSession(sessionID, workspacePath, labels, createdAt)
		m.sessions[sessionID] = session
	}
	// Always update container ID when reattaching, even if session exists
	// This handles the case where a container was restarted externally
	session.SetContainerID(containerID)
	m.mu.Unlock()

	// Mark session as started (after releasing lock, consistent with handleExistingContainer)
	session.MarkStarted(m.clock.Now())

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
		// Return session anyway - container is still accessible even if I/O attach failed
	} else {
		session.mu.Lock()
		// Wait for previous output handler if one exists
		if session.outputDone != nil {
			oldDone := session.outputDone
			session.mu.Unlock()
			select {
			case <-oldDone:
			case <-time.After(2 * time.Second):
				m.logger.Printf("[WARN] Previous output handler did not exit: session=%s", sessionID)
			}
			session.mu.Lock()
		}
		session.outputDone = make(chan struct{})
		outputDone := session.outputDone
		session.mu.Unlock()
		go m.handleContainerOutput(sessionID, containerID, attachResp.Reader, outputDone)
	}

	m.logger.Printf("Successfully attached to session: id=%s container=%s", sessionID, containerID)
	return session, nil
}

// StartContainerSession starts a container and attaches I/O streams
func (m *Manager) StartContainerSession(ctx context.Context, sessionID string) error {
	session := m.GetContainerSession(sessionID)
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

	// Attach to container I/O for logging (unless external attachment is used)
	if !session.skipOutputLogging {
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
			session.mu.Lock()
			// Wait for previous output handler if one exists
			if session.outputDone != nil {
				oldDone := session.outputDone
				session.mu.Unlock()
				select {
				case <-oldDone:
				case <-time.After(2 * time.Second):
					m.logger.Printf("[WARN] Previous output handler did not exit: session=%s", sessionID)
				}
				session.mu.Lock()
			}
			session.outputDone = make(chan struct{})
			outputDone := session.outputDone
			session.mu.Unlock()
			go m.handleContainerOutput(sessionID, containerID, attachResp.Reader, outputDone)
		}
	}

	session.MarkStarted(m.clock.Now())
	m.logger.Printf("[CONTAINER] Session started: id=%s container=%s state=RUNNING", sessionID, containerID)

	return nil
}

// handleContainerOutput demultiplexes Docker container output streams
// The outputDone channel is passed as a parameter to avoid TOCTOU races where
// the session could be deleted or updated between checking and closing the channel.
func (m *Manager) handleContainerOutput(sessionID, containerID string, reader io.Reader, outputDone chan struct{}) {
	// Close the done channel when output handling completes
	if outputDone != nil {
		defer close(outputDone)
	}

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

// StopContainerSession stops a running container gracefully
func (m *Manager) StopContainerSession(ctx context.Context, sessionID string) error {
	session := m.GetContainerSession(sessionID)
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

	// Stop container with configured timeout (graceful shutdown)
	timeout := m.stopTimeout
	err := m.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		session.SetError(err.Error())
		m.logger.Printf("Container stop failed: session=%s container=%s error=%v", sessionID, containerID, err)
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Wait for output handler goroutine with 5-second timeout
	session.mu.Lock()
	outputDone := session.outputDone
	session.mu.Unlock()
	if outputDone != nil {
		select {
		case <-outputDone:
			// Clean shutdown
		case <-time.After(5 * time.Second):
			m.logger.Printf("[WARN] handleContainerOutput goroutine did not exit within timeout: session=%s", sessionID)
		}
	}

	session.MarkStopped(m.clock.Now())
	m.logger.Printf("[CONTAINER] Session stopped: id=%s container=%s state=STOPPED", sessionID, containerID)

	return nil
}

// GetContainerSession retrieves a session by ID
func (m *Manager) GetContainerSession(sessionID string) *ContainerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// ListContainerSessions returns all tracked sessions
func (m *Manager) ListContainerSessions() []*ContainerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*ContainerSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetDockerClient returns the Docker client used by this manager
func (m *Manager) GetDockerClient() DockerClient {
	return m.dockerClient
}
