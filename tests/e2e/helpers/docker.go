package helpers

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// DockerHelper provides utilities for E2E tests with Docker containers.
type DockerHelper struct {
	client *client.Client
}

// NewDockerHelper creates a new Docker helper.
func NewDockerHelper() (*DockerHelper, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerHelper{client: cli}, nil
}

// Close closes the Docker client.
func (h *DockerHelper) Close() error {
	if h.client != nil {
		return h.client.Close()
	}
	return nil
}

// ListAgentContainers lists all containers with ourocodus.agent=true label.
func ListAgentContainers(ctx context.Context) ([]string, error) {
	helper, err := NewDockerHelper()
	if err != nil {
		return nil, err
	}
	defer func() { _ = helper.Close() }()

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "ourocodus.agent=true")

	containers, err := helper.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}

	return ids, nil
}

// WaitForContainer waits for a container to reach running state.
func WaitForContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	helper, err := NewDockerHelper()
	if err != nil {
		return err
	}
	defer func() { _ = helper.Close() }()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for container %s to start", containerID)
		case <-ticker.C:
			inspect, err := helper.client.ContainerInspect(ctx, containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container: %w", err)
			}

			if inspect.State.Running {
				return nil
			}

			if inspect.State.Status == "exited" || inspect.State.Status == "dead" {
				return fmt.Errorf("container %s exited unexpectedly", containerID)
			}
		}
	}
}

// VerifyContainerCleanup verifies a container has been removed.
func VerifyContainerCleanup(ctx context.Context, containerID string) error {
	helper, err := NewDockerHelper()
	if err != nil {
		return err
	}
	defer func() { _ = helper.Close() }()

	_, err = helper.client.ContainerInspect(ctx, containerID)
	if err == nil {
		return fmt.Errorf("container %s still exists", containerID)
	}

	// Check if error is "not found" (expected)
	if errdefs.IsNotFound(err) {
		return nil // Success - container was cleaned up
	}

	return fmt.Errorf("unexpected error inspecting container: %w", err)
}

// GetContainerLogs fetches logs from a container for debugging.
func GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	helper, err := NewDockerHelper()
	if err != nil {
		return "", err
	}
	defer func() { _ = helper.Close() }()

	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       "100",
	}

	reader, err := helper.client.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer func() { _ = reader.Close() }()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(logs), nil
}

// InspectContainer gets detailed container information.
func InspectContainer(ctx context.Context, containerID string) (*container.InspectResponse, error) {
	helper, err := NewDockerHelper()
	if err != nil {
		return nil, err
	}
	defer func() { _ = helper.Close() }()

	inspect, err := helper.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	return &inspect, nil
}
