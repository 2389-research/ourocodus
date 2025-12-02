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

	relaytui "github.com/2389-research/ourocodus/cmd/relay/internal/tui"
	"github.com/2389-research/ourocodus/internal/webapp"
	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/nats"
	"github.com/2389-research/ourocodus/pkg/relay"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/2389-research/ourocodus/pkg/worktree"
	tea "github.com/charmbracelet/bubbletea"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

const (
	port            = 8080
	shutdownTimeout = 10 * time.Second

	// maxConcurrentWSUpgrades limits simultaneous WebSocket upgrade requests
	// to prevent connection exhaustion attacks
	maxConcurrentWSUpgrades = 100
)

// wsSemaphore limits concurrent WebSocket upgrade requests to prevent DoS
var wsSemaphore = make(chan struct{}, maxConcurrentWSUpgrades)

// tuiProgram is the global Bubble Tea program for the relay TUI
var tuiProgram *tea.Program

// tuiLogWriter is the log writer that sends logs to the TUI
var tuiLogWriter *relaytui.LogWriter

// Version is set by ldflags at build time
var Version = "dev"

// headlessFlag is set via --headless flag
var headlessFlag bool

var rootCmd = &cobra.Command{
	Use:   "relay",
	Short: "🔄 relay - Ourocodus WebSocket relay and PWA server",
	Long: `relay is the WebSocket relay server for Ourocodus.

It provides:
  🔄 WebSocket API for PWA↔Agent communication
  🌐 Static file hosting for the PWA
  📦 Container session management
  💓 Agent heartbeat monitoring via NATS

Environment variables:
  RELAY_NO_TUI=1        Run in headless mode (no TUI)
  NATS_URL              NATS server URL (default: nats://localhost:4222)
  WORKSPACE_DIR         Agent workspace directory (default: ./workspaces)
  REPO_PATH             Repository path (default: .)
  AGENT_IMAGE           Docker image for agents (default: ourocodus/agent:latest)
  AGENT_CPU_CORES       CPU cores per agent (default: 2)
  AGENT_MEMORY_MB       Memory per agent in MB (default: 4096)`,
	Version: Version,
	RunE:    runRelay,
}

func init() {
	rootCmd.Flags().BoolVar(&headlessFlag, "headless", false, "Run without TUI (same as RELAY_NO_TUI=1)")
}

func main() {
	app := cli.NewApp(rootCmd)
	os.Exit(app.Execute())
}

// runRelay is the main entry point for the relay server
func runRelay(cmd *cobra.Command, _ []string) error {
	// Check for headless mode (flag or environment)
	headlessMode := headlessFlag || os.Getenv("RELAY_NO_TUI") != ""

	if !headlessMode {
		// Create log writer for TUI (program set later)
		tuiLogWriter = relaytui.NewLogWriter(nil)
		log.SetOutput(tuiLogWriter)
	} else {
		// In headless mode, log to stderr for debugging
		log.SetOutput(os.Stderr)
	}
	log.SetFlags(log.Ldate | log.Ltime)

	// Create dependencies
	logger := &relay.StdLogger{}
	clock := &relay.SystemClock{}
	idGen := &relay.UUIDGenerator{}

	ctx := cmd.Context()

	// Initialize Docker and agent dependencies
	dockerClient, launcherFactory, containerManager := initializeAgentInfrastructure(ctx, logger, clock, idGen)
	defer func() { _ = dockerClient.Close() }()

	// Default ACP runtime to container when available so web-spawned agents appear in Docker-backed tools (agentd list)
	if os.Getenv("OUROCODUS_ACP_RUNTIME") == "" && containerManager != nil {
		_ = os.Setenv("OUROCODUS_ACP_RUNTIME", "container")
		logger.Printf("[INIT] OUROCODUS_ACP_RUNTIME not set; defaulting to container for ACP processes")
	}
	if containerManager != nil {
		containerManager.SetStopTimeout(5) // faster shutdown for agent containers
	}

	// Initialize NATS client if configured
	natsClient := initializeNATS()

	// Create session manager with launcher factory and container manager
	sessionManager, err := relay.NewSessionManager(logger, clock, idGen, natsClient, launcherFactory, containerManager)
	if err != nil {
		return cli.IOError(fmt.Sprintf("failed to create session manager: %v", err))
	}

	// Cleanup expired leases from previous runs (issue #247)
	// This prevents orphaned lease files from blocking agent attachments
	if err := cleanupExpiredLeases(logger); err != nil {
		log.Printf("[CLEANUP] WARN: Failed to cleanup expired leases: %v", err)
	}

	// Create relay server with dependency injection
	server := relay.NewServer(
		idGen,
		logger,
		clock,
		relay.NewGorillaUpgrader(createOriginChecker(logger)),
		sessionManager,
	)

	// Initialize HeartbeatMonitor if NATS is available
	// This enables detection of externally killed agents and PWA notification
	var heartbeatMonitor *session.HeartbeatMonitor
	var heartbeatCancel context.CancelFunc
	if natsClient != nil {
		heartbeatMonitor, _, heartbeatCancel = initializeHeartbeatMonitor(ctx, server, logger)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Rate-limited WebSocket endpoint to prevent connection exhaustion
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		select {
		case wsSemaphore <- struct{}{}:
			defer func() { <-wsSemaphore }()
			server.HandleWebSocket(w, r)
		default:
			logger.Printf("[RATELIMIT] WebSocket upgrade rejected: too many concurrent connections")
			http.Error(w, "Too many connections", http.StatusTooManyRequests)
		}
	})

	// Serve PWA static files from embedded filesystem
	webFS, err := webapp.GetFS()
	if err != nil {
		return cli.IOError(fmt.Sprintf("failed to create web filesystem: %v", err))
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	// Start server in goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[SERVER] Error: %v", err)
		}
	}()

	// Handle shutdown signals (common to both modes)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Construct shutdown context for cleanup functions
	shutdownCtx := shutdownContext{
		httpServer:       httpServer,
		sessionManager:   sessionManager,
		natsClient:       natsClient,
		heartbeatMonitor: heartbeatMonitor,
		heartbeatCancel:  heartbeatCancel,
	}

	if headlessMode {
		runHeadlessMode(sigChan, shutdownCtx)
	} else {
		runTUIMode(sigChan, shutdownCtx, sessionManager, natsClient)
	}

	return nil
}

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

// shutdownContext bundles all resources that need cleanup during shutdown
type shutdownContext struct {
	httpServer       *http.Server
	sessionManager   relay.SessionManagerInterface
	natsClient       nats.Client
	heartbeatMonitor *session.HeartbeatMonitor
	heartbeatCancel  context.CancelFunc
}

// runHeadlessMode runs the server without TUI, logging to stderr
func runHeadlessMode(sigChan <-chan os.Signal, ctx shutdownContext) {
	log.Printf("[SERVER] Relay server running in headless mode on port %d", port)
	log.Printf("[SERVER] PWA: http://localhost:%d/", port)
	log.Printf("[SERVER] WebSocket: ws://localhost:%d/ws", port)

	<-sigChan
	log.Println("[SHUTDOWN] Received shutdown signal")

	stopHeartbeatMonitor(ctx.heartbeatMonitor, ctx.heartbeatCancel)
	executeGracefulShutdown(ctx)
}

// runTUIMode runs the server with Bubble Tea TUI
func runTUIMode(sigChan <-chan os.Signal, ctx shutdownContext, sessionManager relay.SessionManagerInterface, natsClient nats.Client) {
	tuiModel := relaytui.New(relaytui.Config{
		Port:           port,
		NATSConnected:  natsClient != nil,
		DockerOK:       true, // we got this far, Docker is OK
		SessionManager: sessionManager,
	})

	tuiProgram = tea.NewProgram(tuiModel, tea.WithAltScreen())
	tuiLogWriter.SetProgram(tuiProgram)

	// Handle shutdown signals in TUI mode
	go func() {
		<-sigChan

		// Signal TUI to show shutdown state
		tuiProgram.Send(relaytui.ShutdownMsg{})

		stopHeartbeatMonitor(ctx.heartbeatMonitor, ctx.heartbeatCancel)
		executeGracefulShutdown(ctx)

		// Quit TUI
		tuiProgram.Quit()
	}()

	// Run TUI (blocks until quit)
	if _, err := tuiProgram.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

// stopHeartbeatMonitor stops the heartbeat monitor if it's running
func stopHeartbeatMonitor(monitor *session.HeartbeatMonitor, cancel context.CancelFunc) {
	if monitor == nil {
		return
	}
	log.Println("[SHUTDOWN] Stopping HeartbeatMonitor...")
	cancel()
	monitor.Stop()
	log.Println("[SHUTDOWN] HeartbeatMonitor stopped")
}

// executeGracefulShutdown performs the graceful shutdown sequence
func executeGracefulShutdown(ctx shutdownContext) {
	if err := gracefulShutdown(ctx.httpServer, ctx.sessionManager, ctx.natsClient); err != nil {
		log.Printf("[SHUTDOWN] Server stopped with errors: %v", err)
	} else {
		log.Println("[SHUTDOWN] Server stopped successfully")
	}
}

// gracefulShutdown performs a phased shutdown of all server components.
// Returns an error if any phase fails, but continues through all phases.
func gracefulShutdown(httpServer *http.Server, sessionManager relay.SessionManagerInterface, natsClient nats.Client) error {
	var shutdownErrors []error

	// Phase 1: HTTP server shutdown (10s timeout)
	if errs := shutdownHTTPServer(httpServer); len(errs) > 0 {
		shutdownErrors = append(shutdownErrors, errs...)
	}

	// Phase 2: Session termination (2min timeout for graceful cleanup)
	if errs := shutdownSessions(sessionManager); len(errs) > 0 {
		shutdownErrors = append(shutdownErrors, errs...)
	}

	// Phase 3: NATS drain (10s timeout)
	if errs := shutdownNATS(natsClient); len(errs) > 0 {
		shutdownErrors = append(shutdownErrors, errs...)
	}

	if len(shutdownErrors) > 0 {
		return fmt.Errorf("%d shutdown error(s): %v", len(shutdownErrors), shutdownErrors)
	}
	return nil
}

// shutdownHTTPServer gracefully shuts down the HTTP server with a 10s timeout.
func shutdownHTTPServer(httpServer *http.Server) []error {
	log.Println("[SHUTDOWN] Phase 1: Stopping HTTP server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[SHUTDOWN] HTTP server shutdown error: %v", err)
		return []error{fmt.Errorf("HTTP shutdown: %w", err)}
	}
	log.Println("[SHUTDOWN] HTTP server stopped")
	return nil
}

// shutdownSessions terminates all active sessions with a 2min timeout.
func shutdownSessions(sessionManager relay.SessionManagerInterface) []error {
	log.Println("[SHUTDOWN] Phase 2: Cleaning up active sessions...")
	activeSessions := sessionManager.List(nil) // nil filter = all sessions

	if len(activeSessions) == 0 {
		log.Println("[SHUTDOWN] No active sessions to clean up")
		return nil
	}

	log.Printf("[SHUTDOWN] Found %d active session(s) to terminate", len(activeSessions))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var errors []error
	successCount := 0
	for _, session := range activeSessions {
		sessionID := session.GetID()
		log.Printf("[SHUTDOWN] Terminating session: %s", sessionID)

		if _, err := sessionManager.TerminateUserSession(ctx, sessionID); err != nil {
			log.Printf("[SHUTDOWN] WARN: Failed to terminate session %s: %v", sessionID, err)
			errors = append(errors, fmt.Errorf("session %s: %w", sessionID, err))
		} else {
			successCount++
		}
	}

	log.Printf("[SHUTDOWN] Session cleanup complete: %d succeeded, %d failed", successCount, len(errors))
	return errors
}

// shutdownNATS drains the NATS connection with a 10s timeout.
func shutdownNATS(natsClient nats.Client) []error {
	if natsClient == nil {
		return nil
	}

	log.Println("[SHUTDOWN] Phase 3: Draining NATS connection...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := natsClient.Drain(ctx); err != nil {
		log.Printf("[SHUTDOWN] NATS drain error: %v", err)
		return []error{fmt.Errorf("NATS drain: %w", err)}
	}
	log.Println("[SHUTDOWN] NATS connection drained successfully")
	return nil
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

// initializeNATS creates a NATS client, defaulting to localhost:4222.
// Returns nil if connection fails (relay continues without NATS).
func initializeNATS() nats.Client {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	log.Printf("[NATS] Connecting to %s...", redactNATSURL(natsURL))

	natsClient, err := nats.NewClient(
		nats.WithURL(natsURL),
		nats.WithName("ourocodus-relay"),
	)
	if err != nil {
		log.Printf("[NATS] Connection failed: %v (event publishing disabled)", err)
		return nil
	}
	log.Printf("[NATS] Connected successfully")

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

// createOriginChecker returns an origin validation function for WebSocket upgrades.
// By default, allows all origins (development mode). Set RELAY_ALLOWED_ORIGINS
// environment variable to a comma-separated list of allowed origins for production.
// Use "*" to explicitly allow all origins.
func createOriginChecker(logger *relay.StdLogger) func(r *http.Request) bool {
	allowedOriginsEnv := os.Getenv("RELAY_ALLOWED_ORIGINS")

	// Development mode: allow all origins if not configured
	if allowedOriginsEnv == "" {
		logger.Printf("[SECURITY] RELAY_ALLOWED_ORIGINS not set; allowing all origins (development mode)")
		return func(r *http.Request) bool {
			return true
		}
	}

	// Parse allowed origins
	var allowedOrigins []string
	for _, origin := range strings.Split(allowedOriginsEnv, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins = append(allowedOrigins, origin)
		}
	}

	// Check for wildcard
	for _, origin := range allowedOrigins {
		if origin == "*" {
			logger.Printf("[SECURITY] Origin validation: allowing all origins (wildcard)")
			return func(r *http.Request) bool {
				return true
			}
		}
	}

	logger.Printf("[SECURITY] Origin validation: allowed origins = %v", allowedOrigins)

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// No origin header (same-origin request or non-browser client)
		if origin == "" {
			return true
		}

		// Check against allowlist (case-insensitive)
		for _, allowed := range allowedOrigins {
			if strings.EqualFold(allowed, origin) {
				return true
			}
		}

		logger.Printf("[SECURITY] Rejected WebSocket connection from origin: %s", origin)
		return false
	}
}

// cleanupExpiredLeases removes expired lease files on startup
// Sets up logging and calls session.CleanupExpiredLeases
func cleanupExpiredLeases(logger *relay.StdLogger) error {
	// Set up lease logging using the relay logger
	session.DefaultLogger = logger

	cleaned, err := session.CleanupExpiredLeases()
	if err != nil {
		return fmt.Errorf("failed to cleanup expired leases: %w", err)
	}

	if cleaned > 0 {
		log.Printf("[CLEANUP] Cleaned up %d expired lease(s)", cleaned)
	} else {
		log.Println("[CLEANUP] No expired leases found")
	}

	return nil
}

// initializeHeartbeatMonitor creates and starts a HeartbeatMonitor that detects
// externally killed agents (via agentd stop, docker stop, or crash) and notifies
// the PWA via the relay server.
//
// Returns the monitor, context, and cancel function. The cancel function should be
// called during shutdown to stop the reaper goroutine.
func initializeHeartbeatMonitor(ctx context.Context, server *relay.Server, _ *relay.StdLogger) (*session.HeartbeatMonitor, context.Context, context.CancelFunc) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	monitor, err := session.NewHeartbeatMonitor(natsURL)
	if err != nil {
		log.Printf("[HEARTBEAT] Failed to create HeartbeatMonitor: %v (agent death detection disabled)", err)
		return nil, nil, func() {}
	}

	// Set callback to notify PWA when agents die
	monitor.SetOnAgentDeath(func(agentID, userSessionID string) {
		server.NotifyAgentDeath(agentID, userSessionID)
	})

	// Create cancellable context for reaper goroutine
	monitorCtx, monitorCancel := context.WithCancel(ctx)

	// Start monitoring
	if err := monitor.Start(monitorCtx); err != nil {
		log.Printf("[HEARTBEAT] Failed to start HeartbeatMonitor: %v (agent death detection disabled)", err)
		monitorCancel()
		monitor.Stop()
		return nil, nil, func() {}
	}

	log.Println("[HEARTBEAT] HeartbeatMonitor started (agent death detection enabled)")
	return monitor, monitorCtx, monitorCancel
}
