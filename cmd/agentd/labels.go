package main

const (
	// LabelNamespace identifies agentd-managed containers
	// This matches the label set by pkg/agent/container/launcher.go
	LabelNamespace = "ourocodus.agent"

	// LabelAgentID stores the agent identifier (fully qualified label name)
	// CRITICAL: This must match pkg/agent/container/launcher.go and pkg/relay/session/helpers.go
	LabelAgentID = "ourocodus.agent/agent-id"

	// LabelWorkspace stores the workspace path (fully qualified label name)
	// CRITICAL: This must match pkg/agent/container/launcher.go and pkg/relay/session/helpers.go
	LabelWorkspace = "ourocodus.agent/workspace"

	// LabelSpawnSource indicates how the agent was spawned (cli, relay, etc.)
	// This matches the label set by pkg/agent/container/launcher.go
	LabelSpawnSource = "ourocodus.agent/spawn-source"

	// Version is the current agentd version
	Version = "0.1.0"
)
