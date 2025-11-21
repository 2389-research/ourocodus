package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/2389-research/ourocodus/pkg/heartbeat"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/fatih/color"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

var (
	watchRaw     bool
	watchNATSURL string
)

var watchCmd = &cobra.Command{
	Use:   "watch <agent-id>",
	Short: "👁️  Watch agent heartbeats and lease events in real-time",
	Long: `Watch an agent's heartbeats and lease renewals in real-time.

This command subscribes to the agent's NATS heartbeat subject and monitors
its lease file for changes. It displays events as they happen, making it
easy to debug agent liveness and adoption status.`,
	Example: `  # Watch agent events
  agentd watch alice

  # Watch with raw JSON output
  agentd watch alice --raw

  # Watch with custom NATS URL
  agentd watch alice --nats nats://localhost:4222`,
	Args: cobra.ExactArgs(1),
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().BoolVar(&watchRaw, "raw", false, "Output raw JSON events")
	watchCmd.Flags().StringVar(&watchNATSURL, "nats", "nats://localhost:4222", "NATS server URL")
}

func runWatch(cmd *cobra.Command, args []string) error {
	agentID := args[0]
	ctx := cmd.Context()

	// Setup signal handling for clean exit on Ctrl+C
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Print header
	if !watchRaw {
		headerColor := color.New(color.FgCyan, color.Bold)
		_, _ = headerColor.Printf("👁️  Watching agent: %s\n", agentID)
		fmt.Println(color.New(color.FgHiBlack).Sprint("Press Ctrl+C to stop..."))
		fmt.Println()
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
	if !watchRaw {
		fmt.Println(color.New(color.FgGreen).Sprintf("✓ Subscribed to: %s", subject))
		fmt.Println()
	}

	// Read initial lease state
	lastLeaseState, _ := readLeaseState(agentID)

	// Start lease monitor in background
	leaseChan := make(chan *session.Lease, 10)
	go monitorLease(ctx, agentID, lastLeaseState, leaseChan)

	// Event loop
	for {
		select {
		case <-ctx.Done():
			if !watchRaw {
				fmt.Println()
				fmt.Println(color.New(color.FgYellow).Sprint("Stopped watching."))
			}
			return nil

		case msg := <-msgChan:
			if err := handleHeartbeat(msg, watchRaw); err != nil {
				if !watchRaw {
					fmt.Printf("Error handling heartbeat: %v\n", err)
				}
			}

		case lease := <-leaseChan:
			handleLeaseChange(lease, watchRaw)
		}
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
func handleHeartbeat(msg *nats.Msg, raw bool) error {
	var hb heartbeat.Message
	if err := json.Unmarshal(msg.Data, &hb); err != nil {
		return fmt.Errorf("failed to unmarshal heartbeat: %w", err)
	}

	if raw {
		// Output raw JSON
		data, _ := json.Marshal(map[string]interface{}{
			"type":      "heartbeat",
			"agentId":   hb.AgentID,
			"timestamp": hb.Timestamp,
			"status":    hb.Status,
			"lag":       time.Since(hb.Timestamp).Milliseconds(),
		})
		fmt.Println(string(data))
	} else {
		// Human-readable output
		timestamp := time.Now().Format("15:04:05")
		lag := time.Since(hb.Timestamp)

		fmt.Printf("[%s] %s Heartbeat received (lag=%s, status=%s)\n",
			timestamp,
			color.New(color.FgGreen).Sprint("💓"),
			formatWatchDuration(lag),
			hb.Status,
		)
	}

	return nil
}

// handleLeaseChange processes a lease change event
func handleLeaseChange(lease *session.Lease, raw bool) {
	timestamp := time.Now().Format("15:04:05")

	// Handle lease detach (nil lease)
	if lease == nil {
		if raw {
			data, _ := json.Marshal(map[string]interface{}{
				"type": "lease_detached",
			})
			fmt.Println(string(data))
		} else {
			fmt.Printf("[%s] %s Lease detached\n",
				timestamp,
				color.New(color.FgYellow).Sprint("🔓"),
			)
		}
		return
	}

	// Handle lease renewal
	if raw {
		// Output raw JSON
		data, _ := json.Marshal(map[string]interface{}{
			"type":            "lease_renewal",
			"agentId":         lease.AgentID,
			"userSessionId":   lease.UserSessionID,
			"expiresAt":       lease.ExpiresAt,
			"timeUntilExpiry": time.Until(lease.ExpiresAt).Seconds(),
		})
		fmt.Println(string(data))
	} else {
		// Human-readable output
		timeUntil := time.Until(lease.ExpiresAt)

		fmt.Printf("[%s] %s Lease renewed (expires in %s, session=%s)\n",
			timestamp,
			color.New(color.FgCyan).Sprint("🔐"),
			formatWatchDuration(timeUntil),
			formatSessionID(lease.UserSessionID),
		)
	}
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

// formatWatchDuration formats time.Duration as human-readable string for watch command
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
