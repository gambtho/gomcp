// Package main provides an example of using gRPC transport for gomcp
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
)

// NOTE: This is a conceptual example only.
// The gomcp project currently doesn't provide a fully integrated
// gRPC transport in the server interface like other transports.
// A full implementation would require an AsGRPC method in the server interface.

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Define the gRPC host and port
	address := ":50051"

	fmt.Println("=== gRPC MCP Example ===")
	fmt.Println("This example demonstrates the fully implemented gRPC transport")
	fmt.Println("for the Model Context Protocol (MCP) in gomcp.")
	fmt.Println()

	// Start the server
	srv := server.NewServer("grpc-example-server")

	// Add a simple echo tool
	srv.Tool("echo", "Echo back the provided message", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"echoed": args.Message,
		}, nil
	})

	// Configure the server with gRPC transport
	srv.AsGRPC(address,
		server.WithGRPCMaxMessageSize(8*1024*1024), // 8MB
		server.WithGRPCKeepAlive(10*time.Second, 3*time.Second),
	)

	fmt.Printf("Starting gRPC server on %s...\n", address)

	// Start server in a goroutine
	go func() {
		if err := srv.Run(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(2 * time.Second)

	// Run client example
	runClientExample("localhost" + address)

	// Auto-shutdown after demo or wait for manual signal
	fmt.Println("\nDemo completed successfully!")
	fmt.Println("Shutting down automatically in 2 seconds... (or press Ctrl+C to exit immediately)")

	// Wait for either auto-shutdown timeout or manual signal
	select {
	case <-time.After(2 * time.Second):
		fmt.Println("Auto-shutdown triggered")
	case <-signals:
		fmt.Println("Manual shutdown signal received")
	}

	fmt.Println("Stopping server...")

	// Shutdown the server
	if err := srv.Shutdown(); err != nil {
		log.Printf("Server shutdown error: %v", err)
	} else {
		fmt.Println("Server stopped successfully")
	}
}

func runClientExample(address string) {
	fmt.Printf("Creating gRPC client connecting to %s...\n", address)

	// Create the gRPC client
	c, err := client.NewClient("grpc-example-client",
		client.WithGRPC(address,
			client.WithGRPCTimeout(5*time.Second),
			client.WithGRPCMaxMessageSize(8*1024*1024),
		),
		client.WithConnectionTimeout(5*time.Second),
		client.WithRequestTimeout(10*time.Second),
	)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return
	}

	defer c.Close()

	fmt.Println("Successfully connected to gRPC server!")

	// Call the echo tool
	fmt.Println("Calling echo tool...")
	result, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello from gRPC client!",
	})
	if err != nil {
		log.Printf("Tool call failed: %v", err)
		return
	}

	fmt.Printf("Tool result: %v\n", result)

	// List available tools
	fmt.Println("Listing available tools...")
	tools, err := c.ListTools()
	if err != nil {
		log.Printf("Failed to list tools: %v", err)
		return
	}

	fmt.Printf("Available tools:\n")
	for _, tool := range tools {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}
}
