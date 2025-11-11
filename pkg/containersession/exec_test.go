package containersession

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

func TestExecInContainer_ValidatesInput(t *testing.T) {
	mgr := NewManager(&mockDockerClient{}, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, "./workspaces")

	if _, err := mgr.ExecInContainer(context.Background(), "", ExecConfig{Command: []string{"/bin/echo"}}); err == nil {
		t.Fatal("expected error for missing containerID")
	}

	if _, err := mgr.ExecInContainer(context.Background(), "container", ExecConfig{}); err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestExecInContainer_EnvSorted(t *testing.T) {
	captured := map[string][]string{}
	docker := &mockDockerClient{
		execCreateFn: func(ctx context.Context, containerID string, cfg container.ExecOptions) (types.IDResponse, error) {
			captured[containerID] = cfg.Env
			return types.IDResponse{ID: "exec-1"}, nil
		},
		execAttachFn: func(ctx context.Context, execID string, cfg container.ExecAttachOptions) (types.HijackedResponse, error) {
			pr, pw := net.Pipe()
			go func() {
				time.Sleep(10 * time.Millisecond)
				_ = pw.Close()
			}()
			return types.HijackedResponse{
				Conn:   pr,
				Reader: bufio.NewReader(bytes.NewReader(nil)),
			}, nil
		},
	}

	mgr := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, "./workspaces")

	env := map[string]string{"B": "2", "A": "1"}
	if _, err := mgr.ExecInContainer(context.Background(), "cont-123", ExecConfig{Command: []string{"/bin/echo"}, Env: env}); err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	got := captured["cont-123"]
	expected := []string{"A=1", "B=2"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d env vars, got %d", len(expected), len(got))
	}
	for i, v := range expected {
		if got[i] != v {
			t.Fatalf("env not sorted: got %v", got)
		}
	}
}

func TestExecInContainer_Streams(t *testing.T) {
	var stdin bytes.Buffer
	stdoutPayload := &bytes.Buffer{}
	stdoutWriter := stdcopy.NewStdWriter(stdoutPayload, stdcopy.Stdout)
	stderrWriter := stdcopy.NewStdWriter(stdoutPayload, stdcopy.Stderr)
	_, _ = stdoutWriter.Write([]byte("stdout-line\n"))
	_, _ = stderrWriter.Write([]byte("stderr-line\n"))

	reader := bufio.NewReader(bytes.NewReader(stdoutPayload.Bytes()))

	pr, pw := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer pw.Close()
		io.Copy(&stdin, pw)
		close(done)
	}()

	docker := &mockDockerClient{
		execCreateFn: func(ctx context.Context, containerID string, cfg container.ExecOptions) (types.IDResponse, error) {
			return types.IDResponse{ID: "exec-1"}, nil
		},
		execAttachFn: func(ctx context.Context, execID string, cfg container.ExecAttachOptions) (types.HijackedResponse, error) {
			return types.HijackedResponse{
				Conn:   pr,
				Reader: reader,
			}, nil
		},
	}

	mgr := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, "./workspaces")

	attachment, err := mgr.ExecInContainer(context.Background(), "container", ExecConfig{Command: []string{"/bin/sh", "-c", "echo"}})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	buf := make([]byte, len("stdout-line\n"))
	if _, err := io.ReadFull(attachment.Stdout(), buf); err != nil {
		t.Fatalf("failed reading stdout: %v", err)
	}

	if string(buf) != "stdout-line\n" {
		t.Fatalf("unexpected stdout: %s", buf)
	}

	errBuf := make([]byte, len("stderr-line\n"))
	if _, err := io.ReadFull(attachment.Stderr(), errBuf); err != nil {
		t.Fatalf("failed reading stderr: %v", err)
	}

	if string(errBuf) != "stderr-line\n" {
		t.Fatalf("unexpected stderr: %s", errBuf)
	}

	if _, err := attachment.Stdin().Write([]byte("input")); err != nil {
		t.Fatalf("failed to write stdin: %v", err)
	}

	if err := attachment.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	<-done

	if stdin.String() != "input" {
		t.Fatalf("stdin not forwarded, got %q", stdin.String())
	}
}
