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

## Session Management

- [session-management](session-management.md) - Comprehensive session management and workspace root discovery

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

## Session Management & Workspace Roots Integration

GOMCP v1.5.5+ provides comprehensive session management with the MCP Session Architecture, including automatic workspace root discovery and transport-aware session data.

### Session Context API

Tools receive rich session information through the context:

```go
func MyTool(ctx *server.Context, args struct{}) (interface{}, error) {
    // Access session data (NEW in v1.5.5)
    session := ctx.Session
    
    // Get environment variables from transport
    env := session.Env()
    apiKey := env["API_KEY"]
    
    // Get workspace roots (from init + automated roots/list)
    roots := session.Roots()
    
    // Get client capabilities
    caps := session.Capabilities()
    supportsSampling := caps.Sampling.Supported
    
    // Legacy workspace root methods (still supported)
    allRoots := ctx.GetRoots()
    primaryRoot := ctx.GetPrimaryRoot()
    isInWorkspace := ctx.InRoots("/path/to/file")
    
    return map[string]interface{}{
        "session_env": env,
        "workspace_roots": roots,
        "supports_sampling": supportsSampling,
        "primary_root": primaryRoot,
        "all_roots": allRoots,
        "in_workspace": isInWorkspace,
    }, nil
}
```

### Session Management Features

- **Transport-Aware Environment Extraction**: Automatically extracts environment data from transport layer (stdio process env, HTTP headers, etc.)
- **Automated Root Fetching**: Detects client `roots` capability and automatically sends `roots/list` requests
- **MCP Protocol Compliance**: Full compliance across all three protocol versions (2024-11-05, 2025-03-26, draft)
- **Convenience Methods**: Easy access via `ctx.Session.Env()`, `ctx.Session.Roots()`, `ctx.Session.Capabilities()`

### Workspace Root Discovery Process

1. **Initial Extraction**: Workspace roots from `clientInfo.roots` during initialization
2. **Capability Detection**: Server detects if client advertises `roots` capability
3. **Automated Fetching**: Server sends `roots/list` request after `notifications/initialized`
4. **Response Processing**: Server processes `roots/list` response with proper request tracking
5. **Context Integration**: Roots available immediately via both session and legacy methods

### Legacy Compatibility

All existing workspace root methods continue to work:
- `ctx.GetRoots()` - Returns all workspace roots
- `ctx.GetPrimaryRoot()` - Returns primary workspace root  
- `ctx.InRoots(path)` - Checks if path is within workspace

### Features

- Automatic extraction of workspace roots from MCP client initialization AND automated `roots/list` requests
- Transport-aware session data extraction (environment, capabilities, etc.)
- Thread-safe access to workspace and session context
- Convenient helper methods for path validation and session data access
- No manual configuration required
- Full backward compatibility with existing code

## Generating Documentation

API documentation is automatically generated from source code comments. For local documentation:

```bash
go install golang.org/x/tools/cmd/godoc@latest
godoc -http=:6060
```

Then visit [http://localhost:6060/pkg/github.com/localrivet/gomcp/](http://localhost:6060/pkg/github.com/localrivet/gomcp/).
