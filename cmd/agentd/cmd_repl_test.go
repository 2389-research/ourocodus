package main

import (
	"strings"
	"testing"
)

func TestRunREPL_FindsAgent(t *testing.T) {
	// This test verifies the agent lookup logic
	// We can't easily test full docker attach without integration test

	// Test that runREPL validates agent ID
	err := runREPL(replCmd, []string{})
	if err == nil {
		t.Error("Expected error with no agent ID")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("Expected error to mention agent, got: %v", err)
	}
}

func TestFindAgentByID(t *testing.T) {
	agents := []agentInfo{
		{AgentID: "alice", ContainerID: "abc123", Status: "running"},
		{AgentID: "bob", ContainerID: "def456", Status: "running"},
	}

	agent, found := findAgentByID(agents, "alice")
	if !found {
		t.Error("Expected to find alice")
	}
	if agent.ContainerID != "abc123" {
		t.Errorf("Expected container abc123, got %s", agent.ContainerID)
	}

	_, found = findAgentByID(agents, "charlie")
	if found {
		t.Error("Should not find charlie")
	}
}
