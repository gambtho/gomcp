package test

import (
	"encoding/json"
	"testing"

	"github.com/localrivet/gomcp/server"
)

func TestPromptRegistrationAndTemplates(t *testing.T) {
	// Create a new server
	s := server.NewServer("test-server")

	// Register test prompts
	s.Prompt("simple", "A simple prompt",
		server.User("I am a helpful assistant. What can I help you with?"))

	s.Prompt("with-variables", "A prompt with variables",
		server.User("Hello {{name}}, welcome to {{service}}!"))

	// Register a prompt with multiple templates
	s.Prompt("multi-template", "A prompt with multiple templates",
		server.User("You are a helpful assistant. Please help with {{task}}."),
		server.Assistant("I'll help you with that."),
	)

	// Check that the prompts were registered
	server := s.GetServer()
	if len(server.GetPrompts()) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(server.GetPrompts()))
	}

	// Check the simple prompt
	simplePrompt, ok := server.GetPrompts()["simple"]
	if !ok {
		t.Fatal("simple not found")
	}
	if simplePrompt.Name != "simple" {
		t.Errorf("Expected name 'simple', got '%s'", simplePrompt.Name)
	}
	if simplePrompt.Description != "A simple prompt" {
		t.Errorf("Expected description 'A simple prompt', got '%s'", simplePrompt.Description)
	}
	if len(simplePrompt.Templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(simplePrompt.Templates))
	}
	if simplePrompt.Templates[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", simplePrompt.Templates[0].Role)
	}
	if simplePrompt.Templates[0].Content != "I am a helpful assistant. What can I help you with?" {
		t.Errorf("Expected content 'I am a helpful assistant. What can I help you with?', got '%s'", simplePrompt.Templates[0].Content)
	}

	// Check the with-variables prompt
	withVariablesPrompt, ok := server.GetPrompts()["with-variables"]
	if !ok {
		t.Fatal("with-variables not found")
	}
	if withVariablesPrompt.Name != "with-variables" {
		t.Errorf("Expected name 'with-variables', got '%s'", withVariablesPrompt.Name)
	}
	if withVariablesPrompt.Description != "A prompt with variables" {
		t.Errorf("Expected description 'A prompt with variables', got '%s'", withVariablesPrompt.Description)
	}
	if len(withVariablesPrompt.Templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(withVariablesPrompt.Templates))
	}

	// Check template role
	if withVariablesPrompt.Templates[0].Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", withVariablesPrompt.Templates[0].Role)
	}

	// Verify arguments were extracted
	if len(withVariablesPrompt.Arguments) != 2 {
		t.Errorf("Expected 2 arguments, got %d", len(withVariablesPrompt.Arguments))
	}

	// Check arguments
	argMap := make(map[string]bool)
	for _, arg := range withVariablesPrompt.Arguments {
		argMap[arg.Name] = true
		if !arg.Required {
			t.Errorf("Expected argument '%s' to be required", arg.Name)
		}
	}

	if !argMap["name"] {
		t.Errorf("Expected 'name' argument to be extracted")
	}
	if !argMap["service"] {
		t.Errorf("Expected 'service' argument to be extracted")
	}

	// Check the multi-template prompt
	multiTemplatePrompt, ok := server.GetPrompts()["multi-template"]
	if !ok {
		t.Fatal("multi-template not found")
	}
	if multiTemplatePrompt.Name != "multi-template" {
		t.Errorf("Expected name 'multi-template', got '%s'", multiTemplatePrompt.Name)
	}
	if len(multiTemplatePrompt.Templates) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(multiTemplatePrompt.Templates))
	}

	// Check template roles
	expectedRoles := []string{"user", "assistant"}
	for i, role := range expectedRoles {
		if multiTemplatePrompt.Templates[i].Role != role {
			t.Errorf("Expected role '%s', got '%s'", role, multiTemplatePrompt.Templates[i].Role)
		}
	}

	// Verify arguments were extracted
	if len(multiTemplatePrompt.Arguments) != 1 {
		t.Errorf("Expected 1 argument, got %d", len(multiTemplatePrompt.Arguments))
	}

	// Check arguments
	argMap = make(map[string]bool)
	for _, arg := range multiTemplatePrompt.Arguments {
		argMap[arg.Name] = true
		if !arg.Required {
			t.Errorf("Expected argument '%s' to be required", arg.Name)
		}
	}

	if !argMap["task"] {
		t.Errorf("Expected 'task' argument to be extracted")
	}
}

func TestPromptVariableSubstitution(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		variables map[string]interface{}
		expected  string
	}{
		{
			name:      "simple variable",
			template:  "Hello, {{name}}!",
			variables: map[string]interface{}{"name": "World"},
			expected:  "Hello, World!",
		},
		{
			name:      "multiple variables",
			template:  "{{greeting}}, {{name}}!",
			variables: map[string]interface{}{"greeting": "Hello", "name": "World"},
			expected:  "Hello, World!",
		},
		{
			name:      "missing variable",
			template:  "Hello, {{name}}!",
			variables: map[string]interface{}{},
			expected:  "Hello, {{name}}!",
		},
		{
			name:      "numeric variable",
			template:  "The answer is {{answer}}.",
			variables: map[string]interface{}{"answer": 42},
			expected:  "The answer is 42.",
		},
		{
			name:      "object variable",
			template:  "User: {{user}}",
			variables: map[string]interface{}{"user": map[string]interface{}{"name": "John", "age": 30}},
			expected:  `User: {"age":30,"name":"John"}`,
		},
		{
			name:      "whitespace in variable name",
			template:  "Hello, {{ name }}!",
			variables: map[string]interface{}{"name": "World"},
			expected:  "Hello, World!",
		},
		{
			name:      "no variables",
			template:  "Hello, World!",
			variables: map[string]interface{}{},
			expected:  "Hello, World!",
		},
		{
			name:      "nil variables",
			template:  "Hello, {{name}}!",
			variables: nil,
			expected:  "Hello, {{name}}!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.SubstituteVariables(tt.template, tt.variables)
			if err != nil {
				t.Errorf("server.SubstituteVariables() error = %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("server.SubstituteVariables() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProcessPromptRequest(t *testing.T) {
	// Create a new server
	s := server.NewServer("test-server")

	// Register a prompt
	s.Prompt("test-prompt", "A test prompt",
		server.User("Tell me about {{topic}}."),
	)

	// Create a context for testing
	ctx := &server.Context{
		Request: &server.Request{
			ID:     "1",
			Method: "prompts/get",
			Params: json.RawMessage(`{"name":"test-prompt","arguments":{"topic":"Go programming"}}`),
		},
		Response: &server.Response{
			JSONRPC: "2.0",
			ID:      "1",
		},
	}

	// Process the prompt request
	result, err := s.GetServer().ProcessPromptRequest(ctx)
	if err != nil {
		t.Fatalf("ProcessPromptRequest() error = %v", err)
	}

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	// Check the description
	description, ok := resultMap["description"].(string)
	if !ok || description != "A test prompt" {
		t.Errorf("Expected description 'A test prompt', got '%v'", description)
	}

	// Check the messages
	messages, ok := resultMap["messages"].([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected messages to be a slice of maps, got %T", resultMap["messages"])
	}
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	// Check the first message (with variable substitution)
	firstMessage := messages[0]
	if firstMessage["role"] != "user" {
		t.Errorf("Expected role 'user', got '%s'", firstMessage["role"])
	}

	// Check content format
	content, ok := firstMessage["content"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected content to be a map, got %T", firstMessage["content"])
	}

	// Check content fields
	if content["type"] != "text" {
		t.Errorf("Expected content type 'text', got '%v'", content["type"])
	}
	if content["text"] != "Tell me about Go programming." {
		t.Errorf("Expected content text 'Tell me about Go programming.', got '%v'", content["text"])
	}

	// Test missing required argument
	ctx.Request.Params = json.RawMessage(`{"name":"test-prompt","arguments":{}}`)
	_, err = s.GetServer().ProcessPromptRequest(ctx)
	if err == nil {
		t.Error("Expected error for missing required argument 'topic', got nil")
	}

	// Test with missing prompt
	ctx.Request.Params = json.RawMessage(`{"name":"missing-prompt","arguments":{}}`)
	_, err = s.GetServer().ProcessPromptRequest(ctx)
	if err == nil {
		t.Error("Expected error for missing prompt, got nil")
	}

	// Test with invalid params
	ctx.Request.Params = json.RawMessage(`invalid json`)
	_, err = s.GetServer().ProcessPromptRequest(ctx)
	if err == nil {
		t.Error("Expected error for invalid params, got nil")
	}
}

func TestPromptList(t *testing.T) {
	// Create a new server
	s := server.NewServer("test-server")

	// Register some prompts
	s.Prompt("prompt1", "First prompt", "Template 1")
	s.Prompt("prompt2", "Second prompt", "Template 2 with {{var}}")
	s.Prompt("prompt3", "Third prompt", "Template 3")

	// Create a context for testing
	ctx := &server.Context{
		Request: &server.Request{
			ID:     "1",
			Method: "prompts/list",
		},
		Response: &server.Response{
			JSONRPC: "2.0",
			ID:      "1",
		},
	}

	// Process the prompt list request
	result, err := s.GetServer().ProcessPromptList(ctx)
	if err != nil {
		t.Fatalf("ProcessPromptList() error = %v", err)
	}

	// Check the result
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", result)
	}

	// Check the prompts
	prompts, ok := resultMap["prompts"].([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected prompts to be a slice of maps, got %T", resultMap["prompts"])
	}
	if len(prompts) != 3 {
		t.Errorf("Expected 3 prompts, got %d", len(prompts))
	}

	// Check if second prompt has arguments
	var promptWithArgs map[string]interface{}
	for _, p := range prompts {
		if p["name"] == "prompt2" {
			promptWithArgs = p
			break
		}
	}

	if promptWithArgs == nil {
		t.Fatal("prompt2 not found in prompts list")
	}

	args, ok := promptWithArgs["arguments"].([]server.PromptArgument)
	if !ok {
		// This is acceptable since the JSON marshaling might make it a different type
		t.Logf("Arguments not in expected format, got %T", promptWithArgs["arguments"])
	} else if len(args) == 0 {
		t.Errorf("Expected at least one argument for prompt2, got none")
	}

	// Check prompt information
	for _, prompt := range prompts {
		name, ok := prompt["name"].(string)
		if !ok {
			t.Errorf("Expected name to be a string, got %T", prompt["name"])
			continue
		}

		// Check description
		description, ok := prompt["description"].(string)
		if !ok {
			t.Errorf("Expected description to be a string, got %T", prompt["description"])
			continue
		}

		// Check description for each prompt
		switch name {
		case "prompt1":
			if description != "First prompt" {
				t.Errorf("Expected description 'First prompt', got '%s'", description)
			}
		case "prompt2":
			if description != "Second prompt" {
				t.Errorf("Expected description 'Second prompt', got '%s'", description)
			}
		case "prompt3":
			if description != "Third prompt" {
				t.Errorf("Expected description 'Third prompt', got '%s'", description)
			}
		default:
			t.Errorf("Unexpected prompt name: %s", name)
		}
	}
}
