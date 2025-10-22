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

	"github.com/2389-research/ourocodus/pkg/relay"
)

const (
	port            = 8080
	shutdownTimeout = 10 * time.Second
)

func main() {
	// Create relay server
	server := relay.NewServer()

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.HandleWebSocket)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Relay server starting on port %d", port)
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

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		os.Exit(1)
	}

	log.Println("Server stopped")
}
