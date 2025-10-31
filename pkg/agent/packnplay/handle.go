package packnplay

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/docker/docker/api/types/container"
)

// PacknplayHandle implements the AgentHandle interface for Packnplay-managed containers.
type PacknplayHandle struct {
	id           string
	containerID  string
	worktreeName string
	workspace    string
	role         string
	launcher     *PacknplayLauncher

	ctx        context.Context
	cancelFunc context.CancelFunc
	runnerDone chan error

	stdinPipe  *pipeCloser
	stdoutPipe *pipeCloser
	stderrPipe *pipeCloser

	mu     sync.RWMutex
	closed bool
}

// ID returns the unique identifier for this agent instance.
func (h *PacknplayHandle) ID() string {
	return h.id
}

// Workspace returns the filesystem path to the agent's workspace directory.
func (h *PacknplayHandle) Workspace() string {
	return h.workspace
}

// ContainerID returns the container ID.
func (h *PacknplayHandle) ContainerID() string {
	return h.containerID
}

// Stdin returns a writer for sending input to the agent's standard input.
func (h *PacknplayHandle) Stdin() io.WriteCloser {
	return h.stdinPipe.Writer()
}

// Stdout returns a reader for receiving output from the agent's standard output.
func (h *PacknplayHandle) Stdout() io.ReadCloser {
	return h.stdoutPipe.Reader()
}

// Stderr returns a reader for receiving output from the agent's standard error.
func (h *PacknplayHandle) Stderr() io.ReadCloser {
	return h.stderrPipe.Reader()
}

// Wait blocks until the agent process exits and returns its exit status.
func (h *PacknplayHandle) Wait(ctx context.Context) error {
	// Use Docker's ContainerWait for proper exit status
	statusCh, errCh := h.launcher.dockerClient.ContainerWait(ctx, h.containerID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("wait error: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with status code %d", status.StatusCode)
		}
	case err := <-h.runnerDone:
		// Runner goroutine completed (for Spawn, not Attach)
		if err != nil {
			return fmt.Errorf("runner error: %w", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// Close releases any resources associated with this handle.
// This does not stop the agent; use AgentLauncher.Stop for that.
func (h *PacknplayHandle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("handle already closed")
	}

	h.closed = true

	// Cancel runner context if active
	if h.cancelFunc != nil {
		h.cancelFunc()
	}

	// Close all pipes
	var errs []error
	if err := h.stdinPipe.Close(); err != nil {
		errs = append(errs, fmt.Errorf("stdin: %w", err))
	}
	if err := h.stdoutPipe.Close(); err != nil {
		errs = append(errs, fmt.Errorf("stdout: %w", err))
	}
	if err := h.stderrPipe.Close(); err != nil {
		errs = append(errs, fmt.Errorf("stderr: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing pipes: %v", errs)
	}

	return nil
}

// pipeCloser wraps io.Pipe to provide separate Reader() and Writer() access
// with proper close semantics.
type pipeCloser struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	mu     sync.Mutex
	closed bool
}

// newPipeCloser creates a new pipeCloser.
func newPipeCloser() *pipeCloser {
	r, w := io.Pipe()
	return &pipeCloser{
		reader: r,
		writer: w,
	}
}

// Reader returns the read end of the pipe.
func (p *pipeCloser) Reader() io.ReadCloser {
	return p.reader
}

// Writer returns the write end of the pipe.
func (p *pipeCloser) Writer() io.WriteCloser {
	return p.writer
}

// Close closes both ends of the pipe.
func (p *pipeCloser) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	var errs []error
	if err := p.reader.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := p.writer.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	return nil
}
