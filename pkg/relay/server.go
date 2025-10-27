package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/session"
)

// Server handles WebSocket connections with injected dependencies
// Manages session lifecycle through sessionManager and routes messages
// to appropriate ACP agents based on message type and session state
type Server struct {
	serverID       string
	logger         Logger
	clock          Clock
	upgrader       Upgrader
	sessionManager *session.Manager
}

// NewServer creates a new relay server with dependency injection
func NewServer(idGen IDGenerator, logger Logger, clock Clock, upgrader Upgrader, sessionManager *session.Manager) *Server {
	return &Server{
		serverID:       idGen.Generate(),
		logger:         logger,
		clock:          clock,
		upgrader:       upgrader,
		sessionManager: sessionManager,
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

// handleEcho handles test:echo messages (kept for Phase 1 testing)
// Returns true if connection should be closed
func (s *Server) handleEcho(conn WebSocketConn, rawMessage []byte) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		s.logger.Printf("Failed to parse echo message: %v", err)
		return true // Close on parse error
	}

	s.addTimestamp(msg)

	if err := conn.WriteJSON(msg); err != nil {
		s.logger.Printf("Write error: %v", err)
		return true // Close on write error
	}

	return false // Continue processing
}

// routeMessage routes incoming messages to appropriate handlers based on type
// Returns true if connection should be closed
func (s *Server) routeMessage(ctx context.Context, conn WebSocketConn, rawMessage []byte) bool {
	// Validate message
	if err := ValidateMessage(rawMessage); err != nil {
		return s.handleValidationError(conn, err)
	}

	// Parse base message to get type
	var base BaseMessage
	if err := json.Unmarshal(rawMessage, &base); err != nil {
		// This shouldn't happen since ValidateMessage already parsed it
		s.logger.Printf("Failed to parse validated message: %v", err)
		return s.handleValidationError(conn, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Failed to parse message",
			Recoverable: true,
		})
	}

	// Route based on message type
	switch base.Type {
	case "session:create":
		return s.handleSessionCreate(ctx, conn, rawMessage)
	case "agent:spawn":
		return s.handleAgentSpawn(ctx, conn, rawMessage)
	case "agent:message":
		return s.handleAgentMessage(ctx, conn, rawMessage)
	case "test:echo":
		// Keep echo for testing during Phase 1
		return s.handleEcho(conn, rawMessage)
	default:
		return s.handleUnknownMessageType(conn, base.Type)
	}
}

// handleUnknownMessageType handles messages with unknown types
func (s *Server) handleUnknownMessageType(conn WebSocketConn, msgType string) bool {
	s.logger.Printf("Unknown message type: %s", msgType)
	err := ValidationError{
		Code:        "UNKNOWN_MESSAGE_TYPE",
		Message:     fmt.Sprintf("Unknown message type: %s", msgType),
		Recoverable: true,
	}
	return s.handleValidationError(conn, err)
}

// SessionWebSocketAdapter adapts relay.WebSocketConn to session.WebSocketConn
// Bridges the WebSocket interface between relay and session packages
// Server owns all I/O - adapter only exposes WriteJSON and Close to session layer
type SessionWebSocketAdapter struct {
	conn WebSocketConn
}

func (a *SessionWebSocketAdapter) WriteJSON(v interface{}) error {
	return a.conn.WriteJSON(v)
}

func (a *SessionWebSocketAdapter) Close() error {
	return a.conn.Close()
}

// handleSessionCreate handles session:create messages
// Creates a new user session and responds with session:created
func (s *Server) handleSessionCreate(ctx context.Context, conn WebSocketConn, rawMessage []byte) bool {
	// Parse message
	msg, err := parseSessionCreateMessage(rawMessage)
	if err != nil {
		return s.handleValidationError(conn, err)
	}

	// Validate message (currently no-op, but for consistency)
	if validationErr := validateSessionCreateMessage(msg); validationErr != nil {
		return s.handleValidationError(conn, validationErr)
	}

	// Create user session
	// Wrap our WebSocketConn in session.WebSocketConn adapter
	sessionWS := &SessionWebSocketAdapter{conn: conn}
	userSession, err := s.sessionManager.CreateUserSession(ctx, sessionWS)
	if err != nil {
		s.logger.Printf("Failed to create user session: %v", err)
		errorMsg := NewErrorMessage(
			"SESSION_CREATE_FAILED",
			fmt.Sprintf("Failed to create session: %v", err),
			true, // Recoverable - client can retry
		)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
		}
		return false // Keep connection open for retry
	}

	s.logger.Printf("Created user session: %s", userSession.GetID())

	// Send session:created response
	response := NewSessionCreatedMessage(userSession.GetID(), s.clock.Now())
	if err := conn.WriteJSON(response); err != nil {
		s.logger.Printf("Failed to send session:created: %v", err)
		return true // Close connection on write failure
	}

	return false // Continue processing messages
}

// mapError maps session layer errors to protocol error codes
// Returns (errorCode, errorMessage, recoverable)
//
// Error semantics:
// - ErrSessionNotFound: Session ID not found (non-recoverable, client must create session)
// - ErrAgentNotFound: Agent role not found in session (non-recoverable, client must spawn agent)
// - ValidationError: Protocol validation failure (recoverability depends on error)
// - Other errors: Unexpected failures (recoverable, client can retry)
func (s *Server) mapError(err error) (code, message string, recoverable bool) {
	// Check for typed session errors
	if errors.Is(err, session.ErrSessionNotFound) {
		return "SESSION_NOT_FOUND", err.Error(), false
	}

	if errors.Is(err, session.ErrAgentNotFound) {
		return "AGENT_NOT_FOUND", err.Error(), false
	}

	// Check for validation errors
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Code, validationErr.Message, validationErr.Recoverable
	}

	// Unknown error - treat as recoverable generic failure
	return "INTERNAL_ERROR", err.Error(), true
}

// handleAgentSpawn handles agent:spawn messages
// Spawns an agent in an existing session and responds with agent:ready
func (s *Server) handleAgentSpawn(ctx context.Context, conn WebSocketConn, rawMessage []byte) bool {
	// Parse message
	msg, err := parseAgentSpawnMessage(rawMessage)
	if err != nil {
		return s.handleValidationError(conn, err)
	}

	// Validate message
	if validationErr := validateAgentSpawnMessage(msg); validationErr != nil {
		return s.handleValidationError(conn, validationErr)
	}

	s.logger.Printf("Spawning agent: session=%s role=%s workspace=%s",
		msg.SessionID, msg.Role, msg.Workspace)

	// Spawn agent in session
	err = s.sessionManager.SpawnAgent(ctx, msg.SessionID, msg.Role, msg.Workspace)
	if err != nil {
		s.logger.Printf("Failed to spawn agent: %v", err)

		// Map error to protocol error code
		errorCode, errorMessage, recoverable := s.mapError(err)

		// Override generic message for spawn-specific context
		if errorCode == "INTERNAL_ERROR" {
			errorCode = "AGENT_SPAWN_FAILED"
			errorMessage = fmt.Sprintf("Failed to spawn agent: %v", err)
		}

		errorMsg := NewErrorMessage(errorCode, errorMessage, recoverable)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true // Close on write failure
		}

		return !recoverable // Close if non-recoverable
	}

	s.logger.Printf("Agent spawned: session=%s role=%s", msg.SessionID, msg.Role)

	// Send agent:ready response
	response := NewAgentReadyMessage(msg.SessionID, msg.Role)
	if err := conn.WriteJSON(response); err != nil {
		s.logger.Printf("Failed to send agent:ready: %v", err)
		return true // Close on write failure
	}

	return false // Continue processing messages
}

// handleAgentMessage handles agent:message messages
// Forwards messages to agents and returns agent:response
// Note: ctx is not currently used by ACPClient.SendMessage, but is kept for future timeout/cancellation support
func (s *Server) handleAgentMessage(ctx context.Context, conn WebSocketConn, rawMessage []byte) bool {
	// Parse message
	msg, err := parseAgentMessageRequest(rawMessage)
	if err != nil {
		return s.handleValidationError(conn, err)
	}

	// Validate message
	if validationErr := validateAgentMessageRequest(msg); validationErr != nil {
		return s.handleValidationError(conn, validationErr)
	}

	_ = ctx // TODO: Pass context to ACP client when timeout/cancellation support is implemented in ACPClient.SendMessage

	s.logger.Printf("Routing message to agent: session=%s role=%s",
		msg.SessionID, msg.Role)

	// Get agent from session
	agent, err := s.sessionManager.GetAgent(msg.SessionID, msg.Role)
	if err != nil {
		s.logger.Printf("Failed to get agent: %v", err)

		// Map error to protocol error code (distinguishes SESSION_NOT_FOUND vs AGENT_NOT_FOUND)
		errorCode, errorMessage, recoverable := s.mapError(err)

		errorMsg := NewErrorMessage(errorCode, errorMessage, recoverable)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
		}

		return !recoverable // Close if non-recoverable
	}

	// Check agent state
	if agent.GetState() != session.AgentActive {
		s.logger.Printf("Agent not ready: session=%s role=%s state=%s",
			msg.SessionID, msg.Role, agent.GetState())

		errorMsg := NewErrorMessage(
			"AGENT_NOT_READY",
			fmt.Sprintf("Agent is not ready (current state: %s)", agent.GetState()),
			true, // Recoverable - client can wait and retry
		)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true
		}

		return false // Keep connection open for retry
	}

	// Get ACP client and send message
	acpClient := agent.GetACPClient()
	if acpClient == nil {
		s.logger.Printf("Agent has no ACP client: session=%s role=%s", msg.SessionID, msg.Role)

		errorMsg := NewErrorMessage(
			"AGENT_NOT_READY",
			"Agent ACP client not initialized",
			true, // Recoverable
		)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true
		}

		return false // Keep connection open for retry
	}

	// Send message to agent
	response, err := acpClient.SendMessage(msg.Content)
	if err != nil {
		s.logger.Printf("Agent message failed: %v", err)

		errorMsg := NewErrorMessage(
			"AGENT_MESSAGE_FAILED",
			fmt.Sprintf("Failed to send message to agent: %v", err),
			true, // Recoverable - client can retry
		)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true
		}

		return false // Keep connection open for retry
	}

	s.logger.Printf("Agent response received: session=%s role=%s",
		msg.SessionID, msg.Role)

	// Convert response to string
	// response is interface{}, typically will be a string or structured data
	responseStr := fmt.Sprintf("%v", response)

	// Store both messages in history after successful ACP response
	// Note: Using time.Now() directly because relay.Clock returns string for protocol messages,
	// but session layer expects time.Time for internal tracking
	now := time.Now()
	agent.AddMessage("user", msg.Content, now)
	agent.AddMessage("agent", responseStr, now)

	// Send agent:response
	responseMsg := NewAgentMessageResponse(
		msg.SessionID,
		msg.Role,
		responseStr,
		s.clock.Now(),
	)
	if err := conn.WriteJSON(responseMsg); err != nil {
		s.logger.Printf("Failed to send agent:response: %v", err)
		return true // Close on write failure
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
	defer func() {
		if err := conn.Close(); err != nil {
			s.logger.Printf("Error closing connection: %v", err)
		}
	}()

	s.logger.Printf("WebSocket connection established from %s", r.RemoteAddr)

	// Send handshake
	if err := s.sendHandshake(conn); err != nil {
		return
	}

	ctx := r.Context()

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			s.logger.Printf("Read error: %v", err)
			break
		}

		if shouldClose := s.routeMessage(ctx, conn, message); shouldClose {
			break
		}
	}
}
