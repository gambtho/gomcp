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

// ServerCapabilities represents the capabilities declared by the MCP server during initialization.
type ServerCapabilities struct {
	Logging      *LoggingCapability     `json:"logging,omitempty"`
	Prompts      *PromptsCapability     `json:"prompts,omitempty"`
	Resources    *ResourcesCapability   `json:"resources,omitempty"`
	Tools        *ToolsCapability       `json:"tools,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// LoggingCapability represents the server's logging capability.
// Currently defined as an empty object in all MCP specification versions.
type LoggingCapability struct {
	// No fields defined in specification - empty object
}

// PromptsCapability represents the server's prompt template capability.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents the server's resource capability.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolsCapability represents the server's tool capability.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo represents information about the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool is an alias to the shared mcp.Tool type for backward compatibility.
type Tool = mcp.Tool

// Resource represents a server resource that can be accessed via the MCP protocol.
type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Size        *int64                 `json:"size,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// Prompt represents a server prompt template that can be used to generate messages.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents a parameter for a prompt template.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

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
