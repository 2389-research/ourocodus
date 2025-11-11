package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

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
	// Validate first before logging
	if l.dockerClient == nil {
		return nil, fmt.Errorf("docker client is required")
	}
	if l.containerID == "" {
		return nil, fmt.Errorf("container ID is required for container attach launcher")
	}

	// Safe truncation for logging
	shortID := l.containerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	if l.logger != nil {
		l.logger.Printf("[ACP→ATTACH] Attaching to container %s stdio", shortID)
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
		l.logger.Printf("[ACP→ATTACH] ✓ Attached to container %s stdio successfully", shortID)
	}

	// Create pipes for demultiplexed stdout/stderr
	stdoutReader, stdoutWriter := io.Pipe()

	// Create stderr logging writer if logger is available
	var stderrWriter io.Writer
	if l.logger != nil {
		// Log stderr line by line with [ACP→STDERR] prefix
		stderrReader, stderrWriterPipe := io.Pipe()
		stderrWriter = stderrWriterPipe

		go func() {
			scanner := bufio.NewScanner(stderrReader)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					l.logger.Printf("[%s:stderr] %s", shortID, line)
				}
			}
		}()
	} else {
		// Discard stderr if no logger
		stderrWriter = io.Discard
	}

	// Start demultiplexing goroutine
	// Docker uses a special stream format when Tty=false that needs to be demultiplexed
	go func() {
		_, err := stdcopy.StdCopy(stdoutWriter, stderrWriter, attachResp.Reader)
		stdoutWriter.CloseWithError(err)
		if closer, ok := stderrWriter.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	return &containerAttachTransport{
		hijackedResp: attachResp,
		stdout:       stdoutReader,
	}, nil
}

type containerAttachTransport struct {
	hijackedResp types.HijackedResponse
	stdout       io.ReadCloser
}

func (t *containerAttachTransport) Read(p []byte) (int, error) {
	// Read from demultiplexed stdout
	return t.stdout.Read(p)
}

func (t *containerAttachTransport) Write(p []byte) (int, error) {
	return t.hijackedResp.Conn.Write(p)
}

func (t *containerAttachTransport) Close() error {
	// Close stdout pipe
	if t.stdout != nil {
		_ = t.stdout.Close()
	}
	// Close hijacked connection
	t.hijackedResp.Close()
	return nil
}

func (t *containerAttachTransport) Stderr() io.Reader {
	// Stderr is logged separately, not exposed
	// ACP protocol doesn't use stderr for JSON-RPC
	return nil
}
