package embedded

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestJSONRPCIntegration demonstrates full JSON-RPC 2.0 communication over embedded transport
func TestJSONRPCIntegration(t *testing.T) {
	fmt.Println("🚀 Starting JSON-RPC Integration Test...")

	// Create transport pair
	serverTransport, clientTransport := NewTransportPair()

	// Set up server-side message handler that simulates MCP server responses
	serverTransport.SetMessageHandler(func(message []byte) ([]byte, error) {
		var request map[string]interface{}
		if err := json.Unmarshal(message, &request); err != nil {
			return nil, err
		}

		method, _ := request["method"].(string)
		id := request["id"]
		params, _ := request["params"].(map[string]interface{})

		fmt.Printf("   📨 Server received: %s (id: %v)\n", method, id)

		var response map[string]interface{}

		switch method {
		case "initialize":
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"protocolVersion": "2025-03-26",
					"capabilities": map[string]interface{}{
						"tools":     map[string]interface{}{},
						"resources": map[string]interface{}{},
						"prompts":   map[string]interface{}{},
					},
					"serverInfo": map[string]interface{}{
						"name":    "embedded-test-server",
						"version": "1.0.0",
					},
				},
			}

		case "tools/list":
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "greet",
							"description": "Greet someone with a personalized message",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{
										"type":        "string",
										"description": "The name of the person to greet",
									},
								},
								"required": []string{"name"},
							},
						},
						{
							"name":        "calculate",
							"description": "Perform mathematical calculations",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"operation": map[string]interface{}{
										"type": "string",
										"enum": []string{"add", "subtract", "multiply", "divide"},
									},
									"a": map[string]interface{}{"type": "number"},
									"b": map[string]interface{}{"type": "number"},
								},
								"required": []string{"operation", "a", "b"},
							},
						},
					},
				},
			}

		case "tools/call":
			toolName, _ := params["name"].(string)
			arguments, _ := params["arguments"].(map[string]interface{})

			var result map[string]interface{}
			switch toolName {
			case "greet":
				name, _ := arguments["name"].(string)
				result = map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": fmt.Sprintf("Hello, %s! Welcome to the embedded MCP server.", name),
						},
					},
				}
			case "calculate":
				operation, _ := arguments["operation"].(string)
				a, _ := arguments["a"].(float64)
				b, _ := arguments["b"].(float64)
				var calcResult float64
				switch operation {
				case "add":
					calcResult = a + b
				case "subtract":
					calcResult = a - b
				case "multiply":
					calcResult = a * b
				case "divide":
					if b != 0 {
						calcResult = a / b
					} else {
						return json.Marshal(map[string]interface{}{
							"jsonrpc": "2.0",
							"id":      id,
							"error": map[string]interface{}{
								"code":    -1,
								"message": "Division by zero",
							},
						})
					}
				}
				result = map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": fmt.Sprintf("%.2f %s %.2f = %.2f", a, operation, b, calcResult),
						},
					},
				}
			}

			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result":  result,
			}

		case "resources/list":
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"resources": []map[string]interface{}{
						{
							"uri":         "/config",
							"name":        "Configuration",
							"description": "Server configuration data",
							"mimeType":    "application/json",
						},
						{
							"uri":         "/status",
							"name":        "Status",
							"description": "Current server status",
							"mimeType":    "text/plain",
						},
					},
				},
			}

		case "resources/read":
			uri, _ := params["uri"].(string)
			var content string
			switch uri {
			case "/config":
				content = `{"server": "embedded-test-server", "version": "1.0.0", "features": ["tools", "resources", "prompts"]}`
			case "/status":
				content = "Server is running normally"
			default:
				return json.Marshal(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"error": map[string]interface{}{
						"code":    -32602,
						"message": "Resource not found",
					},
				})
			}

			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"contents": []map[string]interface{}{
						{
							"uri":      uri,
							"mimeType": "text/plain",
							"text":     content,
						},
					},
				},
			}

		case "prompts/list":
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"prompts": []map[string]interface{}{
						{
							"name":        "code_review",
							"description": "Generate a code review prompt",
							"arguments": []map[string]interface{}{
								{
									"name":        "language",
									"description": "Programming language",
									"required":    true,
								},
								{
									"name":        "complexity",
									"description": "Code complexity level",
									"required":    false,
								},
							},
						},
					},
				},
			}

		case "prompts/get":
			promptName, _ := params["name"].(string)
			arguments, _ := params["arguments"].(map[string]interface{})

			if promptName == "code_review" {
				language, _ := arguments["language"].(string)
				complexity, _ := arguments["complexity"].(string)
				if complexity == "" {
					complexity = "medium"
				}

				response = map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]interface{}{
						"description": "Code review prompt for " + language,
						"messages": []map[string]interface{}{
							{
								"role": "user",
								"content": map[string]interface{}{
									"type": "text",
									"text": fmt.Sprintf("Please review this %s code with %s complexity level and provide feedback on code quality, potential issues, and improvements.", language, complexity),
								},
							},
						},
					},
				}
			}

		case "ping":
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result":  map[string]interface{}{},
			}

		default:
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "Method not found",
				},
			}
		}

		responseBytes, _ := json.Marshal(response)
		fmt.Printf("   📤 Server responding: %s\n", method)
		return responseBytes, nil
	})

	// Initialize and start both transports
	if err := serverTransport.Initialize(); err != nil {
		t.Fatalf("Server transport initialize failed: %v", err)
	}
	if err := clientTransport.Initialize(); err != nil {
		t.Fatalf("Client transport initialize failed: %v", err)
	}
	if err := serverTransport.Start(); err != nil {
		t.Fatalf("Server transport start failed: %v", err)
	}
	if err := clientTransport.Start(); err != nil {
		t.Fatalf("Client transport start failed: %v", err)
	}

	defer func() {
		serverTransport.Stop()
		clientTransport.Stop()
	}()

	fmt.Println("✅ Server and client transports started successfully")
	fmt.Println("")

	// Helper function to send JSON-RPC request and get response
	sendRequest := func(method string, params interface{}, id int) (map[string]interface{}, error) {
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  method,
			"id":      id,
		}
		if params != nil {
			request["params"] = params
		}

		requestBytes, _ := json.Marshal(request)
		if err := clientTransport.Send(requestBytes); err != nil {
			return nil, err
		}

		responseBytes, err := clientTransport.Receive()
		if err != nil {
			return nil, err
		}

		var response map[string]interface{}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			return nil, err
		}

		return response, nil
	}

	// Test 1: Initialize
	fmt.Println("1. 🤝 Initializing connection...")
	initResponse, err := sendRequest("initialize", map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities": map[string]interface{}{
			"roots":    map[string]interface{}{"listChanged": true},
			"sampling": map[string]interface{}{},
		},
		"clientInfo": map[string]interface{}{
			"name":    "embedded-test-client",
			"version": "1.0.0",
		},
	}, 1)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	result := initResponse["result"].(map[string]interface{})
	serverInfo := result["serverInfo"].(map[string]interface{})
	fmt.Printf("   📥 Server: %s v%s\n", serverInfo["name"], serverInfo["version"])
	fmt.Printf("   📥 Protocol: %s\n", result["protocolVersion"])
	fmt.Println("")

	// Test 2: List tools
	fmt.Println("2. 🔧 Listing tools...")
	toolsResponse, err := sendRequest("tools/list", nil, 2)
	if err != nil {
		t.Fatalf("List tools failed: %v", err)
	}
	toolsResult := toolsResponse["result"].(map[string]interface{})
	tools := toolsResult["tools"].([]interface{})
	fmt.Printf("   📥 Found %d tools:\n", len(tools))
	for _, tool := range tools {
		toolMap := tool.(map[string]interface{})
		fmt.Printf("      - %s: %s\n", toolMap["name"], toolMap["description"])
	}
	fmt.Println("")

	// Test 3: Call greet tool
	fmt.Println("3. ⚡ Calling greet tool...")
	greetResponse, err := sendRequest("tools/call", map[string]interface{}{
		"name": "greet",
		"arguments": map[string]interface{}{
			"name": "Integration Test",
		},
	}, 3)
	if err != nil {
		t.Fatalf("Call greet tool failed: %v", err)
	}
	greetResult := greetResponse["result"].(map[string]interface{})
	content := greetResult["content"].([]interface{})[0].(map[string]interface{})
	fmt.Printf("   📥 Greet result: %s\n", content["text"])
	fmt.Println("")

	// Test 4: Call calculate tool
	fmt.Println("4. 🧮 Calling calculate tool...")
	calcResponse, err := sendRequest("tools/call", map[string]interface{}{
		"name": "calculate",
		"arguments": map[string]interface{}{
			"operation": "multiply",
			"a":         15.5,
			"b":         2.0,
		},
	}, 4)
	if err != nil {
		t.Fatalf("Call calculate tool failed: %v", err)
	}
	calcResult := calcResponse["result"].(map[string]interface{})
	calcContent := calcResult["content"].([]interface{})[0].(map[string]interface{})
	fmt.Printf("   📥 Calculate result: %s\n", calcContent["text"])
	fmt.Println("")

	// Test 5: List resources
	fmt.Println("5. 📁 Listing resources...")
	resourcesResponse, err := sendRequest("resources/list", nil, 5)
	if err != nil {
		t.Fatalf("List resources failed: %v", err)
	}
	resourcesResult := resourcesResponse["result"].(map[string]interface{})
	resources := resourcesResult["resources"].([]interface{})
	fmt.Printf("   📥 Found %d resources:\n", len(resources))
	for _, resource := range resources {
		resourceMap := resource.(map[string]interface{})
		fmt.Printf("      - %s: %s (%s)\n", resourceMap["uri"], resourceMap["description"], resourceMap["mimeType"])
	}
	fmt.Println("")

	// Test 6: Read resource
	fmt.Println("6. 📖 Reading /config resource...")
	configResponse, err := sendRequest("resources/read", map[string]interface{}{
		"uri": "/config",
	}, 6)
	if err != nil {
		t.Fatalf("Read resource failed: %v", err)
	}
	configResult := configResponse["result"].(map[string]interface{})
	configContents := configResult["contents"].([]interface{})[0].(map[string]interface{})
	fmt.Printf("   📥 Config content: %s\n", configContents["text"])
	fmt.Println("")

	// Test 7: List prompts
	fmt.Println("7. 💬 Listing prompts...")
	promptsResponse, err := sendRequest("prompts/list", nil, 7)
	if err != nil {
		t.Fatalf("List prompts failed: %v", err)
	}
	promptsResult := promptsResponse["result"].(map[string]interface{})
	prompts := promptsResult["prompts"].([]interface{})
	fmt.Printf("   📥 Found %d prompts:\n", len(prompts))
	for _, prompt := range prompts {
		promptMap := prompt.(map[string]interface{})
		fmt.Printf("      - %s: %s\n", promptMap["name"], promptMap["description"])
	}
	fmt.Println("")

	// Test 8: Get prompt
	fmt.Println("8. 🎯 Getting code_review prompt...")
	promptResponse, err := sendRequest("prompts/get", map[string]interface{}{
		"name": "code_review",
		"arguments": map[string]interface{}{
			"language":   "Go",
			"complexity": "high",
		},
	}, 8)
	if err != nil {
		t.Fatalf("Get prompt failed: %v", err)
	}
	promptResult := promptResponse["result"].(map[string]interface{})
	messages := promptResult["messages"].([]interface{})[0].(map[string]interface{})
	messageContent := messages["content"].(map[string]interface{})
	fmt.Printf("   📥 Prompt: %s\n", messageContent["text"])
	fmt.Println("")

	// Test 9: Ping
	fmt.Println("9. 🏓 Testing ping...")
	pingResponse, err := sendRequest("ping", nil, 9)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if pingResponse["result"] != nil {
		fmt.Printf("   📥 Ping successful!\n")
	}
	fmt.Println("")

	fmt.Println("✅ All JSON-RPC operations completed successfully!")
	fmt.Println("🎉 Embedded transport integration test finished")

	// Verify results
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
	if len(resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(resources))
	}
	if len(prompts) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(prompts))
	}
}

// TestConcurrentJSONRPC tests concurrent JSON-RPC requests over embedded transport
func TestConcurrentJSONRPC(t *testing.T) {
	fmt.Println("🚀 Starting Concurrent JSON-RPC Test...")

	// Create transport pair
	serverTransport, clientTransport := NewTransportPair()

	// Set up server-side message handler for concurrent processing
	serverTransport.SetMessageHandler(func(message []byte) ([]byte, error) {
		var request map[string]interface{}
		if err := json.Unmarshal(message, &request); err != nil {
			return nil, err
		}

		id := request["id"]
		params, _ := request["params"].(map[string]interface{})

		// Simulate processing time
		time.Sleep(50 * time.Millisecond)

		// Simple echo with processing info
		value, _ := params["value"].(float64)
		result := value * 2

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]interface{}{
				"input":     value,
				"output":    result,
				"processed": time.Now().Unix(),
			},
		}

		return json.Marshal(response)
	})

	// Initialize and start transports
	serverTransport.Initialize()
	clientTransport.Initialize()
	serverTransport.Start()
	clientTransport.Start()

	defer func() {
		serverTransport.Stop()
		clientTransport.Stop()
	}()

	fmt.Println("📡 Making 10 concurrent requests...")

	// Make concurrent requests
	const numRequests = 10
	var wg sync.WaitGroup
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			request := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "process",
				"params": map[string]interface{}{
					"value": float64(requestID * 10),
				},
				"id": requestID,
			}

			requestBytes, _ := json.Marshal(request)
			if err := clientTransport.Send(requestBytes); err != nil {
				results <- fmt.Errorf("request %d send failed: %v", requestID, err)
				return
			}

			responseBytes, err := clientTransport.Receive()
			if err != nil {
				results <- fmt.Errorf("request %d receive failed: %v", requestID, err)
				return
			}

			var response map[string]interface{}
			if err := json.Unmarshal(responseBytes, &response); err != nil {
				results <- fmt.Errorf("request %d parse failed: %v", requestID, err)
				return
			}

			result := response["result"].(map[string]interface{})
			fmt.Printf("   📥 Request %d: %.0f -> %.0f\n", requestID, result["input"], result["output"])
			results <- nil
		}(i)
	}

	wg.Wait()
	close(results)

	// Check results
	errorCount := 0
	for err := range results {
		if err != nil {
			t.Errorf("Concurrent request error: %v", err)
			errorCount++
		}
	}

	if errorCount == 0 {
		fmt.Println("✅ All concurrent requests completed successfully!")
	} else {
		fmt.Printf("❌ %d out of %d requests failed\n", errorCount, numRequests)
	}
}

// TestNotificationHandling tests JSON-RPC notification handling
func TestNotificationHandling(t *testing.T) {
	fmt.Println("🚀 Starting Notification Handling Test...")

	// Create transport pair
	serverTransport, clientTransport := NewTransportPair()

	// Initialize and start server transport
	serverTransport.Initialize()
	serverTransport.Start()

	// For notifications to work properly with the embedded transport,
	// we need to handle them on the client side by reading from the transport
	clientTransport.Initialize()
	clientTransport.Start()

	defer func() {
		serverTransport.Stop()
		clientTransport.Stop()
	}()

	// Track notifications received by client
	clientNotifications := make(chan map[string]interface{}, 5)

	// Start a goroutine to read notifications from the client transport
	go func() {
		for {
			message, err := clientTransport.Receive()
			if err != nil {
				return // Transport closed
			}

			var notification map[string]interface{}
			if err := json.Unmarshal(message, &notification); err != nil {
				continue
			}

			// Notifications don't have an "id" field
			if _, hasID := notification["id"]; !hasID {
				clientNotifications <- notification
			}
		}
	}()

	fmt.Println("📡 Sending notifications from server to client...")

	// Send various notifications
	notifications := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"method":  "progress/update",
			"params": map[string]interface{}{
				"token":      "task-123",
				"percentage": 25,
				"message":    "Processing data...",
			},
		},
		{
			"jsonrpc": "2.0",
			"method":  "log/message",
			"params": map[string]interface{}{
				"level":     "info",
				"message":   "Operation completed successfully",
				"timestamp": time.Now().Unix(),
			},
		},
		{
			"jsonrpc": "2.0",
			"method":  "resource/changed",
			"params": map[string]interface{}{
				"uri":    "/config",
				"action": "updated",
			},
		},
	}

	for i, notification := range notifications {
		notificationBytes, _ := json.Marshal(notification)
		if err := serverTransport.Send(notificationBytes); err != nil {
			t.Fatalf("Failed to send notification %d: %v", i, err)
		}
		fmt.Printf("   📤 Sent: %s\n", notification["method"])
		time.Sleep(10 * time.Millisecond) // Small delay to ensure ordering
	}

	// Collect notifications
	receivedCount := 0
	timeout := time.After(2 * time.Second)

	for receivedCount < len(notifications) {
		select {
		case notification := <-clientNotifications:
			receivedCount++
			fmt.Printf("   📥 Received: %s\n", notification["method"])
		case <-timeout:
			t.Fatalf("Timeout waiting for notifications, received %d out of %d", receivedCount, len(notifications))
		}
	}

	fmt.Printf("✅ All %d notifications received successfully!\n", len(notifications))
}
