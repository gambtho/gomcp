// Package main provides a server example for MCP cancellation functionality.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/localrivet/gomcp/server"
)

// longRunningTool simulates a task that takes a long time to complete.
// It checks for cancellation periodically during execution.
func longRunningTool(ctx *server.Context, args struct {
	Duration int `json:"duration" required:"true"` // Duration in seconds
}) (string, error) {
	// Validate duration
	if args.Duration <= 0 {
		return "", fmt.Errorf("duration must be positive")
	}

	ctx.Logger.Info("Starting long-running task", "duration", args.Duration)

	// Register for cancellation
	cancelCh := ctx.RegisterForCancellation()

	// Simulate work with periodic cancellation checks
	for i := 0; i < args.Duration; i++ {
		// Check for cancellation using multiple methods

		// Method 1: Using the convenience method
		if err := ctx.CheckCancellation(); err != nil {
			ctx.Logger.Info("Task cancelled (using CheckCancellation)")
			return "", fmt.Errorf("task cancelled after %d seconds", i)
		}

		// Method 2: Using the cancelCh directly
		select {
		case <-cancelCh:
			ctx.Logger.Info("Task cancelled (using cancel channel)")
			return "", fmt.Errorf("task cancelled after %d seconds", i)
		default:
			// Not cancelled, continue work
		}

		// Do some "work"
		ctx.Logger.Info("Working...", "progress", fmt.Sprintf("%d/%d seconds completed", i+1, args.Duration))
		time.Sleep(1 * time.Second)
	}

	ctx.Logger.Info("Task completed successfully!")
	return fmt.Sprintf("Completed task that took %d seconds", args.Duration), nil
}

// sendCancellation demonstrates cancelling a request
func sendCancellation(srv server.Server, requestID string) {
	// Get the server implementation
	s := srv.GetServer()

	// Wait a moment to let the request start processing
	time.Sleep(2 * time.Second)

	// Send the cancellation notification
	srv.Logger().Info("Sending cancellation notification...")
	err := s.SendCancelledNotification(requestID, "User requested cancellation")
	if err != nil {
		fmt.Printf("Error sending cancellation: %v\n", err)
	}
}

func main() {
	// Create a new server
	srv := server.NewServer("cancellation-example-server")

	// Configure the server with stdio transport
	srv.AsStdio("logs/mcp-server.log")

	// Register a long-running tool
	srv.Tool("longRunningTask", "Simulates a task that takes a long time to complete", longRunningTool)

	// Prepare a tool call request that will start a long task (10 seconds)
	toolCallRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "12345",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "longRunningTask",
			"arguments": map[string]interface{}{
				"duration": 10,
			},
		},
	}

	// Convert to JSON
	requestBytes, _ := json.Marshal(toolCallRequest)

	// Start a goroutine to send cancellation after a delay
	go sendCancellation(srv, "12345")

	// Directly handle the message (simulating a client request)
	impl := srv.GetServer()
	responseBytes, err := server.HandleMessage(impl, requestBytes)
	if err != nil {
		log.Fatalf("Error handling message: %v", err)
	}

	// Print the response
	var response map[string]interface{}
	json.Unmarshal(responseBytes, &response)
	impl.Logger().Info("Response received:")
	prettyJSON, _ := json.MarshalIndent(response, "", "  ")
	impl.Logger().Info(string(prettyJSON))

	// Also demonstrate cancellation in the real server
	impl.Logger().Info("Starting real server example...")
	if err := srv.Run(); err != nil {
		impl.Logger().Error("Server error", "error", err)
		os.Exit(1)
	}
}
