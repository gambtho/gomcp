package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	fmt.Println("Creating MCP client with sampling support...")

	// Create a new client with sampling capabilities
	c, err := client.NewClient("sampling-client",
		client.WithLogger(logger),
		client.WithProtocolVersion("2025-03-26"), // Use latest version for full sampling support
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Set up a sampling handler to respond to sampling requests from the server
	c = c.WithSamplingHandler(func(params client.SamplingCreateMessageParams) (client.SamplingResponse, error) {
		fmt.Printf("\n🤖 Server requested sampling with %d messages\n", len(params.Messages))

		// Log the request details
		for i, msg := range params.Messages {
			fmt.Printf("  Message %d (%s): %s\n", i+1, msg.Role, getContentPreview(msg.Content))
		}

		if params.SystemPrompt != "" {
			fmt.Printf("  System prompt: %s\n", params.SystemPrompt)
		}

		// Simulate AI model response
		response := client.SamplingResponse{
			Role: "assistant",
			Content: client.SamplingMessageContent{
				Type: "text",
				Text: "This is a simulated AI response. In a real implementation, this would call an actual LLM.",
			},
			Model:      "simulated-model-v1",
			StopReason: "endTurn",
		}

		fmt.Printf("  ✅ Responding with: %s\n", response.Content.Text)
		return response, nil
	})

	fmt.Println("✅ Client connected with sampling support!")

	// Example 1: Basic text sampling request
	fmt.Println("\n📝 Example 1: Basic Text Sampling")
	demonstrateBasicTextSampling(c)

	// Example 2: Image content sampling
	fmt.Println("\n🖼️  Example 2: Image Content Sampling")
	demonstrateImageSampling(c)

	// Example 3: Audio content sampling (2025-03-26 only)
	fmt.Println("\n🎵 Example 3: Audio Content Sampling")
	demonstrateAudioSampling(c)

	// Example 4: Streaming sampling (2025-03-26 only)
	fmt.Println("\n🌊 Example 4: Streaming Sampling")
	demonstrateStreamingSampling(c)

	// Example 5: Model preferences and priorities
	fmt.Println("\n⚙️  Example 5: Model Preferences")
	demonstrateModelPreferences(c)

	// Example 6: Error handling and validation
	fmt.Println("\n❌ Example 6: Error Handling")
	demonstrateErrorHandling(c)

	fmt.Println("\n🎉 All sampling examples completed successfully!")
}

func demonstrateBasicTextSampling(c client.Client) {
	// Create a simple text message
	messages := []client.SamplingMessage{
		client.CreateTextMessage("user", "What is the capital of France?"),
	}

	// Create basic model preferences
	prefs := client.SamplingModelPreferences{
		Hints: []client.SamplingModelHint{
			{Name: "gpt-4"},
			{Name: "claude"},
		},
	}

	// Create sampling options
	opts := client.NewSamplingOptions(messages, prefs).
		WithSystemPrompt("You are a helpful geography assistant.").
		WithMaxTokens(100).
		WithTimeout(30 * time.Second)

	// Send the sampling request
	response, err := c.RequestSampling(opts)
	if err != nil {
		fmt.Printf("  ❌ Sampling failed: %v\n", err)
		return
	}

	fmt.Printf("  ✅ Response: %s\n", response.Content.Text)
	fmt.Printf("  📊 Model: %s, Stop reason: %s\n", response.Model, response.StopReason)
}

func demonstrateImageSampling(c client.Client) {
	// Create a message with image content
	messages := []client.SamplingMessage{
		client.CreateImageMessage("user", "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==", "image/png"),
		client.CreateTextMessage("user", "What do you see in this image?"),
	}

	// Create options for image analysis
	opts := client.NewSamplingOptions(messages, client.SamplingModelPreferences{}).
		WithSystemPrompt("You are a helpful image analysis assistant.").
		WithMaxTokens(200)

	response, err := c.RequestSampling(opts)
	if err != nil {
		fmt.Printf("  ❌ Image sampling failed: %v\n", err)
		return
	}

	fmt.Printf("  ✅ Image analysis: %s\n", response.Content.Text)
}

func demonstrateAudioSampling(c client.Client) {
	// Create a message with audio content (supported in 2025-03-26)
	messages := []client.SamplingMessage{
		client.CreateAudioMessage("user", "UklGRnoGAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQoGAACBhYqFbF1fdJivrJBhNjVgodDbq2EcBj+a2/LDciUFLIHO8tiJNwgZaLvt559NEAxQp+PwtmMcBjiR1/LMeSwFJHfH8N2QQAoUXrTp66hVFApGn+DyvmwhBSuBzvLZiTYIG2m98OScTgwOUarm7blmGgU7k9n1unEiBC13yO/eizEIHWq+8+OWT", "audio/wav"),
		client.CreateTextMessage("user", "What do you hear in this audio?"),
	}

	opts := client.NewSamplingOptions(messages, client.SamplingModelPreferences{}).
		WithSystemPrompt("You are a helpful audio analysis assistant.").
		WithMaxTokens(200)

	response, err := c.RequestSampling(opts)
	if err != nil {
		fmt.Printf("  ❌ Audio sampling failed: %v\n", err)
		return
	}

	fmt.Printf("  ✅ Audio analysis: %s\n", response.Content.Text)
}

func demonstrateStreamingSampling(c client.Client) {
	messages := []client.SamplingMessage{
		client.CreateTextMessage("user", "Tell me a short story about a robot learning to paint."),
	}

	// Track streaming chunks
	var chunks []string
	streamHandler := func(chunk *client.SamplingResponse) error {
		chunks = append(chunks, chunk.Content.Text)
		fmt.Printf("  📦 Chunk %d: %s\n", chunk.ChunkIndex, chunk.Content.Text)

		if chunk.IsComplete {
			fmt.Printf("  ✅ Streaming complete! Total chunks: %d\n", len(chunks))
		}
		return nil
	}

	// Create streaming options
	opts := client.NewSamplingOptions(messages, client.SamplingModelPreferences{}).
		WithSystemPrompt("You are a creative storyteller.").
		WithMaxTokens(300).
		WithStreaming(streamHandler).
		WithChunkSize(50)

	response, err := c.RequestSampling(opts)
	if err != nil {
		fmt.Printf("  ❌ Streaming sampling failed: %v\n", err)
		return
	}

	fmt.Printf("  📖 Final story: %s\n", response.Content.Text)
}

func demonstrateModelPreferences(c client.Client) {
	messages := []client.SamplingMessage{
		client.CreateTextMessage("user", "Explain quantum computing in simple terms."),
	}

	// Create detailed model preferences
	costPriority := 0.8         // High cost priority (prefer cheaper models)
	speedPriority := 0.6        // Medium speed priority
	intelligencePriority := 0.9 // High intelligence priority (prefer smarter models)

	prefs := client.SamplingModelPreferences{
		Hints: []client.SamplingModelHint{
			{Name: "claude-3-sonnet"}, // Prefer Sonnet-class models
			{Name: "gpt-4"},           // Fall back to GPT-4
			{Name: "claude"},          // Any Claude model
		},
		CostPriority:         &costPriority,
		SpeedPriority:        &speedPriority,
		IntelligencePriority: &intelligencePriority,
	}

	opts := client.NewSamplingOptions(messages, prefs).
		WithSystemPrompt("You are an expert physics teacher who explains complex topics simply.").
		WithMaxTokens(250)

	response, err := c.RequestSampling(opts)
	if err != nil {
		fmt.Printf("  ❌ Model preference sampling failed: %v\n", err)
		return
	}

	fmt.Printf("  ✅ Explanation: %s\n", response.Content.Text)
	fmt.Printf("  🎯 Selected model: %s\n", response.Model)
}

func demonstrateErrorHandling(c client.Client) {
	// Example 1: Empty messages (should fail validation)
	fmt.Println("  Testing empty messages...")
	opts := client.NewSamplingOptions([]client.SamplingMessage{}, client.SamplingModelPreferences{})
	_, err := c.RequestSampling(opts)
	if err != nil {
		fmt.Printf("  ✅ Expected error: %v\n", err)
	}

	// Example 2: Streaming without handler (should fail validation)
	fmt.Println("  Testing streaming without handler...")
	messages := []client.SamplingMessage{
		client.CreateTextMessage("user", "Hello"),
	}
	opts = client.NewSamplingOptions(messages, client.SamplingModelPreferences{})
	opts.ProtocolVersion = "2025-03-26"
	opts.Streaming = true
	// No StreamHandler set
	_, err = c.RequestSampling(opts)
	if err != nil {
		fmt.Printf("  ✅ Expected error: %v\n", err)
	}

	// Example 3: Invalid chunk size
	fmt.Println("  Testing invalid chunk size...")
	opts = client.NewSamplingOptions(messages, client.SamplingModelPreferences{})
	opts.ProtocolVersion = "2025-03-26"
	opts.Streaming = true
	opts.StreamHandler = func(*client.SamplingResponse) error { return nil }
	opts.ChunkSize = 5 // Too small
	_, err = c.RequestSampling(opts)
	if err != nil {
		fmt.Printf("  ✅ Expected error: %v\n", err)
	}

	// Example 4: Audio content in unsupported version
	fmt.Println("  Testing audio in 2024-11-05...")
	audioMsg := client.CreateAudioMessage("user", "audio-data", "audio/wav")
	if !audioMsg.Content.IsValidForVersion("2024-11-05") {
		fmt.Printf("  ✅ Audio correctly rejected in 2024-11-05\n")
	}
}

func getContentPreview(content client.SamplingMessageContent) string {
	switch content.Type {
	case "text":
		if len(content.Text) > 50 {
			return content.Text[:50] + "..."
		}
		return content.Text
	case "image":
		return fmt.Sprintf("Image (%s, %d bytes)", content.MimeType, len(content.Data))
	case "audio":
		return fmt.Sprintf("Audio (%s, %d bytes)", content.MimeType, len(content.Data))
	default:
		return fmt.Sprintf("Unknown content type: %s", content.Type)
	}
}
