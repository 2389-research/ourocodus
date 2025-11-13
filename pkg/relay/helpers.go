package relay

import "fmt"

// SendErrorMessage sends an error message over the WebSocket connection.
// Returns true if the connection should be closed (write error), false otherwise.
func SendErrorMessage(conn WebSocketConn, logger Logger, errorCode, errorMessage string, recoverable bool) bool {
	errorMsg := NewErrorMessage(errorCode, errorMessage, recoverable)
	if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
		logger.Printf("Failed to send error response: %v", writeErr)
		return true // Close on write error
	}
	return false // Keep connection open
}

// SendAgentNotReadyError sends an AGENT_NOT_READY error message.
// Returns true if the connection should be closed (write error), false otherwise.
func SendAgentNotReadyError(conn WebSocketConn, logger Logger, reason string) bool {
	return SendErrorMessage(conn, logger, "AGENT_NOT_READY", reason, true)
}

// SendAgentMessageFailedError sends an AGENT_MESSAGE_FAILED error message.
// Returns true if the connection should be closed (write error), false otherwise.
func SendAgentMessageFailedError(conn WebSocketConn, logger Logger, err error) bool {
	return SendErrorMessage(conn, logger, "AGENT_MESSAGE_FAILED",
		fmt.Sprintf("Failed to send message to agent: %v", err), true)
}

// SendMappedError sends an error message after mapping an error to a protocol error code.
// Returns true if the connection should be closed (write error), false otherwise.
func SendMappedError(conn WebSocketConn, logger Logger, err error, mapper func(error) (string, string, bool)) bool {
	errorCode, errorMessage, recoverable := mapper(err)
	return SendErrorMessage(conn, logger, errorCode, errorMessage, recoverable)
}
