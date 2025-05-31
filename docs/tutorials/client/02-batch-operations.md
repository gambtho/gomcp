# Batch Operations

The GoMCP client supports JSON-RPC batch operations, allowing you to send multiple requests in a single message. This can significantly improve performance when making multiple calls to the MCP server.

## Overview

Batch operations allow you to:
- Send multiple requests in a single round-trip
- Mix different types of requests (tools, resources, prompts)
- Include notifications alongside regular requests
- Maintain request order in responses
- Handle partial failures gracefully

## Basic Usage

### Using SendBatch

The most direct way to send a batch is using the `SendBatch` method:

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/localrivet/gomcp/client"
)

func main() {
    c, err := client.NewClient("ws://localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Create batch requests
    requests := []client.BatchRequest{
        {
            Method: "tools/call",
            Params: map[string]interface{}{
                "name": "calculator",
                "arguments": map[string]interface{}{
                    "operation": "add",
                    "a": 5,
                    "b": 3,
                },
            },
            ID: 1,
        },
        {
            Method: "resources/read",
            Params: map[string]interface{}{
                "uri": "/config/settings.json",
            },
            ID: 2,
        },
        {
            Method: "prompts/get",
            Params: map[string]interface{}{
                "name": "greeting",
                "arguments": map[string]interface{}{
                    "name": "Alice",
                },
            },
            ID: 3,
        },
    }
    
    // Send batch
    responses, err := c.SendBatch(requests)
    if err != nil {
        log.Fatal(err)
    }
    
    // Process responses
    for _, response := range responses {
        if response.Error != nil {
            fmt.Printf("Request %v failed: %s\n", response.ID, response.Error.Message)
        } else {
            fmt.Printf("Request %v succeeded: %+v\n", response.ID, response.Result)
        }
    }
}
```

### Using BatchBuilder

For a more fluent interface, use the `BatchBuilder`:

```go
responses, err := client.BatchBuilder().
    AddRequest("tools/call", map[string]interface{}{
        "name": "calculator",
        "arguments": map[string]interface{}{
            "operation": "multiply",
            "a": 4,
            "b": 7,
        },
    }, 1).
    AddRequest("resources/read", map[string]interface{}{
        "uri": "/data/users.json",
    }, 2).
    Execute()

if err != nil {
    log.Fatal(err)
}

for _, response := range responses {
    fmt.Printf("Response %v: %+v\n", response.ID, response.Result)
}
```

## Request Types

### Regular Requests

Regular requests expect a response and must include an ID:

```go
{
    Method: "tools/call",
    Params: map[string]interface{}{
        "name": "my_tool",
        "arguments": map[string]interface{}{"param": "value"},
    },
    ID: 1, // Required for requests expecting responses
}
```

### Notifications

Notifications don't expect a response and should have a nil ID:

```go
{
    Method: "notifications/progress",
    Params: map[string]interface{}{
        "progress": 50,
        "message": "Processing...",
    },
    // ID is nil for notifications
}
```

### Mixed Batches

You can mix requests and notifications in a single batch:

```go
requests := []client.BatchRequest{
    // Regular request
    {
        Method: "tools/call",
        Params: map[string]interface{}{
            "name": "process_data",
            "arguments": map[string]interface{}{"input": "data"},
        },
        ID: 1,
    },
    // Notification
    {
        Method: "notifications/progress",
        Params: map[string]interface{}{
            "progress": 25,
            "message": "Started processing",
        },
        // No ID for notification
    },
    // Another request
    {
        Method: "resources/read",
        Params: map[string]interface{}{
            "uri": "/results/output.json",
        },
        ID: 2,
    },
}

// Only requests with IDs will have responses
responses, err := c.SendBatch(requests)
// responses will contain 2 items (for ID 1 and ID 2)
```

## Error Handling

Batch operations can have partial failures. Individual requests can fail while others succeed:

```go
responses, err := c.SendBatch(requests)
if err != nil {
    // This indicates a transport or protocol error
    log.Fatal(err)
}

// Check individual responses
for _, response := range responses {
    if response.Error != nil {
        fmt.Printf("Request %v failed: Code %d, Message: %s\n", 
            response.ID, response.Error.Code, response.Error.Message)
        
        // Handle specific error codes
        switch response.Error.Code {
        case -32601: // Method not found
            fmt.Println("Method not supported")
        case -32602: // Invalid params
            fmt.Println("Invalid parameters")
        default:
            fmt.Printf("Other error: %d\n", response.Error.Code)
        }
    } else {
        fmt.Printf("Request %v succeeded\n", response.ID)
    }
}
```

## Request Options

Batch operations support the same options as individual requests:

```go
responses, err := c.SendBatch(requests, 
    client.WithRequestTimeoutOption(30*time.Second),
    client.WithContextOption(ctx),
)
```

## Performance Considerations

### When to Use Batches

Batch operations are most beneficial when:
- Making multiple related requests
- Network latency is high
- You need to ensure request ordering
- Processing many independent operations

### Batch Size Limits

Consider these factors when determining batch size:
- Server processing capacity
- Memory usage for large batches
- Timeout constraints
- Network packet size limits

```go
// For large datasets, consider splitting into smaller batches
const maxBatchSize = 50

func processBatch(items []Item) error {
    for i := 0; i < len(items); i += maxBatchSize {
        end := i + maxBatchSize
        if end > len(items) {
            end = len(items)
        }
        
        batch := items[i:end]
        if err := sendBatch(batch); err != nil {
            return err
        }
    }
    return nil
}
```

### Response Ordering

Responses maintain the same order as requests in the batch:

```go
requests := []client.BatchRequest{
    {Method: "tools/call", Params: params1, ID: "first"},
    {Method: "tools/call", Params: params2, ID: "second"},
    {Method: "tools/call", Params: params3, ID: "third"},
}

responses, _ := c.SendBatch(requests)
// responses[0] corresponds to "first"
// responses[1] corresponds to "second" 
// responses[2] corresponds to "third"
```

## Advanced Examples

### Parallel Processing with Error Recovery

```go
func processWorkflowStep(c client.Client, stepData []WorkflowItem) error {
    var requests []client.BatchRequest
    
    // Build batch requests
    for i, item := range stepData {
        requests = append(requests, client.BatchRequest{
            Method: "tools/call",
            Params: map[string]interface{}{
                "name": "process_item",
                "arguments": map[string]interface{}{
                    "item_id": item.ID,
                    "data": item.Data,
                },
            },
            ID: i + 1,
        })
    }
    
    // Send batch
    responses, err := c.SendBatch(requests)
    if err != nil {
        return fmt.Errorf("batch request failed: %w", err)
    }
    
    // Process results and handle failures
    var failures []WorkflowItem
    for i, response := range responses {
        if response.Error != nil {
            log.Printf("Item %s failed: %s", stepData[i].ID, response.Error.Message)
            failures = append(failures, stepData[i])
        } else {
            log.Printf("Item %s processed successfully", stepData[i].ID)
        }
    }
    
    // Retry failures individually if needed
    if len(failures) > 0 {
        return retryFailures(c, failures)
    }
    
    return nil
}
```

### Conditional Batch Building

```go
func buildConditionalBatch(c client.Client, config Config) ([]client.BatchResponse, error) {
    builder := c.BatchBuilder()
    
    // Always include basic tool call
    builder.AddRequest("tools/call", map[string]interface{}{
        "name": "initialize",
        "arguments": map[string]interface{}{},
    }, 1)
    
    // Conditionally add resource requests
    if config.LoadConfig {
        builder.AddRequest("resources/read", map[string]interface{}{
            "uri": "/config.json",
        }, 2)
    }
    
    // Conditionally add prompt requests
    if config.UsePrompts {
        builder.AddRequest("prompts/get", map[string]interface{}{
            "name": "system_prompt",
            "arguments": map[string]interface{}{},
        }, 3)
    }
    
    return builder.Execute()
}
```

## Best Practices

1. **Use appropriate ID types**: IDs can be strings or numbers, choose what works best for your use case
2. **Handle partial failures**: Always check individual response errors
3. **Consider batch size**: Balance performance with resource usage
4. **Use timeouts**: Set appropriate timeouts for batch operations
5. **Order matters**: Responses maintain request order, design accordingly
6. **Test error scenarios**: Ensure your code handles various failure modes
7. **Monitor performance**: Batch operations should improve, not degrade performance

## Integration with Server-Side Batch Support

The client batch functionality integrates seamlessly with server-side batch processing. The server will:
- Process requests in the order received
- Return responses in the same order
- Handle mixed request/notification batches correctly
- Provide appropriate error responses for failed requests

This ensures consistent behavior across different MCP implementations and transport layers. 