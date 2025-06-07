# gRPC Transport Example

The gRPC transport provides high-performance, strongly-typed RPC communication using Protocol Buffers. This transport is ideal for service-to-service communication with strong type safety, bidirectional streaming capabilities, and cross-language compatibility.

## Overview

In this example, we demonstrate:

- Setting up both gRPC server and client for MCP
- Creating an MCP server with gRPC transport
- Creating an MCP client that connects via gRPC
- Understanding how the MCP JSON-RPC protocol maps to gRPC streaming
- Working with request/response matching and concurrent operations

> **Note**: The gRPC transport is fully implemented with both server and client support, including proper request/response matching and concurrent operation handling.

## Protocol Buffer Definitions

The gRPC transport uses Protocol Buffers to define the service interface:

```protobuf
syntax = "proto3";

package mcp;

option go_package = "github.com/localrivet/gomcp/transport/grpc/proto";

service MCPService {
  // Unary RPCs
  rpc Initialize(InitializeRequest) returns (InitializeResponse) {}
  rpc CallTool(ToolRequest) returns (ToolResponse) {}
  rpc ReadResource(ResourceRequest) returns (ResourceResponse) {}
  rpc RenderPrompt(PromptRequest) returns (PromptResponse) {}

  // Streaming RPCs
  rpc Sample(SampleRequest) returns (stream SampleResponse) {}
  rpc Notifications(NotificationRequest) returns (stream Notification) {}
}

// Message definitions continue...
```

## Client Configuration

```go
// Create a new client with gRPC transport
c, err := client.NewClient("grpc-example-client",
    client.WithGRPC("localhost:50051"),
    client.WithConnectionTimeout(5*time.Second),
    client.WithRequestTimeout(30*time.Second),
)
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
defer c.Close()

// Call a tool
result, err := c.CallTool("echo", map[string]interface{}{
    "message": "Hello from gRPC client!",
})
if err != nil {
    log.Fatalf("Tool call failed: %v", err)
}
fmt.Printf("Result: %v\n", result)

// Use streaming sample function
sampleStream, err := c.Sample("generate_text", map[string]interface{}{
    "prompt": "Once upon a time",
    "maxTokens": 100,
})
if err != nil {
    log.Fatalf("Sampling failed: %v", err)
}

// Process streaming responses
for {
    chunk, err := sampleStream.Read()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatalf("Error reading sample chunk: %v", err)
    }
    fmt.Print(chunk)
}
```

## Server Implementation

The gRPC server is fully functional and can be set up easily:

```go
// Create a new server
srv := server.NewServer("grpc-example-server")

// Configure the server with gRPC transport
srv.AsGRPC(":50051")

// Register a simple echo tool
srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
    Message string `json:"message"`
}) (map[string]interface{}, error) {
    fmt.Printf("Server received: %s\n", args.Message)
    return map[string]interface{}{
        "echoed": args.Message,
    }, nil
})

// Start the server
if err := srv.Run(); err != nil {
    log.Fatalf("Server error: %v", err)
}
```

### Key Features

- **Full Protocol Support**: Complete MCP JSON-RPC protocol implementation over gRPC streams
- **Request/Response Matching**: Proper handling of concurrent requests with JSON-RPC ID matching
- **Bidirectional Streaming**: Real-time communication between client and server
- **Auto-shutdown**: Graceful shutdown with configurable timeouts

## Advantages of gRPC Transport

- **Strong Typing**: Protocol Buffer definitions ensure type safety
- **High Performance**: Uses HTTP/2 for multiplexed connections with binary serialization
- **Streaming Support**: Bidirectional streaming allows real-time data exchange
- **Cross-Language**: Protocol Buffers support many programming languages
- **Code Generation**: Automatically generates client and server code
- **Well-defined Interface**: Clear service definitions with methods and messages
- **Modern Features**: Built-in deadline propagation, cancellation, and load balancing

## Limitations of gRPC Transport

- **Protocol Buffer Learning Curve**: Requires understanding Protocol Buffers
- **Less Human-Readable**: Binary protocol not as easy to debug as JSON
- **Proxy/Firewall Issues**: Some environments restrict HTTP/2
- **Code Generation Complexity**: Requires build system integration for .proto files

## Running the Example

```bash
# Navigate to the gRPC example directory
cd examples/grpc

# Run the complete gRPC example (server + client + automatic shutdown)
go run .

# Output shows:
# - Server starting on :50051
# - Client connecting successfully  
# - Tool call execution with proper results
# - Tools list with schema information
# - Automatic graceful shutdown
```

### Example Output

```
=== gRPC MCP Example ===
This example demonstrates the fully implemented gRPC transport
for the Model Context Protocol (MCP) in gomcp.

Starting gRPC server on :50051...
Creating gRPC client connecting to localhost:50051...
Successfully connected to gRPC server!
Calling echo tool...
Tool result: map[content:[map[text:{"echoed": "Hello from gRPC client!"} type:text]] isError:false]
Listing available tools...
Available tools:
  - echo: Echo back the provided message

Demo completed successfully!
Shutting down automatically in 2 seconds...
Auto-shutdown triggered
Stopping server...
Server stopped successfully
```

## Complete Example

See the full working example in [examples/grpc](https://github.com/localrivet/gomcp/tree/main/examples/grpc) in the GOMCP repository.
