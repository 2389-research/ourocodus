package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/2389-research/ourocodus/pkg/nats"
)

func main() {
	// Create a new NATS client
	client, err := nats.NewClient(
		nats.WithURL("nats://localhost:4222"),
		nats.WithName("basic-example"),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Example 1: Simple publish
	fmt.Println("Publishing message...")
	if err := client.Publish(ctx, "demo.subject", []byte("Hello, NATS!")); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}
	fmt.Println("Message published successfully")

	// Example 2: Subscribe to messages
	fmt.Println("\nSubscribing to messages...")
	sub, err := client.Subscribe(ctx, "demo.subject", func(ctx context.Context, msg *nats.Message) error {
		fmt.Printf("Received message on %s: %s (correlation ID: %s)\n",
			msg.Subject, string(msg.Data), msg.CorrelationID)
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Stop(ctx)

	// Publish a message that will be received by our subscriber
	if err := client.Publish(ctx, "demo.subject", []byte("Test message")); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	// Wait a bit for the message to be received
	time.Sleep(100 * time.Millisecond)

	// Example 3: Request/Reply
	fmt.Println("\nTesting request/reply...")

	// First, create a responder
	responder, err := client.Subscribe(ctx, "demo.request", func(ctx context.Context, msg *nats.Message) error {
		fmt.Printf("Responder received request: %s\n", string(msg.Data))
		return msg.Respond([]byte("Response from responder"))
	})
	if err != nil {
		log.Fatalf("Failed to create responder: %v", err)
	}
	defer responder.Stop(ctx)

	// Give the responder time to start
	time.Sleep(100 * time.Millisecond)

	// Send a request
	resp, err := client.Request(ctx, "demo.request", []byte("Request data"))
	if err != nil {
		log.Fatalf("Failed to send request: %v", err)
	}
	fmt.Printf("Received response: %s\n", string(resp.Data))

	// Example 4: Check health
	fmt.Println("\nChecking client health...")
	health := client.Health()
	fmt.Printf("Connected: %v\n", health.Connected)
	if health.LastError != nil {
		fmt.Printf("Last error: %v (at %v)\n", health.LastError, health.LastErrorTime)
	}

	// Check if ready
	if err := client.Ready(); err != nil {
		fmt.Printf("Client not ready: %v\n", err)
	} else {
		fmt.Println("Client is ready")
	}

	fmt.Println("\nExample completed successfully!")
}
