package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport/embedded"
)

// EmbeddedTransport implements the Transport interface for embedded (in-process) communication.
type EmbeddedTransport struct {
	transport           *embedded.Transport
	notificationHandler func(method string, params []byte)
	requestTimeout      time.Duration
	connectionTimeout   time.Duration

	// Request/response correlation
	pendingRequests    map[interface{}]chan []byte
	pendingRequestsMux sync.RWMutex

	mu sync.RWMutex
}

// NewEmbeddedTransport creates a new embedded transport client.
func NewEmbeddedTransport(transport *embedded.Transport) *EmbeddedTransport {
	return &EmbeddedTransport{
		transport:         transport,
		requestTimeout:    30 * time.Second,
		connectionTimeout: 10 * time.Second,
		pendingRequests:   make(map[interface{}]chan []byte),
	}
}

// Connect implements the Transport interface.
func (t *EmbeddedTransport) Connect() error {
	return t.ConnectWithContext(context.Background())
}

// ConnectWithContext implements the Transport interface.
func (t *EmbeddedTransport) ConnectWithContext(ctx context.Context) error {
	// Initialize and start the embedded transport
	if err := t.transport.Initialize(); err != nil {
		return err
	}

	if err := t.transport.Start(); err != nil {
		return err
	}

	// Start response processing goroutine
	// The embedded transport's internal switchboard will handle message routing
	go t.processResponses()

	return nil
}

// Disconnect implements the Transport interface.
func (t *EmbeddedTransport) Disconnect() error {
	return t.transport.Stop()
}

// Send implements the Transport interface.
func (t *EmbeddedTransport) Send(message []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), t.requestTimeout)
	defer cancel()
	return t.SendWithContext(ctx, message)
}

// SendWithContext implements the Transport interface.
func (t *EmbeddedTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	// Parse the message to extract the request ID
	var jsonMsg map[string]interface{}
	if err := json.Unmarshal(message, &jsonMsg); err != nil {
		return nil, fmt.Errorf("invalid JSON message: %w", err)
	}

	// Get the request ID
	requestID, hasID := jsonMsg["id"]
	if !hasID {
		// This is a notification, send and return immediately
		return nil, t.transport.Send(message)
	}

	// Create response channel for this request
	responseCh := make(chan []byte, 1)

	t.pendingRequestsMux.Lock()
	t.pendingRequests[requestID] = responseCh
	t.pendingRequestsMux.Unlock()

	// Clean up on exit
	defer func() {
		t.pendingRequestsMux.Lock()
		delete(t.pendingRequests, requestID)
		t.pendingRequestsMux.Unlock()
		close(responseCh)
	}()

	// Send the message
	if err := t.transport.Send(message); err != nil {
		return nil, err
	}

	// Wait for response or timeout
	select {
	case response := <-responseCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SetRequestTimeout implements the Transport interface.
func (t *EmbeddedTransport) SetRequestTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requestTimeout = timeout
}

// SetConnectionTimeout implements the Transport interface.
func (t *EmbeddedTransport) SetConnectionTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connectionTimeout = timeout
}

// RegisterNotificationHandler implements the Transport interface.
func (t *EmbeddedTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notificationHandler = handler
}

// handleMessage processes incoming messages (responses and notifications).
func (t *EmbeddedTransport) handleMessage(message []byte) ([]byte, error) {
	var jsonMsg map[string]interface{}
	if err := json.Unmarshal(message, &jsonMsg); err != nil {
		return nil, nil // Invalid JSON, ignore
	}

	// Check if this is a response (has ID and either result or error)
	if requestID, hasID := jsonMsg["id"]; hasID && (jsonMsg["result"] != nil || jsonMsg["error"] != nil) {
		// This is a response - route to pending request
		t.pendingRequestsMux.RLock()
		responseCh, exists := t.pendingRequests[requestID]
		t.pendingRequestsMux.RUnlock()

		if exists {
			select {
			case responseCh <- message:
				// Response delivered
			default:
				// Channel full or closed, ignore
			}
		}
		return nil, nil // Don't echo responses
	}

	// This is a notification or request
	if method, ok := jsonMsg["method"].(string); ok {
		t.mu.RLock()
		handler := t.notificationHandler
		t.mu.RUnlock()

		if handler != nil {
			// Pass the full message, not just params
			go handler(method, message)
		}
	}

	return nil, nil // Don't echo notifications
}

// processResponses continuously processes responses from the embedded transport.
func (t *EmbeddedTransport) processResponses() {
	// Get the response channel (this is where server sends responses to client)
	responseCh := t.transport.GetResponseChannel()

	for {
		select {
		case message, ok := <-responseCh:
			if !ok {
				// Channel closed, exit
				return
			}
			// Skip empty messages to avoid JSON parsing errors
			if len(message) == 0 {
				continue
			}
			// Process the message
			t.handleMessage(message)
		}
	}
}
