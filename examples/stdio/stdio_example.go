// Package main provides a simple stdio transport server example for gomcp
package main

import (
	"fmt"
	"os"

	"github.com/localrivet/gomcp/server"
)

func main() {
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

	// Run the server (this blocks until stdin is closed)
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
