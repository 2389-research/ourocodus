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

// CreateDockerClient attempts to connect to Docker using a three-step fallback strategy.
//
// Platform limitations:
// - Supports Unix-like systems (macOS/Linux) and Windows via DOCKER_HOST environment variable
// - Windows users should set DOCKER_HOST to npipe:////./pipe/docker_engine or tcp://localhost:2375
// - Alternatively, Windows users can run in WSL2 for native Unix socket support
//
// The function tries the following connection methods in order:
// 1. DOCKER_HOST environment variable (handles Windows named pipes, TCP, and custom Unix sockets)
// 2. ~/.colima/default/docker.sock (macOS only - Colima convenience fallback)
// 3. /var/run/docker.sock (standard Unix socket for Docker Desktop/Engine)
//
// Each attempt includes a ping verification with a 2-second timeout.
// Returns an error if all connection methods fail.
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
