package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/localrivet/gomcp/client"
)

// OpenAIFunction represents an OpenAI function definition
type OpenAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// OpenAITool represents an OpenAI tool definition
type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

func main() {
	fmt.Println("🔗 MCP to OpenAI Proxy Pattern Example")
	fmt.Println("======================================")
	fmt.Println()

	// Check if MCP server URL is provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <mcp-server-url>")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  go run main.go stdio://path/to/mcp-server")
		fmt.Println("  go run main.go ws://localhost:8080/mcp")
		fmt.Println("  go run main.go http://localhost:8080/mcp")
		fmt.Println()
		fmt.Println("💡 This example demonstrates how to:")
		fmt.Println("   1. Connect to any MCP server")
		fmt.Println("   2. Discover available tools using ListTools()")
		fmt.Println("   3. Convert them to OpenAI function format")
		fmt.Println("   4. Use with github.com/openai/openai-go or similar clients")
		os.Exit(1)
	}

	serverURL := os.Args[1]

	// Step 1: Connect to the MCP server
	fmt.Printf("🔍 Connecting to MCP server: %s\n", serverURL)
	mcpClient, err := client.NewClient(serverURL,
		client.WithProtocolVersion("2025-03-26"),
	)
	if err != nil {
		log.Fatalf("Failed to create MCP client: %v", err)
	}
	defer mcpClient.Close()

	// Step 2: Discover available tools using ListTools()
	fmt.Println("📋 Discovering available tools...")
	tools, err := mcpClient.ListTools()
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	if len(tools) == 0 {
		fmt.Println("❌ No tools found on the MCP server")
		return
	}

	fmt.Printf("✅ Found %d tools from MCP server\n\n", len(tools))

	// Step 3: Convert MCP tools to OpenAI function definitions
	fmt.Println("🔄 Converting MCP tools to OpenAI function format...")
	openAITools := convertMCPToolsToOpenAI(tools)

	// Step 4: Display the conversion results
	fmt.Println("📄 OpenAI Tool Definitions:")
	fmt.Println("===========================")
	for i, tool := range openAITools {
		fmt.Printf("\n%d. %s\n", i+1, tool.Function.Name)
		fmt.Printf("   Description: %s\n", tool.Function.Description)

		// Pretty print the parameters schema
		if tool.Function.Parameters != nil {
			parametersJSON, _ := json.MarshalIndent(tool.Function.Parameters, "   ", "  ")
			fmt.Printf("   Parameters:\n   %s\n", string(parametersJSON))
		}
	}

	// Step 5: Show JSON output for easy integration
	fmt.Println("\n📋 Complete JSON for OpenAI Integration:")
	fmt.Println("========================================")
	jsonOutput, err := json.MarshalIndent(openAITools, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal tools to JSON: %v", err)
	}
	fmt.Println(string(jsonOutput))

	// Step 6: Show example usage code
	fmt.Println("\n🤖 Example Integration Code:")
	fmt.Println("============================")
	showExampleCode(tools)

	// Step 7: Demonstrate an actual MCP tool call
	if len(tools) > 0 {
		fmt.Println("\n🧪 Live Demo: Testing MCP tool call...")
		demonstrateToolCall(mcpClient, tools[0])
	}

	fmt.Println("\n✨ Proxy pattern demonstration complete!")
	fmt.Println("💡 You can now use these tool definitions with any OpenAI-compatible client.")
}

// convertMCPToolsToOpenAI converts MCP tool definitions to OpenAI function format
func convertMCPToolsToOpenAI(mcpTools []client.Tool) []OpenAITool {
	var openAITools []OpenAITool

	for _, tool := range mcpTools {
		openAITool := OpenAITool{
			Type: "function",
			Function: OpenAIFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}

		openAITools = append(openAITools, openAITool)
	}

	return openAITools
}

// showExampleCode displays example integration code
func showExampleCode(tools []client.Tool) {
	fmt.Println(`// 1. Install the OpenAI Go client:
// go get github.com/openai/openai-go

// 2. Use the discovered tools with OpenAI:
package main

import (
    "context"
    "encoding/json"
    "log"
    
    "github.com/openai/openai-go"
    "github.com/openai/openai-go/option"
    "github.com/localrivet/gomcp/client"
)

func main() {
    // Initialize OpenAI client
    openaiClient := openai.NewClient(
        option.WithAPIKey("your-openai-api-key"),
    )
    
    // Initialize MCP client (use your actual server URL)
    mcpClient, _ := client.NewClient("your-mcp-server-url")
    defer mcpClient.Close()
    
    // Discover and convert tools
    mcpTools, _ := mcpClient.ListTools()
    openaiTools := convertMCPToolsToOpenAI(mcpTools)
    
    // Use in chat completion
    response, err := openaiClient.Chat.Completions.New(context.Background(), 
        openai.ChatCompletionNewParams{
            Model: openai.F(openai.ChatModelGPT4),
            Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
                openai.UserMessage("Your prompt here"),
            }),
            Tools: openai.F(openaiTools),
        })
    
    // Handle tool calls
    if response.Choices[0].Message.ToolCalls != nil {
        for _, toolCall := range response.Choices[0].Message.ToolCalls {
            if toolCall.Function != nil {
                var args map[string]interface{}
                json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
                
                // Proxy the call to your MCP server
                result, err := mcpClient.CallTool(toolCall.Function.Name, args)
                if err != nil {
                    log.Printf("MCP tool call failed: %v", err)
                    continue
                }
                
                // Use result in next OpenAI message...
            }
        }
    }
}`)

	if len(tools) > 0 {
		fmt.Printf("\n// 3. Example tool call for '%s':\n", tools[0].Name)
		fmt.Printf(`result, err := mcpClient.CallTool("%s", map[string]interface{}{`, tools[0].Name)

		// Show example parameters based on schema
		if tools[0].InputSchema != nil {
			if props, ok := tools[0].InputSchema["properties"].(map[string]interface{}); ok {
				first := true
				for propName := range props {
					if !first {
						fmt.Print(",")
					}
					fmt.Printf("\n    \"%s\": \"example_value\"", propName)
					first = false
				}
			}
		}
		fmt.Println("\n})")
	}
}

// demonstrateToolCall shows a live example of calling an MCP tool
func demonstrateToolCall(mcpClient client.Client, tool client.Tool) {
	fmt.Printf("Testing tool: %s\n", tool.Name)

	// Create example arguments based on the schema
	args := make(map[string]interface{})

	if tool.InputSchema != nil {
		if props, ok := tool.InputSchema["properties"].(map[string]interface{}); ok {
			for propName, propDef := range props {
				if propMap, ok := propDef.(map[string]interface{}); ok {
					if propType, ok := propMap["type"].(string); ok {
						switch propType {
						case "string":
							args[propName] = "example"
						case "number":
							args[propName] = 42.0
						case "boolean":
							args[propName] = true
						case "integer":
							args[propName] = 42
						}
					}
				}
			}
		}
	}

	if len(args) == 0 {
		fmt.Println("⚠️  Tool has no parameters or complex schema - skipping demo call")
		return
	}

	fmt.Printf("Calling with args: %+v\n", args)
	result, err := mcpClient.CallTool(tool.Name, args)
	if err != nil {
		fmt.Printf("❌ Tool call failed: %v\n", err)
	} else {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("✅ MCP Tool Result:\n%s\n", string(resultJSON))
	}
}
