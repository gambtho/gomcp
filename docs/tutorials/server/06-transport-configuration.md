# Transport Configuration

GOMCP supports multiple transport protocols, each optimized for different use cases. This tutorial covers how to configure and use each transport type.

## Overview

GOMCP provides the following transport options:

| Transport    | Bidirectional | Streaming | Connection | Best For                                        |
| ------------ | :-----------: | :-------: | :--------: | ----------------------------------------------- |
| HTTP         |      No       |    No     | Stateless  | Simple integrations, REST-like services        |
| WebSocket    |      Yes      |    Yes    | Persistent | Web applications, real-time updates            |
| SSE          |      No       |    Yes    | Persistent | Server-to-client updates, monitoring           |
| Unix Socket  |      Yes      |    Yes    | Persistent | High-performance local IPC                     |
| UDP          |      Yes      |    No     | Connectionless | High-throughput, latency-sensitive apps    |
| MQTT         |      Yes      |    No     | Persistent | IoT, telemetry, pub/sub patterns               |
| NATS         |      Yes      |    Yes    | Persistent | Microservices, cloud-native applications       |
| Standard I/O |      Yes      |    Yes    | Direct     | CLI tools, child processes                     |
| gRPC         |      Yes      |    Yes    | Persistent | Service-to-service, high-performance RPC       |

## Standard I/O Transport

Best for command-line tools and subprocess communication:

```go
srv := server.NewServer("my-server").AsStdio()
```

**Characteristics:**
- Uses stdin/stdout for communication
- Perfect for CLI integration
- No network configuration needed
- Blocking I/O model

## HTTP Transport

Simple stateless communication:

```go
srv := server.NewServer("my-server").AsHTTP(":8080")
```

**Configuration Options:**
```go
srv := server.NewServer("my-server")
srv.AsHTTP(":8080", 
    server.WithTimeout(30*time.Second),
    server.WithMaxBodySize(10*1024*1024),
)
```

## WebSocket Transport

Bidirectional real-time communication:

```go
srv := server.NewServer("my-server").AsWebSocket(":8080", "/mcp")
```

**Configuration Options:**
```go
srv := server.NewServer("my-server")
srv.AsWebSocket(":8080", "/mcp",
    server.WithTLS("cert.pem", "key.pem"),
    server.WithTimeout(30*time.Second),
    server.WithOriginChecker(func(r *http.Request) bool {
        return r.Header.Get("Origin") == "https://trusted-domain.com"
    }),
)
```

## Server-Sent Events (SSE)

Server-to-client streaming:

```go
srv := server.NewServer("my-server").AsSSE(":8080", "/events")
```

**Configuration Options:**
```go
srv := server.NewServer("my-server")
srv.AsSSE(":8080", "/events",
    server.WithTLS("cert.pem", "key.pem"),
    server.WithSSEReconnectTime(5*time.Second),
    server.WithSSEHeartbeat(30*time.Second),
)
```

## Unix Socket Transport

High-performance local IPC:

```go
srv := server.NewServer("my-server").AsUnix("/tmp/mcp.sock")
```

**Configuration Options:**
```go
srv := server.NewServer("my-server")
srv.AsUnix("/tmp/mcp.sock",
    server.WithUnixPermissions(0755),
    server.WithTimeout(30*time.Second),
)
```

## UDP Transport

Low-latency connectionless communication:

```go
srv := server.NewServer("my-server").AsUDP(":9090")
```

**Configuration Options:**
```go
srv := server.NewServer("my-server")
srv.AsUDP(":9090",
    server.WithUDPReliability(true), // Add reliability layer
    server.WithUDPMaxPacketSize(8192),
    server.WithUDPTimeout(5*time.Second),
)
```

## MQTT Transport

Publish/subscribe messaging:

```go
srv := server.NewServer("my-server").AsMQTT("tcp://localhost:1883", "mcp/server")
```

**Configuration Options:**
```go
srv := server.NewServer("my-server")
srv.AsMQTT("tcp://localhost:1883", "mcp/server",
    server.WithMQTTClientID("my-server-123"),
    server.WithMQTTAuth("username", "password"),
    server.WithMQTTTLS(&tls.Config{InsecureSkipVerify: true}),
    server.WithMQTTKeepAlive(30*time.Second),
    server.WithMQTTQoS(1),
)
```

## NATS Transport

Cloud-native messaging:

```go
srv := server.NewServer("my-server").AsNATS("nats://localhost:4222", "mcp.server")
```

**Configuration Options:**
```go
srv := server.NewServer("my-server")
srv.AsNATS("nats://localhost:4222", "mcp.server",
    server.WithNATSAuth("user", "password"),
    server.WithNATSTimeout(10*time.Second),
    server.WithNATSReconnectWait(2*time.Second),
    server.WithNATSMaxReconnects(10),
)
```

## gRPC Transport

High-performance RPC with Protocol Buffers:

```go
srv := server.NewServer("my-server").AsGRPC(":50051")
```

**Configuration Options:**
```go
srv := server.NewServer("my-server")
srv.AsGRPC(":50051",
    server.WithGRPCTLS("cert.pem", "key.pem", "ca.pem"),
    server.WithGRPCMaxMessageSize(16*1024*1024),
    server.WithGRPCKeepAlive(30*time.Second, 5*time.Second),
    server.WithGRPCConnectionTimeout(10*time.Second),
)
```

**Key Features:**
- Full bidirectional streaming support
- Request/response matching with JSON-RPC ID correlation
- Protocol Buffer encoding for efficient serialization
- HTTP/2 multiplexing for concurrent operations
- Built-in support for timeouts and cancellation
- Cross-language compatibility

**Example with complete setup:**
```go
package main

import (
    "log"
    "time"
    
    "github.com/localrivet/gomcp/server"
)

func main() {
    srv := server.NewServer("grpc-demo")
    
    // Configure gRPC with options
    srv.AsGRPC(":50051",
        server.WithGRPCMaxMessageSize(8*1024*1024),
        server.WithGRPCKeepAlive(30*time.Second, 5*time.Second),
    )
    
    // Register tools
    srv.Tool("echo", "Echo messages", func(ctx *server.Context, args struct {
        Message string `json:"message"`
    }) (map[string]interface{}, error) {
        return map[string]interface{}{
            "echoed": args.Message,
        }, nil
    })
    
    // Start server
    if err := srv.Run(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## TLS Configuration

Most transports support TLS encryption:

```go
// HTTP/WebSocket/SSE
srv.AsHTTP(":8443", server.WithTLS("cert.pem", "key.pem"))

// gRPC  
srv.AsGRPC(":50051", server.WithGRPCTLS("cert.pem", "key.pem", "ca.pem"))

// MQTT
srv.AsMQTT("ssl://localhost:8883", "topic", 
    server.WithMQTTTLS(&tls.Config{...}))
```

## Choosing the Right Transport

**For Web Applications:** WebSocket or SSE
- WebSocket for bidirectional communication
- SSE for server-to-client streaming only

**For Microservices:** gRPC or NATS
- gRPC for direct service-to-service communication
- NATS for event-driven architectures

**For CLI Tools:** Standard I/O or Unix Socket
- Standard I/O for simple command-line tools
- Unix Socket for higher performance local communication

**For IoT/Embedded:** MQTT or UDP
- MQTT for reliable pub/sub messaging
- UDP for low-latency, high-throughput scenarios

**For Simple Integration:** HTTP
- REST-like request/response patterns
- Easy to debug and test

## Performance Considerations

| Transport | Latency | Throughput | Memory | CPU |
|-----------|---------|------------|--------|-----|
| Unix Socket | Lowest | Highest | Low | Low |
| gRPC | Low | High | Medium | Medium |
| UDP | Low | High | Low | Low |
| WebSocket | Medium | Medium | Medium | Medium |
| NATS | Medium | High | Medium | Medium |
| HTTP | Highest | Lowest | Medium | Medium |

## Error Handling

All transports support comprehensive error handling:

```go
srv := server.NewServer("my-server")

// Global error handler
srv.OnError(func(err error) {
    log.Printf("Server error: %v", err)
})

// Transport-specific configuration
srv.AsGRPC(":50051",
    server.WithGRPCTimeout(30*time.Second),
    server.WithGRPCMaxRetries(3),
)
```

## Next Steps

- See [examples](../../examples/README.md) for complete working examples of each transport
- Check [API Reference](../../api-reference/README.md) for detailed configuration options
- Learn about [Advanced Server Features](07-advanced-features.md) for production deployments 