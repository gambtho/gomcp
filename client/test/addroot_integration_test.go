package test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
)

// TestAddRoot_Integration tests AddRoot against a real MCP server
// This test is skipped by default and should be run manually when debugging
func TestAddRoot_Integration(t *testing.T) {
	// Skip by default unless explicitly running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=1 to run.")
	}

	// Create client with stdio transport (most common for MCP servers)
	// Note: This test requires an actual MCP server to be available
	c, err := client.NewClient("stdio:///",
		client.WithStdio(),
		client.WithRequestTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Ensure cleanup
	defer func() {
		if err := c.Close(); err != nil {
			t.Logf("Error closing client: %v", err)
		}
	}()

	// Try to list tools to verify connection
	tools, err := c.ListTools()
	if err != nil {
		t.Fatalf("Failed to list tools (connection test): %v", err)
	}
	t.Logf("Connected successfully, found %d tools", len(tools))

	// Test AddRoot with the same path causing issues
	testPath := "/Users/almatuck/workspaces/rnd/mcp-client"
	testName := "projectRoot"

	t.Logf("Testing AddRoot with path: %s, name: %s", testPath, testName)

	err = c.AddRoot(testPath, testName)
	if err != nil {
		t.Logf("AddRoot failed with error: %v", err)

		// If it's the internal error, let's try some alternatives
		if fmt.Sprintf("%v", err) == "failed to add root: JSON-RPC error -32603: Internal error" {
			t.Logf("Got the expected internal error. Trying alternatives...")

			// Try with file:// prefix
			fileURI := "file://" + testPath
			t.Logf("Trying with file:// prefix: %s", fileURI)
			err2 := c.AddRoot(fileURI, testName)
			if err2 != nil {
				t.Logf("file:// prefix also failed: %v", err2)
			} else {
				t.Logf("file:// prefix succeeded!")
			}

			// Try with relative path
			t.Logf("Trying with relative path: .")
			err3 := c.AddRoot(".", "currentDir")
			if err3 != nil {
				t.Logf("Relative path also failed: %v", err3)
			} else {
				t.Logf("Relative path succeeded!")
			}

			// Try with a simple test path
			testSimplePath := "/tmp"
			t.Logf("Trying with simple path: %s", testSimplePath)
			err4 := c.AddRoot(testSimplePath, "tmpDir")
			if err4 != nil {
				t.Logf("Simple path also failed: %v", err4)
			} else {
				t.Logf("Simple path succeeded!")
			}
		}

		// Don't fail the test - we want to see what happens
		return
	}

	t.Logf("AddRoot succeeded!")

	// If AddRoot succeeded, try to list roots to verify
	roots, err := c.GetRoots()
	if err != nil {
		t.Logf("GetRoots failed after successful AddRoot: %v", err)
	} else {
		t.Logf("Found %d roots after AddRoot:", len(roots))
		for i, root := range roots {
			t.Logf("  [%d] URI: %s, Name: %s", i, root.URI, root.Name)
		}
	}
}

// TestAddRoot_Debug provides detailed debugging for AddRoot issues
func TestAddRoot_Debug(t *testing.T) {
	if os.Getenv("DEBUG_ADDROOT") == "" {
		t.Skip("Skipping debug test. Set DEBUG_ADDROOT=1 to run.")
	}

	// This test helps debug by showing exactly what's being sent
	c, mockTransport := SetupClientWithMockTransport(t, "2024-11-05")

	// We'll inspect requests after they're sent using GetRequestsByMethod

	// Queue an internal error response
	errorResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"error": map[string]interface{}{
			"code":    -32603,
			"message": "Internal error",
			"data":    "Server-side error processing roots/add request",
		},
	}
	errorJSON, _ := json.Marshal(errorResponse)
	mockTransport.QueueResponse(errorJSON, nil)

	// Test the exact same call that's failing
	testPath := "/Users/almatuck/workspaces/rnd/mcp-client"
	testName := "projectRoot"

	t.Logf("Calling AddRoot with path=%s, name=%s", testPath, testName)
	err := c.AddRoot(testPath, testName)

	if err != nil {
		t.Logf("AddRoot failed as expected: %v", err)
	} else {
		t.Logf("AddRoot unexpectedly succeeded")
	}

	// Show all requests that were sent
	allRequests := mockTransport.GetRequestHistory()
	t.Logf("Total requests sent: %d", len(allRequests))
	for i, req := range allRequests {
		t.Logf("Request %d: %s", i+1, string(req.Message))
	}

	// Show specific AddRoot requests
	addRootRequests := mockTransport.GetRequestsByMethod("roots/add")
	for i, req := range addRootRequests {
		t.Logf("AddRoot request %d: %s", i+1, string(req.Message))

		var request map[string]interface{}
		if err := json.Unmarshal(req.Message, &request); err == nil {
			pretty, _ := json.MarshalIndent(request, "", "  ")
			t.Logf("Parsed AddRoot request %d:\n%s", i+1, string(pretty))
		}
	}
}
