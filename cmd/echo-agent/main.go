package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/agent"
)

func main() {
	// Setup context for graceful shutdown
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

	// Process ACP messages from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()

		// Parse incoming JSON-RPC request
		var req acp.Request
		if err := json.Unmarshal(line, &req); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse request: %v\n", err)
			continue
		}

		// Handle sendMessage method
		if req.Method == acp.MethodSendMessage {
			// Extract params
			paramsData, _ := json.Marshal(req.Params)
			var params acp.SendMessageParams
			if err := json.Unmarshal(paramsData, &params); err != nil {
				sendError(req.ID, -32602, "Invalid params")
				continue
			}

			// Echo the message back
			msg := acp.AgentMessage{
				Type:    "text",
				Content: fmt.Sprintf("Echo: %s", params.Content),
			}

			sendResponse(req.ID, msg)
		} else {
			sendError(req.ID, -32601, "Method not found")
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Scanner error: %v\n", err)
		os.Exit(1)
	}
}

func sendResponse(id interface{}, result interface{}) {
	resp := acp.Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal response: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func sendError(id interface{}, code int, message string) {
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
		fmt.Fprintf(os.Stderr, "Failed to marshal error response: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
