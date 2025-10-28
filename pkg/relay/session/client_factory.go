package session

import (
	"os"

	"github.com/2389-research/ourocodus/pkg/acp"
)

// ACPClientFactory implements ClientFactory using pkg/acp.Client
// Reads ANTHROPIC_API_KEY from environment and spawns claude-code-acp processes
// Optionally reads OUROCODUS_ACP_BINARY to override the default ACP binary path
type ACPClientFactory struct {
	apiKey        string
	acpBinaryPath string // Optional custom ACP binary path (for testing)
}

// NewACPClientFactory creates a new ACP client factory
// Reads ANTHROPIC_API_KEY from environment (required)
// Optionally reads OUROCODUS_ACP_BINARY to use a custom ACP binary (e.g., echo-agent for testing)
func NewACPClientFactory() (*ACPClientFactory, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, ErrMissingAnthropicAPIKey
	}

	// Check for custom ACP binary path (optional, for testing)
	acpBinaryPath := os.Getenv("OUROCODUS_ACP_BINARY")

	return &ACPClientFactory{
		apiKey:        apiKey,
		acpBinaryPath: acpBinaryPath,
	}, nil
}

// GetACPBinaryPath returns the custom ACP binary path (empty string if not set)
func (f *ACPClientFactory) GetACPBinaryPath() string {
	return f.acpBinaryPath
}

// NewClient spawns a new ACP process in the given workspace
// Uses custom binary path if OUROCODUS_ACP_BINARY was set, otherwise defaults to claude-code-acp
func (f *ACPClientFactory) NewClient(workspace string) (ACPClient, error) {
	var client *acp.Client
	var err error

	if f.acpBinaryPath != "" {
		// Use custom binary (e.g., echo-agent for testing)
		client, err = acp.NewClient(workspace, f.apiKey, acp.WithCommand(f.acpBinaryPath))
	} else {
		// Use default claude-code-acp binary
		client, err = acp.NewClient(workspace, f.apiKey)
	}

	if err != nil {
		return nil, err
	}
	return &acpClientAdapter{client: client}, nil
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
	clientFunc func(workspace string) (ACPClient, error)
}

// NewFakeClientFactory creates a fake client factory for testing
func NewFakeClientFactory(clientFunc func(workspace string) (ACPClient, error)) *FakeClientFactory {
	return &FakeClientFactory{
		clientFunc: clientFunc,
	}
}

// NewClient returns a mock client from the provided function
func (f *FakeClientFactory) NewClient(workspace string) (ACPClient, error) {
	return f.clientFunc(workspace)
}
