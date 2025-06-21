package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/embedded"
)

func main() {
	fmt.Println("🚀 Starting embedded transport example...")

	// Create a pair of connected embedded transports
	serverTransport, clientTransport := embedded.NewTransportPair(
		embedded.WithBufferSize(50),
		embedded.WithTimeout(5*time.Second),
	)

	// Create MCP server
	mcpServer := server.NewServer("embedded-example")

	// Add a simple tool using the correct API
	mcpServer.Tool("greet", "Greet someone with a personalized message", func(ctx *server.Context, args *struct {
		Name string `json:"name" description:"The name of the person to greet"`
	}) (interface{}, error) {
		if args == nil || args.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		return map[string]interface{}{
			"message": fmt.Sprintf("Hello, %s! Welcome to the embedded MCP server.", args.Name),
		}, nil
	})

	// Add a resource using the correct API
	mcpServer.Resource("/user_data", "User configuration data", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"username": "embedded_user",
			"settings": map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
			},
		}, nil
	})

	// Start the server with the server transport
	fmt.Println("📡 Starting MCP server...")
	go func() {
		if err := mcpServer.Run(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()
	defer mcpServer.Shutdown()

	// Create embedded client transport
	embeddedClient := client.NewEmbeddedTransport(clientTransport)

	// Connect the client
	if err := embeddedClient.Connect(); err != nil {
		log.Fatalf("Failed to connect client: %v", err)
	}
	defer embeddedClient.Disconnect()

	fmt.Println("✅ Server and client started successfully")
	fmt.Println()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	fmt.Println("🔄 Testing communication...")

	// Test 1: Initialize
	fmt.Println("1. Sending initialize request...")
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "embedded-client", "version": "1.0.0"},
			"protocolVersion": "2024-11-05",
		},
	}

	initRequestBytes, _ := json.Marshal(initRequest)
	fmt.Printf("   📤 Sending: %s\n", string(initRequestBytes))

	initResponse, err := embeddedClient.Send(initRequestBytes)
	if err != nil {
		log.Printf("Initialize failed: %v", err)
	} else {
		fmt.Printf("   📥 Response: %s\n", string(initResponse))
	}
	fmt.Println()

	// Test 2: List tools
	fmt.Println("2. Listing tools...")
	toolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	toolsRequestBytes, _ := json.Marshal(toolsRequest)
	fmt.Printf("   📤 Sending: %s\n", string(toolsRequestBytes))

	toolsResponse, err := embeddedClient.Send(toolsRequestBytes)
	if err != nil {
		log.Printf("List tools failed: %v", err)
	} else {
		fmt.Printf("   📥 Response: %s\n", string(toolsResponse))
	}
	fmt.Println()

	// Test 3: Call greet tool
	fmt.Println("3. Calling greet tool...")
	greetRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "greet",
			"arguments": map[string]interface{}{"name": "Alice"},
		},
	}

	greetRequestBytes, _ := json.Marshal(greetRequest)
	fmt.Printf("   📤 Sending: %s\n", string(greetRequestBytes))

	greetResponse, err := embeddedClient.Send(greetRequestBytes)
	if err != nil {
		log.Printf("Call tool failed: %v", err)
	} else {
		fmt.Printf("   📥 Response: %s\n", string(greetResponse))
	}
	fmt.Println()

	// Test 4: List resources
	fmt.Println("4. Listing resources...")
	resourcesRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	}

	resourcesRequestBytes, _ := json.Marshal(resourcesRequest)
	fmt.Printf("   📤 Sending: %s\n", string(resourcesRequestBytes))

	resourcesResponse, err := embeddedClient.Send(resourcesRequestBytes)
	if err != nil {
		log.Printf("List resources failed: %v", err)
	} else {
		fmt.Printf("   📥 Response: %s\n", string(resourcesResponse))
	}
	fmt.Println()

	// Test 5: Read resource
	fmt.Println("5. Reading user_data resource...")
	readRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "resources/read",
		"params":  map[string]interface{}{"uri": "/user_data"},
	}

	readRequestBytes, _ := json.Marshal(readRequest)
	fmt.Printf("   📤 Sending: %s\n", string(readRequestBytes))

	readResponse, err := embeddedClient.Send(readRequestBytes)
	if err != nil {
		log.Printf("Read resource failed: %v", err)
	} else {
		fmt.Printf("   📥 Response: %s\n", string(readResponse))
	}
	fmt.Println()

	fmt.Println("✅ All tests completed successfully!")
	fmt.Println("🎉 Embedded transport demonstration finished")
	fmt.Println()

	// Show transport statistics
	fmt.Printf("📊 Transport Statistics: %+v\n", serverTransport.GetChannelStats())
}
