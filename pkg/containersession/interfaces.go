package containersession

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// DockerClient abstracts Docker SDK operations for testability
type DockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config,
		hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig,
		platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string,
		options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string,
		options container.StopOptions) error
	ContainerAttach(ctx context.Context, containerID string,
		options container.AttachOptions) (types.HijackedResponse, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerRemove(ctx context.Context, containerID string,
		options container.RemoveOptions) error
}

// IDGenerator abstracts unique ID generation
type IDGenerator interface {
	Generate() string
}

// Clock abstracts time operations for deterministic testing
type Clock interface {
	Now() time.Time
}

// Logger abstracts logging operations
type Logger interface {
	Printf(format string, v ...interface{})
}
