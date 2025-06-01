package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

func TestExtractWorkspaceRoots(t *testing.T) {
	tests := []struct {
		name     string
		params   interface{}
		expected []string
	}{
		{
			name:     "nil params",
			params:   nil,
			expected: nil,
		},
		{
			name:     "non-map params",
			params:   "invalid",
			expected: nil,
		},
		{
			name:     "no clientInfo",
			params:   map[string]interface{}{},
			expected: nil,
		},
		{
			name: "clientInfo but no roots",
			params: map[string]interface{}{
				"clientInfo": map[string]interface{}{},
			},
			expected: nil,
		},
		{
			name: "empty roots array",
			params: map[string]interface{}{
				"clientInfo": map[string]interface{}{
					"roots": []interface{}{},
				},
			},
			expected: nil,
		},
		{
			name: "single file URI root",
			params: map[string]interface{}{
				"clientInfo": map[string]interface{}{
					"roots": []interface{}{
						map[string]interface{}{
							"uri":  "file:///Users/user/project",
							"name": "Project Root",
						},
					},
				},
			},
			expected: []string{"/Users/user/project"},
		},
		{
			name: "multiple file URI roots",
			params: map[string]interface{}{
				"clientInfo": map[string]interface{}{
					"roots": []interface{}{
						map[string]interface{}{
							"uri":  "file:///Users/user/project1",
							"name": "Project 1",
						},
						map[string]interface{}{
							"uri": "file:///Users/user/project2",
						},
					},
				},
			},
			expected: []string{"/Users/user/project1", "/Users/user/project2"},
		},
		{
			name: "mixed URIs (only file URIs should be included)",
			params: map[string]interface{}{
				"clientInfo": map[string]interface{}{
					"roots": []interface{}{
						map[string]interface{}{
							"uri": "file:///Users/user/project",
						},
						map[string]interface{}{
							"uri": "http://example.com/project",
						},
					},
				},
			},
			expected: []string{"/Users/user/project"},
		},
		{
			name: "invalid root entries",
			params: map[string]interface{}{
				"clientInfo": map[string]interface{}{
					"roots": []interface{}{
						"invalid",
						map[string]interface{}{
							"notUri": "value",
						},
						map[string]interface{}{
							"uri": 123, // non-string uri
						},
					},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We need to call the function through server initialization
			// since extractWorkspaceRoots is not exported
			svr := server.NewServer("test-server")

			// Create a mock initialization request with protocol version
			params := tt.params
			if params == nil {
				params = map[string]interface{}{}
			}

			// Add protocol version if not present
			if paramsMap, ok := params.(map[string]interface{}); ok {
				if _, hasVersion := paramsMap["protocolVersion"]; !hasVersion {
					paramsMap["protocolVersion"] = "2024-11-05"
				}
			}

			initJSON := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params":  params,
			}

			jsonBytes, err := json.Marshal(initJSON)
			if err != nil {
				t.Fatalf("Failed to marshal JSON: %v", err)
			}

			// Get initial root count
			initialRoots := len(svr.GetServer().GetRoots())

			// Process the initialization
			_, err = server.HandleMessage(svr.GetServer(), jsonBytes)
			if err != nil {
				t.Fatalf("Failed to process initialize request: %v", err)
			}

			// Check the roots were added correctly
			finalRoots := svr.GetServer().GetRoots()
			addedRoots := finalRoots[initialRoots:] // Get only the newly added roots

			if len(addedRoots) != len(tt.expected) {
				t.Errorf("Expected %d roots to be added, got %d", len(tt.expected), len(addedRoots))
				return
			}

			for i, expectedRoot := range tt.expected {
				if addedRoots[i] != expectedRoot {
					t.Errorf("Expected root %d to be %s, got %s", i, expectedRoot, addedRoots[i])
				}
			}
		})
	}
}

func TestUriToPath(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "simple file URI",
			uri:      "file:///Users/user/project",
			expected: "/Users/user/project",
		},
		{
			name:     "file URI with spaces",
			uri:      "file:///Users/user/my%20project",
			expected: "/Users/user/my project",
		},
		{
			name:     "file URI with special characters",
			uri:      "file:///Users/user/project%2Btest",
			expected: "/Users/user/project+test",
		},
		{
			name:     "non-file URI",
			uri:      "http://example.com/path",
			expected: "",
		},
		{
			name:     "empty URI",
			uri:      "",
			expected: "",
		},
		{
			name:     "invalid URI",
			uri:      "not-a-uri",
			expected: "",
		},
		{
			name:     "file URI without triple slash",
			uri:      "file://server/path",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We need to test this through the integration since uriToPath is not exported
			// We'll use a known input and check the output
			svr := server.NewServer("test-server")

			params := map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"clientInfo": map[string]interface{}{
					"roots": []interface{}{
						map[string]interface{}{
							"uri": tt.uri,
						},
					},
				},
			}

			initJSON := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params":  params,
			}

			jsonBytes, err := json.Marshal(initJSON)
			if err != nil {
				t.Fatalf("Failed to marshal JSON: %v", err)
			}

			initialRoots := len(svr.GetServer().GetRoots())

			// Process the initialization
			_, err = server.HandleMessage(svr.GetServer(), jsonBytes)
			if err != nil {
				t.Fatalf("Failed to process initialize request: %v", err)
			}

			finalRoots := svr.GetServer().GetRoots()
			addedRoots := finalRoots[initialRoots:]

			if tt.expected == "" {
				// Should not add any roots for invalid URIs
				if len(addedRoots) != 0 {
					t.Errorf("Expected no roots to be added for invalid URI %s, got %d", tt.uri, len(addedRoots))
				}
			} else {
				// Should add exactly one root with the expected path
				if len(addedRoots) != 1 {
					t.Errorf("Expected 1 root to be added for URI %s, got %d", tt.uri, len(addedRoots))
					return
				}
				if addedRoots[0] != tt.expected {
					t.Errorf("Expected path %s for URI %s, got %s", tt.expected, tt.uri, addedRoots[0])
				}
			}
		})
	}
}

func TestContextWorkspaceRoots(t *testing.T) {
	// Create a server and add some roots
	svr := server.NewServer("test-server")
	svr.Root("/path/to/root1", "/path/to/root2")

	// Add a test tool to examine the context
	var capturedContext *server.Context
	svr.Tool("test-context", "Test context", func(ctx *server.Context, args interface{}) (interface{}, error) {
		capturedContext = ctx
		return "ok", nil
	})

	// Create a tool call request
	toolCallJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "test-context",
			"arguments": {}
		}
	}`)

	// Process the tool call
	_, err := server.HandleMessage(svr.GetServer(), toolCallJSON)
	if err != nil {
		t.Fatalf("Failed to process tool call: %v", err)
	}

	// Check that context was captured
	if capturedContext == nil {
		t.Fatal("Context was not captured")
	}

	// Test GetRoots()
	roots := capturedContext.GetRoots()
	if len(roots) != 2 {
		t.Errorf("Expected 2 roots, got %d", len(roots))
	}

	expectedRoots := []string{"/path/to/root1", "/path/to/root2"}
	for i, expected := range expectedRoots {
		if roots[i] != expected {
			t.Errorf("Expected root %d to be %s, got %s", i, expected, roots[i])
		}
	}

	// Test GetPrimaryRoot() - should return first root
	primaryRoot := capturedContext.GetPrimaryRoot()
	if primaryRoot != "/path/to/root1" {
		t.Errorf("Expected primary root to be /path/to/root1, got %s", primaryRoot)
	}

	// Test InRoots()
	tests := []struct {
		path     string
		expected bool
		name     string
	}{
		{"/path/to/root1/file.txt", true, "file in root1"},
		{"/path/to/root2/subdir/file.txt", true, "file in root2"},
		{"/path/to/root1", true, "exactly root1"},
		{"/path/to/root3/file.txt", false, "file not in any root"},
		{"/path/to/file.txt", false, "file outside roots"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := capturedContext.InRoots(test.path)
			if result != test.expected {
				t.Errorf("Expected InRoots(%s) to be %v, got %v",
					test.path, test.expected, result)
			}
		})
	}
}

func TestContextWorkspaceRootsEmpty(t *testing.T) {
	// Create a server with no roots
	svr := server.NewServer("test-server")

	// Add a test tool to examine the context
	var capturedContext *server.Context
	svr.Tool("test-context", "Test context", func(ctx *server.Context, args interface{}) (interface{}, error) {
		capturedContext = ctx
		return "ok", nil
	})

	// Create a tool call request
	toolCallJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "test-context",
			"arguments": {}
		}
	}`)

	// Process the tool call
	_, err := server.HandleMessage(svr.GetServer(), toolCallJSON)
	if err != nil {
		t.Fatalf("Failed to process tool call: %v", err)
	}

	// Check that context was captured
	if capturedContext == nil {
		t.Fatal("Context was not captured")
	}

	// Test GetRoots() with empty roots
	roots := capturedContext.GetRoots()
	if len(roots) != 0 {
		t.Errorf("Expected 0 roots, got %d", len(roots))
	}

	// Test GetPrimaryRoot() with empty roots
	primaryRoot := capturedContext.GetPrimaryRoot()
	if primaryRoot != "" {
		t.Errorf("Expected empty primary root, got %s", primaryRoot)
	}

	// Test InRoots() with empty roots
	if capturedContext.InRoots("/any/path") {
		t.Error("Expected InRoots to return false when no roots are set")
	}
}

func TestWorkspaceRootsIntegration(t *testing.T) {
	// Test the full integration: initialization with workspace roots -> context usage
	svr := server.NewServer("test-server")

	// Initialize with workspace roots
	initParams := map[string]interface{}{
		"clientInfo": map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{
					"uri":  "file:///Users/user/project1",
					"name": "Project 1",
				},
				map[string]interface{}{
					"uri": "file:///Users/user/project2",
				},
			},
		},
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
	}

	initJSON := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  initParams,
	}

	jsonBytes, err := json.Marshal(initJSON)
	if err != nil {
		t.Fatalf("Failed to marshal initialization JSON: %v", err)
	}

	// Process initialization
	_, err = server.HandleMessage(svr.GetServer(), jsonBytes)
	if err != nil {
		t.Fatalf("Failed to process initialize request: %v", err)
	}

	// Verify that roots were actually added to the server
	serverRoots := svr.GetServer().GetRoots()
	if len(serverRoots) != 2 {
		t.Fatalf("Expected server to have 2 roots after initialization, got %d: %v", len(serverRoots), serverRoots)
	}

	// Add a test tool to examine the context after initialization
	svr.Tool("test-workspace", "Test workspace", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"workspaceRoots": ctx.GetRoots(),
			"primaryRoot":    ctx.GetPrimaryRoot(),
			"inWorkspace":    ctx.InRoots("/Users/user/project1/file.txt"),
			"notInWorkspace": ctx.InRoots("/other/path/file.txt"),
		}, nil
	})

	// Call the tool
	toolCallJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/call",
		"params": {
			"name": "test-workspace",
			"arguments": {}
		}
	}`)

	responseBytes, err := server.HandleMessage(svr.GetServer(), toolCallJSON)
	if err != nil {
		t.Fatalf("Failed to process tool call: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check the response contains our workspace data
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Response result is not a map, got type %T with value %v", response["result"], response["result"])
	}

	// Extract the actual tool result from MCP response format
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("Response content is not an array or is empty, got %+v", result)
	}

	contentItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Content item is not a map, got %T", content[0])
	}

	textResult, ok := contentItem["text"].(string)
	if !ok {
		t.Fatalf("Content text is not a string, got %T", contentItem["text"])
	}

	// Parse the JSON string to get the actual tool response
	var toolResult map[string]interface{}
	if err := json.Unmarshal([]byte(textResult), &toolResult); err != nil {
		t.Fatalf("Failed to parse tool result JSON: %v", err)
	}

	// Verify workspace roots were properly set in context
	workspaceRootsRaw := toolResult["workspaceRoots"]
	workspaceRoots, ok := workspaceRootsRaw.([]interface{})
	if !ok {
		t.Fatalf("workspaceRoots is not an array, got type %T with value %v", workspaceRootsRaw, workspaceRootsRaw)
	}

	if len(workspaceRoots) != 2 {
		t.Errorf("Expected 2 workspace roots, got %d", len(workspaceRoots))
	}

	expectedRoots := []string{"/Users/user/project1", "/Users/user/project2"}
	for i, expected := range expectedRoots {
		if workspaceRoots[i].(string) != expected {
			t.Errorf("Expected workspace root %d to be %s, got %s", i, expected, workspaceRoots[i])
		}
	}

	// Verify primary root
	primaryRoot, ok := toolResult["primaryRoot"].(string)
	if !ok || primaryRoot != "/Users/user/project1" {
		t.Errorf("Expected primary root to be /Users/user/project1, got %s", primaryRoot)
	}

	// Verify InWorkspace functionality
	inWorkspace, ok := toolResult["inWorkspace"].(bool)
	if !ok || !inWorkspace {
		t.Error("Expected file in workspace to return true")
	}

	notInWorkspace, ok := toolResult["notInWorkspace"].(bool)
	if !ok || notInWorkspace {
		t.Error("Expected file not in workspace to return false")
	}
}

func TestWorkspaceRootsNoClientInfo(t *testing.T) {
	// Test initialization without clientInfo doesn't break anything
	svr := server.NewServer("test-server")

	// Initialize without clientInfo
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
	}

	initJSON := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  initParams,
	}

	jsonBytes, err := json.Marshal(initJSON)
	if err != nil {
		t.Fatalf("Failed to marshal initialization JSON: %v", err)
	}

	// Process initialization - should not fail
	_, err = server.HandleMessage(svr.GetServer(), jsonBytes)
	if err != nil {
		t.Fatalf("Failed to process initialize request: %v", err)
	}

	// Server should have no roots added
	roots := svr.GetServer().GetRoots()
	if len(roots) != 0 {
		t.Errorf("Expected no roots to be added, got %d", len(roots))
	}
}
