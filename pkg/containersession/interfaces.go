package containersession

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// DockerClient abstracts Docker SDK operations for testability
type DockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config,
		hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig,
		platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string,
		options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string,
		options container.StopOptions) error
	ContainerAttach(ctx context.Context, containerID string,
		options container.AttachOptions) (types.HijackedResponse, error)
	ContainerExecCreate(ctx context.Context, containerID string,
		config container.ExecOptions) (container.ExecCreateResponse, error)
	ContainerExecAttach(ctx context.Context, execID string,
		config container.ExecAttachOptions) (types.HijackedResponse, error)
	ContainerKill(ctx context.Context, containerID, signal string) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerRemove(ctx context.Context, containerID string,
		options container.RemoveOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
}

// IDGenerator abstracts unique ID generation
type IDGenerator interface {
	Generate() string
}

// Clock abstracts time operations for deterministic testing
type Clock interface {
	Now() time.Time
}

// Logger abstracts logging operations
type Logger interface {
	Printf(format string, v ...interface{})
}

// LogLevel defines verbosity levels for logging
type LogLevel int

const (
	// LogLevelError logs only errors
	LogLevelError LogLevel = iota
	// LogLevelInfo logs errors and informational messages (default)
	LogLevelInfo
	// LogLevelDebug logs errors, info, and debug messages (verbose)
	LogLevelDebug
)

// LeveledLogger wraps a Logger with level-aware logging
type LeveledLogger struct {
	logger Logger
	level  LogLevel
}

// NewLeveledLogger creates a level-aware logger wrapper
func NewLeveledLogger(logger Logger, level LogLevel) *LeveledLogger {
	return &LeveledLogger{
		logger: logger,
		level:  level,
	}
}

// Printf implements Logger interface (acts as Info)
func (l *LeveledLogger) Printf(format string, v ...interface{}) {
	l.Info(format, v...)
}

// Error logs error messages (always logged)
func (l *LeveledLogger) Error(format string, v ...interface{}) {
	l.logger.Printf("[ERROR] "+format, v...)
}

// Info logs informational messages (logged at Info and Debug levels)
func (l *LeveledLogger) Info(format string, v ...interface{}) {
	if l.level >= LogLevelInfo {
		l.logger.Printf("[INFO] "+format, v...)
	}
}

// Debug logs debug messages (only logged at Debug level)
func (l *LeveledLogger) Debug(format string, v ...interface{}) {
	if l.level >= LogLevelDebug {
		l.logger.Printf("[DEBUG] "+format, v...)
	}
}
