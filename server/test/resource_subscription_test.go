package test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/localrivet/gomcp/events"
	"github.com/localrivet/gomcp/server"
)

// TestResourceSubscriptionSupported tests that server capabilities correctly indicate subscription support
func TestResourceSubscriptionSupported(t *testing.T) {
	s := server.NewServer("test-subscription-server")

	// Register a dummy resource so capabilities are reported
	s.Resource("/dummy", "Dummy resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "dummy", nil
	})

	// Create a capabilities request
	requestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {
				"roots": {},
				"sampling": {}
			},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`)

	// Process the request
	responseBytes, err := server.HandleMessage(s.GetServer(), requestJSON)
	if err != nil {
		t.Fatalf("Failed to process initialize request: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Check that the response has the correct structure
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result object in response, got: %T", response["result"])
	}

	// Check capabilities
	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected capabilities object in result, got: %T", result["capabilities"])
	}

	// Check resources capability
	resources, ok := capabilities["resources"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected resources capability object, got: %T", capabilities["resources"])
	}

	// Check subscribe support
	subscribe, ok := resources["subscribe"].(bool)
	if !ok || !subscribe {
		t.Errorf("Expected resources.subscribe to be true, got: %v", resources["subscribe"])
	}

	// Check listChanged support
	listChanged, ok := resources["listChanged"].(bool)
	if !ok || !listChanged {
		t.Errorf("Expected resources.listChanged to be true, got: %v", resources["listChanged"])
	}
}

// TestResourceSubscription tests subscribing to a resource
func TestResourceSubscription(t *testing.T) {
	s := server.NewServer("test-subscription-server")

	// Register a dummy resource so capabilities are reported
	s.Resource("/dummy", "Dummy resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "dummy", nil
	})

	// First initialize the session
	initRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {
				"roots": {},
				"sampling": {}
			},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`)

	_, err := server.HandleMessage(s.GetServer(), initRequestJSON)
	if err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Create a subscribe request
	subscribeRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "resources/subscribe",
		"params": {
			"uri": "/test/resource"
		}
	}`)

	// Process the subscribe request
	responseBytes, err := server.HandleMessage(s.GetServer(), subscribeRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process resources/subscribe request: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Check for successful response
	if response["error"] != nil {
		t.Errorf("Expected successful subscription, got error: %v", response["error"])
	}

	// The result should be an empty object for successful subscription
	result, ok := response["result"]
	if !ok {
		t.Error("Expected result field in subscription response")
	}

	// Should be an empty object
	if resultMap, ok := result.(map[string]interface{}); !ok || len(resultMap) != 0 {
		t.Errorf("Expected empty result object, got: %v", result)
	}
}

// TestResourceUnsubscribe tests unsubscribing from a resource
func TestResourceUnsubscribe(t *testing.T) {
	s := server.NewServer("test-unsubscribe-server")

	// Register a dummy resource so capabilities are reported
	s.Resource("/dummy", "Dummy resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "dummy", nil
	})

	// First initialize the session
	initRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {
				"roots": {},
				"sampling": {}
			},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`)

	_, err := server.HandleMessage(s.GetServer(), initRequestJSON)
	if err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Subscribe first
	subscribeRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "resources/subscribe",
		"params": {
			"uri": "/test/resource"
		}
	}`)

	_, err = server.HandleMessage(s.GetServer(), subscribeRequestJSON)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Now unsubscribe
	unsubscribeRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 3,
		"method": "resources/unsubscribe",
		"params": {
			"uri": "/test/resource"
		}
	}`)

	// Process the unsubscribe request
	responseBytes, err := server.HandleMessage(s.GetServer(), unsubscribeRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process resources/unsubscribe request: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Check for successful response
	if response["error"] != nil {
		t.Errorf("Expected successful unsubscription, got error: %v", response["error"])
	}

	// The result should be an empty object for successful unsubscription
	result, ok := response["result"]
	if !ok {
		t.Error("Expected result field in unsubscription response")
	}

	// Should be an empty object
	if resultMap, ok := result.(map[string]interface{}); !ok || len(resultMap) != 0 {
		t.Errorf("Expected empty result object, got: %v", result)
	}
}

// TestResourceSubscriptionEvents tests that resource change events are published when subscribed
func TestResourceSubscriptionEvents(t *testing.T) {
	s := server.NewServer("test-events-server")

	// Set up event listener
	var receivedEvents []events.ResourceChangedEvent
	var eventMutex sync.Mutex

	// Subscribe to resource change events using the correct API
	events.Subscribe[events.ResourceChangedEvent](s.Events(), events.TopicResourceChanged,
		func(ctx context.Context, event events.ResourceChangedEvent) error {
			eventMutex.Lock()
			defer eventMutex.Unlock()
			receivedEvents = append(receivedEvents, event)
			return nil
		})

	// First initialize the session
	initRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {
				"roots": {},
				"sampling": {}
			},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`)

	_, err := server.HandleMessage(s.GetServer(), initRequestJSON)
	if err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Subscribe to a resource
	subscribeRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "resources/subscribe",
		"params": {
			"uri": "/test/resource"
		}
	}`)

	_, err = server.HandleMessage(s.GetServer(), subscribeRequestJSON)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give it a moment to process
	time.Sleep(10 * time.Millisecond)

	// Trigger a resource change by publishing directly to the event system
	resourceChangeEvent := events.ResourceChangedEvent{
		URI:       "/test/resource",
		Action:    "modified",
		ChangedAt: time.Now(),
		SessionID: "test-session",
	}
	events.Publish[events.ResourceChangedEvent](s.Events(), events.TopicResourceChanged, resourceChangeEvent)

	// Give the event system time to process
	time.Sleep(100 * time.Millisecond)

	// Check that we received the event
	eventMutex.Lock()
	defer eventMutex.Unlock()

	if len(receivedEvents) != 1 {
		t.Errorf("Expected 1 resource change event, got %d", len(receivedEvents))
		return
	}

	event := receivedEvents[0]
	if event.URI != "/test/resource" {
		t.Errorf("Expected event URI '/test/resource', got '%s'", event.URI)
	}

	if event.ChangedAt.IsZero() {
		t.Error("Expected event ChangedAt to be set")
	}

	// Test that event has proper fields
	if event.SessionID == "" {
		t.Error("Expected event to have a session ID")
	}
}

// TestMultipleResourceSubscriptions tests subscribing to multiple resources
func TestMultipleResourceSubscriptions(t *testing.T) {
	s := server.NewServer("test-multiple-subscriptions")

	// First initialize the session
	initRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {
				"roots": {},
				"sampling": {}
			},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`)

	_, err := server.HandleMessage(s.GetServer(), initRequestJSON)
	if err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Subscribe to multiple resources
	resources := []string{"/resource1", "/resource2", "/resource3"}

	for i, uri := range resources {
		subscribeRequestJSON := []byte(`{
			"jsonrpc": "2.0",
			"id": ` + fmt.Sprintf("%d", i+2) + `,
			"method": "resources/subscribe",
			"params": {
				"uri": "` + uri + `"
			}
		}`)

		responseBytes, err := server.HandleMessage(s.GetServer(), subscribeRequestJSON)
		if err != nil {
			t.Fatalf("Failed to subscribe to %s: %v", uri, err)
		}

		// Parse the response to ensure success
		var response map[string]interface{}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("Failed to parse JSON response for %s: %v", uri, err)
		}

		if response["error"] != nil {
			t.Errorf("Failed to subscribe to %s: %v", uri, response["error"])
		}
	}

	// Now unsubscribe from one
	unsubscribeRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 5,
		"method": "resources/unsubscribe",
		"params": {
			"uri": "/resource2"
		}
	}`)

	responseBytes, err := server.HandleMessage(s.GetServer(), unsubscribeRequestJSON)
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	// Parse the response to ensure success
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse unsubscribe response: %v", err)
	}

	if response["error"] != nil {
		t.Errorf("Failed to unsubscribe: %v", response["error"])
	}
}

// TestInvalidResourceSubscription tests error handling for invalid subscription requests
func TestInvalidResourceSubscription(t *testing.T) {
	s := server.NewServer("test-invalid-subscription")

	// First initialize the session
	initRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {
				"roots": {},
				"sampling": {}
			},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`)

	_, err := server.HandleMessage(s.GetServer(), initRequestJSON)
	if err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Test subscription without URI parameter
	invalidRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "resources/subscribe",
		"params": {}
	}`)

	responseBytes, err := server.HandleMessage(s.GetServer(), invalidRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process invalid subscription request: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Should get an error response
	if response["error"] == nil {
		t.Error("Expected error for subscription without URI, got success")
	}
}

// TestResourceSubscriptionWithoutSession tests that subscription fails without proper session initialization
func TestResourceSubscriptionWithoutSession(t *testing.T) {
	s := server.NewServer("test-no-session")

	// Register a dummy resource so capabilities are reported
	s.Resource("/dummy", "Dummy resource", func(ctx *server.Context, args interface{}) (interface{}, error) {
		return "dummy", nil
	})

	// Try to subscribe without initializing session first
	// This should work because NewContext sets up a defaultSession automatically
	subscribeRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "resources/subscribe",
		"params": {
			"uri": "/test/resource"
		}
	}`)

	responseBytes, err := server.HandleMessage(s.GetServer(), subscribeRequestJSON)
	if err != nil {
		t.Fatalf("Failed to process subscription request: %v", err)
	}

	// Parse the response
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Should succeed because we now automatically set up a default session
	if response["error"] != nil {
		t.Errorf("Expected successful subscription even without explicit initialization, got error: %v", response["error"])
	}

	// The result should be an empty object for successful subscription
	result, ok := response["result"]
	if !ok {
		t.Error("Expected result field in subscription response")
	}

	// Should be an empty object
	if resultMap, ok := result.(map[string]interface{}); !ok || len(resultMap) != 0 {
		t.Errorf("Expected empty result object, got: %v", result)
	}
}

// TestResourceNotificationSent tests that notifications are properly formatted and sent
func TestResourceNotificationSent(t *testing.T) {
	// Create a mock transport to capture notifications
	mockTransport := NewMockTransport()
	var sentNotifications [][]byte
	var notificationMutex sync.Mutex

	// Set up the transport to capture sent messages
	mockTransport.SetSendFunc(func(data []byte) error {
		notificationMutex.Lock()
		defer notificationMutex.Unlock()
		sentNotifications = append(sentNotifications, data)
		return nil
	})

	s := server.NewServer("test-notification-server")
	// Set the transport manually using the internal method
	s.GetServer().SetTransport(mockTransport)

	// Initialize session
	initRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {
				"roots": {},
				"sampling": {}
			},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`)

	_, err := server.HandleMessage(s.GetServer(), initRequestJSON)
	if err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Subscribe to a resource
	subscribeRequestJSON := []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "resources/subscribe",
		"params": {
			"uri": "/test/resource"
		}
	}`)

	_, err = server.HandleMessage(s.GetServer(), subscribeRequestJSON)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Send the initialized notification to complete the MCP handshake
	initializedNotificationJSON := []byte(`{
		"jsonrpc": "2.0",
		"method": "notifications/initialized"
	}`)

	_, err = server.HandleMessage(s.GetServer(), initializedNotificationJSON)
	if err != nil {
		t.Fatalf("Failed to send initialized notification: %v", err)
	}

	// Clear any initialization messages
	notificationMutex.Lock()
	sentNotifications = [][]byte{}
	notificationMutex.Unlock()

	// Trigger a resource change by publishing directly to the event system
	resourceChangeEvent := events.ResourceChangedEvent{
		URI:       "/test/resource",
		Action:    "modified",
		ChangedAt: time.Now(),
		SessionID: "test-session",
	}
	events.Publish[events.ResourceChangedEvent](s.Events(), events.TopicResourceChanged, resourceChangeEvent)

	// Give the notification system time to process
	time.Sleep(100 * time.Millisecond)

	// Check that a notification was sent
	notificationMutex.Lock()
	defer notificationMutex.Unlock()

	if len(sentNotifications) == 0 {
		t.Error("Expected at least one notification to be sent")
		return
	}

	// Parse the notification
	var notification map[string]interface{}
	if err := json.Unmarshal(sentNotifications[0], &notification); err != nil {
		t.Fatalf("Failed to parse notification JSON: %v", err)
	}

	// Verify notification structure
	method, ok := notification["method"].(string)
	if !ok || method != "notifications/resources/list_changed" {
		t.Errorf("Expected method 'notifications/resources/list_changed', got '%v'", method)
	}

	// Should have no ID (notifications don't have IDs)
	if _, hasID := notification["id"]; hasID {
		t.Error("Notification should not have an ID field")
	}

	// Check jsonrpc version
	if jsonrpc, ok := notification["jsonrpc"].(string); !ok || jsonrpc != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got '%v'", jsonrpc)
	}
}
