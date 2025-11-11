package session

import (
	"context"
	"fmt"
	"io"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/containersession"
)

// ContainerExecService abstracts docker exec operations for easier testing.
type ContainerExecService interface {
	ExecInContainer(ctx context.Context, containerID string, cfg containersession.ExecConfig) (*containersession.ExecAttachment, error)
}

// ContainerExecProcessLauncher runs ACP inside an existing agent container via docker exec.
type ContainerExecProcessLauncher struct {
	execService   ContainerExecService
	containerID   string
	workspacePath string
}

// NewContainerExecProcessLauncher constructs a container-based ProcessLauncher.
func NewContainerExecProcessLauncher(service ContainerExecService, containerID string) *ContainerExecProcessLauncher {
	return &ContainerExecProcessLauncher{
		execService:   service,
		containerID:   containerID,
		workspacePath: "/workspace",
	}
}

// WithWorkspacePath overrides the default in-container workspace path.
func (l *ContainerExecProcessLauncher) WithWorkspacePath(path string) *ContainerExecProcessLauncher {
	if path != "" {
		l.workspacePath = path
	}
	return l
}

// Start implements acp.ProcessLauncher.
func (l *ContainerExecProcessLauncher) Start(ctx context.Context, cfg acp.ProcessLaunchConfig) (acp.Transport, error) {
	if l.execService == nil {
		return nil, fmt.Errorf("containersession manager is required")
	}
	if l.containerID == "" {
		return nil, fmt.Errorf("container ID is required for container exec launcher")
	}
	if cfg.CommandPath == "" {
		return nil, fmt.Errorf("command path is required")
	}

	command := buildExecCommand(cfg.CommandPath, cfg.CommandArgs)
	env := mergeEnvMaps(cfg.Env, map[string]string{
		"ANTHROPIC_API_KEY": cfg.APIKey,
	})

	// Determine workspace path
	workspacePath := l.workspacePath
	if workspacePath == "" {
		// Default: assume standard container mount at /workspace
		// In production, this should be configurable via runtime context
		workspacePath = "/workspace"
	}

	execCfg := containersession.ExecConfig{
		Command:    command,
		Env:        env,
		WorkingDir: workspacePath,
	}

	attachment, err := l.execService.ExecInContainer(ctx, l.containerID, execCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to exec ACP command %q in container %s: %w", cfg.CommandPath, l.containerID, err)
	}

	return &containerExecTransport{attachment: attachment}, nil
}

func buildExecCommand(command string, args []string) []string {
	cmd := make([]string, 0, 1+len(args))
	cmd = append(cmd, command)
	cmd = append(cmd, args...)
	return cmd
}

func mergeEnvMaps(base map[string]string, override map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

type containerExecTransport struct {
	attachment *containersession.ExecAttachment
}

func (t *containerExecTransport) Read(p []byte) (int, error) {
	return t.attachment.Stdout().Read(p)
}

func (t *containerExecTransport) Write(p []byte) (int, error) {
	return t.attachment.Stdin().Write(p)
}

func (t *containerExecTransport) Close() error {
	return t.attachment.Close()
}

func (t *containerExecTransport) Stderr() io.Reader {
	return t.attachment.Stderr()
}
