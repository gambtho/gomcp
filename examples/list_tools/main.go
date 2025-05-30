package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a simple logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	fmt.Println("🔧 MCP Tool Discovery Example")
	fmt.Println("=============================")
	fmt.Println()

	// Create a simple MCP server with some tools for demonstration
	srv := server.NewServer("list-tools-demo").
		Tool("calculator", "Perform basic mathematical operations", func(ctx *server.Context, args struct {
			Operation string  `json:"operation" description:"The operation to perform (add, subtract, multiply, divide)"`
			A         float64 `json:"a" description:"First number"`
			B         float64 `json:"b" description:"Second number"`
		}) (float64, error) {
			switch args.Operation {
			case "add":
				return args.A + args.B, nil
			case "subtract":
				return args.A - args.B, nil
			case "multiply":
				return args.A * args.B, nil
			case "divide":
				if args.B == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				return args.A / args.B, nil
			default:
				return 0, fmt.Errorf("unknown operation: %s", args.Operation)
			}
		}).
		Tool("echo", "Echo back the provided text", func(ctx *server.Context, args struct {
			Text string `json:"text" description:"The text to echo back"`
		}) (string, error) {
			return fmt.Sprintf("Echo: %s", args.Text), nil
		}).
		Tool("random", "Generate a random number between min and max", func(ctx *server.Context, args struct {
			Min int `json:"min" description:"Minimum value"`
			Max int `json:"max" description:"Maximum value"`
		}) (int, error) {
			// Simple demonstration - in real use you'd use proper random generation
			return (args.Min + args.Max) / 2, nil // Just return the average for demo
		}).
		Tool("greet", "Generate a personalized greeting", func(ctx *server.Context, args struct {
			Name     string `json:"name" description:"The person's name"`
			Language string `json:"language,omitempty" description:"Language for greeting (en, es, fr)"`
		}) (string, error) {
			switch args.Language {
			case "es":
				return fmt.Sprintf("¡Hola, %s!", args.Name), nil
			case "fr":
				return fmt.Sprintf("Bonjour, %s!", args.Name), nil
			default:
				return fmt.Sprintf("Hello, %s!", args.Name), nil
			}
		}).
		AsStdio("logs/list-tools-demo.log")

	// Start the server in a separate process (this would normally be done separately)
	// For this demo, we'll simulate connecting to an existing server
	go func() {
		if err := srv.Run(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Give the server a moment to start
	// In real usage, the server would already be running
	fmt.Println("Starting demo server...")
	fmt.Println()

	// Create a client to connect to our server
	c, err := client.NewClient("list-tools-client",
		client.WithLogger(logger),
		client.WithProtocolVersion("2025-03-26"),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Demonstrate the ListTools functionality
	fmt.Println("📋 Discovering available tools...")
	fmt.Println()

	tools, err := c.ListTools()
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	fmt.Printf("Found %d tools:\n\n", len(tools))

	for i, tool := range tools {
		fmt.Printf("%d. 🛠️  %s\n", i+1, tool.Name)
		fmt.Printf("   📝 Description: %s\n", tool.Description)

		if tool.InputSchema != nil {
			fmt.Printf("   📋 Input Schema:\n")
			if schemaType, ok := tool.InputSchema["type"].(string); ok {
				fmt.Printf("      Type: %s\n", schemaType)
			}
			if properties, ok := tool.InputSchema["properties"].(map[string]interface{}); ok {
				fmt.Printf("      Parameters:\n")
				for propName, propDef := range properties {
					if propMap, ok := propDef.(map[string]interface{}); ok {
						propType := "unknown"
						propDesc := ""
						if t, ok := propMap["type"].(string); ok {
							propType = t
						}
						if d, ok := propMap["description"].(string); ok {
							propDesc = fmt.Sprintf(" - %s", d)
						}
						fmt.Printf("        • %s (%s)%s\n", propName, propType, propDesc)
					}
				}
			}
			if required, ok := tool.InputSchema["required"].([]interface{}); ok && len(required) > 0 {
				fmt.Printf("      Required: ")
				for i, req := range required {
					if i > 0 {
						fmt.Printf(", ")
					}
					fmt.Printf("%v", req)
				}
				fmt.Printf("\n")
			}
		}

		if tool.Annotations != nil && len(tool.Annotations) > 0 {
			fmt.Printf("   🏷️  Annotations: %+v\n", tool.Annotations)
		}

		fmt.Println()
	}

	fmt.Println("✅ Tool discovery complete!")
	fmt.Println()
	fmt.Println("💡 Use Case: LLM Integration")
	fmt.Println("This demonstrates how LLM client libraries (like OpenAI's Go client)")
	fmt.Println("can use client.ListTools() to discover available MCP tools and their")
	fmt.Println("schemas for function calling. The discovered tools can be converted")
	fmt.Println("to OpenAI function definitions for seamless integration.")
	fmt.Println()

	// Show how this could be used with OpenAI-style function definitions
	fmt.Println("📄 Example: Converting to OpenAI Function Definitions")
	fmt.Println()

	for _, tool := range tools {
		openAIFunc := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.InputSchema,
		}

		funcJSON, _ := json.MarshalIndent(openAIFunc, "", "  ")
		fmt.Printf("// %s\n%s\n\n", tool.Name, funcJSON)

		// Only show first tool to keep output manageable
		break
	}

	fmt.Println("🎯 This enables proxy patterns where MCP servers expose tools")
	fmt.Println("   that can be automatically discovered and used by LLM clients!")
}
