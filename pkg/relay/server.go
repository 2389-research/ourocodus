package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/2389-research/ourocodus/pkg/acp"
	"github.com/2389-research/ourocodus/pkg/relay/session"
	"github.com/gorilla/websocket"
)

// contextWithMaxTimeout creates a context with a timeout that respects the parent's deadline.
// If the parent context has a deadline sooner than maxTimeout, uses the parent's remaining time.
// Otherwise, creates a new context with maxTimeout.
func contextWithMaxTimeout(parent context.Context, maxTimeout time.Duration) (context.Context, context.CancelFunc) {
	deadline, ok := parent.Deadline()
	if !ok {
		// No parent deadline, use maxTimeout
		return context.WithTimeout(parent, maxTimeout)
	}

	remaining := time.Until(deadline)
	if remaining < maxTimeout {
		// Parent deadline is sooner, respect it
		return context.WithDeadline(parent, deadline)
	}

	// maxTimeout is sooner
	return context.WithTimeout(parent, maxTimeout)
}

// Server handles WebSocket connections with injected dependencies
// Manages session lifecycle through sessionManager and routes messages
// to appropriate ACP agents based on message type and session state
type Server struct {
	serverID       string
	logger         Logger
	clock          Clock
	sessionClock   *SessionClockAdapter // Adapts relay.Clock to time.Time for internal use
	upgrader       Upgrader
	sessionManager SessionManagerInterface
}

// NewServer creates a new relay server with dependency injection
func NewServer(idGen IDGenerator, logger Logger, clock Clock, upgrader Upgrader, sessionManager SessionManagerInterface) *Server {
	return &Server{
		serverID:       idGen.Generate(),
		logger:         logger,
		clock:          clock,
		sessionClock:   &SessionClockAdapter{clock: clock},
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
		// Fallback for unexpected errors - sanitize error message
		validationErr = ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     sanitizeError(err),
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
func (s *Server) routeMessage(ctx context.Context, conn WebSocketConn, rawMessage []byte) (sessionID string, shouldClose bool) {
	// Validate message
	if err := ValidateMessage(rawMessage); err != nil {
		return "", s.handleValidationError(conn, err)
	}

	// Parse base message to get type
	var base BaseMessage
	if err := json.Unmarshal(rawMessage, &base); err != nil {
		// This shouldn't happen since ValidateMessage already parsed it
		s.logger.Printf("Failed to parse validated message: %v", err)
		return "", s.handleValidationError(conn, ValidationError{
			Code:        "INVALID_MESSAGE",
			Message:     "Failed to parse message",
			Recoverable: true,
		})
	}

	// Route based on message type
	s.logger.Printf("[RELAY] Routing message type: %s", base.Type)
	switch base.Type {
	case "session:create":
		s.logger.Printf("[RELAY] Handling session:create")
		return s.handleSessionCreate(ctx, conn, rawMessage)
	case "session:end":
		s.logger.Printf("[RELAY] Handling session:end")
		return "", s.handleSessionEnd(ctx, conn, rawMessage)
	case "agent:spawn":
		s.logger.Printf("[RELAY] Handling agent:spawn")
		return "", s.handleAgentSpawn(ctx, conn, rawMessage)
	case "agent:message":
		s.logger.Printf("[RELAY] Handling agent:message")
		return "", s.handleAgentMessage(ctx, conn, rawMessage)
	case "agent:terminate":
		s.logger.Printf("[RELAY] Handling agent:terminate")
		return "", s.handleAgentTerminate(ctx, conn, rawMessage)
	case "test:echo":
		// Keep echo for testing during Phase 1
		s.logger.Printf("[RELAY] Handling test:echo")
		return "", s.handleEcho(conn, rawMessage)
	default:
		s.logger.Printf("[RELAY] Unknown message type: %s", base.Type)
		return "", s.handleUnknownMessageType(conn, base.Type)
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
//
// Thread-safety: WriteJSON is protected by a mutex to prevent concurrent write panics.
// gorilla/websocket requires that only one goroutine writes to a connection at a time.
type SessionWebSocketAdapter struct {
	conn    WebSocketConn
	writeMu sync.Mutex // Protects concurrent writes (issue #213)
}

func (a *SessionWebSocketAdapter) WriteJSON(v interface{}) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.conn.WriteJSON(v)
}

func (a *SessionWebSocketAdapter) Close() error {
	// Note: Close does not need mutex protection as it's typically called once
	// during cleanup. The underlying connection handles concurrent Close calls.
	return a.conn.Close()
}

// handleSessionCreate handles session:create messages
// Creates a new user session and responds with session:created
// Returns (sessionID, shouldClose) where sessionID is non-empty if a session was created
func (s *Server) handleSessionCreate(ctx context.Context, conn WebSocketConn, rawMessage []byte) (string, bool) {
	s.logger.Printf("[RELAY] handleSessionCreate: parsing message")

	// Parse message
	msg, err := parseSessionCreateMessage(rawMessage)
	if err != nil {
		s.logger.Printf("[RELAY] handleSessionCreate: parse error: %v", err)
		return "", s.handleValidationError(conn, err)
	}

	s.logger.Printf("[RELAY] handleSessionCreate: validating message")

	// Validate message (currently no-op, but for consistency)
	if validationErr := validateSessionCreateMessage(msg); validationErr != nil {
		s.logger.Printf("[RELAY] handleSessionCreate: validation error: %v", validationErr)
		return "", s.handleValidationError(conn, validationErr)
	}

	s.logger.Printf("[RELAY] handleSessionCreate: creating user session")

	// Create user session
	// Wrap our WebSocketConn in session.WebSocketConn adapter
	sessionWS := &SessionWebSocketAdapter{conn: conn}
	userSession, err := s.sessionManager.CreateUserSession(ctx, sessionWS)
	if err != nil {
		// Log full error server-side for debugging
		s.logger.Printf("[RELAY] Failed to create user session: %v", err)
		// Send sanitized error to client
		errorMsg := NewErrorMessage(
			"SESSION_CREATE_FAILED",
			sanitizeError(err),
			true, // Recoverable - client can retry
		)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
		}
		return "", false // Keep connection open for retry
	}

	sessionID := userSession.GetID()
	s.logger.Printf("[RELAY] Created user session: %s", sessionID)

	// Send session:created response
	s.logger.Printf("[RELAY] handleSessionCreate: sending session:created response")
	response := NewSessionCreatedMessage(sessionID, s.clock.Now())
	if err := conn.WriteJSON(response); err != nil {
		s.logger.Printf("[RELAY] Failed to send session:created: %v", err)
		return sessionID, true // Close connection on write failure (session will be cleaned up by defer)
	}

	s.logger.Printf("[RELAY] handleSessionCreate: success, continuing message processing")
	return sessionID, false // Continue processing messages
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
	// Check for typed session errors - use sanitized messages
	if errors.Is(err, session.ErrSessionNotFound) {
		return "SESSION_NOT_FOUND", sanitizeError(err), false
	}

	if errors.Is(err, session.ErrAgentNotFound) {
		return "AGENT_NOT_FOUND", sanitizeError(err), false
	}

	// Check for validation errors
	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Code, validationErr.Message, validationErr.Recoverable
	}

	// Unknown error - treat as recoverable generic failure, sanitize message
	return "INTERNAL_ERROR", sanitizeError(err), true
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

	s.logger.Printf("[RELAY→SESSION] Request to spawn agent '%s' for user session %s (workspace: %s)",
		msg.AgentID, msg.UserSessionID, msg.Workspace)

	// Spawn agent in user session
	err = s.sessionManager.SpawnAgent(ctx, msg.UserSessionID, msg.AgentID, msg.Workspace)
	if err != nil {
		// Log full error server-side for debugging
		s.logger.Printf("[ERROR] Agent spawn failed: %+v", err)
		// Map error to protocol error code (distinguishes SESSION_NOT_FOUND vs generic spawn failures)
		// Keep connection open even for non-recoverable errors - client can create missing resources
		return SendMappedError(conn, s.logger, err, s.mapError)
	}

	s.logger.Printf("[RELAY] Agent spawned: userSession=%s agentID=%s", msg.UserSessionID, msg.AgentID)

	// Send agent:ready response
	response := NewAgentReadyMessage(msg.UserSessionID, msg.AgentID)
	if err := conn.WriteJSON(response); err != nil {
		s.logger.Printf("Failed to send agent:ready: %v", err)
		return true // Close on write failure
	}

	return false // Continue processing messages
}

// handleAgentMessage handles agent:message messages
// Forwards messages from PWA to ACP agent process and returns agent:response back to PWA
//
// Message Flow:
//  1. Parse and validate agent:message from PWA
//  2. Route to correct agent via sessionManager.GetAgent(sessionID, role)
//  3. Send to ACP process via agent.GetACPClient().SendMessage(content)
//  4. Parse ACP response (*acp.AgentMessage)
//  5. Store both user+agent messages in conversation history
//  6. Format and send agent:response back to PWA
//
// Error Handling Philosophy:
//   - Recoverable errors: Keep connection open, client can retry (AGENT_NOT_READY, AGENT_MESSAGE_FAILED)
//   - Non-recoverable errors: Keep connection open anyway - client can create missing resources
//     (SESSION_NOT_FOUND, AGENT_NOT_FOUND)
//   - Write errors: Close connection immediately (can't communicate with client)
//
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

	s.logger.Printf("[RELAY] Routing message to agent: userSession=%s agentID=%s",
		msg.UserSessionID, msg.AgentID)

	// Get agent from user session
	agent, err := s.sessionManager.GetAgent(msg.UserSessionID, msg.AgentID)
	if err != nil {
		s.logger.Printf("Failed to get agent: %v", err)
		// Map error to protocol error code (distinguishes SESSION_NOT_FOUND vs AGENT_NOT_FOUND)
		// Keep connection open even for non-recoverable errors - client can create missing resources
		return SendMappedError(conn, s.logger, err, s.mapError)
	}

	// Check agent state
	if agent.GetState() != session.AgentActive {
		s.logger.Printf("Agent not ready: userSession=%s agentID=%s state=%s",
			msg.UserSessionID, msg.AgentID, agent.GetState())
		return SendAgentNotReadyError(conn, s.logger,
			fmt.Sprintf("Agent is not ready (current state: %s)", agent.GetState()))
	}

	// Get ACP client and send message
	acpClient := agent.GetACPClient()
	if acpClient == nil {
		s.logger.Printf("Agent has no ACP client: userSession=%s agentID=%s", msg.UserSessionID, msg.AgentID)
		return SendAgentNotReadyError(conn, s.logger, "Agent ACP client not initialized")
	}

	// Send message to agent with timeout (fixes issue #226)
	// Wrap with explicit timeout to prevent indefinite blocking if agent process hangs
	// Respect parent context deadline if it's shorter than 30s
	acpCtx, acpCancel := contextWithMaxTimeout(ctx, 30*time.Second)
	defer acpCancel()

	response, err := acpClient.SendMessage(acpCtx, msg.Content)
	if err != nil {
		s.logger.Printf("Agent message failed: %v", err)
		return SendAgentMessageFailedError(conn, s.logger, err)
	}

	s.logger.Printf("[RELAY] Agent response received: userSession=%s agentID=%s",
		msg.UserSessionID, msg.AgentID)

	// Type assert response to *acp.AgentMessage and extract content
	agentMsg, ok := response.(*acp.AgentMessage)
	if !ok {
		s.logger.Printf("Invalid response type from agent: %T", response)
		return SendErrorMessage(conn, s.logger, "AGENT_MESSAGE_FAILED",
			"Invalid response format from agent", true)
	}
	responseStr := agentMsg.Content

	// Store both messages in history after successful ACP response
	// Use sessionClock adapter to get time.Time from injected clock
	now := s.sessionClock.Now()
	agent.AddMessage("user", msg.Content, now)
	agent.AddMessage("agent", responseStr, now)

	// Send agent:response
	responseMsg := NewAgentMessageResponse(
		msg.UserSessionID,
		msg.AgentID,
		responseStr,
		s.clock.Now(),
	)
	if err := conn.WriteJSON(responseMsg); err != nil {
		s.logger.Printf("Failed to send agent:response: %v", err)
		return true // Close on write failure
	}

	return false // Continue processing messages
}

// handleSessionEnd handles session:end messages
// Terminates all agents in the session and cleans up resources
func (s *Server) handleSessionEnd(ctx context.Context, conn WebSocketConn, rawMessage []byte) bool {
	// Parse message
	msg, err := parseSessionEndMessage(rawMessage)
	if err != nil {
		return s.handleValidationError(conn, err)
	}

	// Validate message
	if validationErr := validateSessionEndMessage(msg); validationErr != nil {
		return s.handleValidationError(conn, validationErr)
	}

	s.logger.Printf("[RELAY] Terminating user session: %s", msg.UserSessionID)

	// Optional: capture session to log agent count prior to termination
	userSession := s.sessionManager.Get(msg.UserSessionID)
	if userSession != nil {
		s.logger.Printf("[RELAY] Session has %d agents before termination", len(userSession.ListAgents()))
	}

	// Terminate session (this will terminate all agents)
	summary, err := s.sessionManager.TerminateUserSession(ctx, msg.UserSessionID)
	if err != nil {
		s.logger.Printf("Error terminating session: %v", err)
		// Map error to protocol error code
		errorCode, errorMessage, recoverable := s.mapError(err)
		errorMsg := NewErrorMessage(errorCode, errorMessage, recoverable)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true
		}
		return !recoverable
	}

	s.logger.Printf("[RELAY] User session terminated: %s, agents terminated: %d (failures=%d, cleanup=%s)",
		msg.UserSessionID, summary.AgentsTerminated, summary.AgentFailures, summary.CleanupStatus)

	// Send session:ended response
	response := NewSessionEndedMessage(msg.UserSessionID, summary.AgentsTerminated, string(summary.CleanupStatus))
	if err := conn.WriteJSON(response); err != nil {
		s.logger.Printf("Failed to send session:ended: %v", err)
		return true
	}

	return false // Continue processing messages
}

// handleAgentTerminate handles agent:terminate messages
// Terminates a specific agent while keeping the session active
func (s *Server) handleAgentTerminate(ctx context.Context, conn WebSocketConn, rawMessage []byte) bool {
	// Parse message
	msg, err := parseAgentTerminateMessage(rawMessage)
	if err != nil {
		return s.handleValidationError(conn, err)
	}

	// Validate message
	if validationErr := validateAgentTerminateMessage(msg); validationErr != nil {
		return s.handleValidationError(conn, validationErr)
	}

	s.logger.Printf("[RELAY] Terminating agent: userSession=%s agentID=%s", msg.UserSessionID, msg.AgentID)

	// Terminate the agent
	err = s.sessionManager.TerminateAgent(ctx, msg.UserSessionID, msg.AgentID)
	if err != nil {
		s.logger.Printf("Error terminating agent: %v", err)
		// Map error to protocol error code
		errorCode, errorMessage, recoverable := s.mapError(err)
		errorMsg := NewErrorMessage(errorCode, errorMessage, recoverable)
		if writeErr := conn.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true
		}
		return !recoverable
	}

	s.logger.Printf("[RELAY] Agent terminated: userSession=%s agentID=%s", msg.UserSessionID, msg.AgentID)

	// Send agent:terminated response
	// Workspace is always cleaned during termination, so workspaceCleaned is true
	response := NewAgentTerminatedMessage(msg.UserSessionID, msg.AgentID, true)
	if err := conn.WriteJSON(response); err != nil {
		s.logger.Printf("Failed to send agent:terminated: %v", err)
		return true
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

	// WebSocket hardening (issue #215)
	const (
		maxMessageSize = 1024 * 1024   // 1MB max message size
		readDeadline   = 60 * time.Second
		writeDeadline  = 10 * time.Second
		pingInterval   = 30 * time.Second
	)

	// Set read limit to prevent OOM attacks
	conn.SetReadLimit(maxMessageSize)

	// Set initial read deadline
	conn.SetReadDeadline(time.Now().Add(readDeadline))

	// Set pong handler to extend deadline on activity
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readDeadline))
		return nil
	})

	// Track session ID for cleanup on disconnect (issue #214)
	var createdSessionID string

	defer func() {
		// Clean up session if one was created during this connection (issue #214)
		if createdSessionID != "" {
			ctx := context.Background() // Use background context since request context may be cancelled
			if _, err := s.sessionManager.TerminateUserSession(ctx, createdSessionID); err != nil {
				s.logger.Printf("[RELAY] Failed to cleanup session %s on disconnect: %v", createdSessionID, err)
			} else {
				s.logger.Printf("[RELAY] Cleaned up session %s on disconnect", createdSessionID)
			}
		}

		if err := conn.Close(); err != nil {
			s.logger.Printf("Error closing connection: %v", err)
		}
	}()

	s.logger.Printf("[SERVER] WebSocket connection established from %s", r.RemoteAddr)

	// Send handshake
	if err := s.sendHandshake(conn); err != nil {
		return
	}

	ctx := r.Context()

	// Start ping goroutine for liveness checks (issue #215)
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Send ping with write deadline
				conn.SetWriteDeadline(time.Now().Add(writeDeadline))
				if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					s.logger.Printf("Failed to send ping: %v", err)
					return // Connection dead, exit goroutine
				}
			case <-ctx.Done():
				return // Request context cancelled
			case <-pingDone:
				return // Connection closed
			}
		}
	}()
	defer close(pingDone)

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			s.logger.Printf("Read error: %v", err)
			break
		}

		var base BaseMessage
		if err := json.Unmarshal(message, &base); err != nil {
			s.logger.Printf("[RELAY] Received message (%d bytes, type=unknown parse error)", len(message))
		} else {
			s.logger.Printf("[RELAY] Received message (%d bytes, type=%s)", len(message), base.Type)
		}

		sessionID, shouldClose := s.routeMessage(ctx, conn, message)
		// Track session ID if one was created during this connection
		if sessionID != "" {
			createdSessionID = sessionID
		}
		if shouldClose {
			break
		}
	}
}
