package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/2389-research/ourocodus/pkg/nats"
	"github.com/2389-research/ourocodus/pkg/relay"
)

const (
	port            = 8080
	shutdownTimeout = 10 * time.Second
)

func main() {
	// Create dependencies
	logger := &relay.StdLogger{}
	clock := &relay.SystemClock{}
	idGen := &relay.UUIDGenerator{}

	// Initialize NATS client if NATS_URL is set
	var natsClient nats.Client
	natsURL := os.Getenv("NATS_URL")
	if natsURL != "" {
		log.Printf("Connecting to NATS at %s...", natsURL)

		var err error
		natsClient, err = nats.NewClient(
			nats.WithURL(natsURL),
			nats.WithName("ourocodus-relay"),
		)
		if err != nil {
			log.Fatalf("Failed to connect to NATS: %v", err)
		}
		log.Printf("Connected to NATS successfully")
	} else {
		log.Printf("NATS_URL not set, event publishing disabled")
	}

	// Create session manager (with optional NATS client, nil launcher factory for now)
	sessionManager, err := relay.NewSessionManager(logger, clock, idGen, natsClient, nil)
	if err != nil {
		log.Fatalf("Failed to create session manager: %v", err)
	}

	// Create relay server with dependency injection
	server := relay.NewServer(
		idGen,
		logger,
		clock,
		relay.NewGorillaUpgrader(func(r *http.Request) bool {
			// Allow all origins for development (Phase 1)
			return true
		}),
		sessionManager,
	)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.HandleWebSocket)

	// Serve PWA static files from web directory
	fs := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fs)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	// Start server in goroutine
	go func() {
		log.Printf("Relay server starting on port %d", port)
		log.Printf("PWA available at: http://localhost:%d/", port)
		log.Printf("WebSocket endpoint: ws://localhost:%d/ws", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutdown signal received, gracefully stopping server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Attempt graceful HTTP server shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		os.Exit(1)
	}

	// Drain NATS connection if available
	if natsClient != nil {
		log.Println("Draining NATS connection...")
		if err := natsClient.Drain(ctx); err != nil {
			log.Printf("NATS drain error: %v", err)
		} else {
			log.Println("NATS connection drained successfully")
		}
	}

	log.Println("Server stopped")
}
