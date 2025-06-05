// Package main provides a simple stdio transport server example for gomcp
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Check command line arguments to determine mode
	if len(os.Args) > 1 && os.Args[1] == "client" {
		runClient()
		return
	}

	// Default: run as server
	runServer()
}

func runServer() {
	// Create a new server with stdio transport
	srv := server.NewServer("stdio-example-server").AsStdio()

	// Register a simple echo tool
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (map[string]interface{}, error) {
		// Log to stderr only (stdout is reserved for JSON-RPC)
		fmt.Fprintf(os.Stderr, "Server received: %s\n", args.Message)
		return map[string]interface{}{
			"message": args.Message,
		}, nil
	})

	// Register a greeting tool
	srv.Tool("greet", "Generate a greeting message", func(ctx *server.Context, args struct {
		Name string `json:"name"`
	}) (map[string]interface{}, error) {
		greeting := fmt.Sprintf("Hello, %s! Welcome to the stdio example server.", args.Name)
		fmt.Fprintf(os.Stderr, "Generated greeting for: %s\n", args.Name)
		return map[string]interface{}{
			"greeting": greeting,
		}, nil
	})

	// Register a tool with no required fields
	srv.Tool("timestamp", "Get current timestamp", func(ctx *server.Context, args struct {
		Format *string `json:"format,omitempty"`
	}) (map[string]interface{}, error) {
		format := "2006-01-02 15:04:05"
		if args.Format != nil {
			format = *args.Format
		}
		timestamp := time.Now().Format(format)
		return map[string]interface{}{
			"timestamp": timestamp,
		}, nil
	})

	// Run the server (this blocks until stdin is closed)
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func runClient() {
	fmt.Println("=== MCP Stdio Client Example ===")

	// Create a server registry to manage the server process
	registry := client.NewServerRegistry()
	defer func() {
		fmt.Println("Cleaning up server processes...")
		registry.Close()
	}()

	// Define the server configuration - it will run this same binary as server
	serverDef := client.ServerDefinition{
		Command: os.Args[0], // Path to this same executable
		Args:    []string{}, // No "client" argument, so it runs as server
	}

	// Start the server
	fmt.Println("Starting stdio server...")
	err := registry.StartServer("stdio-server", serverDef)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Get the client
	mcpClient, err := registry.GetClient("stdio-server")
	if err != nil {
		log.Fatalf("Failed to get client: %v", err)
	}

	// Wait for the server to be ready
	fmt.Println("Waiting for server to be ready...")
	if err := mcpClient.WaitForReady(5 * time.Second); err != nil {
		log.Fatalf("Server not ready: %v", err)
	}
	fmt.Println("Server is ready!")

	// List available tools
	fmt.Println("\n--- Listing Available Tools ---")
	tools, err := mcpClient.ListTools()
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	for _, tool := range tools {
		fmt.Printf("Tool: %s - %s\n", tool.Name, tool.Description)
	}

	// Test the echo tool
	fmt.Println("\n--- Testing Echo Tool ---")
	result, err := mcpClient.CallTool("echo", map[string]interface{}{
		"message": "Hello from the client!",
	})
	if err != nil {
		log.Fatalf("Failed to call echo tool: %v", err)
	}
	fmt.Printf("Echo result: %+v\n", result)

	// Test the greet tool
	fmt.Println("\n--- Testing Greet Tool ---")
	result, err = mcpClient.CallTool("greet", map[string]interface{}{
		"name": "Alice",
	})
	if err != nil {
		log.Fatalf("Failed to call greet tool: %v", err)
	}
	fmt.Printf("Greet result: %+v\n", result)

	// Test the timestamp tool with default format
	fmt.Println("\n--- Testing Timestamp Tool (default format) ---")
	result, err = mcpClient.CallTool("timestamp", map[string]interface{}{})
	if err != nil {
		log.Fatalf("Failed to call timestamp tool: %v", err)
	}
	fmt.Printf("Timestamp result: %+v\n", result)

	// Test the timestamp tool with custom format
	fmt.Println("\n--- Testing Timestamp Tool (custom format) ---")
	result, err = mcpClient.CallTool("timestamp", map[string]interface{}{
		"format": "Monday, January 2, 2006 at 3:04 PM",
	})
	if err != nil {
		log.Fatalf("Failed to call timestamp tool: %v", err)
	}
	fmt.Printf("Custom timestamp result: %+v\n", result)

	// Demonstrate server capabilities
	fmt.Println("\n--- Server Information ---")
	serverInfo := mcpClient.GetServerInfo()
	if serverInfo != nil {
		fmt.Printf("Server: %s version %s\n", serverInfo.Name, serverInfo.Version)
	}

	capabilities := mcpClient.GetServerCapabilities()
	if capabilities != nil {
		fmt.Printf("Server supports tools: %t\n", capabilities.Tools != nil)
		fmt.Printf("Server supports resources: %t\n", capabilities.Resources != nil)
		fmt.Printf("Server supports prompts: %t\n", capabilities.Prompts != nil)
	}

	fmt.Println("\n--- Example Complete ---")
	fmt.Println("Server will be automatically cleaned up when the program exits.")
}
