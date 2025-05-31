package events

import "time"

// Standard topic constants for GoMCP events.
// These define the public API contract for what topics external consumers can subscribe to.
const (
	// Server lifecycle events
	TopicServerInitialized = "server.initialized"
	TopicServerShutdown    = "server.shutdown"

	// Connection events (can be emitted by both client and server)
	TopicClientConnected    = "client.connected"    // Client connected to server
	TopicClientDisconnected = "client.disconnected" // Client disconnected from server

	// Registration events (server-side)
	TopicToolRegistered     = "tool.registered"
	TopicResourceRegistered = "resource.registered"

	// Operation events (can be emitted by both client and server for same operations)
	TopicToolExecuted     = "tool.executed"     // Tool was executed
	TopicResourceAccessed = "resource.accessed" // Resource was accessed
	TopicPromptExecuted   = "prompt.executed"   // Prompt was executed

	// Error events
	TopicRequestFailed = "request.failed" // Request failed

	// Client-specific lifecycle events
	TopicClientInitializing = "client.initializing" // Client starting up
	TopicClientInitialized  = "client.initialized"  // Client ready
	TopicClientError        = "client.error"        // Client operation failed
)

// Shared struct types for event data

// ClientInfo represents information about a connected client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Client lifecycle event structs

// ClientInitializingEvent is emitted when a client starts connecting to a server
type ClientInitializingEvent struct {
	URL string `json:"url"` // The server URL being connected to
}

// ClientInitializedEvent is emitted when a client successfully connects and initializes
type ClientInitializedEvent struct {
	URL string `json:"url"` // The server URL that was connected to
}

// ClientErrorEvent is emitted when a client operation fails
type ClientErrorEvent struct {
	Error string `json:"error"` // The error message describing what failed
}

// ClientDisconnectedEvent is emitted when a client disconnects from a server
type ClientDisconnectedEvent struct {
	// Client-side fields
	URL string `json:"url,omitempty"` // The server URL that was disconnected from

	// Server-side fields
	SessionID       string `json:"sessionId,omitempty"`       // Unique session identifier
	ProtocolVersion string `json:"protocolVersion,omitempty"` // MCP protocol version used
	ConnectedAt     string `json:"connectedAt,omitempty"`     // When the session was created (RFC3339)
	DisconnectedAt  string `json:"disconnectedAt,omitempty"`  // When the session was closed (RFC3339)
}

// Server lifecycle event structs

// ServerInitializedEvent is emitted when the server has been initialized and is ready to accept requests
type ServerInitializedEvent struct {
	ServerName        string                 `json:"serverName"`
	ProtocolVersion   string                 `json:"protocolVersion"`
	Capabilities      map[string]interface{} `json:"capabilities,omitempty"`
	InitializedAt     time.Time              `json:"initializedAt"`
	TransportType     string                 `json:"transportType,omitempty"`
	TransportEndpoint string                 `json:"transportEndpoint,omitempty"`
	// Additional server metrics
	ToolCount     int            `json:"toolCount"`
	ResourceCount int            `json:"resourceCount"`
	PromptCount   int            `json:"promptCount"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// ServerShutdownEvent is emitted when the server is shutting down
type ServerShutdownEvent struct {
	ServerName   string    `json:"serverName"`
	ShutdownAt   time.Time `json:"shutdownAt"`
	GracefulExit bool      `json:"gracefulExit"`
	Reason       string    `json:"reason,omitempty"`
}

// ClientConnectedEvent is emitted when a client connects to the server (server-side perspective)
type ClientConnectedEvent struct {
	SessionID       string                 `json:"sessionId"`
	ProtocolVersion string                 `json:"protocolVersion"`
	ConnectedAt     time.Time              `json:"connectedAt"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
	Capabilities    map[string]interface{} `json:"capabilities"`
}

// Registration event structs

// ToolRegisteredEvent is emitted when a tool is registered with the server
type ToolRegisteredEvent struct {
	ToolName     string                 `json:"toolName"`
	Description  string                 `json:"description"`
	RegisteredAt time.Time              `json:"registeredAt"`
	Schema       map[string]interface{} `json:"schema"`
	Annotations  map[string]interface{} `json:"annotations,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceRegisteredEvent is emitted when a resource is registered with the server
type ResourceRegisteredEvent struct {
	URI          string    `json:"uri"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	MimeType     string    `json:"mimeType"`
	RegisteredAt time.Time `json:"registeredAt"`
}

// Operation event structs

// RequestFailedEvent is emitted when an MCP request fails on either client or server
type RequestFailedEvent struct {
	Method      string `json:"method"`      // The MCP method that failed (e.g., "tools/call")
	RequestJSON string `json:"requestJSON"` // The actual JSON request that was sent
	Error       string `json:"error"`       // The error message describing the failure
}

// ToolExecutedEvent is emitted when an MCP request succeeds on either client or server
type ToolExecutedEvent struct {
	Method       string `json:"method"`       // The MCP method that was executed (e.g., "tools/call")
	RequestJSON  string `json:"requestJSON"`  // The actual JSON request that was sent
	ResponseJSON string `json:"responseJSON"` // The actual JSON response that was received
}

// ResourceAccessedEvent is emitted when a resource is accessed
type ResourceAccessedEvent struct {
	URI          string    `json:"uri"`
	Method       string    `json:"method"` // "resources/read", "resources/list", etc.
	AccessedAt   time.Time `json:"accessedAt"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	ResponseSize int       `json:"responseSize,omitempty"`
}

// PromptExecutedEvent is emitted when a prompt is executed
type PromptExecutedEvent struct {
	PromptName   string                 `json:"promptName"`
	Arguments    map[string]interface{} `json:"arguments,omitempty"`
	ExecutedAt   time.Time              `json:"executedAt"`
	Success      bool                   `json:"success"`
	ErrorMessage string                 `json:"errorMessage,omitempty"`
	MessageCount int                    `json:"messageCount,omitempty"`
}
