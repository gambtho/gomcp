# Event System Tutorial

This tutorial demonstrates how to use GoMCP's event system to monitor and react to various operations within your MCP server or client applications.

## Overview

The GoMCP event system provides real-time observability into:
- Server lifecycle events (startup, shutdown)
- Client connections and disconnections
- Tool executions and registrations
- Resource access and registrations
- Prompt executions
- Error conditions

## Prerequisites

- Basic knowledge of Go and the MCP protocol
- Familiarity with GoMCP server and client implementations
- Understanding of Go channels and goroutines (helpful but not required)

## Getting Started

### 1. Setting Up Event Monitoring in a Server

Let's start by creating a simple MCP server that monitors its own events:

```go
package main

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "time"

    "github.com/localrivet/gomcp/events"
    "github.com/localrivet/gomcp/server"
)

func main() {
    // Create a logger for output
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    // Create a new server
    srv := server.NewServer("event-tutorial-server",
        server.WithLogger(logger),
    )

    // Set up event monitoring BEFORE starting the server
    setupEventMonitoring(srv, logger)

    // Register a simple tool
    srv.Tool("greet", "Greet someone", func(ctx *server.Context, args struct {
        Name string `json:"name"`
    }) (interface{}, error) {
        return map[string]interface{}{
            "message": fmt.Sprintf("Hello, %s!", args.Name),
        }, nil
    })

    // Register a resource
    srv.Resource("/status", "Server status", func(ctx *server.Context, args *struct{}) (interface{}, error) {
        return map[string]interface{}{
            "status": "healthy",
            "uptime": time.Since(time.Now()), // This would be actual uptime in real code
        }, nil
    })

    logger.Info("Starting server with event monitoring...")
    
    // Start the server (this will trigger events)
    if err := srv.AsStdio().Run(); err != nil {
        logger.Error("Server failed", "error", err)
    }
}

func setupEventMonitoring(srv server.Server, logger *slog.Logger) {
    eventBus := srv.Events()

    // Monitor server lifecycle
    events.Subscribe[events.ServerInitializedEvent](eventBus, events.TopicServerInitialized,
        func(ctx context.Context, evt events.ServerInitializedEvent) error {
            logger.Info("🚀 Server initialized",
                "serverName", evt.ServerName,
                "protocolVersion", evt.ProtocolVersion,
                "toolCount", evt.ToolCount,
                "resourceCount", evt.ResourceCount,
                "promptCount", evt.PromptCount)
            return nil
        })

    events.Subscribe[events.ServerShutdownEvent](eventBus, events.TopicServerShutdown,
        func(ctx context.Context, evt events.ServerShutdownEvent) error {
            logger.Info("🛑 Server shutting down",
                "serverName", evt.ServerName,
                "graceful", evt.GracefulExit,
                "reason", evt.Reason)
            return nil
        })

    // Monitor client connections
    events.Subscribe[events.ClientConnectedEvent](eventBus, events.TopicClientConnected,
        func(ctx context.Context, evt events.ClientConnectedEvent) error {
            logger.Info("🔌 Client connected",
                "sessionId", evt.SessionID,
                "protocolVersion", evt.ProtocolVersion,
                "clientName", evt.ClientInfo.Name)
            return nil
        })

    events.Subscribe[events.ClientDisconnectedEvent](eventBus, events.TopicClientDisconnected,
        func(ctx context.Context, evt events.ClientDisconnectedEvent) error {
            logger.Info("🔌 Client disconnected",
                "sessionId", evt.SessionID)
            return nil
        })

    // Monitor tool operations
    events.Subscribe[events.ToolRegisteredEvent](eventBus, events.TopicToolRegistered,
        func(ctx context.Context, evt events.ToolRegisteredEvent) error {
            logger.Info("🔧 Tool registered",
                "toolName", evt.ToolName,
                "description", evt.Description)
            return nil
        })

    events.Subscribe[events.ToolExecutedEvent](eventBus, events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            logger.Info("⚡ Tool executed",
                "method", evt.Method,
                "hasResponse", len(evt.ResponseJSON) > 0)
            return nil
        })

    // Monitor resource operations
    events.Subscribe[events.ResourceRegisteredEvent](eventBus, events.TopicResourceRegistered,
        func(ctx context.Context, evt events.ResourceRegisteredEvent) error {
            logger.Info("📄 Resource registered",
                "uri", evt.URI,
                "name", evt.Name,
                "mimeType", evt.MimeType)
            return nil
        })

    events.Subscribe[events.ResourceAccessedEvent](eventBus, events.TopicResourceAccessed,
        func(ctx context.Context, evt events.ResourceAccessedEvent) error {
            logger.Info("📖 Resource accessed",
                "uri", evt.URI,
                "method", evt.Method,
                "success", evt.Success,
                "responseSize", evt.ResponseSize)
            return nil
        })

    // Monitor errors
    events.Subscribe[events.RequestFailedEvent](eventBus, events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            logger.Error("❌ Request failed",
                "method", evt.Method,
                "error", evt.Error)
            return nil
        })
}
```

### 2. Client-Side Event Monitoring

You can also monitor events from the client side:

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/localrivet/gomcp/client"
    "github.com/localrivet/gomcp/events"
)

func main() {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    // Create a client
    c, err := client.NewClient("event-tutorial-client")
    if err != nil {
        logger.Error("Failed to create client", "error", err)
        return
    }
    defer c.Close()

    // Set up client event monitoring
    setupClientEventMonitoring(c, logger)

    // Connect to a server (replace with your server command)
    err = c.ConnectStdio("./your-mcp-server")
    if err != nil {
        logger.Error("Failed to connect", "error", err)
        return
    }

    // Perform some operations that will trigger events
    result, err := c.CallTool("greet", map[string]interface{}{
        "name": "World",
    })
    if err != nil {
        logger.Error("Tool call failed", "error", err)
    } else {
        logger.Info("Tool result", "result", result)
    }

    // Give events time to process
    time.Sleep(100 * time.Millisecond)
}

func setupClientEventMonitoring(c client.Client, logger *slog.Logger) {
    eventBus := c.Events()

    // Monitor client lifecycle
    events.Subscribe[events.ClientInitializingEvent](eventBus, events.TopicClientInitializing,
        func(ctx context.Context, evt events.ClientInitializingEvent) error {
            logger.Info("🔄 Client connecting", "url", evt.URL)
            return nil
        })

    events.Subscribe[events.ClientInitializedEvent](eventBus, events.TopicClientInitialized,
        func(ctx context.Context, evt events.ClientInitializedEvent) error {
            logger.Info("✅ Client connected", "url", evt.URL)
            return nil
        })

    events.Subscribe[events.ClientErrorEvent](eventBus, events.TopicClientError,
        func(ctx context.Context, evt events.ClientErrorEvent) error {
            logger.Error("❌ Client error", "error", evt.Error)
            return nil
        })

    events.Subscribe[events.ClientDisconnectedEvent](eventBus, events.TopicClientDisconnected,
        func(ctx context.Context, evt events.ClientDisconnectedEvent) error {
            logger.Info("🔌 Client disconnected", "url", evt.URL)
            return nil
        })

    // Monitor operations
    events.Subscribe[events.ToolExecutedEvent](eventBus, events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            logger.Info("⚡ Tool called", "method", evt.Method)
            return nil
        })

    events.Subscribe[events.RequestFailedEvent](eventBus, events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            logger.Error("❌ Request failed", "method", evt.Method, "error", evt.Error)
            return nil
        })
}
```

## Building Practical Applications

### 3. Metrics Collection

Here's an example of using events to collect metrics:

```go
package main

import (
    "context"
    "fmt"
    "sync/atomic"
    "time"

    "github.com/localrivet/gomcp/events"
    "github.com/localrivet/gomcp/server"
)

type ServerMetrics struct {
    toolCallsTotal    int64
    resourceReads     int64
    errorsTotal       int64
    clientsConnected  int64
    startTime         time.Time
}

func NewServerMetrics() *ServerMetrics {
    return &ServerMetrics{
        startTime: time.Now(),
    }
}

func (m *ServerMetrics) SetupMetricsCollection(srv server.Server) {
    eventBus := srv.Events()

    // Count tool executions
    events.Subscribe[events.ToolExecutedEvent](eventBus, events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            atomic.AddInt64(&m.toolCallsTotal, 1)
            return nil
        })

    // Count resource access
    events.Subscribe[events.ResourceAccessedEvent](eventBus, events.TopicResourceAccessed,
        func(ctx context.Context, evt events.ResourceAccessedEvent) error {
            if evt.Success {
                atomic.AddInt64(&m.resourceReads, 1)
            }
            return nil
        })

    // Count errors
    events.Subscribe[events.RequestFailedEvent](eventBus, events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            atomic.AddInt64(&m.errorsTotal, 1)
            return nil
        })

    // Track client connections
    events.Subscribe[events.ClientConnectedEvent](eventBus, events.TopicClientConnected,
        func(ctx context.Context, evt events.ClientConnectedEvent) error {
            atomic.AddInt64(&m.clientsConnected, 1)
            return nil
        })

    events.Subscribe[events.ClientDisconnectedEvent](eventBus, events.TopicClientDisconnected,
        func(ctx context.Context, evt events.ClientDisconnectedEvent) error {
            atomic.AddInt64(&m.clientsConnected, -1)
            return nil
        })
}

func (m *ServerMetrics) GetMetrics() map[string]interface{} {
    return map[string]interface{}{
        "tool_calls_total":   atomic.LoadInt64(&m.toolCallsTotal),
        "resource_reads":     atomic.LoadInt64(&m.resourceReads),
        "errors_total":       atomic.LoadInt64(&m.errorsTotal),
        "clients_connected":  atomic.LoadInt64(&m.clientsConnected),
        "uptime_seconds":     time.Since(m.startTime).Seconds(),
    }
}

func (m *ServerMetrics) PrintMetrics() {
    metrics := m.GetMetrics()
    fmt.Printf("📊 Server Metrics:\n")
    for key, value := range metrics {
        fmt.Printf("  %s: %v\n", key, value)
    }
}
```

### 4. Health Monitoring

Create a health monitor that tracks server health based on events:

```go
package main

import (
    "context"
    "sync"
    "time"

    "github.com/localrivet/gomcp/events"
    "github.com/localrivet/gomcp/server"
)

type HealthStatus string

const (
    Healthy     HealthStatus = "healthy"
    Degraded    HealthStatus = "degraded"
    Unhealthy   HealthStatus = "unhealthy"
)

type HealthMonitor struct {
    mu               sync.RWMutex
    status           HealthStatus
    lastToolExecution time.Time
    errorRate        float64
    recentErrors     []time.Time
    maxErrorWindow   time.Duration
    maxErrorRate     float64
}

func NewHealthMonitor() *HealthMonitor {
    return &HealthMonitor{
        status:         Healthy,
        maxErrorWindow: 5 * time.Minute,
        maxErrorRate:   0.1, // 10% error rate threshold
    }
}

func (h *HealthMonitor) SetupHealthMonitoring(srv server.Server) {
    eventBus := srv.Events()

    // Track successful tool executions
    events.Subscribe[events.ToolExecutedEvent](eventBus, events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            h.mu.Lock()
            h.lastToolExecution = time.Now()
            h.mu.Unlock()
            h.updateHealthStatus()
            return nil
        })

    // Track errors
    events.Subscribe[events.RequestFailedEvent](eventBus, events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            h.mu.Lock()
            h.recentErrors = append(h.recentErrors, time.Now())
            h.cleanOldErrors()
            h.calculateErrorRate()
            h.mu.Unlock()
            h.updateHealthStatus()
            return nil
        })
}

func (h *HealthMonitor) cleanOldErrors() {
    cutoff := time.Now().Add(-h.maxErrorWindow)
    for i, errorTime := range h.recentErrors {
        if errorTime.After(cutoff) {
            h.recentErrors = h.recentErrors[i:]
            return
        }
    }
    h.recentErrors = nil
}

func (h *HealthMonitor) calculateErrorRate() {
    // This is a simplified calculation
    // In practice, you'd want to compare errors to total requests
    h.errorRate = float64(len(h.recentErrors)) / 100.0 // Assuming 100 requests per window
}

func (h *HealthMonitor) updateHealthStatus() {
    h.mu.Lock()
    defer h.mu.Unlock()

    timeSinceLastTool := time.Since(h.lastToolExecution)
    
    if h.errorRate > h.maxErrorRate || timeSinceLastTool > 10*time.Minute {
        h.status = Unhealthy
    } else if h.errorRate > h.maxErrorRate/2 || timeSinceLastTool > 5*time.Minute {
        h.status = Degraded
    } else {
        h.status = Healthy
    }
}

func (h *HealthMonitor) GetStatus() HealthStatus {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.status
}

func (h *HealthMonitor) GetHealthReport() map[string]interface{} {
    h.mu.RLock()
    defer h.mu.RUnlock()

    return map[string]interface{}{
        "status":                string(h.status),
        "last_tool_execution":   h.lastToolExecution,
        "error_rate":           h.errorRate,
        "recent_error_count":   len(h.recentErrors),
        "time_since_last_tool": time.Since(h.lastToolExecution),
    }
}
```

### 5. Audit Logging

Implement detailed audit logging for security and compliance:

```go
package main

import (
    "context"
    "encoding/json"
    "log/slog"
    "time"

    "github.com/localrivet/gomcp/events"
    "github.com/localrivet/gomcp/server"
)

type AuditLogger struct {
    logger *slog.Logger
}

func NewAuditLogger(logger *slog.Logger) *AuditLogger {
    return &AuditLogger{logger: logger}
}

func (a *AuditLogger) SetupAuditLogging(srv server.Server) {
    eventBus := srv.Events()

    // Audit all tool executions
    events.Subscribe[events.ToolExecutedEvent](eventBus, events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            a.auditToolExecution(evt)
            return nil
        })

    // Audit client connections
    events.Subscribe[events.ClientConnectedEvent](eventBus, events.TopicClientConnected,
        func(ctx context.Context, evt events.ClientConnectedEvent) error {
            a.auditClientConnection(evt)
            return nil
        })

    // Audit failed requests
    events.Subscribe[events.RequestFailedEvent](eventBus, events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            a.auditFailedRequest(evt)
            return nil
        })
}

func (a *AuditLogger) auditToolExecution(evt events.ToolExecutedEvent) {
    // Extract tool name from request JSON
    var request struct {
        Params struct {
            Name      string          `json:"name"`
            Arguments json.RawMessage `json:"arguments"`
        } `json:"params"`
    }
    
    toolName := "unknown"
    if err := json.Unmarshal([]byte(evt.RequestJSON), &request); err == nil {
        toolName = request.Params.Name
    }

    a.logger.Info("AUDIT: Tool execution",
        "event_type", "tool_execution",
        "tool_name", toolName,
        "method", evt.Method,
        "timestamp", time.Now(),
        "request_size", len(evt.RequestJSON),
        "response_size", len(evt.ResponseJSON))
}

func (a *AuditLogger) auditClientConnection(evt events.ClientConnectedEvent) {
    a.logger.Info("AUDIT: Client connection",
        "event_type", "client_connection",
        "session_id", evt.SessionID,
        "protocol_version", evt.ProtocolVersion,
        "client_name", evt.ClientInfo.Name,
        "client_version", evt.ClientInfo.Version,
        "timestamp", evt.ConnectedAt)
}

func (a *AuditLogger) auditFailedRequest(evt events.RequestFailedEvent) {
    a.logger.Warn("AUDIT: Failed request",
        "event_type", "request_failure",
        "method", evt.Method,
        "error", evt.Error,
        "timestamp", time.Now(),
        "request_size", len(evt.RequestJSON))
}
```

## Advanced Patterns

### 6. Event Filtering and Processing

You can create sophisticated event processing pipelines:

```go
package main

import (
    "context"
    "strings"

    "github.com/localrivet/gomcp/events"
    "github.com/localrivet/gomcp/server"
)

type EventProcessor struct {
    highPriorityTools map[string]bool
    alertThreshold    int
    alertHandler      func(string)
}

func NewEventProcessor(alertHandler func(string)) *EventProcessor {
    return &EventProcessor{
        highPriorityTools: map[string]bool{
            "delete_file":   true,
            "execute_command": true,
            "modify_database": true,
        },
        alertThreshold: 5,
        alertHandler:   alertHandler,
    }
}

func (ep *EventProcessor) SetupEventProcessing(srv server.Server) {
    eventBus := srv.Events()

    // Process tool executions with filtering
    events.Subscribe[events.ToolExecutedEvent](eventBus, events.TopicToolExecuted,
        func(ctx context.Context, evt events.ToolExecutedEvent) error {
            return ep.processToolExecution(evt)
        })

    // Process errors with pattern matching
    events.Subscribe[events.RequestFailedEvent](eventBus, events.TopicRequestFailed,
        func(ctx context.Context, evt events.RequestFailedEvent) error {
            return ep.processError(evt)
        })
}

func (ep *EventProcessor) processToolExecution(evt events.ToolExecutedEvent) error {
    // Extract tool name (simplified)
    toolName := ep.extractToolName(evt.RequestJSON)
    
    // Check if this is a high-priority tool
    if ep.highPriorityTools[toolName] {
        ep.alertHandler(fmt.Sprintf("High-priority tool executed: %s", toolName))
    }
    
    return nil
}

func (ep *EventProcessor) processError(evt events.RequestFailedEvent) error {
    // Check for security-related errors
    if strings.Contains(strings.ToLower(evt.Error), "permission") ||
       strings.Contains(strings.ToLower(evt.Error), "unauthorized") ||
       strings.Contains(strings.ToLower(evt.Error), "forbidden") {
        ep.alertHandler(fmt.Sprintf("Security-related error: %s in method %s", evt.Error, evt.Method))
    }
    
    return nil
}

func (ep *EventProcessor) extractToolName(requestJSON string) string {
    // Implementation to extract tool name from JSON
    // This is simplified - you'd want proper JSON parsing
    return "unknown"
}
```

## Running the Complete Example

You can find a complete, runnable example in the GoMCP repository at [`examples/events_integration/main.go`](../../examples/events_integration/main.go). This example demonstrates:

- All event types in action
- Proper event subscription setup
- Structured logging with events
- Event-driven metrics collection

To run the example:

```bash
cd examples/events_integration
go run main.go
```

## Best Practices

1. **Set up event subscriptions early** - Subscribe to events before starting your server or connecting your client
2. **Keep event handlers fast** - Avoid blocking operations in event handlers
3. **Handle errors gracefully** - Always return `nil` from event handlers unless you want to stop processing
4. **Use structured logging** - Leverage the rich data in events for better observability
5. **Consider rate limiting** - For high-volume events, implement rate limiting or sampling
6. **Test your event handlers** - Write unit tests for your event handling logic

## Next Steps

- Explore the [Event System API Reference](../api-reference/events.md) for detailed information about all event types
- Check out integration examples for specific monitoring systems (Prometheus, OpenTelemetry, etc.)
- Consider building custom event aggregation and alerting systems based on your needs

The event system provides a powerful foundation for building production-ready MCP applications with comprehensive observability and monitoring capabilities. 