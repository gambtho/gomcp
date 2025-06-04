package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
)

// TestListPromptsWithRealServerData tests ListPrompts with real server-generated data
func TestListPromptsWithRealServerData(t *testing.T) {
	// Create a server with real prompts to generate actual data
	srv := server.NewServer("test-prompt-server")

	// Add various types of prompts to test different scenarios
	srv.Prompt("greeting", "Generate a personalized greeting",
		server.User("Hello {{name}}! Welcome to {{platform}}."))

	srv.Prompt("code-review", "Review code for best practices",
		server.User("Please review this {{language}} code for best practices:\n\n{{code}}"),
		server.Assistant("I'll review your code for best practices, security issues, and optimization opportunities."))

	srv.Prompt("summary", "Summarize content with specified length",
		server.User("Summarize the following content in {{max_words}} words or less:\n\n{{content}}"))

	srv.Prompt("no-variables", "A prompt without any variables",
		server.User("This is a simple prompt with no template variables."))

	// Get the real server implementation to generate actual prompts/list response
	serverImpl := srv.GetServer()

	// Create a request for prompts/list
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "prompts/list"
	}`)

	// Get the real server response
	responseBytes, err := server.HandleMessage(serverImpl, requestJSON)
	if err != nil {
		t.Fatalf("Failed to get real server response: %v", err)
	}

	// Debug: Log the real server response
	t.Logf("Real server response: %s", string(responseBytes))

	// Create a mock transport with standard setup
	m := SetupMockTransport("2025-03-26")

	// Clear the response queue to remove the default prompts/list response
	m.ClearResponses()

	// Re-add essential responses for client initialization
	initResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"serverInfo": map[string]interface{}{
				"name":    "Test Server",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"enhancedResources": true,
				"multipleRoots":     true,
			},
			"versions": []string{"draft", "2024-11-05", "2025-03-26"},
		},
	}
	initJSON, _ := json.Marshal(initResponse)
	m.QueueConditionalResponse(initJSON, nil, IsRequestMethod("initialize"))

	// Add response for notifications/initialized
	m.QueueConditionalResponse(
		[]byte(`{"jsonrpc":"2.0","result":null}`),
		nil,
		IsRequestMethod("notifications/initialized"),
	)

	// Add the real server response for prompts/list
	m.QueueConditionalResponse(
		responseBytes,
		nil,
		func(message []byte) bool {
			isMatch := IsRequestMethod("prompts/list")(message)
			if isMatch {
				t.Logf("Conditional response matched prompts/list request: %s", string(message))
			}
			return isMatch
		},
	)

	// Create client with mock transport but using the real server data
	c, err := client.NewClient("test://server",
		client.WithTransport(m),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Test ListPrompts - this will use the real server response
	prompts, err := c.ListPrompts()
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}

	t.Logf("Retrieved %d prompts from real server data", len(prompts))

	// Debug: Log the actual mock transport request history
	history := m.GetRequestHistory()
	for _, req := range history {
		if req.Method == "prompts/list" {
			t.Logf("Found prompts/list request in history: %s", string(req.Message))
		}
	}

	// Verify we got all the prompts we added
	if len(prompts) != 4 {
		t.Errorf("Expected 4 prompts, got %d", len(prompts))
	}

	// Create a map for easier lookup
	promptMap := make(map[string]client.Prompt)
	for _, prompt := range prompts {
		promptMap[prompt.Name] = prompt
		t.Logf("Prompt: %s - %s (args: %d)", prompt.Name, prompt.Description, len(prompt.Arguments))
	}

	// Verify specific prompts exist and have correct properties
	testCases := []struct {
		name        string
		description string
		argCount    int
		argNames    []string
	}{
		{
			name:        "greeting",
			description: "Generate a personalized greeting",
			argCount:    2,
			argNames:    []string{"name", "platform"},
		},
		{
			name:        "code-review",
			description: "Review code for best practices",
			argCount:    2,
			argNames:    []string{"language", "code"},
		},
		{
			name:        "summary",
			description: "Summarize content with specified length",
			argCount:    2,
			argNames:    []string{"max_words", "content"},
		},
		{
			name:        "no-variables",
			description: "A prompt without any variables",
			argCount:    0,
			argNames:    []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prompt, exists := promptMap[tc.name]
			if !exists {
				t.Fatalf("Prompt %s not found in response", tc.name)
			}

			if prompt.Description != tc.description {
				t.Errorf("Expected description %q, got %q", tc.description, prompt.Description)
			}

			if len(prompt.Arguments) != tc.argCount {
				t.Errorf("Expected %d arguments, got %d", tc.argCount, len(prompt.Arguments))
			}

			// Verify argument names (order may vary)
			if len(tc.argNames) > 0 {
				foundArgs := make(map[string]bool)
				for _, arg := range prompt.Arguments {
					foundArgs[arg.Name] = true
				}

				for _, expectedArg := range tc.argNames {
					if !foundArgs[expectedArg] {
						t.Errorf("Expected argument %q not found in prompt %s", expectedArg, tc.name)
					}
				}
			}
		})
	}

	// Also test that the raw response structure is correct
	t.Run("RawResponseStructure", func(t *testing.T) {
		var response map[string]interface{}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		// Check basic JSON-RPC structure
		if response["jsonrpc"] != "2.0" {
			t.Errorf("Expected jsonrpc 2.0, got %v", response["jsonrpc"])
		}

		if response["id"] != float64(1) {
			t.Errorf("Expected id 1, got %v", response["id"])
		}

		// Check result structure
		result, ok := response["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be object, got %T", response["result"])
		}

		// Check prompts array
		prompts, ok := result["prompts"].([]interface{})
		if !ok {
			t.Fatalf("Expected prompts to be array, got %T", result["prompts"])
		}

		if len(prompts) != 4 {
			t.Errorf("Expected 4 prompts in raw response, got %d", len(prompts))
		}

		t.Logf("Raw response structure is correct with %d prompts", len(prompts))
	})
}

// TestListPromptsEmpty tests ListPrompts with a server that has no prompts
func TestListPromptsEmpty(t *testing.T) {
	// Create a server with NO prompts
	srv := server.NewServer("test-empty-server")

	// Get the real server implementation
	serverImpl := srv.GetServer()

	// Create a request for prompts/list
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "prompts/list"
	}`)

	// Get the real server response (should be empty)
	responseBytes, err := server.HandleMessage(serverImpl, requestJSON)
	if err != nil {
		t.Fatalf("Failed to get real server response: %v", err)
	}

	// Create a mock transport with the real empty response
	m := SetupMockTransport("2025-03-26")

	// Clear the response queue to remove the default prompts/list response
	m.ClearResponses()

	// Re-add essential responses for client initialization
	initResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"serverInfo": map[string]interface{}{
				"name":    "Test Server",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"enhancedResources": true,
				"multipleRoots":     true,
			},
			"versions": []string{"draft", "2024-11-05", "2025-03-26"},
		},
	}
	initJSON, _ := json.Marshal(initResponse)
	m.QueueConditionalResponse(initJSON, nil, IsRequestMethod("initialize"))

	// Add response for notifications/initialized
	m.QueueConditionalResponse(
		[]byte(`{"jsonrpc":"2.0","result":null}`),
		nil,
		IsRequestMethod("notifications/initialized"),
	)

	// Add the real server response for prompts/list (empty)
	m.QueueConditionalResponse(
		responseBytes,
		nil,
		IsRequestMethod("prompts/list"),
	)

	// Create client with mock transport
	c, err := client.NewClient("test://server",
		client.WithTransport(m),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Test ListPrompts on empty server
	prompts, err := c.ListPrompts()
	if err != nil {
		t.Fatalf("ListPrompts failed on empty server: %v", err)
	}

	if len(prompts) != 0 {
		t.Errorf("Expected 0 prompts from empty server, got %d", len(prompts))
	}

	t.Log("Successfully handled empty prompts list with real server data")
}
