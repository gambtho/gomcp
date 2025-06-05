// Package client provides the client-side implementation of the MCP protocol.
package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ServerConfig represents a complete MCP server configuration file
type ServerConfig struct {
	MCPServers map[string]ServerDefinition `json:"mcpServers"`
}

// ServerDefinition defines how to launch and connect to an MCP server
type ServerDefinition struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// MCPServer represents a running MCP server process with a connected client
type MCPServer struct {
	Name   string
	Client Client
	cmd    *exec.Cmd
}

// ServerRegistry manages a collection of MCP servers loaded from configuration
type ServerRegistry struct {
	servers map[string]*MCPServer
	logger  *slog.Logger
	mu      sync.RWMutex
	closed  bool // Track if registry has been closed
	ctx     context.Context
	cancel  context.CancelFunc
}

// ServerRegistryOption configures a ServerRegistry
type ServerRegistryOption func(*ServerRegistry)

// WithRegistryLogger sets a logger for the server registry.
// When using stdio-based MCP servers, ensure the logger does not write to stdout/stderr
// to avoid interfering with the JSON-RPC communication.
func WithRegistryLogger(logger *slog.Logger) ServerRegistryOption {
	return func(r *ServerRegistry) {
		r.logger = logger
	}
}

// NewServerRegistry creates a new empty server registry.
// By default, no logging is enabled to avoid interfering with stdio-based MCP communication.
func NewServerRegistry(opts ...ServerRegistryOption) *ServerRegistry {
	ctx, cancel := context.WithCancel(context.Background())

	r := &ServerRegistry{
		servers: make(map[string]*MCPServer),
		logger:  nil, // Default to no logging to avoid stdio interference
		closed:  false,
		ctx:     ctx,
		cancel:  cancel,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// LoadConfig loads a server configuration from a file
func (r *ServerRegistry) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return r.ApplyConfig(config)
}

// ApplyConfig applies a server configuration by starting servers and connecting clients
func (r *ServerRegistry) ApplyConfig(config ServerConfig) error {
	if len(config.MCPServers) == 0 {
		return nil
	}

	// Use goroutines to start servers concurrently
	type serverResult struct {
		name string
		err  error
	}

	resultCh := make(chan serverResult, len(config.MCPServers))

	// Start all servers concurrently
	for name, def := range config.MCPServers {
		go func(serverName string, serverDef ServerDefinition) {
			err := r.StartServer(serverName, serverDef)
			resultCh <- serverResult{name: serverName, err: err}
		}(name, def)
	}

	// Collect results and check for errors
	var errors []string
	successCount := 0
	for i := 0; i < len(config.MCPServers); i++ {
		result := <-resultCh
		if result.err != nil {
			errors = append(errors, fmt.Sprintf("server %s: %v", result.name, result.err))
		} else {
			successCount++
		}
	}

	// Return error information if any servers failed
	if len(errors) > 0 {
		return fmt.Errorf("failed to start %d/%d servers: %s", len(errors), len(config.MCPServers), strings.Join(errors, "; "))
	}

	return nil
}

// StartServer starts a server from its definition and connects a client to it
func (r *ServerRegistry) StartServer(name string, def ServerDefinition) error {
	// Check if registry is closed
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return fmt.Errorf("cannot start server %s: registry is closed", name)
	}
	r.mu.RUnlock()
	// Create command
	cmd := exec.Command(def.Command, def.Args...)

	// Set environment variables
	env := os.Environ()
	for k, v := range def.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Set up stdio pipes for communication
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Set stderr to go to the parent process stderr for debugging
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Create a transport for the client
	transport := &stdioPipeTransport{
		reader: stdoutPipe,
		writer: stdinPipe,
	}

	// Create client options - use the standard WithTransport function
	clientOpts := []Option{
		WithTransport(transport),
	}

	// Add logger if configured
	if r.logger != nil {
		serverLogger := r.logger.With("server", name)
		clientOpts = append(clientOpts, WithLogger(serverLogger))
	}

	// Create the client and connect to the server
	client, err := NewClient(name, clientOpts...)
	if err != nil {
		// Clean up the process if client creation fails
		if killErr := r.terminateProcess(cmd, name); killErr != nil {
			if r.logger != nil {
				r.logger.Error("Failed to clean up process after client creation failure",
					"server", name, "error", killErr)
			}
		}
		return fmt.Errorf("failed to create client for server %s: %w", name, err)
	}

	// Only lock for the brief moment we need to update the map
	r.mu.Lock()
	// Check if server already exists (race condition check)
	if _, exists := r.servers[name]; exists {
		r.mu.Unlock()
		// Clean up the client and process we just created
		client.Close()
		if killErr := r.terminateProcess(cmd, name); killErr != nil {
			if r.logger != nil {
				r.logger.Error("Failed to clean up duplicate server process",
					"server", name, "error", killErr)
			}
		}
		return fmt.Errorf("server %s already exists", name)
	}
	// Store the server in our registry
	r.servers[name] = &MCPServer{
		Name:   name,
		Client: client,
		cmd:    cmd,
	}
	r.mu.Unlock()

	return nil
}

// GetClient returns the client for a named server
func (r *ServerRegistry) GetClient(name string) (Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, exists := r.servers[name]
	if !exists {
		return nil, fmt.Errorf("server %s not found", name)
	}

	return server.Client, nil
}

// GetServerNames returns a list of all server names in the registry
func (r *ServerRegistry) GetServerNames() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}

	return names, nil
}

// StopServer stops a server by name
func (r *ServerRegistry) StopServer(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, exists := r.servers[name]
	if !exists {
		return fmt.Errorf("server %s not found", name)
	}

	// Close the client first to signal graceful shutdown (if client exists)
	if server.Client != nil {
		if err := server.Client.Close(); err != nil {
			if r.logger != nil {
				r.logger.Warn("Failed to close client gracefully", "server", name, "error", err)
			}
		}
	}

	// Remove from registry immediately to prevent double-cleanup
	delete(r.servers, name)

	// Gracefully terminate the process with proper timeout and escalation
	if err := r.terminateProcess(server.cmd, name); err != nil {
		if r.logger != nil {
			r.logger.Error("Failed to terminate server process", "server", name, "error", err)
		}
		return fmt.Errorf("failed to terminate server %s: %w", name, err)
	}

	return nil
}

// terminateProcess gracefully terminates a process with escalating signals and timeouts
func (r *ServerRegistry) terminateProcess(cmd *exec.Cmd, name string) error {
	if cmd == nil || cmd.Process == nil {
		return nil // Already dead or never started
	}

	// Create an independent context for termination with a reasonable timeout
	// Don't use r.ctx as it might be cancelled during Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Step 1: First close stdin to signal the process to shut down gracefully
	// This is often how MCP servers detect client disconnect
	if stdinCloser, ok := cmd.Stdin.(io.Closer); ok && stdinCloser != nil {
		if err := stdinCloser.Close(); err != nil && r.logger != nil {
			r.logger.Debug("Failed to close stdin", "server", name, "error", err)
		}
	}

	// Wait up to 3 seconds for graceful shutdown after closing stdin
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited gracefully
		if r.logger != nil {
			r.logger.Debug("Process exited gracefully after stdin close", "server", name)
		}
		return nil
	case <-time.After(3 * time.Second):
		// Graceful shutdown timeout, proceed to force kill
		if r.logger != nil {
			r.logger.Debug("Graceful shutdown timeout, force killing", "server", name)
		}
	case <-ctx.Done():
		return fmt.Errorf("termination context cancelled for process %s", name)
	}

	// Step 2: Force kill with SIGKILL
	if err := cmd.Process.Kill(); err != nil {
		// Process might already be dead
		if strings.Contains(err.Error(), "process already finished") {
			return nil
		}
		return fmt.Errorf("failed to kill process: %w", err)
	}

	// Step 3: Wait for process death with timeout
	select {
	case err := <-done:
		// Process died (ignore "signal: killed" error since we caused it)
		if err != nil && !strings.Contains(err.Error(), "signal: killed") {
			if r.logger != nil {
				r.logger.Warn("Process wait returned error", "server", name, "error", err)
			}
		}
		return nil
	case <-ctx.Done():
		// Process still not dead after timeout - this is serious
		return fmt.Errorf("process %s did not die after SIGKILL within timeout", name)
	}
}

// StopAll stops all servers
func (r *ServerRegistry) StopAll() error {
	r.mu.RLock()
	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	r.mu.RUnlock()

	var lastErr error
	for _, name := range names {
		if err := r.StopServer(name); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Close shuts down the ServerRegistry and ensures all processes are terminated.
// This should be called when the application is shutting down to prevent orphaned processes.
func (r *ServerRegistry) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil // Already closed
	}
	r.closed = true
	r.mu.Unlock()

	// Stop all servers first, then cancel the context
	if err := r.StopAll(); err != nil {
		// Still cancel the context even if StopAll fails
		r.cancel()
		return fmt.Errorf("failed to stop all servers during close: %w", err)
	}

	// Cancel the context after successful shutdown
	r.cancel()

	return nil
}

// stdioPipeTransport implements the Transport interface for stdio pipes
type stdioPipeTransport struct {
	reader         io.Reader
	writer         io.Writer
	requestTimeout time.Duration
	connectTimeout time.Duration
	notifyHandler  func(method string, params []byte)
	connected      bool
	mu             sync.RWMutex
}

func (t *stdioPipeTransport) Connect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = true
	return nil
}

func (t *stdioPipeTransport) ConnectWithContext(ctx context.Context) error {
	return t.Connect()
}

func (t *stdioPipeTransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false
	return nil
}

func (t *stdioPipeTransport) Send(message []byte) ([]byte, error) {
	return t.SendWithContext(context.Background(), message)
}

func (t *stdioPipeTransport) SendWithContext(ctx context.Context, message []byte) ([]byte, error) {
	t.mu.RLock()
	connected := t.connected
	t.mu.RUnlock()

	if !connected {
		return nil, errors.New("transport not connected")
	}

	// Write message to the writer
	if _, err := t.writer.Write(append(message, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	// Create a channel for the response
	responseCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	// Read response in a goroutine
	go func() {
		scanner := bufio.NewScanner(t.reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max size

		if scanner.Scan() {
			responseCh <- scanner.Bytes()
		} else if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("error reading response: %w", err)
		} else {
			errCh <- io.EOF
		}
	}()

	// Wait for response or context cancellation
	select {
	case response := <-responseCh:
		return response, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *stdioPipeTransport) SetRequestTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requestTimeout = timeout
}

func (t *stdioPipeTransport) SetConnectionTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connectTimeout = timeout
}

func (t *stdioPipeTransport) RegisterNotificationHandler(handler func(method string, params []byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.notifyHandler = handler
}

// Root functions to provide a cleaner API following the PRD guidance
// These will be added to the root gomcp.go file

// LoggerOption configures a logger
type LoggerOption func(*loggerConfig)

type loggerConfig struct {
	level  slog.Level
	output io.Writer
}

// WithLogLevel sets the log level
func WithLogLevel(level slog.Level) LoggerOption {
	return func(c *loggerConfig) {
		c.level = level
	}
}

// WithLogOutput sets the output writer
func WithLogOutput(w io.Writer) LoggerOption {
	return func(c *loggerConfig) {
		c.output = w
	}
}

// WithLogFile sets the output to a file (safe for stdio-based MCP communication)
func WithLogFile(filename string) LoggerOption {
	return func(c *loggerConfig) {
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// If file opening fails, fall back to discard to avoid panics
			c.output = io.Discard
			return
		}
		c.output = file
	}
}

// WithLogDiscard disables all logging output
func WithLogDiscard() LoggerOption {
	return func(c *loggerConfig) {
		c.output = io.Discard
	}
}

// NewLogger creates a logger with the specified options.
// By default, creates a logger that writes to stderr with Info level.
func NewLogger(opts ...LoggerOption) *slog.Logger {
	config := &loggerConfig{
		level:  slog.LevelInfo,
		output: os.Stderr,
	}

	for _, opt := range opts {
		opt(config)
	}

	return slog.New(slog.NewTextHandler(config.output, &slog.HandlerOptions{
		Level: config.level,
	}))
}
