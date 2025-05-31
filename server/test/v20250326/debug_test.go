package v20250326

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/localrivet/gomcp/util/schema"
)

func TestDebugReflection(t *testing.T) {
	t.Run("struct by value", func(t *testing.T) {
		// Test the parameter type of our function with struct by value
		handler := func(ctx interface{}, args struct {
			Message string `json:"message"`
		}) (interface{}, error) {
			return nil, nil
		}

		handlerType := reflect.TypeOf(handler)
		paramType := handlerType.In(1)

		t.Logf("Handler type: %v", handlerType)
		t.Logf("Param type: %v", paramType)
		t.Logf("Param kind: %v", paramType.Kind())
		t.Logf("Is struct: %v", paramType.Kind() == reflect.Struct)

		// Test the schema conversion
		args := map[string]interface{}{
			"message": "Hello World",
		}

		testSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"message"},
		}

		result, err := schema.ValidateAndConvertArgs(testSchema, args, paramType)
		if err != nil {
			t.Fatalf("Error: %v", err)
		} else {
			t.Logf("Result type: %T", result)
			t.Logf("Result value: %+v", result)

			// Verify we got the correct type (struct by value)
			if reflect.TypeOf(result) != paramType {
				t.Errorf("Expected type %v, got %v", paramType, reflect.TypeOf(result))
			}
		}
	})

	t.Run("pointer to struct", func(t *testing.T) {
		// Test the parameter type with pointer to struct
		handler := func(ctx interface{}, args *struct {
			Message string `json:"message"`
			Count   int    `json:"count"`
		}) (interface{}, error) {
			return nil, nil
		}

		handlerType := reflect.TypeOf(handler)
		paramType := handlerType.In(1)

		t.Logf("Handler type: %v", handlerType)
		t.Logf("Param type: %v", paramType)
		t.Logf("Param kind: %v", paramType.Kind())
		t.Logf("Is pointer: %v", paramType.Kind() == reflect.Ptr)
		t.Logf("Elem kind: %v", paramType.Elem().Kind())

		// Test the schema conversion
		args := map[string]interface{}{
			"message": "Hello Pointer",
			"count":   42,
		}

		testSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type": "string",
				},
				"count": map[string]interface{}{
					"type": "integer",
				},
			},
			"required": []string{"message", "count"},
		}

		result, err := schema.ValidateAndConvertArgs(testSchema, args, paramType)
		if err != nil {
			t.Fatalf("Error: %v", err)
		} else {
			t.Logf("Result type: %T", result)
			t.Logf("Result value: %+v", result)

			// Verify we got the correct type (pointer to struct)
			if reflect.TypeOf(result) != paramType {
				t.Errorf("Expected type %v, got %v", paramType, reflect.TypeOf(result))
			}

			// Verify we can dereference the pointer
			if reflect.ValueOf(result).Kind() == reflect.Ptr {
				deref := reflect.ValueOf(result).Elem().Interface()
				t.Logf("Dereferenced value: %+v", deref)
			}
		}
	})

	t.Run("interface{} (nil case)", func(t *testing.T) {
		// Test the parameter type with interface{} for tools that don't need arguments
		handler := func(ctx interface{}, args interface{}) (interface{}, error) {
			return nil, nil
		}

		handlerType := reflect.TypeOf(handler)
		paramType := handlerType.In(1)

		t.Logf("Handler type: %v", handlerType)
		t.Logf("Param type: %v", paramType)
		t.Logf("Param kind: %v", paramType.Kind())
		t.Logf("Is interface: %v", paramType.Kind() == reflect.Interface)

		// Test with nil arguments (empty map)
		args := map[string]interface{}{}

		emptySchema := map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		}

		result, err := schema.ValidateAndConvertArgs(emptySchema, args, paramType)
		if err != nil {
			t.Fatalf("Error: %v", err)
		} else {
			t.Logf("Result type: %T", result)
			t.Logf("Result value: %+v", result)

			// For interface{}, the result should be the args map itself
			if resultMap, ok := result.(map[string]interface{}); ok {
				t.Logf("Result as map: %+v", resultMap)
			}
		}
	})

	t.Run("debug pointer to struct conversion", func(t *testing.T) {
		// Test the exact same signature as count-words tool
		handler := func(ctx interface{}, args *struct {
			Message string `json:"message"`
			Limit   int    `json:"limit"`
		}) (interface{}, error) {
			return nil, nil
		}

		handlerType := reflect.TypeOf(handler)
		paramType := handlerType.In(1)

		t.Logf("Handler type: %v", handlerType)
		t.Logf("Param type: %v", paramType)
		t.Logf("Param kind: %v", paramType.Kind())
		t.Logf("Is pointer: %v", paramType.Kind() == reflect.Ptr)
		t.Logf("Elem kind: %v", paramType.Elem().Kind())

		// Test the schema conversion with the exact same args as the test
		args := map[string]interface{}{
			"message": "This is a test message with several words",
			"limit":   10,
		}

		testSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type": "string",
				},
				"limit": map[string]interface{}{
					"type": "integer",
				},
			},
			"required": []string{"message", "limit"},
		}

		result, err := schema.ValidateAndConvertArgs(testSchema, args, paramType)
		if err != nil {
			t.Fatalf("Error: %v", err)
		} else {
			t.Logf("Result type: %T", result)
			t.Logf("Result value: %+v", result)

			// Verify we got the correct type (pointer to struct)
			if reflect.TypeOf(result) != paramType {
				t.Errorf("Expected type %v, got %v", paramType, reflect.TypeOf(result))
			}

			// Verify we can dereference the pointer
			if reflect.ValueOf(result).Kind() == reflect.Ptr {
				deref := reflect.ValueOf(result).Elem().Interface()
				t.Logf("Dereferenced value: %+v", deref)
			}
		}
	})

	t.Run("full tool registration simulation", func(t *testing.T) {
		// Test the exact same flow as the server's validateAndExtractToolHandler
		handler := func(ctx interface{}, args *struct {
			Message string `json:"message"`
			Limit   int    `json:"limit"`
		}) (interface{}, error) {
			return fmt.Sprintf("Word count: %d", len(strings.Fields(args.Message))), nil
		}

		handlerValue := reflect.ValueOf(handler)
		handlerType := handlerValue.Type()
		argsType := handlerType.In(1)

		t.Logf("Original handler type: %v", handlerType)
		t.Logf("Args parameter type: %v", argsType)
		t.Logf("Is pointer: %v", argsType.Kind() == reflect.Ptr)

		// Extract schema exactly like validateAndExtractToolHandler does
		paramType := argsType
		isPointer := false

		if paramType.Kind() == reflect.Ptr {
			paramType = paramType.Elem()
			isPointer = true
		}

		t.Logf("Struct type for schema: %v", paramType)
		t.Logf("Is pointer flag: %v", isPointer)

		// Create an instance for schema generation
		var structInstance interface{}
		if isPointer {
			structInstance = reflect.New(paramType).Interface()
		} else {
			structInstance = reflect.New(paramType).Elem().Interface()
		}

		t.Logf("Struct instance for schema: %T = %+v", structInstance, structInstance)

		// Generate schema from the struct
		generator := schema.NewGenerator()
		schemaMap, err := generator.GenerateSchema(structInstance)
		if err != nil {
			t.Fatalf("Failed to generate schema: %v", err)
		}

		t.Logf("Generated schema: %+v", schemaMap)

		// Now test conversion with the generated schema
		args := map[string]interface{}{
			"message": "This is a test message with several words",
			"limit":   10,
		}

		// Use the original argsType (which should be the pointer type)
		result, err := schema.ValidateAndConvertArgs(schemaMap, args, argsType)
		if err != nil {
			t.Fatalf("Conversion error: %v", err)
		}

		t.Logf("Final result type: %T", result)
		t.Logf("Final result value: %+v", result)

		// Verify the result is the correct pointer type
		if reflect.TypeOf(result) != argsType {
			t.Errorf("Expected type %v, got %v", argsType, reflect.TypeOf(result))
		}
	})
}
