package test

import (
	"testing"

	"github.com/localrivet/gomcp/mcp"
	"github.com/localrivet/gomcp/server"
)

func TestServerListMethods(t *testing.T) {
	// Create a test server
	srv := server.NewServer("test-server")

	// Add some test tools
	srv.Tool("calculator", "Perform mathematical calculations", func(ctx *server.Context, args struct {
		Operation string  `json:"operation"`
		A         float64 `json:"a"`
		B         float64 `json:"b"`
	}) (float64, error) {
		switch args.Operation {
		case "add":
			return args.A + args.B, nil
		case "subtract":
			return args.A - args.B, nil
		default:
			return 0, nil
		}
	}).WithAnnotations("calculator", map[string]interface{}{
		"category": "math",
		"icon":     "calculator",
	})

	srv.Tool("echo", "Echo the input text", func(ctx *server.Context, args struct {
		Text string `json:"text"`
	}) (string, error) {
		return args.Text, nil
	})

	// Add some test resources (simplified without parameters)
	srv.Resource("/users", "Get user information", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return map[string]interface{}{
			"users": []map[string]interface{}{
				{"id": 1, "name": "Test User 1"},
				{"id": 2, "name": "Test User 2"},
			},
		}, nil
	})

	srv.Resource("/posts", "List all posts", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return []map[string]interface{}{
			{"id": 1, "title": "First Post"},
			{"id": 2, "title": "Second Post"},
		}, nil
	})

	// Add some test prompts
	srv.Prompt("greeting", "A friendly greeting", "Hello, {{name}}! How are you today?")
	srv.Prompt("summary", "Summarize content", "Please summarize the following content:\n\n{{content}}")

	// Test ListTools
	t.Run("ListTools", func(t *testing.T) {
		tools, err := srv.ListTools()
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) != 2 {
			t.Fatalf("Expected 2 tools, got %d", len(tools))
		}

		// Check calculator tool
		var calculatorTool *mcp.Tool
		var echoTool *mcp.Tool
		for i := range tools {
			if tools[i].Name == "calculator" {
				calculatorTool = &tools[i]
			} else if tools[i].Name == "echo" {
				echoTool = &tools[i]
			}
		}

		if calculatorTool == nil {
			t.Fatal("Calculator tool not found")
		}
		if calculatorTool.Description != "Perform mathematical calculations" {
			t.Errorf("Expected calculator description 'Perform mathematical calculations', got '%s'", calculatorTool.Description)
		}
		if calculatorTool.InputSchema == nil {
			t.Error("Calculator tool should have input schema")
		}
		if calculatorTool.Annotations == nil {
			t.Error("Calculator tool should have annotations")
		}
		if category, ok := calculatorTool.Annotations["category"]; !ok || category != "math" {
			t.Error("Calculator tool should have category annotation set to 'math'")
		}

		if echoTool == nil {
			t.Fatal("Echo tool not found")
		}
		if echoTool.Description != "Echo the input text" {
			t.Errorf("Expected echo description 'Echo the input text', got '%s'", echoTool.Description)
		}
		if echoTool.InputSchema == nil {
			t.Error("Echo tool should have input schema")
		}
	})

	// Test ListResources
	t.Run("ListResources", func(t *testing.T) {
		resources, err := srv.ListResources()
		if err != nil {
			t.Fatalf("ListResources failed: %v", err)
		}

		if len(resources) != 2 {
			t.Fatalf("Expected 2 resources, got %d", len(resources))
		}

		// Check that we have the expected resources
		var usersResource *mcp.Resource
		var postsResource *mcp.Resource
		for i := range resources {
			if resources[i].URI == "/users" {
				usersResource = &resources[i]
			} else if resources[i].URI == "/posts" {
				postsResource = &resources[i]
			}
		}

		if usersResource == nil {
			t.Fatal("Users resource not found")
		}
		if usersResource.Description != "Get user information" {
			t.Errorf("Expected users description 'Get user information', got '%s'", usersResource.Description)
		}

		if postsResource == nil {
			t.Fatal("Posts resource not found")
		}
		if postsResource.Description != "List all posts" {
			t.Errorf("Expected posts description 'List all posts', got '%s'", postsResource.Description)
		}
	})

	// Test ListPrompts
	t.Run("ListPrompts", func(t *testing.T) {
		prompts, err := srv.ListPrompts()
		if err != nil {
			t.Fatalf("ListPrompts failed: %v", err)
		}

		if len(prompts) != 2 {
			t.Fatalf("Expected 2 prompts, got %d", len(prompts))
		}

		// Check that we have the expected prompts
		var greetingPrompt *mcp.Prompt
		var summaryPrompt *mcp.Prompt
		for i := range prompts {
			if prompts[i].Name == "greeting" {
				greetingPrompt = &prompts[i]
			} else if prompts[i].Name == "summary" {
				summaryPrompt = &prompts[i]
			}
		}

		if greetingPrompt == nil {
			t.Fatal("Greeting prompt not found")
		}
		if greetingPrompt.Description != "A friendly greeting" {
			t.Errorf("Expected greeting description 'A friendly greeting', got '%s'", greetingPrompt.Description)
		}

		if summaryPrompt == nil {
			t.Fatal("Summary prompt not found")
		}
		if summaryPrompt.Description != "Summarize content" {
			t.Errorf("Expected summary description 'Summarize content', got '%s'", summaryPrompt.Description)
		}
	})
}

func TestServerListMethodsEmpty(t *testing.T) {
	// Create a server with no tools, resources, or prompts
	srv := server.NewServer("empty-server")

	// Test ListTools with empty server
	t.Run("ListToolsEmpty", func(t *testing.T) {
		tools, err := srv.ListTools()
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) != 0 {
			t.Fatalf("Expected 0 tools, got %d", len(tools))
		}
	})

	// Test ListResources with empty server
	t.Run("ListResourcesEmpty", func(t *testing.T) {
		resources, err := srv.ListResources()
		if err != nil {
			t.Fatalf("ListResources failed: %v", err)
		}

		if len(resources) != 0 {
			t.Fatalf("Expected 0 resources, got %d", len(resources))
		}
	})

	// Test ListPrompts with empty server
	t.Run("ListPromptsEmpty", func(t *testing.T) {
		prompts, err := srv.ListPrompts()
		if err != nil {
			t.Fatalf("ListPrompts failed: %v", err)
		}

		if len(prompts) != 0 {
			t.Fatalf("Expected 0 prompts, got %d", len(prompts))
		}
	})
}

func TestServerListMethodsConsistency(t *testing.T) {
	// Test that the list methods return the same data as the Process* methods
	srv := server.NewServer("consistency-test")

	// Add a tool with all possible fields
	srv.Tool("test_tool", "Test tool for consistency", func(ctx *server.Context, args struct {
		Input string `json:"input"`
	}) (string, error) {
		return args.Input, nil
	}).WithAnnotations("test_tool", map[string]interface{}{
		"test": "annotation",
	})

	// Test that ListTools returns consistent data
	t.Run("ToolsConsistency", func(t *testing.T) {
		tools, err := srv.ListTools()
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) != 1 {
			t.Fatalf("Expected 1 tool, got %d", len(tools))
		}

		tool := tools[0]
		if tool.Name != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got '%s'", tool.Name)
		}
		if tool.Description != "Test tool for consistency" {
			t.Errorf("Expected tool description 'Test tool for consistency', got '%s'", tool.Description)
		}
		if tool.InputSchema == nil {
			t.Error("Tool should have input schema")
		}
		if tool.Annotations == nil {
			t.Error("Tool should have annotations")
		}
		if testAnnotation, ok := tool.Annotations["test"]; !ok || testAnnotation != "annotation" {
			t.Error("Tool should have test annotation")
		}
	})
}
