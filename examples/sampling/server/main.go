package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/localrivet/gomcp/server"
)

func main() {
	// Create a logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	fmt.Println("Creating MCP server with sampling capabilities...")

	// Create server with sampling support
	srv := server.NewServer("sampling-server",
		server.WithLogger(logger),
	).AsStdio("logs/sampling.log")

	// Tool 1: Text Analysis - Requests text sampling from client
	srv.Tool("analyze_text", "Analyze text using AI sampling", func(ctx *server.Context, args struct {
		Text string `json:"text" description:"Text to analyze"`
	}) (interface{}, error) {
		// Create sampling messages for text analysis
		messages := []server.SamplingMessage{
			server.CreateTextSamplingMessage("user", fmt.Sprintf("Analyze this text for sentiment, tone, and key themes: %s", args.Text)),
		}

		// Create model preferences
		prefs := server.SamplingModelPreferences{
			Hints: []server.SamplingModelHint{
				{Name: "claude-3-sonnet"}, // Prefer analytical models
				{Name: "gpt-4"},
			},
		}

		// Request sampling from the client with system prompt
		response, err := ctx.RequestSampling(messages, prefs, "You are an expert text analyst. Be thorough and analytical.", 300)
		if err != nil {
			return nil, fmt.Errorf("sampling request failed: %w", err)
		}

		return map[string]interface{}{
			"analysis": response.Content.Text,
			"model":    response.Model,
		}, nil
	})

	// Tool 2: Image Description - Requests image sampling from client
	srv.Tool("describe_image", "Describe an image using AI sampling", func(ctx *server.Context, args struct {
		ImageData string `json:"image_data" description:"Base64 encoded image data"`
		MimeType  string `json:"mime_type" description:"Image MIME type (e.g., image/jpeg)"`
	}) (interface{}, error) {
		// Create sampling messages with image content
		messages := []server.SamplingMessage{
			server.CreateImageSamplingMessage("user", args.ImageData, args.MimeType),
			server.CreateTextSamplingMessage("user", "Please describe this image in detail, including objects, colors, composition, and mood."),
		}

		prefs := server.SamplingModelPreferences{
			Hints: []server.SamplingModelHint{
				{Name: "claude-3-sonnet"}, // Good for vision tasks
				{Name: "gpt-4-vision"},
			},
		}

		response, err := ctx.RequestSampling(messages, prefs, "You are an expert art critic and image analyst.", 400)
		if err != nil {
			return nil, fmt.Errorf("image sampling failed: %w", err)
		}

		return map[string]interface{}{
			"description": response.Content.Text,
			"model":       response.Model,
		}, nil
	})

	// Tool 3: Audio Transcription - Requests audio sampling from client (2025-03-26 only)
	srv.Tool("transcribe_audio", "Transcribe audio using AI sampling", func(ctx *server.Context, args struct {
		AudioData string `json:"audio_data" description:"Base64 encoded audio data"`
		MimeType  string `json:"mime_type" description:"Audio MIME type (e.g., audio/wav)"`
	}) (interface{}, error) {
		// Create sampling messages with audio content
		messages := []server.SamplingMessage{
			server.CreateAudioSamplingMessage("user", args.AudioData, args.MimeType),
			server.CreateTextSamplingMessage("user", "Please transcribe this audio and provide a summary of the content."),
		}

		prefs := server.SamplingModelPreferences{
			Hints: []server.SamplingModelHint{
				{Name: "whisper"}, // Prefer speech recognition models
				{Name: "claude-3-sonnet"},
			},
		}

		response, err := ctx.RequestSampling(messages, prefs, "You are an expert transcriptionist.", 500)
		if err != nil {
			return nil, fmt.Errorf("audio sampling failed: %w", err)
		}

		return map[string]interface{}{
			"transcription": response.Content.Text,
			"model":         response.Model,
		}, nil
	})

	// Tool 4: Priority Sampling - Demonstrates high-priority sampling requests
	srv.Tool("priority_analysis", "Analyze text with high priority sampling", func(ctx *server.Context, args struct {
		Text     string `json:"text" description:"Text to analyze"`
		Priority int    `json:"priority" description:"Priority level (1-10, higher is more urgent)"`
	}) (interface{}, error) {
		// Create sampling messages
		messages := []server.SamplingMessage{
			server.CreateTextSamplingMessage("user", fmt.Sprintf("Provide urgent analysis of this text: %s", args.Text)),
		}

		prefs := server.SamplingModelPreferences{
			Hints: []server.SamplingModelHint{
				{Name: "claude-3-sonnet"}, // Good for analysis
				{Name: "gpt-4"},
			},
		}

		// Use priority sampling for urgent requests
		response, err := ctx.RequestSamplingWithPriority(messages, prefs, "You are an expert analyst. Provide immediate insights.", 250, args.Priority)
		if err != nil {
			return nil, fmt.Errorf("priority sampling failed: %w", err)
		}

		return map[string]interface{}{
			"analysis": response.Content.Text,
			"model":    response.Model,
			"priority": args.Priority,
		}, nil
	})

	// Tool 5: Multi-Modal Analysis - Combines text, image, and audio
	srv.Tool("multimodal_analysis", "Analyze multiple content types using AI sampling", func(ctx *server.Context, args struct {
		Text      string `json:"text,omitempty" description:"Text content to analyze"`
		ImageData string `json:"image_data,omitempty" description:"Base64 encoded image data"`
		AudioData string `json:"audio_data,omitempty" description:"Base64 encoded audio data"`
		MimeType  string `json:"mime_type,omitempty" description:"MIME type for media content"`
	}) (interface{}, error) {
		var messages []server.SamplingMessage

		// Add content based on what's provided
		if args.Text != "" {
			messages = append(messages, server.CreateTextSamplingMessage("user", args.Text))
		}
		if args.ImageData != "" {
			messages = append(messages, server.CreateImageSamplingMessage("user", args.ImageData, args.MimeType))
		}
		if args.AudioData != "" {
			messages = append(messages, server.CreateAudioSamplingMessage("user", args.AudioData, args.MimeType))
		}

		if len(messages) == 0 {
			return nil, fmt.Errorf("no content provided for analysis")
		}

		// Add analysis instruction
		messages = append(messages, server.CreateTextSamplingMessage("user", "Please analyze all the provided content and explain how they relate to each other."))

		prefs := server.SamplingModelPreferences{
			Hints: []server.SamplingModelHint{
				{Name: "claude-3-sonnet"}, // Good for multi-modal analysis
				{Name: "gpt-4-vision"},
			},
		}

		response, err := ctx.RequestSampling(messages, prefs, "You are an expert multi-modal analyst.", 600)
		if err != nil {
			return nil, fmt.Errorf("multi-modal sampling failed: %w", err)
		}

		return map[string]interface{}{
			"analysis":      response.Content.Text,
			"content_types": len(messages) - 1, // Subtract the instruction message
			"model":         response.Model,
		}, nil
	})

	// Tool 6: Model Preference Testing - Demonstrates different model preferences
	srv.Tool("test_model_preferences", "Test different model preference strategies", func(ctx *server.Context, args struct {
		Query    string `json:"query" description:"Query to send to different models"`
		Strategy string `json:"strategy" description:"Preference strategy: cost, speed, intelligence, or balanced"`
	}) (interface{}, error) {
		messages := []server.SamplingMessage{
			server.CreateTextSamplingMessage("user", args.Query),
		}

		var prefs server.SamplingModelPreferences

		// Configure preferences based on strategy
		switch args.Strategy {
		case "cost":
			costPriority := 0.9
			speedPriority := 0.3
			intelligencePriority := 0.3
			prefs = server.SamplingModelPreferences{
				Hints: []server.SamplingModelHint{
					{Name: "claude-3-haiku"}, // Cheaper models first
					{Name: "gpt-3.5-turbo"},
				},
				CostPriority:         &costPriority,
				SpeedPriority:        &speedPriority,
				IntelligencePriority: &intelligencePriority,
			}
		case "speed":
			costPriority := 0.3
			speedPriority := 0.9
			intelligencePriority := 0.3
			prefs = server.SamplingModelPreferences{
				Hints: []server.SamplingModelHint{
					{Name: "claude-3-haiku"}, // Faster models first
					{Name: "gpt-3.5-turbo"},
				},
				CostPriority:         &costPriority,
				SpeedPriority:        &speedPriority,
				IntelligencePriority: &intelligencePriority,
			}
		case "intelligence":
			costPriority := 0.2
			speedPriority := 0.2
			intelligencePriority := 0.9
			prefs = server.SamplingModelPreferences{
				Hints: []server.SamplingModelHint{
					{Name: "claude-3-opus"}, // Smarter models first
					{Name: "gpt-4"},
				},
				CostPriority:         &costPriority,
				SpeedPriority:        &speedPriority,
				IntelligencePriority: &intelligencePriority,
			}
		default: // balanced
			costPriority := 0.6
			speedPriority := 0.6
			intelligencePriority := 0.6
			prefs = server.SamplingModelPreferences{
				Hints: []server.SamplingModelHint{
					{Name: "claude-3-sonnet"}, // Balanced models
					{Name: "gpt-4"},
				},
				CostPriority:         &costPriority,
				SpeedPriority:        &speedPriority,
				IntelligencePriority: &intelligencePriority,
			}
		}

		response, err := ctx.RequestSampling(messages, prefs, "Answer the query helpfully and concisely.", 200)
		if err != nil {
			return nil, fmt.Errorf("model preference sampling failed: %w", err)
		}

		return map[string]interface{}{
			"response":    response.Content.Text,
			"model":       response.Model,
			"strategy":    args.Strategy,
			"stop_reason": response.StopReason,
		}, nil
	})

	// Resource: Sampling Capabilities - Shows what the server can do
	srv.Resource("/sampling/capabilities", "Get server sampling capabilities", func(ctx *server.Context, params map[string]interface{}) (server.JSONResource, error) {
		capabilities := map[string]interface{}{
			"supported_content_types": []string{"text", "image", "audio"},
			"max_tokens":              1000,
			"supported_models": []string{
				"claude-3-opus",
				"claude-3-sonnet",
				"claude-3-haiku",
				"gpt-4",
				"gpt-3.5-turbo",
			},
			"tools": []map[string]interface{}{
				{
					"name":          "analyze_text",
					"description":   "Analyze text using AI sampling",
					"content_types": []string{"text"},
				},
				{
					"name":          "describe_image",
					"description":   "Describe an image using AI sampling",
					"content_types": []string{"text", "image"},
				},
				{
					"name":          "transcribe_audio",
					"description":   "Transcribe audio using AI sampling",
					"content_types": []string{"text", "audio"},
					"requires":      "2025-03-26",
				},
				{
					"name":          "priority_analysis",
					"description":   "Analyze text with priority sampling",
					"content_types": []string{"text"},
					"features":      []string{"priority"},
				},
				{
					"name":          "multimodal_analysis",
					"description":   "Analyze multiple content types",
					"content_types": []string{"text", "image", "audio"},
				},
				{
					"name":          "test_model_preferences",
					"description":   "Test different model preference strategies",
					"content_types": []string{"text"},
				},
			},
		}

		return server.JSONResource{
			Data: capabilities,
		}, nil
	})

	fmt.Println("✅ Server configured with comprehensive sampling support!")
	fmt.Println("📋 Available tools:")
	fmt.Println("  - analyze_text: Analyze text using AI sampling")
	fmt.Println("  - describe_image: Describe images using AI sampling")
	fmt.Println("  - transcribe_audio: Transcribe audio using AI sampling (2025-03-26)")
	fmt.Println("  - priority_analysis: Analyze text with priority sampling")
	fmt.Println("  - multimodal_analysis: Analyze multiple content types")
	fmt.Println("  - test_model_preferences: Test different model strategies")
	fmt.Println("📋 Available resources:")
	fmt.Println("  - /sampling/capabilities: Get server sampling capabilities")

	// Run the server
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run server: %v\n", err)
	}
}
