# API Reference

This section provides comprehensive documentation for the GOMCP API.

## Root Package

- [gomcp](gomcp.md) - Core package with version information and entry points

## Main Packages

- [client](client.md) - Client-side implementation of the MCP protocol
- [server](server.md) - Server-side implementation of the MCP protocol
- [mcp](mcp.md) - Core protocol types and version handling
- [events](events.md) - Comprehensive event system for monitoring and observability

## Transport Packages

- [transport](transport.md) - Transport interface and common utilities
- [transport/stdio](transport-stdio.md) - Standard I/O transport
- [transport/ws](transport-ws.md) - WebSocket transport
- [transport/sse](transport-sse.md) - Server-Sent Events transport
- [transport/http](transport-http.md) - HTTP transport

## Utility Packages

- [util/schema](util-schema.md) - JSON Schema generation and validation
- [util/conversion](util-conversion.md) - Type conversion utilities

## API Structure

### Client API

The Client API provides methods for consuming MCP services:

```go
// Create a client
client, err := client.NewClient("ws://localhost:8080/mcp")

// Call a tool
result, err := client.CallTool("toolName", args)

// Get a resource
resource, err := client.GetResource("/path/to/resource")

// Get a prompt
prompt, err := client.GetPrompt("promptName", variables)
```

### Server API

The Server API provides a fluent interface for creating MCP servers:

```go
// Create a server
server := server.NewServer("serverName").AsStdio()

// Add a tool
server.Tool("toolName", "description", toolHandler)

// Add a resource
server.Resource("/path/to/resource", "description", resourceHandler)

// Add a prompt
server.Prompt("promptName", "description", template)

// Start the server
server.Run()
```

## Workspace Roots Integration

GOMCP servers automatically integrate with the MCP roots protocol to provide workspace context to tools. This eliminates the need for manual project_root parameters.

### Context API

Tools receive workspace information through the context:

```go
func MyTool(ctx *server.Context, args struct{}) (interface{}, error) {
    // Get all workspace roots
    roots := ctx.GetRoots()
    
    // Get primary workspace root
    primaryRoot := ctx.GetPrimaryRoot()
    
    // Check if path is within workspace
    isInWorkspace := ctx.InRoots("/path/to/file")
    
    return map[string]interface{}{
        "primary_root": primaryRoot,
        "all_roots": roots,
        "in_workspace": isInWorkspace,
    }, nil
}
```

### Features

- Automatic extraction of workspace roots from MCP client initialization
- Thread-safe access to workspace context
- Convenient helper methods for path validation
- No manual configuration required

## Generating Documentation

API documentation is automatically generated from source code comments. For local documentation:

```bash
go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:6060
```

Then visit [http://localhost:6060/pkg/github.com/localrivet/gomcp/](http://localhost:6060/pkg/github.com/localrivet/gomcp/).
