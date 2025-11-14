package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
			// Origin validation (issue #215 - Phase 1)
			// Phase 1: Allow all origins for development.
			// TODO: In production, validate against allowed origins list.
			// Origin validation is deferred to Phase 2.
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

	// Track if any cleanup step fails (issue #216)
	var shutdownErrors []error

	// Phase 1: HTTP server shutdown (10s timeout)
	log.Println("[SHUTDOWN] Phase 1: Stopping HTTP server...")
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := httpServer.Shutdown(httpCtx); err != nil {
		log.Printf("[SHUTDOWN] HTTP server shutdown error: %v", err)
		shutdownErrors = append(shutdownErrors, fmt.Errorf("HTTP shutdown: %w", err))
	} else {
		log.Println("[SHUTDOWN] HTTP server stopped")
	}
	httpCancel()

	// Phase 2: Session termination (2min timeout for graceful cleanup)
	log.Println("[SHUTDOWN] Phase 2: Cleaning up active sessions...")
	activeSessions := sessionManager.List(nil) // nil filter = all sessions
	if len(activeSessions) > 0 {
		log.Printf("[SHUTDOWN] Found %d active session(s) to terminate", len(activeSessions))

		sessionsCtx, sessionsCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		successCount := 0
		failCount := 0
		for _, session := range activeSessions {
			sessionID := session.GetID()
			log.Printf("[SHUTDOWN] Terminating session: %s", sessionID)

			if _, err := sessionManager.TerminateUserSession(sessionsCtx, sessionID); err != nil {
				log.Printf("[SHUTDOWN] WARN: Failed to terminate session %s: %v", sessionID, err)
				failCount++
				shutdownErrors = append(shutdownErrors, fmt.Errorf("session %s: %w", sessionID, err))
			} else {
				successCount++
			}
		}
		sessionsCancel()

		log.Printf("[SHUTDOWN] Session cleanup complete: %d succeeded, %d failed", successCount, failCount)
	} else {
		log.Println("[SHUTDOWN] No active sessions to clean up")
	}

	// Phase 3: NATS drain (10s timeout)
	if natsClient != nil {
		log.Println("[SHUTDOWN] Phase 3: Draining NATS connection...")
		natsCtx, natsCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := natsClient.Drain(natsCtx); err != nil {
			log.Printf("[SHUTDOWN] NATS drain error: %v", err)
			shutdownErrors = append(shutdownErrors, fmt.Errorf("NATS drain: %w", err))
		} else {
			log.Println("[SHUTDOWN] NATS connection drained successfully")
		}
		natsCancel()
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

	// Verify Docker is accessible (with 5s timeout)
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if _, err := dockerClient.Ping(pingCtx); err != nil {
		log.Fatalf("Docker daemon is not accessible (timeout: 5s): %v", err)
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

// redactNATSURL removes credentials from NATS URL for safe logging
func redactNATSURL(natsURL string) string {
	// Handle comma-separated NATS URLs (HA configuration with multiple servers)
	// Example: nats://user:pass@host1:4222,nats://admin:secret@host2:4222
	if strings.Contains(natsURL, ",") {
		urls := strings.Split(natsURL, ",")
		redacted := make([]string, len(urls))
		for i, u := range urls {
			redacted[i] = redactNATSURL(strings.TrimSpace(u))
		}
		return strings.Join(redacted, ",")
	}

	// Parse the URL to extract and redact credentials
	// NATS URLs can be: nats://host:port or nats://user:pass@host:port

	// Use Go's net/url package for proper URL parsing
	// This handles edge cases like @ symbols in passwords
	parsed, err := url.Parse(natsURL)
	if err != nil {
		// If parsing fails, return safe placeholder to avoid credential leakage
		// Malformed URLs might still contain embedded credentials
		return "INVALID_NATS_URL"
	}

	// Check for opaque URIs (missing // authority separator)
	// These may contain credentials in the opaque part like "nats:user:pass@host"
	// which wouldn't be caught by the User field check
	if parsed.Opaque != "" && (len(parsed.Opaque) > 0 && (parsed.Opaque[0] != '/' || (len(parsed.Opaque) > 1 && parsed.Opaque[1] != '/'))) {
		// Opaque URI detected - if it contains @ it might have credentials
		if len(parsed.Opaque) > 0 && strings.Contains(parsed.Opaque, "@") {
			return "INVALID_NATS_URL"
		}
	}

	// If no user info, return original URL
	if parsed.User == nil {
		return natsURL
	}

	// Manually construct redacted URL to avoid URL encoding of ***
	// Format: scheme://***:***@host:port
	redacted := parsed.Scheme + "://***:***@" + parsed.Host
	if parsed.RawQuery != "" {
		redacted += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		redacted += "#" + parsed.Fragment
	}
	return redacted
}

// initializeNATS creates a NATS client if NATS_URL is configured
func initializeNATS() nats.Client {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Printf("[NATS] NATS_URL not set, event publishing disabled")
		return nil
	}

	log.Printf("[NATS] Connecting to NATS at %s...", redactNATSURL(natsURL))

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

	// List containers with 10s timeout
	listCtx, listCancel := context.WithTimeout(ctx, 10*time.Second)
	defer listCancel()

	containers, err := cli.ContainerList(listCtx, dockercontainer.ListOptions{
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

		// Stop container with 30s timeout (allows graceful shutdown)
		stopCtx, stopCancel := context.WithTimeout(ctx, 30*time.Second)
		timeout := 10
		if err := cli.ContainerStop(stopCtx, cont.ID, dockercontainer.StopOptions{Timeout: &timeout}); err != nil {
			log.Printf("[CLEANUP] WARN: Failed to stop orphaned container %s: %v", shortID, err)
		}
		stopCancel()

		// Remove container with 10s timeout
		removeCtx, removeCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := cli.ContainerRemove(removeCtx, cont.ID, dockercontainer.RemoveOptions{Force: true}); err != nil {
			log.Printf("[CLEANUP] WARN: Failed to remove orphaned container %s: %v", shortID, err)
		} else {
			log.Printf("[CLEANUP] Cleaned up orphaned container: %s", shortID)
		}
		removeCancel()
	}

	return nil
}
