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
	"sync"
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
	mu                  sync.Mutex
	respChan            chan []byte // channel for receiving responses
	respErr             chan error  // channel for receiving errors
	connected           bool
	postEndpoint        string // endpoint for sending messages (received from server)
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
		connected:         false,
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
			t.mu.Lock()
			if t.postEndpoint == "" {
				baseURL := t.transport.GetAddr()
				t.logger.Debug("Base URL from transport", "base_url", baseURL)
				if !strings.HasSuffix(baseURL, "/message") {
					if !strings.HasSuffix(baseURL, "/") {
						baseURL += "/"
					}
					baseURL += "message"
				}
				t.postEndpoint = baseURL
				t.connected = true
				t.logger.Debug("Derived endpoint URL", "endpoint", baseURL)

				// Notify about the endpoint
				if t.notificationHandler != nil {
					go t.notificationHandler("endpoint", []byte(t.postEndpoint))
				}
			}
			t.mu.Unlock()
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

	t.mu.Lock()
	t.postEndpoint = endpointURL
	wasConnected := t.connected
	t.connected = true
	t.mu.Unlock()

	t.logger.Debug("Stored endpoint URL", "endpoint", endpointURL, "was_connected", wasConnected)

	// Notify that the endpoint has been received
	if t.notificationHandler != nil {
		t.logger.Debug("Calling notification handler with endpoint")
		t.notificationHandler("endpoint", message)
	} else {
		t.logger.Debug("No notification handler registered")
	}

	// Signal connection success if this is the first time
	if !wasConnected {
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
	t.mu.Lock()
	if t.connected && t.postEndpoint != "" {
		t.mu.Unlock()
		t.logger.Debug("Already connected with endpoint", "endpoint", t.postEndpoint)
		return nil
	}
	t.mu.Unlock()

	// Initialize the transport
	if err := t.transport.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize SSE transport: %w", err)
	}

	t.logger.Debug("Starting transport")

	// Start the transport
	if err := t.transport.Start(); err != nil {
		return fmt.Errorf("failed to start SSE transport: %w", err)
	}

	t.logger.Debug("Transport started, waiting for endpoint")

	// We need to wait for the endpoint URL to be received
	endpointReceived := make(chan struct{})

	// Create a temporary notification handler that will signal when endpoint is received
	previousHandler := t.notificationHandler
	t.mu.Lock()
	t.notificationHandler = func(method string, params []byte) {
		t.logger.Debug("Notification handler called with method", "method", method, "params", string(params))

		// Call the previous handler if it exists
		if previousHandler != nil {
			previousHandler(method, params)
		}

		// If this is the endpoint notification, signal that we received it
		if method == "endpoint" {
			t.logger.Debug("Endpoint notification received", "endpoint", string(params))
			select {
			case <-endpointReceived: // Already closed
				t.logger.Debug("Endpoint already received, ignoring duplicate")
			default:
				t.logger.Debug("Signaling endpoint received")
				close(endpointReceived)
			}
		} else if t.postEndpoint != "" {
			// If we already have the endpoint URL but haven't signaled it yet
			t.logger.Debug("We have endpoint URL but notification came through different channel")
			select {
			case <-endpointReceived: // Already closed
				t.logger.Debug("Channel already closed, ignoring")
			default:
				t.logger.Debug("Signaling endpoint received")
				close(endpointReceived)
			}
		}
	}
	t.mu.Unlock()

	// Check if we got the endpoint while setting up the handler
	t.mu.Lock()
	if t.connected && t.postEndpoint != "" {
		t.mu.Unlock()
		t.logger.Debug("Endpoint was already set", "endpoint", t.postEndpoint)
		select {
		case <-endpointReceived: // Already closed
			t.logger.Debug("Channel already closed")
		default:
			t.logger.Debug("Closing endpoint channel")
			close(endpointReceived)
		}
	} else {
		t.mu.Unlock()
		t.logger.Debug("Endpoint not set yet, waiting for it")
	}

	// Wait for the endpoint with a timeout
	t.logger.Debug("Waiting for endpoint signal with timeout", "timeout", t.connectionTimeout)
	select {
	case <-endpointReceived:
		// Endpoint received, connection established
		t.logger.Debug("Connection successfully established")
		t.mu.Lock()
		t.logger.Debug("Final endpoint URL", "endpoint", t.postEndpoint)
		t.connected = true
		t.mu.Unlock()
		return nil
	case <-time.After(t.connectionTimeout / 2):
		// Timeout waiting for endpoint - use a derived endpoint
		t.logger.Debug("Partial timeout - generating default endpoint URL")
		t.mu.Lock()
		baseURL := t.transport.GetAddr()
		// If we don't already have a post endpoint, derive one
		if t.postEndpoint == "" {
			if !strings.HasSuffix(baseURL, "/message") {
				if !strings.HasSuffix(baseURL, "/") {
					baseURL += "/"
				}
				baseURL += "message"
			}
			t.postEndpoint = baseURL
			t.logger.Debug("Using derived endpoint URL", "endpoint", baseURL)
		}
		t.connected = true
		t.mu.Unlock()

		// Even if we didn't receive the notification, let's try to proceed
		return nil
	}
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
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		return nil
	}

	// Mark as disconnected before stopping to prevent reconnection attempts
	t.connected = false
	postEndpoint := t.postEndpoint
	t.postEndpoint = ""
	t.mu.Unlock()

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
	t.mu.Lock()
	if !t.connected {
		t.mu.Unlock()
		t.logger.Debug("Error - not connected to SSE server")
		return nil, fmt.Errorf("not connected to SSE server")
	}

	// Get the endpoint URL
	postEndpoint := t.postEndpoint
	t.mu.Unlock()

	if postEndpoint == "" {
		t.logger.Debug("Error - missing POST endpoint URL")
		return nil, fmt.Errorf("missing POST endpoint URL")
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
	t.mu.Lock()
	defer t.mu.Unlock()

	t.requestTimeout = timeout
	if t.debugEnabled {
		t.logger.Debug("Request timeout set to", "timeout", timeout)
	}
}

// SetConnectionTimeout sets the default timeout for connection operations.
func (t *SSETransport) SetConnectionTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connectionTimeout = timeout
	if t.debugEnabled {
		t.logger.Debug("Connection timeout set to", "timeout", timeout)
	}
}

// RegisterNotificationHandler registers a handler for server-initiated messages.
func (t *SSETransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.notificationHandler = handler
	if t.debugEnabled {
		t.logger.Debug("Notification handler registered")
	}
}

// SetDebugEnabled enables or disables debug logging
func (t *SSETransport) SetDebugEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

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
