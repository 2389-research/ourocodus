package session

import (
	"context"
	"fmt"
	"io"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
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

	// Create pipes for demultiplexed stdout/stderr
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	// Start demultiplexing goroutine
	// Docker uses a special stream format when Tty=false that needs to be demultiplexed
	go func() {
		_, err := stdcopy.StdCopy(stdoutWriter, stderrWriter, attachResp.Reader)
		stdoutWriter.CloseWithError(err)
		stderrWriter.CloseWithError(err)
	}()

	return &containerAttachTransport{
		hijackedResp: attachResp,
		stdout:       stdoutReader,
		stderr:       stderrReader,
	}, nil
}

type containerAttachTransport struct {
	hijackedResp types.HijackedResponse
	stdout       io.ReadCloser
	stderr       io.ReadCloser
}

func (t *containerAttachTransport) Read(p []byte) (int, error) {
	// Read from demultiplexed stdout
	return t.stdout.Read(p)
}

func (t *containerAttachTransport) Write(p []byte) (int, error) {
	return t.hijackedResp.Conn.Write(p)
}

func (t *containerAttachTransport) Close() error {
	// Close stdout/stderr pipes
	if t.stdout != nil {
		t.stdout.Close()
	}
	if t.stderr != nil {
		t.stderr.Close()
	}
	// Close hijacked connection
	t.hijackedResp.Close()
	return nil
}

func (t *containerAttachTransport) Stderr() io.Reader {
	// Return demultiplexed stderr stream
	return t.stderr
}
