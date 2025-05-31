package test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
)

// TestClientBatchOperations tests the client's batch functionality.
func TestClientBatchOperations(t *testing.T) {
	tests := []struct {
		name        string
		requests    []client.BatchRequest
		expectCount int
		expectError bool
	}{
		{
			name: "mixed batch with tools, resources, and prompts",
			requests: []client.BatchRequest{
				{
					Method: "tools/call",
					Params: map[string]interface{}{
						"name": "calculator",
						"arguments": map[string]interface{}{
							"operation": "add",
							"a":         1,
							"b":         2,
						},
					},
					ID: 1,
				},
				{
					Method: "resources/read",
					Params: map[string]interface{}{
						"uri": "/test/resource",
					},
					ID: 2,
				},
				{
					Method: "prompts/get",
					Params: map[string]interface{}{
						"name": "greeting",
						"arguments": map[string]interface{}{
							"name": "Alice",
						},
					},
					ID: 3,
				},
			},
			expectCount: 3,
			expectError: false,
		},
		{
			name: "batch with notifications (no responses expected)",
			requests: []client.BatchRequest{
				{
					Method: "notifications/progress",
					Params: map[string]interface{}{
						"progress": 50,
						"message":  "Halfway done",
					},
					// ID is nil for notifications
				},
				{
					Method: "notifications/cancelled",
					Params: map[string]interface{}{
						"reason": "User requested cancellation",
					},
					// ID is nil for notifications
				},
			},
			expectCount: 0, // Notifications don't generate responses
			expectError: false,
		},
		{
			name:        "empty batch",
			requests:    []client.BatchRequest{},
			expectCount: 0,
			expectError: false,
		},
		{
			name: "batch with mixed requests and notifications",
			requests: []client.BatchRequest{
				{
					Method: "tools/call",
					Params: map[string]interface{}{
						"name": "echo",
						"arguments": map[string]interface{}{
							"message": "Hello World",
						},
					},
					ID: 1,
				},
				{
					Method: "notifications/progress",
					Params: map[string]interface{}{
						"progress": 25,
					},
					// ID is nil for notification
				},
				{
					Method: "resources/read",
					Params: map[string]interface{}{
						"uri": "/another/resource",
					},
					ID: 2,
				},
			},
			expectCount: 2, // Only requests generate responses
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create client with mock transport
			c, mockTransport := SetupClientWithMockTransport(t, "2025-03-26")

			// Set up expected batch response based on the requests
			if tt.expectCount == 0 {
				// For all-notification batches or empty batches, no response
				mockTransport.QueueResponse([]byte{}, nil)
			} else {
				// Create mock responses for each request (not notification)
				var responses []interface{}
				for _, req := range tt.requests {
					if req.ID != nil { // Only requests have responses
						responses = append(responses, map[string]interface{}{
							"jsonrpc": "2.0",
							"id":      req.ID,
							"result":  map[string]interface{}{"success": true, "method": req.Method},
						})
					}
				}
				responseJSON, _ := json.Marshal(responses)
				mockTransport.QueueResponse(responseJSON, nil)
			}

			// Send batch request
			responses, err := c.SendBatch(tt.requests)

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify response count
			if len(responses) != tt.expectCount {
				t.Errorf("Expected %d responses, got %d", tt.expectCount, len(responses))
			}

			// Verify response structure for non-empty responses
			if tt.expectCount > 0 {
				for i, response := range responses {
					if response.ID == nil {
						t.Errorf("Response %d missing ID", i)
					}
					if response.Result == nil && response.Error == nil {
						t.Errorf("Response %d missing both result and error", i)
					}
				}
			}
		})
	}
}

// TestBatchBuilder tests the batch builder functionality.
func TestBatchBuilder(t *testing.T) {
	c, mockTransport := SetupClientWithMockTransport(t, "2025-03-26")

	// Set up expected response
	expectedResponse := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"success": true, "value": 3},
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"result":  map[string]interface{}{"content": "resource data"},
		},
	}
	responseJSON, _ := json.Marshal(expectedResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	// Use batch builder
	responses, err := c.BatchBuilder().
		AddRequest("tools/call", map[string]interface{}{
			"name": "calculator",
			"arguments": map[string]interface{}{
				"operation": "add",
				"a":         1,
				"b":         2,
			},
		}, 1).
		AddRequest("resources/read", map[string]interface{}{
			"uri": "/test/resource",
		}, 2).
		Execute()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(responses) != 2 {
		t.Fatalf("Expected 2 responses, got %d", len(responses))
	}

	// Verify first response
	if responses[0].ID != float64(1) {
		t.Errorf("Expected first response ID to be 1, got %v", responses[0].ID)
	}

	// Verify second response
	if responses[1].ID != float64(2) {
		t.Errorf("Expected second response ID to be 2, got %v", responses[1].ID)
	}
}

// TestBatchTimeout tests batch operations with timeout.
func TestBatchTimeout(t *testing.T) {
	c, mockTransport := SetupClientWithMockTransport(t, "2025-03-26")

	expectedResponse := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"success": true},
		},
	}
	responseJSON, _ := json.Marshal(expectedResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	requests := []client.BatchRequest{
		{
			Method: "tools/call",
			Params: map[string]interface{}{
				"name":      "slow_tool",
				"arguments": map[string]interface{}{},
			},
			ID: 1,
		},
	}

	// Test with timeout option
	responses, err := c.SendBatch(requests, client.WithRequestTimeoutOption(5*time.Second))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(responses) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(responses))
	}
}

// TestBatchErrorHandling tests error handling in batch operations.
func TestBatchErrorHandling(t *testing.T) {
	c, mockTransport := SetupClientWithMockTransport(t, "2025-03-26")

	// Set up response with one success and one error
	expectedResponse := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"success": true},
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      2,
			"error": map[string]interface{}{
				"code":    -32601,
				"message": "Method not found",
			},
		},
	}
	responseJSON, _ := json.Marshal(expectedResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	requests := []client.BatchRequest{
		{
			Method: "tools/call",
			Params: map[string]interface{}{
				"name":      "valid_tool",
				"arguments": map[string]interface{}{},
			},
			ID: 1,
		},
		{
			Method: "invalid/method",
			Params: map[string]interface{}{},
			ID:     2,
		},
	}

	responses, err := c.SendBatch(requests)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(responses) != 2 {
		t.Fatalf("Expected 2 responses, got %d", len(responses))
	}

	// First response should be successful
	if responses[0].Error != nil {
		t.Errorf("Expected first response to be successful, got error: %v", responses[0].Error)
	}

	// Second response should have an error
	if responses[1].Error == nil {
		t.Errorf("Expected second response to have an error")
	} else {
		if responses[1].Error.Code != -32601 {
			t.Errorf("Expected error code -32601, got %d", responses[1].Error.Code)
		}
		if responses[1].Error.Message != "Method not found" {
			t.Errorf("Expected error message 'Method not found', got %s", responses[1].Error.Message)
		}
	}
}

// TestBatchResponseOrdering tests that batch responses maintain order.
func TestBatchResponseOrdering(t *testing.T) {
	c, mockTransport := SetupClientWithMockTransport(t, "2025-03-26")

	// Set up responses in order
	expectedResponse := []interface{}{
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "first",
			"result":  map[string]interface{}{"order": 1},
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "second",
			"result":  map[string]interface{}{"order": 2},
		},
		map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "third",
			"result":  map[string]interface{}{"order": 3},
		},
	}
	responseJSON, _ := json.Marshal(expectedResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	requests := []client.BatchRequest{
		{Method: "test/method", Params: map[string]interface{}{}, ID: "first"},
		{Method: "test/method", Params: map[string]interface{}{}, ID: "second"},
		{Method: "test/method", Params: map[string]interface{}{}, ID: "third"},
	}

	responses, err := c.SendBatch(requests)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(responses) != 3 {
		t.Fatalf("Expected 3 responses, got %d", len(responses))
	}

	// Verify order is maintained
	expectedIDs := []string{"first", "second", "third"}
	for i, response := range responses {
		if response.ID != expectedIDs[i] {
			t.Errorf("Expected response %d to have ID %s, got %v", i, expectedIDs[i], response.ID)
		}
	}
}

// TestBatchLargeRequest tests batch operations with many requests.
func TestBatchLargeRequest(t *testing.T) {
	c, mockTransport := SetupClientWithMockTransport(t, "2025-03-26")

	// Create responses for 50 requests
	var responses []interface{}
	for i := 0; i < 50; i++ {
		responses = append(responses, map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      i + 1,
			"result":  map[string]interface{}{"index": i},
		})
	}
	responseJSON, _ := json.Marshal(responses)
	mockTransport.QueueResponse(responseJSON, nil)

	// Create 50 requests
	var requests []client.BatchRequest
	for i := 0; i < 50; i++ {
		requests = append(requests, client.BatchRequest{
			Method: "test/method",
			Params: map[string]interface{}{"index": i},
			ID:     i + 1,
		})
	}

	batchResponses, err := c.SendBatch(requests)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(batchResponses) != 50 {
		t.Fatalf("Expected 50 responses, got %d", len(batchResponses))
	}
}
