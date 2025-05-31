# MCP Sampling Examples

This directory contains comprehensive examples demonstrating how to use the **Model Context Protocol (MCP) Sampling** functionality in Go. Sampling allows MCP servers to request AI model inference from clients, enabling powerful AI-assisted tools and workflows.

## Overview

The MCP Sampling specification allows servers to request AI model inference from clients. This enables:

- **AI-powered tools**: Servers can use AI to analyze text, images, audio, and other content
- **Multi-modal analysis**: Combine different content types in a single request
- **Model preferences**: Specify preferred models and optimization priorities
- **Streaming responses**: Get real-time streaming responses (2025-03-26 protocol)
- **Priority handling**: Mark urgent requests for faster processing

## Examples Structure

```
sampling/
├── client/          # Client example - responds to sampling requests
│   ├── main.go      # Comprehensive client with sampling handler
│   └── go.mod       # Dependencies
├── server/          # Server example - makes sampling requests
│   ├── main.go      # Server with AI-powered tools
│   └── go.mod       # Dependencies
└── README.md        # This file
```

## Protocol Version Support

| Feature | Draft | 2024-11-05 | 2025-03-26 |
|---------|-------|------------|------------|
| Text content | ✅ | ✅ | ✅ |
| Image content | ✅ | ✅ | ✅ |
| Audio content | ❌ | ❌ | ✅ |
| Streaming | ❌ | ❌ | ✅ |
| Model preferences | ✅ | ✅ | ✅ |
| Priority handling | ✅ | ✅ | ✅ |

## Running the Examples

### 1. Start the Server

```bash
cd server
go run main.go
```

The server will start and display available tools:
- `analyze_text`: Analyze text using AI sampling
- `describe_image`: Describe images using AI sampling  
- `transcribe_audio`: Transcribe audio using AI sampling (2025-03-26)
- `priority_analysis`: Analyze text with priority sampling
- `multimodal_analysis`: Analyze multiple content types
- `test_model_preferences`: Test different model strategies

### 2. Connect the Client

In another terminal:

```bash
cd client
go run main.go
```

The client will:
1. Connect to the server with sampling support
2. Set up a sampling handler to respond to AI requests
3. Demonstrate various sampling scenarios
4. Show real-time responses and model information

## Key Features Demonstrated

### 1. Basic Text Sampling

```go
// Create text messages
messages := []client.SamplingMessage{
    client.CreateTextMessage("user", "What is the capital of France?"),
}

// Set model preferences
prefs := client.SamplingModelPreferences{
    Hints: []client.SamplingModelHint{
        {Name: "gpt-4"},
        {Name: "claude"},
    },
}

// Create options and send request
opts := client.NewSamplingOptions(messages, prefs).
    WithSystemPrompt("You are a helpful geography assistant.").
    WithMaxTokens(100)

response, err := c.RequestSampling(opts)
```

### 2. Image Content Analysis

```go
// Create image message
messages := []client.SamplingMessage{
    client.CreateImageMessage("user", base64ImageData, "image/png"),
    client.CreateTextMessage("user", "What do you see in this image?"),
}

opts := client.NewSamplingOptions(messages, prefs).
    WithSystemPrompt("You are an expert image analyst.")

response, err := c.RequestSampling(opts)
```

### 3. Audio Content Processing (2025-03-26)

```go
// Create audio message
messages := []client.SamplingMessage{
    client.CreateAudioMessage("user", base64AudioData, "audio/wav"),
    client.CreateTextMessage("user", "What do you hear in this audio?"),
}

response, err := c.RequestSampling(opts)
```

### 4. Streaming Responses (2025-03-26)

```go
// Set up streaming handler
streamHandler := func(chunk *client.SamplingResponse) error {
    fmt.Printf("Chunk %d: %s\n", chunk.ChunkIndex, chunk.Content.Text)
    return nil
}

// Create streaming options
opts := client.NewSamplingOptions(messages, prefs).
    WithStreaming(streamHandler).
    WithChunkSize(50)

response, err := c.RequestSampling(opts)
```

### 5. Model Preferences and Priorities

```go
// Configure model preferences
costPriority := 0.8         // Prefer cheaper models
speedPriority := 0.6        // Medium speed priority  
intelligencePriority := 0.9 // Prefer smarter models

prefs := client.SamplingModelPreferences{
    Hints: []client.SamplingModelHint{
        {Name: "claude-3-sonnet"},
        {Name: "gpt-4"},
    },
    CostPriority:         &costPriority,
    SpeedPriority:        &speedPriority,
    IntelligencePriority: &intelligencePriority,
}
```

### 6. Server-Side Sampling Requests

```go
// Server tool requesting sampling from client
srv.Tool("analyze_text", "Analyze text using AI", func(ctx *server.Context, args struct {
    Text string `json:"text"`
}) (interface{}, error) {
    messages := []server.SamplingMessage{
        server.CreateTextSamplingMessage("user", args.Text),
    }
    
    prefs := server.SamplingModelPreferences{
        Hints: []server.SamplingModelHint{
            {Name: "claude-3-sonnet"},
        },
    }
    
    response, err := ctx.RequestSampling(messages, prefs, "Analyze this text.", 300)
    if err != nil {
        return nil, err
    }
    
    return map[string]interface{}{
        "analysis": response.Content.Text,
        "model":    response.Model,
    }, nil
})
```

## Error Handling and Validation

The examples demonstrate proper error handling for:

- **Empty messages**: Validation fails for requests with no content
- **Streaming without handler**: Streaming requires a valid handler function
- **Invalid chunk sizes**: Chunk sizes must be within acceptable ranges
- **Unsupported content types**: Audio content requires 2025-03-26 protocol
- **Model availability**: Graceful fallback when preferred models aren't available

## Content Type Validation

```go
// Check if content is valid for protocol version
audioMsg := client.CreateAudioMessage("user", "audio-data", "audio/wav")
if !audioMsg.Content.IsValidForVersion("2024-11-05") {
    // Audio not supported in 2024-11-05
}
```

## Testing the Examples

### Test Text Analysis

```bash
# Call the analyze_text tool
echo '{"method": "tools/call", "params": {"name": "analyze_text", "arguments": {"text": "This is a great day!"}}}' | go run server/main.go
```

### Test Image Description

```bash
# Call the describe_image tool with base64 image
echo '{"method": "tools/call", "params": {"name": "describe_image", "arguments": {"image_data": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==", "mime_type": "image/png"}}}' | go run server/main.go
```

### Test Model Preferences

```bash
# Test different model strategies
echo '{"method": "tools/call", "params": {"name": "test_model_preferences", "arguments": {"query": "Explain quantum computing", "strategy": "intelligence"}}}' | go run server/main.go
```

## Integration with Your Application

To integrate sampling into your own MCP application:

### Client Side (Responding to Sampling Requests)

```go
// Set up sampling handler
client := client.NewClient("my-app").WithSamplingHandler(
    func(params client.SamplingCreateMessageParams) (client.SamplingResponse, error) {
        // Call your AI model here
        result := callYourAIModel(params.Messages, params.SystemPrompt)
        
        return client.SamplingResponse{
            Role: "assistant",
            Content: client.SamplingMessageContent{
                Type: "text",
                Text: result,
            },
            Model: "your-model-name",
            StopReason: "endTurn",
        }, nil
    },
)
```

### Server Side (Making Sampling Requests)

```go
// In your tool handler
srv.Tool("my_ai_tool", "AI-powered tool", func(ctx *server.Context, args MyArgs) (interface{}, error) {
    messages := []server.SamplingMessage{
        server.CreateTextSamplingMessage("user", args.UserInput),
    }
    
    prefs := server.SamplingModelPreferences{
        Hints: []server.SamplingModelHint{{Name: "gpt-4"}},
    }
    
    response, err := ctx.RequestSampling(messages, prefs, "Be helpful.", 200)
    if err != nil {
        return nil, err
    }
    
    return response.Content.Text, nil
})
```

## Best Practices

1. **Always validate content types** for the protocol version you're using
2. **Handle errors gracefully** - AI requests can fail for various reasons
3. **Set appropriate timeouts** to prevent hanging requests
4. **Use model preferences** to optimize for your use case (cost, speed, intelligence)
5. **Implement proper streaming handlers** for real-time responses
6. **Log sampling requests** for debugging and monitoring
7. **Respect rate limits** and implement backoff strategies

## Troubleshooting

### Common Issues

1. **"Audio content not supported"**: Use protocol version 2025-03-26 for audio
2. **"Streaming not supported"**: Use protocol version 2025-03-26 for streaming
3. **"No sampling handler"**: Client must set a sampling handler before connecting
4. **"Model not available"**: Check model hints and ensure fallback models
5. **"Timeout exceeded"**: Increase timeout or implement progress notifications

### Debug Mode

Enable debug logging to see detailed sampling request/response flow:

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

client := client.NewClient("debug-client", client.WithLogger(logger))
```

## Further Reading

- [MCP Sampling Specification](../../specification/2025-03-26/client/sampling.mdx)
- [MCP Protocol Documentation](../../docs/)
- [Other MCP Examples](../)

## Contributing

Found an issue or want to improve these examples? Please open an issue or submit a pull request! 