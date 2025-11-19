package main

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// findAgentContainerID finds the container ID for a given agent ID by querying Docker.
// Returns empty string if no container is found.
func findAgentContainerID(ctx context.Context, agentID string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer func() { _ = cli.Close() }()

	// Find container with matching agent-id label
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "ourocodus.agent=true")
	filterArgs.Add("label", fmt.Sprintf("agent-id=%s", agentID))

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
