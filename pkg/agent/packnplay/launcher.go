package packnplay

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/obra/packnplay/pkg/config"
	"github.com/obra/packnplay/pkg/git"
	"github.com/obra/packnplay/pkg/runner"
	"github.com/oklog/ulid/v2"

	"github.com/2389-research/ourocodus/pkg/agent"
)

// PacknplayLauncher implements the AgentLauncher interface using Packnplay.
type PacknplayLauncher struct {
	mu           sync.RWMutex
	dockerClient *client.Client
	handles      map[string]*PacknplayHandle
	projectPath  string
	defaultImage string
	runtime      string
	verbose      bool
}

// LauncherOption configures the PacknplayLauncher.
type LauncherOption func(*PacknplayLauncher) error

// WithProjectPath sets the project path (monorepo root).
func WithProjectPath(path string) LauncherOption {
	return func(l *PacknplayLauncher) error {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve project path: %w", err)
		}
		l.projectPath = absPath
		return nil
	}
}

// WithDefaultImage sets the default container image.
func WithDefaultImage(image string) LauncherOption {
	return func(l *PacknplayLauncher) error {
		l.defaultImage = image
		return nil
	}
}

// WithRuntime sets the container runtime (docker, podman, or container).
func WithRuntime(runtime string) LauncherOption {
	return func(l *PacknplayLauncher) error {
		l.runtime = runtime
		return nil
	}
}

// WithVerbose enables verbose logging.
func WithVerbose(verbose bool) LauncherOption {
	return func(l *PacknplayLauncher) error {
		l.verbose = verbose
		return nil
	}
}

// WithDockerClient injects a Docker client (useful for testing).
func WithDockerClient(dockerClient *client.Client) LauncherOption {
	return func(l *PacknplayLauncher) error {
		l.dockerClient = dockerClient
		return nil
	}
}

// WithDockerHost sets a custom Docker host (e.g., for Colima).
// Example: "unix://$HOME/.colima/default/docker.sock"
// If not set, uses DOCKER_HOST environment variable or default socket.
func WithDockerHost(host string) LauncherOption {
	return func(l *PacknplayLauncher) error {
		if l.dockerClient != nil {
			return fmt.Errorf("cannot set Docker host when Docker client is already set")
		}
		dockerClient, err := client.NewClientWithOpts(
			client.WithHost(host),
			client.WithAPIVersionNegotiation(),
		)
		if err != nil {
			return fmt.Errorf("failed to create Docker client with host %s: %w", host, err)
		}
		l.dockerClient = dockerClient
		return nil
	}
}

// NewLauncher creates a new PacknplayLauncher with the given options.
func NewLauncher(opts ...LauncherOption) (*PacknplayLauncher, error) {
	l := &PacknplayLauncher{
		handles:      make(map[string]*PacknplayHandle),
		defaultImage: "ubuntu:22.04",
		runtime:      "docker",
		verbose:      false,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(l); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// If no project path set, use current directory
	if l.projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		l.projectPath = cwd
	}

	// Initialize Docker client if not provided
	if l.dockerClient == nil {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("failed to create Docker client: %w", err)
		}
		l.dockerClient = dockerClient
	}

	// Verify project path is a git repo (required for worktrees)
	if !git.IsGitRepo(l.projectPath) {
		return nil, fmt.Errorf("project path %s is not a git repository (required for Packnplay worktrees)", l.projectPath)
	}

	return l, nil
}

// Spawn creates and starts a new agent instance with the given configuration.
func (l *PacknplayLauncher) Spawn(ctx context.Context, cfg *agent.SpawnConfig) (agent.AgentHandle, error) {
	// Generate unique agent ID
	id := ulid.Make().String()

	// Generate worktree name: agent-{id}
	worktreeName := fmt.Sprintf("agent-%s", id)

	// Build Packnplay RunConfig
	runConfig := &runner.RunConfig{
		Path:         l.projectPath,
		Worktree:     worktreeName,
		NoWorktree:   false, // Let Packnplay manage worktrees
		Env:          mapToEnvSlice(cfg.Environment),
		Verbose:      l.verbose,
		Runtime:      l.runtime,
		Reconnect:    false, // Always false for Spawn
		DefaultImage: l.defaultImage,
		Command:      cfg.Command,
		Credentials:  mapSpawnConfigCredentials(cfg.Credentials),
		// Note: Args are combined with Command in Packnplay
	}

	// Override image if specified in SpawnConfig
	if cfg.Image != "" {
		runConfig.DefaultImage = cfg.Image
	}

	// Combine command and args
	if len(cfg.Args) > 0 {
		runConfig.Command = append(cfg.Command, cfg.Args...)
	}

	// Create handle structure
	handle := &PacknplayHandle{
		id:           id,
		worktreeName: worktreeName,
		role:         cfg.Role,
		launcher:     l,
		ctx:          ctx,
		cancelFunc:   nil, // Will be set below
		runnerDone:   make(chan error, 1),
		stdinPipe:    newPipeCloser(),
		stdoutPipe:   newPipeCloser(),
		stderrPipe:   newPipeCloser(),
	}

	// Create cancellable context for runner goroutine
	_, cancelFunc := context.WithCancel(context.Background())
	handle.cancelFunc = cancelFunc

	// Launch runner in goroutine
	go func() {
		if l.verbose {
			fmt.Fprintf(os.Stderr, "[Packnplay] Launching runner for agent %s\n", id)
		}
		err := runner.Run(runConfig)
		handle.runnerDone <- err
		close(handle.runnerDone)
	}()

	// Wait for container to be created and discover its container ID
	// We'll poll for the container using Packnplay's label scheme
	containerID, workspacePath, err := l.discoverContainer(ctx, worktreeName)
	if err != nil {
		cancelFunc()
		return nil, fmt.Errorf("failed to discover spawned container: %w", err)
	}

	handle.containerID = containerID
	handle.workspace = workspacePath

	// Attach I/O streams via Docker Engine API
	if err := l.attachStreams(ctx, handle); err != nil {
		cancelFunc()
		_ = l.stopContainer(ctx, containerID)
		return nil, fmt.Errorf("failed to attach I/O streams: %w", err)
	}

	// Store handle
	l.mu.Lock()
	l.handles[id] = handle
	l.mu.Unlock()

	if l.verbose {
		fmt.Fprintf(os.Stderr, "[Packnplay] Agent %s spawned successfully (container: %s)\n", id, containerID[:12])
	}

	return handle, nil
}

// Attach reconnects to an existing agent instance by its ID.
func (l *PacknplayLauncher) Attach(ctx context.Context, id string) (agent.AgentHandle, error) {
	// Check if already attached in memory
	l.mu.RLock()
	if handle, exists := l.handles[id]; exists {
		l.mu.RUnlock()
		return handle, nil
	}
	l.mu.RUnlock()

	// Reconstruct worktree name
	worktreeName := fmt.Sprintf("agent-%s", id)

	// Discover container
	containerID, workspacePath, err := l.discoverContainer(ctx, worktreeName)
	if err != nil {
		return nil, fmt.Errorf("agent not found or container not running: %w", err)
	}

	// Create handle structure
	handle := &PacknplayHandle{
		id:           id,
		containerID:  containerID,
		worktreeName: worktreeName,
		workspace:    workspacePath,
		launcher:     l,
		ctx:          ctx,
		runnerDone:   make(chan error, 1), // Won't be used for attach
		stdinPipe:    newPipeCloser(),
		stdoutPipe:   newPipeCloser(),
		stderrPipe:   newPipeCloser(),
	}

	// Attach I/O streams
	if err := l.attachStreams(ctx, handle); err != nil {
		return nil, fmt.Errorf("failed to attach I/O streams: %w", err)
	}

	// Store handle
	l.mu.Lock()
	l.handles[id] = handle
	l.mu.Unlock()

	if l.verbose {
		fmt.Fprintf(os.Stderr, "[Packnplay] Attached to agent %s (container: %s)\n", id, containerID[:12])
	}

	return handle, nil
}

// Stop terminates an agent instance gracefully.
func (l *PacknplayLauncher) Stop(ctx context.Context, handle agent.AgentHandle) error {
	h, ok := handle.(*PacknplayHandle)
	if !ok {
		return fmt.Errorf("invalid handle type: expected *PacknplayHandle")
	}

	// Cancel runner goroutine if running
	if h.cancelFunc != nil {
		h.cancelFunc()
	}

	// Stop container
	if err := l.stopContainer(ctx, h.containerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Remove from tracking
	l.mu.Lock()
	delete(l.handles, h.id)
	l.mu.Unlock()

	// Close handle
	if err := h.Close(); err != nil {
		return fmt.Errorf("failed to close handle: %w", err)
	}

	if l.verbose {
		fmt.Fprintf(os.Stderr, "[Packnplay] Agent %s stopped\n", h.id)
	}

	return nil
}

// discoverContainer finds a running container by worktree name using Packnplay labels.
func (l *PacknplayLauncher) discoverContainer(ctx context.Context, worktreeName string) (containerID string, workspacePath string, err error) {
	// Build filter for Packnplay-managed containers with matching worktree
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "managed-by=packnplay")
	filterArgs.Add("label", fmt.Sprintf("packnplay-worktree=%s", worktreeName))
	filterArgs.Add("status", "running")

	// List containers with retry (container might not be ready immediately after spawn)
	var containers []container.Summary
	maxRetries := 30 // 30 * 100ms = 3 seconds max wait
	for i := 0; i < maxRetries; i++ {
		containers, err = l.dockerClient.ContainerList(ctx, container.ListOptions{
			Filters: filterArgs,
		})
		if err != nil {
			return "", "", fmt.Errorf("failed to list containers: %w", err)
		}

		if len(containers) > 0 {
			break
		}

		// Wait and retry
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	if len(containers) == 0 {
		return "", "", fmt.Errorf("no running container found for worktree %s", worktreeName)
	}

	if len(containers) > 1 {
		fmt.Fprintf(os.Stderr, "[Packnplay] Warning: found %d containers for worktree %s, using most recent\n", len(containers), worktreeName)
	}

	// Use most recent container
	c := containers[0]
	containerID = c.ID

	// Determine workspace path from worktree
	workspacePath = git.DetermineWorktreePath(l.projectPath, worktreeName)

	return containerID, workspacePath, nil
}

// attachStreams attaches stdin/stdout/stderr to a container via Docker Engine API.
func (l *PacknplayLauncher) attachStreams(ctx context.Context, handle *PacknplayHandle) error {
	// Attach to container with stream demux (TTY=false for separate stdout/stderr)
	attachResp, err := l.dockerClient.ContainerAttach(ctx, handle.containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to container: %w", err)
	}

	// Wire stdin: copy from handle's stdin pipe to container
	go func() {
		defer attachResp.Close()
		_, _ = io.Copy(attachResp.Conn, handle.stdinPipe.Reader())
	}()

	// Wire stdout/stderr: demultiplex Docker's multiplexed stream
	go func() {
		_, _ = stdcopy.StdCopy(handle.stdoutPipe.Writer(), handle.stderrPipe.Writer(), attachResp.Reader)
		// Close pipes when container stops producing output
		_ = handle.stdoutPipe.Writer().Close()
		_ = handle.stderrPipe.Writer().Close()
	}()

	return nil
}

// stopContainer stops a container gracefully, with forceful termination as fallback.
func (l *PacknplayLauncher) stopContainer(ctx context.Context, containerID string) error {
	// Try graceful stop with timeout
	timeout := int(10) // 10 seconds
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}

	if err := l.dockerClient.ContainerStop(ctx, containerID, stopOptions); err != nil {
		// If graceful stop fails, force remove
		removeOptions := container.RemoveOptions{
			Force: true,
		}
		if removeErr := l.dockerClient.ContainerRemove(ctx, containerID, removeOptions); removeErr != nil {
			return fmt.Errorf("failed to stop container (stop error: %v, remove error: %w)", err, removeErr)
		}
	}

	return nil
}

// FindRunningAgents returns a list of agent IDs for all running Packnplay-managed containers.
func (l *PacknplayLauncher) FindRunningAgents(ctx context.Context) ([]string, error) {
	// Build filter for Packnplay-managed containers
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "managed-by=packnplay")
	filterArgs.Add("status", "running")

	containers, err := l.dockerClient.ContainerList(ctx, container.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var agentIDs []string
	for _, c := range containers {
		// Extract agent ID from worktree label
		if worktree, ok := c.Labels["packnplay-worktree"]; ok {
			// Worktree format: agent-{ULID}
			if len(worktree) > 6 && worktree[:6] == "agent-" {
				agentID := worktree[6:]
				agentIDs = append(agentIDs, agentID)
			}
		}
	}

	return agentIDs, nil
}

// Close closes the launcher and releases resources.
func (l *PacknplayLauncher) Close() error {
	if l.dockerClient != nil {
		return l.dockerClient.Close()
	}
	return nil
}

// mapToEnvSlice converts a map to KEY=VALUE slice.
func mapToEnvSlice(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	env := make([]string, 0, len(m))
	for k, v := range m {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

// mapSpawnConfigCredentials maps SpawnConfig.Credentials to Packnplay config.Credentials.
func mapSpawnConfigCredentials(creds map[string]string) config.Credentials {
	if len(creds) == 0 {
		// Default: enable common credentials
		return config.Credentials{
			Git: true,
			SSH: true,
			GH:  true,
			GPG: false,
			NPM: false,
		}
	}

	// Map from SpawnConfig credential keys
	return config.Credentials{
		Git: creds["git"] == "true",
		SSH: creds["ssh"] == "true",
		GH:  creds["gh"] == "true" || creds["github"] == "true",
		GPG: creds["gpg"] == "true",
		NPM: creds["npm"] == "true",
	}
}
