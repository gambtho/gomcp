package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

// TestEnsureContentsArray tests the ensureContentsArray function for MCP compliance
func TestEnsureContentsArray(t *testing.T) {
	testCases := []struct {
		name     string
		response map[string]interface{}
		uri      string
		expected map[string]interface{}
	}{
		{
			name:     "Empty response creates default contents",
			response: map[string]interface{}{},
			uri:      "/test/resource",
			expected: map[string]interface{}{
				"contents": []map[string]interface{}{
					{
						"uri":  "/test/resource",
						"text": "Empty content",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Empty content",
							},
						},
					},
				},
			},
		},
		{
			name: "Response with content array converts to contents",
			response: map[string]interface{}{
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Hello World",
					},
				},
			},
			uri: "/test/resource",
			expected: map[string]interface{}{
				"contents": []map[string]interface{}{
					{
						"uri":  "/test/resource",
						"text": "Hello World",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Hello World",
							},
						},
					},
				},
			},
		},
		{
			name: "Response with existing contents array ensures URI",
			response: map[string]interface{}{
				"contents": []interface{}{
					map[string]interface{}{
						"text": "Existing content",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Existing content",
							},
						},
					},
				},
			},
			uri: "/test/resource",
			expected: map[string]interface{}{
				"contents": []map[string]interface{}{
					{
						"uri":  "/test/resource",
						"text": "Existing content",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Existing content",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use reflection to call the internal function
			result := server.TestEnsureContentsArray(tc.response, tc.uri)

			// Convert to JSON for comparison
			resultJSON, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("Failed to marshal result: %v", err)
			}

			expectedJSON, err := json.Marshal(tc.expected)
			if err != nil {
				t.Fatalf("Failed to marshal expected: %v", err)
			}

			if string(resultJSON) != string(expectedJSON) {
				t.Errorf("Result mismatch.\nExpected: %s\nGot: %s", expectedJSON, resultJSON)
			}
		})
	}
}

// TestEnsureValidContentItems tests content item validation
func TestEnsureValidContentItems(t *testing.T) {
	testCases := []struct {
		name     string
		items    []interface{}
		expected []interface{}
	}{
		{
			name: "Valid text content items",
			items: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Hello World",
				},
			},
			expected: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Hello World",
				},
			},
		},
		{
			name: "Text content missing text field gets default",
			items: []interface{}{
				map[string]interface{}{
					"type": "text",
				},
			},
			expected: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Missing text",
				},
			},
		},
		{
			name: "Invalid image content is skipped",
			items: []interface{}{
				map[string]interface{}{
					"type": "image",
					// Missing imageUrl
				},
			},
			expected: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "No valid content",
				},
			},
		},
		{
			name: "Valid image content is preserved",
			items: []interface{}{
				map[string]interface{}{
					"type":     "image",
					"imageUrl": "https://example.com/image.jpg",
				},
			},
			expected: []interface{}{
				map[string]interface{}{
					"type":     "image",
					"imageUrl": "https://example.com/image.jpg",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := server.TestEnsureValidContentItems(tc.items)

			resultJSON, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("Failed to marshal result: %v", err)
			}

			expectedJSON, err := json.Marshal(tc.expected)
			if err != nil {
				t.Fatalf("Failed to marshal expected: %v", err)
			}

			if string(resultJSON) != string(expectedJSON) {
				t.Errorf("Result mismatch.\nExpected: %s\nGot: %s", expectedJSON, resultJSON)
			}
		})
	}
}
