package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/2389-research/ourocodus/internal/webapp"
	"github.com/2389-research/ourocodus/pkg/agent"
	"github.com/2389-research/ourocodus/pkg/agent/container"
	"github.com/2389-research/ourocodus/pkg/containersession"
	"github.com/2389-research/ourocodus/pkg/nats"
	"github.com/2389-research/ourocodus/pkg/relay"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/2389-research/ourocodus/pkg/worktree"
	"github.com/charmbracelet/lipgloss"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
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

// initLogBuffer captures log output during initialization so the banner displays first.
// After init, logs are colorized.
type initLogBuffer struct {
	mu          sync.Mutex
	buf         bytes.Buffer
	active      bool
	colorWriter *colorLogWriter
}

var initLogs = &initLogBuffer{
	active:      true,
	colorWriter: &colorLogWriter{out: os.Stderr},
}

// Write implements io.Writer, buffering logs during init, then colorizing after
func (b *initLogBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.active {
		return b.buf.Write(p)
	}
	// After init, colorize output
	return b.colorWriter.Write(p)
}

// Flush writes all buffered logs to stderr (colorized) and disables buffering
func (b *initLogBuffer) Flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len() > 0 {
		// Colorize each line of buffered output
		lines := strings.Split(b.buf.String(), "\n")
		for _, line := range lines {
			if line != "" {
				colored := colorizeLogLine(line + "\n")
				_, _ = os.Stderr.Write([]byte(colored))
			}
		}
		b.buf.Reset()
	}
	b.active = false
}

// colorLogWriter is a log writer that colorizes log output based on tags like [INIT], [ERROR], etc.
type colorLogWriter struct {
	out io.Writer
}

// Write implements io.Writer with log colorization
func (c *colorLogWriter) Write(p []byte) (n int, err error) {
	line := string(p)

	// Extract and colorize the tag if present
	colored := colorizeLogLine(line)
	return c.out.Write([]byte(colored))
}

// colorizeLogLine applies colors to a log line based on its tag
func colorizeLogLine(line string) string {
	// Color definitions - ordered by specificity (longer/more specific tags first)
	// so that [RELAY→SESSION] is matched before [RELAY] and [SESSION]
	type tagDef struct {
		tag   string
		color lipgloss.Color
	}
	tagColors := []tagDef{
		// More specific tags first (contain arrows or compound forms)
		{"[RELAY→SESSION]", colorSecondary}, // Magenta - relay-to-session routing
		{"[ACP→ATTACH]", colorSuccess},      // Green - ACP attach operations
		// Main subsystem tags
		{"[INIT]", colorSuccess},      // Green - initialization
		{"[SHUTDOWN]", colorWarning},  // Amber - shutdown
		{"[NATS]", colorPrimary},      // Cyan - NATS messages
		{"[CLEANUP]", colorMuted},     // Gray - cleanup
		{"[HEARTBEAT]", colorPrimary}, // Cyan - heartbeat
		{"[RELAY]", colorPrimary},     // Cyan - relay messages
		{"[SESSION]", colorSecondary}, // Magenta - session management
		{"[CONTAINER]", colorAccent},  // Yellow - container operations
		{"[ACP]", colorSuccess},       // Green - ACP protocol
		{"[LEASE]", colorAccent},      // Yellow - lease management
		{"[AUDIT]", colorMuted},       // Gray - audit logs
		{"[SERVER]", colorPrimary},    // Cyan - HTTP server
		{"[RATELIMIT]", colorWarning}, // Amber - rate limiting
		{"[SECURITY]", colorWarning},  // Amber - security
	}

	// Check each tag (ordered by specificity, more specific first)
	for _, td := range tagColors {
		if strings.Contains(line, td.tag) {
			// Find the tag position and colorize it
			tagStyle := lipgloss.NewStyle().Foreground(td.color).Bold(true)
			coloredTag := tagStyle.Render(td.tag)

			// Check if this is a warning or error line
			if strings.Contains(line, "WARN:") || strings.Contains(line, "ERROR:") || strings.Contains(line, "failed") {
				// Make the whole line amber for warnings
				warnStyle := lipgloss.NewStyle().Foreground(colorWarning)
				line = strings.Replace(line, td.tag, coloredTag, 1)
				// Color "WARN:" or "ERROR:" specially
				if strings.Contains(line, "WARN:") {
					warnLabel := lipgloss.NewStyle().Foreground(colorWarning).Bold(true).Render("WARN:")
					line = strings.Replace(line, "WARN:", warnLabel, 1)
				}
				return warnStyle.Render(strings.Replace(line, coloredTag, td.tag, 1))
			}

			return strings.Replace(line, td.tag, coloredTag, 1)
		}
	}

	return line
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

func main() {
	// Buffer log output during initialization so banner appears first
	log.SetOutput(initLogs)
	log.SetFlags(log.Ldate | log.Ltime)

	// Create dependencies
	logger := &relay.StdLogger{}
	clock := &relay.SystemClock{}
	idGen := &relay.UUIDGenerator{}

	ctx := context.Background()

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
		log.Fatalf("Failed to create session manager: %v", err)
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
	var heartbeatCtx context.Context
	var heartbeatCancel context.CancelFunc
	if natsClient != nil {
		heartbeatMonitor, heartbeatCtx, heartbeatCancel = initializeHeartbeatMonitor(ctx, server, logger)
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
		log.Fatalf("Failed to create web filesystem: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	// Print beautiful startup banner FIRST, then flush buffered logs
	printStartupBanner(natsClient != nil)
	initLogs.Flush()

	// Start stats reporter goroutine (updates status line periodically)
	statsCtx, statsCancel := context.WithCancel(ctx)
	go runStatsReporter(statsCtx, sessionManager)

	// Start server in goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Stop stats reporter
	statsCancel()

	// Print shutdown banner
	printShutdownBanner()

	// Stop HeartbeatMonitor if running
	if heartbeatMonitor != nil {
		log.Println("[SHUTDOWN] Stopping HeartbeatMonitor...")
		heartbeatCancel() // Stop the reaper goroutine
		heartbeatMonitor.Stop()
		log.Println("[SHUTDOWN] HeartbeatMonitor stopped")
		_ = heartbeatCtx // Ensure ctx is used (lint)
	}

	// Execute graceful shutdown sequence
	if err := gracefulShutdown(httpServer, sessionManager, natsClient); err != nil {
		log.Printf("[SHUTDOWN] Server stopped with errors: %v", err)
		os.Exit(1)
	}

	log.Println("[SHUTDOWN] Server stopped successfully")
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

// Banner colors (matching CGA theme from agentd)
var (
	colorPrimary   = lipgloss.Color("#00F6FF") // Cyan
	colorSecondary = lipgloss.Color("#FF63D8") // Magenta
	colorAccent    = lipgloss.Color("#FFEF5C") // Soft yellow
	colorSuccess   = lipgloss.Color("#39FF14") // Green
	colorWarning   = lipgloss.Color("#F8C537") // Amber
	colorMuted     = lipgloss.Color("#9CA3AF") // Light gray
)

// Rainbow gradient colors for logo
var rainbowColors = []lipgloss.Color{
	lipgloss.Color("#FF5555"), // Red
	lipgloss.Color("#FFB86C"), // Orange
	lipgloss.Color("#F1FA8C"), // Yellow
	lipgloss.Color("#50FA7B"), // Green
	lipgloss.Color("#8BE9FD"), // Cyan
	lipgloss.Color("#6272A4"), // Blue
	lipgloss.Color("#BD93F9"), // Purple
}

// ASCII art logo (medium size)
const relayLogo = ` ▄▄ ▗  ▖▗▄▄  ▄▄  ▗▄  ▄▄ ▗▄▖ ▗  ▖ ▄▄
▗▘▝▖▐  ▌▐ ▝▌▗▘▝▖▗▘ ▘▗▘▝▖▐ ▝▖▐  ▌▐▘ ▘
▐  ▌▐  ▌▐▄▄▘▐  ▌▐   ▐  ▌▐  ▌▐  ▌▝▙▄
▐  ▌▐  ▌▐▗▖ ▐  ▌▐   ▐  ▌▐  ▌▐  ▌  ▝▖
▝▙▟▘▝▄▄▘▐ ▝▖▝▙▟▘▝▙▄▐▝▙▟▘▐▄▟▘▝▄▄▘▝▄▟▘

Multi-Agent Coordination Platform`

// printStartupBanner prints a beautiful ASCII art banner on relay startup
func printStartupBanner(natsConnected bool) {
	// Apply rainbow gradient to logo (line by line)
	lines := strings.Split(relayLogo, "\n")

	var coloredLines []string
	for i, line := range lines {
		color := rainbowColors[i%len(rainbowColors)]
		coloredLine := lipgloss.NewStyle().Foreground(color).Render(line)
		coloredLines = append(coloredLines, coloredLine)
	}
	coloredLogo := strings.Join(coloredLines, "\n")

	// Create the logo box
	logoBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Align(lipgloss.Center).
		Render(coloredLogo)

	fmt.Println(logoBox)
	fmt.Println()

	// Status section with styled labels
	labelStyle := lipgloss.NewStyle().Foreground(colorMuted).Width(12)
	urlStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(colorSuccess)
	warningStyle := lipgloss.NewStyle().Foreground(colorWarning)

	// Server info
	fmt.Printf("  %s %s\n", labelStyle.Render("PWA:"), urlStyle.Render(fmt.Sprintf("http://localhost:%d/", port)))
	fmt.Printf("  %s %s\n", labelStyle.Render("WebSocket:"), urlStyle.Render(fmt.Sprintf("ws://localhost:%d/ws", port)))

	// NATS status
	natsStatus := warningStyle.Render("disabled")
	if natsConnected {
		natsStatus = successStyle.Render("connected")
	}
	fmt.Printf("  %s %s\n", labelStyle.Render("NATS:"), natsStatus)

	// Docker status (assumed connected if we got this far)
	fmt.Printf("  %s %s\n", labelStyle.Render("Docker:"), successStyle.Render("connected"))

	fmt.Println()

	// Ready message
	readyBox := lipgloss.NewStyle().
		Foreground(colorSuccess).
		Bold(true).
		Render("✓ Relay server ready")

	fmt.Printf("  %s\n", readyBox)
	fmt.Println()

	// Help hint
	hintStyle := lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	fmt.Printf("  %s\n", hintStyle.Render("Press Ctrl+C to stop"))
	fmt.Println()

	// Divider
	dividerStyle := lipgloss.NewStyle().Foreground(colorMuted)
	fmt.Println(dividerStyle.Render(strings.Repeat("─", 50)))
	fmt.Println()
}

// printShutdownBanner prints a styled shutdown message
func printShutdownBanner() {
	// Clear the status line before printing shutdown banner
	fmt.Print("\r\033[K") // Clear current line
	fmt.Println()
	dividerStyle := lipgloss.NewStyle().Foreground(colorMuted)
	fmt.Println(dividerStyle.Render(strings.Repeat("─", 50)))

	shutdownStyle := lipgloss.NewStyle().
		Foreground(colorWarning).
		Bold(true)

	fmt.Printf("\n  %s\n\n", shutdownStyle.Render("⏹  Shutting down gracefully..."))
}

// runStatsReporter periodically updates a status line showing session and agent counts.
// It runs until the context is cancelled.
func runStatsReporter(ctx context.Context, sm relay.SessionManagerInterface) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Styles for status line
	labelStyle := lipgloss.NewStyle().Foreground(colorMuted)
	countStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get stats
			sessions := sm.List(nil) // nil filter = all sessions
			sessionCount := len(sessions)

			// Count total agents across all sessions
			agentCount := 0
			for _, s := range sessions {
				agentCount += s.AgentCount()
			}

			// Build status line
			status := fmt.Sprintf("  %s %s  %s %s",
				labelStyle.Render("Sessions:"),
				countStyle.Render(fmt.Sprintf("%d", sessionCount)),
				labelStyle.Render("Agents:"),
				countStyle.Render(fmt.Sprintf("%d", agentCount)),
			)

			// Update the status line (carriage return to beginning of line)
			fmt.Print("\r\033[K" + status) // \033[K clears to end of line
		}
	}
}
