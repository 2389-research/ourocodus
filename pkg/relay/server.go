package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/2389-research/ourocodus/pkg/relay/ratelimit"
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
	rateLimiter    *ratelimit.Limiter // Phase 4: Rate limiting for attach operations
}

// NewServer creates a new relay server with dependency injection
func NewServer(idGen IDGenerator, logger Logger, clock Clock, upgrader Upgrader, sessionManager SessionManagerInterface) *Server {
	// Phase 4: Configure rate limiter for attach operations
	// 10 tokens max (burst capacity), 1 token per second refill
	// Allows 10 rapid attach attempts, then 1 per second
	return &Server{
		serverID:       idGen.Generate(),
		logger:         logger,
		clock:          clock,
		sessionClock:   &SessionClockAdapter{clock: clock},
		upgrader:       upgrader,
		sessionManager: sessionManager,
		rateLimiter:    ratelimit.NewLimiter(10, 1),
	}
}

// sendHandshake sends the connection established message using the adapter for thread-safe writes.
// All WebSocket writes go through the adapter for synchronization (issue #213).
func (s *Server) sendHandshake(adapter *SessionWebSocketAdapter) error {
	handshake := NewConnectionEstablished(s.serverID, s.clock.Now())
	if err := adapter.WriteJSON(handshake); err != nil {
		s.logger.Printf("Failed to send handshake: %v", err)
		return err
	}
	return nil
}

// handleValidationError processes validation errors and sends appropriate responses
// Returns true if connection should be closed
// Uses adapter for thread-safe writes (issue #213 - WebSocket write synchronization)
func (s *Server) handleValidationError(adapter *SessionWebSocketAdapter, err error) bool {
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

	// Send error response via adapter for thread-safe writes
	errorMsg := NewErrorMessage(validationErr.Code, validationErr.Message, validationErr.Recoverable)
	if err := adapter.WriteJSON(errorMsg); err != nil {
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
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleEcho(adapter *SessionWebSocketAdapter, rawMessage []byte) bool {
	var msg map[string]interface{}
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		s.logger.Printf("Failed to parse echo message: %v", err)
		return s.handleValidationError(adapter, ValidationError{
			Code: "INVALID_MESSAGE", Message: "Failed to parse echo message", Recoverable: true,
		})
	}

	s.addTimestamp(msg)

	if err := adapter.WriteJSON(msg); err != nil {
		s.logger.Printf("Write error: %v", err)
		return true // Close on write error
	}

	return false // Continue processing
}

// routeMessage routes incoming messages to appropriate handlers based on type
// Returns true if connection should be closed
// adapter parameter is the SessionWebSocketAdapter for this connection (reused for all writes)
// All handlers now use adapter for thread-safe writes (issue #213 - WebSocket write synchronization)
func (s *Server) routeMessage(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) (sessionID string, shouldClose bool) {
	// Validate message
	if err := ValidateMessage(rawMessage); err != nil {
		return "", s.handleValidationError(adapter, err)
	}

	// Parse base message to get type
	var base BaseMessage
	if err := json.Unmarshal(rawMessage, &base); err != nil {
		// This shouldn't happen since ValidateMessage already parsed it
		s.logger.Printf("Failed to parse validated message: %v", err)
		return "", s.handleValidationError(adapter, ValidationError{
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
		return s.handleSessionCreate(ctx, adapter, rawMessage)
	case "session:end":
		s.logger.Printf("[RELAY] Handling session:end")
		return "", s.handleSessionEnd(ctx, adapter, rawMessage)
	case "agent:spawn":
		s.logger.Printf("[RELAY] Handling agent:spawn")
		return "", s.handleAgentSpawn(ctx, adapter, rawMessage)
	case "agent:message":
		s.logger.Printf("[RELAY] Handling agent:message")
		return "", s.handleAgentMessage(ctx, adapter, rawMessage)
	case "agent:terminate":
		s.logger.Printf("[RELAY] Handling agent:terminate")
		return "", s.handleAgentTerminate(ctx, adapter, rawMessage)
	case "agent:discover":
		s.logger.Printf("[RELAY] Handling agent:discover")
		return "", s.handleAgentDiscover(ctx, adapter, rawMessage)
	case "agent:attach":
		s.logger.Printf("[RELAY] Handling agent:attach")
		return "", s.handleAgentAttach(ctx, adapter, rawMessage)
	case "agent:detach":
		s.logger.Printf("[RELAY] Handling agent:detach")
		return "", s.handleAgentDetach(ctx, adapter, rawMessage)
	case "test:echo":
		// Keep echo for testing during Phase 1
		s.logger.Printf("[RELAY] Handling test:echo")
		return "", s.handleEcho(adapter, rawMessage)
	default:
		s.logger.Printf("[RELAY] Unknown message type: %s", base.Type)
		return "", s.handleUnknownMessageType(adapter, base.Type)
	}
}

// handleUnknownMessageType handles messages with unknown types
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleUnknownMessageType(adapter *SessionWebSocketAdapter, msgType string) bool {
	s.logger.Printf("Unknown message type: %s", msgType)
	err := ValidationError{
		Code:        "UNKNOWN_MESSAGE_TYPE",
		Message:     fmt.Sprintf("Unknown message type: %s", msgType),
		Recoverable: true,
	}
	return s.handleValidationError(adapter, err)
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

// WriteMessage sends a binary message with mutex protection
// Used for control frames like pings that must be synchronized with data writes
func (a *SessionWebSocketAdapter) WriteMessage(messageType int, data []byte) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.conn.WriteMessage(messageType, data)
}

// SetWriteDeadline sets write deadline with mutex protection
// Deadline applies to all subsequent writes until changed
func (a *SessionWebSocketAdapter) SetWriteDeadline(t time.Time) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.conn.SetWriteDeadline(t)
}

func (a *SessionWebSocketAdapter) Close() error {
	// Note: Close does not need mutex protection as it's typically called once
	// during cleanup. The underlying connection handles concurrent Close calls.
	return a.conn.Close()
}

// handleSessionCreate handles session:create messages
// Creates a new user session and responds with session:created
// Returns (sessionID, shouldClose) where sessionID is non-empty if a session was created
func (s *Server) handleSessionCreate(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) (string, bool) {
	s.logger.Printf("[RELAY] handleSessionCreate: parsing message")

	// Parse message
	msg, err := parseSessionCreateMessage(rawMessage)
	if err != nil {
		s.logger.Printf("[RELAY] handleSessionCreate: parse error: %v", err)
		return "", s.handleValidationError(adapter, err)
	}

	s.logger.Printf("[RELAY] handleSessionCreate: validating message")

	// Validate message (currently no-op, but for consistency)
	if validationErr := validateSessionCreateMessage(msg); validationErr != nil {
		s.logger.Printf("[RELAY] handleSessionCreate: validation error: %v", validationErr)
		return "", s.handleValidationError(adapter, validationErr)
	}

	s.logger.Printf("[RELAY] handleSessionCreate: creating user session")

	// Create user session using the existing adapter (already has write mutex)
	userSession, err := s.sessionManager.CreateUserSession(ctx, adapter)
	if err != nil {
		// Log full error server-side for debugging
		s.logger.Printf("[RELAY] Failed to create user session: %v", err)
		// Send sanitized error to client
		errorMsg := NewErrorMessage(
			"SESSION_CREATE_FAILED",
			sanitizeError(err),
			true, // Recoverable - client can retry
		)
		if writeErr := adapter.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
		}
		return "", false // Keep connection open for retry
	}

	sessionID := userSession.GetID()
	s.logger.Printf("[RELAY] Created user session: %s", sessionID)

	// Send session:created response
	s.logger.Printf("[RELAY] handleSessionCreate: sending session:created response")
	response := NewSessionCreatedMessage(sessionID, s.clock.Now())
	if err := adapter.WriteJSON(response); err != nil {
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
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleAgentSpawn(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) bool {
	// Parse message
	msg, err := parseAgentSpawnMessage(rawMessage)
	if err != nil {
		return s.handleValidationError(adapter, err)
	}

	// Validate message
	if validationErr := validateAgentSpawnMessage(msg); validationErr != nil {
		return s.handleValidationError(adapter, validationErr)
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
		return SendMappedError(adapter, s.logger, err, s.mapError)
	}

	s.logger.Printf("[RELAY] Agent spawned: userSession=%s agentID=%s", msg.UserSessionID, msg.AgentID)

	// Send agent:ready response
	response := NewAgentReadyMessage(msg.UserSessionID, msg.AgentID)
	if err := adapter.WriteJSON(response); err != nil {
		s.logger.Printf("Failed to send agent:ready: %v", err)
		return true // Close on write failure
	}

	return false // Continue processing messages
}

// handleAgentMessage handles agent:message messages
// Forwards messages from PWA to ACP agent process and returns agent:response back to PWA
// Uses adapter for thread-safe writes (issue #213)
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
func (s *Server) handleAgentMessage(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) bool {
	// Parse message
	msg, err := parseAgentMessageRequest(rawMessage)
	if err != nil {
		return s.handleValidationError(adapter, err)
	}

	// Validate message
	if validationErr := validateAgentMessageRequest(msg); validationErr != nil {
		return s.handleValidationError(adapter, validationErr)
	}

	s.logger.Printf("[RELAY] Routing message to agent: userSession=%s agentID=%s",
		msg.UserSessionID, msg.AgentID)

	// Get agent from user session
	agent, err := s.sessionManager.GetAgent(msg.UserSessionID, msg.AgentID)
	if err != nil {
		s.logger.Printf("Failed to get agent: %v", err)
		// Map error to protocol error code (distinguishes SESSION_NOT_FOUND vs AGENT_NOT_FOUND)
		// Keep connection open even for non-recoverable errors - client can create missing resources
		return SendMappedError(adapter, s.logger, err, s.mapError)
	}

	// Check agent state
	if agent.GetState() != session.AgentActive {
		s.logger.Printf("Agent not ready: userSession=%s agentID=%s state=%s",
			msg.UserSessionID, msg.AgentID, agent.GetState())
		return SendAgentNotReadyError(adapter, s.logger,
			fmt.Sprintf("Agent is not ready (current state: %s)", agent.GetState()))
	}

	// Get ACP client and send message
	acpClient := agent.GetACPClient()
	if acpClient == nil {
		s.logger.Printf("Agent has no ACP client: userSession=%s agentID=%s", msg.UserSessionID, msg.AgentID)
		return SendAgentNotReadyError(adapter, s.logger, "Agent ACP client not initialized")
	}

	// Send message to agent with timeout (fixes issue #226)
	// Wrap with explicit timeout to prevent indefinite blocking if agent process hangs
	// Respect parent context deadline if it's shorter than 30s
	acpCtx, acpCancel := contextWithMaxTimeout(ctx, 30*time.Second)
	defer acpCancel()

	agentMsg, err := acpClient.SendMessage(acpCtx, msg.Content)
	if err != nil {
		s.logger.Printf("Agent message failed: %v", err)
		return SendAgentMessageFailedError(adapter, s.logger, err)
	}

	s.logger.Printf("[RELAY] Agent response received: userSession=%s agentID=%s",
		msg.UserSessionID, msg.AgentID)

	responseStr := agentMsg.Content

	// Store both messages in history after successful ACP response
	// Use sessionClock adapter to get time.Time from injected clock
	now := s.sessionClock.Now()
	agent.AddMessage("user", msg.Content, now)
	agent.AddMessage("agent", responseStr, now)

	// Renew lease on meaningful interaction (user message + successful agent response)
	// This extends the lease TTL, preventing expiration during active conversations
	if err := session.RenewLease(msg.AgentID); err != nil {
		s.logger.Printf("[LEASE] Failed to renew lease for agent %s: %v", msg.AgentID, err)
		// Non-fatal: lease expiration will be handled by heartbeat monitor
	}

	// Send agent:response
	responseMsg := NewAgentMessageResponse(
		msg.UserSessionID,
		msg.AgentID,
		responseStr,
		s.clock.Now(),
	)
	if err := adapter.WriteJSON(responseMsg); err != nil {
		s.logger.Printf("Failed to send agent:response: %v", err)
		return true // Close on write failure
	}

	return false // Continue processing messages
}

// handleSessionEnd handles session:end messages
// Terminates all agents in the session and cleans up resources
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleSessionEnd(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) bool {
	// Parse message
	msg, err := parseSessionEndMessage(rawMessage)
	if err != nil {
		return s.handleValidationError(adapter, err)
	}

	// Validate message
	if validationErr := validateSessionEndMessage(msg); validationErr != nil {
		return s.handleValidationError(adapter, validationErr)
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
		if writeErr := adapter.WriteJSON(errorMsg); writeErr != nil {
			s.logger.Printf("Failed to send error response: %v", writeErr)
			return true
		}
		// Keep connection open even on non-recoverable errors - consistent with other handlers
		return false
	}

	s.logger.Printf("[RELAY] User session terminated: %s, agents terminated: %d (failures=%d, cleanup=%s)",
		msg.UserSessionID, summary.AgentsTerminated, summary.AgentFailures, summary.CleanupStatus)

	// Send session:ended response
	response := NewSessionEndedMessage(msg.UserSessionID, summary.AgentsTerminated, string(summary.CleanupStatus))
	if err := adapter.WriteJSON(response); err != nil {
		s.logger.Printf("Failed to send session:ended: %v", err)
		return true
	}

	return false // Continue processing messages
}

// handleAgentTerminate handles agent:terminate messages
// Terminates a specific agent while keeping the session active
// Uses adapter for thread-safe writes (issue #213)
func (s *Server) handleAgentTerminate(ctx context.Context, adapter *SessionWebSocketAdapter, rawMessage []byte) bool {
	// Parse message
	msg, err := parseAgentTerminateMessage(rawMessage)
	if err != nil {
		return s.handleValidationError(adapter, err)
	}

	// Validate message
	if validationErr := validateAgentTerminateMessage(msg); validationErr != nil {
		return s.handleValidationError(adapter, validationErr)
	}

	s.logger.Printf("[RELAY] Terminating agent: userSession=%s agentID=%s", msg.UserSessionID, msg.AgentID)

	// Terminate the agent
	err = s.sessionManager.TerminateAgent(ctx, msg.UserSessionID, msg.AgentID)
	if err != nil {
		s.logger.Printf("Error terminating agent: %v", err)
		// Map error to protocol error code
		// Keep connection open even for non-recoverable errors - consistent with other handlers
		return SendMappedError(adapter, s.logger, err, s.mapError)
	}

	s.logger.Printf("[RELAY] Agent terminated: userSession=%s agentID=%s", msg.UserSessionID, msg.AgentID)

	// Send agent:terminated response
	// Workspace is always cleaned during termination, so workspaceCleaned is true
	response := NewAgentTerminatedMessage(msg.UserSessionID, msg.AgentID, true)
	if err := adapter.WriteJSON(response); err != nil {
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
		maxMessageSize = 1024 * 1024 // 1MB max message size
		readDeadline   = 60 * time.Second
		writeDeadline  = 10 * time.Second
		pingInterval   = 30 * time.Second
	)

	// Set read limit to prevent OOM attacks
	conn.SetReadLimit(maxMessageSize)

	// Set initial read deadline
	if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		s.logger.Printf("Failed to set initial read deadline: %v", err)
	}

	// Set pong handler to extend deadline on activity
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
			s.logger.Printf("Failed to extend read deadline on pong: %v", err)
		}
		return nil
	})

	// Wrap connection in adapter for write synchronization (issue #213)
	// This must be created before any writes to ensure all writes are synchronized
	adapter := &SessionWebSocketAdapter{conn: conn}

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

	// Send handshake using adapter for synchronized writes
	if err := s.sendHandshake(adapter); err != nil {
		return
	}

	ctx := r.Context()

	// Start ping goroutine for liveness checks (issue #215)
	// Use adapter to synchronize ping writes with data writes
	pingDone := s.startPingRoutine(ctx, adapter, pingInterval, writeDeadline)
	defer close(pingDone)

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			s.logger.Printf("Read error: %v", err)
			break
		}

		messageType := extractMessageType(message)
		s.logger.Printf("[RELAY] dir=recv type=%s size=%dB", messageType, len(message))

		sessionID, shouldClose := s.routeMessage(ctx, adapter, message)
		// Track session ID if one was created during this connection
		if sessionID != "" {
			createdSessionID = sessionID
		}
		if shouldClose {
			break
		}
	}
}

// NotifyAgentDeath is called when an agent dies externally (via agentd stop, docker stop, or crash).
// This method looks up the UserSession and pushes an agent:terminated message to the PWA
// so the UI can update to reflect the agent's termination.
//
// This is typically invoked from the HeartbeatMonitor's callback when a lease expires.
// Errors are logged but not returned since this is best-effort notification.
func (s *Server) NotifyAgentDeath(agentID, userSessionID string) {
	s.logger.Printf("[RELAY] Agent death detected: agentID=%s userSession=%s", agentID, userSessionID)

	// Look up the session
	userSession := s.sessionManager.Get(userSessionID)
	if userSession == nil {
		s.logger.Printf("[RELAY] Session not found for dead agent notification: %s", userSessionID)
		return
	}

	// Get the WebSocket connection
	ws := userSession.GetWebSocket()
	if ws == nil {
		s.logger.Printf("[RELAY] No WebSocket for session: %s", userSessionID)
		return
	}

	// Send agent:terminated message to PWA
	// workspaceCleaned is false because we didn't clean the workspace - the agent died externally
	response := NewAgentTerminatedMessage(userSessionID, agentID, false)
	if err := ws.WriteJSON(response); err != nil {
		s.logger.Printf("[RELAY] Failed to send agent:terminated notification: %v", err)
		return
	}

	s.logger.Printf("[RELAY] Sent agent:terminated to PWA: agentID=%s userSession=%s", agentID, userSessionID)
}

// startPingRoutine starts a goroutine that sends periodic pings to keep the connection alive.
// Returns a channel that should be closed when the connection is done to stop the ping routine.
// Uses the adapter to ensure ping writes are synchronized with all other writes (issue #213).
func (s *Server) startPingRoutine(ctx context.Context, adapter *SessionWebSocketAdapter, pingInterval, writeDeadline time.Duration) chan struct{} {
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Set write deadline for this ping only
				if err := adapter.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
					s.logger.Printf("Failed to set write deadline for ping: %v", err)
				}

				// Send ping message (synchronized by adapter's mutex)
				if err := adapter.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					s.logger.Printf("Failed to send ping: %v", err)
					return // Connection dead, exit goroutine
				}

				// Clear deadline to prevent it from affecting subsequent writes
				// SetWriteDeadline(zero) = no deadline
				if err := adapter.SetWriteDeadline(time.Time{}); err != nil {
					s.logger.Printf("Failed to clear write deadline after ping: %v", err)
				}
			case <-ctx.Done():
				return // Request context cancelled
			case <-pingDone:
				return // Connection closed
			}
		}
	}()
	return pingDone
}
