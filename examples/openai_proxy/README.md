# MCP to OpenAI Proxy Pattern

This example demonstrates how to bridge **MCP servers** with **OpenAI Go client library** (`github.com/openai/openai-go`) using the proxy pattern.

## The Problem

Popular LLM client libraries like OpenAI's Go client require you to manually define available functions:

```go
// You have to manually define each function like this:
tools := []openai.ChatCompletionToolParam{
    {
        Type: openai.F(openai.ChatCompletionToolTypeFunction),
        Function: openai.F(openai.FunctionDefinitionParam{
            Name:        openai.F("calculator"),
            Description: openai.F("Perform mathematical calculations"),
            Parameters: openai.F(map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "operation": map[string]interface{}{
                        "type": "string",
                        "enum": []string{"add", "subtract", "multiply", "divide"},
                    },
                    "a": map[string]interface{}{"type": "number"},
                    "b": map[string]interface{}{"type": "number"},
                },
                "required": []string{"operation", "a", "b"},
            }),
        }),
    },
    // ... manually define every other function
}
```

But **MCP servers already expose this information** via the `tools/list` endpoint! 

## The Solution: Proxy Pattern

Instead of manually defining functions, this example shows how to:

1. **Automatically discover** tools from any MCP server using `client.ListTools()`
2. **Convert** MCP tool definitions to OpenAI function format
3. **Proxy** function calls from OpenAI back to the MCP server

## Usage

```bash
# Run against any MCP server
go run main.go <mcp-server-url>

# Examples:
go run main.go stdio://path/to/mcp-server
go run main.go ws://localhost:8080/mcp
go run main.go http://localhost:8080/mcp
```

## What It Does

1. **Connects** to your MCP server
2. **Discovers** all available tools using `ListTools()`
3. **Converts** them to OpenAI-compatible format
4. **Shows** the exact JSON you need for OpenAI integration
5. **Demonstrates** a live tool call to verify everything works

## Example Output

```
🔗 MCP to OpenAI Proxy Pattern Example
======================================

🔍 Connecting to MCP server: ws://localhost:8080/mcp
📋 Discovering available tools...
✅ Found 3 tools from MCP server

📄 OpenAI Tool Definitions:
===========================

1. calculator
   Description: Perform mathematical calculations
   Parameters:
   {
     "type": "object",
     "properties": {
       "operation": {
         "type": "string",
         "enum": ["add", "subtract", "multiply", "divide"]
       },
       "a": { "type": "number" },
       "b": { "type": "number" }
     },
     "required": ["operation", "a", "b"]
   }

📋 Complete JSON for OpenAI Integration:
========================================
[
  {
    "type": "function",
    "function": {
      "name": "calculator",
      "description": "Perform mathematical calculations",
      "parameters": {
        "type": "object",
        "properties": {
          "operation": {
            "type": "string",
            "enum": ["add", "subtract", "multiply", "divide"]
          },
          "a": { "type": "number" },
          "b": { "type": "number" }
        },
        "required": ["operation", "a", "b"]
      }
    }
  }
]
```

## Integration Code

The example shows you exactly how to integrate with `github.com/openai/openai-go`:

```go
// 1. Install the OpenAI Go client
// go get github.com/openai/openai-go

// 2. Discover tools from your MCP server
mcpClient, _ := client.NewClient("your-mcp-server-url")
mcpTools, _ := mcpClient.ListTools()
openaiTools := convertMCPToolsToOpenAI(mcpTools)

// 3. Use with OpenAI
openaiClient := openai.NewClient(option.WithAPIKey("your-api-key"))
response, err := openaiClient.Chat.Completions.New(context.Background(), 
    openai.ChatCompletionNewParams{
        Model: openai.F(openai.ChatModelGPT4),
        Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
            openai.UserMessage("Calculate 15 + 27"),
        }),
        Tools: openai.F(openaiTools), // Use discovered MCP tools!
    })

// 4. When OpenAI calls a function, proxy it to MCP
if response.Choices[0].Message.ToolCalls != nil {
    for _, toolCall := range response.Choices[0].Message.ToolCalls {
        if toolCall.Function != nil {
            var args map[string]interface{}
            json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
            
            // Proxy the call to your MCP server
            result, err := mcpClient.CallTool(toolCall.Function.Name, args)
            // ... use result in next OpenAI message
        }
    }
}
```

## Benefits

✅ **No manual function definitions** - tools are discovered automatically  
✅ **Always up-to-date** - new tools are discovered automatically  
✅ **Works with any MCP server** - not tied to specific implementations  
✅ **Type-safe schemas** - JSON Schema validation from MCP  
✅ **Easy integration** - drop-in replacement for manual definitions  

## Use Cases

This pattern is perfect for:

- **Building AI assistants** that can use multiple MCP servers
- **Creating tool marketplaces** where tools are discovered dynamically  
- **Integrating existing MCP servers** with OpenAI-based applications
- **Rapid prototyping** without manually defining every function
- **Enterprise applications** where tools change frequently

## Related

- [MCP Tools Documentation](https://modelcontextprotocol.io/docs/concepts/tools)
- [OpenAI Go Client](https://github.com/openai/openai-go)
- [GitHub Issue #5: Tool Discovery](https://github.com/localrivet/gomcp/issues/5)

---

**💡 This example solves the exact problem described in the MCP documentation**: bridging the gap between MCP's dynamic tool discovery and LLM clients that expect static function definitions. 