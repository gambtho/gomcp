# Session Management

GoMCP v1.5.5 introduces comprehensive session management through the **MCP Session Architecture**, providing rich context and automated workspace discovery for server-side tools.

## Overview

The MCP Session Architecture automatically extracts and provides session data including:

- **Environment variables** from transport layers (not initialization parameters)
- **Workspace roots** from both `clientInfo` initialization AND automated `roots/list` requests
- **Client capabilities** including sampling, audio support, and protocol version information

## Session Context API

### Accessing Session Data

```go
srv.Tool("workspace_analyzer", "Analyze workspace with full context", func(ctx *server.Context, args struct {
    AnalysisType string `json:"analysis_type"`
    Deep         bool   `json:"deep,omitempty"`
}) (interface{}, error) {
    // Access the session
    session := ctx.Session
    
    // Get environment variables (from transport)
    env := session.Env()
    openaiKey := env["OPENAI_API_KEY"]
    githubToken := env["GITHUB_TOKEN"]
    
    // Get workspace roots (from init + automated roots/list)
    roots := session.Roots()
    primaryRoot := ""
    if len(roots) > 0 {
        primaryRoot = roots[0]
    }
    
    // Get client capabilities
    caps := session.Capabilities()
    
    return map[string]interface{}{
        "analysis_type": args.AnalysisType,
        "primary_workspace": primaryRoot,
        "all_workspaces": roots,
        "has_openai_access": openaiKey != "",
        "has_github_access": githubToken != "",
        "supports_sampling": caps.Sampling.Supported,
        "supports_audio": caps.Audio.Supported,
        "protocol_version": caps.ProtocolVersion,
        "deep_analysis": args.Deep,
    }, nil
})
```

### Session Interface

The `ClientSession` interface provides three main methods:

```go
type ClientSession interface {
    // Env returns environment variables from transport layer
    Env() map[string]string
    
    // Roots returns workspace root paths from init + automated roots/list
    Roots() []string
    
    // Capabilities returns client capability information
    Capabilities() *ClientInfo
}
```

## Transport-Aware Environment Extraction

Environment data is automatically extracted based on the transport type:

### stdio Transport

Environment variables come from the server process environment:

```bash
OPENAI_API_KEY=sk-xxx GITHUB_TOKEN=ghp_xxx ./my-mcp-server
```

```go
// In tool handler
env := ctx.Session.Env()
apiKey := env["OPENAI_API_KEY"]  // From process env
```

### HTTP Transport

Environment variables come from request headers using `X-Env-*` pattern:

```http
POST /mcp HTTP/1.1
X-Env-OPENAI_API_KEY: sk-xxx
X-Env-GITHUB_TOKEN: ghp_xxx
Content-Type: application/json

{"jsonrpc": "2.0", "method": "initialize", ...}
```

```go
// In tool handler
env := ctx.Session.Env()
apiKey := env["OPENAI_API_KEY"]  // From X-Env-OPENAI_API_KEY header
```

### WebSocket Transport

Environment variables come from connection headers during WebSocket handshake:

```javascript
const ws = new WebSocket('ws://localhost:8080/mcp', [], {
    headers: {
        'X-Env-OPENAI_API_KEY': 'sk-xxx',
        'X-Env-GITHUB_TOKEN': 'ghp_xxx'
    }
});
```

### SSE Transport

Environment variables come from initial request headers:

```http
GET /mcp HTTP/1.1
Accept: text/event-stream
X-Env-OPENAI_API_KEY: sk-xxx
X-Env-GITHUB_TOKEN: ghp_xxx
```

## Automated Workspace Root Discovery

The server implements a sophisticated root discovery process:

### 1. Initial Extraction

During client initialization, workspace roots are extracted from `clientInfo.roots`:

```json
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-03-26",
    "clientInfo": {
      "name": "my-client",
      "version": "1.0.0",
      "roots": [
        {"uri": "file:///workspace/project1"},
        {"uri": "file:///workspace/project2"}
      ]
    }
  }
}
```

### 2. Capability Detection

Server detects if client supports the `roots` capability:

```json
{
  "jsonrpc": "2.0",
  "method": "initialize", 
  "params": {
    "capabilities": {
      "roots": {
        "listChanged": true
      }
    }
  }
}
```

### 3. Automated Fetching

After receiving `notifications/initialized`, server automatically sends `roots/list`:

```json
{
  "jsonrpc": "2.0",
  "id": "roots-request-1",
  "method": "roots/list"
}
```

### 4. Response Processing

Server processes the `roots/list` response and updates session context:

```json
{
  "jsonrpc": "2.0",
  "id": "roots-request-1",
  "result": {
    "roots": [
      {"uri": "file:///workspace/project1", "name": "Main Project"},
      {"uri": "file:///workspace/project2", "name": "Dependencies"},
      {"uri": "file:///workspace/project3", "name": "Additional Workspace"}
    ]
  }
}
```

### 5. Context Integration

All workspace roots are immediately available via `ctx.Session.Roots()`.

## MCP Protocol Compliance

Full compliance across all three MCP protocol versions:

### 2024-11-05
- Basic root extraction from `clientInfo.roots`
- Capability detection for `roots` support
- Automated `roots/list` requests
- Environment extraction per transport specification

### 2025-03-26  
- Enhanced session management
- Audio capability detection (`caps.Audio.Supported`)
- Advanced sampling capabilities
- Streamable HTTP transport support

### draft
- Latest session architecture features
- Full protocol compliance with newest specifications
- Enhanced capability negotiation

## ClientInfo Structure

The enhanced `ClientInfo` structure provides comprehensive session data:

```go
type ClientInfo struct {
    Name              string                 `json:"name"`                    // Client name
    Version           string                 `json:"version"`                 // Client version
    SamplingSupported bool                   `json:"sampling_supported"`      // Basic sampling support
    SamplingCaps      *SamplingCapabilities  `json:"sampling_caps,omitempty"` // Detailed sampling capabilities
    ProtocolVersion   string                 `json:"protocol_version"`        // MCP protocol version
    Env               map[string]string      `json:"env,omitempty"`           // Environment from transport
    Roots             []string               `json:"roots,omitempty"`         // Workspace roots from init + roots/list
}

type SamplingCapabilities struct {
    Supported bool `json:"supported"`
    Audio     struct {
        Supported bool `json:"supported"`
    } `json:"audio"`
}
```

## Backward Compatibility

### Legacy Context Methods

All existing workspace root methods continue to work unchanged:

```go
// Legacy methods (still supported)
allRoots := ctx.GetRoots()
primaryRoot := ctx.GetPrimaryRoot()
isInWorkspace := ctx.InRoots("/path/to/file")

// These methods return the same data as the new session methods
sessionRoots := ctx.Session.Roots()
// allRoots == sessionRoots (same data, different access method)
```

### Migration Path

Existing code requires no changes. New session methods provide additional functionality:

```go
// Before v1.5.5
func MyTool(ctx *server.Context, args MyArgs) (interface{}, error) {
    roots := ctx.GetRoots()
    // No access to environment or capabilities
    return map[string]interface{}{"roots": roots}, nil
}

// After v1.5.5 (enhanced, but old code still works)
func MyTool(ctx *server.Context, args MyArgs) (interface{}, error) {
    // Old method still works
    roots := ctx.GetRoots()
    
    // New session methods provide additional data
    env := ctx.Session.Env()
    caps := ctx.Session.Capabilities()
    
    return map[string]interface{}{
        "roots": roots,
        "environment": env,
        "capabilities": caps,
    }, nil
}
```

## Benefits

### Zero Configuration
- Automatic session data extraction with no manual setup required
- Works out of the box with any MCP-compliant client
- No breaking changes to existing tool handlers

### MCP Compliant
- Follows official MCP specification for session handling
- Proper capability negotiation and detection
- Correct environment data sourcing from transport layer

### Transport Agnostic
- Works consistently across all transport types (stdio, HTTP, WebSocket, SSE)
- Automatically adapts environment extraction to transport capabilities
- Uniform API regardless of underlying transport

### Rich Context
- Environment variables for API keys and configuration
- Workspace roots for file system operations
- Client capabilities for feature detection
- Protocol version information for compatibility

## Examples

### File System Tool with Session Context

```go
srv.Tool("read_file", "Read file with workspace context", func(ctx *server.Context, args struct {
    Path string `json:"path"`
}) (interface{}, error) {
    // Check if path is in workspace
    roots := ctx.Session.Roots()
    inWorkspace := ctx.InRoots(args.Path)
    
    if !inWorkspace {
        return nil, fmt.Errorf("path %s is not in workspace roots: %v", args.Path, roots)
    }
    
    // Use environment for additional context
    env := ctx.Session.Env()
    encoding := env["FILE_ENCODING"]
    if encoding == "" {
        encoding = "utf-8"
    }
    
    // Read file logic...
    content := fmt.Sprintf("File content for %s (encoding: %s)", args.Path, encoding)
    
    return map[string]interface{}{
        "path": args.Path,
        "content": content,
        "encoding": encoding,
        "workspace_roots": roots,
    }, nil
})
```

### API Integration Tool

```go
srv.Tool("call_openai", "Call OpenAI API with session credentials", func(ctx *server.Context, args struct {
    Prompt string `json:"prompt"`
    Model  string `json:"model,omitempty"`
}) (interface{}, error) {
    // Get API key from session environment
    env := ctx.Session.Env()
    apiKey := env["OPENAI_API_KEY"]
    
    if apiKey == "" {
        return nil, fmt.Errorf("OPENAI_API_KEY not found in session environment")
    }
    
    // Check client capabilities
    caps := ctx.Session.Capabilities()
    if !caps.Sampling.Supported {
        return nil, fmt.Errorf("client does not support sampling")
    }
    
    // Default model based on capabilities
    model := args.Model
    if model == "" {
        if caps.Audio.Supported {
            model = "gpt-4-audio-preview"
        } else {
            model = "gpt-4"
        }
    }
    
    // Call OpenAI API (mock implementation)
    response := fmt.Sprintf("OpenAI response to '%s' using model %s", args.Prompt, model)
    
    return map[string]interface{}{
        "response": response,
        "model": model,
        "supports_audio": caps.Audio.Supported,
    }, nil
})
``` 