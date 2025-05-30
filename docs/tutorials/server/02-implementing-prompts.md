# Implementing Prompts

This tutorial covers how to implement prompts in your GOMCP server. Prompts are reusable templates for LLM interactions that can include variables and multiple message roles.

## What are Prompts?

Prompts in MCP are named, reusable templates that can be rendered with provided arguments. They support:

- **Variable substitution** using `{{variable}}` syntax
- **Multiple message roles** (user, assistant)
- **Automatic argument extraction** from template variables
- **Rich content types** (text, images, resources)

## Basic Prompt Registration

### Simple Text Prompt

The simplest way to register a prompt is with a plain text template:

```go
s.Prompt("greeting", "A simple greeting prompt", 
    "Hello! How can I help you today?")
```

### Prompt with Variables

Use `{{variable}}` syntax for dynamic content:

```go
s.Prompt("personalized-greeting", "A personalized greeting",
    "Hello {{name}}! Welcome to {{service}}. How can I help you today?")
```

The server automatically:
- Extracts variable names (`name`, `service`)
- Creates argument definitions
- Validates required arguments when the prompt is called

## Multi-Role Prompts

Use helper functions to create prompts with different message roles:

```go
s.Prompt("code-review", "Review code for quality and improvements",
    server.User("Please review this {{language}} code:\n\n{{code}}"),
    server.Assistant("I'll analyze the code for quality, security, and best practices."),
)
```

### Available Role Functions

- **`server.User(content)`** - User messages  
- **`server.Assistant(content)`** - Assistant responses/examples

**Note:** The MCP specification only supports `"user"` and `"assistant"` roles. System messages are not supported.

## Advanced Prompt Patterns

### Workflow Prompts

Create structured workflows with detailed instructions:

```go
s.Prompt("debugging-workflow", "Step-by-step debugging assistance",
    server.User(`As a debugging expert, help me debug this {{issue_type}} issue in {{technology}}:

## 🔍 STEP 1: Problem Analysis
- Analyze the error: {{error_message}}
- Review the context: {{context}}

## 🛠️ STEP 2: Investigation
- Check common causes for {{issue_type}} issues
- Examine the relevant code/configuration

## ✅ STEP 3: Solution
- Provide specific fix recommendations
- Include code examples if applicable

## 🧪 STEP 4: Verification
- Suggest testing steps
- Recommend prevention measures

Issue Details:
{{description}}`),
    server.Assistant("I'll help you systematically debug this issue using the workflow you've outlined."),
)
```

### Instructional Prompts

Embed instructions directly in user messages:

```go
s.Prompt("task-assistant", "Adaptive task assistance",
    server.User(`You are a helpful assistant. Help me with {{task_type}} tasks.
{{#if priority}}This is a {{priority}} priority task.{{/if}}
{{#if deadline}}Deadline: {{deadline}}{{/if}}

Task: {{description}}`),
    server.Assistant("I'll help you complete this task efficiently."),
)
```

## Dynamic Prompts with Functions

For complex logic, use function-based prompts:

```go
s.Prompt("dynamic-prompt", "A prompt with dynamic content", 
    func(ctx *server.Context, args struct {
        Action   string `json:"action"`
        Context  string `json:"context"`
        Urgency  string `json:"urgency,omitempty"`
    }) (interface{}, error) {
        
        // Build dynamic content based on arguments
        userPrompt := fmt.Sprintf("As a helpful assistant, help me %s in the context of %s.", 
            args.Action, args.Context)
        
        if args.Urgency == "high" {
            userPrompt += " This is urgent and requires immediate attention."
        } else if args.Urgency != "" {
            userPrompt += fmt.Sprintf(" This has %s urgency.", args.Urgency)
        }

        assistantPrompt := "I'll help you with this task using my expertise."
        if args.Urgency == "high" {
            assistantPrompt = "I understand this is urgent. I'll provide immediate assistance."
        }

        // Return properly formatted MCP response
        return map[string]interface{}{
            "description": "A prompt with dynamic content",
            "messages": []map[string]interface{}{
                {
                    "role": "user",
                    "content": map[string]interface{}{
                        "type": "text",
                        "text": userPrompt,
                    },
                },
                {
                    "role": "assistant",
                    "content": map[string]interface{}{
                        "type": "text",
                        "text": assistantPrompt,
                    },
                },
            },
        }, nil
    })
```

## Prompt Arguments

### Automatic Argument Extraction

When using template variables, arguments are automatically extracted:

```go
s.Prompt("translation", "Translate text between languages",
    server.User("Translate '{{text}}' from {{source_language}} to {{target_language}}"))

// Automatically creates arguments:
// - text (required)
// - source_language (required) 
// - target_language (required)
```

### Argument Validation

The server automatically validates required arguments:

```json
{
  "jsonrpc": "2.0",
  "method": "prompts/get",
  "params": {
    "name": "translation",
    "arguments": {
      "text": "Hello world",
      "source_language": "English"
      // Missing target_language - will return error
    }
  }
}
```

## Content Types

### Text Content (Default)

Most prompts use text content:

```go
s.Prompt("text-prompt", "A text-based prompt",
    server.User("Analyze this text: {{content}}"))
```

### Rich Content with Resources

Reference server resources in prompts:

```go
s.Prompt("document-analysis", "Analyze a document",
    server.User("As a document analysis expert, please analyze the document at {{document_uri}} focusing on {{analysis_type}}."))
```

## Best Practices

### 1. Clear Naming and Descriptions

```go
// ✅ Good: Clear, descriptive names
s.Prompt("k8s-troubleshooting", "Kubernetes cluster troubleshooting assistant", ...)

// ❌ Avoid: Vague names
s.Prompt("helper", "Does stuff", ...)
```

### 2. Structured Instructions

```go
// ✅ Good: Well-structured with clear steps
s.Prompt("code-review", "Systematic code review",
    server.User(`As a senior developer conducting code reviews, review this {{language}} code:

## Code to Review:
{{code}}

## Review Focus:
- Code quality and readability
- Security considerations  
- Performance implications
- Best practices adherence

## Requirements:
- Provide specific feedback
- Suggest improvements
- Rate overall quality (1-10)`),
    server.Assistant("I'll provide a comprehensive code review based on the criteria you've outlined."))
```

### 3. Meaningful Variable Names

```go
// ✅ Good: Descriptive variable names
s.Prompt("api-documentation", "Generate API documentation",
    server.User("Document the {{endpoint_method}} {{endpoint_path}} endpoint that {{endpoint_description}}"))

// ❌ Avoid: Generic variable names  
s.Prompt("api-docs", "Generate docs",
    server.User("Document {{a}} {{b}} that {{c}}"))
```

### 4. Provide Context

```go
// ✅ Good: Rich context for better responses
s.Prompt("bug-report-analysis", "Analyze bug reports",
    server.User(`As a senior QA engineer analyzing bug reports, analyze this bug report:

**Focus Areas:**
- Severity assessment
- Root cause analysis  
- Reproduction steps validation
- Fix priority recommendations

**Bug Report:**
**Title:** {{title}}
**Description:** {{description}}
**Steps to Reproduce:** {{steps}}
**Expected Result:** {{expected}}
**Actual Result:** {{actual}}
**Environment:** {{environment}}`),
    server.Assistant("I'll analyze this bug report systematically and provide recommendations."))
```

## Testing Prompts

### Using the MCP Client

Test your prompts using the GOMCP client:

```go
// Get available prompts
prompts, err := client.ListPrompts()

// Render a prompt with arguments
result, err := client.GetPrompt("code-review", map[string]interface{}{
    "language": "Go",
    "code": "func main() { fmt.Println(\"Hello\") }",
})
```

### JSON-RPC Testing

Test directly with JSON-RPC:

```json
{
  "jsonrpc": "2.0",
  "method": "prompts/get", 
  "params": {
    "name": "code-review",
    "arguments": {
      "language": "Go",
      "code": "func main() { fmt.Println(\"Hello\") }"
    }
  },
  "id": 1
}
```

## Error Handling

### Common Errors

- **Prompt not found**: `-32602` (Invalid params)
- **Missing required arguments**: `-32602` (Invalid params)  
- **Internal errors**: `-32603` (Internal error)

### Error Response Example

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "missing required argument: target_language"
  },
  "id": 1
}
```

## Complete Example

Here's a complete server with various prompt types:

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/localrivet/gomcp/server"
)

func main() {
    s := server.NewServer("prompt-examples").AsStdio()

    // Simple prompt
    s.Prompt("greeting", "Basic greeting",
        "Hello! How can I help you?")

    // Multi-role prompt with variables
    s.Prompt("code-review", "Code review assistant",
        server.User("Review this {{language}} code:\n{{code}}"),
        server.Assistant("I'll provide detailed feedback on your code."))

    // Dynamic prompt with function
    s.Prompt("adaptive-help", "Context-aware assistance",
        func(ctx *server.Context, args struct {
            Task     string `json:"task"`
            Skill    string `json:"skill_level"`
            TimeLeft string `json:"time_available,omitempty"`
        }) (interface{}, error) {
            
            expertise := "I'm here to help"
            if args.Skill == "beginner" {
                expertise = "I'll explain things step-by-step"
            } else if args.Skill == "expert" {
                expertise = "I'll focus on advanced techniques"
            }

            prompt := fmt.Sprintf("%s with %s.", expertise, args.Task)
            if args.TimeLeft != "" {
                prompt += fmt.Sprintf(" We have %s available.", args.TimeLeft)
            }

            return map[string]interface{}{
                "description": "Context-aware assistance",
                "messages": []map[string]interface{}{
                    {
                        "role": "user",
                        "content": map[string]interface{}{
                            "type": "text",
                            "text": prompt,
                        },
                    },
                },
            }, nil
        })

    if err := s.Run(); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

## Next Steps

- Learn about [implementing tools](03-implementing-tools.md)
- Explore [implementing resources](04-implementing-resources.md)
- See the [API reference](../../api-reference/README.md) for complete prompt options
- Check the [MCP specification](../../spec-reference/README.md) for protocol details 