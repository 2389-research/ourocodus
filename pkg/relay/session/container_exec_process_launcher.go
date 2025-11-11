package session

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/containersession"
)

// ContainerExecService abstracts docker exec operations for easier testing.
type ContainerExecService interface {
	ExecInContainer(ctx context.Context, containerID string, cfg containersession.ExecConfig) (*containersession.ExecAttachment, error)
}

// DefaultContainerWorkspacePath is the standard mount point for workspaces inside agent containers.
const DefaultContainerWorkspacePath = "/workspace"

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
		workspacePath: DefaultContainerWorkspacePath,
	}
}

// WithWorkspacePath overrides the default in-container workspace path.
func (l *ContainerExecProcessLauncher) WithWorkspacePath(path string) *ContainerExecProcessLauncher {
	if path != "" {
		l.workspacePath = path
	}
	return l
}

// rewriteWorkspaceArg rewrites --workspace arguments to use the container mount path.
// This is critical for container mode: ACP receives host workspace paths (e.g. /Users/dev/workspaces/session-123)
// but these paths don't exist inside the container where the workspace is mounted at containerPath (e.g. /workspace).
//
// Handles both formats:
//   - "--workspace /host/path" → "--workspace /workspace"
//   - "--workspace=/host/path" → "--workspace=/workspace"
//
// Args without --workspace are returned unchanged.
func rewriteWorkspaceArg(args []string, containerPath string) []string {
	if len(args) == 0 {
		return args
	}

	result := make([]string, 0, len(args))
	i := 0
	for i < len(args) {
		arg := args[i]

		// Handle "--workspace=/path" format
		if strings.HasPrefix(arg, "--workspace=") {
			result = append(result, "--workspace="+containerPath)
			i++
			continue
		}

		// Handle "--workspace /path" format (two separate args)
		if arg == "--workspace" && i+1 < len(args) {
			result = append(result, "--workspace", containerPath)
			i += 2 // Skip both --workspace and the path
			continue
		}

		// Pass through all other args unchanged
		result = append(result, arg)
		i++
	}

	return result
}

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
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Use configured workspace path (set via WithWorkspacePath or defaulted in constructor)
	workspacePath := l.workspacePath

	// CRITICAL: Rewrite workspace arguments from host paths to container paths
	// ACP receives host workspace path (e.g. /Users/dev/workspaces/session-123)
	// but inside the container the workspace is mounted at a different location (e.g. /workspace)
	rewrittenArgs := rewriteWorkspaceArg(cfg.CommandArgs, workspacePath)

	command := buildExecCommand(cfg.CommandPath, rewrittenArgs)
	env := mergeEnvMaps(cfg.Env, map[string]string{
		"ANTHROPIC_API_KEY": cfg.APIKey,
	})

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
