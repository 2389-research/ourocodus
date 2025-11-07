package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

// encodeMessageData safely encodes NATS message data for JSON output.
// If data is valid JSON, returns it as json.RawMessage.
// If data is not valid JSON (binary/text), returns base64-encoded string.
func encodeMessageData(data []byte) (interface{}, string) {
	if json.Valid(data) {
		return json.RawMessage(data), "json"
	}
	// Non-JSON data - encode as base64 for safe transport
	return base64.StdEncoding.EncodeToString(data), "base64"
}

func main() {
	// Get NATS URL from environment
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Fatal("NATS_URL environment variable is required")
	}

	// Connect to NATS with auto-reconnect
	nc, err := nats.Connect(natsURL,
		nats.Name("event-logger"),
		nats.MaxReconnects(-1), // Infinite reconnection attempts
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Printf("Disconnected from NATS: %v", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Println("Reconnected to NATS")
		}),
	)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	log.Printf("Connected to NATS at %s", natsURL)

	// Subscribe to all subjects using wildcard
	_, err = nc.Subscribe(">", func(msg *nats.Msg) {
		// Safely encode message data (JSON or base64)
		data, encoding := encodeMessageData(msg.Data)

		// Create log entry with timestamp, subject, data, and encoding type
		logEntry := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"subject":   msg.Subject,
			"data":      data,
			"encoding":  encoding,
		}

		// Write JSON line to stdout
		if err := json.NewEncoder(os.Stdout).Encode(logEntry); err != nil {
			log.Printf("Failed to encode log entry: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to NATS: %v", err)
	}

	log.Println("Event logger started, logging all NATS messages to stdout")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Drain connection gracefully (flushes pending messages)
	if err := nc.Drain(); err != nil {
		log.Printf("Error draining NATS connection: %v", err)
	}

	log.Println("Event logger stopped")
}
