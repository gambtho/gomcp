// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// sendRequest sends a JSON-RPC request to the server and parses the response.
func (c *clientImpl) sendRequest(method string, params interface{}) (interface{}, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Create the request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.generateRequestID(),
		"method":  method,
	}

	if params != nil {
		request["params"] = params
	}

	// Convert the request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create a context with the request timeout
	ctx, cancel := context.WithTimeout(c.ctx, c.requestTimeout)
	defer cancel()

	// Send the request
	responseJSON, err := c.transport.SendWithContext(ctx, requestJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Parse the response
	var response struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      int64       `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   *struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for error response
	if response.Error != nil {
		return nil, fmt.Errorf("server returned error: %s (code %d)", response.Error.Message, response.Error.Code)
	}

	return response.Result, nil
}

// CallTool calls a tool on the server.
func (c *clientImpl) CallTool(name string, args map[string]interface{}) (interface{}, error) {
	params := map[string]interface{}{
		"name": name,
	}

	if args != nil {
		params["arguments"] = args
	}

	return c.sendRequest("tools/call", params)
}

// GetResource retrieves a resource from the server.
func (c *clientImpl) GetResource(path string) (interface{}, error) {
	params := map[string]interface{}{
		"path": path,
	}

	return c.sendRequest("resource/get", params)
}

// GetPrompt retrieves a prompt from the server.
func (c *clientImpl) GetPrompt(name string, variables map[string]interface{}) (interface{}, error) {
	params := map[string]interface{}{
		"name": name,
	}

	if variables != nil {
		params["variables"] = variables
	}

	return c.sendRequest("prompt/get", params)
}

// GetRoot retrieves the root resource from the server.
func (c *clientImpl) GetRoot() (interface{}, error) {
	return c.GetResource("/")
}

// ListTools retrieves the list of available tools from the server.
// It automatically handles pagination internally and returns all available tools.
func (c *clientImpl) ListTools() ([]Tool, error) {
	var allTools []Tool
	cursor := ""

	for {
		// Prepare parameters for the request
		var params map[string]interface{}
		if cursor != "" {
			params = map[string]interface{}{
				"cursor": cursor,
			}
		}

		// Send the tools/list request
		result, err := c.sendRequest("tools/list", params)
		if err != nil {
			return nil, fmt.Errorf("failed to list tools: %w", err)
		}

		// Parse the response
		response, ok := result.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid response format from tools/list")
		}

		// Extract tools from the response
		toolsData, ok := response["tools"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid tools format in response")
		}

		// Convert each tool to our Tool struct
		for _, toolData := range toolsData {
			toolMap, ok := toolData.(map[string]interface{})
			if !ok {
				continue
			}

			tool := Tool{
				Name:         getString(toolMap, "name"),
				Description:  getString(toolMap, "description"),
				InputSchema:  getMap(toolMap, "inputSchema"),
				OutputSchema: getMap(toolMap, "outputSchema"),
				Annotations:  getMap(toolMap, "annotations"),
			}

			allTools = append(allTools, tool)
		}

		// Check if there are more pages
		nextCursor, hasMore := response["nextCursor"].(string)
		if !hasMore || nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allTools, nil
}

// Helper functions for safe type conversion
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}
