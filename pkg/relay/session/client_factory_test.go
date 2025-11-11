package session

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// TestACPClientFactory_MissingAPIKey tests error when ANTHROPIC_API_KEY not set
func TestACPClientFactory_MissingAPIKey(t *testing.T) {
	// Unset the key (t.Setenv automatically restores original value)
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := NewACPClientFactory(nil, nil)
	if err == nil {
		t.Fatal("Expected error when ANTHROPIC_API_KEY not set, got nil")
	}
	if err != ErrMissingAnthropicAPIKey {
		t.Errorf("Expected ErrMissingAnthropicAPIKey, got: %v", err)
	}
}

// TestACPClientFactory_WithAPIKey tests successful factory creation
func TestACPClientFactory_WithAPIKey(t *testing.T) {
	// Set API key (t.Setenv automatically restores original value)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error with API key set, got: %v", err)
	}
	if factory == nil {
		t.Fatal("Expected factory, got nil")
	}
}

// TestFakeClientFactory tests the fake factory
func TestFakeClientFactory(t *testing.T) {
	called := false
	var receivedWorkspace string
	expectedClient := &mockACPClient{}

	factory := NewFakeClientFactory(func(workspace string) (ACPClient, error) {
		called = true
		receivedWorkspace = workspace
		return expectedClient, nil
	})

	runtime := &AgentRuntimeContext{Workspace: "test-workspace"}
	client, err := factory.NewClient(context.Background(), runtime)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if client == nil {
		t.Fatal("Expected client, got nil")
	}
	if !called {
		t.Error("Expected clientFunc to be called")
	}
	if receivedWorkspace != "test-workspace" {
		t.Errorf("Expected workspace 'test-workspace', got '%s'", receivedWorkspace)
	}
	if client != expectedClient {
		t.Error("Expected returned client to be the same instance created by clientFunc")
	}
}

// TestFakeClientFactory_Error tests error handling in fake factory
func TestFakeClientFactory_Error(t *testing.T) {
	expectedError := fmt.Errorf("simulated client creation error")
	factory := NewFakeClientFactory(func(workspace string) (ACPClient, error) {
		return nil, expectedError
	})

	runtime := &AgentRuntimeContext{Workspace: "test-workspace"}
	client, err := factory.NewClient(context.Background(), runtime)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
	if client != nil {
		t.Error("Expected nil client on error, got non-nil")
	}
}

// TestACPClientFactory_CustomBinary tests that custom ACP binary path is respected
func TestACPClientFactory_CustomBinary(t *testing.T) {
	// Set environment variables (t.Setenv automatically restores original values)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_BINARY", "/path/to/echo-agent")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if factory.GetACPBinaryPath() != "/path/to/echo-agent" {
		t.Errorf("Expected acpBinaryPath='/path/to/echo-agent', got '%s'", factory.GetACPBinaryPath())
	}
}

// TestACPClientFactory_DefaultBinary tests that binary path defaults to empty when env var not set
func TestACPClientFactory_DefaultBinary(t *testing.T) {
	// Set API key but unset custom binary (t.Setenv automatically restores original values)
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_BINARY", "")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if factory.GetACPBinaryPath() != "" {
		t.Errorf("Expected acpBinaryPath='', got '%s'", factory.GetACPBinaryPath())
	}
}

// TestGetRuntimeMode_Default tests that empty env var returns "host" as default
func TestGetRuntimeMode_Default(t *testing.T) {
	t.Setenv("OUROCODUS_ACP_RUNTIME", "")

	mode, err := getRuntimeMode()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if mode != "host" {
		t.Errorf("Expected mode='host' (default), got '%s'", mode)
	}
}

// TestGetRuntimeMode_Host tests explicit "host" value
func TestGetRuntimeMode_Host(t *testing.T) {
	t.Setenv("OUROCODUS_ACP_RUNTIME", "host")

	mode, err := getRuntimeMode()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if mode != "host" {
		t.Errorf("Expected mode='host', got '%s'", mode)
	}
}

// TestGetRuntimeMode_Container tests "container" value
func TestGetRuntimeMode_Container(t *testing.T) {
	t.Setenv("OUROCODUS_ACP_RUNTIME", "container")

	mode, err := getRuntimeMode()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if mode != "container" {
		t.Errorf("Expected mode='container', got '%s'", mode)
	}
}

// TestGetRuntimeMode_Invalid tests error for invalid values
func TestGetRuntimeMode_Invalid(t *testing.T) {
	testCases := []struct {
		name  string
		value string
	}{
		{"invalid string", "invalid"},
		{"kubernetes", "kubernetes"},
		{"CONTAINER", "CONTAINER"},
		{"Host", "Host"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OUROCODUS_ACP_RUNTIME", tc.value)

			mode, err := getRuntimeMode()
			if err == nil {
				t.Fatalf("Expected error for invalid value '%s', got nil", tc.value)
			}
			if mode != "" {
				t.Errorf("Expected empty mode on error, got '%s'", mode)
			}
			expectedMsg := fmt.Sprintf("invalid OUROCODUS_ACP_RUNTIME value: %q (must be 'host' or 'container')", tc.value)
			if err.Error() != expectedMsg {
				t.Errorf("Expected error message: %q, got: %q", expectedMsg, err.Error())
			}
		})
	}
}

// TestValidateContainerPrerequisites_Success tests validation passes with all prerequisites
func TestValidateContainerPrerequisites_Success(t *testing.T) {
	runtime := &AgentRuntimeContext{
		SessionID:   "session-1",
		AgentID:     "agent-1",
		Workspace:   "/workspace",
		ContainerID: "container-123",
	}
	mockManager := &mockContainerExecService{}

	err := validateContainerPrerequisites(runtime, mockManager)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// TestValidateContainerPrerequisites_MissingContainerID tests error when no container ID
func TestValidateContainerPrerequisites_MissingContainerID(t *testing.T) {
	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/workspace",
		// ContainerID is empty
	}
	mockManager := &mockContainerExecService{}

	err := validateContainerPrerequisites(runtime, mockManager)
	if err == nil {
		t.Fatal("Expected error when container ID missing, got nil")
	}
	expectedMsg := "container runtime requested but no container ID in runtime context (session=session-1 agent=agent-1)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// TestValidateContainerPrerequisites_NilManager tests error when manager is nil
func TestValidateContainerPrerequisites_NilManager(t *testing.T) {
	runtime := &AgentRuntimeContext{
		SessionID:   "session-1",
		AgentID:     "agent-1",
		Workspace:   "/workspace",
		ContainerID: "container-123",
	}

	err := validateContainerPrerequisites(runtime, nil)
	if err == nil {
		t.Fatal("Expected error when manager is nil, got nil")
	}
	expectedMsg := "container runtime requested but container session manager not available (session=session-1 agent=agent-1)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// TestValidateContainerPrerequisites_NilRuntime tests error when runtime is nil
func TestValidateContainerPrerequisites_NilRuntime(t *testing.T) {
	mockManager := &mockContainerExecService{}

	err := validateContainerPrerequisites(nil, mockManager)
	if err == nil {
		t.Fatal("Expected error when runtime is nil, got nil")
	}
	expectedMsg := "container runtime requested but runtime context is nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// TestCreateHostLauncher_WithoutLogger tests host launcher creation without logging
func TestCreateHostLauncher_WithoutLogger(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/workspace",
	}

	launcher := factory.createHostLauncher(runtime)
	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
}

// TestCreateHostLauncher_WithLogger tests host launcher creation with logging
func TestCreateHostLauncher_WithLogger(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	logger := &mockLogger{}
	factory, err := NewACPClientFactory(nil, logger)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/workspace",
	}

	launcher := factory.createHostLauncher(runtime)
	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
	if len(logger.messages) == 0 {
		t.Fatal("Expected logger to be called")
	}
	expectedMsg := "[ACP] Using host process launcher for session=session-1 agent=agent-1"
	if logger.messages[0] != expectedMsg {
		t.Errorf("Expected log message: %q, got: %q", expectedMsg, logger.messages[0])
	}
}

// TestCreateContainerLauncher_WithoutLogger tests container launcher creation without logging
func TestCreateContainerLauncher_WithoutLogger(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	mockManager := &mockContainerExecService{}
	factory, err := NewACPClientFactory(mockManager, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID:   "session-1",
		AgentID:     "agent-1",
		Workspace:   "/workspace",
		ContainerID: "container-123",
	}

	launcher, err := factory.createContainerLauncher(runtime)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
	// Verify it's a ContainerAttachProcessLauncher
	if _, ok := launcher.(*ContainerAttachProcessLauncher); !ok {
		t.Error("Expected ContainerAttachProcessLauncher type")
	}
}

// TestCreateContainerLauncher_WithLogger tests container launcher creation with logging
func TestCreateContainerLauncher_WithLogger(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	logger := &mockLogger{}
	mockManager := &mockContainerExecService{}
	factory, err := NewACPClientFactory(mockManager, logger)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID:   "session-1",
		AgentID:     "agent-1",
		Workspace:   "/workspace",
		ContainerID: "container-123",
	}

	launcher, err := factory.createContainerLauncher(runtime)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
	if len(logger.messages) == 0 {
		t.Fatal("Expected logger to be called")
	}
	expectedMsg := "[ACP] Using container attach launcher for session=session-1 agent=agent-1 container=container-123"
	if logger.messages[0] != expectedMsg {
		t.Errorf("Expected log message: %q, got: %q", expectedMsg, logger.messages[0])
	}
}

// TestSelectLauncher_HostMode_Default tests launcher selection with default (empty) env var
func TestSelectLauncher_HostMode_Default(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "") // Default to host

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/workspace",
	}

	launcher, err := factory.selectLauncher(runtime)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
}

// TestSelectLauncher_HostMode_Explicit tests launcher selection with explicit "host" value
func TestSelectLauncher_HostMode_Explicit(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "host")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/workspace",
	}

	launcher, err := factory.selectLauncher(runtime)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
}

// TestSelectLauncher_ContainerMode_Success tests successful container launcher selection
func TestSelectLauncher_ContainerMode_Success(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "container")

	mockManager := &mockContainerExecService{}
	factory, err := NewACPClientFactory(mockManager, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID:   "session-1",
		AgentID:     "agent-1",
		Workspace:   "/workspace",
		ContainerID: "container-123",
	}

	launcher, err := factory.selectLauncher(runtime)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if launcher == nil {
		t.Fatal("Expected launcher, got nil")
	}
	if _, ok := launcher.(*ContainerAttachProcessLauncher); !ok {
		t.Error("Expected ContainerAttachProcessLauncher type")
	}
}

// TestSelectLauncher_ContainerMode_MissingContainerID tests error when container mode requested without container ID
func TestSelectLauncher_ContainerMode_MissingContainerID(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "container")

	mockManager := &mockContainerExecService{}
	factory, err := NewACPClientFactory(mockManager, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/workspace",
		// No ContainerID
	}

	launcher, err := factory.selectLauncher(runtime)
	if err == nil {
		t.Fatal("Expected error when container ID missing, got nil")
	}
	if launcher != nil {
		t.Error("Expected nil launcher on error")
	}
	expectedMsg := "container runtime requested but no container ID in runtime context (session=session-1 agent=agent-1)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// TestSelectLauncher_ContainerMode_MissingManager tests error when container mode requested without manager
func TestSelectLauncher_ContainerMode_MissingManager(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "container")

	factory, err := NewACPClientFactory(nil, nil) // No container manager
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID:   "session-1",
		AgentID:     "agent-1",
		Workspace:   "/workspace",
		ContainerID: "container-123",
	}

	launcher, err := factory.selectLauncher(runtime)
	if err == nil {
		t.Fatal("Expected error when manager missing, got nil")
	}
	if launcher != nil {
		t.Error("Expected nil launcher on error")
	}
	expectedMsg := "container runtime requested but container session manager not available (session=session-1 agent=agent-1)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// TestSelectLauncher_InvalidMode tests error for invalid runtime mode
func TestSelectLauncher_InvalidMode(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "kubernetes")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/workspace",
	}

	launcher, err := factory.selectLauncher(runtime)
	if err == nil {
		t.Fatal("Expected error for invalid mode, got nil")
	}
	if launcher != nil {
		t.Error("Expected nil launcher on error")
	}
	// Error should contain the invalid value and session/agent context
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

// Integration Tests - Full flow from factory to client creation

// TestNewClient_Integration_HostMode tests full client creation flow in host mode
func TestNewClient_Integration_HostMode(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "host")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/tmp/test-workspace",
	}

	// This will fail because we don't have a real workspace, but it proves launcher selection works
	_, err = factory.NewClient(context.Background(), runtime)
	if err == nil {
		t.Fatal("Expected error due to missing workspace, got nil")
	}

	// Error should be about workspace/command, not launcher selection
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

// TestNewClient_Integration_ContainerMode_MissingPrerequisites tests error handling
func TestNewClient_Integration_ContainerMode_MissingPrerequisites(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "container")

	factory, err := NewACPClientFactory(nil, nil) // No container manager
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID:   "session-1",
		AgentID:     "agent-1",
		Workspace:   "/tmp/test-workspace",
		ContainerID: "container-123",
	}

	_, err = factory.NewClient(context.Background(), runtime)
	if err == nil {
		t.Fatal("Expected error for missing container manager, got nil")
	}

	expectedMsg := "failed to select launcher: container runtime requested but container session manager not available (session=session-1 agent=agent-1)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// TestNewClient_Integration_ContainerMode_MissingContainerID tests error for missing container
func TestNewClient_Integration_ContainerMode_MissingContainerID(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OUROCODUS_ACP_RUNTIME", "container")

	mockManager := &mockContainerExecService{}
	factory, err := NewACPClientFactory(mockManager, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "/tmp/test-workspace",
		// No ContainerID
	}

	_, err = factory.NewClient(context.Background(), runtime)
	if err == nil {
		t.Fatal("Expected error for missing container ID, got nil")
	}

	expectedMsg := "failed to select launcher: container runtime requested but no container ID in runtime context (session=session-1 agent=agent-1)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// TestNewClient_Integration_NilRuntime tests error for nil runtime
func TestNewClient_Integration_NilRuntime(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	_, err = factory.NewClient(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error for nil runtime, got nil")
	}

	expectedMsg := "runtime context is required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// TestNewClient_Integration_EmptyWorkspace tests error for empty workspace
func TestNewClient_Integration_EmptyWorkspace(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	factory, err := NewACPClientFactory(nil, nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	runtime := &AgentRuntimeContext{
		SessionID: "session-1",
		AgentID:   "agent-1",
		Workspace: "", // Empty workspace
	}

	_, err = factory.NewClient(context.Background(), runtime)
	if err == nil {
		t.Fatal("Expected error for empty workspace, got nil")
	}

	expectedMsg := "workspace is required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error: %q, got: %q", expectedMsg, err.Error())
	}
}

// Mock implementations for testing

type mockContainerExecService struct{}

func (m *mockContainerExecService) ExecInContainer(ctx context.Context, containerID string, cfg containersession.ExecConfig) (*containersession.ExecAttachment, error) {
	return nil, nil
}

func (m *mockContainerExecService) GetDockerClient() containersession.DockerClient {
	return &mockDockerClient{}
}

// mockDockerClient is a minimal mock implementation of DockerClient for testing
type mockDockerClient struct{}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *dockercontainer.Config,
	hostConfig *dockercontainer.HostConfig, networkingConfig *network.NetworkingConfig,
	platform *ocispec.Platform, containerName string,
) (dockercontainer.CreateResponse, error) {
	return dockercontainer.CreateResponse{}, nil
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string,
	options dockercontainer.StartOptions,
) error {
	return nil
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string,
	options dockercontainer.StopOptions,
) error {
	return nil
}

func (m *mockDockerClient) ContainerAttach(ctx context.Context, containerID string,
	options dockercontainer.AttachOptions,
) (types.HijackedResponse, error) {
	return types.HijackedResponse{}, nil
}

func (m *mockDockerClient) ContainerExecCreate(ctx context.Context, containerID string,
	config dockercontainer.ExecOptions,
) (dockercontainer.ExecCreateResponse, error) {
	return dockercontainer.ExecCreateResponse{}, nil
}

func (m *mockDockerClient) ContainerExecAttach(ctx context.Context, execID string,
	config dockercontainer.ExecAttachOptions,
) (types.HijackedResponse, error) {
	return types.HijackedResponse{}, nil
}

func (m *mockDockerClient) ContainerExecInspect(ctx context.Context, execID string) (dockercontainer.ExecInspect, error) {
	return dockercontainer.ExecInspect{}, nil
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string,
	options dockercontainer.RemoveOptions,
) error {
	return nil
}

func (m *mockDockerClient) ContainerList(ctx context.Context,
	options dockercontainer.ListOptions,
) ([]dockercontainer.Summary, error) {
	return nil, nil
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (dockercontainer.InspectResponse, error) {
	return dockercontainer.InspectResponse{}, nil
}

func (m *mockDockerClient) ImagePull(ctx context.Context, refStr string,
	options image.PullOptions,
) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
