package session

import (
	"context"
	"fmt"
	"io"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// ContainerAttachProcessLauncher runs ACP by attaching to a container's main process stdio.
// This is the simpler, more Docker-native approach where ACP runs as the container's ENTRYPOINT.
type ContainerAttachProcessLauncher struct {
	dockerClient containersession.DockerClient
	containerID  string
	logger       Logger
}

// NewContainerAttachProcessLauncher constructs a container attach based ProcessLauncher.
func NewContainerAttachProcessLauncher(dockerClient containersession.DockerClient, containerID string, logger Logger) *ContainerAttachProcessLauncher {
	return &ContainerAttachProcessLauncher{
		dockerClient: dockerClient,
		containerID:  containerID,
		logger:       logger,
	}
}

func (l *ContainerAttachProcessLauncher) Start(ctx context.Context, cfg acp.ProcessLaunchConfig) (acp.Transport, error) {
	if l.logger != nil {
		l.logger.Printf("[ACP→ATTACH] Attaching to container %s stdio", l.containerID[:12])
	}

	if l.dockerClient == nil {
		return nil, fmt.Errorf("docker client is required")
	}
	if l.containerID == "" {
		return nil, fmt.Errorf("container ID is required for container attach launcher")
	}

	// Attach to the container's stdin/stdout/stderr
	// The container should already be running with ACP as its main process
	if l.logger != nil {
		l.logger.Printf("[ACP→ATTACH] ├─ Attaching to container stdio streams...")
	}

	attachResp, err := l.dockerClient.ContainerAttach(ctx, l.containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		if l.logger != nil {
			l.logger.Printf("[ACP→ATTACH] ✗ Container attach failed: %v", err)
		}
		return nil, fmt.Errorf("failed to attach to container %s: %w", l.containerID, err)
	}

	if l.logger != nil {
		l.logger.Printf("[ACP→ATTACH] ✓ Attached to container %s stdio successfully", l.containerID[:12])
	}

	return &containerAttachTransport{
		hijackedResp: attachResp,
	}, nil
}

type containerAttachTransport struct {
	hijackedResp types.HijackedResponse
}

func (t *containerAttachTransport) Read(p []byte) (int, error) {
	return t.hijackedResp.Reader.Read(p)
}

func (t *containerAttachTransport) Write(p []byte) (int, error) {
	return t.hijackedResp.Conn.Write(p)
}

func (t *containerAttachTransport) Close() error {
	t.hijackedResp.Close()
	return nil
}

func (t *containerAttachTransport) Stderr() io.Reader {
	// With Docker attach, stderr is multiplexed in the same stream
	// The Reader will contain both stdout and stderr in Docker's stream format
	// For now, return the same reader - Docker's stdcopy package can demux if needed
	return t.hijackedResp.Reader
}
