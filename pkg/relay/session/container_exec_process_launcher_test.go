package session

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/containersession"
)

func TestContainerExecProcessLauncher_Validates(t *testing.T) {
	launcher := NewContainerExecProcessLauncher(nil, "", nil)
	if _, err := launcher.Start(context.Background(), acp.ProcessLaunchConfig{}); err == nil {
		t.Fatal("expected error when service missing")
	}
}

func TestContainerExecProcessLauncher_Start(t *testing.T) {
	stdinBuf := &bytes.Buffer{}
	stdoutBuf := bytes.NewBufferString("ok")
	stderrBuf := bytes.NewBufferString("err")

	attachment := containersession.NewExecAttachment(
		nopWriteCloser{Writer: stdinBuf},
		io.NopCloser(stdoutBuf),
		io.NopCloser(stderrBuf),
		func() error { return nil },
	)

	service := &stubExecService{stream: attachment}
	launcher := NewContainerExecProcessLauncher(service, "container-123", nil)

	transport, err := launcher.Start(context.Background(), acp.ProcessLaunchConfig{CommandPath: "claude-code-acp", CommandArgs: []string{"--workspace", "/workspace"}, APIKey: "key"})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	buf := make([]byte, 2)
	if _, err := transport.Read(buf); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(buf) != "ok" {
		t.Fatalf("unexpected stdout: %s", buf)
	}

	if _, err := transport.Write([]byte("hi")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if stdinBuf.String() != "hi" {
		t.Fatalf("stdin not forwarded, got %q", stdinBuf.String())
	}

	if err := transport.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if !service.called {
		t.Fatal("expected exec service to be invoked")
	}
}

type stubExecService struct {
	stream *containersession.ExecAttachment
	called bool
	err    error
}

func (s *stubExecService) ExecInContainer(ctx context.Context, containerID string, cfg containersession.ExecConfig) (*containersession.ExecAttachment, error) {
	s.called = true
	return s.stream, s.err
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error { return nil }

// TestRewriteWorkspaceArg tests the workspace path rewriting function
func TestRewriteWorkspaceArg(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		containerPath string
		want          []string
	}{
		{
			name:          "rewrites --workspace with space",
			args:          []string{"--workspace", "/Users/dev/session-123", "--verbose"},
			containerPath: "/workspace",
			want:          []string{"--workspace", "/workspace", "--verbose"},
		},
		{
			name:          "rewrites --workspace= format",
			args:          []string{"--workspace=/Users/dev/session-123", "--verbose"},
			containerPath: "/workspace",
			want:          []string{"--workspace=/workspace", "--verbose"},
		},
		{
			name:          "passes through args without workspace",
			args:          []string{"--verbose", "--debug", "--log-level=info"},
			containerPath: "/workspace",
			want:          []string{"--verbose", "--debug", "--log-level=info"},
		},
		{
			name:          "handles empty args",
			args:          []string{},
			containerPath: "/workspace",
			want:          []string{},
		},
		{
			name:          "handles workspace at start",
			args:          []string{"--workspace", "/host/path"},
			containerPath: "/workspace",
			want:          []string{"--workspace", "/workspace"},
		},
		{
			name:          "handles workspace at end",
			args:          []string{"--verbose", "--workspace", "/host/path"},
			containerPath: "/workspace",
			want:          []string{"--verbose", "--workspace", "/workspace"},
		},
		{
			name:          "handles workspace= at end",
			args:          []string{"--verbose", "--workspace=/host/path"},
			containerPath: "/workspace",
			want:          []string{"--verbose", "--workspace=/workspace"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteWorkspaceArg(tt.args, tt.containerPath)

			if len(got) != len(tt.want) {
				t.Errorf("rewriteWorkspaceArg() length = %d, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("rewriteWorkspaceArg()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
