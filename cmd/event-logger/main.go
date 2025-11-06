package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

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
		// Create log entry with timestamp, subject, and raw data
		logEntry := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"subject":   msg.Subject,
			"data":      json.RawMessage(msg.Data),
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
