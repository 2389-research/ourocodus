# Docker Labels Package

This package provides centralized Docker label management for Ourocodus agents to ensure consistency and prevent label format errors across the codebase.

## Problem This Solves

Previously, Docker labels were defined inconsistently across the codebase:
- Some code used `"agent-id"` (wrong - partial name without namespace)
- Some code used `"ourocodus.agent/agent-id"` (correct - fully qualified)
- Constants were defined but not used consistently
- Label queries failed because format mismatches

This led to bugs where agents couldn't be discovered even though they existed.

## Label Format

All Ourocodus agent labels MUST use the fully qualified format:

```
ourocodus.agent/<key>
```

**Examples**:
- ✅ `ourocodus.agent/agent-id` (correct)
- ✅ `ourocodus.agent/workspace` (correct)
- ❌ `agent-id` (WRONG - missing namespace)
- ❌ `workspace` (WRONG - missing namespace)

## Usage

### Setting Labels on Containers

**DON'T** use hardcoded strings:
```go
// ❌ WRONG - prone to typos and inconsistency
labels := map[string]string{
    "agent-id": agentID,  // Missing namespace!
    "workspace": workspace,
}
```

**DO** use the label builder:
```go
// ✅ CORRECT - type-safe and consistent
import "github.com/2389-research/ourocodus/pkg/labels"

containerLabels := labels.NewBuilder().
    WithNamespace().
    WithAgentID(agentID).
    WithWorkspace(workspace).
    WithSpawnSource("cli").
    Build()
```

**Or** use the convenience function:
```go
// ✅ CORRECT - for standard label set
containerLabels := labels.Standard(agentID, workspace, "cli")
```

### Querying Docker for Containers

**DON'T** build filters manually:
```go
// ❌ WRONG - error-prone
filterArgs := filters.NewArgs()
filterArgs.Add("label", "agent-id="+agentID)  // Missing namespace!
```

**DO** use the filter builder:
```go
// ✅ CORRECT - type-safe
import "github.com/2389-research/ourocodus/pkg/labels"

filters := labels.FindAgentFilter(agentID)

containers, err := cli.ContainerList(ctx, container.ListOptions{
    Filters: filters,
})
```

### Reading Labels from Containers

**DON'T** use hardcoded strings:
```go
// ❌ WRONG
workspace := ctr.Labels["workspace"]  // Will be empty!
```

**DO** use the constants:
```go
// ✅ CORRECT
import "github.com/2389-research/ourocodus/pkg/labels"

workspace := ctr.Labels[labels.Workspace]
agentID := ctr.Labels[labels.AgentID]
```

## API Reference

### Constants

- `labels.Namespace` - The namespace prefix (`"ourocodus.agent"`)
- `labels.AgentID` - Fully qualified agent ID label (`"ourocodus.agent/agent-id"`)
- `labels.Workspace` - Fully qualified workspace label (`"ourocodus.agent/workspace"`)
- `labels.SpawnSource` - Fully qualified spawn source label (`"ourocodus.agent/spawn-source"`)

### Label Builder

```go
builder := labels.NewBuilder()
builder.WithNamespace()                    // Add namespace label
builder.WithAgentID("alice")               // Add agent ID label
builder.WithWorkspace("/workspace/alice")  // Add workspace label
builder.WithSpawnSource("cli")             // Add spawn source label
builder.WithCustom("key", "value")         // Add custom label
labelMap := builder.Build()                // Get map[string]string
```

### Filter Builder

```go
filterBuilder := labels.NewFilterBuilder()
filterBuilder.WithNamespace()         // Filter by namespace
filterBuilder.WithAgentID("alice")    // Filter by agent ID
filterBuilder.WithSpawnSource("cli")  // Filter by spawn source
filters := filterBuilder.Build()      // Get filters.Args
```

### Convenience Functions

```go
// Standard labels for an agent container
labels := labels.Standard(agentID, workspace, spawnSource)

// Filter to find a specific agent
filters := labels.FindAgentFilter(agentID)

// Filter to list all Ourocodus agents
filters := labels.ListAgentsFilter()
```

## Testing

The package includes comprehensive tests that verify:
- ✅ All constants use the correct fully qualified format
- ✅ No constants use partial names without namespace
- ✅ Builder correctly constructs labels
- ✅ Filter builder correctly constructs Docker filters
- ✅ Convenience functions work as expected

Run tests:
```bash
go test ./pkg/labels/...
```

## Migration Guide

If you're updating existing code to use this package:

### Step 1: Replace hardcoded label strings

**Before**:
```go
labels := map[string]string{
    "agent-id": agentID,
    "workspace": workspace,
}
```

**After**:
```go
import "github.com/2389-research/ourocodus/pkg/labels"

containerLabels := labels.Standard(agentID, workspace, "cli")
```

### Step 2: Replace manual filter construction

**Before**:
```go
filterArgs := filters.NewArgs()
filterArgs.Add("label", fmt.Sprintf("agent-id=%s", agentID))
```

**After**:
```go
import "github.com/2389-research/ourocodus/pkg/labels"

filterArgs := labels.FindAgentFilter(agentID)
```

### Step 3: Replace label reads

**Before**:
```go
agentID := ctr.Labels["agent-id"]
workspace := ctr.Labels["workspace"]
```

**After**:
```go
import "github.com/2389-research/ourocodus/pkg/labels"

agentID := ctr.Labels[labels.AgentID]
workspace := ctr.Labels[labels.Workspace]
```

## Why This Matters

Inconsistent label usage causes **agent discovery failures**:

1. Agent container is created with label `"agent-id"` (wrong)
2. Discovery code queries for label `"ourocodus.agent/agent-id"` (correct)
3. Query returns no results even though agent exists
4. System thinks agent doesn't exist and fails

Using this package ensures all code uses the same label format, preventing these silent failures.

## Rules

1. **NEVER** use hardcoded label strings outside this package
2. **ALWAYS** use the constants and builders from this package
3. **NEVER** use partial names like `"agent-id"` - always use fully qualified names
4. **DO** add tests when adding new label types
5. **DO** use the filter builders for Docker queries

## Examples

See `labels_test.go` for comprehensive examples of all functionality.
