package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/util/schema"
)

// Tool represents a tool registered with the server.
// Tools are functions that clients can call to perform specific operations.
type Tool struct {
	// Name is the unique identifier for the tool
	Name string

	// Description explains what the tool does
	Description string

	// Handler is the function that executes when the tool is called
	Handler interface{}

	// Schema defines the expected input format for the tool
	Schema interface{}

	// Annotations contains additional metadata about the tool
	Annotations map[string]interface{}
}

// Tool registers a tool with the server.
// The name parameter is used as the identifier for the tool.
// The description parameter explains what the tool does.
// The handler parameter must be a function with signature: func(ctx *Context, args *StructType) (interface{}, error)
// where StructType is a pointer to a struct (nillable).
// The annotations parameter allows you to add metadata directly during registration.
func (s *serverImpl) Tool(name, description string, handler interface{}, annotations ...map[string]interface{}) Server {
	// Validate handler is not nil
	if handler == nil {
		s.logger.Error("tool handler cannot be nil", "name", name)
		return s
	}

	// Validate that handler is a function and its args parameter is a struct, *struct, or nil
	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() != reflect.Func {
		s.logger.Error("tool handler must be a function", "name", name)
		return s
	}

	// Must have exactly 2 parameters
	if handlerType.NumIn() != 2 {
		s.logger.Error("tool handler must have exactly 2 parameters: func(ctx *Context, args StructType) (interface{}, error)", "name", name)
		return s
	}

	// Check that args parameter (second parameter) is a struct, *struct, or interface{} (for nil case)
	argsType := handlerType.In(1)
	argsKind := argsType.Kind()

	// Allow struct, pointer to struct, or interface{} (for nil case)
	isValidArgsType := argsKind == reflect.Struct ||
		(argsKind == reflect.Ptr && argsType.Elem().Kind() == reflect.Struct) ||
		argsType == reflect.TypeOf((*interface{})(nil)).Elem()

	if !isValidArgsType {
		s.logger.Error("tool handler args parameter must be a struct, *struct, or interface{} (for nil), got", "name", name, "type", argsType.String())
		return s
	}

	// Validate handler signature and extract schema
	handlerFunc, schema, err := s.validateAndExtractToolHandler(handler)
	if err != nil {
		s.logger.Error("invalid tool handler", "name", name, "error", err)
		return s
	}

	// Merge all annotation maps
	mergedAnnotations := make(map[string]interface{})
	for _, annotationMap := range annotations {
		for k, v := range annotationMap {
			mergedAnnotations[k] = v
		}
	}

	// Use the internal registerTool method to store the tool
	s.registerTool(name, description, handlerFunc, schema, mergedAnnotations)
	return s
}

// validateAndExtractToolHandler validates a handler function and extracts its schema.
// The handler must be a function with signature: func(ctx *Context, args *StructType) (interface{}, error)
// where StructType is a struct type (either by value or pointer) for proper schema generation.
func (s *serverImpl) validateAndExtractToolHandler(handler interface{}) (interface{}, map[string]interface{}, error) {
	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	// Must be a function
	if handlerType.Kind() != reflect.Func {
		return nil, nil, errors.New("handler must be a function")
	}

	// Must have exactly 2 parameters and 2 return values
	if handlerType.NumIn() != 2 || handlerType.NumOut() != 2 {
		return nil, nil, errors.New("handler must have signature: func(ctx *Context, args StructType) (interface{}, error) where StructType is a struct type")
	}

	// First parameter must be *Context
	if handlerType.In(0) != reflect.TypeOf((*Context)(nil)) {
		return nil, nil, errors.New("first parameter must be *Context")
	}

	// First return value must be assignable to interface{}
	if !handlerType.Out(0).AssignableTo(reflect.TypeOf((*interface{})(nil)).Elem()) {
		return nil, nil, errors.New("first return value must be assignable to interface{}")
	}

	// Second return value must be error
	if !handlerType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, nil, errors.New("second return value must be error")
	}

	// Validate second parameter type - must be a struct or pointer to struct for schema generation
	argsType := handlerType.In(1)

	// Check if it's interface{} - this is allowed for tools that don't need arguments (nil case)
	if argsType == reflect.TypeOf((*interface{})(nil)).Elem() {
		// For interface{} (nil) arguments, create a simple empty schema
		emptySchema := map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		}

		// Create a wrapper that handles nil arguments
		wrappedHandler := func(ctx *Context, args interface{}) (interface{}, error) {
			// Call the original handler using reflection
			results := handlerValue.Call([]reflect.Value{
				reflect.ValueOf(ctx),
				reflect.ValueOf(args), // args will be nil for tools that don't need arguments
			})

			// Extract the results
			var resultValue interface{}
			var errValue error

			if !results[0].IsNil() {
				resultValue = results[0].Interface()
			}

			if !results[1].IsNil() {
				errValue = results[1].Interface().(error)
			}

			return resultValue, errValue
		}

		return wrappedHandler, emptySchema, nil
	}

	// Check if it's map[string]interface{} - these are not allowed
	if argsType == reflect.TypeOf(map[string]interface{}{}) {
		return nil, nil, errors.New("tool handler second parameter cannot be map[string]interface{} - must use a struct type for proper schema generation")
	}

	// Extract schema from struct parameter type
	var schemaMap map[string]interface{}
	paramType := argsType
	isPointer := false

	if paramType.Kind() == reflect.Ptr {
		paramType = paramType.Elem()
		isPointer = true
	}

	// Must be a struct type for schema generation (including anonymous structs)
	if paramType.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("tool handler second parameter must be a struct type (or pointer to struct) for proper schema generation, got %s (kind: %s)", paramType.String(), paramType.Kind().String())
	}

	// Create an instance for schema generation
	var structInstance interface{}
	if isPointer {
		structInstance = reflect.New(paramType).Interface()
	} else {
		structInstance = reflect.New(paramType).Elem().Interface()
	}

	// Generate schema from the struct
	generator := schema.NewGenerator()
	var err error
	schemaMap, err = generator.GenerateSchema(structInstance)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate schema from struct: %w", err)
	}

	// Create a wrapper that converts the specific handler to work with our validation system
	wrappedHandler := func(ctx *Context, args interface{}) (interface{}, error) {
		// Convert args map to the correct struct type using schema validation
		argsMap, ok := args.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("args must be a map[string]interface{}, got %T", args)
		}

		// Validate and convert the arguments to the expected type
		convertedArgs, err := schema.ValidateAndConvertArgs(schemaMap, argsMap, argsType)
		if err != nil {
			return nil, fmt.Errorf("argument validation failed: %w", err)
		}

		// Call the original handler using reflection with the converted args
		results := handlerValue.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(convertedArgs),
		})

		// Extract the results
		var resultValue interface{}
		var errValue error

		if !results[0].IsNil() {
			resultValue = results[0].Interface()
		}

		if !results[1].IsNil() {
			errValue = results[1].Interface().(error)
		}

		return resultValue, errValue
	}

	return wrappedHandler, schemaMap, nil
}

// registerTool is an internal method that stores a tool in the server's registry.
// It handles the actual registration logic and manages tool metadata.
// This method is called by the public Tool method after validation.
func (s *serverImpl) registerTool(name string, description string, handler interface{}, schema map[string]interface{}, annotations map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		s.logger.Error("tool name cannot be empty")
		return
	}

	// Create the tool
	tool := &Tool{
		Name:        name,
		Description: description,
		Handler:     handler,
		Schema:      schema,
		Annotations: annotations,
	}

	// Store the tool
	s.tools[name] = tool

	// Emit tool registration event
	go func() {
		events.Publish[events.ToolRegisteredEvent](s.events, events.TopicToolRegistered, events.ToolRegisteredEvent{
			ToolName:     name,
			Description:  description,
			RegisteredAt: time.Now(),
			Schema:       schema,
			Annotations:  annotations,
		})
	}()

	// Mark tools as changed for potential notifications
	s.capabilityCache.MarkToolsChanged()

	// Send simple notification if client is already initialized
	s.sendCapabilityNotification("tools")

	s.logger.Debug("tool registered", "name", name, "description", description)
}

// ProcessToolList processes a tool list request and returns the list of available tools.
// It supports pagination through an optional cursor parameter.
// The response includes the tools' name, description, and input schema.
func (s *serverImpl) ProcessToolList(ctx *Context) (interface{}, error) {
	// Get pagination cursor if provided
	var cursor string
	if ctx.Request.Params != nil {
		var params struct {
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
		cursor = params.Cursor
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// For now, we'll use a simple pagination that returns all tools
	// In a real implementation, you'd parse the cursor and limit results
	const maxPageSize = 50
	var tools = make([]map[string]interface{}, 0, len(s.tools))
	var nextCursor string

	// Convert tools to the expected format
	i := 0
	for name, tool := range s.tools {
		// If we have a cursor, skip until we find it
		// This is a simplistic approach; real cursor would be more sophisticated
		if cursor != "" && name <= cursor {
			continue
		}

		// Add the tool to the result
		toolInfo := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.Schema,
		}

		// Only include annotations if they exist
		if len(tool.Annotations) > 0 {
			toolInfo["annotations"] = tool.Annotations
		}

		tools = append(tools, toolInfo)

		i++
		if i >= maxPageSize {
			// Set cursor for next page
			nextCursor = name
			break
		}
	}

	// Return the list of tools
	result := map[string]interface{}{
		"tools": tools,
	}

	// Only add nextCursor if there are more results
	if nextCursor != "" {
		result["nextCursor"] = nextCursor
	}

	return result, nil
}

// executeTool executes a registered tool with the given arguments.
// It handles argument validation, conversion, and execution of the tool handler.
// Returns the result from the tool handler or an error if execution fails.
func (s *serverImpl) executeTool(ctx *Context, name string, args map[string]interface{}) (interface{}, error) {
	// First get the tool without holding any locks during cancellation registration
	s.mu.RLock()
	tool, exists := s.tools[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	// Build raw request data
	rawRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}
	if ctx.Request != nil && ctx.Request.ID != nil {
		rawRequest["id"] = ctx.Request.ID
	}

	// Execute the tool handler with cancellation awareness
	resultCh := make(chan struct {
		result interface{}
		err    error
	}, 1)

	go func() {
		// The tool.Handler is already a wrapped function that handles validation and conversion
		// Call it directly with the context and original args map
		wrappedHandler, ok := tool.Handler.(func(*Context, interface{}) (interface{}, error))
		if !ok {
			resultCh <- struct {
				result interface{}
				err    error
			}{nil, fmt.Errorf("invalid handler type for tool %s", name)}
			return
		}

		// Call the wrapped handler with the original args
		result, err := wrappedHandler(ctx, args)

		// Check if cancelled after execution but before sending result
		select {
		case <-ctx.RegisterForCancellation():
			// Execution completed but was cancelled - don't send result
			return
		default:
			// Not cancelled, send result
			resultCh <- struct {
				result interface{}
				err    error
			}{result, err}
		}
	}()

	// Wait for either result or cancellation
	var finalResult interface{}
	var finalErr error

	select {
	case <-ctx.RegisterForCancellation():
		// Request was cancelled during execution
		finalErr = fmt.Errorf("tool execution cancelled: %s", name)
	case res := <-resultCh:
		// Execution completed
		finalResult = res.result
		finalErr = res.err
	}

	// Build raw response data
	var rawResponse map[string]interface{}
	if finalErr != nil {
		rawResponse = map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      ctx.Request.ID,
			"error": map[string]interface{}{
				"code":    -32000,
				"message": finalErr.Error(),
			},
		}
	} else {
		rawResponse = map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      ctx.Request.ID,
			"result":  finalResult,
		}
	}

	// Publish tool execution event with actual request/response objects
	go func() {
		requestJSON, _ := json.Marshal(rawRequest)
		responseJSON, _ := json.Marshal(rawResponse)
		events.Publish[events.ToolExecutedEvent](s.events, events.TopicToolExecuted, events.ToolExecutedEvent{
			Method:       "tools/call",
			RequestJSON:  string(requestJSON),
			ResponseJSON: string(responseJSON),
		})
	}()

	// Return the final result
	if finalErr != nil {
		if finalErr.Error() == fmt.Sprintf("tool execution cancelled: %s", name) {
			return nil, finalErr
		}
		return nil, fmt.Errorf("tool execution failed: %w", finalErr)
	}
	return finalResult, nil
}

// ProcessToolCall processes a tool call message and returns the result.
// It executes the requested tool with the provided arguments and formats the response
// according to the MCP protocol specification.
func (s *serverImpl) ProcessToolCall(ctx *Context) (interface{}, error) {
	if ctx.Request == nil || ctx.Request.ToolName == "" {
		return nil, errors.New("invalid tool call request")
	}

	// Execute the requested tool
	result, err := s.executeTool(ctx, ctx.Request.ToolName, ctx.Request.ToolArgs)
	if err != nil {
		// For tool-specific errors, we still return a valid result but with isError=true
		if strings.Contains(err.Error(), "tool execution failed:") {
			return map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": err.Error(),
					},
				},
				"isError": true,
			}, nil
		}
		// For other errors (like tool not found), return a protocol error
		return nil, err
	}

	// Format the result according to the specification
	formattedResult := map[string]interface{}{
		"content": []map[string]interface{}{},
		"isError": false,
	}

	// Add appropriate content based on result type
	switch v := result.(type) {
	case string:
		// Simple text result
		formattedResult["content"] = []map[string]interface{}{
			{
				"type": "text",
				"text": v,
			},
		}
	case map[string]interface{}:
		// If result is already in the expected format with content field, use it directly
		if content, ok := v["content"]; ok {
			formattedResult["content"] = content
			if isError, ok := v["isError"].(bool); ok {
				formattedResult["isError"] = isError
			}
		} else if imageUrl, ok := v["imageUrl"].(string); ok {
			// Handle image result
			formattedResult["content"] = []map[string]interface{}{
				{
					"type":     "image",
					"imageUrl": imageUrl,
					"altText":  v["altText"], // Include alt text if provided
				},
			}
		} else if url, ok := v["url"].(string); ok {
			// Handle link result
			formattedResult["content"] = []map[string]interface{}{
				{
					"type":  "link",
					"url":   url,
					"title": v["title"], // Include title if provided
				},
			}
		} else if mimeType, ok := v["mimeType"].(string); ok && v["data"] != nil {
			// Handle binary/file data
			formattedResult["content"] = []map[string]interface{}{
				{
					"type":     "file",
					"mimeType": mimeType,
					"data":     v["data"],
					"filename": v["filename"], // Include filename if provided
				},
			}
		} else {
			// Otherwise convert the map to JSON and use as text
			jsonData, _ := json.MarshalIndent(v, "", "  ")
			formattedResult["content"] = []map[string]interface{}{
				{
					"type": "text",
					"text": string(jsonData),
				},
			}
		}
	case []interface{}:
		// If it's an array of content items, try to use it directly
		contentItems := make([]map[string]interface{}, 0, len(v))

		// Process each item and add to content
		for _, item := range v {
			if contentMap, ok := item.(map[string]interface{}); ok {
				// Verify it has a type field
				if contentType, hasType := contentMap["type"].(string); hasType {
					// Validate based on content type
					switch contentType {
					case "text":
						if _, hasText := contentMap["text"]; !hasText {
							contentMap["text"] = "Missing text content"
						}
					case "image":
						if _, hasUrl := contentMap["imageUrl"]; !hasUrl {
							continue // Skip invalid image items
						}
					case "link":
						if _, hasUrl := contentMap["url"]; !hasUrl {
							continue // Skip invalid link items
						}
					case "file":
						if _, hasMime := contentMap["mimeType"]; !hasMime || contentMap["data"] == nil {
							continue // Skip invalid file items
						}
					default:
						// Unknown content type, skip
						continue
					}

					contentItems = append(contentItems, contentMap)
				}
			}
		}

		// If we found valid content items, use them
		if len(contentItems) > 0 {
			formattedResult["content"] = contentItems
		} else {
			// Fallback: Convert the array to JSON
			jsonData, _ := json.MarshalIndent(v, "", "  ")
			formattedResult["content"] = []map[string]interface{}{
				{
					"type": "text",
					"text": string(jsonData),
				},
			}
		}
	default:
		// For other types, convert to JSON
		jsonData, _ := json.MarshalIndent(v, "", "  ")
		formattedResult["content"] = []map[string]interface{}{
			{
				"type": "text",
				"text": string(jsonData),
			},
		}
	}

	return formattedResult, nil
}

// SendToolsListChangedNotification sends a notification to inform clients that the tool list has changed.
// This is called when tools are added, removed, or updated, allowing clients to refresh their available tools.
func (s *serverImpl) SendToolsListChangedNotification() error {
	// Create the notification message
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/tools/list_changed",
	}

	// Marshal the notification to JSON
	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Check if the server is initialized (minimize lock scope)
	s.mu.RLock()
	initialized := s.initialized
	transport := s.transport
	s.mu.RUnlock()

	// If the server is not initialized, queue the notification for later
	if !initialized {
		s.capabilityCache.QueueNotification(notificationBytes)
		s.logger.Debug("queued tools/list_changed notification for after initialization")
		return nil
	}

	// Send the notification through the configured transport (no mutex needed for this)
	if transport != nil {
		if err := transport.Send(notificationBytes); err != nil {
			s.logger.Error("failed to send notification", "error", err)
			return fmt.Errorf("failed to send notification: %w", err)
		}
	} else {
		s.logger.Warn("no transport configured, skipping notification")
	}

	s.logger.Debug("sent tools/list_changed notification")
	return nil
}
