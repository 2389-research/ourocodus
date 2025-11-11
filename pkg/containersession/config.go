package containersession

import (
	"github.com/docker/docker/api/types/mount"
)

// CreateConfig provides configuration for creating a container session with custom options.
// This allows callers to specify custom mounts, environment variables, and labels
// beyond the defaults provided by CreateContainerSession.
type CreateConfig struct {
	// ImageName is the Docker image to use (required)
	ImageName string

	// Command is the command to run in the container (required)
	Command []string

	// Entrypoint overrides the Docker image ENTRYPOINT (optional)
	// If nil, uses the image's ENTRYPOINT. If empty slice, clears the ENTRYPOINT.
	Entrypoint []string

	// WorkspaceDir is the workspace directory path. If empty, a default workspace
	// will be created in baseWorkspaceDir/sessionID and mounted at /workspace
	WorkspaceDir string

	// CustomMounts are additional mounts to add to the container beyond the workspace mount.
	// The workspace mount (/workspace) is always added automatically using either the
	// provided WorkspaceDir or a default workspace path.
	CustomMounts []mount.Mount

	// Env are environment variables to set in the container (optional)
	Env []string

	// Labels are additional labels to add to the container beyond the default session labels (optional)
	Labels map[string]string

	// SkipOutputLogging skips attaching to stdout/stderr for logging (optional, default false)
	// Set to true when using external stdio attachment (e.g., ACP container attach mode)
	// to avoid competing for the same streams.
	SkipOutputLogging bool
}
