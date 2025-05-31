package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/localrivet/gomcp/transport/stdio"
)

func main() {
	fmt.Println("Testing stdio transport for null byte issues...")

	// Create a test message
	testMessage := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "claude-ai",
				"version": "0.1.0",
			},
		},
		"id": 0,
	}

	// Marshal to JSON
	messageBytes, err := json.Marshal(testMessage)
	if err != nil {
		fmt.Printf("Failed to marshal test message: %v\n", err)
		return
	}

	fmt.Printf("Original message: %s\n", string(messageBytes))
	fmt.Printf("Original length: %d\n", len(messageBytes))

	// Create an in-memory buffer to capture output
	var output bytes.Buffer
	input := strings.NewReader(string(messageBytes) + "\n")

	// Create stdio transport with our test I/O
	transport := stdio.NewTransportWithIO(input, &output)

	// Set up message handler that echoes back
	transport.SetMessageHandler(func(message []byte) ([]byte, error) {
		fmt.Printf("Handler received message: %q\n", string(message))
		fmt.Printf("Handler received length: %d\n", len(message))

		// Check for null bytes in received message
		for i, b := range message {
			if b == 0 {
				fmt.Printf("NULL BYTE FOUND at position %d in received message!\n", i)
			}
		}

		// Return the same message
		return message, nil
	})

	// Initialize and start transport
	if err := transport.Initialize(); err != nil {
		fmt.Printf("Transport initialize failed: %v\n", err)
		return
	}

	if err := transport.Start(); err != nil {
		fmt.Printf("Transport start failed: %v\n", err)
		return
	}

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Stop transport
	transport.Stop()

	// Check the output for null bytes
	outputBytes := output.Bytes()
	fmt.Printf("Output: %q\n", string(outputBytes))
	fmt.Printf("Output length: %d\n", len(outputBytes))

	// Check for null bytes in output
	for i, b := range outputBytes {
		if b == 0 {
			fmt.Printf("NULL BYTE FOUND at position %d in output!\n", i)
		}
	}

	fmt.Println("Test completed.")
}
