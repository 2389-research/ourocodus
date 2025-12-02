package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/2389-research/ourocodus/pkg/cli"
	"github.com/2389-research/ourocodus/pkg/cli/format"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// DockerClient interface for Docker operations needed by attach command.
type DockerClient interface {
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (container.ExecCreateResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error)
	Close() error
}

var attachCmd = &cobra.Command{
	Use:   "attach <agent-id>",
	Short: "📎 Attach to an agent's interactive shell",
	Long: `Attach to a running agent's container and get an interactive shell.

This opens an interactive bash session inside the agent's container where you
can explore the workspace, run commands, and interact with the agent's environment.

Press Ctrl-D or type 'exit' to detach from the agent.`,
	Example: `  # Attach to agent alice
  agentd attach alice

  # Once attached, you can:
  #  - Explore the workspace: ls /workspace
  #  - Check agent status: ps aux
  #  - View logs: tail -f /var/log/*.log
  #  - Run git commands: git status

  # Detach: Ctrl-D or type 'exit'`,
	Args: cobra.ExactArgs(1),
	RunE: runAttach,
}

func runAttach(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	agentID := args[0]

	// Get AppContext for mode-aware output
	appCtx := cli.FromContext(ctx)

	// Find and validate the container
	containerID, dockerCli, err := findAndValidateContainer(ctx, agentID)
	if err != nil {
		return err
	}
	defer func() { _ = dockerCli.Close() }()

	// Print attach message based on mode
	printAttachMessage(agentID, containerID, appCtx)

	// Run the interactive session
	return runAttachSession(ctx, containerID, dockerCli)
}

// findAndValidateContainer finds the agent container and validates it is running.
func findAndValidateContainer(ctx context.Context, agentID string) (string, DockerClient, error) {
	containerID, err := findAgentContainerID(ctx, agentID)
	if err != nil {
		return "", nil, cli.IOError("failed to find agent: " + err.Error())
	}

	if containerID == "" {
		return "", nil, cli.UsageError("agent '" + agentID + "' not found")
	}

	dockerCli, err := newDockerClient()
	if err != nil {
		return "", nil, err
	}

	inspect, err := dockerCli.ContainerInspect(ctx, containerID)
	if err != nil {
		_ = dockerCli.Close()
		return "", nil, cli.IOError("failed to inspect container: " + err.Error())
	}

	if !inspect.State.Running {
		_ = dockerCli.Close()
		return "", nil, cli.UsageError(fmt.Sprintf("agent '%s' is not running (status: %s)", agentID, inspect.State.Status))
	}

	return containerID, dockerCli, nil
}

// printAttachMessage dispatches to mode-specific attach output.
// Uses custom functions rather than Output interface because:
// - JSON mode outputs structured container metadata
// - Rich mode uses multi-line themed layout with emojis
// - Plain mode displays formatted connection details
func printAttachMessage(agentID, containerID string, appCtx *cli.AppContext) {
	if appCtx == nil {
		printAttachPlain(agentID, containerID)
		return
	}

	switch appCtx.Mode {
	case cli.ModeJSON:
		printAttachJSON(agentID, containerID)
	case cli.ModePlain:
		printAttachPlain(agentID, containerID)
	default: // ModeRich
		printAttachRich(agentID, containerID, appCtx)
	}
}

// printAttachJSON outputs attach metadata as JSON for machine parsing.
func printAttachJSON(agentID, containerID string) {
	info := map[string]interface{}{
		"agent_id":     agentID,
		"container_id": containerID,
		"workspace":    "/workspace",
		"status":       "attaching",
	}
	jsonData, _ := json.Marshal(info)
	fmt.Println(string(jsonData))
}

// printAttachPlain displays formatted attach info for plain text output.
func printAttachPlain(agentID, containerID string) {
	fmt.Printf("Attaching to agent '%s'\n", agentID)
	fmt.Printf("Container: %s\n", format.FormatContainerID(containerID))
	fmt.Printf("Workspace: /workspace\n")
	fmt.Println()
	fmt.Println("Press Ctrl-D or type 'exit' to detach")
	fmt.Println()
}

// printAttachRich displays themed multi-line attach info with emojis for rich terminal output.
func printAttachRich(agentID, containerID string, appCtx *cli.AppContext) {
	th := appCtx.Theme

	fmt.Println()
	fmt.Println(th.Title.Render(fmt.Sprintf("📎 Attaching to agent '%s'", agentID)))
	fmt.Println(th.MutedText.Render(fmt.Sprintf("   Container: %s", format.FormatContainerID(containerID))))
	fmt.Println(th.MutedText.Render("   Workspace: /workspace"))
	fmt.Println()
	fmt.Println(th.WarningText.Render("   Press Ctrl-D or type 'exit' to detach"))
	fmt.Println()
}

// runAttachSession runs the interactive attach session.
func runAttachSession(ctx context.Context, containerID string, dockerCli DockerClient) error {
	execConfig := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{"/bin/bash"},
	}

	execID, err := dockerCli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return cli.IOError("failed to create exec: " + err.Error())
	}

	resp, err := dockerCli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		return cli.IOError("failed to attach to exec: " + err.Error())
	}
	defer resp.Close()

	oldState, err := setRawTerminal()
	if err != nil {
		return cli.IOError("failed to set raw terminal: " + err.Error())
	}
	defer func() { _ = restoreTerminal(oldState) }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(resp.Conn, os.Stdin)
		errCh <- err
	}()
	go func() {
		_, err := io.Copy(os.Stdout, resp.Reader)
		errCh <- err
	}()

	select {
	case <-sigCh:
		fmt.Println("\n\nDetached from agent")
		return nil
	case err := <-errCh:
		if err != nil && err != io.EOF {
			return cli.IOError("error during attach: " + err.Error())
		}
		fmt.Println("\nDetached from agent")
		return nil
	}
}

// setRawTerminal sets the terminal to raw mode and returns the old state
func setRawTerminal() (*term.State, error) {
	fd := int(os.Stdin.Fd())
	return term.MakeRaw(fd)
}

// restoreTerminal restores the terminal to its previous state
func restoreTerminal(oldState *term.State) error {
	if oldState == nil {
		return nil
	}
	fd := int(os.Stdin.Fd())
	return term.Restore(fd, oldState)
}
