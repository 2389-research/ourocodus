package agent

import (
	"context"
)

// MockLauncherFactory is a mock implementation for testing.
type MockLauncherFactory struct {
	CreateLauncherFunc func(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error)
}

// NewMockLauncherFactory creates a new mock factory.
func NewMockLauncherFactory() *MockLauncherFactory {
	return &MockLauncherFactory{
		CreateLauncherFunc: func(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error) {
			return NewMockLauncher(), nil
		},
	}
}

// CreateLauncher calls the mock function.
func (m *MockLauncherFactory) CreateLauncher(ctx context.Context, agentID string, config LauncherConfig) (AgentLauncher, error) {
	if m.CreateLauncherFunc != nil {
		return m.CreateLauncherFunc(ctx, agentID, config)
	}
	return NewMockLauncher(), nil
}
