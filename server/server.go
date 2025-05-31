// Package server provides the server-side implementation of the MCP protocol.
// It offers a comprehensive API for building and running MCP servers that can
// register tools, resources, and prompt templates for client interaction.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/transport"
	"github.com/localrivet/gomcp/transport/mqtt"
	"github.com/localrivet/gomcp/transport/nats"
	"github.com/localrivet/gomcp/transport/sse"
	"github.com/localrivet/gomcp/transport/stdio"
	"github.com/localrivet/gomcp/transport/udp"
	"github.com/localrivet/gomcp/transport/unix"
)

// Server represents an MCP server with fluent configuration methods.
// It provides a builder-style API for configuring all aspects of an MCP server
// including tools, resources, prompts, and transport options.
type Server interface {
	// Run starts the server and blocks until it exits.
	//
	// This method initializes the server, starts listening for connections,
	// and processes incoming requests. It blocks until the server encounters
	// an error or is explicitly stopped.
	//
	// Example:
	//  if err := server.Run(); err != nil {
	//      log.Fatalf("Server error: %v", err)
	//  }
	Run() error

	// Shutdown gracefully shuts down the server.
	//
	// This method stops accepting new connections and gracefully terminates
	// existing connections. It returns any error encountered during shutdown.
	//
	// Example:
	//  if err := server.Shutdown(); err != nil {
	//      log.Printf("Server shutdown error: %v", err)
	//  }
	Shutdown() error

	// Tool registers a tool with the server.
	//
	// The name parameter is the unique identifier for the tool. The description
	// parameter provides human-readable documentation. The handler parameter must
	// be a function with the signature:
	//   func(ctx *Context, args *StructType) (interface{}, error)
	//
	// Where StructType is a pointer to a struct and can be nil. The schema is automatically
	// extracted from the struct type using reflection and JSON tags.
	//
	// Optional annotations can be provided as additional map parameters to add
	// metadata that can be used by clients.
	//
	// Example:
	//  server.Tool("calculator", "Perform calculations", func(ctx *Context, args struct{
	//		Operation string `json:"operation"`
	//		A float64 `json:"a"`
	//		B float64 `json:"b"`
	//	}) (interface{}, error) {
	//      // Implementation here - args.Operation, args.A, args.B are fully typed
	//      return result, nil
	//  })
	Tool(name, description string, handler interface{}, annotations ...map[string]interface{}) Server

	// Resource registers a resource with the server.
	//
	// The pattern parameter is a URL path pattern that matches requests to this
	// resource. The description parameter provides human-readable documentation.
	// The handler parameter must be a function with signature:
	//   func(ctx *Context, args *StructType) (interface{}, error)
	//
	// Where StructType is a pointer to a struct and can be nil. The schema is automatically
	// extracted from the struct type using reflection and JSON tags.
	//
	// Path parameters are extracted from URI templates (e.g., /users/{id}) and
	// JSON parameters come from request body. Use struct tags to map them:
	//   - `path:"name"` for URI template parameters
	//   - `json:"name"` for JSON body parameters
	//
	// Example:
	//  server.Resource("/users/{id}", "Update user name", func(ctx *Context, args struct{
	//		ID   string `path:"id"`
	//		Name string `json:"name"`
	//	}) (interface{}, error) {
	//      // args.ID contains the path parameter from /users/{id}
	//      // args.Name contains the JSON body parameter
	//      user, err := getUserById(args.ID)
	//      if err != nil {
	//          return nil, err
	//      }
	//      user.Name = args.Name
	//      user, err = updateUser(user)
	//      if err != nil {
	//          return nil, err
	//      }
	//      return user, nil
	//  })
	Resource(path, description string, handler interface{}) Server

	// Prompt registers a prompt template with the server.
	//
	// The name parameter is the unique identifier for the prompt. The description
	// parameter provides human-readable documentation. The templates parameter
	// contains one or more prompt templates created using User() or Assistant().
	//
	// At least one template must be provided. Use server.User() and server.Assistant()
	// helper functions to create templates with the appropriate roles.
	//
	// Example:
	//  server.Prompt("greeting", "A friendly greeting",
	//      server.User("Hello, {{name}}! How are you today?"))
	//
	//  server.Prompt("conversation", "A multi-turn conversation",
	//      server.User("Please help me with {{task}}."),
	//      server.Assistant("I'll be happy to help you with that."))
	Prompt(name, description string, templates ...PromptTemplate) Server

	// Root sets the allowed root paths.
	//
	// Root paths are the entry points for resource navigation. At least one
	// root path must be defined for resources to be accessible.
	//
	// Example:
	//  server.Root("/api/v1", "/api/v2")
	Root(paths ...string) Server

	// IsPathInRoots checks if the given path is within any of the registered roots.
	// This security method ensures that file operations can only access paths within
	// the authorized boundaries defined by the registered root paths, preventing
	// directory traversal attacks and unauthorized file system access.
	//
	// Parameters:
	IsPathInRoots(path string) bool

	// Logger returns the server's logger.
	//
	// This method provides access to the server's configured logger for custom logging needs.
	// It can be used to log additional information or to reconfigure logging at runtime.
	//
	// Example:
	//
	//	// Log a custom message with the server's logger
	//	server.Logger().Info("custom event occurred",
	//	    "correlation_id", correlationID,
	//	    "user_id", userID,
	//	)
	Logger() *slog.Logger

	// Events returns the server's event system.
	//
	// This method provides access to the server's event system for subscribing to
	// server lifecycle events. External consumers can hook into events like server
	// initialization, client connections, tool executions, and more.
	//
	// Example:
	//
	//	// Subscribe to server initialization events
	//	events.Subscribe[MyServerEvent](server.Events(), events.TopicServerInitialized,
	//	    func(ctx context.Context, evt MyServerEvent) error {
	//	        log.Printf("Server %s initialized with %d tools", evt.Name, evt.ToolCount)
	//	        return nil
	//	    })
	//
	//	// Subscribe to client connection events
	//	events.Subscribe[MyClientEvent](server.Events(), events.TopicClientConnected,
	//	    func(ctx context.Context, evt MyClientEvent, conn net.Conn) error {
	//	        log.Printf("Client connected from %s", conn.RemoteAddr())
	//	        return nil
	//	    })
	Events() *events.Subject

	// ListTools returns a list of all registered tools.
	//
	// This method provides programmatic access to the server's tool registry,
	// returning the same information that would be provided via the tools/list
	// MCP endpoint but in a convenient Go slice format. This is useful for
	// debugging, monitoring, and multi-proxy aggregation scenarios.
	//
	// Example:
	//  tools, err := server.ListTools()
	//  if err != nil {
	//      log.Printf("Failed to list tools: %v", err)
	//      return
	//  }
	//  for _, tool := range tools {
	//      fmt.Printf("Tool: %s - %s\n", tool.Name, tool.Description)
	//  }
	ListTools() ([]mcp.Tool, error)

	// ListResources returns a list of all registered resources.
	//
	// This method provides programmatic access to the server's resource registry,
	// returning the same information that would be provided via the resources/list
	// MCP endpoint but in a convenient Go slice format. This is useful for
	// debugging, monitoring, and multi-proxy aggregation scenarios.
	//
	// Example:
	//  resources, err := server.ListResources()
	//  if err != nil {
	//      log.Printf("Failed to list resources: %v", err)
	//      return
	//  }
	//  for _, resource := range resources {
	//      fmt.Printf("Resource: %s - %s\n", resource.URI, resource.Description)
	//  }
	ListResources() ([]mcp.Resource, error)

	// ListPrompts returns a list of all registered prompts.
	//
	// This method provides programmatic access to the server's prompt registry,
	// returning the same information that would be provided via the prompts/list
	// MCP endpoint but in a convenient Go slice format. This is useful for
	// debugging, monitoring, and multi-proxy aggregation scenarios.
	//
	// Example:
	//  prompts, err := server.ListPrompts()
	//  if err != nil {
	//      log.Printf("Failed to list prompts: %v", err)
	//      return
	//  }
	//  for _, prompt := range prompts {
	//      fmt.Printf("Prompt: %s - %s\n", prompt.Name, prompt.Description)
	//  }
	ListPrompts() ([]mcp.Prompt, error)

	// AsHTTP configures the server to use HTTP for communication.
	//
	// The address parameter specifies the host and port to listen on.
	//
	// Example:
	//  server.AsHTTP("localhost:8080")
	AsHTTP(address string) Server

	// AsWebsocket configures the server to use WebSocket for communication.
	//
	// The address parameter specifies the host and port to listen on.
	//
	// Example:
	//  server.AsWebsocket("localhost:8080")
	AsWebsocket(address string) Server

	// AsSSE configures the server to use Server-Sent Events for communication.
	//
	// The address parameter specifies the host and port to listen on.
	// Optional SSE configuration options can be provided using sse.SSE.With* functions.
	//
	// Example:
	//  // Basic configuration
	//  server.AsSSE("localhost:8080")
	//
	//  // With custom paths
	//  server.AsSSE("localhost:8080", sse.SSE.WithPathPrefix("/api"), sse.SSE.WithEventsPath("/events"))
	AsSSE(address string, options ...sse.Option) Server

	// AsUnixSocket configures the server to use Unix Domain Sockets for communication.
	//
	// Unix Domain Sockets provide high-performance inter-process communication for
	// processes running on the same machine.
	//
	// Example:
	//
	//	server.AsUnixSocket("/tmp/mcp.sock")
	//	// With options:
	//	server.AsUnixSocket("/tmp/mcp.sock", unix.WithPermissions(0600))
	AsUnixSocket(socketPath string, options ...unix.UnixSocketOption) Server

	// AsUDP configures the server to use UDP for communication.
	//
	// UDP provides low-latency communication with minimal overhead,
	// suitable for high-throughput scenarios where occasional packet
	// loss is acceptable.
	//
	// Example:
	//
	//	server.AsUDP(":8080")
	//	// With options:
	//	server.AsUDP(":8080", udp.WithMaxPacketSize(2048))
	AsUDP(address string, options ...udp.UDPOption) Server

	// AsMQTT configures the server to use MQTT for communication
	// with optional configuration options.
	//
	// MQTT provides a publish/subscribe-based communication model,
	// suitable for IoT applications and distributed systems with
	// potentially intermittent connectivity.
	//
	// Example:
	//
	//	server.AsMQTT("tcp://broker.example.com:1883")
	//	// With options:
	//	server.AsMQTT("tcp://broker.example.com:1883",
	//	    mqtt.WithQoS(1),
	//	    mqtt.WithCredentials("username", "password"),
	//	    mqtt.WithTopicPrefix("custom/topic/prefix"))
	AsMQTT(brokerURL string, options ...mqtt.MQTTOption) Server

	// AsStdio configures the server to use Standard I/O for communication.
	//
	// This is useful for child processes or integration with other MCP systems.
	// An optional logFile parameter can be provided to redirect stdio logs.
	//
	// Example:
	//  server.AsStdio("./mcp-server.log")
	AsStdio(logFile ...string) Server

	// AsNATS configures the server to use NATS for communication
	// with optional configuration options.
	//
	// NATS provides a high-performance, cloud native communication system,
	// suitable for microservices architectures, IoT messaging, and
	// event-driven applications.
	//
	// Example:
	//
	//	server.AsNATS("nats://localhost:4222")
	//	// With options:
	//	server.AsNATS("nats://localhost:4222",
	//	    nats.WithCredentials("username", "password"),
	//	    nats.WithSubjectPrefix("custom/subject/prefix"))
	AsNATS(serverURL string, options ...nats.NATSOption) Server

	// GetServer returns the underlying server implementation
	// This is primarily for internal use and testing.
	GetServer() *serverImpl
}

// Option represents a server configuration option.
// Server options are used to customize the behavior and configuration of a server instance
// when it is created with NewServer.
type Option func(*serverImpl)

// serverImpl is the concrete implementation of the Server interface.
type serverImpl struct {
	// name is the unique identifier for this server instance, used in logs and server info.
	name string

	// tools is a map of registered tool handlers keyed by tool name.
	tools map[string]*Tool

	// resources is a map of registered resource handlers keyed by path pattern.
	resources map[string]*Resource

	// prompts is a map of registered prompt templates keyed by prompt name.
	prompts map[string]*Prompt

	// roots is a slice of registered root paths for resource navigation.
	roots []string

	// transport is the communication transport used by this server (stdio, websocket, etc.).
	transport transport.Transport

	// logger is the structured logger used for server logs.
	logger *slog.Logger

	// versionDetector handles MCP protocol version detection and negotiation.
	versionDetector *mcp.VersionDetector

	// mu protects concurrent access to server state.
	mu sync.RWMutex

	// protocolVersion is the negotiated MCP protocol version for this server.
	protocolVersion string

	// requestTracker manages pending requests and matches responses to requests.
	requestTracker *requestTracker

	// requestCanceller manages cancellable requests and processes cancellation notifications.
	requestCanceller *RequestCanceller

	// progressTokenManager manages progress tokens for long-running operations.
	progressTokenManager *mcp.ProgressTokenManager

	// progressNotificationHandler manages progress notifications and bidirectional communication.
	progressNotificationHandler *ProgressNotificationHandler

	// sessionManager handles client session creation, retrieval, and management.
	sessionManager *SessionManager

	// defaultSession is a session used for simple implementations that don't track
	// multiple client sessions explicitly.
	defaultSession *ClientSession

	// lastRequestID tracks the last used request ID for generating unique request IDs.
	// This is used in the sampling.go file to generate sequential request identifiers
	// for JSON-RPC requests, particularly for sampling operations.
	lastRequestID int64

	// Sampling configuration and controller
	// samplingConfig defines the parameters for sampling behavior (rate limits, caching, etc.).
	samplingConfig *SamplingConfig

	// samplingController manages sampling requests and applies sampling configuration.
	samplingController *SamplingController

	// initialized indicates whether the client has sent the initialized notification
	// Only after receiving this notification should the server send feature-specific notifications
	initialized bool

	// pendingNotifications stores notifications that should be sent after initialization
	pendingNotifications [][]byte

	// toolsChanged indicates if tools have been modified since the last notification
	toolsChanged bool

	// events provides the event system for server lifecycle events
	events *events.Subject
}

// GetName returns the server's name.
//
// The server name is set during initialization and is typically used
// in logging and protocol messages.
func (s *serverImpl) GetName() string {
	return s.name
}

// GetTools returns a map of all registered tools.
//
// The map keys are tool names, and the values are the corresponding Tool objects
// containing metadata and handler functions.
func (s *serverImpl) GetTools() map[string]*Tool {
	return s.tools
}

// GetResources returns a map of all registered resources.
//
// The map keys are resource path patterns, and the values are the corresponding
// Resource objects containing metadata and handler functions.
func (s *serverImpl) GetResources() map[string]*Resource {
	return s.resources
}

// GetPrompts returns a map of all registered prompts.
//
// The map keys are prompt names, and the values are the corresponding Prompt
// objects containing metadata and template functions.
func (s *serverImpl) GetPrompts() map[string]*Prompt {
	return s.prompts
}

// GetTransport returns the server's configured transport.
//
// The transport is responsible for communication between the server and clients
// (e.g., stdio, WebSocket, HTTP).
func (s *serverImpl) GetTransport() transport.Transport {
	return s.transport
}

// WithSamplingConfig sets the sampling configuration for the server.
//
// This method configures how the server handles sampling requests, including
// rate limits, caching behavior, and other performance parameters.
//
// Example:
//
//	config := server.NewSamplingConfig().
//	    WithRateLimit(10).
//	    WithCacheSize(100)
//	server.WithSamplingConfig(config)
func (s *serverImpl) WithSamplingConfig(config *SamplingConfig) Server {
	s.samplingConfig = config
	return s
}

// WithSamplingController sets a custom sampling controller for the server.
//
// This is an advanced method for applications that need fine-grained control
// over sampling behavior beyond what is provided by the standard SamplingConfig.
func (s *serverImpl) WithSamplingController(controller *SamplingController) Server {
	s.samplingController = controller
	return s
}

// NewServer creates a new MCP server with the given name and options.
//
// The server is initialized with default settings that can be customized using
// the provided options. By default, the server uses stdio transport and default
// logging configuration.
//
// The name parameter is a human-readable identifier for the server, used in logs
// and server information.
//
// Example:
//
//	// Create a basic server with default settings
//	server := server.NewServer("my-service")
//
//	// Create a server with custom logger and sampling configuration
//	customLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	samplingConfig := server.NewSamplingConfig().WithRateLimit(100)
//
//	server := server.NewServer("my-service",
//	    server.WithLogger(customLogger),
//	    server.WithSamplingConfig(samplingConfig),
//	)
func NewServer(name string, options ...Option) Server {
	// Create a new server instance
	s := &serverImpl{
		name:                 name,
		tools:                make(map[string]*Tool),
		resources:            make(map[string]*Resource),
		prompts:              make(map[string]*Prompt),
		roots:                []string{},
		logger:               slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		versionDetector:      mcp.NewVersionDetector(),
		sessionManager:       NewSessionManager(),
		initialized:          false,
		pendingNotifications: [][]byte{},
		toolsChanged:         false,
		requestCanceller:     NewRequestCanceller(),
		progressTokenManager: mcp.NewProgressTokenManager(),
	}

	// Initialize progress notification handler
	s.progressNotificationHandler = NewProgressNotificationHandler(s)

	// Set the default transport to stdio
	s.transport = stdio.NewTransport()

	// Create a default session for simple implementations
	defaultClientInfo := ClientInfo{
		SamplingSupported: true,
		SamplingCaps: SamplingCapabilities{
			Supported:    true,
			TextSupport:  true,
			ImageSupport: true,
			AudioSupport: false,
		},
		ProtocolVersion: "draft",
	}
	s.defaultSession = s.sessionManager.CreateSession(defaultClientInfo, "draft")

	// Initialize sampling configuration with defaults
	s.samplingConfig = NewDefaultSamplingConfig()
	s.samplingController = NewSamplingController(s.samplingConfig, s.logger)

	// Apply all options first to get the final logger
	for _, option := range options {
		option(s)
	}

	// Initialize events system with the server's logger
	s.events = events.NewSubject(
		events.WithLogger(s.logger),
		events.WithBufferSize(1024),
		events.WithReplay(100),
	)

	return s
}

// WithLogger sets the server's logger.
//
// This option configures the structured logger used by the server for logging events,
// errors, and debug information.
//
// Example:
//
//	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
//	    Level: slog.LevelDebug,
//	})
//	logger := slog.New(jsonHandler)
//
//	server := server.NewServer("my-service",
//	    server.WithLogger(logger),
//	)
func WithLogger(logger *slog.Logger) Option {
	return func(s *serverImpl) {
		s.logger = logger
	}
}

// WithProtocolVersion sets a specific protocol version for the server to use.
// This bypasses the normal negotiation process and forces the server to use this version.
// This is useful for testing or when you need to enforce a specific protocol version.
//
// Example:
//
//	server := server.NewServer("my-service",
//	    server.WithProtocolVersion("2025-03-26"),
//	)
func WithProtocolVersion(version string) Option {
	return func(s *serverImpl) {
		s.protocolVersion = version
		// Update the default session to use this protocol version
		if s.defaultSession != nil {
			s.defaultSession.ClientInfo.ProtocolVersion = version
		}
	}
}

// Logger returns the server's logger.
//
// This method provides access to the server's configured logger for custom logging needs.
// It can be used to log additional information or to reconfigure logging at runtime.
//
// Example:
//
//	// Log a custom message with the server's logger
//	server.Logger().Info("custom event occurred",
//	    "correlation_id", correlationID,
//	    "user_id", userID,
//	)
func (s *serverImpl) Logger() *slog.Logger {
	return s.logger
}

// Events returns the server's event system.
func (s *serverImpl) Events() *events.Subject {
	return s.events
}

// ProcessInitialize processes an initialize request.
//
// This method handles the initial handshake between client and server, including
// protocol version negotiation, capability exchange, and session creation.
//
// The ctx parameter contains the client's initialization request. The method returns
// a response containing the negotiated protocol version and server capabilities.
func (s *serverImpl) ProcessInitialize(ctx *Context) (interface{}, error) {
	// Extract the client's requested protocol version
	clientProtocolVersion, err := ExtractProtocolVersion(ctx.Request.Params)
	if err != nil {
		return nil, err
	}

	// Validate and potentially normalize the protocol version
	protocolVersion, err := s.ValidateProtocolVersion(clientProtocolVersion)
	if err != nil {
		return nil, err
	}

	// Store the validated protocol version without locking
	s.protocolVersion = protocolVersion

	// Update the transport with the negotiated protocol version
	s.mu.RLock()
	if s.transport != nil {
		s.transport.SetProtocolVersion(protocolVersion)
	}
	s.mu.RUnlock()

	// Determine sampling capabilities based on protocol version
	samplingCaps := DetectClientCapabilities(protocolVersion)

	// Update or create client info
	clientInfo := ClientInfo{
		SamplingSupported: samplingCaps.Supported,
		SamplingCaps:      samplingCaps,
		ProtocolVersion:   protocolVersion,
	}

	// Create a new session for this client
	session := s.sessionManager.CreateSession(clientInfo, protocolVersion)

	// Store the session ID in the context metadata
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]interface{})
	}
	ctx.Metadata["sessionID"] = string(session.ID)

	// For simple implementations that don't track multiple sessions, update the default session without locking
	s.defaultSession = session

	// Log the session creation
	s.logger.Info("client connected",
		"sessionID", string(session.ID),
		"protocolVersion", protocolVersion,
		"samplingSupported", samplingCaps.Supported,
		"audioSupport", samplingCaps.AudioSupport)

	// Build server capabilities according to MCP specification
	// Only declare capability flags, not actual data
	capabilities := map[string]interface{}{
		"logging": map[string]interface{}{},
	}

	// Add prompts capability if we have any registered
	if len(s.prompts) > 0 {
		capabilities["prompts"] = map[string]interface{}{
			"listChanged": true,
		}
	}

	// Add resources capability if we have any registered
	if len(s.resources) > 0 {
		capabilities["resources"] = map[string]interface{}{
			"subscribe":   true,
			"listChanged": true,
		}
	}

	// Add tools capability if we have any registered
	if len(s.tools) > 0 {
		capabilities["tools"] = map[string]interface{}{
			"listChanged": true,
		}
	}

	// Emit client connected event
	go func() {
		events.Publish[events.ClientConnectedEvent](s.events, events.TopicClientConnected, events.ClientConnectedEvent{
			SessionID:       string(session.ID),
			ProtocolVersion: session.ProtocolVersion,
			ConnectedAt:     session.Created,
			ClientInfo: events.ClientInfo{
				Name:    "Unknown Client",
				Version: "Unknown",
			},
			Capabilities: capabilities,
		})
	}()

	// Build the response according to MCP specification
	response := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities":    capabilities,
		"serverInfo": map[string]interface{}{
			"name":    s.name,
			"version": "1.0.0",
		},
	}

	// Add optional instructions field if needed (available in 2025-03-26 and draft)
	if protocolVersion == "2025-03-26" || protocolVersion == "draft" {
		// Could add instructions here if needed
		// response["instructions"] = "Optional instructions for the client"
	}

	return response, nil
}

// ProcessShutdown processes a shutdown request.
//
// This method handles graceful shutdown requests from clients. It returns a success
// response to the client and initiates server shutdown.
//
// The ctx parameter contains the shutdown request. The method returns a simple
// response indicating whether the shutdown was initiated successfully.
func (s *serverImpl) ProcessShutdown(ctx *Context) (interface{}, error) {
	// Emit server shutdown event
	go func() {
		events.Publish[events.ServerShutdownEvent](s.events, events.TopicServerShutdown, events.ServerShutdownEvent{
			ServerName:   s.name,
			ShutdownAt:   time.Now(),
			GracefulExit: true,
			Reason:       "client_requested",
		})
	}()

	// TODO: Implement proper shutdown handling
	go func() {
		s.logger.Info("shutdown requested, will exit soon")

		// Clean up events system
		if s.events != nil {
			events.Complete(s.events)
		}

		// Give time for the response to be sent before actually shutting down
		time.Sleep(100 * time.Millisecond)
		// TODO: Implement clean shutdown
	}()
	return map[string]interface{}{"success": true}, nil
}

// Run starts the server and blocks until it exits.
//
// This method initializes the server's transport, sets up message handling,
// and begins processing client requests. It blocks until an error occurs or
// the server is explicitly stopped.
//
// Run returns an error if the server fails to start or encounters a fatal error
// during operation. Common error scenarios include transport initialization failure
// or missing transport configuration.
//
// Example:
//
//	server := server.NewServer("my-service").AsStdio()
//
//	// Add tools, resources, etc.
//	server.Tool("add", "Add two numbers", addHandler)
//
//	// Start the server (this will block until exit)
//	if err := server.Run(); err != nil {
//	    log.Fatalf("Server error: %v", err)
//	}
func (s *serverImpl) Run() error {
	s.mu.RLock()
	t := s.transport
	s.mu.RUnlock()

	if t == nil {
		return fmt.Errorf("no transport configured, use AsStdio(), AsWebsocket(), AsSSE(), or AsHTTP()")
	}

	// Initialize the request tracker
	s.mu.Lock()
	s.requestTracker = newRequestTracker()
	s.mu.Unlock()

	// Set up transport debug logging
	t.SetDebugHandler(func(message string) {
		s.logger.Debug("transport", "message", message)
	})

	// Set the message handler using the non-exported handleMessage method
	t.SetMessageHandler(s.handleMessage)

	// Initialize the transport
	if err := t.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize transport: %w", err)
	}

	// Start the transport
	if err := t.Start(); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	s.logger.Info("server started", "name", s.name, "transport", fmt.Sprintf("%T", t))

	// Block until the transport is done
	// TODO: Implement proper shutdown handling
	select {}
}

// Shutdown gracefully shuts down the server
func (s *serverImpl) Shutdown() error {
	s.logger.Info("shutting down server", "name", s.name)

	// Stop the underlying transport
	if s.transport != nil {
		if err := s.transport.Stop(); err != nil {
			s.logger.Error("error stopping transport", "error", err)
			return err
		}
	}

	// Clean up events system
	if s.events != nil {
		events.Complete(s.events)
	}

	s.logger.Info("server shutdown complete", "name", s.name)
	return nil
}

// GetServer returns the underlying server implementation
// This is primarily for internal use and testing.
func (s *serverImpl) GetServer() *serverImpl {
	return s
}

// sendNotification sends a notification message to the client.
//
// Notifications are one-way messages from the server to the client that do not
// require a response. They are used for events like resource changes, tool list
// updates, and other asynchronous events.
//
// The method parameter specifies the notification type (e.g., "notifications/tools/list_changed").
// The params parameter contains any additional data to include with the notification.
//
// If the notification cannot be sent, an error is logged but not returned to the caller.
func (s *serverImpl) sendNotification(method string, params interface{}) {
	if s.transport == nil {
		return
	}

	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}

	if params != nil {
		notification["params"] = params
	}

	// Convert to JSON
	message, err := json.Marshal(notification)
	if err != nil {
		s.logger.Error("failed to marshal notification", "error", err)
		return
	}

	// Send the notification
	if err := s.transport.Send(message); err != nil {
		s.logger.Error("failed to send notification", "error", err)
	}
}

// handleInitializedNotification processes the initialized notification from the client
// and sends any pending notifications that were queued during the initialization phase.
func (s *serverImpl) handleInitializedNotification() {
	s.mu.Lock()
	s.initialized = true

	// Process any pending notifications
	pendingNotifications := s.pendingNotifications
	s.pendingNotifications = nil

	// Reset the toolsChanged flag
	s.toolsChanged = false
	s.mu.Unlock()

	s.logger.Debug("client initialized, processing pending notifications",
		"count", len(pendingNotifications))

	// Publish server initialized event
	evt := events.ServerInitializedEvent{
		ServerName:      s.name,
		ProtocolVersion: s.protocolVersion,
		ToolCount:       len(s.tools),
		ResourceCount:   len(s.resources),
		PromptCount:     len(s.prompts),
		InitializedAt:   time.Now(),
		Metadata:        make(map[string]any),
	}

	// Publish the event (ignore errors to avoid breaking server startup)
	if err := events.Publish[events.ServerInitializedEvent](s.events, events.TopicServerInitialized, evt); err != nil {
		s.logger.Debug("failed to publish server initialized event", "error", err)
	}

	// Send any pending notifications
	for _, notification := range pendingNotifications {
		if s.transport != nil {
			if err := s.transport.Send(notification); err != nil {
				s.logger.Error("failed to send pending notification after initialization", "error", err)
			}
		}
	}

	// Always send tools/list_changed notification after initialization
	// This ensures the client is aware of available tools even if no tools were
	// added since the server started
	go func() {
		// Add slight delay to ensure client is ready to receive the notification
		time.Sleep(50 * time.Millisecond)
		if err := s.SendToolsListChangedNotification(); err != nil {
			s.logger.Error("failed to send tools list changed notification after initialization", "error", err)
		} else {
			s.logger.Debug("sent tools/list_changed notification after client initialization")
		}
	}()
}

// ListTools returns a list of all registered tools.
//
// This method provides programmatic access to the server's tool registry,
// internally calling ProcessToolList and converting the response to the shared
// mcp.Tool format for consistency with the client interface.
func (s *serverImpl) ListTools() ([]mcp.Tool, error) {
	// Create a mock context for the ProcessToolList call
	ctx := &Context{
		Request: &Request{},
	}

	// Call the existing ProcessToolList method
	result, err := s.ProcessToolList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to process tool list: %w", err)
	}

	// Convert the result to the expected format
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result format from ProcessToolList")
	}

	toolsInterface, ok := resultMap["tools"]
	if !ok {
		return nil, fmt.Errorf("tools field not found in ProcessToolList result")
	}

	toolsSlice, ok := toolsInterface.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("tools field has unexpected format")
	}

	// Convert to mcp.Tool slice
	tools := make([]mcp.Tool, 0, len(toolsSlice))
	for _, toolMap := range toolsSlice {
		tool := mcp.Tool{
			Name:        getString(toolMap, "name"),
			Description: getString(toolMap, "description"),
			InputSchema: getMap(toolMap, "inputSchema"),
			Annotations: getMap(toolMap, "annotations"),
		}

		// Handle OutputSchema if present (for draft spec compatibility)
		if outputSchema, exists := toolMap["outputSchema"]; exists {
			if outputSchemaMap, ok := outputSchema.(map[string]interface{}); ok {
				tool.OutputSchema = outputSchemaMap
			}
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

// ListResources returns a list of all registered resources.
//
// This method provides programmatic access to the server's resource registry,
// internally calling ProcessResourceList and converting the response to the shared
// mcp.Resource format for consistency with the client interface.
func (s *serverImpl) ListResources() ([]mcp.Resource, error) {
	// Create a mock context for the ProcessResourceList call
	ctx := &Context{
		Request: &Request{},
	}

	// Call the existing ProcessResourceList method
	result, err := s.ProcessResourceList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to process resource list: %w", err)
	}

	// Convert the result to the expected format
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result format from ProcessResourceList")
	}

	resourcesInterface, ok := resultMap["resources"]
	if !ok {
		return nil, fmt.Errorf("resources field not found in ProcessResourceList result")
	}

	resourcesSlice, ok := resourcesInterface.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("resources field has unexpected format")
	}

	// Convert to mcp.Resource slice
	resources := make([]mcp.Resource, 0, len(resourcesSlice))
	for _, resourceMap := range resourcesSlice {
		resource := mcp.Resource{
			URI:         getString(resourceMap, "uri"),
			Name:        getString(resourceMap, "name"),
			Description: getString(resourceMap, "description"),
			MimeType:    getString(resourceMap, "mimeType"),
			Annotations: getMap(resourceMap, "annotations"),
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// ListPrompts returns a list of all registered prompts.
//
// This method provides programmatic access to the server's prompt registry,
// internally calling ProcessPromptList and converting the response to the shared
// mcp.Prompt format for consistency with the client interface.
func (s *serverImpl) ListPrompts() ([]mcp.Prompt, error) {
	// Create a mock context for the ProcessPromptList call
	ctx := &Context{
		Request: &Request{},
	}

	// Call the existing ProcessPromptList method
	result, err := s.ProcessPromptList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to process prompt list: %w", err)
	}

	// Convert the result to the expected format
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result format from ProcessPromptList")
	}

	promptsInterface, ok := resultMap["prompts"]
	if !ok {
		return nil, fmt.Errorf("prompts field not found in ProcessPromptList result")
	}

	promptsSlice, ok := promptsInterface.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("prompts field has unexpected format")
	}

	// Convert to mcp.Prompt slice
	prompts := make([]mcp.Prompt, 0, len(promptsSlice))
	for _, promptMap := range promptsSlice {
		prompt := mcp.Prompt{
			Name:        getString(promptMap, "name"),
			Description: getString(promptMap, "description"),
			Annotations: getMap(promptMap, "annotations"),
		}

		// Handle arguments if present
		if argsInterface, exists := promptMap["arguments"]; exists {
			if argsSlice, ok := argsInterface.([]interface{}); ok {
				arguments := make([]mcp.PromptArgument, 0, len(argsSlice))
				for _, argInterface := range argsSlice {
					if argMap, ok := argInterface.(map[string]interface{}); ok {
						arg := mcp.PromptArgument{
							Name:        getString(argMap, "name"),
							Description: getString(argMap, "description"),
							Required:    getBool(argMap, "required"),
						}
						arguments = append(arguments, arg)
					}
				}
				prompt.Arguments = arguments
			}
		}

		prompts = append(prompts, prompt)
	}

	return prompts, nil
}

// Helper functions for type conversion
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key]; ok {
		if mapVal, ok := val.(map[string]interface{}); ok {
			return mapVal
		}
	}
	return nil
}

func getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}
