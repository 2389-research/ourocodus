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

	"github.com/2389-research/ourocodus/cmd/agentd/internal/detect"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/output"
	"github.com/2389-research/ourocodus/cmd/agentd/internal/theme"
	watchtui "github.com/2389-research/ourocodus/cmd/agentd/internal/tui/watch"
	"github.com/2389-research/ourocodus/pkg/heartbeat"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var (
	watchJSON    bool
	watchPlain   bool
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
	watchCmd.Flags().BoolVar(&watchJSON, "json", false, "Output JSON events")
	watchCmd.Flags().BoolVar(&watchPlain, "plain", false, "Output plain text (no colors)")
	watchCmd.Flags().StringVar(&watchNATSURL, "nats", "nats://localhost:4222", "NATS server URL")
	watchCmd.Flags().BoolVar(&watchLogs, "logs", true, "Stream container stdout/stderr for the agent")
}

func runWatch(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	ctx := cmd.Context()

	mode := detectWatchOutputMode()

	// Setup signal handling for clean exit on Ctrl+C
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// For rich mode, use the Bubble Tea TUI
	if mode.IsRich() {
		return runWatchTUI(ctx, agentID)
	}

	// For JSON/plain modes, use the legacy streaming output
	// Print header
	if !mode.IsJSON() {
		printWatchHeader(agentID, mode)
	}

	// Setup NATS subscription
	_, msgChan, subject, cleanup, err := setupNATSSubscription(agentID)
	if err != nil {
		return err
	}
	defer cleanup()

	// Print subscription info
	if !mode.IsJSON() {
		printWatchSubscribed(subject, mode)
	}

	// Start background monitors
	leaseChan := startLeaseMonitor(ctx, agentID)
	logCancel := startLogStreamer(ctx, agentID, mode)

	runWatchEventLoop(ctx, mode, msgChan, leaseChan, logCancel)
	return nil
}

// runWatchTUI runs the watch command with Bubble Tea TUI.
func runWatchTUI(ctx context.Context, agentID string) error {
	// Setup NATS subscription
	_, msgChan, subject, cleanup, err := setupNATSSubscription(agentID)
	if err != nil {
		return err
	}
	defer cleanup()

	// Create TUI
	m := watchtui.New(agentID)
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

// detectWatchOutputMode determines the output mode based on flags and TTY.
// For watch, we only care about explicit flags and TTY,
// not terminal size (watch is simple streaming output, doesn't need 80x24).
func detectWatchOutputMode() output.Mode {
	switch {
	case watchJSON:
		return output.ModeJSON
	case watchPlain:
		return output.ModePlain
	case !detect.IsTTY():
		return output.ModePlain
	default:
		return output.ModeRich
	}
}

// setupNATSSubscription connects to NATS and subscribes to the agent's heartbeat subject.
// Returns the connection, message channel, subject name, cleanup function, and any error.
func setupNATSSubscription(agentID string) (*nats.Conn, <-chan *nats.Msg, string, func(), error) {
	nc, err := connectNATS(watchNATSURL)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	subject := fmt.Sprintf("%s.%s", heartbeat.SubjectPrefix, agentID)
	msgChan := make(chan *nats.Msg, 100)

	sub, err := nc.ChanSubscribe(subject, msgChan)
	if err != nil {
		nc.Close()
		return nil, nil, "", nil, fmt.Errorf("failed to subscribe to heartbeats: %w", err)
	}

	cleanup := func() {
		_ = sub.Unsubscribe()
		nc.Close()
	}

	return nc, msgChan, subject, cleanup, nil
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
func startLogStreamer(ctx context.Context, agentID string, mode output.Mode) context.CancelFunc {
	if !watchLogs {
		return nil
	}
	logsCtx, logCancel := context.WithCancel(ctx)
	go streamAgentLogs(logsCtx, agentID, mode)
	return logCancel
}

// runWatchEventLoop runs the main event loop, processing heartbeats and lease changes.
// Returns when the context is cancelled.
func runWatchEventLoop(ctx context.Context, mode output.Mode, msgChan <-chan *nats.Msg, leaseChan <-chan *session.Lease, logCancel context.CancelFunc) {
	for {
		select {
		case <-ctx.Done():
			if logCancel != nil {
				logCancel()
			}
			if !mode.IsJSON() {
				printWatchStopped(mode)
			}
			return

		case msg := <-msgChan:
			if err := handleHeartbeat(msg, mode); err != nil && !mode.IsJSON() {
				fmt.Printf("Error handling heartbeat: %v\n", err)
			}

		case lease := <-leaseChan:
			handleLeaseChange(lease, mode)
		}
	}
}

// printWatchHeader prints the watch header based on mode
func printWatchHeader(agentID string, mode output.Mode) {
	if mode.IsRich() {
		th := theme.NewRetroTheme(theme.PaletteCGA)
		headerStyle := lipgloss.NewStyle().Foreground(th.Primary).Bold(true)
		mutedStyle := lipgloss.NewStyle().Foreground(th.Muted)
		fmt.Println(headerStyle.Render(fmt.Sprintf("👁️  Watching agent: %s", agentID)))
		fmt.Println(mutedStyle.Render("Press Ctrl+C to stop..."))
		fmt.Println()
	} else {
		fmt.Printf("Watching agent: %s\n", agentID)
		fmt.Println("Press Ctrl+C to stop...")
		fmt.Println()
	}
}

// printWatchSubscribed prints the subscription info based on mode
func printWatchSubscribed(subject string, mode output.Mode) {
	if mode.IsRich() {
		th := theme.NewRetroTheme(theme.PaletteCGA)
		successStyle := lipgloss.NewStyle().Foreground(th.Success)
		fmt.Println(successStyle.Render(fmt.Sprintf("✓ Subscribed to: %s", subject)))
		fmt.Println()
	} else {
		fmt.Printf("Subscribed to: %s\n", subject)
		fmt.Println()
	}
}

// printWatchStopped prints the stopped message based on mode
func printWatchStopped(mode output.Mode) {
	fmt.Println()
	if mode.IsRich() {
		th := theme.NewRetroTheme(theme.PaletteCGA)
		warningStyle := lipgloss.NewStyle().Foreground(th.Warning)
		fmt.Println(warningStyle.Render("Stopped watching."))
	} else {
		fmt.Println("Stopped watching.")
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
func handleHeartbeat(msg *nats.Msg, mode output.Mode) error {
	var hb heartbeat.Message
	if err := json.Unmarshal(msg.Data, &hb); err != nil {
		return fmt.Errorf("failed to unmarshal heartbeat: %w", err)
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
			output.FormatDurationShort(lag),
			hb.Status,
		)
	default: // Rich mode
		th := theme.NewRetroTheme(theme.PaletteCGA)
		successStyle := lipgloss.NewStyle().Foreground(th.Success)
		fmt.Printf("[%s] %s Heartbeat received (lag=%s, status=%s)\n",
			timestamp,
			successStyle.Render("💓"),
			output.FormatDurationShort(lag),
			hb.Status,
		)
	}

	return nil
}

// handleLeaseChange processes a lease change event
func handleLeaseChange(lease *session.Lease, mode output.Mode) {
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
			th := theme.NewRetroTheme(theme.PaletteCGA)
			warningStyle := lipgloss.NewStyle().Foreground(th.Warning)
			fmt.Printf("[%s] %s Lease detached\n",
				timestamp,
				warningStyle.Render("🔓"),
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
			output.FormatDurationShort(timeUntil),
			output.FormatSessionID(lease.UserSessionID),
		)
	default: // Rich mode
		th := theme.NewRetroTheme(theme.PaletteCGA)
		cyanStyle := lipgloss.NewStyle().Foreground(th.Primary)
		fmt.Printf("[%s] %s Lease renewed (expires in %s, session=%s)\n",
			timestamp,
			cyanStyle.Render("🔐"),
			output.FormatDurationShort(timeUntil),
			output.FormatSessionID(lease.UserSessionID),
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
func streamAgentLogs(ctx context.Context, agentID string, mode output.Mode) {
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
