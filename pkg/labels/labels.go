// Package labels provides centralized Docker label management for Ourocodus agents.
//
// This package defines all Docker labels used across the system to ensure consistency
// and prevent label format errors. All components must use these constants instead of
// hardcoded strings.
//
// Label Naming Convention:
//   - Namespace: ourocodus.agent
//   - Format: ourocodus.agent/<key>
//   - Examples: ourocodus.agent/agent-id, ourocodus.agent/workspace
//
// CRITICAL: Never use partial label names (e.g., "agent-id") without the namespace.
// Always use the fully qualified constants from this package.
package labels

import (
	"fmt"

	"github.com/docker/docker/api/types/filters"
)

const (
	// Namespace is the Docker label namespace for all Ourocodus agents
	Namespace = "ourocodus.agent"

	// AgentID is the fully qualified label for the agent identifier
	// Value: Unique agent ID (e.g., "alice", "agent-abc123")
	AgentID = "ourocodus.agent/agent-id"

	// Workspace is the fully qualified label for the workspace path
	// Value: Absolute path to the agent's workspace directory
	Workspace = "ourocodus.agent/workspace"

	// SpawnSource is the fully qualified label for how the agent was spawned
	// Value: "cli", "relay", "api", etc.
	SpawnSource = "ourocodus.agent/spawn-source"
)

// Builder provides type-safe label construction
type Builder struct {
	labels map[string]string
}

// NewBuilder creates a new label builder
func NewBuilder() *Builder {
	return &Builder{
		labels: make(map[string]string),
	}
}

// WithAgentID sets the agent ID label
func (b *Builder) WithAgentID(agentID string) *Builder {
	b.labels[AgentID] = agentID
	return b
}

// WithWorkspace sets the workspace path label
func (b *Builder) WithWorkspace(workspace string) *Builder {
	b.labels[Workspace] = workspace
	return b
}

// WithSpawnSource sets the spawn source label
func (b *Builder) WithSpawnSource(source string) *Builder {
	b.labels[SpawnSource] = source
	return b
}

// WithNamespace sets the namespace label (always "true")
func (b *Builder) WithNamespace() *Builder {
	b.labels[Namespace] = "true"
	return b
}

// WithCustom adds a custom label (use sparingly)
func (b *Builder) WithCustom(key, value string) *Builder {
	b.labels[key] = value
	return b
}

// Build returns the constructed labels map
func (b *Builder) Build() map[string]string {
	return b.labels
}

// Standard returns a standard set of labels for agent containers
// This is a convenience function for common use cases
func Standard(agentID, workspace, spawnSource string) map[string]string {
	return NewBuilder().
		WithNamespace().
		WithAgentID(agentID).
		WithWorkspace(workspace).
		WithSpawnSource(spawnSource).
		Build()
}

// FilterBuilder provides type-safe Docker filter construction for agent queries
type FilterBuilder struct {
	args filters.Args
}

// NewFilterBuilder creates a new filter builder
func NewFilterBuilder() *FilterBuilder {
	return &FilterBuilder{
		args: filters.NewArgs(),
	}
}

// WithNamespace filters for containers with the Ourocodus agent namespace
func (f *FilterBuilder) WithNamespace() *FilterBuilder {
	f.args.Add("label", fmt.Sprintf("%s=true", Namespace))
	return f
}

// WithAgentID filters for containers with a specific agent ID
func (f *FilterBuilder) WithAgentID(agentID string) *FilterBuilder {
	f.args.Add("label", fmt.Sprintf("%s=%s", AgentID, agentID))
	return f
}

// WithSpawnSource filters for containers with a specific spawn source
func (f *FilterBuilder) WithSpawnSource(source string) *FilterBuilder {
	f.args.Add("label", fmt.Sprintf("%s=%s", SpawnSource, source))
	return f
}

// Build returns the constructed Docker filters
func (f *FilterBuilder) Build() filters.Args {
	return f.args
}

// FindAgentFilter returns Docker filters for finding a specific agent
// This is the standard filter used by most agent discovery code
func FindAgentFilter(agentID string) filters.Args {
	return NewFilterBuilder().
		WithAgentID(agentID).
		Build()
}

// ListAgentsFilter returns Docker filters for listing all Ourocodus agents
func ListAgentsFilter() filters.Args {
	return NewFilterBuilder().
		WithNamespace().
		Build()
}
