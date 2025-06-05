# ServerRegistry Management Example

This example demonstrates comprehensive usage of the `ServerRegistry` for managing multiple MCP servers in a single application. It showcases the robust process management improvements, concurrent operations, error handling, and proper cleanup mechanisms.

## Overview

The ServerRegistry provides centralized management for multiple MCP server processes, with features including:

- **Process Lifecycle Management**: Start, stop, and monitor server processes
- **Robust Process Cleanup**: Graceful shutdown with escalation to force-kill
- **Concurrent Operations**: Handle multiple servers simultaneously  
- **Error Recovery**: Detect and handle server failures gracefully
- **Resource Management**: Automatic cleanup on application exit
- **Health Monitoring**: Connection health checking and readiness testing

## Example Architecture

This demo creates a system with three different MCP servers:

1. **Math Server**: Provides mathematical operations (add, multiply, factorial)
2. **Text Server**: Offers text processing tools (uppercase, reverse, word count)
3. **Slow Server**: Simulates long-running operations for timeout testing

## Running the Example

### Full Demonstration
```bash
go run main.go demo
```

This runs a comprehensive 7-phase demonstration:

1. **Server Startup**: Launches all three servers concurrently
2. **Readiness Check**: Waits for servers to be ready using `WaitForReady()`
3. **Capability Discovery**: Explores each server's tools and capabilities
4. **Operation Testing**: Calls tools on each server with different parameters
5. **Management Demo**: Shows server lifecycle operations (start/stop individual servers)
6. **Error Handling**: Demonstrates graceful error handling for various failure scenarios
7. **Concurrent Operations**: Tests thread-safe concurrent access across all servers

### Individual Server Modes

You can also run individual servers for testing:

```bash
# Run specific servers individually
go run main.go math-server
go run main.go text-server  
go run main.go slow-server
```

## Key Features Demonstrated

### 1. Multi-Server Management
```go
// Define multiple server configurations
servers := map[string]client.ServerDefinition{
    "math-server": {
        Command: execPath,
        Args:    []string{"math-server"},
        Env: map[string]string{
            "SERVER_NAME": "Math Operations Server",
        },
    },
    "text-server": {
        Command: execPath,
        Args:    []string{"text-server"},
        // ... more config
    },
}

// Start all servers
for name, def := range servers {
    registry.StartServer(name, def)
}
```

### 2. Health Checking & Readiness
```go
client, _ := registry.GetClient("math-server")

// Wait for server to be fully ready
if err := client.WaitForReady(10 * time.Second); err != nil {
    log.Printf("Server not ready: %v", err)
}

// Server is now ready for operations
tools, _ := client.ListTools()
```

### 3. Robust Cleanup
```go
// Automatic cleanup on exit (even with panics)
defer func() {
    if err := registry.Close(); err != nil {
        fmt.Printf("Error during cleanup: %v\n", err)
    }
}()
```

### 4. Error Handling & Recovery
```go
// Test invalid server configuration
badDef := client.ServerDefinition{
    Command: "/nonexistent/command",
    Args:    []string{},
}

// Registry handles errors gracefully
if err := registry.StartServer("bad-server", badDef); err != nil {
    fmt.Printf("Expected error: %v\n", err)
}
```

### 5. Concurrent Operations
```go
// Safe concurrent access across multiple servers
for name, client := range readyServers {
    go func(serverName string, c client.Client) {
        tools, _ := c.ListTools()
        c.Ping()
        // All operations are thread-safe
    }(name, client)
}
```

## Process Management Improvements

This example showcases the enhanced process management features added to ServerRegistry:

### Graceful Termination
- **Step 1**: Close stdin pipe to signal MCP server to shutdown gracefully
- **Step 2**: Wait up to 3 seconds for graceful shutdown
- **Step 3**: Send SIGKILL if process doesn't terminate gracefully
- **Step 4**: Wait for process cleanup with timeout

### Resource Cleanup
- Automatically cleans up all processes on registry Close()
- Handles orphaned processes and zombie process prevention
- Cross-platform support (Unix signals + Windows process termination)
- Proper file descriptor and pipe cleanup

### Error Recovery
- Detects and handles server crashes
- Prevents duplicate server registration
- Graceful handling of communication failures
- Automatic resource cleanup on failures

## Sample Output

```
=== ServerRegistry Comprehensive Demo ===

=== Phase 1: Starting Multiple Servers ===
Starting math-server...
✓ math-server started successfully
Starting text-server...
✓ text-server started successfully
Starting slow-server...
✓ slow-server started successfully

=== Phase 2: Waiting for Server Readiness ===
Waiting for math-server to be ready...
✓ math-server is ready!
Waiting for text-server to be ready...
✓ text-server is ready!
Waiting for slow-server to be ready...
✓ slow-server is ready!

=== Phase 3: Server Discovery ===

--- Math-Server Capabilities ---
  Server: math-server v1.0.0
  Tools (3):
    - add: Add two numbers
    - multiply: Multiply two numbers  
    - factorial: Calculate factorial
  Supports: tools=true resources=false prompts=false

--- Text-Server Capabilities ---
  Server: text-server v1.0.0
  Tools (3):
    - uppercase: Convert text to uppercase
    - reverse: Reverse text
    - count_words: Count words in text
  Supports: tools=true resources=false prompts=false

=== Phase 4: Testing Server Operations ===

--- Testing Math Server ---
  add[a:15 b:25] = map[result:40]
  multiply[a:7 b:8] = map[result:56]
  factorial[n:5] = map[result:120]

--- Testing Text Server ---
  uppercase(hello world) = map[result:HELLO WORLD]
  reverse(MCP Demo) = map[result:omeD PCM]
  count_words(This is a test sentence) = map[word_count:5 words:[This is a test sentence]]

=== Phase 5: Server Management Demo ===
Active servers: [math-server slow-server text-server]
Stopping slow-server individually...
✓ slow-server stopped successfully
Attempting to use stopped server...
✓ Expected error: server slow-server not found

=== Phase 6: Error Handling Demo ===
Testing error handling with invalid server...
✓ Expected error caught: failed to start command: exec: "/nonexistent/command": file does not exist

=== Phase 7: Concurrent Operations Test ===
Running concurrent operations across all servers...
  math-server-list: OK (3 tools)
  text-server-list: OK (3 tools)
  math-server-ping: OK
  text-server-ping: OK
✓ All concurrent operations completed

=== Demo Complete ===
All servers will be automatically cleaned up...

=== Cleaning Up All Servers ===
All servers cleaned up successfully!
```

## Integration Patterns

### Configuration-Based Setup
The ServerRegistry can load server definitions from JSON configuration files:

```go
registry := client.NewServerRegistry()
err := registry.LoadConfig("servers.json")
```

### Custom Logger Integration
Configure detailed logging for debugging:

```go
registry := client.NewServerRegistry(
    client.WithRegistryLogger(
        client.NewLogger(
            client.WithLogLevel(slog.LevelDebug),
            client.WithLogFile("server_registry.log"),
        ),
    ),
)
```

### Service Discovery
Use the registry for dynamic service discovery:

```go
serverNames, _ := registry.GetServerNames()
for _, name := range serverNames {
    client, _ := registry.GetClient(name)
    tools, _ := client.ListTools()
    // Build service catalog
}
```

## Best Practices

1. **Always Use Defer for Cleanup**: Ensure `registry.Close()` is called even if the application panics
2. **Check Server Readiness**: Use `WaitForReady()` before making API calls to ensure servers are responsive
3. **Handle Errors Gracefully**: Server startup can fail for various reasons - implement proper error handling
4. **Use Timeouts**: Configure appropriate timeouts for slow operations
5. **Concurrent Safety**: All ServerRegistry operations are thread-safe and can be called concurrently
6. **Resource Monitoring**: Monitor server health and restart failed servers as needed
7. **Logging Configuration**: Use appropriate log levels and avoid stdout/stderr for stdio-based servers

## Troubleshooting

### Common Issues

**Server Won't Start**
- Check that the command path is correct and executable
- Verify environment variables are set properly  
- Check file permissions and system limits

**Connection Timeouts**
- Increase readiness timeout with `WaitForReady(longerTimeout)`
- Check server logs for initialization issues
- Verify stdio pipes aren't blocked

**Resource Leaks**
- Always call `registry.Close()` on shutdown
- Monitor process counts during long-running operations
- Check for proper cleanup in error scenarios

**Concurrent Access Issues**
- All operations are thread-safe by design
- Use proper error handling in goroutines
- Avoid sharing client instances across uncontrolled goroutines

This example demonstrates production-ready patterns for managing multiple MCP servers in real applications, showcasing the robustness and reliability improvements made to the gomcp ServerRegistry. 