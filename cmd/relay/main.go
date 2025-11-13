package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/2389-research/ourocodus/internal/webapp"
	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/nats"
	"github.com/2389-research/ourocodus/pkg/relay"
	"github.com/2389-research/ourocodus/pkg/worktree"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const (
	port            = 8080
	shutdownTimeout = 10 * time.Second
)

// clockAdapter adapts relay.Clock to containersession.Clock
type clockAdapter struct {
	clock *relay.SystemClock
}

func (c *clockAdapter) Now() time.Time {
	// relay.SystemClock.Now() returns string (RFC3339), but containersession.Clock.Now() needs time.Time
	// Both ultimately call time.Now(), so we call it directly here
	return time.Now()
}

// idGenAdapter adapts relay.IDGenerator to containersession.IDGenerator
type idGenAdapter struct {
	idGen *relay.UUIDGenerator
}

func (i *idGenAdapter) Generate() string {
	return i.idGen.Generate()
}

// loggerAdapter adapts relay.Logger to containersession.Logger
type loggerAdapter struct {
	logger *relay.StdLogger
}

func (l *loggerAdapter) Printf(format string, v ...interface{}) {
	l.logger.Printf(format, v...)
}

func main() {
	// Create dependencies
	logger := &relay.StdLogger{}
	clock := &relay.SystemClock{}
	idGen := &relay.UUIDGenerator{}

	ctx := context.Background()

	// Initialize Docker and agent dependencies
	dockerClient, launcherFactory, containerManager := initializeAgentInfrastructure(ctx, logger, clock, idGen)
	defer func() { _ = dockerClient.Close() }()

	// Initialize NATS client if configured
	natsClient := initializeNATS()

	// Create session manager with launcher factory and container manager
	sessionManager, err := relay.NewSessionManager(logger, clock, idGen, natsClient, launcherFactory, containerManager)
	if err != nil {
		log.Fatalf("Failed to create session manager: %v", err)
	}

	// Create relay server with dependency injection
	server := relay.NewServer(
		idGen,
		logger,
		clock,
		relay.NewGorillaUpgrader(func(r *http.Request) bool {
			// Allow all origins for development (Phase 1)
			return true
		}),
		sessionManager,
	)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.HandleWebSocket)

	// Serve PWA static files from embedded filesystem
	webFS, err := webapp.GetFS()
	if err != nil {
		log.Fatalf("Failed to create web filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	// Start server in goroutine
	go func() {
		log.Printf("[SERVER] Relay server starting on port %d", port)
		log.Printf("[SERVER] PWA available at: http://localhost:%d/", port)
		log.Printf("[SERVER] WebSocket endpoint: ws://localhost:%d/ws", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[SHUTDOWN] Signal received, gracefully stopping server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Track if any cleanup step fails (issue #216)
	var shutdownErrors []error

	// Attempt graceful HTTP server shutdown
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[SHUTDOWN] Server shutdown error: %v", err)
		shutdownErrors = append(shutdownErrors, fmt.Errorf("HTTP shutdown: %w", err))
	} else {
		log.Println("[SHUTDOWN] HTTP server stopped")
	}

	// Cleanup all active sessions (agents, containers, worktrees, credentials)
	log.Println("[SHUTDOWN] Cleaning up active sessions...")
	activeSessions := sessionManager.List(nil) // nil filter = all sessions
	if len(activeSessions) > 0 {
		log.Printf("[SHUTDOWN] Found %d active session(s) to terminate", len(activeSessions))

		successCount := 0
		failCount := 0
		for _, session := range activeSessions {
			sessionID := session.GetID()
			log.Printf("[SHUTDOWN] Terminating session: %s", sessionID)

			if _, err := sessionManager.TerminateUserSession(shutdownCtx, sessionID); err != nil {
				log.Printf("[SHUTDOWN] WARN: Failed to terminate session %s: %v", sessionID, err)
				failCount++
				shutdownErrors = append(shutdownErrors, fmt.Errorf("session %s: %w", sessionID, err))
			} else {
				successCount++
			}
		}

		log.Printf("[SHUTDOWN] Session cleanup complete: %d succeeded, %d failed", successCount, failCount)
	} else {
		log.Println("[SHUTDOWN] No active sessions to clean up")
	}

	// Drain NATS connection if available
	if natsClient != nil {
		log.Println("[SHUTDOWN] Draining NATS connection...")
		if err := natsClient.Drain(shutdownCtx); err != nil {
			log.Printf("[SHUTDOWN] NATS drain error: %v", err)
			shutdownErrors = append(shutdownErrors, fmt.Errorf("NATS drain: %w", err))
		} else {
			log.Println("[SHUTDOWN] NATS connection drained successfully")
		}
	}

	// Docker client will be closed by defer at line 71

	// Exit with error if any cleanup step failed (issue #216)
	if len(shutdownErrors) > 0 {
		log.Printf("[SHUTDOWN] Server stopped with %d error(s)", len(shutdownErrors))
		os.Exit(1)
	}

	log.Println("[SHUTDOWN] Server stopped successfully")
}

// initializeAgentInfrastructure sets up Docker, worktree, credentials, and launcher factory
func initializeAgentInfrastructure(ctx context.Context, logger *relay.StdLogger, clock *relay.SystemClock, idGen *relay.UUIDGenerator) (*client.Client, agent.LauncherFactory, *containersession.Manager) {
	// Get configuration from environment
	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		workspaceDir = "./workspaces"
	}

	// Convert workspaceDir to absolute path (Docker requires absolute paths for bind mounts)
	absWorkspaceDir, err := filepath.Abs(workspaceDir)
	if err != nil {
		log.Fatalf("Failed to resolve absolute workspace directory: %v", err)
	}
	workspaceDir = absWorkspaceDir

	repoPath := os.Getenv("REPO_PATH")
	if repoPath == "" {
		repoPath = "."
	}

	agentImage := os.Getenv("AGENT_IMAGE")
	if agentImage == "" {
		agentImage = "ourocodus/agent:latest"
	}

	// Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	// Verify Docker is accessible
	if _, err := dockerClient.Ping(ctx); err != nil {
		log.Fatalf("Docker daemon is not accessible: %v", err)
	}
	log.Printf("[INIT] Docker client initialized successfully")

	// Initialize worktree manager
	worktreeManager, err := worktree.NewAgentWorktreeManager(repoPath)
	if err != nil {
		log.Fatalf("Failed to create worktree manager: %v", err)
	}
	log.Printf("[INIT] Worktree manager initialized")

	// Initialize credential mounter
	credsDir := fmt.Sprintf("%s/credentials", workspaceDir)
	if err := os.MkdirAll(credsDir, 0o700); err != nil {
		log.Fatalf("Failed to create credentials directory: %v", err)
	}
	credMounter := container.NewAgentCredentialMounter(credsDir)
	log.Printf("[INIT] Credential mounter initialized")

	// Initialize container session manager (with adapters)
	containerManager := containersession.NewManager(
		dockerClient,
		&idGenAdapter{idGen: idGen},
		&clockAdapter{clock: clock},
		&loggerAdapter{logger: logger},
		workspaceDir,
	)
	log.Printf("[INIT] Container session manager initialized")

	// Get resource limits from environment or use defaults
	cpuCores := int64(2)
	if val := os.Getenv("AGENT_CPU_CORES"); val != "" {
		if cores, err := strconv.ParseInt(val, 10, 64); err == nil && cores > 0 {
			cpuCores = cores
		}
	}

	memoryMB := int64(4096)
	if val := os.Getenv("AGENT_MEMORY_MB"); val != "" {
		if mem, err := strconv.ParseInt(val, 10, 64); err == nil && mem > 0 {
			memoryMB = mem
		}
	}

	// Create launcher factory
	factoryConfig := agent.LauncherFactoryConfig{
		DockerClient:     dockerClient,
		WorktreeManager:  worktreeManager,
		CredMounter:      credMounter,
		ContainerManager: containerManager,
		BaseWorkspaceDir: workspaceDir,
		DefaultImageName: agentImage,
		DefaultResourceLimits: agent.ResourceLimits{
			CPUCores: cpuCores,
			MemoryMB: memoryMB,
		},
	}
	launcherFactory := agent.NewDefaultLauncherFactory(factoryConfig)
	log.Printf("[INIT] Launcher factory initialized")

	// Cleanup orphaned containers on startup
	if err := cleanupOrphanedContainers(ctx, dockerClient); err != nil {
		log.Printf("[CLEANUP] WARN: Failed to cleanup orphaned containers: %v", err)
	}

	return dockerClient, launcherFactory, containerManager
}

// initializeNATS creates a NATS client if NATS_URL is configured
func initializeNATS() nats.Client {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Printf("[NATS] NATS_URL not set, event publishing disabled")
		return nil
	}

	log.Printf("[NATS] Connecting to NATS at %s...", natsURL)

	natsClient, err := nats.NewClient(
		nats.WithURL(natsURL),
		nats.WithName("ourocodus-relay"),
	)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	log.Printf("[NATS] Connected to NATS successfully")

	return natsClient
}

// cleanupOrphanedContainers removes orphaned agent containers from previous runs
func cleanupOrphanedContainers(ctx context.Context, cli *client.Client) error {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "ourocodus.agent=true")

	containers, err := cli.ContainerList(ctx, dockercontainer.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		log.Println("[CLEANUP] No orphaned containers found")
		return nil
	}

	log.Printf("[CLEANUP] Found %d orphaned container(s), cleaning up...", len(containers))

	for _, cont := range containers {
		// Safe truncation for logging
		shortID := cont.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		// Check container age (Created is Unix timestamp in seconds)
		created := time.Unix(cont.Created, 0)
		age := time.Since(created)

		if age < 1*time.Hour {
			log.Printf("[CLEANUP] Skipping recent container %s (age: %v)", shortID, age)
			continue
		}

		// Stop and remove
		timeout := 10
		if err := cli.ContainerStop(ctx, cont.ID, dockercontainer.StopOptions{Timeout: &timeout}); err != nil {
			log.Printf("[CLEANUP] WARN: Failed to stop orphaned container %s: %v", shortID, err)
		}

		if err := cli.ContainerRemove(ctx, cont.ID, dockercontainer.RemoveOptions{Force: true}); err != nil {
			log.Printf("[CLEANUP] WARN: Failed to remove orphaned container %s: %v", shortID, err)
		} else {
			log.Printf("[CLEANUP] Cleaned up orphaned container: %s", shortID)
		}
	}

	return nil
}
