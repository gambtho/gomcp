# MCP Tool Discovery Example

This example demonstrates how to use the `client.ListTools()` method to discover available tools from an MCP server. This functionality is essential for implementing proxy patterns where LLM client libraries need to dynamically discover and use MCP tools.

## Overview

The Model Context Protocol (MCP) specifies a `tools/list` endpoint that servers should expose to allow clients to discover available tools. This example shows:

1. **Server Setup**: Creating an MCP server with multiple tools
2. **Tool Discovery**: Using `client.ListTools()` to discover available tools
3. **Schema Inspection**: Examining tool schemas for parameter requirements
4. **LLM Integration**: Converting MCP tool definitions to OpenAI-compatible function definitions

## Use Case: LLM Client Integration

This functionality addresses the issue described in [GitHub Issue #5](https://github.com/localrivet/gomcp/issues/5), where LLM client libraries (like OpenAI's Go client or Ollama's Python library) need to:

1. **Discover Tools**: Automatically find what tools are available from an MCP server
2. **Extract Schemas**: Get the parameter schemas for each tool
3. **Convert Formats**: Transform MCP tool definitions into the format expected by their LLM API

## Key Features

### Single Method API
Following Go's principle of "one way to do things," the client provides a single `ListTools()` method that:
- Automatically handles pagination internally
- Returns all available tools in one call
- Provides complete tool information including schemas

### Rich Tool Information
Each discovered tool includes:
- **Name**: The tool identifier
- **Description**: Human-readable description of what the tool does
- **Input Schema**: JSON Schema describing expected parameters
- **Output Schema**: JSON Schema describing expected output structure (draft spec only)
- **Annotations**: Optional metadata about the tool

## Running the Example

```bash
# Build the example
go build ./examples/list_tools

# Run it
./list_tools
```

## Example Output

```
🔧 MCP Tool Discovery Example
=============================

📋 Discovering available tools...

Found 4 tools:

1. 🛠️  calculator
   📝 Description: Perform basic mathematical operations
   📋 Input Schema:
      Type: object
      Parameters:
        • operation (string) - The operation to perform (add, subtract, multiply, divide)
        • a (number) - First number
        • b (number) - Second number
      Required: operation, a, b

2. 🛠️  echo
   📝 Description: Echo back the provided text
   📋 Input Schema:
      Type: object
      Parameters:
        • text (string) - The text to echo back
      Required: text

...

📄 Example: Converting to OpenAI Function Definitions

// calculator
{
  "name": "calculator",
  "description": "Perform basic mathematical operations",
  "parameters": {
    "type": "object",
    "properties": {
      "operation": {
        "type": "string",
        "description": "The operation to perform (add, subtract, multiply, divide)"
      },
      "a": {
        "type": "number", 
        "description": "First number"
      },
      "b": {
        "type": "number",
        "description": "Second number"
      }
    },
    "required": ["operation", "a", "b"]
  }
}
```

## Integration with LLM Libraries

The discovered tools can be easily converted to work with popular LLM libraries:

### OpenAI Go Client
```go
tools, err := client.ListTools()
if err != nil {
    return err
}

var openAIFunctions []openai.FunctionDefinition
for _, tool := range tools {
    openAIFunctions = append(openAIFunctions, openai.FunctionDefinition{
        Name:        tool.Name,
        Description: tool.Description,
        Parameters:  tool.InputSchema,
    })
}
```

### Ollama Python Library
```python
# Convert MCP tools to Ollama function format
tools = client.list_tools()
ollama_tools = []
for tool in tools:
    ollama_tools.append({
        'type': 'function',
        'function': {
            'name': tool['name'],
            'description': tool['description'],
            'parameters': tool['inputSchema']
        }
    })
```

## Benefits

1. **Dynamic Discovery**: No need to hardcode tool definitions
2. **Automatic Updates**: New tools are discovered automatically
3. **Type Safety**: Rich schema information enables proper validation
4. **Proxy Patterns**: Enables building bridges between MCP and LLM APIs
5. **Standardization**: Uses the official MCP `tools/list` endpoint

## Related

- [MCP Tools Documentation](https://modelcontextprotocol.io/docs/concepts/tools)
- [GitHub Issue #5: Tool Discovery](https://github.com/localrivet/gomcp/issues/5)
- [MCP Specification](https://modelcontextprotocol.io/docs/specification) 