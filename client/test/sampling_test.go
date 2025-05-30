package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/localrivet/gomcp/client"
)

// TestClientHandleSamplingCreateMessage tests the client's handling of sampling/createMessage requests
func TestClientHandleSamplingCreateMessage(t *testing.T) {
	// Create a mock transport with proper initialization
	mockTransport := SetupMockTransport("2024-11-05")
	EnsureConnected(mockTransport)

	// Create a client
	c, err := client.NewClient("test-client",
		client.WithTransport(mockTransport),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Test that we can register a sampling handler
	var receivedParams client.SamplingCreateMessageParams
	handlerCalled := false

	SetSamplingHandler(c, func(params client.SamplingCreateMessageParams) (client.SamplingResponse, error) {
		receivedParams = params
		handlerCalled = true
		return client.SamplingResponse{
			Role: "assistant",
			Content: client.SamplingMessageContent{
				Type: "text",
				Text: "Test response from sampling handler",
			},
		}, nil
	})

	// Verify the handler was registered
	handler := c.GetSamplingHandler()
	if handler == nil {
		t.Fatal("Sampling handler was not registered")
	}

	// Test calling the handler directly
	testParams := client.SamplingCreateMessageParams{
		Messages: []client.SamplingMessage{
			{
				Role: "user",
				Content: client.SamplingMessageContent{
					Type: "text",
					Text: "Hello, how are you?",
				},
			},
		},
		ModelPreferences: client.SamplingModelPreferences{},
		SystemPrompt:     "You are a helpful assistant",
		MaxTokens:        100,
	}

	response, err := handler(testParams)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	// Verify the handler was called
	if !handlerCalled {
		t.Error("Sampling handler was not called")
	}

	// Verify the response
	if response.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", response.Role)
	}
	if response.Content.Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", response.Content.Type)
	}
	if response.Content.Text != "Test response from sampling handler" {
		t.Errorf("Expected text 'Test response from sampling handler', got '%s'", response.Content.Text)
	}

	// Verify the parameters were received correctly
	if len(receivedParams.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(receivedParams.Messages))
	}

	if receivedParams.Messages[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", receivedParams.Messages[0].Role)
	}

	if receivedParams.Messages[0].Content.Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", receivedParams.Messages[0].Content.Type)
	}

	if receivedParams.Messages[0].Content.Text != "Hello, how are you?" {
		t.Errorf("Expected text 'Hello, how are you?', got '%s'", receivedParams.Messages[0].Content.Text)
	}

	if receivedParams.SystemPrompt != "You are a helpful assistant" {
		t.Errorf("Expected system prompt 'You are a helpful assistant', got '%s'", receivedParams.SystemPrompt)
	}

	if receivedParams.MaxTokens != 100 {
		t.Errorf("Expected max tokens 100, got %d", receivedParams.MaxTokens)
	}
}

// TestTextSamplingContent tests the TextSamplingContent type
func TestTextSamplingContent(t *testing.T) {
	// Test creating text content
	textContent := &client.TextSamplingContent{
		Text: "Hello, world!",
	}

	// Test validation
	if err := textContent.Validate(); err != nil {
		t.Errorf("Valid text content failed validation: %v", err)
	}

	// Test empty text validation
	emptyContent := &client.TextSamplingContent{
		Text: "",
	}
	if err := emptyContent.Validate(); err == nil {
		t.Error("Empty text content should fail validation")
	}

	// Test conversion to message content
	msgContent := textContent.ToMessageContent()
	if msgContent.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", msgContent.Type)
	}
	if msgContent.Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got '%s'", msgContent.Text)
	}
}

// TestImageSamplingContent tests the ImageSamplingContent type
func TestImageSamplingContent(t *testing.T) {
	// Test creating image content
	imageContent := &client.ImageSamplingContent{
		Data:     "base64encodeddata",
		MimeType: "image/png",
	}

	// Test validation
	if err := imageContent.Validate(); err != nil {
		t.Errorf("Valid image content failed validation: %v", err)
	}

	// Test empty data validation
	emptyDataContent := &client.ImageSamplingContent{
		Data:     "",
		MimeType: "image/png",
	}
	if err := emptyDataContent.Validate(); err == nil {
		t.Error("Image content with empty data should fail validation")
	}

	// Test empty mime type validation
	emptyMimeContent := &client.ImageSamplingContent{
		Data:     "base64encodeddata",
		MimeType: "",
	}
	if err := emptyMimeContent.Validate(); err == nil {
		t.Error("Image content with empty mime type should fail validation")
	}

	// Test conversion to message content
	msgContent := imageContent.ToMessageContent()
	if msgContent.Type != "image" {
		t.Errorf("Expected type 'image', got '%s'", msgContent.Type)
	}
	if msgContent.Data != "base64encodeddata" {
		t.Errorf("Expected data 'base64encodeddata', got '%s'", msgContent.Data)
	}
	if msgContent.MimeType != "image/png" {
		t.Errorf("Expected mime type 'image/png', got '%s'", msgContent.MimeType)
	}
}

// TestAudioSamplingContent tests the AudioSamplingContent type
func TestAudioSamplingContent(t *testing.T) {
	// Test creating audio content
	audioContent := &client.AudioSamplingContent{
		Data:     "base64encodedaudio",
		MimeType: "audio/wav",
	}

	// Test validation
	if err := audioContent.Validate(); err != nil {
		t.Errorf("Valid audio content failed validation: %v", err)
	}

	// Test empty data validation
	emptyDataContent := &client.AudioSamplingContent{
		Data:     "",
		MimeType: "audio/wav",
	}
	if err := emptyDataContent.Validate(); err == nil {
		t.Error("Audio content with empty data should fail validation")
	}

	// Test empty mime type validation
	emptyMimeContent := &client.AudioSamplingContent{
		Data:     "base64encodedaudio",
		MimeType: "",
	}
	if err := emptyMimeContent.Validate(); err == nil {
		t.Error("Audio content with empty mime type should fail validation")
	}

	// Test conversion to message content
	msgContent := audioContent.ToMessageContent()
	if msgContent.Type != "audio" {
		t.Errorf("Expected type 'audio', got '%s'", msgContent.Type)
	}
	if msgContent.Data != "base64encodedaudio" {
		t.Errorf("Expected data 'base64encodedaudio', got '%s'", msgContent.Data)
	}
	if msgContent.MimeType != "audio/wav" {
		t.Errorf("Expected mime type 'audio/wav', got '%s'", msgContent.MimeType)
	}
}

// TestValidateContentForVersion tests content validation against protocol versions
func TestValidateContentForVersion(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		version     string
		shouldPass  bool
	}{
		{"text content in draft", "text", "draft", true},
		{"text content in 2024-11-05", "text", "2024-11-05", true},
		{"text content in 2025-03-26", "text", "2025-03-26", true},
		{"image content in draft", "image", "draft", true},
		{"image content in 2024-11-05", "image", "2024-11-05", true},
		{"image content in 2025-03-26", "image", "2025-03-26", true},
		{"audio content in draft", "audio", "draft", true},
		{"audio content in 2024-11-05", "audio", "2024-11-05", false}, // Audio not supported in 2024-11-05
		{"audio content in 2025-03-26", "audio", "2025-03-26", true},
		{"unknown content in any version", "unknown", "draft", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := client.SamplingMessageContent{
				Type: tc.contentType,
				Text: "test",
			}

			isValid := content.IsValidForVersion(tc.version)
			if isValid != tc.shouldPass {
				t.Errorf("Expected validation result %v for %s in version %s, got %v",
					tc.shouldPass, tc.contentType, tc.version, isValid)
			}
		})
	}
}

// TestCreateSamplingMessage tests creating sampling messages
func TestCreateSamplingMessage(t *testing.T) {
	// Test creating text message
	textMsg := client.CreateTextSamplingMessage("user", "Hello, world!")
	if textMsg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", textMsg.Role)
	}
	if textMsg.Content.Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", textMsg.Content.Type)
	}
	if textMsg.Content.Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got '%s'", textMsg.Content.Text)
	}

	// Test creating image message
	imageMsg := client.CreateImageSamplingMessage("user", "base64data", "image/png")
	if imageMsg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", imageMsg.Role)
	}
	if imageMsg.Content.Type != "image" {
		t.Errorf("Expected content type 'image', got '%s'", imageMsg.Content.Type)
	}
	if imageMsg.Content.Data != "base64data" {
		t.Errorf("Expected data 'base64data', got '%s'", imageMsg.Content.Data)
	}
	if imageMsg.Content.MimeType != "image/png" {
		t.Errorf("Expected mime type 'image/png', got '%s'", imageMsg.Content.MimeType)
	}

	// Test creating audio message
	audioMsg := client.CreateAudioSamplingMessage("user", "base64audio", "audio/wav")
	if audioMsg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", audioMsg.Role)
	}
	if audioMsg.Content.Type != "audio" {
		t.Errorf("Expected content type 'audio', got '%s'", audioMsg.Content.Type)
	}
	if audioMsg.Content.Data != "base64audio" {
		t.Errorf("Expected data 'base64audio', got '%s'", audioMsg.Content.Data)
	}
	if audioMsg.Content.MimeType != "audio/wav" {
		t.Errorf("Expected mime type 'audio/wav', got '%s'", audioMsg.Content.MimeType)
	}
}

// TestSamplingRequest tests the SamplingRequest functionality
func TestSamplingRequest(t *testing.T) {
	// Create test messages
	messages := []client.SamplingMessage{
		client.CreateTextSamplingMessage("user", "Hello"),
		client.CreateTextSamplingMessage("assistant", "Hi there!"),
	}

	// Create model preferences
	prefs := client.SamplingModelPreferences{
		Hints: []client.SamplingModelHint{
			{Name: "gpt-4"},
		},
	}

	// Create a sampling request
	req := client.NewSamplingRequest(messages, prefs)
	req.SystemPrompt = "You are a helpful assistant"
	req.MaxTokens = 150

	// Test validation
	if err := req.Validate(); err != nil {
		t.Errorf("Valid sampling request failed validation: %v", err)
	}

	// Test with context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	if req.Context != ctx {
		t.Error("Context was not set correctly")
	}

	// Test with timeout
	req = req.WithTimeout(10 * time.Second)
	if req.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", req.Timeout)
	}

	// Test building request
	requestJSON, err := req.BuildCreateMessageRequest(123)
	if err != nil {
		t.Errorf("Failed to build request: %v", err)
	}

	// Parse the request to verify structure
	var parsedRequest map[string]interface{}
	if err := json.Unmarshal(requestJSON, &parsedRequest); err != nil {
		t.Errorf("Failed to parse built request: %v", err)
	}

	// Verify request structure
	if parsedRequest["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got '%v'", parsedRequest["jsonrpc"])
	}
	if parsedRequest["method"] != "sampling/createMessage" {
		t.Errorf("Expected method 'sampling/createMessage', got '%v'", parsedRequest["method"])
	}
	if parsedRequest["id"] != float64(123) {
		t.Errorf("Expected id 123, got %v", parsedRequest["id"])
	}

	// Verify params
	params, ok := parsedRequest["params"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected params to be an object")
	}

	if params["systemPrompt"] != "You are a helpful assistant" {
		t.Errorf("Expected system prompt 'You are a helpful assistant', got '%v'", params["systemPrompt"])
	}
	if params["maxTokens"] != float64(150) {
		t.Errorf("Expected max tokens 150, got %v", params["maxTokens"])
	}
}
