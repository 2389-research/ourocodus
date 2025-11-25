package main

import (
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
	"github.com/2389-research/ourocodus/pkg/heartbeat"
	"github.com/2389-research/ourocodus/pkg/relay/session"
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

	// Detect output mode - for watch, we only care about explicit flags and TTY,
	// not terminal size (watch is simple streaming output, doesn't need 80x24)
	mode := output.ModeRich
	if watchJSON {
		mode = output.ModeJSON
	} else if watchPlain {
		mode = output.ModePlain
	} else if !detect.IsTTY() {
		mode = output.ModePlain
	}

	// Setup signal handling for clean exit on Ctrl+C
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Print header
	if !mode.IsJSON() {
		printWatchHeader(agentID, mode)
	}

	// Connect to NATS
	nc, err := connectNATS(watchNATSURL)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer nc.Close()

	// Subscribe to agent heartbeats
	subject := fmt.Sprintf("%s.%s", heartbeat.SubjectPrefix, agentID)

	msgChan := make(chan *nats.Msg, 100)
	sub, err := nc.ChanSubscribe(subject, msgChan)
	if err != nil {
		return fmt.Errorf("failed to subscribe to heartbeats: %w", err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	// Print subscription info
	if !mode.IsJSON() {
		printWatchSubscribed(subject, mode)
	}

	// Read initial lease state
	lastLeaseState, _ := readLeaseState(agentID)

	// Start lease monitor in background
	leaseChan := make(chan *session.Lease, 10)
	go monitorLease(ctx, agentID, lastLeaseState, leaseChan)

	// Optionally stream container logs
	var logCancel context.CancelFunc
	if watchLogs {
		var logsCtx context.Context
		logsCtx, logCancel = context.WithCancel(ctx)
		go streamAgentLogs(logsCtx, agentID, mode)
	}

	// Event loop
	for {
		select {
		case <-ctx.Done():
			if logCancel != nil {
				logCancel()
			}
			if !mode.IsJSON() {
				printWatchStopped(mode)
			}
			return nil

		case msg := <-msgChan:
			if err := handleHeartbeat(msg, mode); err != nil {
				if !mode.IsJSON() {
					fmt.Printf("Error handling heartbeat: %v\n", err)
				}
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
