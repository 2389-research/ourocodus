package relay

import (
	"encoding/json"
	"net/http"
)

// Server handles WebSocket connections with injected dependencies
type Server struct {
	serverID    string
	logger      Logger
	clock       Clock
	upgrader    Upgrader
}

// NewServer creates a new relay server with dependency injection
func NewServer(idGen IDGenerator, logger Logger, clock Clock, upgrader Upgrader) *Server {
	return &Server{
		serverID: idGen.Generate(),
		logger:   logger,
		clock:    clock,
		upgrader: upgrader,
	}
}

// sendHandshake sends the connection established message (single responsibility)
func (s *Server) sendHandshake(conn WebSocketConn) error {
	handshake := NewConnectionEstablished(s.serverID, s.clock.Now())
	if err := conn.WriteJSON(handshake); err != nil {
		s.logger.Printf("Failed to send handshake: %v", err)
		return err
	}
	return nil
}

// handleValidationError processes validation errors and sends appropriate responses
// Returns true if connection should be closed
func (s *Server) handleValidationError(conn WebSocketConn, err error) bool {
	s.logger.Printf("Invalid message: %v", err)

	// Convert to ValidationError
	var validationErr ValidationError
	if verr, ok := err.(ValidationError); ok {
		validationErr = verr
	} else {
		// Fallback for unexpected errors
		validationErr = ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     err.Error(),
			Recoverable: true,
		}
	}

	// Send error response
	errorMsg := NewErrorMessage(validationErr.Code, validationErr.Message, validationErr.Recoverable)
	if err := conn.WriteJSON(errorMsg); err != nil {
		s.logger.Printf("Failed to send error response: %v", err)
	}

	// Determine if connection should close
	if !validationErr.Recoverable {
		s.logger.Printf("Closing connection due to non-recoverable error: %s", validationErr.Code)
		return true // Close connection
	}

	return false // Keep connection open
}

// addTimestamp adds timestamp to message (pure-ish - operates on provided map)
func (s *Server) addTimestamp(msg map[string]interface{}) {
	msg["timestamp"] = s.clock.Now()
}

// echoMessage parses, timestamps, and echoes back a message
func (s *Server) echoMessage(conn WebSocketConn, rawMessage []byte) error {
	var msg map[string]interface{}
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		s.logger.Printf("Failed to parse message: %v", err)
		return err
	}

	s.addTimestamp(msg)

	if err := conn.WriteJSON(msg); err != nil {
		s.logger.Printf("Write error: %v", err)
		return err
	}

	return nil
}

// handleMessage processes a single incoming message
// Returns true if connection should be closed
func (s *Server) handleMessage(conn WebSocketConn, rawMessage []byte) bool {
	// Validate message
	if err := ValidateMessage(rawMessage); err != nil {
		return s.handleValidationError(conn, err)
	}

	// Echo message back
	if err := s.echoMessage(conn, rawMessage); err != nil {
		return true // Close on echo failure
	}

	return false // Continue processing messages
}

// HandleWebSocket handles WebSocket upgrade and connection lifecycle
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	s.logger.Printf("WebSocket connection established from %s", r.RemoteAddr)

	// Send handshake
	if err := s.sendHandshake(conn); err != nil {
		return
	}

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			s.logger.Printf("Read error: %v", err)
			break
		}

		if shouldClose := s.handleMessage(conn, message); shouldClose {
			break
		}
	}
}
