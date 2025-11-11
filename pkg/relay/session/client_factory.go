package session

import (
	"context"
	"fmt"
	"os"

	"github.com/2389-research/ourocodus/pkg/acp"
)

// ACPClientFactory implements ClientFactory using pkg/acp.Client with runtime-based launcher selection.
// Reads ANTHROPIC_API_KEY from environment and spawns claude-code-acp processes.
// Optionally reads OUROCODUS_ACP_BINARY to override the default ACP binary path.
// Optionally reads OUROCODUS_ACP_RUNTIME to select execution mode (host or container).
type ACPClientFactory struct {
	apiKey              string
	acpBinaryPath       string
	containerSessionMgr ContainerExecService // Optional: enables container exec mode
	logger              Logger               // Optional: for runtime logging
}

// NewACPClientFactory creates a new ACP client factory.
// Reads ANTHROPIC_API_KEY from environment (required).
// Optionally reads OUROCODUS_ACP_BINARY to use a custom ACP binary (e.g., echo-agent for testing).
// Optionally reads OUROCODUS_ACP_RUNTIME to select execution mode (host or container).
//
// Parameters:
//   - containerSessionMgr: Optional container session manager for container exec mode. If nil, only host mode is available.
//   - logger: Optional logger for runtime diagnostics. If nil, no logging is performed.
func NewACPClientFactory(containerSessionMgr ContainerExecService, logger Logger) (*ACPClientFactory, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, ErrMissingAnthropicAPIKey
	}

	// Check for custom ACP binary path (optional, for testing)
	acpBinaryPath := os.Getenv("OUROCODUS_ACP_BINARY")

	return &ACPClientFactory{
		apiKey:              apiKey,
		acpBinaryPath:       acpBinaryPath,
		containerSessionMgr: containerSessionMgr,
		logger:              logger,
	}, nil
}

// GetACPBinaryPath returns the custom ACP binary path (empty string if not set)
func (f *ACPClientFactory) GetACPBinaryPath() string {
	return f.acpBinaryPath
}

// NewClient spawns a new ACP process using the appropriate launcher based on runtime context.
// Launcher selection:
//   - If OUROCODUS_ACP_RUNTIME=container and runtime has container ID: uses ContainerExecProcessLauncher
//   - Otherwise: uses HostProcessLauncher (default)
//
// Uses custom binary path if OUROCODUS_ACP_BINARY was set, otherwise defaults to claude-code-acp.
func (f *ACPClientFactory) NewClient(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error) {
	if runtime == nil {
		return nil, fmt.Errorf("runtime context is required")
	}
	workspace := runtime.Workspace
	if workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}

	// Select launcher based on runtime context and feature flag
	launcher, err := f.selectLauncher(runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to select launcher: %w", err)
	}

	// Build client options
	opts := []acp.ClientOption{
		acp.WithProcessLauncher(launcher),
		acp.WithLaunchContext(ctx), // Enable cancellation for launcher operations
	}

	if f.acpBinaryPath != "" {
		opts = append(opts, acp.WithCommand(f.acpBinaryPath))
	}

	client, err := acp.NewClient(workspace, f.apiKey, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACP client: %w", err)
	}

	return &acpClientAdapter{client: client}, nil
}

// getRuntimeMode reads and validates the OUROCODUS_ACP_RUNTIME environment variable.
// Returns "host" (default), "container", or an error for invalid values.
func getRuntimeMode() (string, error) {
	mode := os.Getenv("OUROCODUS_ACP_RUNTIME")
	if mode == "" {
		return "host", nil // default
	}
	if mode == "host" || mode == "container" {
		return mode, nil
	}
	return "", fmt.Errorf("invalid OUROCODUS_ACP_RUNTIME value: %q (must be 'host' or 'container')", mode)
}

// validateContainerPrerequisites checks if all prerequisites for container execution are met.
// Returns an error if runtime is nil, container ID is missing, or container session manager is unavailable.
func validateContainerPrerequisites(runtime *AgentRuntimeContext, containerSessionMgr ContainerExecService) error {
	if runtime == nil {
		return fmt.Errorf("container runtime requested but runtime context is nil")
	}
	if !runtime.HasContainer() {
		return fmt.Errorf("container runtime requested but no container ID in runtime context (session=%s agent=%s)",
			runtime.SessionID, runtime.AgentID)
	}
	if containerSessionMgr == nil {
		return fmt.Errorf("container runtime requested but container session manager not available (session=%s agent=%s)",
			runtime.SessionID, runtime.AgentID)
	}
	return nil
}

// createHostLauncher creates a host process launcher and logs the decision if logger is available.
func (f *ACPClientFactory) createHostLauncher(runtime *AgentRuntimeContext) acp.ProcessLauncher {
	if f.logger != nil {
		f.logger.Printf("[ACP] Using host process launcher for session=%s agent=%s",
			runtime.SessionID, runtime.AgentID)
	}
	return &acp.HostProcessLauncher{}
}

// createContainerLauncher creates a container attach launcher configured for the runtime context.
// Logs the decision if logger is available.
func (f *ACPClientFactory) createContainerLauncher(runtime *AgentRuntimeContext) acp.ProcessLauncher {
	if f.logger != nil {
		f.logger.Printf("[ACP] Using container attach launcher for session=%s agent=%s container=%s",
			runtime.SessionID, runtime.AgentID, runtime.ContainerID)
	}

	// Get Docker client from container session manager
	dockerClient := f.containerSessionMgr.GetDockerClient()

	launcher := NewContainerAttachProcessLauncher(
		dockerClient,
		runtime.ContainerID,
		f.logger,
	)

	return launcher
}

// selectLauncher chooses between host and container execution based on runtime context and environment.
// This is the main orchestrator that delegates to specialized functions for clarity and testability.
func (f *ACPClientFactory) selectLauncher(runtime *AgentRuntimeContext) (acp.ProcessLauncher, error) {
	mode, err := getRuntimeMode()
	if err != nil {
		return nil, fmt.Errorf("%w (session=%s agent=%s)", err, runtime.SessionID, runtime.AgentID)
	}

	switch mode {
	case "host":
		return f.createHostLauncher(runtime), nil
	case "container":
		if err := validateContainerPrerequisites(runtime, f.containerSessionMgr); err != nil {
			return nil, err
		}
		return f.createContainerLauncher(runtime), nil
	default:
		// Should never reach here due to getRuntimeMode validation
		return nil, fmt.Errorf("unexpected runtime mode: %q (session=%s agent=%s)", mode, runtime.SessionID, runtime.AgentID)
	}
}

// acpClientAdapter adapts pkg/acp.Client to ACPClient interface
type acpClientAdapter struct {
	client *acp.Client
}

// SendMessage sends a message to the ACP client
func (a *acpClientAdapter) SendMessage(content string) (interface{}, error) {
	return a.client.SendMessage(content)
}

// Close closes the ACP client
func (a *acpClientAdapter) Close() error {
	return a.client.Close()
}

// FakeClientFactory implements ClientFactory for testing
// Returns mock clients without spawning real processes
type FakeClientFactory struct {
	clientFunc func(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error)
}

// NewFakeClientFactory creates a fake client factory for testing using only workspace input.
func NewFakeClientFactory(clientFunc func(workspace string) (ACPClient, error)) *FakeClientFactory {
	return &FakeClientFactory{
		clientFunc: func(_ context.Context, runtime *AgentRuntimeContext) (ACPClient, error) {
			workspace := ""
			if runtime != nil {
				workspace = runtime.Workspace
			}
			return clientFunc(workspace)
		},
	}
}

// NewRuntimeFakeClientFactory creates a fake factory using the full runtime context.
func NewRuntimeFakeClientFactory(clientFunc func(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error)) *FakeClientFactory {
	return &FakeClientFactory{clientFunc: clientFunc}
}

// NewClient returns a mock client from the provided function
func (f *FakeClientFactory) NewClient(ctx context.Context, runtime *AgentRuntimeContext) (ACPClient, error) {
	if f.clientFunc == nil {
		return nil, fmt.Errorf("fake client factory not configured")
	}
	return f.clientFunc(ctx, runtime)
}
