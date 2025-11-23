package main

import (
	"context"

	"github.com/2389-research/ourocodus/pkg/labels"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// findAgentContainerID finds the container ID for a given agent ID by querying Docker.
// Returns empty string if no container is found.
// Uses centralized label package to ensure consistency across codebase.
func findAgentContainerID(ctx context.Context, agentID string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer func() { _ = cli.Close() }()

	// Use centralized label filter builder to ensure consistency
	filterArgs := labels.FindAgentFilter(agentID)

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return "", err
	}

	if len(containers) == 0 {
		return "", nil
	}

	return containers[0].ID, nil
}
