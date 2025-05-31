// Package client provides the client-side implementation of the MCP protocol.
package client

import "github.com/localrivet/gomcp/mcp"

// Root represents a filesystem root exposed to the MCP server.
type Root struct {
	URI      string                 `json:"uri"`
	Name     string                 `json:"name,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ClientCapabilities represents the capabilities supported by this client.
type ClientCapabilities struct {
	Roots        RootsCapability        `json:"roots,omitempty"`
	Sampling     map[string]interface{} `json:"sampling,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// RootsCapability represents the client's roots capability.
type RootsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// Tool is an alias to the shared mcp.Tool type for backward compatibility.
type Tool = mcp.Tool

// BatchRequest represents a single request within a batch operation.
type BatchRequest struct {
	// Method is the JSON-RPC method to call (e.g., "tools/call", "resources/read")
	Method string `json:"method"`

	// Params contains the parameters for the method call
	Params map[string]interface{} `json:"params,omitempty"`

	// ID is the request identifier. If nil, this is treated as a notification.
	// Notifications do not generate responses.
	ID interface{} `json:"id,omitempty"`
}

// BatchResponse represents a single response within a batch operation.
type BatchResponse struct {
	// ID is the request identifier that this response corresponds to
	ID interface{} `json:"id"`

	// Result contains the successful result of the method call
	Result interface{} `json:"result,omitempty"`

	// Error contains error information if the method call failed
	Error *BatchError `json:"error,omitempty"`
}

// BatchError represents an error within a batch response.
type BatchError struct {
	// Code is the JSON-RPC error code
	Code int `json:"code"`

	// Message is a human-readable error message
	Message string `json:"message"`

	// Data contains additional error information
	Data interface{} `json:"data,omitempty"`
}

// BatchRequestBuilder provides a fluent interface for constructing batch requests.
type BatchRequestBuilder struct {
	client   *clientImpl
	requests []BatchRequest
	nextID   int64
}

// AddRequest adds a request to the batch.
// For requests that expect a response, provide an ID. For notifications, set ID to nil.
func (b *BatchRequestBuilder) AddRequest(method string, params map[string]interface{}, id interface{}) *BatchRequestBuilder {
	b.requests = append(b.requests, BatchRequest{
		Method: method,
		Params: params,
		ID:     id,
	})
	return b
}

// Execute sends the batch request and returns the responses.
func (b *BatchRequestBuilder) Execute(opts ...RequestOption) ([]BatchResponse, error) {
	return b.client.SendBatch(b.requests, opts...)
}
