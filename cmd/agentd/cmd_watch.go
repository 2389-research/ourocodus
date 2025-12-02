package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	watchtui "github.com/2389-research/ourocodus/cmd/agentd/internal/tui/watch"
	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/cli/format"
	"github.com/2389-research/ourocodus/pkg/heartbeat"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/2389-research/ourocodus/pkg/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var (
	watchNATSURL string
	watchLogs    bool
)

// JSON event types for structured output
type (
	// HeartbeatEvent represents a heartbeat message in JSON output
	HeartbeatEvent struct {
		Type      string    `json:"type"`
		AgentID   string    `json:"agentId"`
		Timestamp time.Time `json:"timestamp"`
		Status    string    `json:"status"`
		LagMs     int64     `json:"lag"`
	}

	// LeaseDetachedEvent represents a lease detachment in JSON output
	LeaseDetachedEvent struct {
		Type string `json:"type"`
	}

	// LeaseRenewalEvent represents a lease renewal in JSON output
	LeaseRenewalEvent struct {
		Type            string    `json:"type"`
		AgentID         string    `json:"agentId"`
		UserSessionID   string    `json:"userSessionId"`
		ExpiresAt       time.Time `json:"expiresAt"`
		TimeUntilExpiry float64   `json:"timeUntilExpiry"`
	}
)

var watchCmd = &cobra.Command{
	Use:   "watch <agent-id>",
	Short: "👁️  Watch agent heartbeats and lease events in real-time",
	Long: `Watch an agent's heartbeats and lease renewals in real-time.

This command subscribes to the agent's NATS heartbeat subject and monitors
its lease file for changes. It displays events as they happen, making it
easy to debug agent liveness and adoption status.`,
	Example: `  # Watch agent events (rich mode with colors)
  agentd watch alice

  # Watch with JSON output for scripting
  agentd watch alice --json

  # Watch with plain text output (no colors)
  agentd watch alice --plain

  # Watch with custom NATS URL
  agentd watch alice --nats nats://localhost:4222`,
	Args: cobra.ExactArgs(1),
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().StringVar(&watchNATSURL, "nats", "nats://localhost:4222", "NATS server URL")
	watchCmd.Flags().BoolVar(&watchLogs, "logs", true, "Stream container stdout/stderr for the agent")
}

func runWatch(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	ctx := cmd.Context()

	// Get mode from AppContext (set by cli.App wrapper)
	appCtx := cli.FromContext(ctx)
	if appCtx == nil {
		return cli.ContextError()
	}
	mode := appCtx.Mode

	// Setup signal handling for clean exit on Ctrl+C
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// For rich mode, use the Bubble Tea TUI
	if mode.IsRich() {
		return runWatchTUI(ctx, agentID, appCtx)
	}

	// For JSON/plain modes, use the legacy streaming output
	// Print header
	if !mode.IsJSON() {
		printWatchHeader(agentID, appCtx)
	}

	// Setup NATS subscription
	msgChan, subject, cleanup, err := setupNATSSubscription(agentID)
	if err != nil {
		return err
	}
	defer cleanup()

	// Print subscription info
	if !mode.IsJSON() {
		if appCtx.Mode.IsRich() {
			fmt.Println(appCtx.Theme.SuccessText.Render(fmt.Sprintf("✓ Subscribed to: %s", subject)))
			fmt.Println()
		} else {
			appCtx.Output.Success(fmt.Sprintf("Subscribed to: %s", subject))
			fmt.Println()
		}
	}

	// Start background monitors
	leaseChan := startLeaseMonitor(ctx, agentID)
	logCancel := startLogStreamer(ctx, agentID, mode)

	runWatchEventLoop(ctx, appCtx, msgChan, leaseChan, logCancel)
	return nil
}

// runWatchTUI runs the watch command with Bubble Tea TUI.
func runWatchTUI(ctx context.Context, agentID string, appCtx *cli.AppContext) error {
	// Setup NATS subscription
	msgChan, subject, cleanup, err := setupNATSSubscription(agentID)
	if err != nil {
		return err
	}
	defer cleanup()

	// Create TUI
	m := watchtui.New(agentID, appCtx.Theme)
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Start background goroutines to feed events to TUI
	go func() {
		// Send subscription notification
		p.Send(watchtui.SubscribedMsg{Subject: subject})

		// Process heartbeats
		for msg := range msgChan {
			var hb heartbeat.Message
			if err := json.Unmarshal(msg.Data, &hb); err != nil {
				p.Send(watchtui.ErrorMsg{Err: err})
				continue
			}
			lag := time.Since(hb.Timestamp)
			p.Send(watchtui.HeartbeatMsg{
				AgentID:   hb.AgentID,
				Timestamp: hb.Timestamp,
				Status:    hb.Status,
				Lag:       lag,
			})
		}
	}()

	// Monitor lease changes
	go func() {
		lastLeaseState, _ := readLeaseState(agentID)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentLease, err := readLeaseState(agentID)
				if err != nil {
					continue
				}
				if hasLeaseChanged(lastLeaseState, currentLease) {
					p.Send(watchtui.LeaseChangeMsg{Lease: currentLease})
					lastLeaseState = currentLease
				}
			}
		}
	}()

	// Stream container logs if enabled
	if watchLogs {
		go streamAgentLogsTUI(ctx, agentID, p)
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		p.Quit()
	}()

	// Run TUI
	_, err = p.Run()
	return err
}

// streamAgentLogsTUI streams container logs to the TUI.
func streamAgentLogsTUI(ctx context.Context, agentID string, p *tea.Program) {
	containerID, err := findAgentContainerID(ctx, agentID)
	if err != nil || containerID == "" {
		return
	}

	cli, err := newDockerClient()
	if err != nil {
		return
	}
	defer func() { _ = cli.Close() }()

	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "20",
	}
	rc, err := cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return
	}
	defer func() { _ = rc.Close() }()

	// Use a scanner to read lines
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip Docker multiplexing header bytes (first 8 bytes of each frame)
		if len(line) > 8 {
			line = line[8:]
		}
		p.Send(watchtui.LogLineMsg{Line: line, IsErr: false})
	}
}

// detectWatchOutputMode removed - now using cli.Mode from AppContext

// setupNATSSubscription connects to NATS and subscribes to the agent's heartbeat subject.
// Returns the message channel, subject name, cleanup function, and any error.
func setupNATSSubscription(agentID string) (<-chan *nats.Msg, string, func(), error) {
	nc, err := connectNATS(watchNATSURL)
	if err != nil {
		return nil, "", nil, cli.IOError("failed to connect to NATS: " + err.Error())
	}

	subject := fmt.Sprintf("%s.%s", heartbeat.SubjectPrefix, agentID)
	msgChan := make(chan *nats.Msg, 100)

	sub, err := nc.ChanSubscribe(subject, msgChan)
	if err != nil {
		nc.Close()
		return nil, "", nil, cli.IOError("failed to subscribe to heartbeats: " + err.Error())
	}

	cleanup := func() {
		_ = sub.Unsubscribe()
		nc.Close()
	}

	return msgChan, subject, cleanup, nil
}

// startLeaseMonitor starts a background goroutine to monitor lease changes.
// Returns the channel that will receive lease change events.
func startLeaseMonitor(ctx context.Context, agentID string) <-chan *session.Lease {
	lastLeaseState, _ := readLeaseState(agentID)
	leaseChan := make(chan *session.Lease, 10)
	go monitorLease(ctx, agentID, lastLeaseState, leaseChan)
	return leaseChan
}

// startLogStreamer starts a background goroutine to stream container logs if enabled.
// Returns a cancel function that should be called on shutdown, or nil if streaming is disabled.
func startLogStreamer(ctx context.Context, agentID string, mode cli.Mode) context.CancelFunc {
	if !watchLogs {
		return nil
	}
	logsCtx, logCancel := context.WithCancel(ctx)
	go streamAgentLogs(logsCtx, agentID, mode)
	return logCancel
}

// runWatchEventLoop runs the main event loop, processing heartbeats and lease changes.
// Returns when the context is cancelled.
func runWatchEventLoop(ctx context.Context, appCtx *cli.AppContext, msgChan <-chan *nats.Msg, leaseChan <-chan *session.Lease, logCancel context.CancelFunc) {
	for {
		select {
		case <-ctx.Done():
			if logCancel != nil {
				logCancel()
			}
			if !appCtx.Mode.IsJSON() {
				fmt.Println()
				if appCtx.Mode.IsRich() {
					fmt.Println(appCtx.Theme.WarningText.Render("Stopped watching."))
				} else {
					appCtx.Output.Info("Stopped watching.")
				}
			}
			return

		case msg := <-msgChan:
			if err := handleHeartbeat(msg, appCtx.Mode, appCtx.Theme); err != nil && !appCtx.Mode.IsJSON() {
				appCtx.Output.Error(err)
			}

		case lease := <-leaseChan:
			handleLeaseChange(lease, appCtx.Mode, appCtx.Theme)
		}
	}
}

// printWatchHeader prints the watch header based on mode.
// This is complex (themed box with multiple lines) so kept custom instead of using Output interface.
func printWatchHeader(agentID string, appCtx *cli.AppContext) {
	if appCtx.Mode.IsRich() {
		fmt.Println(appCtx.Theme.Title.Render(fmt.Sprintf("👁️  Watching agent: %s", agentID)))
		fmt.Println(appCtx.Theme.MutedText.Render("Press Ctrl+C to stop..."))
		fmt.Println()
	} else {
		fmt.Printf("Watching agent: %s\n", agentID)
		fmt.Println("Press Ctrl+C to stop...")
		fmt.Println()
	}
}

// connectNATS connects to NATS server
func connectNATS(url string) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("agentd-watch"),
		nats.Timeout(5 * time.Second),
		nats.MaxReconnects(-1), // Reconnect indefinitely
		nats.ReconnectWait(2 * time.Second),
	}

	return nats.Connect(url, opts...)
}

// handleHeartbeat processes a heartbeat message
func handleHeartbeat(msg *nats.Msg, mode cli.Mode, th *theme.Theme) error {
	var hb heartbeat.Message
	if err := json.Unmarshal(msg.Data, &hb); err != nil {
		return cli.IOError("failed to unmarshal heartbeat: " + err.Error())
	}

	timestamp := time.Now().Format("15:04:05")
	lag := time.Since(hb.Timestamp)

	switch {
	case mode.IsJSON():
		data, _ := json.Marshal(HeartbeatEvent{
			Type:      "heartbeat",
			AgentID:   hb.AgentID,
			Timestamp: hb.Timestamp,
			Status:    hb.Status,
			LagMs:     lag.Milliseconds(),
		})
		fmt.Println(string(data))
	case mode.IsPlain():
		fmt.Printf("[%s] Heartbeat received (lag=%s, status=%s)\n",
			timestamp,
			format.FormatDurationShort(lag),
			hb.Status,
		)
	default: // Rich mode
		fmt.Printf("[%s] %s Heartbeat received (lag=%s, status=%s)\n",
			timestamp,
			th.SuccessText.Render("💓"),
			format.FormatDurationShort(lag),
			hb.Status,
		)
	}

	return nil
}

// handleLeaseChange processes a lease change event
func handleLeaseChange(lease *session.Lease, mode cli.Mode, th *theme.Theme) {
	timestamp := time.Now().Format("15:04:05")

	// Handle lease detach (nil lease)
	if lease == nil {
		switch {
		case mode.IsJSON():
			data, _ := json.Marshal(LeaseDetachedEvent{
				Type: "lease_detached",
			})
			fmt.Println(string(data))
		case mode.IsPlain():
			fmt.Printf("[%s] Lease detached\n", timestamp)
		default: // Rich mode
			fmt.Printf("[%s] %s Lease detached\n",
				timestamp,
				th.WarningText.Render("🔓"),
			)
		}
		return
	}

	// Handle lease renewal
	timeUntil := time.Until(lease.ExpiresAt)

	switch {
	case mode.IsJSON():
		data, _ := json.Marshal(LeaseRenewalEvent{
			Type:            "lease_renewal",
			AgentID:         lease.AgentID,
			UserSessionID:   lease.UserSessionID,
			ExpiresAt:       lease.ExpiresAt,
			TimeUntilExpiry: timeUntil.Seconds(),
		})
		fmt.Println(string(data))
	case mode.IsPlain():
		fmt.Printf("[%s] Lease renewed (expires in %s, session=%s)\n",
			timestamp,
			format.FormatDurationShort(timeUntil),
			format.FormatSessionID(lease.UserSessionID),
		)
	default: // Rich mode
		fmt.Printf("[%s] %s Lease renewed (expires in %s, session=%s)\n",
			timestamp,
			th.PrimaryText.Render("🔐"),
			format.FormatDurationShort(timeUntil),
			format.FormatSessionID(lease.UserSessionID),
		)
	}
}

// readLeaseState reads the current lease state for an agent
// Returns nil for expired leases (treats them as unattached for instant validity)
func readLeaseState(agentID string) (*session.Lease, error) {
	lease, err := session.ReadLease(agentID)
	if err != nil {
		if err == session.ErrLeaseNotFound {
			return nil, nil // No lease yet
		}
		return nil, err
	}
	// Return nil for expired leases - agent should appear unattached immediately
	// This decouples lease file existence from validity (reaper handles file cleanup)
	if session.IsLeaseExpired(lease) {
		return nil, nil
	}
	return lease, nil
}

// monitorLease polls the lease file for changes
func monitorLease(ctx context.Context, agentID string, lastState *session.Lease, leaseChan chan<- *session.Lease) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentLease, err := readLeaseState(agentID)
			if err != nil {
				// Log unexpected errors (ErrLeaseNotFound is already handled in readLeaseState)
				fmt.Fprintf(os.Stderr, "Warning: failed to read lease state: %v\n", err)
				continue
			}

			// Check if lease changed
			if hasLeaseChanged(lastState, currentLease) {
				// Send lease change event (including nil for detach)
				leaseChan <- currentLease
				lastState = currentLease
			}
		}
	}
}

// streamAgentLogs streams container stdout/stderr to the console to show agent messages.
// Falls back silently if the container cannot be found.
func streamAgentLogs(ctx context.Context, agentID string, mode cli.Mode) {
	// Don't stream logs in JSON mode to avoid mixing formats
	if mode.IsJSON() {
		return
	}

	containerID, err := findAgentContainerID(ctx, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch: failed to find container for agent %s: %v\n", agentID, err)
		return
	}
	if containerID == "" {
		fmt.Fprintf(os.Stderr, "watch: container for agent %s not found\n", agentID)
		return
	}

	cli, err := newDockerClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch: failed to create Docker client: %v\n", err)
		return
	}
	defer func() { _ = cli.Close() }()

	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "20",
	}
	rc, err := cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch: failed to tail logs: %v\n", err)
		return
	}
	defer func() { _ = rc.Close() }()

	if _, err := stdcopy.StdCopy(os.Stdout, os.Stderr, rc); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "watch: log stream error: %v\n", err)
	}
}

// hasLeaseChanged checks if lease state has changed
func hasLeaseChanged(old, new *session.Lease) bool {
	// Both nil - no change
	if old == nil && new == nil {
		return false
	}

	// One is nil - changed
	if (old == nil) != (new == nil) {
		return true
	}

	// Compare attachment identity only (ignore expiry jitter to avoid spam)
	return old.AgentID != new.AgentID || old.UserSessionID != new.UserSessionID
}
