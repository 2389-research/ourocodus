package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/2389-research/ourocodus/pkg/heartbeat"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/fatih/color"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var (
	monitorRaw     bool
	monitorNATSURL string
	monitorTail    string
)

var monitorCmd = &cobra.Command{
	Use:   "monitor <agent-id>",
	Short: "📡 Monitor all agent activity in real-time",
	Long: `Monitor everything happening with an agent in real-time:
  • Container logs (stdout/stderr)
  • Heartbeat events (NATS)
  • Lease renewals and attachment status

This provides a complete streaming lifeline of what an agent is doing,
combining logs, heartbeats, and lifecycle events into a single view.`,
	Example: `  # Monitor all agent activity
  agentd monitor alice

  # Monitor with raw JSON output (for scripting)
  agentd monitor alice --raw

  # Monitor with last 50 log lines
  agentd monitor alice --tail 50

  # Monitor with custom NATS URL
  agentd monitor alice --nats nats://localhost:4222`,
	Args: cobra.ExactArgs(1),
	RunE: runMonitor,
}

func init() {
	monitorCmd.Flags().BoolVar(&monitorRaw, "raw", false, "Output raw JSON events")
	monitorCmd.Flags().StringVar(&monitorNATSURL, "nats", "nats://localhost:4222", "NATS server URL")
	monitorCmd.Flags().StringVar(&monitorTail, "tail", "50", "Number of initial log lines to show (default: 50)")
}

func runMonitor(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	ctx := cmd.Context()

	// Setup signal handling for clean exit on Ctrl+C
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Find the agent container
	containerID, err := findAgentContainerID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to find agent: %w", err)
	}
	if containerID == "" {
		return fmt.Errorf("agent '%s' not found", agentID)
	}

	// Print header
	if !monitorRaw {
		headerColor := color.New(color.FgCyan, color.Bold)
		fmt.Println(strings.Repeat("━", 80))
		_, _ = headerColor.Printf("📡 Monitoring Agent: %s\n", agentID)
		fmt.Printf("   Container: %s\n", formatContainerID(containerID))
		fmt.Println(strings.Repeat("━", 80))
		fmt.Println(color.New(color.FgHiBlack).Sprint("Press Ctrl+C to stop..."))
		fmt.Println()
	}

	// Create unified event channel
	eventChan := make(chan monitorEvent, 100)

	// Start log streamer
	logCtx, logCancel := context.WithCancel(ctx)
	defer logCancel()
	go streamLogsToEvents(logCtx, containerID, agentID, eventChan)

	// Start NATS heartbeat watcher
	natsCtx, natsCancel := context.WithCancel(ctx)
	defer natsCancel()
	go watchHeartbeatsToEvents(natsCtx, agentID, monitorNATSURL, eventChan)

	// Start lease monitor
	leaseCtx, leaseCancel := context.WithCancel(ctx)
	defer leaseCancel()
	go watchLeaseToEvents(leaseCtx, agentID, eventChan)

	// Event loop - display events as they arrive
	for {
		select {
		case <-ctx.Done():
			if !monitorRaw {
				fmt.Println()
				fmt.Println(color.New(color.FgYellow).Sprint("Stopped monitoring."))
			}
			return nil

		case event := <-eventChan:
			displayEvent(event, monitorRaw)
		}
	}
}

// monitorEvent represents a unified event from any source
type monitorEvent struct {
	Type      string                 `json:"type"` // "log", "heartbeat", "lease_renewal", "lease_detached"
	Timestamp time.Time              `json:"timestamp"`
	AgentID   string                 `json:"agentId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	LogLine   string                 `json:"logLine,omitempty"` // For log events
}

// streamLogsToEvents streams container logs and converts to events
func streamLogsToEvents(ctx context.Context, containerID, agentID string, eventChan chan<- monitorEvent) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		eventChan <- monitorEvent{
			Type:      "error",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"message": fmt.Sprintf("Docker client error: %v", err)},
		}
		return
	}
	defer func() { _ = cli.Close() }()

	// Build log options
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	}

	// Set tail parameter
	if monitorTail != "all" {
		options.Tail = monitorTail
	}

	// Get log stream
	reader, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		eventChan <- monitorEvent{
			Type:      "error",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"message": fmt.Sprintf("Failed to get logs: %v", err)},
		}
		return
	}
	defer func() { _ = reader.Close() }()

	// Docker multiplexes stdout/stderr, use stdcopy to demultiplex
	// Create custom writers that emit events for each line
	stdoutWriter := &lineWriter{
		ctx:       ctx,
		agentID:   agentID,
		stream:    "stdout",
		eventChan: eventChan,
	}
	stderrWriter := &lineWriter{
		ctx:       ctx,
		agentID:   agentID,
		stream:    "stderr",
		eventChan: eventChan,
	}

	// Demux and forward logs (blocking call)
	_, err = stdcopy.StdCopy(stdoutWriter, stderrWriter, reader)
	if err != nil && err != io.EOF {
		eventChan <- monitorEvent{
			Type:      "system",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"message": fmt.Sprintf("Log stream error: %v", err)},
		}
	}
}

// lineWriter is an io.Writer that emits events for each complete line
type lineWriter struct {
	ctx       context.Context
	agentID   string
	stream    string
	eventChan chan<- monitorEvent
	buf       []byte
}

func (lw *lineWriter) Write(p []byte) (n int, err error) {
	// Check context cancellation
	select {
	case <-lw.ctx.Done():
		return 0, lw.ctx.Err()
	default:
	}

	n = len(p)
	lw.buf = append(lw.buf, p...)

	// Process complete lines
	for {
		idx := bytes.IndexByte(lw.buf, '\n')
		if idx < 0 {
			break
		}

		line := string(lw.buf[:idx])
		lw.buf = lw.buf[idx+1:]

		// Emit event
		lw.eventChan <- monitorEvent{
			Type:      "log",
			Timestamp: time.Now(),
			AgentID:   lw.agentID,
			LogLine:   line,
			Data: map[string]interface{}{
				"stream": lw.stream,
			},
		}
	}

	return n, nil
}

// watchHeartbeatsToEvents subscribes to heartbeats and converts to events
func watchHeartbeatsToEvents(ctx context.Context, agentID, natsURL string, eventChan chan<- monitorEvent) {
	// Connect to NATS
	nc, err := connectNATS(natsURL)
	if err != nil {
		eventChan <- monitorEvent{
			Type:      "error",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"message": fmt.Sprintf("NATS connection failed: %v", err)},
		}
		return
	}
	defer nc.Close()

	// Subscribe to agent heartbeats
	subject := fmt.Sprintf("%s.%s", heartbeat.SubjectPrefix, agentID)
	msgChan := make(chan *nats.Msg, 100)
	sub, err := nc.ChanSubscribe(subject, msgChan)
	if err != nil {
		eventChan <- monitorEvent{
			Type:      "error",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"message": fmt.Sprintf("Failed to subscribe to heartbeats: %v", err)},
		}
		return
	}
	defer func() { _ = sub.Unsubscribe() }()

	// Send subscription success event
	eventChan <- monitorEvent{
		Type:      "system",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"message": fmt.Sprintf("Subscribed to heartbeats: %s", subject)},
	}

	// Listen for heartbeats
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgChan:
			var hb heartbeat.Message
			if err := json.Unmarshal(msg.Data, &hb); err != nil {
				continue
			}

			lag := time.Since(hb.Timestamp)
			eventChan <- monitorEvent{
				Type:      "heartbeat",
				Timestamp: time.Now(),
				AgentID:   hb.AgentID,
				Data: map[string]interface{}{
					"status":    hb.Status,
					"lag":       lag.Milliseconds(),
					"lagString": formatWatchDuration(lag),
				},
			}
		}
	}
}

// watchLeaseToEvents monitors lease changes and converts to events
func watchLeaseToEvents(ctx context.Context, agentID string, eventChan chan<- monitorEvent) {
	// Read initial lease state
	lastState, _ := readLeaseState(agentID)

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

			// Check if lease changed
			if hasLeaseChanged(lastState, currentLease) {
				if currentLease == nil {
					// Detached
					eventChan <- monitorEvent{
						Type:      "lease_detached",
						Timestamp: time.Now(),
						AgentID:   agentID,
					}
				} else {
					// Renewed or attached
					timeUntil := time.Until(currentLease.ExpiresAt)
					eventChan <- monitorEvent{
						Type:      "lease_renewal",
						Timestamp: time.Now(),
						AgentID:   currentLease.AgentID,
						Data: map[string]interface{}{
							"userSessionId":   currentLease.UserSessionID,
							"expiresAt":       currentLease.ExpiresAt,
							"timeUntilExpiry": timeUntil.Seconds(),
							"expiryString":    formatWatchDuration(timeUntil),
						},
					}
				}
				lastState = currentLease
			}
		}
	}
}

// Helper functions from watch/lease commands

// connectNATS connects to NATS server
func connectNATS(url string) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("agentd-monitor"),
		nats.Timeout(5 * time.Second),
		nats.MaxReconnects(-1), // Reconnect indefinitely
		nats.ReconnectWait(2 * time.Second),
	}

	return nats.Connect(url, opts...)
}

// readLeaseState reads the current lease state for an agent
func readLeaseState(agentID string) (*session.Lease, error) {
	lease, err := session.ReadLease(agentID)
	if err != nil {
		if err == session.ErrLeaseNotFound {
			return nil, nil // No lease yet
		}
		return nil, err
	}
	return lease, nil
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

	// Compare expiry times (renewal detected if expiry time changed significantly)
	if old.ExpiresAt.Sub(new.ExpiresAt).Abs() > 10*time.Second {
		return true
	}

	// Compare session IDs
	return old.UserSessionID != new.UserSessionID
}

// formatSessionID truncates session ID for display
func formatSessionID(sessionID string) string {
	if len(sessionID) <= 16 {
		return sessionID
	}
	return sessionID[:13] + "..."
}

// formatWatchDuration formats time.Duration as human-readable string
func formatWatchDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// displayEvent renders an event to the user
func displayEvent(event monitorEvent, raw bool) {
	if raw {
		// Raw JSON output
		data, _ := json.Marshal(event)
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	timestamp := event.Timestamp.Format("15:04:05")

	switch event.Type {
	case "log":
		// Log lines - simple format
		stream := event.Data["stream"].(string)
		icon := "📄"
		if stream == "stderr" {
			icon = "⚠️"
		}
		fmt.Printf("[%s] %s %s\n", timestamp, icon, event.LogLine)

	case "heartbeat":
		// Heartbeat - show lag and status
		status := event.Data["status"].(string)
		lagStr := event.Data["lagString"].(string)
		fmt.Printf("[%s] %s Heartbeat (lag=%s, status=%s)\n",
			timestamp,
			color.New(color.FgGreen).Sprint("💓"),
			lagStr,
			status,
		)

	case "lease_renewal":
		// Lease renewal - show session and expiry
		sessionID := event.Data["userSessionId"].(string)
		expiryStr := event.Data["expiryString"].(string)
		fmt.Printf("[%s] %s Lease renewed (expires in %s, session=%s)\n",
			timestamp,
			color.New(color.FgCyan).Sprint("🔐"),
			expiryStr,
			formatSessionID(sessionID),
		)

	case "lease_detached":
		// Lease detached
		fmt.Printf("[%s] %s Lease detached\n",
			timestamp,
			color.New(color.FgYellow).Sprint("🔓"),
		)

	case "system":
		// System messages (e.g., subscription confirmation)
		msg := event.Data["message"].(string)
		fmt.Printf("[%s] %s %s\n",
			timestamp,
			color.New(color.FgBlue).Sprint("ℹ️"),
			msg,
		)

	case "error":
		// Error messages
		msg := event.Data["message"].(string)
		fmt.Printf("[%s] %s %s\n",
			timestamp,
			color.New(color.FgRed).Sprint("❌"),
			msg,
		)
	}
}
