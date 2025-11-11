package containersession

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// ExecConfig describes the command to run inside an existing container.
type ExecConfig struct {
	Command    []string
	Env        map[string]string
	WorkingDir string
	User       string
}

// ExecAttachment exposes the stdio streams for a docker exec invocation.
type ExecAttachment struct {
	ExecID string

	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	closeOnce sync.Once
	closeFn   func() error
}

// Stdin returns the exec stdin writer.
func (a *ExecAttachment) Stdin() io.WriteCloser { return a.stdin }

// Stdout returns the reader for stdout.
func (a *ExecAttachment) Stdout() io.ReadCloser { return a.stdout }

// Stderr returns the reader for stderr.
func (a *ExecAttachment) Stderr() io.ReadCloser { return a.stderr }

// Close releases all exec resources.
func (a *ExecAttachment) Close() error {
	var err error
	a.closeOnce.Do(func() {
		if a.closeFn != nil {
			err = a.closeFn()
		}
	})
	return err
}

// NewExecAttachment constructs an attachment with custom streams and close behavior.
func NewExecAttachment(stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser, closeFn func() error) *ExecAttachment {
	return &ExecAttachment{
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		closeFn: closeFn,
	}
}

// ExecInContainer runs a command inside an existing container and returns its stdio streams.
func (m *Manager) ExecInContainer(ctx context.Context, containerID string, cfg ExecConfig) (*ExecAttachment, error) {
	if containerID == "" {
		return nil, fmt.Errorf("containerID is required")
	}
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("exec command is required")
	}

	env := envMapToSlice(cfg.Env)

	execCfg := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  true,
		Cmd:          cfg.Command,
		Env:          env,
		WorkingDir:   cfg.WorkingDir,
		User:         cfg.User,
		Tty:          false,
	}

	createResp, err := m.dockerClient.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := m.dockerClient.ContainerExecAttach(ctx, createResp.ID, container.ExecAttachOptions{Detach: false, Tty: false})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	// Create cancellation context for goroutine lifecycle management
	_, copyCancel := context.WithCancel(context.Background())

	// Goroutine for copying stdout/stderr from docker exec
	go func() {
		defer copyCancel() // Ensure context is cancelled when goroutine exits
		_, copyErr := stdcopy.StdCopy(stdoutWriter, stderrWriter, attachResp.Reader)
		_ = stdoutWriter.CloseWithError(copyErr)
		_ = stderrWriter.CloseWithError(copyErr)
	}()

	closeFn := func() error {
		// Cancel context to signal goroutine cleanup
		copyCancel()

		// Close resources in correct order
		// Note: attachResp.Reader is a *bufio.Reader without Close method
		// The underlying connection is closed by attachResp.Close()
		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
		attachResp.Close()
		return nil
	}

	attachment := NewExecAttachment(attachResp.Conn, stdoutReader, stderrReader, closeFn)
	attachment.ExecID = createResp.ID
	return attachment, nil
}

func envMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, k := range keys {
		result = append(result, fmt.Sprintf("%s=%s", k, env[k]))
	}
	return result
}
