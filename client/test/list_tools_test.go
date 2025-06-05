package test

import (
	"encoding/json"
	"testing"
)

func TestListTools(t *testing.T) {
	// Create client with mock transport
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Add tools/list response
	toolsResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2, // ID 1 is used for initialization
		"result": map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "calculator",
					"description": "Perform mathematical calculations",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"operation": map[string]interface{}{
								"type": "string",
								"enum": []string{"add", "subtract", "multiply", "divide"},
							},
							"values": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "number",
								},
							},
						},
						"required": []string{"operation", "values"},
					},
					"outputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"result": map[string]interface{}{
								"type": "number",
							},
						},
						"required": []string{"result"},
					},
				},
				map[string]interface{}{
					"name":        "echo",
					"description": "Echo back the input text",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"text": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"text"},
					},
				},
			},
		},
	}

	toolsJSON, err := json.Marshal(toolsResponse)
	if err != nil {
		t.Fatalf("Failed to marshal tools response: %v", err)
	}

	// Queue the response for tools/list request
	m.QueueConditionalResponse(
		toolsJSON,
		nil,
		func(req []byte) bool {
			return isRequestMethod(req, "tools/list")
		},
	)

	// Test ListTools
	tools, err := c.ListTools()
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	// Verify we got the expected tools
	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(tools))
	}

	// Check that tools have the expected structure
	expectedTools := []string{"calculator", "echo"}
	for i, tool := range tools {
		if tool.Name != expectedTools[i] {
			t.Errorf("Expected tool %d to be %s, got %s", i, expectedTools[i], tool.Name)
		}
		if tool.Name == "" {
			t.Error("Tool name should not be empty")
		}
		if tool.InputSchema == nil {
			t.Error("Tool input schema should not be nil")
		}

		// Check outputSchema for calculator tool (draft spec feature)
		if tool.Name == "calculator" && tool.OutputSchema != nil {
			if schemaType, ok := tool.OutputSchema["type"].(string); !ok || schemaType != "object" {
				t.Error("Calculator tool outputSchema should have type 'object'")
			}
		}
	}

	t.Logf("Successfully retrieved %d tools", len(tools))
	for _, tool := range tools {
		t.Logf("Tool: %s - %s", tool.Name, tool.Description)
	}
}

func TestListToolsWithPagination(t *testing.T) {
	// Create client with mock transport
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Add first page response
	firstPageResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "tool1",
					"description": "First tool",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"param1": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			"nextCursor": "page2",
		},
	}

	// Add second page response
	secondPageResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"result": map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "tool2",
					"description": "Second tool",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"param2": map[string]interface{}{
								"type": "number",
							},
						},
					},
				},
			},
			// No nextCursor means this is the last page
		},
	}

	firstPageJSON, _ := json.Marshal(firstPageResponse)
	secondPageJSON, _ := json.Marshal(secondPageResponse)

	// Queue responses for pagination
	m.QueueConditionalResponse(
		firstPageJSON,
		nil,
		func(req []byte) bool {
			var request map[string]interface{}
			if err := json.Unmarshal(req, &request); err != nil {
				return false
			}

			if method, ok := request["method"].(string); !ok || method != "tools/list" {
				return false
			}

			// First request should have no cursor or empty cursor
			params, ok := request["params"].(map[string]interface{})
			if !ok {
				return true // No params means first request
			}

			cursor, exists := params["cursor"]
			return !exists || cursor == ""
		},
	)

	m.QueueConditionalResponse(
		secondPageJSON,
		nil,
		func(req []byte) bool {
			var request map[string]interface{}
			if err := json.Unmarshal(req, &request); err != nil {
				return false
			}

			if method, ok := request["method"].(string); !ok || method != "tools/list" {
				return false
			}

			// Second request should have cursor = "page2"
			params, ok := request["params"].(map[string]interface{})
			if !ok {
				return false
			}

			cursor, ok := params["cursor"].(string)
			return ok && cursor == "page2"
		},
	)

	// Test ListTools with pagination
	tools, err := c.ListTools()
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	// Should get all tools from both pages
	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(tools))
	}

	expectedNames := []string{"tool1", "tool2"}
	for i, tool := range tools {
		if tool.Name != expectedNames[i] {
			t.Errorf("Expected tool %d to be %s, got %s", i, expectedNames[i], tool.Name)
		}
	}

	t.Logf("Successfully retrieved %d tools across multiple pages", len(tools))
}

func TestListToolsDebugOutput(t *testing.T) {
	// Create client with mock transport using debug logger
	c, m := SetupClientWithMockTransport(t, "2025-03-26")
	defer c.Close()

	// Add tools/list response
	toolsResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "calculator",
					"description": "Perform mathematical calculations",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"operation": map[string]interface{}{
								"type": "string",
							},
							"values": map[string]interface{}{
								"type": "array",
							},
						},
					},
				},
			},
		},
	}

	toolsJSON, err := json.Marshal(toolsResponse)
	if err != nil {
		t.Fatalf("Failed to marshal tools response: %v", err)
	}

	// Queue the response for tools/list request
	m.QueueConditionalResponse(
		toolsJSON,
		nil,
		func(req []byte) bool {
			return isRequestMethod(req, "tools/list")
		},
	)

	// Test ListTools - should show debug output
	tools, err := c.ListTools()
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	t.Logf("Successfully retrieved %d tools with debug output visible above", len(tools))
}
