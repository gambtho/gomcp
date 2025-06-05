package main

import (
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
)

// TestToolCallSequence tests the exact sequence that's failing in the demo
func TestToolCallSequence(t *testing.T) {
	registry := client.NewServerRegistry()
	defer registry.Close()

	// Start math server
	mathDef := client.ServerDefinition{
		Command: "go",
		Args:    []string{"run", ".", "math-server"},
	}

	if err := registry.StartServer("math-server", mathDef); err != nil {
		t.Fatalf("Failed to start math server: %v", err)
	}

	// Get client
	mathClient, err := registry.GetClient("math-server")
	if err != nil {
		t.Fatalf("Failed to get math client: %v", err)
	}

	// Wait for readiness
	if err := mathClient.WaitForReady(5 * time.Second); err != nil {
		t.Fatalf("Math server not ready: %v", err)
	}

	// Step 1: List tools (this works fine)
	tools, err := mathClient.ListTools()
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}
	t.Logf("Found %d tools", len(tools))
	for _, tool := range tools {
		t.Logf("  Tool: %s - %s", tool.Name, tool.Description)
	}

	// Step 2: Call first tool (this is where the issue happens)
	t.Log("\n=== Calling first tool (add) ===")
	result1, err := mathClient.CallTool("add", map[string]interface{}{
		"a": 15,
		"b": 25,
	})
	if err != nil {
		t.Fatalf("Failed to call add tool: %v", err)
	}
	t.Logf("Add result: %+v", result1)

	// Check if result1 is actually the tools list (the bug)
	if resultMap, ok := result1.(map[string]interface{}); ok {
		if toolsArray, hasTools := resultMap["tools"]; hasTools {
			t.Errorf("BUG DETECTED: First tool call returned tools list instead of tool result: %+v", toolsArray)
		}
	}

	// Step 3: Call second tool (this usually works)
	t.Log("\n=== Calling second tool (multiply) ===")
	result2, err := mathClient.CallTool("multiply", map[string]interface{}{
		"a": 7,
		"b": 8,
	})
	if err != nil {
		t.Fatalf("Failed to call multiply tool: %v", err)
	}
	t.Logf("Multiply result: %+v", result2)

	// Step 4: Call third tool (this usually works)
	t.Log("\n=== Calling third tool (factorial) ===")
	result3, err := mathClient.CallTool("factorial", map[string]interface{}{
		"n": 5,
	})
	if err != nil {
		t.Fatalf("Failed to call factorial tool: %v", err)
	}
	t.Logf("Factorial result: %+v", result3)
}

// TestSequentialCalls tests multiple sequential calls to isolate timing issues
func TestSequentialCalls(t *testing.T) {
	registry := client.NewServerRegistry()
	defer registry.Close()

	// Start math server
	mathDef := client.ServerDefinition{
		Command: "go",
		Args:    []string{"run", ".", "math-server"},
	}

	if err := registry.StartServer("math-server", mathDef); err != nil {
		t.Fatalf("Failed to start math server: %v", err)
	}

	mathClient, err := registry.GetClient("math-server")
	if err != nil {
		t.Fatalf("Failed to get math client: %v", err)
	}

	if err := mathClient.WaitForReady(5 * time.Second); err != nil {
		t.Fatalf("Math server not ready: %v", err)
	}

	// Call tools multiple times in sequence
	for i := 0; i < 5; i++ {
		t.Logf("\n=== Call %d ===", i+1)

		// Add small delay between calls
		time.Sleep(100 * time.Millisecond)

		result, err := mathClient.CallTool("add", map[string]interface{}{
			"a": i,
			"b": i + 1,
		})
		if err != nil {
			t.Errorf("Call %d failed: %v", i+1, err)
			continue
		}

		t.Logf("Call %d result: %+v", i+1, result)

		// Check if we got tools list instead of result
		if resultMap, ok := result.(map[string]interface{}); ok {
			if _, hasTools := resultMap["tools"]; hasTools {
				t.Errorf("Call %d: Got tools list instead of tool result!", i+1)
			}
		}
	}
}
