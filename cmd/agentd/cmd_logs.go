package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsTail   string
)

var logsCmd = &cobra.Command{
	Use:   "logs <agent-id>",
	Short: "📜 Stream agent container logs",
	Long: `Stream logs from an agent's container in real-time.

By default, follows the log output. Use Ctrl-C to stop streaming.`,
	Example: `  # Follow logs in real-time
  agentd logs alice

  # Show last 50 lines without following
  agentd logs alice --tail 50 --follow=false

  # Stream logs until manually stopped
  agentd logs alice -f`,
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", true, "Follow log output")
	logsCmd.Flags().StringVar(&logsTail, "tail", "all", "Number of lines to show from the end (default: all)")
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	agentID := args[0]

	// Find the container
	containerID, err := findAgentContainerID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("failed to find agent: %w", err)
	}

	if containerID == "" {
		return fmt.Errorf("agent '%s' not found", agentID)
	}

	// Stream logs
	return streamContainerLogs(ctx, agentID, containerID)
}

func findAgentContainerID(ctx context.Context, agentID string) (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer cli.Close()

	// Find container with matching agent-id label
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "ourocodus.agent=true")
	filterArgs.Add("label", fmt.Sprintf("agent-id=%s", agentID))

	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return "", err
	}

	if len(containers) == 0 {
		return "", nil
	}

	return containers[0].ID, nil
}

func streamContainerLogs(ctx context.Context, agentID, containerID string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	// Build log options
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     logsFollow,
		Timestamps: false,
	}

	// Set tail parameter
	if logsTail != "all" {
		options.Tail = logsTail
	}

	// Get log stream
	reader, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}
	defer reader.Close()

	// Print header
	infoColor.Printf("=== Logs for agent '%s' (container: %s) ===\n\n", agentID, formatContainerID(containerID))

	// Stream logs to stdout/stderr
	// Docker multiplexes stdout/stderr, so we use stdcopy to demultiplex
	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
	if err != nil && err != io.EOF {
		return fmt.Errorf("error streaming logs: %w", err)
	}

	return nil
}
