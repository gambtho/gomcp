package main

import (
	"errors"
	"log"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// must only log to a file, not stdout
	// logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// Create a comprehensive MCP server example
	srv := server.NewServer("comprehensive-example").
		AsStdio("logs/comprehensive.log").
		Root("/api", "/files")

	logger := srv.Logger()

	// Tool: Calculator with multiple operations
	// Note: This demonstrates inline struct definition usage
	// The validation system will handle field extraction and type conversion
	srv.Tool("calculator", "Perform mathematical calculations", func(ctx *server.Context, args struct {
		Operation string  `json:"operation"`
		A         float64 `json:"a"`
		B         float64 `json:"b"`
	}) (interface{}, error) {
		// For demonstration, we'll show basic operations
		switch args.Operation {
		case "add":
			return map[string]interface{}{
				"result":    args.A + args.B,
				"operation": "addition",
			}, nil
		case "subtract":
			return map[string]interface{}{
				"result":    args.A - args.B,
				"operation": "subtraction",
			}, nil
		case "multiply":
			return map[string]interface{}{
				"result":    args.A * args.B,
				"operation": "multiplication",
			}, nil
		case "divide":
			if args.B == 0 {
				return nil, errors.New("division by zero")
			}
			return map[string]interface{}{
				"result":    args.A / args.B,
				"operation": "division",
			}, nil
		default:
			return nil, errors.New("unsupported operation: " + args.Operation)
		}
	})

	// Resource: User management with path parameter
	// Demonstrates URI template parameter extraction
	srv.Resource("/api/users/{id}", "Get or update user information", func(ctx *server.Context, args *struct {
		ID   string `path:"id"`   // Extracted from URI template
		Name string `json:"name"` // From request body (for updates)
		Age  int    `json:"age"`  // From request body (for updates)
	}) (interface{}, error) {
		// Simulate user data retrieval/update
		return map[string]interface{}{
			"user": map[string]interface{}{
				"id":   args.ID,
				"name": args.Name,
				"age":  args.Age,
			},
			"message": "User processed successfully",
		}, nil
	})

	// Resource: File access with path parameter
	// Demonstrates file path handling
	srv.Resource("/files/{path}", "Access file system resources", func(ctx *server.Context, args *struct {
		Path string `path:"path"` // File path from URI
	}) (interface{}, error) {
		// Simulate file access (in real implementation, add proper security checks)
		return map[string]interface{}{
			"file": map[string]interface{}{
				"path":   args.Path,
				"exists": true, // Simulated
				"size":   1024, // Simulated
			},
		}, nil
	})

	// Prompt: Professional email generation
	// Demonstrates prompt template functionality
	srv.Prompt("professional_email", "Generate professional email content",
		server.Assistant("I'll be happy to help you with that."),
		server.User("Hello, {{name}}! How are you today?"),
	)

	// Log startup message
	logger.Info("Starting comprehensive MCP server example...")
	logger.Info("Features:")
	logger.Info("  - Tool: calculator (math operations)")
	logger.Info("  - Resource: /api/users/{id} (user management)")
	logger.Info("  - Resource: /files/{path} (file access)")
	logger.Info("  - Prompt: professional_email (email generation)")
	logger.Info("This example demonstrates the server.Schema type usage.")
	logger.Info("The validation system will handle field extraction and type conversion.")
	logger.Info("Logs will be written to: logs/comprehensive.log")
	logger.Info("Send JSON-RPC messages to interact with the server.")

	// Run the server
	if err := srv.Run(); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
