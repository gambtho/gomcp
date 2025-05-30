// Package mcp provides shared types for the MCP protocol implementation.
package mcp

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
