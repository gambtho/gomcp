package v20241105

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/client/test"
	"github.com/localrivet/gomcp/mcp"
)

// No need to define MockTransport here as we're using the shared one from client/test/mocktransport.go

// setupTest creates a client with a mock transport and ensures it's initialized
func setupTest(t *testing.T) (client.Client, *test.MockTransport) {
	mockTransport := test.SetupMockTransport("2024-11-05")

	// Create a new client with the mock transport
	c, err := client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Verify the correct protocol version was negotiated
	if c.Version() != "2024-11-05" {
		t.Fatalf("Expected protocol version 2024-11-05, got %s", c.Version())
	}

	// Reset the mock transport's response queue
	mockTransport.ClearResponses()

	return c, mockTransport
}

func TestClientInitialization_v20241105(t *testing.T) {
	mockTransport := test.SetupMockTransport("2024-11-05")

	// Create a new client with the mock transport
	c, err := client.NewClient("test://server",
		client.WithTransport(mockTransport),
		client.WithVersionDetector(mcp.NewVersionDetector()),
	)
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// The Connect method will be automatically called when needed
	// Just verify the client is in the expected state
	if !mockTransport.ConnectCalled {
		// Manually call a method that should trigger a connection
		_, err := c.GetResource("/")
		if err != nil {
			t.Fatalf("Failed to trigger connection: %v", err)
		}

		if !mockTransport.ConnectCalled {
			t.Error("Connect was not called on the transport")
		}
	}

	// Verify the client is connected
	if !c.IsConnected() {
		t.Error("Client should be connected")
	}

	// Verify the client is initialized
	if !c.IsInitialized() {
		t.Error("Client should be initialized")
	}

	// Verify the correct protocol version was negotiated
	if c.Version() != "2024-11-05" {
		t.Errorf("Expected protocol version 2024-11-05, got %s", c.Version())
	}

	// Test closing the client
	if err := c.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify the transport was disconnected
	if !mockTransport.DisconnectCalled {
		t.Error("Disconnect was not called on the transport")
	}

	// Verify the client is no longer connected
	if c.IsConnected() {
		t.Error("Client should not be connected after close")
	}
}

func TestGetResource_v20241105(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up the mock response for the resource request
	// Note: In 2024-11-05, the resource response has a different structure from 2025-03-26
	resourceResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"result": map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Hello, World!",
				},
			},
		},
	}

	responseJSON, _ := json.Marshal(resourceResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	// Call GetResource
	result, err := c.GetResource("/test/resource")
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}

	// Verify the result - now returns concrete ResourceResponse type
	if result == nil {
		t.Fatalf("Expected ResourceResponse result, got nil")
	}

	if len(result.Content) == 0 {
		t.Fatalf("Expected result to have content array, got %v", result)
	}

	// Parse the sent request to verify it matches the 2024-11-05 spec
	var sentRequest map[string]interface{}
	if err := json.Unmarshal(mockTransport.LastSentMessage, &sentRequest); err != nil {
		t.Fatalf("Failed to parse sent request: %v", err)
	}

	params, ok := sentRequest["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params in sent request, got %v", sentRequest)
	}

	path, ok := params["uri"].(string)
	if !ok || path != "/test/resource" {
		t.Errorf("Expected uri parameter to be /test/resource, got %v", params)
	}

	method, ok := sentRequest["method"].(string)
	if !ok || method != "resources/read" {
		t.Errorf("Expected method to be resources/read, got %v", sentRequest)
	}
}

func TestCallTool_v20241105(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up the mock response for the tool request
	toolResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"result": map[string]interface{}{
			"output": "Tool executed successfully",
		},
	}

	responseJSON, _ := json.Marshal(toolResponse)
	mockTransport.QueueResponse(responseJSON, nil)

	// Call the tool
	result, err := c.CallTool("test-tool", map[string]interface{}{
		"param1": "value1",
		"param2": 42,
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	// Verify the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	output, ok := resultMap["output"].(string)
	if !ok || output != "Tool executed successfully" {
		t.Errorf("Expected output to be 'Tool executed successfully', got %v", resultMap)
	}

	// Parse the sent request to verify it matches the 2024-11-05 spec
	var sentRequest map[string]interface{}
	if err := json.Unmarshal(mockTransport.LastSentMessage, &sentRequest); err != nil {
		t.Fatalf("Failed to parse sent request: %v", err)
	}

	params, ok := sentRequest["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params in sent request, got %v", sentRequest)
	}

	name, ok := params["name"].(string)
	if !ok || name != "test-tool" {
		t.Errorf("Expected name parameter to be test-tool, got %v", params)
	}

	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected arguments parameter, got %v", params)
	}

	if args["param1"] != "value1" || args["param2"] != float64(42) {
		t.Errorf("Expected argument values to match, got %v", args)
	}

	method, ok := sentRequest["method"].(string)
	if !ok || method != "tools/call" {
		t.Errorf("Expected method to be tools/call, got %v", sentRequest)
	}
}

func TestGetPrompt_v20241105(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Set up mock response
	mockResponse := test.CreatePromptResponse("Hello {{name}}", "Hello Test User")
	mockTransport.QueueResponse(mockResponse, nil)

	// Call the method
	result, err := c.GetPrompt("test-prompt", map[string]interface{}{
		"name": "Test User",
	})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	// Verify the result - now returns concrete PromptResponse type
	if result == nil {
		t.Fatalf("Expected PromptResponse result, got nil")
	}

	if len(result.Messages) == 0 {
		t.Fatalf("Expected at least one message in prompt response")
	}

	// Check that the message was rendered correctly
	firstMessage := result.Messages[0]
	if firstMessage.Content.Text != "Hello Test User" {
		t.Errorf("Expected rendered text 'Hello Test User', got %v", firstMessage.Content.Text)
	}
}

// TestRoots_v20241105 tests root management operations in the 2024-11-05 version using correct MCP protocol
func TestRoots_v20241105(t *testing.T) {
	c, mockTransport := setupTest(t)

	// Test add root - should only send notifications/roots/list_changed
	err := c.AddRoot("file:///test/2024-11-05/root", "2024-11-05 Test Root")
	if err != nil {
		t.Fatalf("AddRoot failed: %v", err)
	}

	// Verify that NO roots/add request was sent (this method doesn't exist in MCP)
	addRequests := mockTransport.GetRequestsByMethod("roots/add")
	if len(addRequests) != 0 {
		t.Errorf("Expected 0 roots/add requests (method doesn't exist in MCP), got %d", len(addRequests))
	}

	// Verify that notifications/roots/list_changed was sent (correct MCP behavior)
	// Wait for asynchronous notification to be sent
	if !mockTransport.WaitForNotification("notifications/roots/list_changed", 1*time.Second) {
		t.Fatal("Timeout waiting for roots/list_changed notification")
	}

	notifications := mockTransport.GetRequestsByMethod("notifications/roots/list_changed")
	if len(notifications) != 1 {
		t.Fatalf("Expected 1 roots/list_changed notification, got %d", len(notifications))
	}

	// Test get roots - should read from local cache, no server requests
	roots, err := c.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots failed: %v", err)
	}

	if len(roots) != 1 {
		t.Fatalf("Expected 1 root, got %d", len(roots))
	}

	if roots[0].URI != "file:///test/2024-11-05/root" || roots[0].Name != "2024-11-05 Test Root" {
		t.Errorf("Root doesn't match expected: %+v", roots[0])
	}

	// Verify that NO roots/list request was sent (GetRoots uses local cache)
	listRequests := mockTransport.GetRequestsByMethod("roots/list")
	if len(listRequests) != 0 {
		t.Errorf("Expected 0 roots/list requests (GetRoots uses local cache), got %d", len(listRequests))
	}

	// Test remove root - should only send notifications/roots/list_changed
	err = c.RemoveRoot("file:///test/2024-11-05/root")
	if err != nil {
		t.Fatalf("RemoveRoot failed: %v", err)
	}

	// Verify that NO roots/remove request was sent (this method doesn't exist in MCP)
	removeRequests := mockTransport.GetRequestsByMethod("roots/remove")
	if len(removeRequests) != 0 {
		t.Errorf("Expected 0 roots/remove requests (method doesn't exist in MCP), got %d", len(removeRequests))
	}

	// Wait for the second notification
	if !mockTransport.WaitForNotification("notifications/roots/list_changed", 1*time.Second) {
		t.Fatal("Timeout waiting for second roots/list_changed notification")
	}

	// Verify that a second notifications/roots/list_changed was sent
	notifications = mockTransport.GetRequestsByMethod("notifications/roots/list_changed")
	if len(notifications) != 2 {
		t.Fatalf("Expected 2 roots/list_changed notifications (add + remove), got %d", len(notifications))
	}

	// Verify root was removed from local cache
	roots, err = c.GetRoots()
	if err != nil {
		t.Fatalf("GetRoots failed after remove: %v", err)
	}

	if len(roots) != 0 {
		t.Errorf("Expected 0 roots after removal, got %d", len(roots))
	}
}
