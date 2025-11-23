package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConstants verifies that all label constants use the correct format
func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "Namespace",
			constant: Namespace,
			expected: "ourocodus.agent",
		},
		{
			name:     "AgentID",
			constant: AgentID,
			expected: "ourocodus.agent/agent-id",
		},
		{
			name:     "Workspace",
			constant: Workspace,
			expected: "ourocodus.agent/workspace",
		},
		{
			name:     "SpawnSource",
			constant: SpawnSource,
			expected: "ourocodus.agent/spawn-source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant, "Label constant must use fully qualified format")
		})
	}
}

// TestConstants_NoPartialNames ensures no constants use partial names without namespace
func TestConstants_NoPartialNames(t *testing.T) {
	// These are the WRONG formats that must never be used
	forbiddenPartials := []string{
		"agent-id",      // Must be "ourocodus.agent/agent-id"
		"workspace",     // Must be "ourocodus.agent/workspace"
		"spawn-source",  // Must be "ourocodus.agent/spawn-source"
	}

	allConstants := []string{
		Namespace,
		AgentID,
		Workspace,
		SpawnSource,
	}

	for _, forbidden := range forbiddenPartials {
		t.Run("NotUsing_"+forbidden, func(t *testing.T) {
			for _, constant := range allConstants {
				// Namespace is allowed to not have a slash
				if constant == Namespace {
					continue
				}
				assert.NotEqual(t, forbidden, constant,
					"Label constant must not use partial name %q without namespace", forbidden)
			}
		})
	}
}

// TestBuilder_AllLabels verifies that the builder correctly constructs all labels
func TestBuilder_AllLabels(t *testing.T) {
	labels := NewBuilder().
		WithNamespace().
		WithAgentID("test-agent").
		WithWorkspace("/workspace/test").
		WithSpawnSource("cli").
		Build()

	require.Len(t, labels, 4, "Expected 4 labels")
	assert.Equal(t, "true", labels[Namespace])
	assert.Equal(t, "test-agent", labels[AgentID])
	assert.Equal(t, "/workspace/test", labels[Workspace])
	assert.Equal(t, "cli", labels[SpawnSource])
}

// TestBuilder_PartialLabels verifies that the builder works with partial sets
func TestBuilder_PartialLabels(t *testing.T) {
	labels := NewBuilder().
		WithAgentID("test-agent").
		WithWorkspace("/workspace/test").
		Build()

	require.Len(t, labels, 2, "Expected 2 labels")
	assert.Equal(t, "test-agent", labels[AgentID])
	assert.Equal(t, "/workspace/test", labels[Workspace])
	assert.NotContains(t, labels, Namespace)
	assert.NotContains(t, labels, SpawnSource)
}

// TestBuilder_CustomLabels verifies that custom labels can be added
func TestBuilder_CustomLabels(t *testing.T) {
	labels := NewBuilder().
		WithAgentID("test-agent").
		WithCustom("custom-key", "custom-value").
		Build()

	require.Len(t, labels, 2, "Expected 2 labels")
	assert.Equal(t, "test-agent", labels[AgentID])
	assert.Equal(t, "custom-value", labels["custom-key"])
}

// TestBuilder_Chaining verifies that builder methods return the builder for chaining
func TestBuilder_Chaining(t *testing.T) {
	b1 := NewBuilder()
	b2 := b1.WithAgentID("test")
	b3 := b2.WithWorkspace("/workspace")
	b4 := b3.WithSpawnSource("cli")

	// All should be the same instance
	assert.Same(t, b1, b2, "WithAgentID should return same builder")
	assert.Same(t, b2, b3, "WithWorkspace should return same builder")
	assert.Same(t, b3, b4, "WithSpawnSource should return same builder")
}

// TestStandard verifies the convenience function for standard labels
func TestStandard(t *testing.T) {
	labels := Standard("test-agent", "/workspace/test", "cli")

	require.Len(t, labels, 4, "Expected 4 labels")
	assert.Equal(t, "true", labels[Namespace])
	assert.Equal(t, "test-agent", labels[AgentID])
	assert.Equal(t, "/workspace/test", labels[Workspace])
	assert.Equal(t, "cli", labels[SpawnSource])
}

// TestFilterBuilder_AgentID verifies agent ID filter construction
func TestFilterBuilder_AgentID(t *testing.T) {
	filters := NewFilterBuilder().
		WithAgentID("test-agent").
		Build()

	// Verify the filter was added correctly
	assert.True(t, filters.Contains("label"),
		"Filter should contain label filter")

	// Get the label filters
	labelFilters := filters.Get("label")
	require.Len(t, labelFilters, 1, "Expected 1 label filter")
	assert.Equal(t, "ourocodus.agent/agent-id=test-agent", labelFilters[0],
		"Filter must use fully qualified label name")
}

// TestFilterBuilder_Namespace verifies namespace filter construction
func TestFilterBuilder_Namespace(t *testing.T) {
	filters := NewFilterBuilder().
		WithNamespace().
		Build()

	labelFilters := filters.Get("label")
	require.Len(t, labelFilters, 1, "Expected 1 label filter")
	assert.Equal(t, "ourocodus.agent=true", labelFilters[0])
}

// TestFilterBuilder_Multiple verifies multiple filters can be combined
func TestFilterBuilder_Multiple(t *testing.T) {
	filters := NewFilterBuilder().
		WithNamespace().
		WithAgentID("test-agent").
		WithSpawnSource("cli").
		Build()

	labelFilters := filters.Get("label")
	require.Len(t, labelFilters, 3, "Expected 3 label filters")
	assert.Contains(t, labelFilters, "ourocodus.agent=true")
	assert.Contains(t, labelFilters, "ourocodus.agent/agent-id=test-agent")
	assert.Contains(t, labelFilters, "ourocodus.agent/spawn-source=cli")
}

// TestFindAgentFilter verifies the convenience function for finding a specific agent
func TestFindAgentFilter(t *testing.T) {
	filters := FindAgentFilter("test-agent")

	labelFilters := filters.Get("label")
	require.Len(t, labelFilters, 1, "Expected 1 label filter")
	assert.Equal(t, "ourocodus.agent/agent-id=test-agent", labelFilters[0],
		"FindAgentFilter must use fully qualified label name")
}

// TestListAgentsFilter verifies the convenience function for listing all agents
func TestListAgentsFilter(t *testing.T) {
	filters := ListAgentsFilter()

	labelFilters := filters.Get("label")
	require.Len(t, labelFilters, 1, "Expected 1 label filter")
	assert.Equal(t, "ourocodus.agent=true", labelFilters[0])
}

// TestFilterBuilder_Chaining verifies that filter builder methods return the builder
func TestFilterBuilder_Chaining(t *testing.T) {
	f1 := NewFilterBuilder()
	f2 := f1.WithNamespace()
	f3 := f2.WithAgentID("test")
	f4 := f3.WithSpawnSource("cli")

	// All should be the same instance
	assert.Same(t, f1, f2, "WithNamespace should return same builder")
	assert.Same(t, f2, f3, "WithAgentID should return same builder")
	assert.Same(t, f3, f4, "WithSpawnSource should return same builder")
}

// TestFilterBuilder_EmptyBuild verifies that building without adding filters works
func TestFilterBuilder_EmptyBuild(t *testing.T) {
	filters := NewFilterBuilder().Build()

	// Should return valid but empty filters
	assert.NotNil(t, filters)
	labelFilters := filters.Get("label")
	assert.Len(t, labelFilters, 0, "Expected 0 label filters")
}

// Benchmark_Builder benchmarks label building performance
func Benchmark_Builder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewBuilder().
			WithNamespace().
			WithAgentID("test-agent").
			WithWorkspace("/workspace/test").
			WithSpawnSource("cli").
			Build()
	}
}

// Benchmark_Standard benchmarks the Standard convenience function
func Benchmark_Standard(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Standard("test-agent", "/workspace/test", "cli")
	}
}

// Benchmark_FilterBuilder benchmarks filter construction
func Benchmark_FilterBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewFilterBuilder().
			WithNamespace().
			WithAgentID("test-agent").
			WithSpawnSource("cli").
			Build()
	}
}

// Benchmark_FindAgentFilter benchmarks the FindAgentFilter convenience function
func Benchmark_FindAgentFilter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FindAgentFilter("test-agent")
	}
}

// ExampleBuilder demonstrates how to use the label builder
func ExampleBuilder() {
	labels := NewBuilder().
		WithNamespace().
		WithAgentID("alice").
		WithWorkspace("/workspace/alice").
		WithSpawnSource("cli").
		WithCustom("custom-label", "custom-value").
		Build()

	// Use labels when creating a Docker container
	_ = labels
}

// ExampleStandard demonstrates the convenience function for standard labels
func ExampleStandard() {
	labels := Standard("alice", "/workspace/alice", "cli")

	// Use labels when creating a Docker container
	_ = labels
}

// ExampleFilterBuilder demonstrates how to use the filter builder
func ExampleFilterBuilder() {
	filters := NewFilterBuilder().
		WithAgentID("alice").
		Build()

	// Use filters when querying Docker for containers
	_ = filters
}

// ExampleFindAgentFilter demonstrates the convenience function for finding an agent
func ExampleFindAgentFilter() {
	filters := FindAgentFilter("alice")

	// Use filters with Docker client's ContainerList
	_ = filters
}
