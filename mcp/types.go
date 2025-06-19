// Package mcp provides shared types for the MCP protocol implementation.
package mcp

import (
	"encoding/json"
)

// Tool represents a tool available from an MCP server.
// This type is used by both client and server implementations for consistency.
type Tool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	InputSchema  map[string]interface{} `json:"inputSchema"`
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"`
	Annotations  map[string]interface{} `json:"annotations,omitempty"`
}

// Resource represents a resource available from an MCP server.
// This type is used by both client and server implementations for consistency.
type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// Prompt represents a prompt template available from an MCP server.
// This type is used by both client and server implementations for consistency.
type Prompt struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Arguments   []PromptArgument       `json:"arguments,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// PromptArgument represents an argument for a prompt template.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response message
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request message
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC 2.0 notification message (no ID)
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// NewSuccessResponse creates a successful JSON-RPC response
func NewSuccessResponse(id interface{}, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates an error JSON-RPC response
func NewErrorResponse(id interface{}, code int, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// NewRequest creates a JSON-RPC request
func NewRequest(id interface{}, method string, params interface{}) *JSONRPCRequest {
	return &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewNotification creates a JSON-RPC notification
func NewNotification(method string, params interface{}) *JSONRPCNotification {
	return &JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
}

// Marshal serializes the response to JSON bytes
func (r *JSONRPCResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Marshal serializes the request to JSON bytes
func (r *JSONRPCRequest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Marshal serializes the notification to JSON bytes
func (n *JSONRPCNotification) Marshal() ([]byte, error) {
	return json.Marshal(n)
}

// IsNotification returns true if this is a notification (no ID field)
func (r *JSONRPCRequest) IsNotification() bool {
	return r.ID == nil
}

// Error implements the error interface for JSONRPCError
func (e *JSONRPCError) Error() string {
	return e.Message
}
