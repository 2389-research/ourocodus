package agent_test

import (
	"context"
	"testing"

	"github.com/2389-research/ourocodus/pkg/agent"
)

func TestDefaultFactory_CreateLauncher(t *testing.T) {
	config := agent.LauncherFactoryConfig{
		// nil dependencies OK for this basic test
	}
	factory := agent.NewDefaultLauncherFactory(config)

	launcherConfig := agent.LauncherConfig{
		AgentID: "test-agent",
	}

	launcher, err := factory.CreateLauncher(context.Background(), "test-agent", launcherConfig)
	if err != nil {
		t.Fatalf("CreateLauncher failed: %v", err)
	}

	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
}
