package main

const (
	// LabelNamespace identifies agentd-managed containers
	// This matches the label set by pkg/agent/container/launcher.go
	LabelNamespace = "ourocodus.agent"

	// LabelAgentID stores the agent identifier
	// This matches the label set by pkg/agent/container/launcher.go
	LabelAgentID = "agent-id"

	// LabelSpawnSource indicates how the agent was spawned (cli, relay, etc.)
	// This matches the label set by pkg/agent/container/launcher.go
	LabelSpawnSource = "ourocodus.agent/spawn-source"

	// Version is the current agentd version
	Version = "0.1.0"
)
