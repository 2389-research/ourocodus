package helpers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// WSClient wraps a WebSocket connection with helper methods
type WSClient struct {
	conn *websocket.Conn
}

// Connect establishes a WebSocket connection to the given URL
func Connect(url string) (*WSClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", url, err)
	}
	return &WSClient{conn: conn}, nil
}

// Send sends a JSON message over the WebSocket
func (c *WSClient) Send(message interface{}) error {
	if err := c.conn.WriteJSON(message); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}

// Receive reads a JSON message from the WebSocket with a timeout
func (c *WSClient) Receive(v interface{}, timeout time.Duration) error {
	// Set read deadline
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read message
	if err := c.conn.ReadJSON(v); err != nil {
		return fmt.Errorf("failed to receive message: %w", err)
	}

	// Clear deadline for next read
	if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
		return fmt.Errorf("failed to clear read deadline: %w", err)
	}

	return nil
}

// ReceiveRaw reads a raw message from the WebSocket with a timeout
func (c *WSClient) ReceiveRaw(timeout time.Duration) ([]byte, error) {
	// Set read deadline
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read message
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to receive raw message: %w", err)
	}

	// Clear deadline for next read
	if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("failed to clear read deadline: %w", err)
	}

	return message, nil
}

// ReceiveWithFilter reads messages until one matches the filter function
func (c *WSClient) ReceiveWithFilter(filter func(map[string]interface{}) bool, timeout time.Duration) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		var msg map[string]interface{}
		remaining := time.Until(deadline)
		if remaining < 0 {
			remaining = 0
		}

		if err := c.Receive(&msg, remaining); err != nil {
			return nil, err
		}

		if filter(msg) {
			return msg, nil
		}
	}

	return nil, fmt.Errorf("timeout waiting for matching message")
}

// Close closes the WebSocket connection
func (c *WSClient) Close() error {
	if err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		return fmt.Errorf("failed to send close message: %w", err)
	}
	return c.conn.Close()
}

// SendMessage is a convenience function to send a message with type and data
func (c *WSClient) SendMessage(msgType string, data interface{}) error {
	msg := map[string]interface{}{
		"type": msgType,
	}

	// Add data fields
	if data != nil {
		dataMap, ok := data.(map[string]interface{})
		if ok {
			for k, v := range dataMap {
				msg[k] = v
			}
		} else {
			msg["data"] = data
		}
	}

	return c.Send(msg)
}

// WaitForMessageType waits for a message with the specified type
func (c *WSClient) WaitForMessageType(msgType string, timeout time.Duration) (map[string]interface{}, error) {
	return c.ReceiveWithFilter(func(msg map[string]interface{}) bool {
		if t, ok := msg["type"].(string); ok {
			return t == msgType
		}
		return false
	}, timeout)
}

// ParseJSON is a helper to parse JSON strings
func ParseJSON(data string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}
