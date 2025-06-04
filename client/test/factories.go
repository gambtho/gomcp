package test

import (
	"fmt"
	"time"
)

// ToolRequest generates a tool request with the given name and arguments
func ToolRequest(name string, args map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}
}

// ToolRequestWithID generates a tool request with a specific ID
func ToolRequestWithID(id interface{}, name string, args map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}
}

// ResourceRequest generates a resource request for the given path
func ResourceRequest(path string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": path,
		},
	}
}

// ResourceRequestWithID generates a resource request with a specific ID
func ResourceRequestWithID(id interface{}, path string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": path,
		},
	}
}

// ResourceRequestWithOptions generates a resource request with additional options
func ResourceRequestWithOptions(id interface{}, path string, options map[string]interface{}) map[string]interface{} {
	params := map[string]interface{}{
		"uri": path,
	}

	// Add additional options
	for k, v := range options {
		params[k] = v
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "resources/read",
		"params":  params,
	}
}

// PromptRequest generates a prompt request with the given name and variables
func PromptRequest(name string, variables map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "prompts/get",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": variables,
		},
	}
}

// PromptRequestWithID generates a prompt request with a specific ID
func PromptRequestWithID(id interface{}, name string, variables map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "prompts/get",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": variables,
		},
	}
}

// Note: According to MCP specification, there are NO client-to-server root management methods.
// Clients manage roots locally and only send notifications/roots/list_changed notifications.
// The server can request root lists via roots/list FROM the client TO the server.

// RootsListChangedNotification generates a notification for when roots change
func RootsListChangedNotification() map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/roots/list_changed",
		"params":  map[string]interface{}{},
	}
}

// InitializeRequest generates an initialize request with the given client info
func InitializeRequest(clientName, clientVersion string, supportedVersions []string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"clientInfo": map[string]interface{}{
				"name":    clientName,
				"version": clientVersion,
			},
			"versions": supportedVersions,
		},
	}
}

// ShutdownRequest generates a shutdown request
func ShutdownRequest(id interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "shutdown",
	}
}

// ErrorResponse generates an error response with the given code and message
func ErrorResponse(code int, message string) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      7,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
}

// ErrorResponseWithID generates an error response with a specific ID
func ErrorResponseWithID(id interface{}, code int, message string, data interface{}) map[string]interface{} {
	errorObj := map[string]interface{}{
		"code":    code,
		"message": message,
	}

	if data != nil {
		errorObj["data"] = data
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   errorObj,
	}
}

// SuccessResponse generates a success response with the given result
func SuccessResponse(id interface{}, result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
}

// EmptySuccessResponse generates a success response with an empty result
func EmptySuccessResponse(id interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]interface{}{},
	}
}

// NotificationRequest generates a notification with the given method and params
func NotificationRequest(method string, params interface{}) map[string]interface{} {
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}

	if params != nil {
		notification["params"] = params
	}

	return notification
}

// CreateResourceContent generates content for a resource based on the given type
func CreateResourceContent(contentType string, data interface{}) interface{} {
	switch contentType {
	case "text":
		return map[string]interface{}{
			"type": "text",
			"text": data,
		}
	case "image":
		imageData, ok := data.(map[string]interface{})
		if !ok {
			imageData = map[string]interface{}{
				"url":     "https://example.com/image.jpg",
				"altText": "Example image",
			}
		}
		return map[string]interface{}{
			"type":    "image",
			"url":     imageData["url"],
			"altText": imageData["altText"],
		}
	case "link":
		linkData, ok := data.(map[string]interface{})
		if !ok {
			linkData = map[string]interface{}{
				"url":   "https://example.com",
				"title": "Example link",
			}
		}
		return map[string]interface{}{
			"type":  "link",
			"url":   linkData["url"],
			"title": linkData["title"],
		}
	case "code":
		codeData, ok := data.(map[string]interface{})
		if !ok {
			codeData = map[string]interface{}{
				"code":     "console.log('Hello World');",
				"language": "javascript",
			}
		}
		return map[string]interface{}{
			"type":     "code",
			"code":     codeData["code"],
			"language": codeData["language"],
		}
	case "table":
		tableData, ok := data.(map[string]interface{})
		if !ok {
			tableData = map[string]interface{}{
				"headers": []string{"Column 1", "Column 2"},
				"rows":    [][]string{{"Row 1, Col 1", "Row 1, Col 2"}, {"Row 2, Col 1", "Row 2, Col 2"}},
			}
		}
		return map[string]interface{}{
			"type":    "table",
			"headers": tableData["headers"],
			"rows":    tableData["rows"],
		}
	default:
		return map[string]interface{}{
			"type": "text",
			"text": fmt.Sprintf("Unknown content type: %s", contentType),
		}
	}
}

// CreateResource creates a complete resource with the given URI, text, and content
func CreateResource(uri, text string, content []interface{}) map[string]interface{} {
	return map[string]interface{}{
		"uri":     uri,
		"text":    text,
		"content": content,
	}
}

// ServerInfo generates server information with the given name and version
func ServerInfo(name, version string) map[string]interface{} {
	if name == "" {
		name = "Test Server"
	}
	if version == "" {
		version = "1.0.0"
	}
	return map[string]interface{}{
		"name":    name,
		"version": version,
	}
}

// ClientInfo generates client information with the given name and version
func ClientInfo(name, version string) map[string]interface{} {
	if name == "" {
		name = "Test Client"
	}
	if version == "" {
		version = "1.0.0"
	}
	return map[string]interface{}{
		"name":    name,
		"version": version,
	}
}

// Capabilities generates capabilities for the given protocol version
func Capabilities(version string) map[string]interface{} {
	capabilities := map[string]interface{}{
		"roots": map[string]interface{}{
			"listChanged": true,
		},
	}

	// Add version-specific capabilities
	switch version {
	case "draft":
		capabilities["experimental"] = map[string]interface{}{
			"featureX": true,
		}
	case "2025-03-26":
		capabilities["enhancedResources"] = true
		capabilities["multipleRoots"] = true
	}

	return capabilities
}

// InitializeResponse generates an initialize response for the given version
func InitializeResponse(id interface{}, version string, serverName, serverVersion string, capabilities map[string]interface{}) map[string]interface{} {
	if capabilities == nil {
		capabilities = Capabilities(version)
	}

	serverInfoMap := ServerInfo(serverName, serverVersion)

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"protocolVersion": version,
			"serverInfo":      serverInfoMap,
			"capabilities":    capabilities,
		},
	}
}

// TimestampNow generates a current timestamp string in ISO 8601 format
func TimestampNow() string {
	return time.Now().Format(time.RFC3339)
}

// CreateRoot creates a root object for roots/list responses
func CreateRoot(path, name string, metadata map[string]interface{}) map[string]interface{} {
	root := map[string]interface{}{
		"uri":  path,
		"name": name,
	}

	for k, v := range metadata {
		root[k] = v
	}

	return root
}
