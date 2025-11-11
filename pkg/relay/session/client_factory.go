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

// selectLauncher chooses between host and container execution based on runtime context and environment.
func (f *ACPClientFactory) selectLauncher(runtime *AgentRuntimeContext) (acp.ProcessLauncher, error) {
	acpRuntime := os.Getenv("OUROCODUS_ACP_RUNTIME")

	// Default to host if not specified or explicitly set to "host"
	if acpRuntime == "" || acpRuntime == "host" {
		if f.logger != nil {
			f.logger.Printf("[ACP] Using host process launcher for session=%s agent=%s",
				runtime.SessionID, runtime.AgentID)
		}
		return &acp.HostProcessLauncher{}, nil
	}

	// Container mode requested
	if acpRuntime == "container" {
		// Validate prerequisites
		if !runtime.HasContainer() {
			return nil, fmt.Errorf("container runtime requested but no container ID in runtime context (session=%s agent=%s)",
				runtime.SessionID, runtime.AgentID)
		}
		if f.containerSessionMgr == nil {
			return nil, fmt.Errorf("container runtime requested but container session manager not available (session=%s agent=%s)",
				runtime.SessionID, runtime.AgentID)
		}

		if f.logger != nil {
			f.logger.Printf("[ACP] Using container exec launcher for session=%s agent=%s container=%s",
				runtime.SessionID, runtime.AgentID, runtime.ContainerID)
		}

		// Create container exec launcher
		launcher := NewContainerExecProcessLauncher(
			f.containerSessionMgr,
			runtime.ContainerID,
		)

		// Configure workspace path mapping
		// Container workspace path should match the mount point (standard: /workspace)
		launcher = launcher.WithWorkspacePath("/workspace")

		return launcher, nil
	}

	return nil, fmt.Errorf("invalid OUROCODUS_ACP_RUNTIME value: %q (must be 'host' or 'container', session=%s agent=%s)",
		acpRuntime, runtime.SessionID, runtime.AgentID)
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
