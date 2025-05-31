// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/transport/sse"
)

// SSETransport adapts the sse.Transport to implement the client.Transport interface
type SSETransport struct {
	transport           *sse.Transport
	requestTimeout      time.Duration
	connectionTimeout   time.Duration
	notificationHandler func(method string, params []byte)
	respChan            chan []byte // channel for receiving responses
	respErr             chan error  // channel for receiving errors
	connected           atomic.Bool
	postEndpoint        atomic.Pointer[string] // endpoint for sending messages (received from server)
	debugEnabled        bool
	logger              *slog.Logger
}

// NewSSETransport creates a new SSE transport adapter.
func NewSSETransport(url string, logger *slog.Logger) *SSETransport {
	// Ensure the URL uses a valid scheme (http:// or https://)
	// First check if it's an SSE scheme, which we'll convert to HTTP
	if strings.HasPrefix(url, "sse://") {
		url = "http://" + url[6:]
		logger.Debug("Converting 'sse://' to 'http://'", "url", url)
	} else if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// If no scheme or unsupported scheme, default to http://
		if !strings.Contains(url, "://") {
			url = "http://" + url
			logger.Debug("No scheme provided, defaulting to 'http://'", "url", url)
		} else {
			// Extract host and path from URL with unknown scheme
			parts := strings.SplitN(url, "://", 2)
			if len(parts) == 2 {
				url = "http://" + parts[1]
				logger.Debug("Converting unknown scheme to 'http://'", "url", url)
			}
		}
	}

	t := &SSETransport{
		transport:         sse.NewTransport(url),
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		respChan:          make(chan []byte, 10),
		respErr:           make(chan error, 5),
		connected:         atomic.Bool{},
		debugEnabled:      true,
		logger:            logger,
	}

	// Set message handler to capture responses
	t.transport.SetMessageHandler(t.handleMessage)

	// Set debug handler
	t.transport.SetDebugHandler(func(msg string) {
		if t.debugEnabled {
			logger.Debug("SSE transport debug", "message", msg)
		}
	})

	return t
}

// handleMessage processes incoming messages and routes them accordingly
func (t *SSETransport) handleMessage(message []byte) ([]byte, error) {
	t.logger.Debug("SSE adapter received message", "bytes", len(message), "message", string(message))

	// Check if this looks like the endpoint message
	// The endpoint message could be either:
	// 1. A plain URL string starting with http(s)://
	// 2. A JSON object containing an endpoint field

	// Case 1: Plain URL string
	if len(message) > 0 && (bytes.HasPrefix(message, []byte("http://")) ||
		bytes.HasPrefix(message, []byte("https://"))) {
		t.logger.Debug("Detected endpoint URL (direct)", "endpoint", string(message))
		t.handleEndpointMessage(message)
		return nil, nil
	}

	// Case 2: JSON object with endpoint
	var jsonMsg map[string]interface{}
	if err := json.Unmarshal(message, &jsonMsg); err == nil {
		// Check if it's an endpoint notification
		if endpoint, ok := jsonMsg["endpoint"].(string); ok && strings.HasPrefix(endpoint, "http") {
			t.logger.Debug("Detected endpoint URL (in JSON)", "endpoint", endpoint)
			t.handleEndpointMessage([]byte(endpoint))
			return nil, nil
		}

		// Check if it's a connected notification
		if connected, ok := jsonMsg["connected"].(bool); ok && connected {
			t.logger.Debug("Received connected notification")
			// If we don't have an endpoint yet but received confirmation, use the base URL
			t.connected.Store(true)
			return nil, nil
		}
	}

	// Forward to notification handler if it's a notification
	if t.notificationHandler != nil {
		// Try to determine if this is a JSON-RPC notification vs a response
		var msg struct {
			ID interface{} `json:"id"`
		}
		if err := json.Unmarshal(message, &msg); err == nil && msg.ID == nil {
			// No ID means it's a notification
			t.logger.Debug("Detected notification (no ID), forwarding to handler")
			go t.notificationHandler("", message)
			return nil, nil
		}
	}

	// Put on response channel for any waiting requests
	t.logger.Debug("Putting message on response channel")
	select {
	case t.respChan <- message:
		t.logger.Debug("Successfully put message on response channel")
	default:
		t.logger.Debug("Response channel full or no one waiting")
	}

	// Return nil to prevent the SSE transport from automatically responding
	return nil, nil
}

// handleEndpointMessage processes and stores the endpoint URL
func (t *SSETransport) handleEndpointMessage(message []byte) {
	endpointURL := string(message)

	t.logger.Debug("Processing endpoint URL", "endpoint", endpointURL)

	t.postEndpoint.Store(&endpointURL)

	// Notify that the endpoint has been received
	if t.notificationHandler != nil {
		t.logger.Debug("Calling notification handler with endpoint")
		t.notificationHandler("endpoint", message)
	} else {
		t.logger.Debug("No notification handler registered")
	}

	// Signal connection success if this is the first time
	if !t.connected.Load() {
		t.logger.Debug("Connection established with endpoint")
		select {
		case t.respChan <- []byte(`{"connected":true}`):
			t.logger.Debug("Sent connected notification")
		default:
			t.logger.Debug("Connected notification channel full, skipping")
		}
	}
}

// Connect establishes a connection to the server.
func (t *SSETransport) Connect() error {
	endpointPtr := t.postEndpoint.Load()
	if t.connected.Load() && endpointPtr != nil && *endpointPtr != "" {
		t.logger.Debug("Already connected with endpoint", "endpoint", *endpointPtr)
		return nil
	}

	// Initialize the transport
	if err := t.transport.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize SSE transport: %w", err)
	}

	t.logger.Debug("Starting transport")

	// Start the transport
	if err := t.transport.Start(); err != nil {
		return fmt.Errorf("failed to start SSE transport: %w", err)
	}

	t.logger.Debug("Transport started")

	// For 2025-03-26 unified MCP endpoint behavior:
	// The transport connects to /mcp directly and stores the URL for POST requests
	// We should check if the transport has already established the MCP URL

	// Wait a short time for the transport to establish connection
	waitTime := 2 * time.Second
	if t.connectionTimeout < waitTime {
		waitTime = t.connectionTimeout
	}

	startTime := time.Now()
	for time.Since(startTime) < waitTime {
		// Check if the underlying transport has connected and has the MCP URL
		mcpURL := t.transport.GetStoredMCPURL()
		if mcpURL != "" {
			t.postEndpoint.Store(&mcpURL)
			t.connected.Store(true)
			t.logger.Debug("Got MCP endpoint from transport", "endpoint", mcpURL)
			return nil
		}

		// Short wait before checking again
		time.Sleep(100 * time.Millisecond)
	}

	// If we didn't get the endpoint from the transport, derive it
	t.logger.Debug("Deriving unified MCP endpoint URL")
	baseURL := t.transport.GetAddr()
	// For 2025-03-26 spec, use the unified MCP endpoint
	if !strings.HasSuffix(baseURL, "/mcp") {
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		baseURL = strings.TrimSuffix(baseURL, "/") + "/mcp"
	}
	t.postEndpoint.Store(&baseURL)
	t.connected.Store(true)
	t.logger.Debug("Using derived unified MCP endpoint URL", "endpoint", baseURL)

	return nil
}

// ConnectWithContext establishes a connection to the server with context for timeout/cancellation.
func (t *SSETransport) ConnectWithContext(ctx context.Context) error {
	// Create a channel to signal completion
	done := make(chan error, 1)

	// Start connection in a goroutine
	go func() {
		done <- t.Connect()
	}()

	// Wait for connection or context cancellation
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Disconnect closes the connection to the server.
func (t *SSETransport) Disconnect() error {
	if !t.connected.Load() {
		return nil
	}

	// Mark as disconnected before stopping to prevent reconnection attempts
	t.connected.Store(false)
	endpointPtr := t.postEndpoint.Load()
	var postEndpoint string
	if endpointPtr != nil {
		postEndpoint = *endpointPtr
	}
	t.postEndpoint.Store(nil)

	if t.debugEnabled {
		t.logger.Debug("Disconnecting from endpoint", "endpoint", postEndpoint)
	}

	// Stop the transport
	err := t.transport.Stop()
	if err != nil && t.debugEnabled {
		t.logger.Debug("Error stopping transport", "error", err)
	}

	return err
}

// Send sends a message to the server and waits for a response.
func (t *SSETransport) Send(message []byte) ([]byte, error) {
	return t.SendWithContext(context.Background(), message)
}

// SendWithContext sends a message with context for timeout/cancellation.
func (t *SSETransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	// Add more detailed debug logging about the message
	var msgType string
	// Try to detect if this is an initialization message with capabilities
	if bytes.Contains(message, []byte(`"method":"initialize"`)) {
		msgType = "initialize"
	} else if bytes.Contains(message, []byte(`"method":"ping"`)) {
		msgType = "ping"
	} else {
		msgType = "regular"
	}

	t.logger.Debug("Sending message", "type", msgType, "message", string(message))

	// Check if we're connected
	if !t.connected.Load() {
		t.logger.Debug("Error - not connected to SSE server")
		return nil, fmt.Errorf("not connected to SSE server")
	}

	// Get the endpoint URL
	endpointPtr := t.postEndpoint.Load()
	if endpointPtr == nil {
		t.logger.Debug("Error - missing POST endpoint URL")
		return nil, fmt.Errorf("missing POST endpoint URL")
	}
	postEndpoint := *endpointPtr

	if postEndpoint == "" {
		t.logger.Debug("Error - empty POST endpoint URL")
		return nil, fmt.Errorf("empty POST endpoint URL")
	}

	// Create the HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", postEndpoint, bytes.NewReader(message))
	if err != nil {
		t.logger.Debug("Error creating request", "error", err)
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set appropriate headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Create a client with appropriate timeout
	client := &http.Client{
		Timeout: t.requestTimeout,
	}

	t.logger.Debug("Sending HTTP POST to", "endpoint", postEndpoint)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		t.logger.Debug("Error sending HTTP POST", "error", err)
		// Check if error was due to context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Errorf("HTTP request failed with status: %d, body: %s", resp.StatusCode, string(body))
		t.logger.Debug(errMsg.Error())
		return nil, errMsg
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.logger.Debug("Error reading response", "error", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	t.logger.Debug("Received response", "bytes", len(body), "response", string(body))

	// Check for empty response
	if len(body) == 0 {
		t.logger.Debug("Empty response body")
		return nil, nil
	}

	return body, nil
}

// SetRequestTimeout sets the default timeout for request operations.
func (t *SSETransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
	if t.debugEnabled {
		t.logger.Debug("Request timeout set to", "timeout", timeout)
	}
}

// SetConnectionTimeout sets the default timeout for connection operations.
func (t *SSETransport) SetConnectionTimeout(timeout time.Duration) {
	t.connectionTimeout = timeout
	if t.debugEnabled {
		t.logger.Debug("Connection timeout set to", "timeout", timeout)
	}
}

// RegisterNotificationHandler registers a handler for server-initiated messages.
func (t *SSETransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.notificationHandler = handler
	if t.debugEnabled {
		t.logger.Debug("Notification handler registered")
	}
}

// SetDebugEnabled enables or disables debug logging
func (t *SSETransport) SetDebugEnabled(enabled bool) {
	t.debugEnabled = enabled
}

// WithSSE returns a client configuration option that uses SSE transport.
// The SSE transport provides server-sent events for real-time updates from server to client.
// By default, it uses the oldest protocol version for maximum compatibility unless
// the user has explicitly set a different protocol version.
//
// Parameters:
//   - url: The SSE server URL to connect to (e.g., "sse://localhost:8080", "http://localhost:8080")
//
// Returns:
//   - A client configuration option
func WithSSE(url string) Option {
	return func(c *clientImpl) {
		// Log the configuration
		c.logger.Debug("Configuring SSE transport with URL", "url", url)

		// Create and configure the SSE transport adapter
		transport := NewSSETransport(url, c.logger)

		// Always enable debug logging for now
		transport.SetDebugEnabled(true)

		// Set timeouts if specified
		transport.SetRequestTimeout(c.requestTimeout)
		transport.SetConnectionTimeout(c.connectionTimeout)

		// Set the transport on the client
		c.transport = transport

		// If user hasn't explicitly set a protocol version, use the oldest one
		// for maximum compatibility with SSE connections
		if c.negotiatedVersion == "" {
			// Get the last element in the supported versions slice, which is the oldest
			if len(mcp.SupportedVersions) > 0 {
				c.negotiatedVersion = mcp.SupportedVersions[len(mcp.SupportedVersions)-1]
				c.logger.Debug("Using oldest protocol version for maximum compatibility", "version", c.negotiatedVersion)
			}
		}
	}
}
