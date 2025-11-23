package session

import (
	"context"
	"fmt"

	"github.com/2389-research/ourocodus/pkg/labels"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// findAgentContainerID discovers a CLI-spawned agent container by agent ID.
// This is the production version used by AttachAgent to locate containers for adoption.
//
// It uses the Phase 3 labels package to query Docker for containers with proper
// ourocodus.agent/* labels, validating the agent ID and extracting workspace information.
//
// Returns:
//   - containerID: The Docker container ID
//   - workspace: The workspace path from container labels or mounts
//   - error: If container not found, multiple found, or Docker API errors
func findAgentContainerID(ctx context.Context, agentID string) (string, string, error) {
	// Validate agent ID to prevent path traversal attacks
	if err := ValidateAgentID(agentID); err != nil {
		return "", "", fmt.Errorf("invalid agent ID: %w", err)
	}

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	// Use Phase 3 labels package for consistent label querying
	filters := labels.FindAgentFilter(agentID)

	// List containers with the agent label (running only)
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters,
		All:     false, // Only running containers
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return "", "", fmt.Errorf("no running container found for agent ID: %s", agentID)
	}

	if len(containers) > 1 {
		return "", "", fmt.Errorf("multiple containers found for agent ID %s (found %d)", agentID, len(containers))
	}

	// Extract container ID and workspace
	ctr := containers[0]
	containerID := ctr.ID

	// Try to get workspace from labels first (Phase 3 pattern)
	workspace := ctr.Labels[labels.Workspace]

	// Fallback: Check mounts if label not present
	if workspace == "" {
		for _, mnt := range ctr.Mounts {
			if mnt.Destination == "/workspace" {
				workspace = mnt.Source
				break
			}
		}
	}

	if workspace == "" {
		return "", "", fmt.Errorf("container %s missing workspace (no label or mount)", containerID[:12])
	}

	return containerID, workspace, nil
}
