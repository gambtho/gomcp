# Stdio Transport Example

This example demonstrates both server and client implementations using the stdio transport in gomcp.

## Overview

The stdio transport allows communication between MCP clients and servers using standard input/output streams. This is commonly used when the server runs as a subprocess of the client application.

## Features

This example includes:

### Server Features
- **Echo Tool**: Echoes back any message you send
- **Greet Tool**: Generates personalized greeting messages  
- **Timestamp Tool**: Returns current timestamp with optional custom formatting

### Client Features
- **ServerRegistry**: Demonstrates process management for MCP servers
- **Tool Discovery**: Lists available tools from the server
- **Tool Execution**: Calls all available tools with different parameters
- **Server Information**: Shows server capabilities and metadata
- **Automatic Cleanup**: Properly terminates server processes when done

## Usage

### Run as Server Only
```bash
go run stdio_example.go
```

The server will listen on stdin/stdout for MCP protocol messages.

### Run as Client (with managed server)
```bash
go run stdio_example.go client
```

This will:
1. Start the server as a subprocess
2. Connect to it via stdio transport
3. Demonstrate various MCP operations
4. Automatically clean up the server process

## Key Implementation Details

### Process Management
The client uses `ServerRegistry` to manage the server process lifecycle:
- Spawns the server as a subprocess
- Handles graceful shutdown and cleanup
- Prevents orphaned processes

### Transport Configuration
```go
serverDef := client.ServerDefinition{
    Command: os.Args[0], // Path to this same executable
    Args:    []string{}, // Run without "client" arg = server mode
}
```

### Error Handling
The example demonstrates proper error handling for:
- Server startup failures
- Connection timeouts
- Tool call errors
- Process cleanup

## Server Logs

When running as client, you'll see server logs on stderr like:
```
Server received: Hello from the client!
Generated greeting for: Alice
```

These don't interfere with the JSON-RPC communication on stdout.

## Testing

You can test the server manually by running it and sending JSON-RPC requests:

```bash
# Terminal 1 - Start server
go run stdio_example.go

# Terminal 2 - Send test request
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"echo","arguments":{"message":"test"}},"id":1}' | go run stdio_example.go
```

## Architecture Benefits

This example showcases:
- **Robust process management** - No orphaned processes
- **Clean separation** - Server and client in same binary
- **Production patterns** - Proper error handling and cleanup
- **MCP protocol compliance** - Full protocol support 