# Creating a Client

This tutorial covers how to create and configure a GOMCP client to communicate with MCP servers.

## Basic Client Creation

Creating a client requires specifying a service URL or transport type. The simplest form is:

```go
package main

import (
    "fmt"
    "log"

    "github.com/localrivet/gomcp/client"
)

func main() {
    // Create a client connected to an MCP server via stdio
    c, err := client.NewClient("stdio:///")
    if err != nil {
        log.Fatalf("Error creating client: %v", err)
    }
    defer c.Close()

    fmt.Println("Client successfully connected!")
}
```

## Client Options

The client can be configured with various options:

```go
c, err := client.NewClient("ws://localhost:8080/mcp",
    client.WithProtocolVersion("2025-03-26"),
    client.WithLogger(myLogger),
    client.WithRequestTimeout(time.Second * 20),
)
```

## Transport Types

GOMCP supports multiple transport types:

- **stdio**: `stdio:///` - Standard I/O for CLI tools and child processes
- **WebSocket**: `ws://host:port/path` - WebSocket communication
- **HTTP**: `http://host:port/path` - HTTP communication
- **SSE**: `sse://host:port/path` - Server-Sent Events
- **gRPC**: `grpc://host:port` - High-performance RPC with streaming support

### gRPC Client Configuration

For gRPC transport, you can use the specific gRPC client options:

```go
import "github.com/localrivet/gomcp/client"

c, err := client.NewClient("grpc-client",
    client.WithGRPC("localhost:50051"),
    client.WithGRPCTimeout(30*time.Second),
    client.WithGRPCKeepAlive(60*time.Second, 5*time.Second),
    client.WithGRPCMaxMessageSize(16*1024*1024),
)
if err != nil {
    log.Fatalf("Failed to create gRPC client: %v", err)
}
defer c.Close()

// The gRPC client supports all standard MCP operations
result, err := c.CallTool("echo", map[string]interface{}{
    "message": "Hello from gRPC!",
})
```

### gRPC with TLS

For secure gRPC connections:

```go
c, err := client.NewClient("grpc-secure-client",
    client.WithGRPC("localhost:50051"),
    client.WithGRPCTLS("cert.pem", "key.pem", "ca.pem"),
)
```

## Client Lifecycle

The typical lifecycle of a client includes:

1. Creation via `NewClient()`
2. Making API calls (tools, resources, etc.)
3. Proper shutdown via `Close()`

## Error Handling

Always check errors from client creation and API calls:

```go
c, err := client.NewClient("stdio:///")
if err != nil {
    // Handle initialization error
    log.Fatalf("Failed to create client: %v", err)
}

result, err := c.CallTool("example", map[string]interface{}{})
if err != nil {
    // Handle API call error
    log.Printf("Tool call failed: %v", err)
}
```

## Next Steps

- Learn how to [call tools](02-calling-tools.md)
- See the [API reference](../../api-reference/README.md) for all client options
