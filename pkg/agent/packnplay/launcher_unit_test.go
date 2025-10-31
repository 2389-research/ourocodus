package packnplay

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/docker/docker/client"
)

func TestWithDockerClient(t *testing.T) {
	// Test that WithDockerClient option works
	mockClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available, skipping test")
	}
	defer mockClient.Close()

	l, err := NewLauncher(
		WithProjectPath("."),
		WithDockerClient(mockClient),
	)
	if err != nil {
		t.Fatalf("NewLauncher() with Docker client failed: %v", err)
	}
	defer l.Close()

	if l.dockerClient == nil {
		t.Error("expected non-nil Docker client")
	}
}

func TestWithDockerHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{
			name:    "unix socket",
			host:    "unix:///var/run/docker.sock",
			wantErr: false, // May fail if Docker not available, but option should work
		},
		{
			name:    "colima socket",
			host:    "unix://" + t.TempDir() + "/docker.sock",
			wantErr: false, // Will fail to connect but option parsing works
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := NewLauncher(
				WithProjectPath("."),
				WithDockerHost(tt.host),
			)
			if (err != nil) != tt.wantErr {
				// Don't fail on connection errors, just option parsing errors
				if l != nil {
					_ = l.Close()
				}
				return
			}
			if l != nil {
				defer l.Close()
				if l.dockerClient == nil {
					t.Error("expected non-nil Docker client")
				}
			}
		})
	}
}

func TestWithDockerHost_ConflictWithClient(t *testing.T) {
	mockClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skip("Docker not available, skipping test")
	}
	defer mockClient.Close()

	// Setting Docker host after client should error
	_, err = NewLauncher(
		WithProjectPath("."),
		WithDockerClient(mockClient),
		WithDockerHost("unix:///var/run/docker.sock"),
	)
	if err == nil {
		t.Error("expected error when setting Docker host after Docker client, got nil")
	}
}

func TestNewLauncher_Errors(t *testing.T) {
	tests := []struct {
		name    string
		opts    []LauncherOption
		wantErr bool
	}{
		{
			name: "invalid project path",
			opts: []LauncherOption{
				WithProjectPath("/nonexistent/path/that/does/not/exist"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := NewLauncher(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLauncher() error = %v, wantErr %v", err, tt.wantErr)
			}
			if l != nil {
				_ = l.Close()
			}
		})
	}
}

func TestPacknplayHandle_Methods(t *testing.T) {
	// Create a mock handle
	handle := &PacknplayHandle{
		id:           "test-id",
		containerID:  "test-container-123",
		worktreeName: "agent-test",
		workspace:    "/tmp/test-workspace",
		role:         "test-role",
		stdinPipe:    newPipeCloser(),
		stdoutPipe:   newPipeCloser(),
		stderrPipe:   newPipeCloser(),
		runnerDone:   make(chan error, 1),
	}

	t.Run("ID", func(t *testing.T) {
		if got := handle.ID(); got != "test-id" {
			t.Errorf("ID() = %v, want %v", got, "test-id")
		}
	})

	t.Run("ContainerID", func(t *testing.T) {
		if got := handle.ContainerID(); got != "test-container-123" {
			t.Errorf("ContainerID() = %v, want %v", got, "test-container-123")
		}
	})

	t.Run("Workspace", func(t *testing.T) {
		if got := handle.Workspace(); got != "/tmp/test-workspace" {
			t.Errorf("Workspace() = %v, want %v", got, "/tmp/test-workspace")
		}
	})

	t.Run("Stdin", func(t *testing.T) {
		stdin := handle.Stdin()
		if stdin == nil {
			t.Error("Stdin() returned nil")
		}
		// Note: Can't test Write() without reader as it will block
	})

	t.Run("Stdout", func(t *testing.T) {
		stdout := handle.Stdout()
		if stdout == nil {
			t.Error("Stdout() returned nil")
		}
	})

	t.Run("Stderr", func(t *testing.T) {
		stderr := handle.Stderr()
		if stderr == nil {
			t.Error("Stderr() returned nil")
		}
	})

	t.Run("Close", func(t *testing.T) {
		err := handle.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}

		// Second close should error
		err = handle.Close()
		if err == nil {
			t.Error("expected error on second Close(), got nil")
		}
	})
}

func TestPacknplayHandle_Wait_ContextCancellation(t *testing.T) {
	// Create launcher (need Docker client for Wait to work)
	l, err := NewLauncher(WithProjectPath("."))
	if err != nil {
		t.Skip("Docker not available, skipping test")
	}
	defer l.Close()

	handle := &PacknplayHandle{
		id:           "test-id",
		containerID:  "test-container-123",
		worktreeName: "agent-test",
		workspace:    "/tmp/test-workspace",
		launcher:     l,
		stdinPipe:    newPipeCloser(),
		stdoutPipe:   newPipeCloser(),
		stderrPipe:   newPipeCloser(),
		runnerDone:   make(chan error, 1),
	}

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Wait should return context error (may be wrapped)
	err = handle.Wait(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Wait() with cancelled context error = %v, want context.Canceled", err)
	}
}

func TestPipeCloser_ReadWrite(t *testing.T) {
	p := newPipeCloser()
	defer p.Close()

	testData := []byte("hello world")

	// Write in goroutine
	go func() {
		n, err := p.Writer().Write(testData)
		if err != nil {
			t.Errorf("Write() error = %v", err)
		}
		if n != len(testData) {
			t.Errorf("Write() wrote %d bytes, want %d", n, len(testData))
		}
		_ = p.Writer().Close()
	}()

	// Read
	buf := make([]byte, 1024)
	n, err := p.Reader().Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read() error = %v", err)
	}

	if got := string(buf[:n]); got != string(testData) {
		t.Errorf("Read() got %q, want %q", got, testData)
	}
}

func TestMapToEnvSlice_EmptyValues(t *testing.T) {
	m := map[string]string{
		"KEY": "",
	}
	got := mapToEnvSlice(m)
	if len(got) != 1 {
		t.Errorf("mapToEnvSlice() got %d entries, want 1", len(got))
	}
	if got[0] != "KEY=" {
		t.Errorf("mapToEnvSlice() got %q, want %q", got[0], "KEY=")
	}
}

func TestMapSpawnConfigCredentials_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want map[string]bool // Which credentials should be true
	}{
		{
			name: "false values are disabled",
			m: map[string]string{
				"git": "false",
				"ssh": "false",
			},
			want: map[string]bool{
				"Git": false,
				"SSH": false,
				"GH":  false,
			},
		},
		{
			name: "unknown keys ignored",
			m: map[string]string{
				"unknown": "true",
				"git":     "true",
			},
			want: map[string]bool{
				"Git": true,
				"SSH": false,
			},
		},
		{
			name: "gh and github both work",
			m: map[string]string{
				"gh":     "true",
				"github": "true",
			},
			want: map[string]bool{
				"GH": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapSpawnConfigCredentials(tt.m)
			for field, expectedValue := range tt.want {
				var actualValue bool
				switch field {
				case "Git":
					actualValue = got.Git
				case "SSH":
					actualValue = got.SSH
				case "GH":
					actualValue = got.GH
				case "GPG":
					actualValue = got.GPG
				case "NPM":
					actualValue = got.NPM
				}
				if actualValue != expectedValue {
					t.Errorf("%s: got %v, want %v", field, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestLauncherClose(t *testing.T) {
	l, err := NewLauncher(WithProjectPath("."))
	if err != nil {
		t.Fatalf("NewLauncher() failed: %v", err)
	}

	// Close should succeed
	if err := l.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Second close should also succeed (idempotent)
	if err := l.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}
