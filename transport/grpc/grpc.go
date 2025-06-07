// Package grpc provides a gRPC transport implementation for the MCP protocol.
//
// This transport uses gRPC for communication between clients and servers,
// offering bi-directional streaming and strongly-typed interactions.
// It supports both client and server modes, TLS encryption, and various
// configuration options for fine-tuning performance and reliability.
package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/localrivet/gomcp/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Default configuration values.
const (
	DefaultPort              = 50051
	DefaultConnectionTimeout = 10 * time.Second
	DefaultMaxMessageSize    = 4 * 1024 * 1024 // 4MB
	DefaultKeepAliveTime     = 10 * time.Second
	DefaultKeepAliveTimeout  = 3 * time.Second
	DefaultBufferSize        = 100
)

// Transport errors.
var (
	ErrNotInitialized = errors.New("transport not initialized")
	ErrAlreadyRunning = errors.New("transport already running")
	ErrNotRunning     = errors.New("transport not running")
	ErrInvalidConfig  = errors.New("invalid transport configuration")
)

// Option represents a configuration option for the gRPC transport.
type Option func(*Transport)

// WithAddress sets the server address.
func WithAddress(address string) Option {
	return func(t *Transport) {
		t.address = address
	}
}

// WithTLS enables TLS with the provided credentials.
func WithTLS(certFile, keyFile, caFile string) Option {
	return func(t *Transport) {
		t.useTLS = true
		t.tlsCertFile = certFile
		t.tlsKeyFile = keyFile
		t.tlsCAFile = caFile
	}
}

// WithMaxMessageSize sets the maximum message size.
func WithMaxMessageSize(size int) Option {
	return func(t *Transport) {
		if size > 0 {
			t.maxMessageSize = size
		}
	}
}

// WithConnectionTimeout sets the connection timeout.
func WithConnectionTimeout(timeout time.Duration) Option {
	return func(t *Transport) {
		if timeout > 0 {
			t.connectionTimeout = timeout
		}
	}
}

// WithBufferSize sets the buffer size for message channels.
func WithBufferSize(size int) Option {
	return func(t *Transport) {
		if size > 0 {
			t.bufferSize = size
		}
	}
}

// WithKeepAliveParams sets the keepalive parameters.
func WithKeepAliveParams(time, timeout time.Duration) Option {
	return func(t *Transport) {
		if time > 0 {
			t.keepAliveTime = time
		}
		if timeout > 0 {
			t.keepAliveTimeout = timeout
		}
	}
}

// Transport implements the transport.Transport interface using gRPC.
//
// It provides bidirectional communication between clients and servers using
// the Protocol Buffers serialization format and gRPC streaming capabilities.
// The Transport can operate in both server and client modes.
type Transport struct {
	// Base transport functionality
	transport.BaseTransport

	// Core transport options
	isServer bool
	address  string
	running  bool

	// TLS configuration
	useTLS      bool
	tlsCertFile string
	tlsKeyFile  string
	tlsCAFile   string

	// gRPC specific options
	maxMessageSize    int
	connectionTimeout time.Duration
	keepAliveTime     time.Duration
	keepAliveTimeout  time.Duration
	bufferSize        int

	// Runtime state
	server     *grpc.Server
	clientConn *grpc.ClientConn
	ctx        context.Context
	cancel     context.CancelFunc
	runningMu  sync.Mutex
	closeOnce  sync.Once

	// Channels for messaging
	sendCh chan []byte
	recvCh chan []byte
	errCh  chan error

	// Request/response matching for client mode
	pendingRequests map[interface{}]chan []byte
	pendingMu       sync.RWMutex
}

// NewTransport creates a new gRPC transport.
//
// The address parameter is used differently depending on the value of isServer:
//   - For server mode (isServer=true): a local address to bind to, e.g., ":50051"
//   - For client mode (isServer=false): the server address to connect to, e.g., "localhost:50051"
//
// Optional configuration can be provided via Option functions.
func NewTransport(address string, isServer bool, options ...Option) *Transport {
	t := &Transport{
		isServer:          isServer,
		address:           address,
		maxMessageSize:    DefaultMaxMessageSize,
		connectionTimeout: DefaultConnectionTimeout,
		keepAliveTime:     DefaultKeepAliveTime,
		keepAliveTimeout:  DefaultKeepAliveTimeout,
		bufferSize:        DefaultBufferSize,
	}

	// Apply configuration options
	for _, option := range options {
		option(t)
	}

	return t
}

// Initialize initializes the transport.
//
// This method sets up internal channels and prepares the transport for operation.
// It must be called before Start() and any attempts to send or receive messages.
func (t *Transport) Initialize() error {
	t.ctx, t.cancel = context.WithCancel(context.Background())

	// Initialize channels
	t.sendCh = make(chan []byte, t.bufferSize)
	t.recvCh = make(chan []byte, t.bufferSize)
	t.errCh = make(chan error, t.bufferSize)

	// Initialize request/response matching for client mode
	if !t.isServer {
		t.pendingRequests = make(map[interface{}]chan []byte)
	}

	return nil
}

// Start starts the transport.
//
// In server mode, this starts a gRPC server that listens on the configured address.
// In client mode, this establishes a connection to the server.
//
// This method must be called after Initialize().
func (t *Transport) Start() error {
	t.runningMu.Lock()
	defer t.runningMu.Unlock()

	if t.running {
		return ErrAlreadyRunning
	}

	var err error
	if t.isServer {
		err = t.startServer()
	} else {
		err = t.startClient()
	}

	if err != nil {
		return err
	}

	t.running = true
	return nil
}

// Stop stops the transport.
//
// In server mode, this shuts down the gRPC server.
// In client mode, this closes the connection to the server.
//
// Any ongoing operations will be terminated.
func (t *Transport) Stop() error {
	t.runningMu.Lock()
	defer t.runningMu.Unlock()

	if !t.running {
		return nil
	}

	// Cancel the context to signal all goroutines to stop
	if t.cancel != nil {
		t.cancel()
	}

	// Close connections
	if t.isServer {
		if t.server != nil {
			t.server.GracefulStop()
			t.server = nil
		}
	} else {
		if t.clientConn != nil {
			err := t.clientConn.Close()
			t.clientConn = nil
			if err != nil {
				return fmt.Errorf("failed to close gRPC client connection: %w", err)
			}
		}
	}

	// Reset state
	t.running = false

	// Close channels safely using sync.Once
	t.closeOnce.Do(func() {
		close(t.sendCh)
		close(t.recvCh)
		close(t.errCh)
	})

	return nil
}

// Send sends a message through the transport.
//
// In client mode, this sends a message to the server.
// In server mode, this sends a message to the connected client.
//
// This method returns an error if the transport is not running
// or if the message cannot be sent.
func (t *Transport) Send(message []byte) error {
	t.runningMu.Lock()
	defer t.runningMu.Unlock()

	if !t.running {
		return ErrNotRunning
	}

	select {
	case t.sendCh <- message:
		return nil
	case <-t.ctx.Done():
		return fmt.Errorf("send canceled: %w", t.ctx.Err())
	}
}

// Receive receives a message from the transport.
//
// This method blocks until a message is received, an error occurs,
// or the transport is stopped.
//
// In client mode, this receives a message from the server.
// In server mode, this receives a message from a connected client.
func (t *Transport) Receive() ([]byte, error) {
	select {
	case message := <-t.recvCh:
		return message, nil
	case err := <-t.errCh:
		return nil, err
	case <-t.ctx.Done():
		return nil, fmt.Errorf("receive canceled: %w", t.ctx.Err())
	}
}

// SendWithContext sends a message and waits for the corresponding response (client mode only).
// This method implements proper request/response matching using JSON-RPC IDs.
func (t *Transport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	if t.isServer {
		// Server mode doesn't support request/response matching
		return nil, fmt.Errorf("SendWithContext not supported in server mode")
	}

	// Parse message to extract request ID
	var requestMap map[string]interface{}
	if err := json.Unmarshal(message, &requestMap); err != nil {
		return nil, fmt.Errorf("invalid JSON message: %w", err)
	}

	requestID := requestMap["id"]
	if requestID == nil {
		// This is a notification (no response expected)
		if err := t.Send(message); err != nil {
			return nil, err
		}
		return []byte{}, nil
	}

	// Create response channel for this request
	responseCh := make(chan []byte, 1)

	// Register the request
	t.pendingMu.Lock()
	t.pendingRequests[requestID] = responseCh
	t.pendingMu.Unlock()

	// Clean up when done
	defer func() {
		t.pendingMu.Lock()
		delete(t.pendingRequests, requestID)
		t.pendingMu.Unlock()
		close(responseCh)
	}()

	// Send the request
	if err := t.Send(message); err != nil {
		return nil, err
	}

	// Wait for the response with context timeout
	select {
	case response := <-responseCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.ctx.Done():
		return nil, fmt.Errorf("transport stopped: %w", t.ctx.Err())
	}
}

// routeMessage routes incoming messages to the appropriate handler (client mode only).
// For JSON-RPC responses, it routes them to pending request channels.
// For notifications, it routes them to the recvCh for the Receive() method.
func (t *Transport) routeMessage(message []byte) {
	// Parse message to check if it's a response or notification
	var messageMap map[string]interface{}
	if err := json.Unmarshal(message, &messageMap); err != nil {
		t.GetLogger().Warn("Failed to parse received message", "error", err)
		// If we can't parse it, just put it in recvCh
		select {
		case t.recvCh <- message:
		case <-t.ctx.Done():
		default:
			t.GetLogger().Warn("Client recvCh is full, dropping message")
		}
		return
	}

	// Check if this is a response (has "id" and either "result" or "error")
	if id, hasID := messageMap["id"]; hasID && (messageMap["result"] != nil || messageMap["error"] != nil) {
		// This is a response - route to pending request
		t.pendingMu.RLock()
		responseCh, exists := t.pendingRequests[id]
		t.pendingMu.RUnlock()

		if exists {
			t.GetLogger().Info("Routing response to pending request", "id", id)
			select {
			case responseCh <- message:
			case <-t.ctx.Done():
			default:
				t.GetLogger().Warn("Pending request channel is full", "id", id)
			}
		} else {
			t.GetLogger().Warn("Received response for unknown request", "id", id)
			// Put it in recvCh as fallback
			select {
			case t.recvCh <- message:
			case <-t.ctx.Done():
			default:
				t.GetLogger().Warn("Client recvCh is full, dropping message")
			}
		}
	} else {
		// This is a notification or request - put in recvCh for Receive() method
		t.GetLogger().Info("Routing notification/request to recvCh")
		select {
		case t.recvCh <- message:
		case <-t.ctx.Done():
		default:
			t.GetLogger().Warn("Client recvCh is full, dropping message")
		}
	}
}

// startServer starts the gRPC server.
func (t *Transport) startServer() error {
	// Implementation is provided in server.go
	return t.startGRPCServer()
}

// startClient is implemented in client.go

// getServerOptions returns the gRPC server options.
func (t *Transport) getServerOptions() []grpc.ServerOption {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(t.maxMessageSize),
		grpc.MaxSendMsgSize(t.maxMessageSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    t.keepAliveTime,
			Timeout: t.keepAliveTimeout,
		}),
	}

	// Add TLS if enabled
	if t.useTLS && t.tlsCertFile != "" && t.tlsKeyFile != "" {
		creds, err := t.getServerTLSCredentials()
		if err != nil {
			// Log the error but continue with insecure credentials
			t.GetLogger().Warn("Failed to load TLS credentials, continuing with insecure connection", "error", err)
		} else {
			opts = append(opts, grpc.Creds(creds))
		}
	}

	return opts
}

// getClientOptions returns the gRPC client dial options.
func (t *Transport) getClientOptions() []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(t.maxMessageSize),
			grpc.MaxCallSendMsgSize(t.maxMessageSize),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                t.keepAliveTime,
			Timeout:             t.keepAliveTimeout,
			PermitWithoutStream: true,
		}),
	}

	// Add TLS if enabled
	if t.useTLS {
		creds, err := t.getClientTLSCredentials()
		if err != nil {
			// Log the error but continue with insecure credentials
			t.GetLogger().Warn("Failed to load TLS credentials, continuing with insecure connection", "error", err)
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts
}
