// Package client provides the client-side implementation of the MCP protocol.
//
// This package contains the Client interface and implementation for communicating with MCP services.
// It enables Go applications to interact with MCP servers through a clean, type-safe API that handles
// all aspects of the protocol, including version negotiation, connection management, and request/response
// handling.
//
// # Basic Usage
//
//	// Create a new client and connect to an MCP server
//	client, err := client.NewClient("my-client",
//		client.WithProtocolVersion("2025-03-26"),
//		client.WithLogger(logger),
//	)
//	if err != nil {
//		log.Fatalf("Failed to connect: %v", err)
//	}
//	defer client.Close()
//
//	// Call a tool
//	result, err := client.CallTool("calculate", map[string]interface{}{
//		"operation": "add",
//		"values": []float64{1.5, 2.5, 3.0},
//	})
//
// # Client Options
//
// The NewClient function accepts various options to customize client behavior:
//
//   - WithProtocolVersion: Set a specific protocol version
//   - WithProtocolNegotiation: Enable/disable automatic protocol negotiation
//   - WithLogger: Configure a custom logger
//   - WithTransport: Specify a custom transport implementation
//   - WithRequestTimeout: Set request timeout duration
//   - WithConnectionTimeout: Set connection timeout duration
//   - WithSamplingOptimizations: Configure sampling performance optimizations
//
// # Thread Safety
//
// All Client methods are thread-safe and can be called concurrently from multiple goroutines.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/mcp"
)

// RequestOption represents an option that can be passed to client methods.
type RequestOption interface {
	apply()
}

// TimeoutOption creates a request option that sets a timeout for the request.
type TimeoutOption struct {
	Duration time.Duration
}

func (t TimeoutOption) apply() {}

// WithRequestTimeoutOption creates a TimeoutOption from a duration.
func WithRequestTimeoutOption(d time.Duration) TimeoutOption {
	return TimeoutOption{Duration: d}
}

// Client represents an MCP client for communicating with MCP servers.
// It provides methods for all MCP operations including tool calls, resource access,
// prompt rendering, root management, and sampling functionality.
type Client interface {
	// CallTool invokes a tool on the connected MCP server.
	//
	// The name parameter specifies the tool to call. The args parameter contains
	// the arguments to pass to the tool as key-value pairs. The returned interface{}
	// contains the tool's output, which can be any JSON-serializable value.
	//
	// Example:
	//  result, err := client.CallTool("translate", map[string]interface{}{
	//      "text": "Hello world",
	//      "target_language": "Spanish",
	//  })
	//
	// For timeout support:
	//  result, err := client.CallTool("translate", map[string]interface{}{
	//      "text": "Hello world",
	//      "target_language": "Spanish",
	//  }, client.WithRequestTimeoutOption(10*time.Second))
	CallTool(name string, args map[string]interface{}, opts ...RequestOption) (interface{}, error)

	// GetResource retrieves a resource from the server.
	//
	// The path parameter specifies the resource URI to retrieve.
	//
	// Example:
	//  resource, err := client.GetResource("/files/readme.txt")
	//
	// For timeout support:
	//  resource, err := client.GetResource("/files/readme.txt",
	//      client.WithRequestTimeoutOption(5*time.Second))
	GetResource(path string, opts ...RequestOption) (interface{}, error)

	// GetPrompt retrieves a prompt from the server.
	//
	// The name parameter specifies the prompt to retrieve. The variables parameter
	// contains any variables to substitute in the prompt template.
	//
	// Example:
	//  prompt, err := client.GetPrompt("greeting", map[string]interface{}{
	//      "name": "Alice",
	//  })
	//
	// For timeout support:
	//  prompt, err := client.GetPrompt("greeting", map[string]interface{}{
	//      "name": "Alice",
	//  }, client.WithRequestTimeoutOption(3*time.Second))
	GetPrompt(name string, variables map[string]interface{}, opts ...RequestOption) (interface{}, error)

	// GetRoot retrieves the root resource from the server.
	//
	// This is a convenience method equivalent to calling GetResource("/").
	//
	// Example:
	//  root, err := client.GetRoot()
	GetRoot() (interface{}, error)

	// Close closes the client connection to the server and releases all resources.
	//
	// After calling Close, the client cannot be used for further operations.
	// It is good practice to defer this call after creating a client.
	//
	// Example:
	//  client, err := client.NewClient("my-client")
	//  if err != nil {
	//      log.Fatal(err)
	//  }
	//  defer client.Close()
	Close() error

	// AddRoot registers a new root endpoint with the server.
	//
	// The uri parameter specifies the path of the root. The name parameter
	// provides a human-readable name for the root.
	//
	// Example:
	//  err := client.AddRoot("/api/v2", "API Version 2")
	AddRoot(uri string, name string) error

	// RemoveRoot unregisters a root endpoint from the server.
	//
	// The uri parameter specifies the path of the root to remove.
	//
	// Example:
	//  err := client.RemoveRoot("/api/v1")
	RemoveRoot(uri string) error

	// GetRoots retrieves the list of root endpoints from the server.
	//
	// The returned slice contains all registered roots with their URIs and names.
	//
	// Example:
	//  roots, err := client.GetRoots()
	//  for _, root := range roots {
	//      fmt.Printf("Root: %s (%s)\n", root.URI, root.Name)
	//  }
	GetRoots() ([]Root, error)

	// ListTools retrieves the list of available tools from the server.
	//
	// This method calls the tools/list endpoint as specified in the MCP protocol.
	// It automatically handles pagination internally and returns all available tools.
	// The returned slice contains all available tools with their names, descriptions,
	// and input schemas, which can be used for tool discovery and proxy patterns.
	//
	// Example:
	//  tools, err := client.ListTools()
	//  for _, tool := range tools {
	//      fmt.Printf("Tool: %s - %s\n", tool.Name, tool.Description)
	//      fmt.Printf("Schema: %+v\n", tool.InputSchema)
	//  }
	ListTools() ([]Tool, error)

	// Version returns the negotiated protocol version with the server.
	//
	// This returns one of the standardized version strings: "draft", "2024-11-05",
	// or "2025-03-26".
	//
	// Example:
	//  version := client.Version()
	//  fmt.Printf("Connected using MCP protocol version %s\n", version)
	Version() string

	// IsInitialized returns whether the client has been initialized.
	//
	// Initialization occurs during the first operation that requires
	// server communication.
	IsInitialized() bool

	// IsConnected returns whether the client is currently connected to the server.
	//
	// Example:
	//  if client.IsConnected() {
	//      fmt.Println("Client is connected to the server")
	//  } else {
	//      fmt.Println("Client is not connected")
	//  }
	IsConnected() bool

	// WithSamplingHandler registers a handler for sampling requests.
	//
	// The handler will be called when the server requests sampling (e.g., for LLM interactions).
	// Returns the client instance for method chaining.
	//
	// Example:
	//  client = client.WithSamplingHandler(func(params SamplingCreateMessageParams) (SamplingResponse, error) {
	//      // Process sampling request
	//      return SamplingResponse{...}, nil
	//  })
	WithSamplingHandler(handler SamplingHandler) Client

	// GetSamplingHandler returns the currently registered sampling handler.
	GetSamplingHandler() SamplingHandler

	// RequestSampling initiates a sampling request to the server.
	//
	// This is the unified method for all sampling operations, supporting both
	// regular and streaming modes through options configuration.
	//
	// Example:
	//  opts := client.NewSamplingOptions(messages, prefs).
	//      WithSystemPrompt("You are a helpful assistant").
	//      WithMaxTokens(1000)
	//  response, err := client.RequestSampling(opts)
	//
	// For streaming (protocol version 2025-03-26 only):
	//  opts := client.NewSamplingOptions(messages, prefs).
	//      WithStreaming(func(chunk *SamplingResponse) error {
	//          fmt.Printf("Received chunk: %s\n", chunk.Content.Text)
	//          return nil
	//      })
	//  response, err := client.RequestSampling(opts)
	RequestSampling(opts *SamplingOptions) (*SamplingResponse, error)

	// SendBatch sends multiple requests to the server in a single batch operation.
	//
	// This method implements JSON-RPC 2.0 batch requests, allowing multiple operations
	// to be sent together for improved efficiency. The requests parameter contains
	// the individual requests to include in the batch.
	//
	// The method returns a slice of BatchResponse objects, where each response
	// corresponds to a request in the batch (excluding notifications). The order
	// of responses matches the order of requests.
	//
	// Example:
	//  requests := []BatchRequest{
	//      {Method: "tools/call", Params: map[string]interface{}{"name": "calculator", "arguments": map[string]interface{}{"op": "add", "a": 1, "b": 2}}},
	//      {Method: "resources/read", Params: map[string]interface{}{"uri": "/config.json"}},
	//      {Method: "prompts/get", Params: map[string]interface{}{"name": "greeting", "arguments": map[string]interface{}{"name": "Alice"}}},
	//  }
	//  responses, err := client.SendBatch(requests)
	//  for i, response := range responses {
	//      if response.Error != nil {
	//          fmt.Printf("Request %d failed: %v\n", i, response.Error)
	//      } else {
	//          fmt.Printf("Request %d result: %v\n", i, response.Result)
	//      }
	//  }
	//
	// For timeout support:
	//  responses, err := client.SendBatch(requests, client.WithRequestTimeoutOption(30*time.Second))
	SendBatch(requests []BatchRequest, opts ...RequestOption) ([]BatchResponse, error)

	// BatchBuilder creates a new batch builder for constructing batch requests.
	//
	// The batch builder provides a fluent interface for adding multiple requests
	// to a batch operation. Use AddRequest to add individual requests.
	//
	// Example:
	//  responses, err := client.BatchBuilder().
	//      AddRequest("tools/call", map[string]interface{}{
	//          "name": "calculator",
	//          "arguments": map[string]interface{}{"op": "add", "a": 1, "b": 2}
	//      }, 1).
	//      AddRequest("resources/read", map[string]interface{}{"uri": "/config.json"}, 2).
	//      AddRequest("prompts/get", map[string]interface{}{
	//          "name": "greeting",
	//          "arguments": map[string]interface{}{"name": "Alice"}
	//      }, 3).
	//      Execute()
	BatchBuilder() *BatchRequestBuilder
}

// clientImpl is the concrete implementation of the Client interface.
type clientImpl struct {
	url               string
	transport         Transport
	logger            *slog.Logger
	versionDetector   *mcp.VersionDetector
	negotiatedVersion string
	requestTimeout    time.Duration
	connectionTimeout time.Duration
	requestIDCounter  atomic.Int64
	initialized       bool
	connected         bool
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	roots             []Root
	rootsMu           sync.RWMutex
	capabilities      ClientCapabilities
	samplingHandler   SamplingHandler

	// Server management
	serverRegistry *ServerRegistry
	serverName     string

	// Events
	events *events.Subject
}

// NewClient creates a new MCP client with the given URL and options.
// The client will automatically detect and adapt to the server's MCP specification version.
// It immediately establishes a connection to the server and returns an error if the connection fails.
//
// The url parameter is interpreted based on its format:
//   - "stdio:///": Uses Standard I/O for communication (useful for child processes)
//   - "ws://host:port/path": Uses WebSocket protocol
//   - "http://host:port/path": Uses HTTP protocol
//   - "sse://host:port/path": Uses Server-Sent Events protocol
//   - Custom schemes can be handled with a custom Transport implementation
//
// Errors returned by NewClient may include:
//   - Connection failures (e.g., server unreachable)
//   - Protocol negotiation failures
//   - Transport initialization errors
//
// Example:
//
//	// Basic client with default options
//	client, err := client.NewClient("ws://localhost:8080/mcp")
//	if err != nil {
//		log.Fatalf("Failed to create client: %v", err)
//	}
//
//	// Client with custom options
//	client, err := client.NewClient("http://api.example.com/mcp",
//		client.WithProtocolVersion("2025-03-26"),
//		client.WithLogger(myCustomLogger),
//		client.WithRequestTimeout(time.Second * 20),
//	)
func NewClient(url string, options ...Option) (Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	c := &clientImpl{
		url:               url,
		logger:            slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		versionDetector:   mcp.NewVersionDetector(),
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		ctx:               ctx,
		cancel:            cancel,
		roots:             []Root{},
		capabilities: ClientCapabilities{
			Roots: RootsCapability{
				ListChanged: true,
			},
		},
		events: events.NewSubject(),
	}

	// Apply options
	for _, option := range options {
		option(c)
	}

	// Emit client initializing event
	go func() {
		events.Publish[events.ClientInitializingEvent](c.events, events.TopicClientInitializing, events.ClientInitializingEvent{
			URL: url,
		})
	}()

	// If no transport is provided, one will be selected based on the URL
	// when Connect() is called

	// Immediately connect to the server
	if err := c.Connect(); err != nil {
		cancel() // Clean up resources
		go func() {
			events.Publish[events.ClientErrorEvent](c.events, events.TopicClientError, events.ClientErrorEvent{
				Error: err.Error(),
			})
		}()
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// Emit client initialized event
	go func() {
		events.Publish[events.ClientInitializedEvent](c.events, events.TopicClientInitialized, events.ClientInitializedEvent{
			URL: url,
		})
	}()

	return c, nil
}

// generateRequestID generates a unique request ID.
func (c *clientImpl) generateRequestID() int64 {
	return c.requestIDCounter.Add(1)
}

// Version returns the negotiated protocol version.
func (c *clientImpl) Version() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.negotiatedVersion
}

// IsInitialized returns whether the client has been initialized.
func (c *clientImpl) IsInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

// IsConnected returns whether the client is connected.
func (c *clientImpl) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// CallTool calls a tool on the server.
func (c *clientImpl) CallTool(name string, args map[string]interface{}, opts ...RequestOption) (interface{}, error) {
	timeout := c.extractTimeout(opts...)
	return c.sendRequestWithTimeout("tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	}, timeout)
}

// GetResource retrieves a resource from the server.
func (c *clientImpl) GetResource(uri string, opts ...RequestOption) (interface{}, error) {
	timeout := c.extractTimeout(opts...)
	return c.sendRequestWithTimeout("resources/read", map[string]interface{}{
		"uri": uri,
	}, timeout)
}

// GetPrompt retrieves a prompt from the server.
func (c *clientImpl) GetPrompt(name string, variables map[string]interface{}, opts ...RequestOption) (interface{}, error) {
	timeout := c.extractTimeout(opts...)

	params := map[string]interface{}{
		"name": name,
	}

	if variables != nil {
		params["arguments"] = variables
	}

	return c.sendRequestWithTimeout("prompts/get", params, timeout)
}

// extractTimeout extracts a timeout duration from request options.
func (c *clientImpl) extractTimeout(opts ...RequestOption) time.Duration {
	if len(opts) > 0 {
		for _, opt := range opts {
			if timeout, ok := opt.(TimeoutOption); ok {
				return timeout.Duration
			}
		}
	}
	return c.requestTimeout
}

// GetRoot retrieves the root resource from the server.
func (c *clientImpl) GetRoot() (interface{}, error) {
	return c.GetResource("/")
}

// ListTools retrieves the list of available tools from the server.
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

// SendBatch sends multiple requests to the server in a single batch operation.
func (c *clientImpl) SendBatch(requests []BatchRequest, opts ...RequestOption) ([]BatchResponse, error) {
	if len(requests) == 0 {
		return []BatchResponse{}, nil
	}

	timeout := c.extractTimeout(opts...)

	// Convert BatchRequest to JSON-RPC format
	var jsonRPCRequests []map[string]interface{}
	for _, req := range requests {
		jsonReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  req.Method,
		}

		if req.Params != nil {
			jsonReq["params"] = req.Params
		}

		if req.ID != nil {
			jsonReq["id"] = req.ID
		}

		jsonRPCRequests = append(jsonRPCRequests, jsonReq)
	}

	// Send the batch request
	result, err := c.sendBatchRequestWithTimeout(jsonRPCRequests, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to send batch request: %w", err)
	}

	// Handle the case where all requests were notifications (no response)
	if result == nil {
		return []BatchResponse{}, nil
	}

	// Parse the batch response
	responseArray, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid batch response format")
	}

	var responses []BatchResponse
	for _, respData := range responseArray {
		respMap, ok := respData.(map[string]interface{})
		if !ok {
			continue
		}

		response := BatchResponse{
			ID: respMap["id"],
		}

		if result, hasResult := respMap["result"]; hasResult {
			response.Result = result
		}

		if errorData, hasError := respMap["error"].(map[string]interface{}); hasError {
			response.Error = &BatchError{
				Code:    int(errorData["code"].(float64)),
				Message: errorData["message"].(string),
			}
			if data, hasData := errorData["data"]; hasData {
				response.Error.Data = data
			}
		}

		responses = append(responses, response)
	}

	return responses, nil
}

// BatchBuilder creates a new batch builder for constructing batch requests.
func (c *clientImpl) BatchBuilder() *BatchRequestBuilder {
	return &BatchRequestBuilder{
		client:   c,
		requests: make([]BatchRequest, 0),
		nextID:   0,
	}
}

// sendBatchRequestWithTimeout sends a batch request with timeout support.
func (c *clientImpl) sendBatchRequestWithTimeout(requests []map[string]interface{}, timeout time.Duration) (interface{}, error) {
	// Use the existing transport mechanism but send as a batch
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	// Marshal the batch request
	requestBytes, err := json.Marshal(requests)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	// Send via transport
	responseBytes, err := c.transport.SendWithContext(ctx, requestBytes)
	if err != nil {
		return nil, fmt.Errorf("transport error: %w", err)
	}

	// Handle empty response (all notifications)
	if len(responseBytes) == 0 {
		return nil, nil
	}

	// Parse response
	var result interface{}
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch response: %w", err)
	}

	return result, nil
}
