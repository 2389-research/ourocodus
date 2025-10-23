package relay

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development (Phase 1)
		return true
	},
}

// Server handles WebSocket connections
type Server struct {
	serverID string
}

// NewServer creates a new relay server
func NewServer() *Server {
	return &Server{
		serverID: uuid.New().String(),
	}
}

// HandleWebSocket handles WebSocket upgrade and connection lifecycle
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WebSocket connection established from %s", r.RemoteAddr)

	// Send connection:established message
	handshake := NewConnectionEstablished(s.serverID)
	if err := conn.WriteJSON(handshake); err != nil {
		log.Printf("Failed to send handshake: %v", err)
		return
	}

	// Handle incoming messages
	for {
		// Read message
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}

		// Validate message
		if err := ValidateMessage(message); err != nil {
			log.Printf("Invalid message: %v", err)

			// Send error response
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

			errorMsg := NewErrorMessage(validationErr.Code, validationErr.Message, validationErr.Recoverable)
			if err := conn.WriteJSON(errorMsg); err != nil {
				log.Printf("Failed to send error response: %v", err)
			}

			// Close connection if error is not recoverable
			if !validationErr.Recoverable {
				log.Printf("Closing connection due to non-recoverable error: %s", validationErr.Code)
				break
			}

			continue
		}

		// Parse message as generic JSON
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Add timestamp
		msg["timestamp"] = time.Now().UTC().Format(time.RFC3339)

		// Echo back
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("Write error: %v", err)
			break
		}
	}
}
