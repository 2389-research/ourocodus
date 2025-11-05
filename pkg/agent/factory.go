package agent

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/docker/docker/client"
)

// ErrFactoryNotReady is returned when the factory is missing required dependencies
var ErrFactoryNotReady = errors.New("launcher factory not ready: missing required dependencies")

// LauncherFactory creates AgentLauncher instances based on agent type and configuration.
type LauncherFactory interface {
	// CreateLauncher creates a new launcher for the specified agent.
	// Returns an error if launcher creation fails.
	CreateLauncher(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error)
}

// LauncherConfig contains configuration for creating a launcher.
type LauncherConfig struct {
	AgentID        string
	ImageName      string
	Command        []string
	Workspace      string
	GitSSHKey      []byte
	GitHubToken    []byte
	AnthropicKey   string
	ResourceLimits ResourceLimits
}

// ResourceLimits defines container resource constraints.
type ResourceLimits struct {
	CPUCores int64 // CPU cores (e.g., 2)
	MemoryMB int64 // Memory in MB (e.g., 4096)
}

// LauncherFactoryConfig contains dependencies for creating launchers.
type LauncherFactoryConfig struct {
	DockerClient          *client.Client
	WorktreeManager       *worktree.AgentWorktreeManager
	CredMounter           *container.AgentCredentialMounter
	ContainerManager      *containersession.Manager
	BaseWorkspaceDir      string
	DefaultImageName      string
	DefaultResourceLimits ResourceLimits
}

// DefaultLauncherFactory creates AgentContainerLauncher instances.
type DefaultLauncherFactory struct {
	config LauncherFactoryConfig
}

// NewDefaultLauncherFactory creates a new factory with the provided configuration.
func NewDefaultLauncherFactory(config LauncherFactoryConfig) *DefaultLauncherFactory {
	return &DefaultLauncherFactory{
		config: config,
	}
}

// CreateLauncher creates an AgentContainerLauncher for the specified agent.
func (f *DefaultLauncherFactory) CreateLauncher(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error) {
	// Wrap the factory config to satisfy AgentLauncher interface
	// The actual container launcher is created lazily to avoid panics on nil dependencies during testing
	launcher := &containerLauncherAdapter{
		factory:  f,
		lazyInit: sync.Once{},
		agentID:  agentID,
	}

	return launcher, nil
}

// containerLauncherAdapter wraps AgentContainerLauncher to implement AgentLauncher interface
type containerLauncherAdapter struct {
	factory           *DefaultLauncherFactory
	containerLauncher *container.AgentContainerLauncher
	lazyInit          sync.Once
	agentID           string
	lazyInitErr       error
}

// getContainerLauncher lazily initializes the container launcher
func (a *containerLauncherAdapter) getContainerLauncher() (*container.AgentContainerLauncher, error) {
	a.lazyInit.Do(func() {
		if a.factory.config.ContainerManager == nil ||
			a.factory.config.WorktreeManager == nil ||
			a.factory.config.CredMounter == nil {
			a.lazyInitErr = ErrFactoryNotReady
			return
		}

		a.containerLauncher = container.NewAgentContainerLauncher(
			a.factory.config.ContainerManager,
			a.factory.config.WorktreeManager,
			a.factory.config.CredMounter,
			a.factory.config.BaseWorkspaceDir,
		)
	})

	return a.containerLauncher, a.lazyInitErr
}

// Spawn implements AgentLauncher.Spawn
func (a *containerLauncherAdapter) Spawn(ctx context.Context, config *SpawnConfig) (AgentHandle, error) {
	launcher, err := a.getContainerLauncher()
	if err != nil {
		return nil, err
	}

	spawnConfig := container.SpawnConfig{
		AgentID:     config.Role, // Use Role as AgentID
		ImageName:   config.Image,
		Command:     config.Command,
		GitSSHKey:   nil, // Credentials will be handled separately
		GitHubToken: nil,
		Env:         convertMapToSlice(config.Environment),
	}

	handle, err := launcher.Spawn(ctx, spawnConfig)
	if err != nil {
		return nil, err
	}

	return &containerHandleAdapter{handle: handle}, nil
}

// Attach implements AgentLauncher.Attach
func (a *containerLauncherAdapter) Attach(ctx context.Context, id string) (AgentHandle, error) {
	launcher, err := a.getContainerLauncher()
	if err != nil {
		return nil, err
	}

	handle, err := launcher.Attach(ctx, id)
	if err != nil {
		return nil, err
	}

	return &containerHandleAdapter{handle: handle}, nil
}

// Stop implements AgentLauncher.Stop
func (a *containerLauncherAdapter) Stop(ctx context.Context, handle AgentHandle) error {
	launcher, err := a.getContainerLauncher()
	if err != nil {
		return err
	}

	if adapter, ok := handle.(*containerHandleAdapter); ok {
		// AgentContainerLauncher.Stop takes agentID, not handle
		agentID := adapter.handle.AgentID()
		return launcher.Stop(ctx, agentID)
	}
	return nil
}

// containerHandleAdapter wraps AgentContainerHandle to implement AgentHandle interface
type containerHandleAdapter struct {
	handle *container.AgentContainerHandle
}

// ID implements AgentHandle.ID
func (a *containerHandleAdapter) ID() string {
	return a.handle.AgentID()
}

// Workspace implements AgentHandle.Workspace
func (a *containerHandleAdapter) Workspace() string {
	return a.handle.WorkspacePath()
}

// ContainerID implements AgentHandle.ContainerID
func (a *containerHandleAdapter) ContainerID() string {
	return a.handle.ContainerID()
}

// Stdin implements AgentHandle.Stdin
func (a *containerHandleAdapter) Stdin() io.WriteCloser {
	// ContainerSession doesn't provide direct I/O access in this implementation
	// TODO: Implement I/O forwarding from container exec streams
	return &nilWriteCloser{}
}

// Stdout implements AgentHandle.Stdout
func (a *containerHandleAdapter) Stdout() io.ReadCloser {
	// ContainerSession doesn't provide direct I/O access in this implementation
	// TODO: Implement I/O forwarding from container exec streams
	return &nilReadCloser{}
}

// Stderr implements AgentHandle.Stderr
func (a *containerHandleAdapter) Stderr() io.ReadCloser {
	// ContainerSession doesn't provide direct I/O access in this implementation
	// TODO: Implement I/O forwarding from container exec streams
	return &nilReadCloser{}
}

// Wait implements AgentHandle.Wait
func (a *containerHandleAdapter) Wait(ctx context.Context) error {
	// ContainerSession doesn't provide wait in this implementation
	// TODO: Implement by polling container state
	return nil
}

// Close implements AgentHandle.Close
func (a *containerHandleAdapter) Close() error {
	// ContainerHandleAdapter doesn't require explicit cleanup
	return nil
}

// nilWriteCloser is a no-op WriteCloser for I/O stubs
type nilWriteCloser struct{}

func (nwc *nilWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (nwc *nilWriteCloser) Close() error {
	return nil
}

// nilReadCloser is a no-op ReadCloser for I/O stubs
type nilReadCloser struct{}

func (nrc *nilReadCloser) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (nrc *nilReadCloser) Close() error {
	return nil
}

// convertMapToSlice converts environment map to []string format (KEY=value)
func convertMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}

	var result []string
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}
