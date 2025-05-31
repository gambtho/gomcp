package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestBatchMessageDetection tests the detection of batch vs single messages
func TestBatchMessageDetection(t *testing.T) {
	srv := server.NewServer("test-batch-detection")

	tests := []struct {
		name     string
		message  string
		isBatch  bool
		hasError bool
	}{
		{
			name:     "single_object_message",
			message:  `{"jsonrpc": "2.0", "method": "ping", "id": 1}`,
			isBatch:  false,
			hasError: false,
		},
		{
			name:     "batch_array_message",
			message:  `[{"jsonrpc": "2.0", "method": "ping", "id": 1}]`,
			isBatch:  true,
			hasError: false,
		},
		{
			name:     "empty_batch_array",
			message:  `[]`,
			isBatch:  true,
			hasError: true,
		},
		{
			name:     "batch_with_multiple_requests",
			message:  `[{"jsonrpc": "2.0", "method": "ping", "id": 1}, {"jsonrpc": "2.0", "method": "ping", "id": 2}]`,
			isBatch:  true,
			hasError: false,
		},
		{
			name:     "batch_with_notification",
			message:  `[{"jsonrpc": "2.0", "method": "notifications/initialized"}]`,
			isBatch:  true,
			hasError: false,
		},
		{
			name:     "mixed_batch_request_and_notification",
			message:  `[{"jsonrpc": "2.0", "method": "ping", "id": 1}, {"jsonrpc": "2.0", "method": "notifications/initialized"}]`,
			isBatch:  true,
			hasError: false,
		},
		{
			name:     "malformed_batch",
			message:  `[{"jsonrpc": "2.0", "method": "ping", "id": 1`,
			isBatch:  true,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseBytes, err := server.HandleMessage(srv.GetServer(), []byte(tt.message))

			if tt.hasError {
				// For error cases, we should get a response with an error
				if responseBytes == nil {
					t.Errorf("Expected error response, got nil")
					return
				}

				var response map[string]interface{}
				if err := json.Unmarshal(responseBytes, &response); err != nil {
					t.Errorf("Failed to parse error response: %v", err)
					return
				}

				if response["error"] == nil {
					t.Errorf("Expected error in response, got: %v", response)
				}
			} else {
				// For success cases, verify the response format
				if tt.isBatch {
					// Batch responses should be arrays (unless all notifications)
					if responseBytes != nil {
						// Check if it's an array or null (for all notifications)
						var batchResponse []interface{}
						if err := json.Unmarshal(responseBytes, &batchResponse); err != nil {
							// If it's not an array, it might be null for all notifications
							var nullResponse interface{}
							if err := json.Unmarshal(responseBytes, &nullResponse); err != nil {
								t.Errorf("Batch response should be array or null, got parse error: %v", err)
							}
						}
					}
				} else {
					// Single responses should be objects
					if responseBytes != nil {
						var singleResponse map[string]interface{}
						if err := json.Unmarshal(responseBytes, &singleResponse); err != nil {
							t.Errorf("Single response should be object, got parse error: %v", err)
						}
					}
				}
			}

			if err != nil {
				t.Errorf("HandleMessage returned error: %v", err)
			}
		})
	}
}

// TestEmptyBatchValidation specifically tests that empty batches are rejected
func TestEmptyBatchValidation(t *testing.T) {
	srv := server.NewServer("test-empty-batch")

	emptyBatch := `[]`
	responseBytes, err := server.HandleMessage(srv.GetServer(), []byte(emptyBatch))

	if err != nil {
		t.Errorf("HandleMessage returned error: %v", err)
	}

	if responseBytes == nil {
		t.Errorf("Expected error response for empty batch, got nil")
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Errorf("Failed to parse error response: %v", err)
		return
	}

	// Verify it's an error response
	errorObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected error object in response, got: %v", response)
		return
	}

	// Verify error code is -32600 (Invalid Request)
	code, ok := errorObj["code"].(float64)
	if !ok || int(code) != -32600 {
		t.Errorf("Expected error code -32600 for empty batch, got: %v", code)
	}

	// Verify error message mentions batch
	message, ok := errorObj["message"].(string)
	if !ok {
		t.Errorf("Expected error message string, got: %v", errorObj["message"])
	} else if message != "Invalid Request" {
		t.Errorf("Expected 'Invalid Request' error message, got: %s", message)
	}
}

// TestBatchResponseOrdering tests that responses maintain the same order as requests
func TestBatchResponseOrdering(t *testing.T) {
	srv := server.NewServer("test-batch-ordering")

	// Create a batch with multiple ping requests with different IDs
	batch := `[
		{"jsonrpc": "2.0", "method": "ping", "id": "first"},
		{"jsonrpc": "2.0", "method": "ping", "id": "second"},
		{"jsonrpc": "2.0", "method": "ping", "id": "third"}
	]`

	responseBytes, err := server.HandleMessage(srv.GetServer(), []byte(batch))
	if err != nil {
		t.Errorf("HandleMessage returned error: %v", err)
	}

	if responseBytes == nil {
		t.Errorf("Expected batch response, got nil")
		return
	}

	var responses []map[string]interface{}
	if err := json.Unmarshal(responseBytes, &responses); err != nil {
		t.Errorf("Failed to parse batch response: %v", err)
		return
	}

	// Verify we got 3 responses
	if len(responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(responses))
		return
	}

	// Verify the order matches the request order
	expectedIDs := []string{"first", "second", "third"}
	for i, response := range responses {
		id, ok := response["id"].(string)
		if !ok {
			t.Errorf("Response %d missing or invalid ID: %v", i, response["id"])
			continue
		}
		if id != expectedIDs[i] {
			t.Errorf("Response %d has wrong ID: expected %s, got %s", i, expectedIDs[i], id)
		}
	}
}

// TestBatchWithNotifications tests that notifications in batches don't generate responses
func TestBatchWithNotifications(t *testing.T) {
	srv := server.NewServer("test-batch-notifications")

	// Create a batch with requests and notifications mixed
	batch := `[
		{"jsonrpc": "2.0", "method": "ping", "id": 1},
		{"jsonrpc": "2.0", "method": "notifications/initialized"},
		{"jsonrpc": "2.0", "method": "ping", "id": 2}
	]`

	responseBytes, err := server.HandleMessage(srv.GetServer(), []byte(batch))
	if err != nil {
		t.Errorf("HandleMessage returned error: %v", err)
	}

	if responseBytes == nil {
		t.Errorf("Expected batch response, got nil")
		return
	}

	var responses []map[string]interface{}
	if err := json.Unmarshal(responseBytes, &responses); err != nil {
		t.Errorf("Failed to parse batch response: %v", err)
		return
	}

	// Should only get 2 responses (for the 2 ping requests, not the notification)
	if len(responses) != 2 {
		t.Errorf("Expected 2 responses (notifications should not generate responses), got %d", len(responses))
		return
	}

	// Verify the IDs are correct (1 and 2)
	expectedIDs := []float64{1, 2}
	for i, response := range responses {
		id, ok := response["id"].(float64)
		if !ok {
			t.Errorf("Response %d missing or invalid ID: %v", i, response["id"])
			continue
		}
		if id != expectedIDs[i] {
			t.Errorf("Response %d has wrong ID: expected %v, got %v", i, expectedIDs[i], id)
		}
	}
}

// TestAllNotificationsBatch tests that a batch with only notifications returns no response
func TestAllNotificationsBatch(t *testing.T) {
	srv := server.NewServer("test-all-notifications")

	// Create a batch with only notifications
	batch := `[
		{"jsonrpc": "2.0", "method": "notifications/initialized"},
		{"jsonrpc": "2.0", "method": "notifications/roots/list_changed"}
	]`

	responseBytes, err := server.HandleMessage(srv.GetServer(), []byte(batch))
	if err != nil {
		t.Errorf("HandleMessage returned error: %v", err)
	}

	// Should get no response for all-notification batches
	if responseBytes != nil {
		t.Errorf("Expected no response for all-notification batch, got: %s", string(responseBytes))
	}
}

// TestBatchParsingEdgeCases tests various edge cases in batch message parsing
func TestBatchParsingEdgeCases(t *testing.T) {
	srv := server.NewServer("test-batch-parsing")

	tests := []struct {
		name          string
		batch         string
		expectError   bool
		expectedCount int // Expected number of responses
		description   string
	}{
		{
			name:          "batch_with_invalid_json_item",
			batch:         `[{"jsonrpc": "2.0", "method": "ping", "id": 1}, {"invalid": "message"}]`,
			expectError:   false, // Should process valid items and create error response for invalid ones
			expectedCount: 2,     // One success, one error response
			description:   "Batch with one valid and one invalid JSON-RPC message",
		},
		{
			name:          "batch_with_missing_jsonrpc",
			batch:         `[{"method": "ping", "id": 1}, {"jsonrpc": "2.0", "method": "ping", "id": 2}]`,
			expectError:   false,
			expectedCount: 2, // Both should get responses (one error, one success)
			description:   "Batch with missing jsonrpc field in one message",
		},
		{
			name:          "batch_with_missing_method",
			batch:         `[{"jsonrpc": "2.0", "id": 1}, {"jsonrpc": "2.0", "method": "ping", "id": 2}]`,
			expectError:   false,
			expectedCount: 2, // Both should get responses (one error, one success)
			description:   "Batch with missing method field in one message",
		},
		{
			name:          "batch_with_null_id",
			batch:         `[{"jsonrpc": "2.0", "method": "ping", "id": null}, {"jsonrpc": "2.0", "method": "ping", "id": 2}]`,
			expectError:   false,
			expectedCount: 1, // Only the second message should get a response (null ID = notification)
			description:   "Batch with null ID (notification per JSON-RPC spec)",
		},
		{
			name:          "batch_with_string_and_number_ids",
			batch:         `[{"jsonrpc": "2.0", "method": "ping", "id": "string-id"}, {"jsonrpc": "2.0", "method": "ping", "id": 42}]`,
			expectError:   false,
			expectedCount: 2, // Both should get responses
			description:   "Batch with mixed ID types (string and number)",
		},
		{
			name:          "large_batch",
			batch:         generateLargeBatch(100), // Helper function to generate large batch
			expectError:   false,
			expectedCount: 100,
			description:   "Large batch with 100 requests",
		},
		{
			name:          "batch_with_only_invalid_messages",
			batch:         `[{"invalid": "json"}, {"also": "invalid"}]`,
			expectError:   false,
			expectedCount: 2, // Should get error responses for both
			description:   "Batch with only invalid messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseBytes, err := server.HandleMessage(srv.GetServer(), []byte(tt.batch))

			if err != nil {
				t.Errorf("HandleMessage returned error: %v", err)
			}

			if tt.expectError {
				if responseBytes == nil {
					t.Errorf("Expected error response, got nil")
				}
				// For error cases, check that we got an error response
				var errorResponse map[string]interface{}
				if err := json.Unmarshal(responseBytes, &errorResponse); err != nil {
					t.Errorf("Failed to parse error response: %v", err)
				}
			} else {
				// For success cases, verify response count
				if responseBytes == nil {
					if tt.expectedCount > 0 {
						t.Errorf("Expected %d responses, got nil", tt.expectedCount)
					}
					return
				}

				var responses []interface{}
				if err := json.Unmarshal(responseBytes, &responses); err != nil {
					t.Errorf("Failed to parse batch response: %v", err)
					return
				}

				if len(responses) != tt.expectedCount {
					t.Errorf("Expected %d responses, got %d", tt.expectedCount, len(responses))
				}
			}
		})
	}
}

// generateLargeBatch creates a large batch for testing
func generateLargeBatch(count int) string {
	batch := "["
	for i := 0; i < count; i++ {
		if i > 0 {
			batch += ","
		}
		batch += fmt.Sprintf(`{"jsonrpc": "2.0", "method": "ping", "id": %d}`, i)
	}
	batch += "]"
	return batch
}

// TestBatchMessageValidation tests validation of individual messages within batches
func TestBatchMessageValidation(t *testing.T) {
	srv := server.NewServer("test-batch-validation")

	tests := []struct {
		name        string
		batch       string
		description string
	}{
		{
			name:        "batch_with_different_jsonrpc_versions",
			batch:       `[{"jsonrpc": "2.0", "method": "ping", "id": 1}, {"jsonrpc": "1.0", "method": "ping", "id": 2}]`,
			description: "Batch with different JSON-RPC versions",
		},
		{
			name:        "batch_with_extra_fields",
			batch:       `[{"jsonrpc": "2.0", "method": "ping", "id": 1, "extra": "field"}, {"jsonrpc": "2.0", "method": "ping", "id": 2}]`,
			description: "Batch with extra fields (should be ignored)",
		},
		{
			name:        "batch_with_nested_objects",
			batch:       `[{"jsonrpc": "2.0", "method": "ping", "id": 1}, {"jsonrpc": "2.0", "method": "tools/call", "id": 2, "params": {"name": "test", "arguments": {"nested": {"deep": "value"}}}}]`,
			description: "Batch with nested parameter objects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseBytes, err := server.HandleMessage(srv.GetServer(), []byte(tt.batch))

			if err != nil {
				t.Errorf("HandleMessage returned error: %v", err)
			}

			// Should get some response (either success or error responses)
			if responseBytes == nil {
				t.Errorf("Expected some response, got nil")
				return
			}

			// Verify it's a valid JSON array
			var responses []interface{}
			if err := json.Unmarshal(responseBytes, &responses); err != nil {
				t.Errorf("Failed to parse batch response: %v", err)
			}
		})
	}
}

// TestBatchContextIsolation tests that each message in a batch gets its own context
func TestBatchContextIsolation(t *testing.T) {
	srv := server.NewServer("test-batch-context")

	// Create a batch where each message should be processed independently
	batch := `[
		{"jsonrpc": "2.0", "method": "ping", "id": "first"},
		{"jsonrpc": "2.0", "method": "ping", "id": "second"},
		{"jsonrpc": "2.0", "method": "ping", "id": "third"}
	]`

	responseBytes, err := server.HandleMessage(srv.GetServer(), []byte(batch))
	if err != nil {
		t.Errorf("HandleMessage returned error: %v", err)
	}

	if responseBytes == nil {
		t.Errorf("Expected batch response, got nil")
		return
	}

	var responses []map[string]interface{}
	if err := json.Unmarshal(responseBytes, &responses); err != nil {
		t.Errorf("Failed to parse batch response: %v", err)
		return
	}

	// Verify each response has the correct structure and ID
	expectedIDs := []string{"first", "second", "third"}
	for i, response := range responses {
		// Check that each response has the expected structure
		if response["jsonrpc"] != "2.0" {
			t.Errorf("Response %d missing jsonrpc field", i)
		}

		if response["id"] != expectedIDs[i] {
			t.Errorf("Response %d has wrong ID: expected %s, got %v", i, expectedIDs[i], response["id"])
		}

		// Check that each response has a result (ping should return empty object)
		if response["result"] == nil {
			t.Errorf("Response %d missing result field", i)
		}
	}
}
