package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/2389-research/ourocodus/pkg/acp"
)

func main() {
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

	data, _ := json.Marshal(resp)
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

	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}
