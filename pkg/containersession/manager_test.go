package containersession

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Mock implementations for testing

type mockDockerClient struct {
	createFn func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	startFn  func(ctx context.Context, containerID string, options container.StartOptions) error
	stopFn   func(ctx context.Context, containerID string, options container.StopOptions) error
	attachFn func(ctx context.Context, containerID string, options container.AttachOptions) (types.HijackedResponse, error)
	listFn   func(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	removeFn func(ctx context.Context, containerID string, options container.RemoveOptions) error
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
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	// Store logs for verification
	m.logs = append(m.logs, format)
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

func TestCreateSession(t *testing.T) {
	t.Run("creates session successfully", func(t *testing.T) {
		docker := &mockDockerClient{}
		idGen := &mockIDGenerator{nextID: "test-123"}
		clock := &mockClock{}
		logger := &mockLogger{}

		manager := NewManager(docker, idGen, clock, logger, t.TempDir())

		session, err := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
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

		session, err := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if session != nil {
			t.Error("Expected nil session on error")
		}

		// Session should not be in manager's map
		if manager.GetSession("test-session-id") != nil {
			t.Error("Session should not be stored after creation failure")
		}
	})
}

func TestStartSession(t *testing.T) {
	t.Run("starts session successfully", func(t *testing.T) {
		docker := &mockDockerClient{}
		idGen := &mockIDGenerator{}
		clock := &mockClock{}
		logger := &mockLogger{}

		manager := NewManager(docker, idGen, clock, logger, t.TempDir())

		session, _ := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		err := manager.StartSession(context.Background(), session.ID())
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

		err := manager.StartSession(context.Background(), "non-existent")
		if err == nil {
			t.Error("Expected error for non-existent session")
		}
	})

	t.Run("fails when session in wrong state", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		session, _ := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		session.SetState(StateRunning) // Already running

		err := manager.StartSession(context.Background(), session.ID())
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

		session, _ := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		err := manager.StartSession(context.Background(), session.ID())
		if err == nil {
			t.Error("Expected error when container start fails")
		}

		if session.State() != StateFailed {
			t.Errorf("Expected state FAILED, got %s", session.State())
		}
	})
}

func TestStopSession(t *testing.T) {
	t.Run("stops session successfully", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		session, _ := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		_ = manager.StartSession(context.Background(), session.ID())

		err := manager.StopSession(context.Background(), session.ID())
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

		session, _ := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		_ = manager.StartSession(context.Background(), session.ID())
		_ = manager.StopSession(context.Background(), session.ID())

		// Stop again - should be idempotent
		err := manager.StopSession(context.Background(), session.ID())
		if err != nil {
			t.Errorf("Expected no error for idempotent stop, got %v", err)
		}
	})

	t.Run("idempotent - session not found", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		// Should not error
		err := manager.StopSession(context.Background(), "non-existent")
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

		session, _ := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})
		_ = manager.StartSession(context.Background(), session.ID())

		err := manager.StopSession(context.Background(), session.ID())
		if err == nil {
			t.Error("Expected error when container stop fails")
		}
	})
}

func TestGetSession(t *testing.T) {
	t.Run("returns session when found", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		created, _ := manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		retrieved := manager.GetSession(created.ID())
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

		session := manager.GetSession("non-existent")
		if session != nil {
			t.Error("Expected nil session")
		}
	})
}

func TestListSessions(t *testing.T) {
	t.Run("returns empty list when no sessions", func(t *testing.T) {
		docker := &mockDockerClient{}
		manager := NewManager(docker, &mockIDGenerator{}, &mockClock{}, &mockLogger{}, t.TempDir())

		sessions := manager.ListSessions()
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
		_, _ = manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		idGen.nextID = "session-2"
		_, _ = manager.CreateSession(context.Background(), "ubuntu:latest", []string{"/bin/bash"})

		sessions := manager.ListSessions()
		if len(sessions) != 2 {
			t.Errorf("Expected 2 sessions, got %d", len(sessions))
		}
	})
}
