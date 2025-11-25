package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/agent"
)

func main() {
	// Setup context for graceful shutdown (SIGINT/SIGTERM)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start heartbeat publisher if NATS is configured
	agentID := os.Getenv("AGENT_ID")
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222" // Default
	}

	if agentID != "" {
		publisher, err := agent.NewHeartbeatPublisher(agentID, natsURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to start heartbeat publisher: %v\n", err)
		} else {
			// Start heartbeat publisher in background
			go publisher.Start(ctx)
			defer publisher.Stop()
		}
	}

	if err := run(ctx, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "echo-agent error: %v\n", err)
		os.Exit(1)
	}
}

// run processes ACP messages from the provided reader until context cancellation or EOF.
func run(ctx context.Context, r io.Reader, out io.Writer, errW io.Writer) error {
	lines := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			// Copy bytes because scanner.Bytes reuses buffer
			b := append([]byte(nil), scanner.Bytes()...)
			lines <- b
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
		close(lines)
		close(errCh)
	}()

	for {
		select {
		case <-ctx.Done():
			log.Printf("received shutdown signal, exiting")
			return nil
		case line, ok := <-lines:
			if !ok {
				// channel closed, exit cleanly
				return nil
			}
			handleLine(line, out, errW)
		case err := <-errCh:
			if err != nil {
				_, _ = fmt.Fprintf(errW, "Scanner error: %v\n", err)
				return err
			}
			return nil
		}
	}
}

func handleLine(line []byte, out io.Writer, errW io.Writer) {
	var req acp.Request
	if err := json.Unmarshal(line, &req); err != nil {
		_, _ = fmt.Fprintf(errW, "Failed to parse request: %v\n", err)
		return
	}

	if req.Method == acp.MethodSendMessage {
		// Extract params
		paramsData, _ := json.Marshal(req.Params)
		var params acp.SendMessageParams
		if err := json.Unmarshal(paramsData, &params); err != nil {
			sendError(out, req.ID, -32602, "Invalid params")
			return
		}

		// Echo the message back
		msg := acp.AgentMessage{
			Type:    "text",
			Content: fmt.Sprintf("Echo: %s", params.Content),
		}

		sendResponse(out, req.ID, msg)
		return
	}

	sendError(out, req.ID, -32601, "Method not found")
}

func sendResponse(w io.Writer, id interface{}, result interface{}) {
	resp := acp.Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		_, _ = fmt.Fprintf(w, "Failed to marshal response: %v\n", err)
		return
	}
	_, _ = fmt.Fprintln(w, string(data))
}

func sendError(w io.Writer, id interface{}, code int, message string) {
	resp := acp.Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &acp.Error{
			Code:    code,
			Message: message,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		_, _ = fmt.Fprintf(w, "Failed to marshal error response: %v\n", err)
		return
	}
	_, _ = fmt.Fprintln(w, string(data))
}
