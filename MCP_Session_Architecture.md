# MCP Session Architecture and Project Root Detection

## Overview

This document explains how Model Context Protocol (MCP) sessions work, how project root detection is implemented, and how MCP servers can access session information from clients like Cursor, Claude Desktop, and other MCP-compatible tools.

## How MCP Sessions Are Created

### Client-Side Session Creation

**Important**: The MCP session is **created by the MCP client** (Cursor, Claude Desktop, etc.), not by the MCP server. The server receives this session object and can extract project information from it.

When an MCP client starts a server, it creates a session object containing:

- **`session.roots`** - Workspace/project directories the client wants to expose
- **`session.env`** - Environment variables from the client context  
- **`session.capabilities`** - Features the client supports
- **`session.loggingLevel`** - Logging level set by the client

### How `session.roots` Gets Populated

The `session.roots` array is populated differently by each MCP client:

#### **Cursor IDE:**
- Automatically passes the **currently open workspace root** as `session.roots[0].uri`
- Uses the workspace directory you have open in Cursor
- Happens automatically when Cursor starts the MCP server process

#### **Claude Desktop:**
- Uses the working directory where the MCP server process is started
- Can be configured in Claude Desktop's MCP configuration

#### **Other MCP Clients:**
- Implementation varies by client
- Usually current working directory or configured project paths

## Session Object Structure

```javascript
{
  roots: [
    {
      uri: "file:///Users/username/workspace/project",  // Project root URI
      name: "project"                                    // Optional name
    }
    // Can contain multiple roots
  ],
  env: {
    // Environment variables from the client
    TASK_MASTER_PROJECT_ROOT: "/custom/path",  // If set by user
    DEBUG: "true",
    // ... other environment variables
  },
  capabilities: {
    // Features the client supports
    progress: true,      // Can show progress indicators
    sampling: false,     // Can request LLM completions
    // ... other capabilities
  },
  loggingLevel: "info"   // Logging level preference
}
```

## URI Format and Normalization

The `session.roots[0].uri` contains a **file:// URI** that requires processing:

1. **URI Decoding** - Handle spaces and special characters
2. **Protocol Stripping** - Remove `file://` prefix
3. **Path Normalization** - Handle OS-specific paths (especially Windows `/C:/...`)

### Example URI Processing

```javascript
// Input from session
const rawUri = "file:///Users/username/My%20Project";

// Processing steps
const decoded = decodeURIComponent(rawUri);        // "file:///Users/username/My Project"  
const withoutProtocol = decoded.replace('file://', '');  // "/Users/username/My Project"

// Handle Windows paths (remove leading slash from /C:/...)
const normalized = withoutProtocol.startsWith('/') && /[A-Za-z]:/.test(withoutProtocol.substring(1, 3))
  ? withoutProtocol.substring(1)  // Remove leading slash for Windows
  : withoutProtocol;

const finalPath = path.resolve(normalized);  // OS-normalized absolute path
```

## Accessing the Session Object in MCP Tools

### Basic Pattern

```javascript
server.addTool({
  name: 'example_tool',
  description: 'Shows how to access session data',
  parameters: z.object({
    // tool parameters
  }),
  execute: async (args, { log, session }) => {
    // Access session properties
    const projectRoot = session?.roots?.[0]?.uri;
    const envVars = session?.env;
    const capabilities = session?.capabilities;
    
    log.info(`Project root: ${projectRoot}`);
    log.info(`Environment vars: ${JSON.stringify(envVars)}`);
    
    return "Tool executed with session data";
  }
});
```

### Real-World Example (Task Master)

```javascript
// From Task Master's get-tasks tool
execute: withNormalizedProjectRoot(async (args, { log, session }) => {
  try {
    // Session is used to resolve project paths
    let tasksJsonPath = resolveTasksPath(args, session);
    let complexityReportPath = resolveComplexityReportPath(args, session);
    
    // Tool logic using resolved paths...
    const result = await listTasksDirect({
      tasksJsonPath: tasksJsonPath,
      // ... other parameters
    }, log);
    
    return handleApiResult(result, log, 'Error getting tasks');
  } catch (error) {
    log.error(`Error: ${error.message}`);
    return createErrorResponse(error.message);
  }
})
```

### Context Object Structure

The complete context object passed to MCP tools:

```javascript
{
  log: {           // Logger with methods
    info(msg),     // Log info message
    warn(msg),     // Log warning
    error(msg),    // Log error
    debug(msg)     // Log debug info
  },
  session: {       // MCP session from client
    roots: [...],  // Project roots
    env: {...},    // Environment variables
    capabilities: {...}, // Client capabilities
    loggingLevel: "info"
  }
  // ... other MCP context properties
}
```

## Project Root Detection Strategy

A robust MCP server should implement a hierarchical fallback system:

### 1. Environment Variable Override (Highest Priority)
```javascript
if (process.env.PROJECT_ROOT) {
  return process.env.PROJECT_ROOT;
}
if (session?.env?.PROJECT_ROOT) {
  return session.env.PROJECT_ROOT;  
}
```

### 2. Explicit Parameter
```javascript
if (args.projectRoot) {
  return normalizeProjectRoot(args.projectRoot);
}
```

### 3. Session-Based Detection
```javascript
if (session?.roots?.[0]?.uri) {
  return normalizeUri(session.roots[0].uri);
}
```

### 4. Project Marker Search
```javascript
// Search upward for project indicators
const markers = ['.git', '.taskmaster', 'package.json', 'go.mod'];
return findProjectRoot(process.cwd(), markers);
```

### 5. Current Directory (Fallback)
```javascript
return process.cwd();
```

## Implementation Considerations

### For MCP Server Developers

1. **Always access session through context parameter**
2. **Implement proper URI normalization** for cross-platform compatibility
3. **Use hierarchical fallback** for project root detection
4. **Handle missing session gracefully** (some clients may not provide full session data)
5. **Log session usage** for debugging client integration issues

### For MCP Client Developers

1. **Always populate `session.roots`** with workspace directories
2. **Use proper file:// URI format** for root paths
3. **Include environment variables** in `session.env`
4. **Set appropriate capabilities** in `session.capabilities`
5. **Maintain session consistency** across tool calls

## Testing Session Handling

### Unit Test Example

```javascript
// Test session-based project root detection
const mockSession = {
  roots: [
    { uri: "file:///Users/test/workspace/project" }
  ],
  env: {},
  capabilities: {}
};

const result = await toolFunction(args, { log: mockLog, session: mockSession });
// Assert result uses correct project root
```

### Integration Test

```javascript
// Test with actual MCP client
const client = new MCPClient("path/to/server");
await client.connect();

// Verify server receives session data correctly
const tools = await client.listTools();
const result = await client.callTool("get_project_info", {});
```

## Security Considerations

1. **Validate session data** - Don't trust session content blindly
2. **Sanitize file paths** - Prevent directory traversal attacks  
3. **Respect client boundaries** - Only access paths in `session.roots`
4. **Environment variable safety** - Be cautious with `session.env` data

## Compatibility Notes

- **FastMCP (Python)**: Full session support with automatic normalization
- **FastMCP (TypeScript)**: Complete session handling implementation
- **Official MCP SDKs**: Basic session support, may need manual normalization
- **Custom implementations**: Session support varies

## Conclusion

The MCP session architecture provides a standardized way for clients to communicate workspace context to servers. By properly implementing session handling, MCP servers can automatically detect project roots and provide seamless integration with development tools like Cursor.

The key insight is that **clients create and populate the session** - servers should extract and normalize this information rather than trying to detect project context independently. 