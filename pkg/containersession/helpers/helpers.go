package helpers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// UUIDGenerator generates unique IDs using Google's UUID library.
// Implements containersession.IDGenerator interface.
type UUIDGenerator struct{}

// Generate returns a new UUID v4 string.
func (g *UUIDGenerator) Generate() string {
	return uuid.New().String()
}

// SystemClock provides real time.
// Implements containersession.Clock interface.
type SystemClock struct{}

// Now returns the current time.
func (c *SystemClock) Now() time.Time {
	return time.Now()
}

// StdLogger wraps standard logger to implement containersession.Logger interface.
type StdLogger struct {
	*log.Logger
}

// Printf logs a formatted message using the underlying logger.
func (l *StdLogger) Printf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// CreateDockerClient attempts to connect to Docker, trying Colima first, then Docker Desktop.
//
// Platform limitations:
// - This function assumes a Unix-like environment (macOS/Linux)
// - On Windows, Docker Desktop uses named pipes (npipe:////./pipe/docker_engine)
// - Windows support requires detecting OS and using appropriate connection string
//
// The function tries two common Docker socket locations:
// 1. ~/.colima/default/docker.sock (Colima on macOS)
// 2. /var/run/docker.sock (Docker Desktop on macOS/Linux)
//
// Returns an error if neither location is accessible.
func CreateDockerClient(ctx context.Context) (*client.Client, error) {
	// Try 1: Environment variables (DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH)
	// This handles Windows npipe, TCP, and explicit Unix socket configurations
	if dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	); err == nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if _, err := dockerClient.Ping(pingCtx); err == nil {
			return dockerClient, nil
		}
		_ = dockerClient.Close()
	}

	// Try 2: macOS Colima fallback (preserves existing convenience)
	if runtime.GOOS == "darwin" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			colimaSocket := filepath.Join(homeDir, ".colima", "default", "docker.sock")
			if _, err := os.Stat(colimaSocket); err == nil {
				colimaHost := "unix://" + colimaSocket
				if dockerClient, err := client.NewClientWithOpts(
					client.WithHost(colimaHost),
					client.WithAPIVersionNegotiation(),
				); err == nil {
					pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()
					if _, err := dockerClient.Ping(pingCtx); err == nil {
						return dockerClient, nil
					}
					_ = dockerClient.Close()
				}
			}
		}
	}

	// Try 3: Standard Unix socket fallback
	dockerHost := "unix:///var/run/docker.sock"
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		if dockerClient, err := client.NewClientWithOpts(
			client.WithHost(dockerHost),
			client.WithAPIVersionNegotiation(),
		); err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if _, err := dockerClient.Ping(pingCtx); err == nil {
				return dockerClient, nil
			}
			_ = dockerClient.Close()
		}
	}

	return nil, fmt.Errorf("cannot connect to Docker: tried DOCKER_HOST env, Colima (macOS), and /var/run/docker.sock")
}
