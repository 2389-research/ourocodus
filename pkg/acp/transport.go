package acp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Transport represents a bidirectional communication channel to an ACP runtime.
// Implementations abstract whether ACP runs as a host process, inside a container,
// or over any other medium that exposes stdin/stdout style semantics.
type Transport interface {
	io.Reader
	io.Writer

	// Close terminates the transport and releases resources.
	// The provided context controls shutdown timeout and cancellation.
	Close(ctx context.Context) error

	// Stderr returns a reader for diagnostic output. May be nil if stderr is unavailable.
	Stderr() io.Reader
}

// ProcessLaunchConfig describes how to start an ACP runtime.
// Concrete launchers (host exec, docker exec, etc.) translate this config
// into their runtime-specific operations.
type ProcessLaunchConfig struct {
	Workspace   string
	APIKey      string
	CommandPath string
	CommandArgs []string
	Env         map[string]string
}

// ProcessLauncher starts ACP runtimes and returns transports that the client can use.
type ProcessLauncher interface {
	Start(ctx context.Context, cfg ProcessLaunchConfig) (Transport, error)
}

// HostProcessLauncher starts ACP as a host process via os/exec.
type HostProcessLauncher struct{}

// Start implements ProcessLauncher using os/exec on the local host.
func (l *HostProcessLauncher) Start(ctx context.Context, cfg ProcessLaunchConfig) (Transport, error) {
	if cfg.CommandPath == "" {
		return nil, fmt.Errorf("command path is required")
	}
	if cfg.Workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// #nosec G204 -- command path intentionally configurable for testing
	cmd := exec.CommandContext(ctx, cfg.CommandPath, cfg.CommandArgs...)
	cmd.Dir = cfg.Workspace

	env := append([]string{}, os.Environ()...)
	env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", cfg.APIKey))
	for k, v := range cfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("failed to start %q: %w", cfg.CommandPath, err)
	}

	return &hostProcessTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

// hostProcessTransport adapts an *exec.Cmd pipes to the Transport interface.
type hostProcessTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	closeOnce sync.Once
}

func (t *hostProcessTransport) Read(p []byte) (int, error) {
	return t.stdout.Read(p)
}

func (t *hostProcessTransport) Write(p []byte) (int, error) {
	return t.stdin.Write(p)
}

func (t *hostProcessTransport) Stderr() io.Reader {
	return t.stderr
}

func (t *hostProcessTransport) Close(ctx context.Context) error {
	var waitErr error
	t.closeOnce.Do(func() {
		_ = t.stdin.Close()

		done := make(chan error, 1)
		go func() {
			done <- t.cmd.Wait()
		}()

		select {
		case err := <-done:
			waitErr = err
		case <-time.After(5 * time.Second):
			_ = t.cmd.Process.Kill()

			select {
			case err := <-done:
				waitErr = err
			case <-time.After(2 * time.Second):
				waitErr = fmt.Errorf("process %d did not exit after kill", t.cmd.Process.Pid)
			case <-ctx.Done():
				waitErr = fmt.Errorf("close cancelled by context: %w", ctx.Err())
			}
		case <-ctx.Done():
			waitErr = fmt.Errorf("close cancelled by context before graceful shutdown: %w", ctx.Err())
		}

		_ = t.stdout.Close()
		_ = t.stderr.Close()
	})

	if waitErr != nil {
		if _, ok := waitErr.(*exec.ExitError); ok {
			return nil
		}
		return waitErr
	}

	return nil
}
