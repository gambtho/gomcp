package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/server"
)

// Define our event types to match what the server emits
type ServerInitializedEvent struct {
	Name            string         `json:"name"`
	ProtocolVersion string         `json:"protocolVersion"`
	ToolCount       int            `json:"toolCount"`
	ResourceCount   int            `json:"resourceCount"`
	PromptCount     int            `json:"promptCount"`
	InitializedAt   time.Time      `json:"initializedAt"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type ServerShutdownEvent struct {
	Name       string         `json:"name"`
	ShutdownAt time.Time      `json:"shutdownAt"`
	Reason     string         `json:"reason"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ClientConnectedEvent struct {
	SessionID       string         `json:"sessionId"`
	ProtocolVersion string         `json:"protocolVersion"`
	ConnectedAt     time.Time      `json:"connectedAt"`
	Capabilities    map[string]any `json:"capabilities"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type ClientDisconnectedEvent struct {
	SessionID      string         `json:"sessionId"`
	DisconnectedAt time.Time      `json:"disconnectedAt"`
	Reason         string         `json:"reason"`
	Duration       time.Duration  `json:"duration"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type ToolRegisteredEvent struct {
	ToolName     string         `json:"toolName"`
	Description  string         `json:"description"`
	RegisteredAt time.Time      `json:"registeredAt"`
	Schema       map[string]any `json:"schema"`
	Annotations  map[string]any `json:"annotations,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type ToolExecutedEvent struct {
	ToolName   string         `json:"toolName"`
	Arguments  map[string]any `json:"arguments"`
	ExecutedAt time.Time      `json:"executedAt"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
	Result     any            `json:"result,omitempty"`
	Duration   time.Duration  `json:"duration"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ResourceRegisteredEvent struct {
	ResourcePath string         `json:"resourcePath"`
	Description  string         `json:"description"`
	RegisteredAt time.Time      `json:"registeredAt"`
	IsTemplate   bool           `json:"isTemplate"`
	Schema       map[string]any `json:"schema"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type ResourceAccessedEvent struct {
	ResourceURI  string         `json:"resourceUri"`
	ResourcePath string         `json:"resourcePath"`
	Parameters   map[string]any `json:"parameters"`
	AccessedAt   time.Time      `json:"accessedAt"`
	Success      bool           `json:"success"`
	Error        string         `json:"error,omitempty"`
	Duration     time.Duration  `json:"duration"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type PromptExecutedEvent struct {
	PromptName string         `json:"promptName"`
	Arguments  map[string]any `json:"arguments"`
	ExecutedAt time.Time      `json:"executedAt"`
	Success    bool           `json:"success"`
	Templates  int            `json:"templateCount"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type RequestFailedEvent struct {
	Method    string         `json:"method"`
	RequestID interface{}    `json:"requestId"`
	Error     string         `json:"error"`
	ErrorCode int            `json:"errorCode"`
	FailedAt  time.Time      `json:"failedAt"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func main() {
	// Create a server with events
	srv := server.NewServer("events-demo")

	logger := srv.Logger()

	// Subscribe to all server lifecycle events
	setupEventSubscriptions(srv, logger)

	// Add some tools, resources, and prompts to the server
	setupServerCapabilities(srv)

	logger.Info("🎯 Events Integration Example")
	logger.Info("============================")
	logger.Info("Server configured with comprehensive event monitoring")
	logger.Info("")
	logger.Info("📋 Available Event Topics:")
	logger.Info("  🚀 server initialization", "topic", events.TopicServerInitialized)
	logger.Info("  🛑 server shutdown", "topic", events.TopicServerShutdown)
	logger.Info("  🔌 client connections", "topic", events.TopicClientConnected)
	logger.Info("  🔌 client disconnections", "topic", events.TopicClientDisconnected)
	logger.Info("  🔧 tool registration", "topic", events.TopicToolRegistered)
	logger.Info("  ⚡ tool executions", "topic", events.TopicToolExecuted)
	logger.Info("  📄 resource registration", "topic", events.TopicResourceRegistered)
	logger.Info("  📖 resource access", "topic", events.TopicResourceAccessed)
	logger.Info("  💭 prompt execution", "topic", events.TopicPromptExecuted)
	logger.Info("  ❌ request failures", "topic", events.TopicRequestFailed)
	logger.Info("")
	logger.Info("🧪 Testing Event System:")

	// Demonstrate event publishing with some test events
	demonstrateEvents(srv, logger)

	logger.Info("")
	logger.Info("✅ Events integration example complete!")
	logger.Info("💡 In a real MCP server, these events fire automatically when:")
	logger.Info("   - Server starts and clients connect")
	logger.Info("   - Tools and resources are registered")
	logger.Info("   - Clients execute tools, access resources, or use prompts")
	logger.Info("   - Errors occur during request processing")
}

func setupEventSubscriptions(srv server.Server, logger *slog.Logger) {
	// Server lifecycle events
	events.Subscribe[ServerInitializedEvent](srv.Events(), events.TopicServerInitialized,
		func(ctx context.Context, evt ServerInitializedEvent) error {
			logger.Info("🚀 Server initialized",
				"name", evt.Name,
				"protocolVersion", evt.ProtocolVersion,
				"toolCount", evt.ToolCount,
				"resourceCount", evt.ResourceCount,
				"promptCount", evt.PromptCount)
			return nil
		})

	events.Subscribe[ServerShutdownEvent](srv.Events(), events.TopicServerShutdown,
		func(ctx context.Context, evt ServerShutdownEvent) error {
			logger.Info("🛑 Server shutting down",
				"name", evt.Name,
				"reason", evt.Reason)
			return nil
		})

	// Client connection events
	events.Subscribe[ClientConnectedEvent](srv.Events(), events.TopicClientConnected,
		func(ctx context.Context, evt ClientConnectedEvent) error {
			logger.Info("🔌 Client connected",
				"sessionID", evt.SessionID,
				"protocolVersion", evt.ProtocolVersion)
			return nil
		})

	events.Subscribe[ClientDisconnectedEvent](srv.Events(), events.TopicClientDisconnected,
		func(ctx context.Context, evt ClientDisconnectedEvent) error {
			logger.Info("🔌 Client disconnected",
				"sessionID", evt.SessionID,
				"reason", evt.Reason,
				"duration", evt.Duration)
			return nil
		})

	// Tool events
	events.Subscribe[ToolRegisteredEvent](srv.Events(), events.TopicToolRegistered,
		func(ctx context.Context, evt ToolRegisteredEvent) error {
			logger.Info("🔧 Tool registered",
				"toolName", evt.ToolName,
				"description", evt.Description)
			return nil
		})

	events.Subscribe[ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
		func(ctx context.Context, evt ToolExecutedEvent) error {
			if evt.Success {
				logger.Info("⚡ Tool executed successfully",
					"toolName", evt.ToolName,
					"duration", evt.Duration)
			} else {
				logger.Info("❌ Tool execution failed",
					"toolName", evt.ToolName,
					"error", evt.Error,
					"duration", evt.Duration)
			}
			return nil
		})

	// Resource events
	events.Subscribe[ResourceRegisteredEvent](srv.Events(), events.TopicResourceRegistered,
		func(ctx context.Context, evt ResourceRegisteredEvent) error {
			logger.Info("📄 Resource registered",
				"resourcePath", evt.ResourcePath,
				"description", evt.Description,
				"isTemplate", evt.IsTemplate)
			return nil
		})

	events.Subscribe[ResourceAccessedEvent](srv.Events(), events.TopicResourceAccessed,
		func(ctx context.Context, evt ResourceAccessedEvent) error {
			if evt.Success {
				logger.Info("📖 Resource accessed successfully",
					"resourceURI", evt.ResourceURI,
					"duration", evt.Duration)
			} else {
				logger.Info("❌ Resource access failed",
					"resourceURI", evt.ResourceURI,
					"error", evt.Error,
					"duration", evt.Duration)
			}
			return nil
		})

	// Prompt events
	events.Subscribe[PromptExecutedEvent](srv.Events(), events.TopicPromptExecuted,
		func(ctx context.Context, evt PromptExecutedEvent) error {
			logger.Info("💭 Prompt executed",
				"promptName", evt.PromptName,
				"templateCount", evt.Templates,
				"success", evt.Success)
			return nil
		})

	// Error events
	events.Subscribe[RequestFailedEvent](srv.Events(), events.TopicRequestFailed,
		func(ctx context.Context, evt RequestFailedEvent) error {
			logger.Info("❌ Request failed",
				"method", evt.Method,
				"requestID", evt.RequestID,
				"errorCode", evt.ErrorCode,
				"error", evt.Error)
			return nil
		})
}

func setupServerCapabilities(srv server.Server) {
	// Add tools
	srv.Tool("greet", "Greet someone with a personalized message", func(ctx *server.Context, args struct {
		Name string `json:"name" description:"The name of the person to greet"`
	}) (interface{}, error) {
		return fmt.Sprintf("Hello, %s! Welcome to the GoMCP events demo!", args.Name), nil
	})

	srv.Tool("calculate", "Perform basic mathematical operations", func(ctx *server.Context, args struct {
		Operation string  `json:"operation" description:"The operation to perform (add, subtract, multiply, divide)"`
		A         float64 `json:"a" description:"The first number"`
		B         float64 `json:"b" description:"The second number"`
	}) (interface{}, error) {
		switch args.Operation {
		case "add":
			return args.A + args.B, nil
		case "subtract":
			return args.A - args.B, nil
		case "multiply":
			return args.A * args.B, nil
		case "divide":
			if args.B == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return args.A / args.B, nil
		default:
			return nil, fmt.Errorf("unknown operation: %s", args.Operation)
		}
	})

	srv.Tool("timestamp", "Get the current timestamp", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"iso8601":   time.Now().Format(time.RFC3339),
			"readable":  time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	})

	// Add resources
	srv.Resource("/info", "Server information and statistics", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"name":        "events-demo",
			"version":     "1.0.0",
			"description": "GoMCP Events Integration Example",
			"uptime":      time.Since(time.Now()),
			"events":      "enabled",
		}, nil
	})

	srv.Resource("/status", "Current server health status", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now(),
			"components": map[string]string{
				"events":    "operational",
				"tools":     "operational",
				"resources": "operational",
			},
		}, nil
	})

	srv.Resource("/metrics/{type}", "Get server metrics by type", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// This would extract {type} from the URL in a real implementation
		return map[string]interface{}{
			"metrics": map[string]interface{}{
				"events_published":   42,
				"events_processed":   42,
				"tools_executed":     15,
				"resources_accessed": 8,
			},
			"timestamp": time.Now(),
		}, nil
	})

	// Add prompts
	srv.Prompt("greeting", "A friendly greeting prompt",
		server.User("Hello! My name is {{name}} and I'd like to {{action}}."))

	srv.Prompt("conversation", "A multi-turn conversation starter",
		server.User("Please help me with {{task}}."),
		server.Assistant("I'll be happy to help you with that. Let me gather some information first."),
		server.User("Here are the details: {{details}}"))

	srv.Prompt("analysis", "A prompt for analyzing data",
		server.User("Please analyze the following data: {{data}}"),
		server.User("Focus on these aspects: {{aspects}}"),
		server.Assistant("Based on my analysis, here are the key findings:"))
}

func demonstrateEvents(srv server.Server, logger *slog.Logger) {
	logger.Info("📢 Publishing test events to demonstrate the system...")

	// Test server initialized event
	testServerEvent := ServerInitializedEvent{
		Name:            "events-demo",
		ProtocolVersion: "2025-03-26",
		ToolCount:       3,
		ResourceCount:   3,
		PromptCount:     3,
		InitializedAt:   time.Now(),
		Metadata:        make(map[string]any),
	}

	if err := events.Publish[ServerInitializedEvent](srv.Events(), events.TopicServerInitialized, testServerEvent); err != nil {
		logger.Info("Failed to publish server initialized event", "error", err)
	}

	// Small delay between events to show async behavior
	time.Sleep(10 * time.Millisecond)

	// Test tool registered event
	testToolEvent := ToolRegisteredEvent{
		ToolName:     "demo-tool",
		Description:  "A demonstration tool for event testing",
		RegisteredAt: time.Now(),
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "Input parameter",
				},
			},
		},
		Metadata: make(map[string]any),
	}

	if err := events.Publish[ToolRegisteredEvent](srv.Events(), events.TopicToolRegistered, testToolEvent); err != nil {
		logger.Info("Failed to publish tool registered event", "error", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Test resource registered event
	testResourceEvent := ResourceRegisteredEvent{
		ResourcePath: "/demo/resource",
		Description:  "A demonstration resource for event testing",
		RegisteredAt: time.Now(),
		IsTemplate:   false,
		Schema: map[string]any{
			"type": "object",
		},
		Metadata: make(map[string]any),
	}

	if err := events.Publish[ResourceRegisteredEvent](srv.Events(), events.TopicResourceRegistered, testResourceEvent); err != nil {
		logger.Info("Failed to publish resource registered event", "error", err)
	}

	time.Sleep(10 * time.Millisecond)

	// Test request failed event
	testFailEvent := RequestFailedEvent{
		Method:    "demo/test",
		RequestID: "test-123",
		Error:     "This is a demonstration error for event testing",
		ErrorCode: -32603,
		FailedAt:  time.Now(),
		Metadata:  make(map[string]any),
	}

	if err := events.Publish[RequestFailedEvent](srv.Events(), events.TopicRequestFailed, testFailEvent); err != nil {
		logger.Info("Failed to publish request failed event", "error", err)
	}

	// Give events time to process
	time.Sleep(50 * time.Millisecond)
}
