// Package sse provides a Server-Sent Events implementation of the MCP transport.
//
// This package implements the Transport interface using Server-Sent Events (SSE),
// suitable for applications requiring server-to-client real-time updates.
package sse

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport"
)

// Option is a function that configures a Transport
type Option func(*Transport)

// Options provides a fluent API for configuring SSE transport options
type Options struct{}

// SSE provides access to SSE transport configuration options
var SSE = Options{}

// WithPathPrefix returns an option that sets the path prefix for all endpoints
func (Options) WithPathPrefix(prefix string) Option {
	return func(t *Transport) {
		t.SetPathPrefix(prefix)
	}
}

// WithEventsPath returns an option that sets the path for the SSE events endpoint
func (Options) WithEventsPath(path string) Option {
	return func(t *Transport) {
		t.SetEventPath(path)
	}
}

// WithMessagePath returns an option that sets the path for the message posting endpoint
func (Options) WithMessagePath(path string) Option {
	return func(t *Transport) {
		t.SetMessagePath(path)
	}
}

// DefaultShutdownTimeout is the default timeout for graceful shutdown
const DefaultShutdownTimeout = 10 * time.Second

// DefaultEventsPath is the default endpoint path for SSE connections
const DefaultEventsPath = "/sse"

// DefaultMessagePath is the default endpoint path for message posting
const DefaultMessagePath = "/message"

// Transport implements the transport.Transport interface for SSE
type Transport struct {
	transport.BaseTransport
	addr     string
	server   *http.Server
	isClient bool

	// For server mode
	clients     map[string]chan []byte // Map client ID to message channel
	clientsMu   sync.Mutex
	pathPrefix  string // Optional prefix for endpoint paths (e.g., "/mcp")
	eventsPath  string // Endpoint for SSE connections
	messagePath string // Endpoint for receiving messages

	// For client mode
	url          string
	client       *http.Client
	readCh       chan []byte
	errCh        chan error
	doneCh       chan struct{}
	connected    bool
	connMu       sync.Mutex
	postEndpoint string // Endpoint for sending messages (received from server)
}

// NewTransport creates a new SSE transport
func NewTransport(addr string) *Transport {
	isClient := strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://")

	t := &Transport{
		addr:       addr,
		isClient:   isClient,
		pathPrefix: "", // Empty by default
	}

	if isClient {
		t.url = addr
		t.client = &http.Client{}
		t.readCh = make(chan []byte, 100)
		t.errCh = make(chan error, 1)
		t.doneCh = make(chan struct{})
	} else {
		t.clients = make(map[string]chan []byte)
		// Set default endpoint paths
		t.eventsPath = DefaultEventsPath
		t.messagePath = DefaultMessagePath
	}

	return t
}

// SetPathPrefix sets a prefix for all endpoint paths
// For example, SetPathPrefix("/mcp") will result in endpoints like "/mcp/sse"
func (t *Transport) SetPathPrefix(prefix string) *Transport {
	if !t.isClient {
		// Ensure the prefix starts with a slash if not empty
		if prefix != "" && !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		t.pathPrefix = prefix
	}
	return t
}

// SetEventPath sets the path for the SSE events endpoint
func (t *Transport) SetEventPath(path string) *Transport {
	if !t.isClient {
		t.eventsPath = path
	}
	return t
}

// SetMessagePath sets the path for the message posting endpoint
func (t *Transport) SetMessagePath(path string) *Transport {
	if !t.isClient {
		t.messagePath = path
	}
	return t
}

// GetFullEventsPath returns the complete path for the events endpoint
func (t *Transport) GetFullEventsPath() string {
	if t.pathPrefix == "" {
		return t.eventsPath
	}
	return t.pathPrefix + t.eventsPath
}

// GetFullMessagePath returns the complete path for the message endpoint
func (t *Transport) GetFullMessagePath() string {
	if t.pathPrefix == "" {
		return t.messagePath
	}
	return t.pathPrefix + t.messagePath
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	if t.isClient {
		// Client mode - nothing to initialize yet
		// We'll connect when Start is called
		return nil
	}

	// Server mode - nothing to initialize yet
	// We'll start the HTTP server when Start is called
	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	if t.isClient {
		// Start the client connection
		go t.startClientConnection()
		return nil
	}

	// Start the server
	mux := http.NewServeMux()

	// SSE endpoint for clients to connect and receive messages
	mux.HandleFunc(t.GetFullEventsPath(), t.handleSSERequest)

	// HTTP POST endpoint for clients to send messages
	mux.HandleFunc(t.GetFullMessagePath(), t.handleMessageRequest)

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error
		}
	}()

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	if t.isClient {
		close(t.doneCh)
		t.connMu.Lock()
		t.connected = false
		t.connMu.Unlock()
		return nil
	}

	// Server mode
	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	// Notify all clients that we're shutting down
	t.clientsMu.Lock()
	for _, clientCh := range t.clients {
		close(clientCh)
	}
	t.clients = make(map[string]chan []byte)
	t.clientsMu.Unlock()

	// Shutdown the server
	return t.server.Shutdown(ctx)
}

// Send sends a message
func (t *Transport) Send(message []byte) error {
	if t.isClient {
		// In client mode, use the POST endpoint received from the server
		t.connMu.Lock()
		postEndpoint := t.postEndpoint
		connected := t.connected
		t.connMu.Unlock()

		if !connected || postEndpoint == "" {
			return errors.New("not connected to server or missing POST endpoint")
		}

		// Send message to server via HTTP POST
		req, err := http.NewRequest("POST", postEndpoint, bytes.NewReader(message))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := t.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		return nil
	}

	// Server mode - send to all clients
	t.clientsMu.Lock()
	defer t.clientsMu.Unlock()

	for _, clientCh := range t.clients {
		select {
		case clientCh <- message:
			// Message sent
		default:
			// Client channel full, message dropped
		}
	}

	return nil
}

// Receive receives a message (client mode only)
func (t *Transport) Receive() ([]byte, error) {
	if !t.isClient {
		return nil, errors.New("receive is only supported in client mode")
	}

	t.connMu.Lock()
	connected := t.connected
	t.connMu.Unlock()

	if !connected {
		return nil, errors.New("not connected to server")
	}

	select {
	case msg := <-t.readCh:
		return msg, nil
	case err := <-t.errCh:
		return nil, err
	case <-t.doneCh:
		return nil, errors.New("transport closed")
	}
}

// generateClientID creates a unique client ID
func (t *Transport) generateClientID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
}

// handleSSERequest handles incoming SSE connection requests
func (t *Transport) handleSSERequest(w http.ResponseWriter, r *http.Request) {
	t.GetLogger().Debug("New SSE connection", "remote_addr", r.RemoteAddr)

	// Validate Origin header for security
	origin := r.Header.Get("Origin")
	if origin != "" {
		// In a production environment, implement proper origin validation
		// For now, we'll accept any origin for development purposes
		w.Header().Set("Access-Control-Allow-Origin", origin)
		t.GetLogger().Debug("Origin header received", "origin", origin)
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	t.GetLogger().Debug("Set SSE headers")

	// Generate a unique client ID
	clientID := t.generateClientID()
	t.GetLogger().Debug("Generated client ID", "client_id", clientID)

	// Create a channel for this client
	clientCh := make(chan []byte, 10)

	// Register the client
	t.clientsMu.Lock()
	t.clients[clientID] = clientCh
	t.clientsMu.Unlock()
	t.GetLogger().Debug("Registered client", "client_id", clientID)

	// Create the full message endpoint for this client
	messageURL := fmt.Sprintf("http://%s%s", r.Host, t.GetFullMessagePath())
	t.GetLogger().Debug("Message endpoint URL", "url", messageURL)

	// Clean up when the client disconnects
	defer func() {
		t.GetLogger().Debug("Client disconnected", "client_id", clientID)
		t.clientsMu.Lock()
		delete(t.clients, clientID)
		close(clientCh)
		t.clientsMu.Unlock()
	}()

	// Ensure the connection stays open with a flush
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.GetLogger().Debug("Streaming not supported by client")
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial endpoint event to tell the client where to send messages
	t.GetLogger().Debug("Sending endpoint event", "url", messageURL)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageURL)
	flusher.Flush()
	t.GetLogger().Debug("Flushed endpoint event")

	// Handle client disconnect
	clientClosed := r.Context().Done()
	t.GetLogger().Debug("Waiting for client messages or disconnect")

	// Send events to the client
	for {
		select {
		case <-clientClosed:
			// Client disconnected
			t.GetLogger().Debug("Client context done, client disconnected")
			return
		case msg, ok := <-clientCh:
			if !ok {
				// Channel closed
				t.GetLogger().Debug("Client channel closed")
				return
			}

			// Format the message as an SSE event
			t.GetLogger().Debug("Sending message to client", "message", string(msg))
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
			t.GetLogger().Debug("Flushed message to client")
		}
	}
}

// handleMessageRequest handles incoming client messages via HTTP POST
func (t *Transport) handleMessageRequest(w http.ResponseWriter, r *http.Request) {
	// Validate method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate content type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	// Read message
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Process the message
	var response []byte
	response, err = t.HandleMessage(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error processing message: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response if available
	if response != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	} else {
		// No response, return empty success
		w.WriteHeader(http.StatusOK)
	}
}

// startClientConnection establishes and maintains the SSE connection
func (t *Transport) startClientConnection() {
	defer func() {
		t.connMu.Lock()
		t.connected = false
		t.connMu.Unlock()
	}()

	for {
		select {
		case <-t.doneCh:
			return
		default:
			// Attempt to connect or reconnect
			if err := t.connectToSSE(); err != nil {
				select {
				case t.errCh <- err:
					// Error sent
				default:
					// Error channel full, discard
				}

				// Wait before reconnecting
				select {
				case <-time.After(5 * time.Second):
					// Try again
				case <-t.doneCh:
					return
				}
			}
		}
	}
}

// connectToSSE establishes a connection to the SSE server
func (t *Transport) connectToSSE() error {
	// Connect to the events endpoint
	eventsURL := t.url

	// Only append the SSE endpoint if the URL doesn't already end with it
	// or doesn't already contain it as a query parameter
	if !strings.HasSuffix(eventsURL, DefaultEventsPath) &&
		!strings.Contains(eventsURL, DefaultEventsPath+"?") {
		// Append the default events path if not already present
		if !strings.HasSuffix(eventsURL, "/") {
			eventsURL += "/"
		}
		eventsURL = strings.TrimSuffix(eventsURL, "/") + DefaultEventsPath
	}

	// Log connection attempt
	t.GetLogger().Debug("Connecting to SSE server", "url", eventsURL)

	req, err := http.NewRequest("GET", eventsURL, nil)
	if err != nil {
		errMsg := fmt.Errorf("Failed to create SSE request: %w", err)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg.Error())
		}
		return errMsg
	}

	// Set headers for SSE request
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Context that can be canceled when Stop is called
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register cancel function to be called when doneCh is closed
	go func() {
		select {
		case <-t.doneCh:
			cancel()
		case <-ctx.Done():
			// Context already canceled
			return
		}
	}()

	req = req.WithContext(ctx)

	if debugHandler := t.GetDebugHandler(); debugHandler != nil {
		debugHandler("Sending SSE connection request...")
	}
	t.GetLogger().Debug("Sending SSE request")

	resp, err := t.client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("SSE request failed: %v", err)
		t.GetLogger().Debug("SSE request failed", "error", err)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg)
		}
		return errors.New(errMsg)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("SSE request returned status code %d", resp.StatusCode)
		t.GetLogger().Debug("SSE request returned error status", "status_code", resp.StatusCode)
		if debugHandler := t.GetDebugHandler(); debugHandler != nil {
			debugHandler(errMsg)
		}
		return errors.New(errMsg)
	}

	t.GetLogger().Debug("SSE connection established, parsing events")

	// Parse SSE events
	reader := bufio.NewReader(resp.Body)
	var buf bytes.Buffer
	var eventType string

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				t.GetLogger().Debug("SSE connection closed (EOF)")
				break
			}
			t.GetLogger().Debug("Error reading SSE stream", "error", err)
			return err
		}

		line = bytes.TrimSpace(line)
		t.GetLogger().Debug("SSE line received", "line", string(line))

		// Skip comment lines
		if bytes.HasPrefix(line, []byte(":")) {
			continue
		}

		// Handle event type
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("event:"))))
			t.GetLogger().Debug("Event type", "type", eventType)
			continue
		}

		// Handle data lines
		if bytes.HasPrefix(line, []byte("data:")) {
			// Extract the data
			data := bytes.TrimPrefix(line, []byte("data:"))
			data = bytes.TrimSpace(data)
			buf.Write(data)
			t.GetLogger().Debug("Event data", "data", string(data))
		} else if len(line) == 0 && buf.Len() > 0 {
			// Empty line indicates end of event
			msg := buf.Bytes()
			t.GetLogger().Debug("Complete event received", "data", string(msg), "type", eventType)

			// Handle different event types
			if eventType == "endpoint" {
				// Store the message endpoint
				t.connMu.Lock()
				t.postEndpoint = string(msg)
				t.connected = true
				t.GetLogger().Debug("POST endpoint set", "endpoint", t.postEndpoint)
				t.connMu.Unlock()

				// Notify that connection is established with an encoded JSON response
				// that includes both connected status and the endpoint URL
				jsonResp := fmt.Sprintf(`{"connected":true,"endpoint":"%s"}`, string(msg))
				select {
				case t.readCh <- []byte(jsonResp):
					t.GetLogger().Debug("Sent connected notification with endpoint")
				default:
					t.GetLogger().Debug("Connected notification channel full, skipping")
				}
			} else if eventType == "message" || eventType == "" {
				// Regular message, process it
				response, err := t.HandleMessage(msg)
				if err != nil {
					t.GetLogger().Debug("Error handling message", "error", err)
					// Log error but continue processing
					buf.Reset()
					eventType = ""
					continue
				}

				if response != nil {
					t.GetLogger().Debug("Sending response", "response", string(response))
					select {
					case t.readCh <- response:
						// Message sent
					default:
						// Channel full, discard oldest message
						<-t.readCh
						t.readCh <- response
					}
				}
			}

			// Reset buffer and event type for next event
			buf.Reset()
			eventType = ""
		}
	}

	t.GetLogger().Debug("SSE connection closed")
	return errors.New("SSE connection closed")
}

// GetAddr returns the transport's address
func (t *Transport) GetAddr() string {
	return t.addr
}
