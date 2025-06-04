// Package main provides a simple example of using the Streamable HTTP transport for gomcp
// This example demonstrates how to set up both a server and client using HTTP transport
// following the MCP 2025-03-26 specification for streamable HTTP.
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
)

func main() {
	fmt.Println("=== Streamable HTTP Transport Example ===")
	fmt.Println()

	// finds a random available port on localhost
	// EXAMPLE ONLY, DO NOT USE THIS IN PRODUCTION
	port := func() string {
		// Listen on port 0 to get a random available port
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			log.Fatalf("Failed to find available port: %v", err)
		}
		defer listener.Close()

		// Extract the port number from the address
		port := listener.Addr().(*net.TCPAddr).Port
		return fmt.Sprintf("%d", port)
	}()

	// Start the HTTP server in a goroutine so it runs concurrently
	// The server will listen on localhost:PORT with the MCP endpoint at /mcp
	go startServer(port)

	// Give the server a moment to start up and begin listening
	time.Sleep(1 * time.Second)

	// Run the client to test the server functionality
	// This will connect to the server and call some tools
	runClient(port)
}

// startServer creates and runs an HTTP MCP server
// The server listens on localhost:PORT and provides tools via the /mcp endpoint
func startServer(port string) {
	fmt.Println("Starting server...")

	// Create a new MCP server instance
	srv := server.NewServer("http-example-server")

	// Start the HTTP server on a random available port
	// This creates the endpoint: http://localhost:PORT/mcp
	srv.AsHTTP("localhost:" + port)
	// With custom paths using options:
	// srv.AsHTTP("localhost:"+port, http.WithPathPrefix("/api/v1"), http.WithMCPEndpoint("/mcp"))

	// Register an echo tool that returns the input message with a timestamp
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (map[string]interface{}, error) {
		fmt.Printf("Server received: %s\n", args.Message)
		return map[string]interface{}{
			"echo":      args.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		}, nil
	})

	// Start the server and block until it shuts down
	if err := srv.Run(); err != nil {
		log.Printf("Server error: %v", err)
	}
}

// runClient creates an HTTP client and tests the server's tools
// This demonstrates how to connect to and interact with an HTTP MCP server
func runClient(ipAddress string) {
	fmt.Println("Connecting client...")

	// Create a new MCP client that connects to the HTTP server
	// The URL must include the /mcp endpoint path
	c, err := client.NewClient("http://localhost:" + ipAddress + "/mcp")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	fmt.Printf("Connected! Protocol version: %s\n", c.Version())

	// Test the echo tool by sending a message
	echoResult, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello from HTTP client!",
	})
	if err != nil {
		log.Fatalf("Echo failed: %v", err)
	}
	fmt.Printf("Echo result: %v\n", echoResult)

	fmt.Println("✅ Example completed successfully!")
}
