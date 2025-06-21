// Package embedded provides an in-process implementation of the MCP transport.
//
// This package implements the Transport interface for direct in-process communication,
// allowing MCP servers and clients to communicate without network overhead.
// Ideal for testing, library integration, and embedded use cases.
package embedded

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport"
)

// Transport implements the transport.Transport interface for in-process communication.
type Transport struct {
	transport.BaseTransport

	// Channels for bidirectional communication
	serverToClient chan []byte
	clientToServer chan []byte

	// Error channels
	serverErrors chan error
	clientErrors chan error

	// Control channels
	done    chan struct{}
	started bool
	mu      sync.RWMutex

	// Configuration
	bufferSize int
	timeout    time.Duration
}

// Option configures the embedded transport
type Option func(*Transport)

// WithBufferSize sets the buffer size for message channels
func WithBufferSize(size int) Option {
	return func(t *Transport) {
		t.bufferSize = size
	}
}

// WithTimeout sets the default timeout for operations
func WithTimeout(timeout time.Duration) Option {
	return func(t *Transport) {
		t.timeout = timeout
	}
}

// NewTransport creates a new embedded transport
func NewTransport(options ...Option) *Transport {
	t := &Transport{
		bufferSize: 100,              // Default buffer size
		timeout:    30 * time.Second, // Default timeout
	}

	// Apply options
	for _, option := range options {
		option(t)
	}

	return t
}

// NewTransportPair creates a connected pair of embedded transports
// Returns (serverTransport, clientTransport)
func NewTransportPair(options ...Option) (*Transport, *Transport) {
	// Create shared channels for bidirectional communication
	serverToClient := make(chan []byte, 100)
	clientToServer := make(chan []byte, 100)
	serverErrors := make(chan error, 10)
	clientErrors := make(chan error, 10)
	done := make(chan struct{})

	// Create server transport
	server := &Transport{
		serverToClient: serverToClient, // Server Send() writes here
		clientToServer: clientToServer, // Server Receive() reads here
		serverErrors:   serverErrors,
		clientErrors:   clientErrors,
		done:           done,
		bufferSize:     100,
		timeout:        30 * time.Second,
	}

	// Create client transport (channels are swapped so client and server communicate)
	client := &Transport{
		serverToClient: clientToServer, // Client Send() writes here (same as server Receive() reads)
		clientToServer: serverToClient, // Client Receive() reads here (same as server Send() writes)
		serverErrors:   clientErrors,
		clientErrors:   serverErrors,
		done:           done,
		bufferSize:     100,
		timeout:        30 * time.Second,
	}

	// Apply options to both
	for _, option := range options {
		option(server)
		option(client)
	}

	return server, client
}

// Initialize initializes the transport
func (t *Transport) Initialize() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.serverToClient == nil {
		// Initialize channels if not already set (single transport mode)
		t.serverToClient = make(chan []byte, t.bufferSize)
		t.clientToServer = make(chan []byte, t.bufferSize)
		t.serverErrors = make(chan error, 10)
		t.clientErrors = make(chan error, 10)
		t.done = make(chan struct{})
	}

	return nil
}

// Start starts the transport
func (t *Transport) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return errors.New("transport already started")
	}

	t.started = true

	// Start message processing goroutine
	go t.messageLoop()

	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.started {
		return nil
	}

	t.started = false

	// Signal shutdown
	select {
	case <-t.done:
		// Already closed
	default:
		close(t.done)
	}

	return nil
}

// Send sends a message over the transport
func (t *Transport) Send(message []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.started {
		return errors.New("transport not started")
	}

	// Create a copy of the message to avoid data races
	msgCopy := make([]byte, len(message))
	copy(msgCopy, message)

	// Send with timeout
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()

	select {
	case t.serverToClient <- msgCopy:
		return nil
	case <-ctx.Done():
		return errors.New("send timeout")
	case <-t.done:
		return errors.New("transport stopped")
	}
}

// Receive receives a message from the transport
func (t *Transport) Receive() ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.started {
		return nil, errors.New("transport not started")
	}

	select {
	case message := <-t.clientToServer:
		return message, nil
	case err := <-t.clientErrors:
		return nil, err
	case <-t.done:
		return nil, errors.New("transport stopped")
	}
}

// messageLoop processes incoming messages using the message handler
func (t *Transport) messageLoop() {
	for {
		select {
		case message := <-t.clientToServer:
			// Process message with handler
			go func(msg []byte) {
				response, err := t.HandleMessage(msg)
				if err != nil {
					// Send error to client
					select {
					case t.serverErrors <- err:
					case <-t.done:
					default:
						// Error channel full, log and continue
						if logger := t.GetLogger(); logger != nil {
							logger.Error("embedded transport: error channel full", "error", err)
						}
					}
					return
				}

				if response != nil {
					// Send response back
					select {
					case t.serverToClient <- response:
					case <-t.done:
					default:
						// Response channel full, log and continue
						if logger := t.GetLogger(); logger != nil {
							logger.Error("embedded transport: response channel full")
						}
					}
				}
			}(message)
		case <-t.done:
			return
		}
	}
}

// Note: GetMessageHandler is not needed - we use HandleMessage directly from BaseTransport

// IsStarted returns whether the transport is started
func (t *Transport) IsStarted() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.started
}

// GetChannelStats returns statistics about channel usage (for debugging)
func (t *Transport) GetChannelStats() map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return map[string]int{
		"serverToClient": len(t.serverToClient),
		"clientToServer": len(t.clientToServer),
		"serverErrors":   len(t.serverErrors),
		"clientErrors":   len(t.clientErrors),
	}
}

// GetResponseChannel returns the channel for receiving responses (for client use)
func (t *Transport) GetResponseChannel() <-chan []byte {
	return t.serverToClient
}
