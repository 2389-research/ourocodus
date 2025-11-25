package relay

// SendErrorMessage sends an error message over the WebSocket connection using the adapter.
// Returns true if the connection should be closed (write error), false otherwise.
// All WebSocket writes go through the adapter for thread-safe synchronization (issue #213).
func SendErrorMessage(adapter *SessionWebSocketAdapter, logger Logger, errorCode, errorMessage string, recoverable bool) bool {
	errorMsg := NewErrorMessage(errorCode, errorMessage, recoverable)
	if writeErr := adapter.WriteJSON(errorMsg); writeErr != nil {
		logger.Printf("Failed to send error response: %v", writeErr)
		return true // Close on write error
	}
	return false // Keep connection open
}

// SendAgentNotReadyError sends an AGENT_NOT_READY error message using the adapter.
// Returns true if the connection should be closed (write error), false otherwise.
func SendAgentNotReadyError(adapter *SessionWebSocketAdapter, logger Logger, reason string) bool {
	return SendErrorMessage(adapter, logger, "AGENT_NOT_READY", reason, true)
}

// SendAgentMessageFailedError sends an AGENT_MESSAGE_FAILED error message using the adapter.
// Returns true if the connection should be closed (write error), false otherwise.
func SendAgentMessageFailedError(adapter *SessionWebSocketAdapter, logger Logger, err error) bool {
	// Log full error server-side
	logger.Printf("[ERROR] Failed to send message to agent: %v", err)
	// Send sanitized error to client
	return SendErrorMessage(adapter, logger, "AGENT_MESSAGE_FAILED",
		sanitizeError(err), true)
}

// SendMappedError sends an error message after mapping an error to a protocol error code using the adapter.
// Returns true if the connection should be closed (write error), false otherwise.
func SendMappedError(adapter *SessionWebSocketAdapter, logger Logger, err error, mapper func(error) (string, string, bool)) bool {
	errorCode, errorMessage, recoverable := mapper(err)
	return SendErrorMessage(adapter, logger, errorCode, errorMessage, recoverable)
}
