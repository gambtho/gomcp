# Event System API Reference

The GoMCP event system provides comprehensive monitoring and observability for MCP server and client operations through a type-safe, channel-based architecture.

## Overview

The event system allows you to:
- Monitor server lifecycle events (initialization, shutdown)
- Track client connections and disconnections
- Observe tool, resource, and prompt operations
- Handle error events for debugging and alerting
- Build custom integrations for logging, metrics, and monitoring

## Core Event Interface

All events in GoMCP follow a consistent pattern using generics for type safety:

```go
// Subscribe to events with type safety
events.Subscribe[EventType](eventBus, topicConstant, handlerFunction)

// Publish events (typically done internally by GoMCP)
events.Publish[EventType](eventBus, topicConstant, eventData)
```

## Event Topics

### Server Lifecycle Topics

| Topic | Constant | Description |
|-------|----------|-------------|
| `server.initialized` | `events.TopicServerInitialized` | Server has been initialized and is ready |
| `server.shutdown` | `events.TopicServerShutdown` | Server is shutting down |

### Client Connection Topics

| Topic | Constant | Description |
|-------|----------|-------------|
| `client.connected` | `events.TopicClientConnected` | Client connected to server |
| `client.disconnected` | `events.TopicClientDisconnected` | Client disconnected from server |
| `client.initializing` | `events.TopicClientInitializing` | Client starting connection |
| `client.initialized` | `events.TopicClientInitialized` | Client successfully connected |
| `client.error` | `events.TopicClientError` | Client operation failed |

### Registration Topics

| Topic | Constant | Description |
|-------|----------|-------------|
| `tool.registered` | `events.TopicToolRegistered` | Tool registered with server |
| `resource.registered` | `events.TopicResourceRegistered` | Resource registered with server |

### Operation Topics

| Topic | Constant | Description |
|-------|----------|-------------|
| `tool.executed` | `events.TopicToolExecuted` | Tool execution completed |
| `resource.accessed` | `events.TopicResourceAccessed` | Resource access completed |
| `prompt.executed` | `events.TopicPromptExecuted` | Prompt execution completed |
| `request.failed` | `events.TopicRequestFailed` | MCP request failed |

## Event Data Structures

### Server Events

#### ServerInitializedEvent

Emitted when the server has been initialized and is ready to accept requests.

```go
type ServerInitializedEvent struct {
    ServerName        string                 `json:"serverName"`
    ProtocolVersion   string                 `json:"protocolVersion"`
    Capabilities      map[string]interface{} `json:"capabilities,omitempty"`
    InitializedAt     time.Time              `json:"initializedAt"`
    TransportType     string                 `json:"transportType,omitempty"`
    TransportEndpoint string                 `json:"transportEndpoint,omitempty"`
    ToolCount         int                    `json:"toolCount"`
    ResourceCount     int                    `json:"resourceCount"`
    PromptCount       int                    `json:"promptCount"`
    Metadata          map[string]any         `json:"metadata,omitempty"`
}
```

#### ServerShutdownEvent

Emitted when the server is shutting down.

```go
type ServerShutdownEvent struct {
    ServerName   string    `json:"serverName"`
    ShutdownAt   time.Time `json:"shutdownAt"`
    GracefulExit bool      `json:"gracefulExit"`
    Reason       string    `json:"reason,omitempty"`
}
```

### Client Events

#### ClientConnectedEvent

Emitted when a client connects to the server (server-side perspective).

```go
type ClientConnectedEvent struct {
    SessionID       string                 `json:"sessionId"`
    ProtocolVersion string                 `json:"protocolVersion"`
    ConnectedAt     time.Time              `json:"connectedAt"`
    ClientInfo      ClientInfo             `json:"clientInfo"`
    Capabilities    map[string]interface{} `json:"capabilities"`
}
```

#### ClientDisconnectedEvent

Emitted when a client disconnects from the server.

```go
type ClientDisconnectedEvent struct {
    // Client-side fields
    URL string `json:"url,omitempty"`

    // Server-side fields
    SessionID       string `json:"sessionId,omitempty"`
    ProtocolVersion string `json:"protocolVersion,omitempty"`
    ConnectedAt     string `json:"connectedAt,omitempty"`
    DisconnectedAt  string `json:"disconnectedAt,omitempty"`
}
```

#### ClientInitializingEvent

Emitted when a client starts connecting to a server (client-side).

```go
type ClientInitializingEvent struct {
    URL string `json:"url"`
}
```

#### ClientInitializedEvent

Emitted when a client successfully connects and initializes (client-side).

```go
type ClientInitializedEvent struct {
    URL string `json:"url"`
}
```

#### ClientErrorEvent

Emitted when a client operation fails (client-side).

```go
type ClientErrorEvent struct {
    Error string `json:"error"`
}
```

### Registration Events

#### ToolRegisteredEvent

Emitted when a tool is registered with the server.

```go
type ToolRegisteredEvent struct {
    ToolName     string                 `json:"toolName"`
    Description  string                 `json:"description"`
    RegisteredAt time.Time              `json:"registeredAt"`
    Schema       map[string]interface{} `json:"schema"`
    Annotations  map[string]interface{} `json:"annotations,omitempty"`
    Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
```

#### ResourceRegisteredEvent

Emitted when a resource is registered with the server.

```go
type ResourceRegisteredEvent struct {
    URI          string    `json:"uri"`
    Name         string    `json:"name"`
    Description  string    `json:"description"`
    MimeType     string    `json:"mimeType"`
    RegisteredAt time.Time `json:"registeredAt"`
}
```

### Operation Events

#### ToolExecutedEvent

Emitted when a tool execution completes successfully.

```go
type ToolExecutedEvent struct {
    Method       string `json:"method"`       // MCP method (e.g., "tools/call")
    RequestJSON  string `json:"requestJSON"`  // Actual JSON request sent
    ResponseJSON string `json:"responseJSON"` // Actual JSON response received
}
```

#### ResourceAccessedEvent

Emitted when a resource is accessed.

```go
type ResourceAccessedEvent struct {
    URI          string    `json:"uri"`
    Method       string    `json:"method"`
    AccessedAt   time.Time `json:"accessedAt"`
    Success      bool      `json:"success"`
    ErrorMessage string    `json:"errorMessage,omitempty"`
    ResponseSize int       `json:"responseSize,omitempty"`
}
```

#### PromptExecutedEvent

Emitted when a prompt is executed.

```go
type PromptExecutedEvent struct {
    PromptName   string                 `json:"promptName"`
    Arguments    map[string]interface{} `json:"arguments,omitempty"`
    ExecutedAt   time.Time              `json:"executedAt"`
    Success      bool                   `json:"success"`
    ErrorMessage string                 `json:"errorMessage,omitempty"`
    MessageCount int                    `json:"messageCount,omitempty"`
}
```

#### RequestFailedEvent

Emitted when any MCP request fails.

```go
type RequestFailedEvent struct {
    Method      string `json:"method"`      // MCP method that failed
    RequestJSON string `json:"requestJSON"` // Actual JSON request that failed
    Error       string `json:"error"`       // Error message
}
```

## Common Usage Patterns

### Basic Event Subscription

```go
events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
    func(ctx context.Context, evt events.ToolExecutedEvent) error {
        log.Printf("Tool executed: %s", evt.Method)
        return nil
    })
```

### Error Handling in Event Subscribers

```go
events.Subscribe[events.RequestFailedEvent](srv.Events(), events.TopicRequestFailed,
    func(ctx context.Context, evt events.RequestFailedEvent) error {
        // Log the error
        log.Printf("Request failed: method=%s, error=%s", evt.Method, evt.Error)
        
        // Optional: send to external monitoring system
        if err := sendToMonitoring(evt); err != nil {
            log.Printf("Failed to send event to monitoring: %v", err)
        }
        
        // Return nil to continue processing other events
        return nil
    })
```

### Multiple Event Subscriptions

```go
func setupEventHandlers(srv server.Server) {
    eventBus := srv.Events()
    
    // Server lifecycle
    events.Subscribe[events.ServerInitializedEvent](eventBus, events.TopicServerInitialized, onServerInitialized)
    events.Subscribe[events.ServerShutdownEvent](eventBus, events.TopicServerShutdown, onServerShutdown)
    
    // Client lifecycle
    events.Subscribe[events.ClientConnectedEvent](eventBus, events.TopicClientConnected, onClientConnected)
    events.Subscribe[events.ClientDisconnectedEvent](eventBus, events.TopicClientDisconnected, onClientDisconnected)
    
    // Operations
    events.Subscribe[events.ToolExecutedEvent](eventBus, events.TopicToolExecuted, onToolExecuted)
    events.Subscribe[events.ResourceAccessedEvent](eventBus, events.TopicResourceAccessed, onResourceAccessed)
    events.Subscribe[events.RequestFailedEvent](eventBus, events.TopicRequestFailed, onRequestFailed)
}
```

### Conditional Event Processing

```go
events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
    func(ctx context.Context, evt events.ToolExecutedEvent) error {
        // Only process tool calls from specific methods
        if evt.Method == "tools/call" {
            // Extract tool name from request JSON
            var request struct {
                Params struct {
                    Name string `json:"name"`
                } `json:"params"`
            }
            if err := json.Unmarshal([]byte(evt.RequestJSON), &request); err == nil {
                log.Printf("Tool %s was executed successfully", request.Params.Name)
            }
        }
        return nil
    })
```

## Best Practices

### 1. Always Return nil from Event Handlers

Unless you want to stop event processing, always return `nil` from event handlers:

```go
events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
    func(ctx context.Context, evt events.ToolExecutedEvent) error {
        // Do your processing...
        
        // Return nil to continue processing
        return nil
    })
```

### 2. Handle Context Cancellation

Respect context cancellation in long-running event handlers:

```go
events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
    func(ctx context.Context, evt events.ToolExecutedEvent) error {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Process the event...
        }
        return nil
    })
```

### 3. Use Structured Logging

Leverage the structured data in events for better logging:

```go
events.Subscribe[events.ClientConnectedEvent](srv.Events(), events.TopicClientConnected,
    func(ctx context.Context, evt events.ClientConnectedEvent) error {
        logger.Info("Client connected",
            "sessionId", evt.SessionID,
            "protocolVersion", evt.ProtocolVersion,
            "clientName", evt.ClientInfo.Name,
            "clientVersion", evt.ClientInfo.Version)
        return nil
    })
```

### 4. Avoid Blocking Operations

Keep event handlers fast and non-blocking. For heavy operations, use goroutines:

```go
events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
    func(ctx context.Context, evt events.ToolExecutedEvent) error {
        // Start heavy processing in background
        go func() {
            processToolExecutionMetrics(evt)
        }()
        return nil
    })
```

## Integration Examples

### Metrics Collection with Prometheus

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    toolCallsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gomcp_tool_calls_total",
            Help: "Total number of tool calls",
        },
        []string{"method", "tool_name"},
    )
)

func setupPrometheusMetrics(srv server.Server) {
    events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            // Extract tool name from request
            var request struct {
                Params struct {
                    Name string `json:"name"`
                } `json:"params"`
            }
            toolName := "unknown"
            if err := json.Unmarshal([]byte(evt.RequestJSON), &request); err == nil {
                toolName = request.Params.Name
            }
            
            toolCallsTotal.WithLabelValues(evt.Method, toolName).Inc()
            return nil
        })
}
```

### OpenTelemetry Tracing

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func setupOpenTelemetryTracing(srv server.Server) {
    tracer := otel.Tracer("gomcp-server")
    
    events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            _, span := tracer.Start(ctx, "tool.executed")
            defer span.End()
            
            span.SetAttributes(
                attribute.String("method", evt.Method),
                attribute.String("request", evt.RequestJSON),
                attribute.String("response", evt.ResponseJSON),
            )
            
            return nil
        })
}
```

### Custom Event Aggregation

```go
type EventAggregator struct {
    mu       sync.RWMutex
    stats    map[string]int64
    errors   []string
    lastSeen map[string]time.Time
}

func (ea *EventAggregator) SetupEventSubscriptions(srv server.Server) {
    ea.stats = make(map[string]int64)
    ea.lastSeen = make(map[string]time.Time)
    
    // Track all successful operations
    events.Subscribe[events.ToolExecutedEvent](srv.Events(), events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            ea.mu.Lock()
            ea.stats["tool_executions"]++
            ea.lastSeen["tool_execution"] = time.Now()
            ea.mu.Unlock()
            return nil
        })
    
    // Track errors
    events.Subscribe[events.RequestFailedEvent](srv.Events(), events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            ea.mu.Lock()
            ea.stats["errors"]++
            ea.errors = append(ea.errors, fmt.Sprintf("%s: %s", evt.Method, evt.Error))
            // Keep only last 100 errors
            if len(ea.errors) > 100 {
                ea.errors = ea.errors[1:]
            }
            ea.mu.Unlock()
            return nil
        })
}

func (ea *EventAggregator) GetStats() map[string]interface{} {
    ea.mu.RLock()
    defer ea.mu.RUnlock()
    
    return map[string]interface{}{
        "stats":      ea.stats,
        "recentErrors": ea.errors[max(0, len(ea.errors)-10):], // Last 10 errors
        "lastSeen":   ea.lastSeen,
    }
}
```

## See Also

- [Complete Event Integration Example](../../examples/events_integration/main.go)
- [Server API Reference](./server.md)
- [Client API Reference](./client.md) 