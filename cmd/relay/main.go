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
	dockerClient, launcherFactory := initializeAgentInfrastructure(ctx, logger, clock, idGen)
	defer func() { _ = dockerClient.Close() }()

	// Initialize NATS client if configured
	natsClient := initializeNATS()

	// Create session manager with launcher factory
	sessionManager, err := relay.NewSessionManager(logger, clock, idGen, natsClient, launcherFactory)
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
		log.Printf("Relay server starting on port %d", port)
		log.Printf("PWA available at: http://localhost:%d/", port)
		log.Printf("WebSocket endpoint: ws://localhost:%d/ws", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutdown signal received, gracefully stopping server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Attempt graceful HTTP server shutdown
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		os.Exit(1)
	}

	// Drain NATS connection if available
	if natsClient != nil {
		log.Println("Draining NATS connection...")
		if err := natsClient.Drain(shutdownCtx); err != nil {
			log.Printf("NATS drain error: %v", err)
		} else {
			log.Println("NATS connection drained successfully")
		}
	}

	log.Println("Server stopped")
}

// initializeAgentInfrastructure sets up Docker, worktree, credentials, and launcher factory
func initializeAgentInfrastructure(ctx context.Context, logger *relay.StdLogger, clock *relay.SystemClock, idGen *relay.UUIDGenerator) (*client.Client, agent.LauncherFactory) {
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
	log.Printf("Docker client initialized successfully")

	// Initialize worktree manager
	worktreeManager, err := worktree.NewAgentWorktreeManager(repoPath)
	if err != nil {
		log.Fatalf("Failed to create worktree manager: %v", err)
	}
	log.Printf("Worktree manager initialized")

	// Initialize credential mounter
	credsDir := fmt.Sprintf("%s/credentials", workspaceDir)
	if err := os.MkdirAll(credsDir, 0o700); err != nil {
		log.Fatalf("Failed to create credentials directory: %v", err)
	}
	credMounter := container.NewAgentCredentialMounter(credsDir)
	log.Printf("Credential mounter initialized")

	// Initialize container session manager (with adapters)
	containerManager := containersession.NewManager(
		dockerClient,
		&idGenAdapter{idGen: idGen},
		&clockAdapter{clock: clock},
		&loggerAdapter{logger: logger},
		workspaceDir,
	)
	log.Printf("Container session manager initialized")

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
	log.Printf("Launcher factory initialized")

	// Cleanup orphaned containers on startup
	if err := cleanupOrphanedContainers(ctx, dockerClient); err != nil {
		log.Printf("WARN: Failed to cleanup orphaned containers: %v", err)
	}

	return dockerClient, launcherFactory
}

// initializeNATS creates a NATS client if NATS_URL is configured
func initializeNATS() nats.Client {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Printf("NATS_URL not set, event publishing disabled")
		return nil
	}

	log.Printf("Connecting to NATS at %s...", natsURL)

	natsClient, err := nats.NewClient(
		nats.WithURL(natsURL),
		nats.WithName("ourocodus-relay"),
	)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	log.Printf("Connected to NATS successfully")

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
		log.Println("No orphaned containers found")
		return nil
	}

	log.Printf("Found %d orphaned container(s), cleaning up...", len(containers))

	for _, cont := range containers {
		// Check container age (Created is Unix timestamp in seconds)
		created := time.Unix(cont.Created, 0)
		age := time.Since(created)

		if age < 1*time.Hour {
			log.Printf("Skipping recent container %s (age: %v)", cont.ID[:12], age)
			continue
		}

		// Stop and remove
		timeout := 10
		if err := cli.ContainerStop(ctx, cont.ID, dockercontainer.StopOptions{Timeout: &timeout}); err != nil {
			log.Printf("WARN: Failed to stop orphaned container %s: %v", cont.ID[:12], err)
		}

		if err := cli.ContainerRemove(ctx, cont.ID, dockercontainer.RemoveOptions{Force: true}); err != nil {
			log.Printf("WARN: Failed to remove orphaned container %s: %v", cont.ID[:12], err)
		} else {
			log.Printf("Cleaned up orphaned container: %s", cont.ID[:12])
		}
	}

	return nil
}
