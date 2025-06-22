package main

import (
	"fmt"
	"os"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a simple debug server with proper logging
	srv := server.NewServer("debug-server").
		AsStdio("debug_server.log").
		Tool("echo", "Echo back the input message", func(ctx *server.Context, args struct {
			Message string `json:"message"`
		}) (map[string]interface{}, error) {
			// Log to file (not stdout/stderr)
			fmt.Fprintf(os.Stderr, "[DEBUG] Echo tool called with message: %s\n", args.Message)

			return map[string]interface{}{
				"echoed":    args.Message,
				"timestamp": "2025-01-22T10:00:00Z",
			}, nil
		}).
		Tool("test", "Simple test tool", func(ctx *server.Context, args struct{}) (string, error) {
			fmt.Fprintf(os.Stderr, "[DEBUG] Test tool called\n")
			return "test successful", nil
		})

	// Run the server
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
