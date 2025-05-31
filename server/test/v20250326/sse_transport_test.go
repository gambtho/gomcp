package v20250326

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
)

// TestSSETransport_v20250326 tests SSE transport compliance with MCP 2025-03-26 specification
func TestSSETransport_v20250326(t *testing.T) {
	// Get a dynamic port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	address := listener.Addr().String()
	listener.Close()

	// Create server with SSE transport
	srv := server.NewServer("sse-test-server")
	srv.AsSSE(address)

	// Register a test tool with function handler that has struct args
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (interface{}, error) {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": args.Message},
			},
		}, nil
	})

	// Register a tool that takes no arguments (nil case)
	srv.Tool("get-list", "Get a list of items", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "List of items: item1, item2, item3"},
			},
		}, nil
	})

	// Register a tool with pointer to struct
	srv.Tool("count-words", "Count words in a message", func(ctx *server.Context, args *struct {
		Message string `json:"message"`
		Limit   int    `json:"limit"`
	}) (interface{}, error) {
		words := len(strings.Fields(args.Message))
		if args.Limit > 0 && words > args.Limit {
			words = args.Limit
		}
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Word count: %d", words)},
			},
		}, nil
	})

	// Start server
	go func() {
		if err := srv.Run(); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Wait for server to start with health check
	maxRetries := 20
	retryDelay := 100 * time.Millisecond
	serverReady := false

	for i := 0; i < maxRetries; i++ {
		// Try to connect to the server
		testURL := fmt.Sprintf("http://%s/mcp", address)
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			t.Logf("Health check %d: Failed to create request: %v", i+1, err)
			time.Sleep(retryDelay)
			continue
		}
		req.Header.Set("Accept", "text/event-stream")

		client := &http.Client{Timeout: 100 * time.Millisecond}
		resp, err := client.Do(req)
		if err != nil {
			t.Logf("Health check %d: Connection failed to %s: %v", i+1, testURL, err)
			time.Sleep(retryDelay)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Logf("Health check %d: Server ready at %s", i+1, testURL)
			serverReady = true
			break
		} else {
			t.Logf("Health check %d: Got status %d from %s", i+1, resp.StatusCode, testURL)
		}
		time.Sleep(retryDelay)
	}

	if !serverReady {
		t.Fatalf("Server failed to start within %v", time.Duration(maxRetries)*retryDelay)
	}

	t.Run("Unified MCP endpoint supports GET for SSE", func(t *testing.T) {
		testUnifiedEndpointSSE_v20250326(t, address)
	})

	t.Run("Unified MCP endpoint supports POST for messages", func(t *testing.T) {
		testUnifiedEndpointPOST_v20250326(t, address)
	})

	t.Run("Batch request support", func(t *testing.T) {
		testBatchRequests_v20250326(t, address)
	})

	t.Run("SSE streaming for requests", func(t *testing.T) {
		testSSEStreaming_v20250326(t, address)
	})

	t.Run("Session management", func(t *testing.T) {
		testSessionManagement_v20250326(t, address)
	})

	t.Run("Error handling", func(t *testing.T) {
		testErrorHandling_v20250326(t, address)
	})

	t.Run("All parameter types", func(t *testing.T) {
		testAllParameterTypes_v20250326(t, address)
	})
}

// testUnifiedEndpointSSE_v20250326 tests that the unified MCP endpoint supports GET for SSE
func testUnifiedEndpointSSE_v20250326(t *testing.T, address string) {
	// Connect to unified MCP endpoint for SSE
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// Set timeout for the entire test (use context with timeout for proper cancellation)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) // Reduced timeout
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", mcpURL, nil)
	if err != nil {
		t.Fatalf("Failed to create GET request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to MCP endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Verify response is SSE
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Errorf("Expected Content-Type to contain text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	// For 2025-03-26, we shouldn't receive an "endpoint" event since the client
	// already knows the endpoint from the URL they connected to
	reader := bufio.NewReader(resp.Body)

	receivedEndpointEvent := false

	// Create a channel to receive read results
	type readResult struct {
		line []byte
		err  error
	}
	readCh := make(chan readResult, 1)

	// Start a goroutine to read from the stream
	go func() {
		for {
			line, err := reader.ReadBytes('\n')
			select {
			case readCh <- readResult{line: line, err: err}:
				if err != nil {
					return // Exit on error (including EOF)
				}
			case <-ctx.Done():
				return // Exit if context is cancelled
			}
		}
	}()

	// Read events for a short time to check if endpoint event is sent
	// We expect NO events to be sent for 2025-03-26 unified endpoint pattern
	readTimeout := time.NewTimer(500 * time.Millisecond) // Short timeout to check for events
	defer readTimeout.Stop()

	for {
		select {
		case <-readTimeout.C:
			// Timeout reached without receiving endpoint event - this is expected for 2025-03-26
			if receivedEndpointEvent {
				t.Error("Should not receive endpoint event in 2025-03-26 unified endpoint pattern")
			}
			return
		case <-ctx.Done():
			// Context timeout - also acceptable
			if receivedEndpointEvent {
				t.Error("Should not receive endpoint event in 2025-03-26 unified endpoint pattern")
			}
			return
		case result := <-readCh:
			if result.err != nil {
				if result.err == io.EOF {
					// EOF is acceptable - connection closed
					return
				}
				t.Fatalf("Error reading SSE stream: %v", result.err)
			}

			line := bytes.TrimSpace(result.line)

			// Check if we receive an endpoint event (which we shouldn't)
			if bytes.HasPrefix(line, []byte("event: endpoint")) {
				receivedEndpointEvent = true
				t.Error("Should not receive endpoint event in 2025-03-26 unified endpoint pattern")
				return
			}

			// If we receive any other events, that's unexpected for this test but not necessarily an error
			if len(line) > 0 && bytes.HasPrefix(line, []byte("event:")) {
				t.Logf("Received unexpected event: %s", string(line))
			}
		}
	}
}

// testUnifiedEndpointPOST_v20250326 tests that the unified MCP endpoint supports POST for messages
func testUnifiedEndpointPOST_v20250326(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// Test initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities": map[string]interface{}{
				"roots": map[string]interface{}{
					"listChanged": true,
				},
			},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	reqBody, err := json.Marshal(initRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Client must include Accept header for both JSON and SSE
	req, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// Server can respond with either JSON or SSE stream
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type to be application/json or text/event-stream, got %s", contentType)
	}

	// If it's JSON response, parse and validate
	if strings.Contains(contentType, "application/json") {
		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode JSON response: %v", err)
		}

		// Validate JSON-RPC response
		if response["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc '2.0', got %v", response["jsonrpc"])
		}
		if response["id"] != float64(1) {
			t.Errorf("Expected id 1, got %v", response["id"])
		}

		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %T", response["result"])
		}

		// Validate 2025-03-26 specific response
		if result["protocolVersion"] != "2025-03-26" {
			t.Errorf("Expected protocolVersion '2025-03-26', got %v", result["protocolVersion"])
		}
	}
}

// testBatchRequests_v20250326 tests batch request support in 2025-03-26
func testBatchRequests_v20250326(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// First initialize
	initAndInitialized_v20250326(t, mcpURL)

	// Create batch request with multiple tool calls
	batchRequest := []map[string]interface{}{
		{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "echo",
				"arguments": map[string]interface{}{
					"message": "First message",
				},
			},
		},
		{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "echo",
				"arguments": map[string]interface{}{
					"message": "Second message",
				},
			},
		},
	}

	reqBody, err := json.Marshal(batchRequest)
	if err != nil {
		t.Fatalf("Failed to marshal batch request: %v", err)
	}

	req, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send batch request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// Response can be either JSON array or SSE stream
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// JSON batch response
		var responses []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&responses); err != nil {
			t.Fatalf("Failed to decode batch response: %v", err)
		}

		if len(responses) != 2 {
			t.Fatalf("Expected 2 responses, got %d", len(responses))
		}

		// Validate each response
		for i, response := range responses {
			if response["jsonrpc"] != "2.0" {
				t.Errorf("Response %d: Expected jsonrpc '2.0', got %v", i, response["jsonrpc"])
			}
			expectedID := float64(i + 1)
			if response["id"] != expectedID {
				t.Errorf("Response %d: Expected id %v, got %v", i, expectedID, response["id"])
			}
		}
	} else if strings.Contains(contentType, "text/event-stream") {
		// SSE batch response - should receive multiple response events
		reader := bufio.NewReader(resp.Body)
		receivedResponses := 0

		// Read SSE events for responses
		for receivedResponses < 2 {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("Error reading SSE stream: %v", err)
			}

			line = bytes.TrimSpace(line)

			if bytes.HasPrefix(line, []byte("event: message")) {
				// Read data line
				dataLine, err := reader.ReadBytes('\n')
				if err != nil {
					t.Fatalf("Error reading response data: %v", err)
				}
				dataLine = bytes.TrimSpace(dataLine)

				if bytes.HasPrefix(dataLine, []byte("data: ")) {
					responseData := bytes.TrimPrefix(dataLine, []byte("data: "))
					var response map[string]interface{}
					if err := json.Unmarshal(responseData, &response); err != nil {
						t.Fatalf("Error parsing response JSON: %v", err)
					}

					if response["jsonrpc"] == "2.0" && response["id"] != nil {
						receivedResponses++
					}
				}
			}
		}

		if receivedResponses != 2 {
			t.Errorf("Expected 2 responses via SSE, got %d", receivedResponses)
		}
	}
}

// testSSEStreaming_v20250326 tests SSE streaming behavior for requests
func testSSEStreaming_v20250326(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// First initialize
	initAndInitialized_v20250326(t, mcpURL)

	// Send a request that should stream responses
	toolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"message": "Test streaming",
			},
		},
	}

	reqBody, err := json.Marshal(toolRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// The server can choose to stream or return immediate JSON
	// Both are valid according to the spec
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("Expected Content-Type to be application/json or text/event-stream, got %s", contentType)
	}

	// If streaming, ensure we get a proper response
	if strings.Contains(contentType, "text/event-stream") {
		reader := bufio.NewReader(resp.Body)
		foundResponse := false

		// Set timeout
		done := make(chan bool, 1)
		go func() {
			time.Sleep(3 * time.Second)
			done <- true
		}()

		for !foundResponse {
			select {
			case <-done:
				if !foundResponse {
					t.Error("Timeout waiting for streamed response")
				}
				return
			default:
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						return
					}
					t.Fatalf("Error reading SSE stream: %v", err)
				}

				line = bytes.TrimSpace(line)

				if bytes.HasPrefix(line, []byte("event: message")) {
					// Read data line
					dataLine, err := reader.ReadBytes('\n')
					if err != nil {
						t.Fatalf("Error reading response data: %v", err)
					}
					dataLine = bytes.TrimSpace(dataLine)

					if bytes.HasPrefix(dataLine, []byte("data: ")) {
						responseData := bytes.TrimPrefix(dataLine, []byte("data: "))
						var response map[string]interface{}
						if err := json.Unmarshal(responseData, &response); err == nil {
							if response["jsonrpc"] == "2.0" && response["id"] == float64(1) {
								foundResponse = true
							}
						}
					}
				}
			}
		}
	}
}

// testSessionManagement_v20250326 tests session management features
func testSessionManagement_v20250326(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// Send initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	reqBody, err := json.Marshal(initRequest)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check if server returns a session ID in Mcp-Session-Id header
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID != "" {
		t.Logf("Received session ID: %s", sessionID)

		// Session ID validation according to spec
		if len(sessionID) == 0 {
			t.Error("Session ID cannot be empty")
		}

		// Should only contain visible ASCII characters (0x21 to 0x7E)
		for _, char := range []byte(sessionID) {
			if char < 0x21 || char > 0x7E {
				t.Errorf("Session ID contains invalid character: %x", char)
			}
		}

		// Test subsequent request with session ID
		toolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "echo",
				"arguments": map[string]interface{}{
					"message": "Session test",
				},
			},
		}

		reqBody2, err := json.Marshal(toolRequest)
		if err != nil {
			t.Fatalf("Failed to marshal tool request: %v", err)
		}

		req2, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody2))
		if err != nil {
			t.Fatalf("Failed to create second POST request: %v", err)
		}
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Accept", "application/json, text/event-stream")
		req2.Header.Set("Mcp-Session-Id", sessionID) // Include session ID

		resp2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("Failed to send request with session ID: %v", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 with valid session ID, got %d", resp2.StatusCode)
		}
	} else {
		t.Log("Server does not use session management (optional feature)")
	}
}

// testErrorHandling_v20250326 tests error handling scenarios
func testErrorHandling_v20250326(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// Test invalid JSON
	req, err := http.NewRequest("POST", mcpURL, strings.NewReader("invalid json"))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send invalid JSON: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}

	// Test unsupported method
	req2, err := http.NewRequest("PUT", mcpURL, nil)
	if err != nil {
		t.Fatalf("Failed to create PUT request: %v", err)
	}

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Failed to send PUT request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for unsupported method, got %d", resp2.StatusCode)
	}

	// Test notification handling (should return 202)
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	reqBody, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	req3, err := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to create notification request: %v", err)
	}
	req3.Header.Set("Content-Type", "application/json")

	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}
	defer resp3.Body.Close()

	// 2025-03-26 spec: notifications should return 202 Accepted
	if resp3.StatusCode != http.StatusAccepted && resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected status 202 or 200 for notification, got %d", resp3.StatusCode)
	}
}

// testAllParameterTypes_v20250326 tests all three parameter types in actual tool calls
func testAllParameterTypes_v20250326(t *testing.T, address string) {
	mcpURL := fmt.Sprintf("http://%s/mcp", address)

	// First initialize
	initAndInitialized_v20250326(t, mcpURL)

	t.Run("struct by value", func(t *testing.T) {
		// Test echo tool (struct by value)
		toolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "echo",
				"arguments": map[string]interface{}{
					"message": "Test struct by value",
				},
			},
		}

		response, err := sendJSONRPCRequest_v20250326(t, mcpURL, toolRequest)
		if err != nil {
			t.Fatalf("Failed to call echo tool: %v", err)
		}

		// Verify successful response
		if response["error"] != nil {
			t.Fatalf("Tool call failed: %v", response["error"])
		}

		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %T", response["result"])
		}

		t.Logf("Echo tool result: %+v", result)
	})

	t.Run("pointer to struct", func(t *testing.T) {
		// Test count-words tool (pointer to struct)
		toolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name": "count-words",
				"arguments": map[string]interface{}{
					"message": "This is a test message with several words",
					"limit":   10,
				},
			},
		}

		response, err := sendJSONRPCRequest_v20250326(t, mcpURL, toolRequest)
		if err != nil {
			t.Fatalf("Failed to call count-words tool: %v", err)
		}

		// Verify successful response
		if response["error"] != nil {
			t.Fatalf("Tool call failed: %v", response["error"])
		}

		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %T", response["result"])
		}

		t.Logf("Count-words tool result: %+v", result)
	})

	t.Run("interface{} (nil case)", func(t *testing.T) {
		// Test get-list tool (interface{} - no arguments needed)
		toolRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params": map[string]interface{}{
				"name":      "get-list",
				"arguments": map[string]interface{}{}, // Empty arguments
			},
		}

		response, err := sendJSONRPCRequest_v20250326(t, mcpURL, toolRequest)
		if err != nil {
			t.Fatalf("Failed to call get-list tool: %v", err)
		}

		// Verify successful response
		if response["error"] != nil {
			t.Fatalf("Tool call failed: %v", response["error"])
		}

		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result object, got %T", response["result"])
		}

		t.Logf("Get-list tool result: %+v", result)
	})
}

// Helper functions

func initAndInitialized_v20250326(t *testing.T, mcpURL string) {
	// Send initialize
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	_, err := sendJSONRPCRequest_v20250326(t, mcpURL, initRequest)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Send initialized notification
	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	reqBody, _ := json.Marshal(initNotification)
	req, _ := http.NewRequest("POST", mcpURL, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send initialized notification: %v", err)
	}
	defer resp.Body.Close()
}

func sendJSONRPCRequest_v20250326(t *testing.T, url string, request map[string]interface{}) (map[string]interface{}, error) {
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Handle both JSON and SSE responses
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, err
		}
		return response, nil
	} else if strings.Contains(contentType, "text/event-stream") {
		// Read SSE stream until we get the response
		reader := bufio.NewReader(resp.Body)

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return nil, err
			}

			line = bytes.TrimSpace(line)

			if bytes.HasPrefix(line, []byte("event: message")) {
				dataLine, err := reader.ReadBytes('\n')
				if err != nil {
					return nil, err
				}
				dataLine = bytes.TrimSpace(dataLine)

				if bytes.HasPrefix(dataLine, []byte("data: ")) {
					responseData := bytes.TrimPrefix(dataLine, []byte("data: "))
					var response map[string]interface{}
					if err := json.Unmarshal(responseData, &response); err != nil {
						return nil, err
					}
					return response, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("unsupported content type: %s", contentType)
}
