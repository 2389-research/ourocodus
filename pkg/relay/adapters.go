package relay

import (
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// StdLogger wraps Go's standard logger
type StdLogger struct{}

func (l *StdLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// SystemClock returns current system time
type SystemClock struct{}

func (c *SystemClock) Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// UUIDGenerator generates UUIDs
type UUIDGenerator struct{}

func (g *UUIDGenerator) Generate() string {
	return uuid.New().String()
}

// GorillaUpgrader wraps gorilla websocket upgrader
type GorillaUpgrader struct {
	upgrader websocket.Upgrader
}

func NewGorillaUpgrader(checkOrigin func(*http.Request) bool) *GorillaUpgrader {
	return &GorillaUpgrader{
		upgrader: websocket.Upgrader{
			CheckOrigin: checkOrigin,
		},
	}
}

func (u *GorillaUpgrader) Upgrade(w interface{}, r interface{}, responseHeader interface{}) (WebSocketConn, error) {
	httpW, ok := w.(http.ResponseWriter)
	if !ok {
		panic("w must be http.ResponseWriter")
	}
	httpR, ok := r.(*http.Request)
	if !ok {
		panic("r must be *http.Request")
	}

	var headers http.Header
	if responseHeader != nil {
		headers, ok = responseHeader.(http.Header)
		if !ok {
			panic("responseHeader must be http.Header")
		}
	}

	return u.upgrader.Upgrade(httpW, httpR, headers)
}
