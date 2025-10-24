package session

import (
	"fmt"
	"os"

	"github.com/2389-research/ourocodus/pkg/acp"
)

// ACPClientFactory implements ClientFactory using pkg/acp.Client
// Reads ANTHROPIC_API_KEY from environment and spawns claude-code-acp processes
type ACPClientFactory struct {
	apiKey string
}

// NewACPClientFactory creates a new ACP client factory
// Reads ANTHROPIC_API_KEY from environment
func NewACPClientFactory() (*ACPClientFactory, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	return &ACPClientFactory{
		apiKey: apiKey,
	}, nil
}

// NewClient spawns a new claude-code-acp process in the given workspace
func (f *ACPClientFactory) NewClient(workspace string) (ACPClient, error) {
	client, err := acp.NewClient(workspace, f.apiKey)
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
