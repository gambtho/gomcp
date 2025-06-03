package v20241105

import (
	"bufio"
	"bytes"
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

// TestSSETransport_v20241105 tests SSE transport compliance with MCP 2024-11-05 specification
func TestSSETransport_v20241105(t *testing.T) {
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

	// Register a test tool
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (interface{}, error) {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": args.Message},
			},
		}, nil
	})

	// Start server
	go func() {
		if err := srv.Run(); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	t.Run("SSE endpoint provides endpoint discovery", func(t *testing.T) {
		testSSEEndpointDiscovery_v20241105(t, address)
	})

	t.Run("POST endpoint handles JSON-RPC requests", func(t *testing.T) {
		testPOSTEndpoint_v20241105(t, address)
	})

	t.Run("Complete initialization flow", func(t *testing.T) {
		testCompleteFlow_v20241105(t, address)
	})

	t.Run("Error handling", func(t *testing.T) {
		testErrorHandling_v20241105(t, address)
	})
}

// testSSEEndpointDiscovery_v20241105 tests that the SSE endpoint provides proper endpoint discovery
func testSSEEndpointDiscovery_v20241105(t *testing.T, address string) {
	// Connect to SSE endpoint
	sseURL := fmt.Sprintf("http://%s/sse", address)

	req, err := http.NewRequest("GET", sseURL, nil)
	if err != nil {
		t.Fatalf("Failed to create SSE request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to SSE endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Verify SSE headers
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type: text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control: no-cache, got %s", resp.Header.Get("Cache-Control"))
	}

	// Read SSE events
	reader := bufio.NewReader(resp.Body)
	var endpointURL string

	// Set timeout for reading
	done := make(chan bool, 1)
	go func() {
		time.Sleep(2 * time.Second)
		done <- true
	}()

	// Read events until we get the endpoint event or timeout
	for {
		select {
		case <-done:
			if endpointURL == "" {
				t.Fatal("Timeout waiting for endpoint event")
			}
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					// End of stream reached without finding endpoint
					if endpointURL == "" {
						t.Error("Did not receive endpoint event from SSE stream")
					}
					return
				}
				t.Fatalf("Error reading SSE stream: %v", err)
			}

			line = bytes.TrimSpace(line)

			// Look for endpoint event
			if bytes.HasPrefix(line, []byte("event: endpoint")) {
				// Next line should be the data
				dataLine, err := reader.ReadBytes('\n')
				if err != nil {
					t.Fatalf("Error reading endpoint data: %v", err)
				}
				dataLine = bytes.TrimSpace(dataLine)

				if bytes.HasPrefix(dataLine, []byte("data: ")) {
					endpointURL = string(bytes.TrimPrefix(dataLine, []byte("data: ")))
					t.Logf("Received endpoint URL: %s", endpointURL)

					// Validate endpoint URL format
					if !strings.HasPrefix(endpointURL, "http://") {
						t.Errorf("Expected endpoint URL to start with http://, got %s", endpointURL)
					}
					if !strings.Contains(endpointURL, address) {
						t.Errorf("Expected endpoint URL to contain address %s, got %s", address, endpointURL)
					}
					return
				}
			}
		}
	}
}

// testPOSTEndpoint_v20241105 tests that the POST endpoint handles JSON-RPC requests properly
func testPOSTEndpoint_v20241105(t *testing.T, address string) {
	// First get the endpoint URL via SSE
	endpointURL := getEndpointURL_v20241105(t, address)

	// Test initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
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

	resp, err := http.Post(endpointURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to send POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// Verify Content-Type
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Errorf("Expected Content-Type to contain application/json, got %s", resp.Header.Get("Content-Type"))
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
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

	// Validate 2024-11-05 specific response
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("Expected protocolVersion '2024-11-05', got %v", result["protocolVersion"])
	}
}

// testCompleteFlow_v20241105 tests the complete initialization and tool call flow
func testCompleteFlow_v20241105(t *testing.T, address string) {
	endpointURL := getEndpointURL_v20241105(t, address)

	// 1. Initialize
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	// Send initialize
	_, err := sendJSONRPCRequest_v20241105(t, endpointURL, initRequest)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// 2. Send initialized notification
	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	resp, err := sendRawRequest_v20241105(t, endpointURL, initNotification)
	if err != nil {
		t.Fatalf("Failed to send initialized notification: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for notification, got %d", resp.StatusCode)
	}

	// 3. Call tool
	toolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"message": "Hello, World!",
			},
		},
	}

	toolResponse, err := sendJSONRPCRequest_v20241105(t, endpointURL, toolRequest)
	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}

	// Validate tool response format for 2024-11-05
	result, ok := toolResponse["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected tool result object, got %T", toolResponse["result"])
	}

	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("Expected content array, got %T", result["content"])
	}

	if len(content) == 0 {
		t.Fatal("Expected at least one content item")
	}

	contentItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected content item to be object, got %T", content[0])
	}

	if contentItem["type"] != "text" {
		t.Errorf("Expected content type 'text', got %v", contentItem["type"])
	}
	if contentItem["text"] != "Hello, World!" {
		t.Errorf("Expected text 'Hello, World!', got %v", contentItem["text"])
	}
}

// testErrorHandling_v20241105 tests error handling scenarios
func testErrorHandling_v20241105(t *testing.T, address string) {
	endpointURL := getEndpointURL_v20241105(t, address)

	// Test invalid JSON
	resp, err := http.Post(endpointURL, "application/json", strings.NewReader("invalid json"))
	if err != nil {
		t.Fatalf("Failed to send invalid JSON: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}

	// Test invalid method
	invalidRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "invalid/method",
		"params":  map[string]interface{}{},
	}

	invalidResponse, err := sendJSONRPCRequest_v20241105(t, endpointURL, invalidRequest)
	if err != nil {
		t.Fatalf("Failed to send invalid method request: %v", err)
	}

	// Should get JSON-RPC error response
	if invalidResponse["error"] == nil {
		t.Error("Expected error response for invalid method")
	}
}

// Helper functions

func getEndpointURL_v20241105(t *testing.T, address string) string {
	sseURL := fmt.Sprintf("http://%s/sse", address)

	req, err := http.NewRequest("GET", sseURL, nil)
	if err != nil {
		t.Fatalf("Failed to create SSE request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatalf("Error reading SSE: %v", err)
		}

		line = bytes.TrimSpace(line)

		if bytes.HasPrefix(line, []byte("event: endpoint")) {
			dataLine, err := reader.ReadBytes('\n')
			if err != nil {
				t.Fatalf("Error reading endpoint data: %v", err)
			}
			dataLine = bytes.TrimSpace(dataLine)

			if bytes.HasPrefix(dataLine, []byte("data: ")) {
				return string(bytes.TrimPrefix(dataLine, []byte("data: ")))
			}
		}
	}
}

func sendJSONRPCRequest_v20241105(t *testing.T, url string, request map[string]interface{}) (map[string]interface{}, error) {
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response, nil
}

func sendRawRequest_v20241105(t *testing.T, url string, request map[string]interface{}) (*http.Response, error) {
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	return http.Post(url, "application/json", bytes.NewReader(reqBody))
}
