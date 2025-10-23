package relay

import "github.com/gorilla/websocket"

// Logger abstracts logging operations
type Logger interface {
	Printf(format string, v ...interface{})
}

// Clock abstracts time operations
type Clock interface {
	Now() string // Returns RFC3339 formatted timestamp
}

// IDGenerator abstracts unique ID generation
type IDGenerator interface {
	Generate() string
}

// WebSocketConn abstracts websocket connection operations
type WebSocketConn interface {
	WriteJSON(v interface{}) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

// Upgrader abstracts WebSocket upgrade operations
type Upgrader interface {
	Upgrade(w interface{}, r interface{}, responseHeader interface{}) (WebSocketConn, error)
}

// Ensure gorilla websocket implements our interface
var _ WebSocketConn = (*websocket.Conn)(nil)
