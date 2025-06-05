// Package main demonstrates comprehensive ServerRegistry usage for managing multiple MCP servers
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
)

func main() {
	// Check command line arguments to determine mode
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "math-server":
			runMathServer()
			return
		case "text-server":
			runTextServer()
			return
		case "slow-server":
			runSlowServer()
			return
		case "demo":
			runRegistryDemo()
			return
		}
	}

	// Default: show help
	showHelp()
}

func showHelp() {
	fmt.Println("=== MCP ServerRegistry Demo ===")
	fmt.Println()
	fmt.Println("This example demonstrates comprehensive ServerRegistry usage.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run main.go demo              # Run the full ServerRegistry demonstration")
	fmt.Println("  go run main.go math-server       # Run as math server only")
	fmt.Println("  go run main.go text-server       # Run as text server only")
	fmt.Println("  go run main.go slow-server       # Run as slow server only")
	fmt.Println()
	fmt.Println("The 'demo' mode will:")
	fmt.Println("  - Start multiple MCP servers with different capabilities")
	fmt.Println("  - Demonstrate concurrent server management")
	fmt.Println("  - Show error handling and recovery")
	fmt.Println("  - Test process cleanup and lifecycle management")
	fmt.Println("  - Showcase the robust ServerRegistry improvements")
}

func runRegistryDemo() {
	fmt.Println("=== ServerRegistry Comprehensive Demo ===")
	fmt.Println()

	// Create a server registry with logging for visibility
	registry := client.NewServerRegistry(
		client.WithRegistryLogger(client.NewLogger(
			client.WithLogLevel(slog.LevelInfo),
		)),
	)

	// Ensure cleanup happens even if we panic or get interrupted
	defer func() {
		fmt.Println("\n=== Cleaning Up All Servers ===")
		if err := registry.Close(); err != nil {
			fmt.Printf("Error during cleanup: %v\n", err)
		} else {
			fmt.Println("All servers cleaned up successfully!")
		}
	}()

	// Get the path to this executable
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	// Define multiple server configurations
	servers := map[string]client.ServerDefinition{
		"math-server": {
			Command: execPath,
			Args:    []string{"math-server"},
			Env: map[string]string{
				"SERVER_NAME": "Math Operations Server",
			},
		},
		"text-server": {
			Command: execPath,
			Args:    []string{"text-server"},
			Env: map[string]string{
				"SERVER_NAME": "Text Processing Server",
			},
		},
		"slow-server": {
			Command: execPath,
			Args:    []string{"slow-server"},
			Env: map[string]string{
				"SERVER_NAME": "Slow Response Server",
			},
		},
	}

	// Phase 1: Start all servers
	fmt.Println("=== Phase 1: Starting Multiple Servers ===")
	for name, def := range servers {
		fmt.Printf("Starting %s...\n", name)
		if err := registry.StartServer(name, def); err != nil {
			log.Printf("Failed to start %s: %v", name, err)
			continue
		}
		fmt.Printf("✓ %s started successfully\n", name)
	}

	// Phase 2: Wait for all servers to be ready
	fmt.Println("\n=== Phase 2: Waiting for Server Readiness ===")
	readyServers := make(map[string]client.Client)

	for name := range servers {
		clientConn, err := registry.GetClient(name)
		if err != nil {
			log.Printf("Failed to get client for %s: %v", name, err)
			continue
		}

		fmt.Printf("Waiting for %s to be ready...\n", name)
		if err := clientConn.WaitForReady(10 * time.Second); err != nil {
			log.Printf("Server %s not ready: %v", name, err)
			continue
		}

		readyServers[name] = clientConn
		fmt.Printf("✓ %s is ready!\n", name)
	}

	// Phase 3: Discover capabilities
	fmt.Println("\n=== Phase 3: Server Discovery ===")
	for name, clientConn := range readyServers {
		fmt.Printf("\n--- %s Capabilities ---\n", strings.Title(name))

		// Get server info
		if info := clientConn.GetServerInfo(); info != nil {
			fmt.Printf("  Server: %s v%s\n", info.Name, info.Version)
		}

		// List tools
		tools, err := clientConn.ListTools()
		if err != nil {
			log.Printf("Failed to list tools for %s: %v", name, err)
			continue
		}

		fmt.Printf("  Tools (%d):\n", len(tools))
		for _, tool := range tools {
			fmt.Printf("    - %s: %s\n", tool.Name, tool.Description)
		}

		// Check capabilities
		caps := clientConn.GetServerCapabilities()
		if caps != nil {
			fmt.Printf("  Supports: tools=%t resources=%t prompts=%t\n",
				caps.Tools != nil, caps.Resources != nil, caps.Prompts != nil)
		}
	}

	// Phase 4: Test server operations
	fmt.Println("\n=== Phase 4: Testing Server Operations ===")

	// Test math server
	if mathClient, ok := readyServers["math-server"]; ok {
		fmt.Println("\n--- Testing Math Server ---")
		testMathOperations(mathClient)
	}

	// Test text server
	if textClient, ok := readyServers["text-server"]; ok {
		fmt.Println("\n--- Testing Text Server ---")
		testTextOperations(textClient)
	}

	// Test slow server (with timeout)
	if slowClient, ok := readyServers["slow-server"]; ok {
		fmt.Println("\n--- Testing Slow Server ---")
		testSlowOperations(slowClient)
	}

	// Phase 5: Demonstrate server management
	fmt.Println("\n=== Phase 5: Server Management Demo ===")

	// List all servers
	serverNames, err := registry.GetServerNames()
	if err != nil {
		log.Printf("Failed to get server names: %v", err)
	} else {
		fmt.Printf("Active servers: %v\n", serverNames)
	}

	// Stop one server individually
	fmt.Println("Stopping slow-server individually...")
	if err := registry.StopServer("slow-server"); err != nil {
		log.Printf("Failed to stop slow-server: %v", err)
	} else {
		fmt.Println("✓ slow-server stopped successfully")
	}

	// Try to use stopped server (should fail gracefully)
	fmt.Println("Attempting to use stopped server...")
	if _, err := registry.GetClient("slow-server"); err != nil {
		fmt.Printf("✓ Expected error: %v\n", err)
	}

	// Show remaining servers
	if serverNames, err := registry.GetServerNames(); err == nil {
		fmt.Printf("Remaining servers: %v\n", serverNames)
	}

	// Phase 6: Error handling demonstration
	fmt.Println("\n=== Phase 6: Error Handling Demo ===")

	// Try to start a server with invalid command
	fmt.Println("Testing error handling with invalid server...")
	badDef := client.ServerDefinition{
		Command: "/nonexistent/command",
		Args:    []string{},
	}

	if err := registry.StartServer("bad-server", badDef); err != nil {
		fmt.Printf("✓ Expected error caught: %v\n", err)
	}

	// Try to start duplicate server
	fmt.Println("Testing duplicate server detection...")
	if err := registry.StartServer("math-server", servers["math-server"]); err != nil {
		fmt.Printf("✓ Duplicate server error: %v\n", err)
	}

	// Phase 7: Concurrent operations test
	fmt.Println("\n=== Phase 7: Concurrent Operations Test ===")
	testConcurrentOperations(readyServers)

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("All servers will be automatically cleaned up...")
}

func testMathOperations(clientConn client.Client) {
	operations := []struct {
		name string
		args map[string]interface{}
	}{
		{"add", map[string]interface{}{"a": 15, "b": 25}},
		{"multiply", map[string]interface{}{"a": 7, "b": 8}},
		{"factorial", map[string]interface{}{"n": 5}},
	}

	for _, op := range operations {
		result, err := clientConn.CallTool(op.name, op.args)
		if err != nil {
			log.Printf("  %s failed: %v", op.name, err)
			continue
		}
		fmt.Printf("  %s%v = %v\n", op.name, op.args, result)
	}
}

func testTextOperations(clientConn client.Client) {
	operations := []struct {
		name string
		args map[string]interface{}
	}{
		{"uppercase", map[string]interface{}{"text": "hello world"}},
		{"reverse", map[string]interface{}{"text": "MCP Demo"}},
		{"count_words", map[string]interface{}{"text": "This is a test sentence"}},
	}

	for _, op := range operations {
		result, err := clientConn.CallTool(op.name, op.args)
		if err != nil {
			log.Printf("  %s failed: %v", op.name, err)
			continue
		}
		fmt.Printf("  %s(%s) = %v\n", op.name, op.args["text"], result)
	}
}

func testSlowOperations(clientConn client.Client) {
	// Test with timeout
	fmt.Println("  Testing slow operation with timeout...")

	start := time.Now()
	result, err := clientConn.CallTool("slow_process", map[string]interface{}{
		"duration": "2s",
		"message":  "Processing data...",
	}, client.WithRequestTimeoutOption(5*time.Second))

	elapsed := time.Since(start)

	if err != nil {
		log.Printf("  slow_process failed: %v", err)
	} else {
		fmt.Printf("  slow_process completed in %v: %v\n", elapsed, result)
	}
}

func testConcurrentOperations(clients map[string]client.Client) {
	fmt.Println("Running concurrent operations across all servers...")

	done := make(chan string, len(clients)*2)

	// Launch concurrent operations
	for name, clientConn := range clients {
		go func(serverName string, c client.Client) {
			// First operation
			tools, err := c.ListTools()
			if err != nil {
				done <- fmt.Sprintf("%s-list: ERROR %v", serverName, err)
			} else {
				done <- fmt.Sprintf("%s-list: OK (%d tools)", serverName, len(tools))
			}

			// Second operation (ping)
			if err := c.Ping(); err != nil {
				done <- fmt.Sprintf("%s-ping: ERROR %v", serverName, err)
			} else {
				done <- fmt.Sprintf("%s-ping: OK", serverName)
			}
		}(name, clientConn)
	}

	// Collect results
	for i := 0; i < len(clients)*2; i++ {
		result := <-done
		fmt.Printf("  %s\n", result)
	}

	fmt.Println("✓ All concurrent operations completed")
}

// Math server implementation
func runMathServer() {
	srv := server.NewServer("math-server").AsStdio()

	srv.Tool("add", "Add two numbers", func(ctx *server.Context, args struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
	}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": args.A + args.B}, nil
	})

	srv.Tool("multiply", "Multiply two numbers", func(ctx *server.Context, args struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
	}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": args.A * args.B}, nil
	})

	srv.Tool("factorial", "Calculate factorial", func(ctx *server.Context, args struct {
		N int `json:"n"`
	}) (map[string]interface{}, error) {
		if args.N < 0 {
			return nil, fmt.Errorf("factorial not defined for negative numbers")
		}

		result := 1
		for i := 2; i <= args.N; i++ {
			result *= i
		}

		return map[string]interface{}{"result": result}, nil
	})

	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Math server error: %v\n", err)
		os.Exit(1)
	}
}

// Text server implementation
func runTextServer() {
	srv := server.NewServer("text-server").AsStdio()

	srv.Tool("uppercase", "Convert text to uppercase", func(ctx *server.Context, args struct {
		Text string `json:"text"`
	}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": strings.ToUpper(args.Text)}, nil
	})

	srv.Tool("reverse", "Reverse text", func(ctx *server.Context, args struct {
		Text string `json:"text"`
	}) (map[string]interface{}, error) {
		runes := []rune(args.Text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return map[string]interface{}{"result": string(runes)}, nil
	})

	srv.Tool("count_words", "Count words in text", func(ctx *server.Context, args struct {
		Text string `json:"text"`
	}) (map[string]interface{}, error) {
		words := strings.Fields(args.Text)
		return map[string]interface{}{
			"word_count": len(words),
			"words":      words,
		}, nil
	})

	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Text server error: %v\n", err)
		os.Exit(1)
	}
}

// Slow server implementation (for testing timeouts)
func runSlowServer() {
	srv := server.NewServer("slow-server").AsStdio()

	srv.Tool("slow_process", "Simulate slow processing", func(ctx *server.Context, args struct {
		Duration string  `json:"duration"`
		Message  *string `json:"message,omitempty"`
	}) (map[string]interface{}, error) {
		duration, err := time.ParseDuration(args.Duration)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %v", err)
		}

		message := "Processing..."
		if args.Message != nil {
			message = *args.Message
		}

		// Simulate work
		time.Sleep(duration)

		return map[string]interface{}{
			"result":   fmt.Sprintf("Completed: %s (took %v)", message, duration),
			"duration": args.Duration,
		}, nil
	})

	srv.Tool("quick_ping", "Quick response test", func(ctx *server.Context, args struct{}) (map[string]interface{}, error) {
		return map[string]interface{}{"status": "pong", "timestamp": time.Now().Unix()}, nil
	})

	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Slow server error: %v\n", err)
		os.Exit(1)
	}
}
