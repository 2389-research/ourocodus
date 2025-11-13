package containersession

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Mock implementations for testing

// mockConn implements net.Conn for testing
type mockConn struct {
	closed bool
	mu     sync.Mutex
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, io.EOF }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// newMockHijackedResponse creates a HijackedResponse with proper mocks for testing
func newMockHijackedResponse() types.HijackedResponse {
	conn := &mockConn{}
	reader := bufio.NewReader(strings.NewReader(""))
	return types.HijackedResponse{
		Conn:   conn,
		Reader: reader,
	}
}

type mockDockerClient struct {
	createFn     func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	startFn      func(ctx context.Context, containerID string, options container.StartOptions) error
	stopFn       func(ctx context.Context, containerID string, options container.StopOptions) error
	attachFn     func(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error)
	execCreateFn func(ctx context.Context, containerID string, config container.ExecOptions) (container.ExecCreateResponse, error)
	execAttachFn func(ctx context.Context, execID string, config container.ExecAttachOptions) (types.HijackedResponse, error)
	listFn       func(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	removeFn     func(ctx context.Context, containerID string, options container.RemoveOptions) error
	inspectFn    func(ctx context.Context, containerID string) (container.InspectResponse, error)
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, config, hostConfig, networkingConfig, platform, containerName)
	}
	return container.CreateResponse{ID: "test-container-id"}, nil
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	if m.startFn != nil {
		return m.startFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	if m.stopFn != nil {
		return m.stopFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerAttach(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
	if m.attachFn != nil {
		return m.attachFn(ctx, containerID, options)
	}
	return types.HijackedResponse{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerExecCreate(ctx context.Context, containerID string, config container.ExecOptions) (container.ExecCreateResponse, error) {
	if m.execCreateFn != nil {
		return m.execCreateFn(ctx, containerID, config)
	}
	return container.ExecCreateResponse{ID: "exec-id"}, nil
}

func (m *mockDockerClient) ContainerExecAttach(ctx context.Context, execID string, config container.ExecAttachOptions) (types.HijackedResponse, error) {
	if m.execAttachFn != nil {
		return m.execAttachFn(ctx, execID, config)
	}
	return types.HijackedResponse{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	if m.listFn != nil {
		return m.listFn(ctx, options)
	}
	return []container.Summary{}, nil
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	if m.removeFn != nil {
		return m.removeFn(ctx, containerID, options)
	}
	return nil
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	if m.inspectFn != nil {
		return m.inspectFn(ctx, containerID)
	}
	return container.InspectResponse{}, nil
}

type mockIDGenerator struct {
	nextID string
}

func (m *mockIDGenerator) Generate() string {
	if m.nextID == "" {
		return "test-session-id"
	}
	return m.nextID
}

type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	if m.now.IsZero() {
		return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	}
	return m.now
}

type mockLogger struct {
	logs []string
	mu   sync.Mutex
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	// Store logs for verification (thread-safe)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, fmt.Sprintf(format, v...))
}

// Tests

func TestNewManager(t *testing.T) {
	t.Run("panics on nil dockerClient", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but didn't get one")
			}
		}()
		NewManager(nil, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, "./workspaces")
	})

	t.Run("panics on nil idGen", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but didn't get one")
			}
		}()
		NewManager(&mockDockerClient{}, nil, &mockClock{}, &mockLogger{}, "./workspaces")
	})

	t.Run("panics on nil clock", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but didn't get one")
			}
		}()
		NewManager(&mockDockerClient{}, &mockIDGenerator{}, nil, &mockLogger{}, "./workspaces")
	})

	t.Run("panics on nil logger", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but didn't get one")
			}
		}()
		NewManager(&mockDockerClient{}, &mockIDGenerator{}, &mockClock{}, nil, "./workspaces")
	})

	t.Run("sets default workspace dir", func(t *testing.T) {
		manager := NewManager(&mockDockerClient{}, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, "")
		if manager.baseWorkspaceDir != "./workspaces" {
			t.Errorf("Expected default workspace dir './workspaces', got %s", manager.baseWorkspaceDir)
		}
	})

	t.Run("creates manager successfully", func(t *testing.T) {
		manager := NewManager(&mockDockerClient{}, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, "/tmp/workspaces")
		if manager == nil {
			t.Fatal("Expected non-nil manager")
		}
		if manager.baseWorkspaceDir != "/tmp/workspaces" {
			t.Errorf("Expected workspace dir '/tmp/workspaces', got %s", manager.baseWorkspaceDir)
		}
	})
}

func TestCreateContainerSession(t *testing.T) {
	t.Run("creates session successfully", func(t *testing.T) {
		docker := &mockDockerClient{}
		idGen := &mockIDGenerator{nextID: "test-123"}
		clock := &mockClock{}
		logger := &mockLogger{}

		manager := NewManager(docker, idGen, clock, logger, t.TempDir())

		session, err := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if session == nil {
			t.Fatal("Expected non-nil session")
		}

		if session.ID() != "test-123" {
			t.Errorf("Expected session ID 'test-123', got %s", session.ID())
		}

		if session.State() != StatePending {
			t.Errorf("Expected state PENDING, got %s", session.State())
		}

		if session.ContainerID() != "test-container-id" {
			t.Errorf("Expected container ID 'test-container-id', got %s", session.ContainerID())
		}
	})

	t.Run("fails when container creation fails", func(t *testing.T) {
		docker := &mockDockerClient{
			createFn: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
				return container.CreateResponse{}, errors.New("docker error")
			},
		}
		idGen := &mockIDGenerator{}
		clock := &mockClock{}
		logger := &mockLogger{}

		manager := NewManager(docker, idGen, clock, logger, t.TempDir())

		session, err := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if session != nil {
			t.Error("Expected nil session on error")
		}

		// Session should not be in manager's map
		if manager.GetContainerSession("test-session-id") != nil {
			t.Error("Session should not be stored after creation failure")
		}
	})
}

func TestStartContainerSession(t *testing.T) {
	t.Run("starts session successfully", func(t *testing.T) {
		docker := &mockDockerClient{}
		idGen := &mockIDGenerator{}
		clock := &mockClock{}
		logger := &mockLogger{}

		manager := NewManager(docker, idGen, clock, logger, t.TempDir())

		session, _ := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		err := manager.StartContainerSession(context.Background(), session.ID())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if session.State() != StateRunning {
			t.Errorf("Expected state RUNNING, got %s", session.State())
		}
	})

	t.Run("fails when session not found", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		err := manager.StartContainerSession(context.Background(), "non-existent")
		if err == nil {
			t.Error("Expected error for non-existent session")
		}
	})

	t.Run("fails when session in wrong state", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		session, _ := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		session.SetState(StateRunning) // Already running

		err := manager.StartContainerSession(context.Background(), session.ID())
		if err == nil {
			t.Error("Expected error when starting already-running session")
		}
	})

	t.Run("fails when container start fails", func(t *testing.T) {
		docker := &mockDockerClient{
			startFn: func(ctx context.Context, containerID string, options container.StartOptions) error {
				return errors.New("start failed")
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		session, _ := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		err := manager.StartContainerSession(context.Background(), session.ID())
		if err == nil {
			t.Error("Expected error when container start fails")
		}

		if session.State() != StateFailed {
			t.Errorf("Expected state FAILED, got %s", session.State())
		}
	})
}

func TestStopContainerSession(t *testing.T) {
	t.Run("stops session successfully", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		session, _ := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		_ = manager.StartContainerSession(context.Background(), session.ID())

		err := manager.StopContainerSession(context.Background(), session.ID())
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if session.State() != StateStopped {
			t.Errorf("Expected state STOPPED, got %s", session.State())
		}
	})

	t.Run("idempotent - already stopped", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		session, _ := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		_ = manager.StartContainerSession(context.Background(), session.ID())
		_ = manager.StopContainerSession(context.Background(), session.ID())

		// Stop again - should be idempotent
		err := manager.StopContainerSession(context.Background(), session.ID())
		if err != nil {
			t.Errorf("Expected no error for idempotent stop, got %v", err)
		}
	})

	t.Run("idempotent - session not found", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		// Should not error
		err := manager.StopContainerSession(context.Background(), "non-existent")
		if err != nil {
			t.Errorf("Expected no error for non-existent session, got %v", err)
		}
	})

	t.Run("fails when container stop fails", func(t *testing.T) {
		docker := &mockDockerClient{
			stopFn: func(ctx context.Context, containerID string, options container.StopOptions) error {
				return errors.New("stop failed")
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		session, _ := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		_ = manager.StartContainerSession(context.Background(), session.ID())

		err := manager.StopContainerSession(context.Background(), session.ID())
		if err == nil {
			t.Error("Expected error when container stop fails")
		}
	})
}

func TestGetContainerSession(t *testing.T) {
	t.Run("returns session when found", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		created, _ := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		retrieved := manager.GetContainerSession(created.ID())
		if retrieved == nil {
			t.Error("Expected non-nil session")
		}

		if retrieved.ID() != created.ID() {
			t.Errorf("Expected session ID %s, got %s", created.ID(), retrieved.ID())
		}
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		session := manager.GetContainerSession("non-existent")
		if session != nil {
			t.Error("Expected nil session")
		}
	})
}

func TestListContainerSessions(t *testing.T) {
	t.Run("returns empty list when no sessions", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		sessions := manager.ListContainerSessions()
		if len(sessions) != 0 {
			t.Errorf("Expected 0 sessions, got %d", len(sessions))
		}
	})

	t.Run("returns all sessions", func(t *testing.T) {
		docker := &mockDockerClient{}
		idGen := &mockIDGenerator{}
		manager := NewManager(docker, idGen, &mockClock{}, &mockLogger{}, t.TempDir())

		// Create multiple sessions
		idGen.nextID = "session-1"
		_, _ = manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		idGen.nextID = "session-2"
		_, _ = manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		sessions := manager.ListContainerSessions()
		if len(sessions) != 2 {
			t.Errorf("Expected 2 sessions, got %d", len(sessions))
		}
	})
}

// Phase 2: Container Reuse & Attach Tests

func TestFindContainer(t *testing.T) {
	t.Run("returns container ID and state when found", func(t *testing.T) {
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{
					{ID: "found-container-123", State: "running"},
				}, nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		containerID, state, err := manager.findContainer(context.Background(), "test-session")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if containerID != "found-container-123" {
			t.Errorf("Expected container ID 'found-container-123', got %s", containerID)
		}
		if state != "running" {
			t.Errorf("Expected state 'running', got %s", state)
		}
	})

	t.Run("returns empty when no container found", func(t *testing.T) {
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{}, nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		containerID, state, err := manager.findContainer(context.Background(), "test-session")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if containerID != "" {
			t.Errorf("Expected empty container ID, got %s", containerID)
		}
		if state != "" {
			t.Errorf("Expected empty state, got %s", state)
		}
	})

	t.Run("handles multiple containers - returns first", func(t *testing.T) {
		logger := &mockLogger{}
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{
					{ID: "container-1", State: "running"},
					{ID: "container-2", State: "running"},
				}, nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, logger, t.TempDir())

		containerID, state, err := manager.findContainer(context.Background(), "test-session")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if containerID != "container-1" {
			t.Errorf("Expected first container 'container-1', got %s", containerID)
		}
		if state != "running" {
			t.Errorf("Expected state 'running', got %s", state)
		}

		// Verify warning was logged
		logger.mu.Lock()
		foundWarning := false
		for _, log := range logger.logs {
			if strings.HasPrefix(log, "WARNING") {
				foundWarning = true
				break
			}
		}
		logger.mu.Unlock()
		if !foundWarning {
			t.Error("Expected warning log for multiple containers")
		}
	})

	t.Run("returns different states correctly", func(t *testing.T) {
		states := []string{"running", "created", "exited", "paused", "dead"}
		for _, expectedState := range states {
			docker := &mockDockerClient{
				listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
					return []container.Summary{
						{ID: "test-container", State: expectedState},
					}, nil
				},
			}
			manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

			_, state, err := manager.findContainer(context.Background(), "test-session")
			if err != nil {
				t.Fatalf("Expected no error for state %s, got %v", expectedState, err)
			}
			if state != expectedState {
				t.Errorf("Expected state '%s', got '%s'", expectedState, state)
			}
		}
	})

	t.Run("returns error when ContainerList fails", func(t *testing.T) {
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return nil, errors.New("docker API error")
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		_, _, err := manager.findContainer(context.Background(), "test-session")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !errors.Is(err, errors.New("docker API error")) && err.Error() != "failed to list containers: docker API error" {
			t.Errorf("Expected wrapped error, got %v", err)
		}
	})
}

func TestHandleExistingContainer(t *testing.T) {
	t.Run("reattaches to running container", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-session")
		docker := &mockDockerClient{
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: workspacePath, Destination: "/workspace"},
					},
					Config: &container.Config{
						Labels: map[string]string{
							LabelSessionID: "test-session",
							LabelCreatedAt: "2024-01-01T12:00:00Z",
						},
					},
				}, nil
			},
			attachFn: func(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
				return newMockHijackedResponse(), nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, baseDir)

		session, err := manager.handleExistingContainer(context.Background(), "container-123", "running", "test-session")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if session == nil {
			t.Fatal("Expected non-nil session")
		}
		if session.ContainerID() != "container-123" {
			t.Errorf("Expected container ID 'container-123', got %s", session.ContainerID())
		}
		if session.State() != StateRunning {
			t.Errorf("Expected state RUNNING, got %s", session.State())
		}
	})

	t.Run("starts and attaches to stopped container", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-session")
		startCalled := false
		docker := &mockDockerClient{
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: workspacePath, Destination: "/workspace"},
					},
					Config: &container.Config{
						Labels: map[string]string{
							LabelSessionID: "test-session",
						},
					},
				}, nil
			},
			startFn: func(ctx context.Context, containerID string, options container.StartOptions) error {
				startCalled = true
				return nil
			},
			attachFn: func(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
				return newMockHijackedResponse(), nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, baseDir)

		session, err := manager.handleExistingContainer(context.Background(), "container-123", "exited", "test-session")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !startCalled {
			t.Error("Expected ContainerStart to be called")
		}
		if session.State() != StateRunning {
			t.Errorf("Expected state RUNNING after start, got %s", session.State())
		}
	})

	t.Run("returns error for paused container", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-session")
		docker := &mockDockerClient{
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: workspacePath, Destination: "/workspace"},
					},
					Config: &container.Config{
						Labels: map[string]string{
							LabelSessionID: "test-session",
						},
					},
				}, nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, baseDir)

		_, err := manager.handleExistingContainer(context.Background(), "container-123", "paused", "test-session")
		if err == nil {
			t.Fatal("Expected error for paused container, got nil")
		}
	})

	t.Run("removes dead container and returns error", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-session")
		removeCalled := false
		docker := &mockDockerClient{
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: workspacePath, Destination: "/workspace"},
					},
					Config: &container.Config{
						Labels: map[string]string{
							LabelSessionID: "test-session",
						},
					},
				}, nil
			},
			removeFn: func(ctx context.Context, containerID string, options container.RemoveOptions) error {
				removeCalled = true
				return nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, baseDir)

		_, err := manager.handleExistingContainer(context.Background(), "container-123", "dead", "test-session")
		if err == nil {
			t.Fatal("Expected error for dead container, got nil")
		}
		if !removeCalled {
			t.Error("Expected ContainerRemove to be called for dead container")
		}
	})

	t.Run("returns error when no workspace mount found", func(t *testing.T) {
		docker := &mockDockerClient{
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: "/host/other", Destination: "/other"},
					},
					Config: &container.Config{
						Labels: map[string]string{},
					},
				}, nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		_, err := manager.handleExistingContainer(context.Background(), "container-123", "running", "test-session")
		if err == nil {
			t.Fatal("Expected error for missing workspace mount, got nil")
		}
	})

	t.Run("returns error when inspect fails", func(t *testing.T) {
		docker := &mockDockerClient{
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{}, errors.New("inspect failed")
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		_, err := manager.handleExistingContainer(context.Background(), "container-123", "running", "test-session")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
	})
}

func TestAttachContainerSession(t *testing.T) {
	t.Run("successfully attaches to running container", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-session")
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{
					{ID: "container-123", State: "running"},
				}, nil
			},
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: workspacePath, Destination: "/workspace"},
					},
					Config: &container.Config{
						Labels: map[string]string{
							LabelSessionID: "test-session",
							LabelCreatedAt: "2024-01-01T12:00:00Z",
						},
					},
				}, nil
			},
			attachFn: func(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
				return newMockHijackedResponse(), nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, baseDir)

		session, err := manager.AttachContainerSession(context.Background(), "test-session")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if session == nil {
			t.Fatal("Expected non-nil session")
		}
		if session.ID() != "test-session" {
			t.Errorf("Expected session ID 'test-session', got %s", session.ID())
		}
		if session.ContainerID() != "container-123" {
			t.Errorf("Expected container ID 'container-123', got %s", session.ContainerID())
		}
	})

	t.Run("returns error when container not found", func(t *testing.T) {
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{}, nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		_, err := manager.AttachContainerSession(context.Background(), "test-session")
		if err == nil {
			t.Fatal("Expected error for not found, got nil")
		}
		if !errors.Is(err, ErrSessionNotFound) && err.Error() != "session not found: test-session" {
			t.Errorf("Expected ErrSessionNotFound, got %v", err)
		}
	})

	t.Run("returns error when container not running", func(t *testing.T) {
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{
					{ID: "container-123", State: "exited"},
				}, nil
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		_, err := manager.AttachContainerSession(context.Background(), "test-session")
		if err == nil {
			t.Fatal("Expected error for non-running container, got nil")
		}
	})

	t.Run("continues even if attach fails", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-session")
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{
					{ID: "container-123", State: "running"},
				}, nil
			},
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: workspacePath, Destination: "/workspace"},
					},
					Config: &container.Config{
						Labels: map[string]string{
							LabelSessionID: "test-session",
						},
					},
				}, nil
			},
			attachFn: func(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
				return types.HijackedResponse{}, errors.New("attach failed")
			},
		}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, baseDir)

		session, err := manager.AttachContainerSession(context.Background(), "test-session")
		if err != nil {
			t.Fatalf("Expected no error (attach failure is logged but not fatal), got %v", err)
		}
		if session == nil {
			t.Fatal("Expected non-nil session even with attach failure")
		}
	})
}

func TestCreateContainerSession_Reuse(t *testing.T) {
	t.Run("reuses running container", func(t *testing.T) {
		createCalled := false
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-session")
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{
					{ID: "existing-container", State: "running"},
				}, nil
			},
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: workspacePath, Destination: "/workspace"},
					},
					Config: &container.Config{
						Labels: map[string]string{
							LabelSessionID: "test-123",
						},
					},
				}, nil
			},
			attachFn: func(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
				return newMockHijackedResponse(), nil
			},
			createFn: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
				createCalled = true
				return container.CreateResponse{ID: "new-container"}, nil
			},
		}
		idGen := &mockIDGenerator{nextID: "test-123"}
		manager := NewManager(docker, idGen, &mockClock{}, &mockLogger{}, baseDir)

		session, err := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if createCalled {
			t.Error("Expected ContainerCreate to NOT be called (should reuse existing)")
		}
		if session.ContainerID() != "existing-container" {
			t.Errorf("Expected existing container ID 'existing-container', got %s", session.ContainerID())
		}
	})

	t.Run("creates new when no existing container", func(t *testing.T) {
		createCalled := false
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{}, nil
			},
			createFn: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
				createCalled = true
				return container.CreateResponse{ID: "new-container"}, nil
			},
		}
		idGen := &mockIDGenerator{nextID: "test-123"}
		manager := NewManager(docker, idGen, &mockClock{}, &mockLogger{}, t.TempDir())

		session, err := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !createCalled {
			t.Error("Expected ContainerCreate to be called")
		}
		if session.ContainerID() != "new-container" {
			t.Errorf("Expected new container ID 'new-container', got %s", session.ContainerID())
		}
	})

	t.Run("starts stopped container", func(t *testing.T) {
		baseDir := t.TempDir()
		workspacePath := filepath.Join(baseDir, "test-session")
		startCalled := false
		createCalled := false
		docker := &mockDockerClient{
			listFn: func(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
				return []container.Summary{
					{ID: "stopped-container", State: "exited"},
				}, nil
			},
			inspectFn: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
				return container.InspectResponse{
					ContainerJSONBase: &container.ContainerJSONBase{
						ID: containerID,
					},
					Mounts: []container.MountPoint{
						{Source: workspacePath, Destination: "/workspace"},
					},
					Config: &container.Config{
						Labels: map[string]string{
							LabelSessionID: "test-123",
						},
					},
				}, nil
			},
			startFn: func(ctx context.Context, containerID string, options container.StartOptions) error {
				startCalled = true
				return nil
			},
			attachFn: func(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error) {
				return newMockHijackedResponse(), nil
			},
			createFn: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
				createCalled = true
				return container.CreateResponse{ID: "new-container"}, nil
			},
		}
		idGen := &mockIDGenerator{nextID: "test-123"}
		manager := NewManager(docker, idGen, &mockClock{}, &mockLogger{}, baseDir)

		session, err := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if createCalled {
			t.Error("Expected ContainerCreate to NOT be called (should reuse existing)")
		}
		if !startCalled {
			t.Error("Expected ContainerStart to be called for stopped container")
		}
		if session.ContainerID() != "stopped-container" {
			t.Errorf("Expected stopped container ID 'stopped-container', got %s", session.ContainerID())
		}
	})
}

func TestCreateContainerSession_TimestampInvariant(t *testing.T) {
	// This test ensures that CreateContainerSession uses a single
	// timestamp for both the session's createdAt field and the container labels.
	// This prevents timestamp drift and test flakiness.
	t.Run("uses consistent timestamp across session and labels", func(t *testing.T) {
		// Set up a frozen clock with a specific timestamp
		frozenTime := time.Date(2025, 11, 7, 15, 30, 45, 0, time.UTC)
		docker := &mockDockerClient{
			createFn: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
				// Verify the label timestamp matches our frozen time
				labelTimestamp, exists := config.Labels[LabelCreatedAt]
				if !exists {
					t.Error("Expected LabelCreatedAt to exist in container labels")
				}

				// Parse the label timestamp
				parsedTime, err := time.Parse(time.RFC3339, labelTimestamp)
				if err != nil {
					t.Errorf("Failed to parse label timestamp: %v", err)
				}

				// Verify label timestamp matches frozen time
				if !parsedTime.Equal(frozenTime) {
					t.Errorf("Label timestamp %v does not match frozen time %v", parsedTime, frozenTime)
				}

				return container.CreateResponse{ID: "test-container-id"}, nil
			},
		}
		idGen := &mockIDGenerator{nextID: "test-timestamp-123"}
		clock := &mockClock{now: frozenTime}
		logger := &mockLogger{}

		manager := NewManager(docker, idGen, clock, logger, t.TempDir())

		session, err := manager.CreateContainerSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify session's createdAt matches frozen time
		if !session.CreatedAt().Equal(frozenTime) {
			t.Errorf("Session createdAt %v does not match frozen time %v", session.CreatedAt(), frozenTime)
		}

		// Verify label timestamp matches frozen time (via our createFn check above)
		labels := session.Labels()
		labelTimestamp, exists := labels[LabelCreatedAt]
		if !exists {
			t.Error("Expected LabelCreatedAt to exist in session labels")
		}

		parsedTime, err := time.Parse(time.RFC3339, labelTimestamp)
		if err != nil {
			t.Errorf("Failed to parse session label timestamp: %v", err)
		}

		if !parsedTime.Equal(frozenTime) {
			t.Errorf("Session label timestamp %v does not match frozen time %v", parsedTime, frozenTime)
		}

		// Critical invariant: session.CreatedAt() and parsed label timestamp must be equal
		if !session.CreatedAt().Equal(parsedTime) {
			t.Errorf("Session createdAt %v does not match label timestamp %v", session.CreatedAt(), parsedTime)
		}
	})
}
