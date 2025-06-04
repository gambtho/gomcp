package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/localrivet/gomcp/transport"
)

func TestNewTransport(t *testing.T) {
	addr := "localhost:8080"
	tr := NewTransport(addr)

	if tr.addr != addr {
		t.Errorf("Expected address %s, got %s", addr, tr.addr)
	}

	if tr.mcpEndpoint != DefaultMCPEndpoint {
		t.Errorf("Expected default MCP endpoint %s, got %s", DefaultMCPEndpoint, tr.mcpEndpoint)
	}

	if !tr.enableSessions {
		t.Error("Expected sessions to be enabled by default")
	}
}

func TestTransportOptions(t *testing.T) {
	addr := "localhost:8080"
	customEndpoint := "/custom"
	customPrefix := "/api/v1"
	customHeaders := map[string]string{"X-Custom": "value"}

	tr := NewTransport(addr,
		WithMCPEndpoint(customEndpoint),
		WithPathPrefix(customPrefix),
		WithHeaders(customHeaders),
	)

	if tr.mcpEndpoint != customEndpoint {
		t.Errorf("Expected MCP endpoint %s, got %s", customEndpoint, tr.mcpEndpoint)
	}

	if tr.pathPrefix != customPrefix {
		t.Errorf("Expected path prefix %s, got %s", customPrefix, tr.pathPrefix)
	}

	if tr.headers["X-Custom"] != "value" {
		t.Errorf("Expected custom header value 'value', got %s", tr.headers["X-Custom"])
	}
}

func TestGetFullMCPEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		endpoint string
		expected string
	}{
		{"no prefix", "", "/mcp", "/mcp"},
		{"with prefix", "/api", "/mcp", "/api/mcp"},
		{"prefix with slash", "/api/", "/mcp", "/api/mcp"},
		{"endpoint no slash", "/api", "mcp", "/api/mcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTransport("localhost:8080")
			tr.SetPathPrefix(tt.prefix)
			tr.SetMCPEndpoint(tt.endpoint)

			result := tr.GetFullMCPEndpoint()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSessionManagement(t *testing.T) {
	tr := NewTransport("localhost:8080")

	// Test session ID generation
	sessionID1 := tr.generateSessionID()
	sessionID2 := tr.generateSessionID()

	if sessionID1 == sessionID2 {
		t.Error("Session IDs should be unique")
	}

	if len(sessionID1) != 32 { // 16 bytes hex encoded = 32 chars
		t.Errorf("Expected session ID length 32, got %d", len(sessionID1))
	}
}

func TestServerMode(t *testing.T) {
	tr := NewTransport("127.0.0.1:0") // Use port 0 for automatic assignment
	tr.isClient = false

	// Set up a simple message handler
	tr.SetMessageHandler(func(message []byte) ([]byte, error) {
		var req map[string]interface{}
		if err := json.Unmarshal(message, &req); err != nil {
			return nil, err
		}

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  map[string]string{"echo": req["method"].(string)},
		}

		return json.Marshal(response)
	})

	// Initialize and start
	if err := tr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}

	if err := tr.Start(); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}
	defer tr.Stop()

	// Test POST request to MCP endpoint
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "test",
		"id":      1,
	}
	reqBytes, _ := json.Marshal(requestBody)

	req := httptest.NewRequest("POST", tr.GetFullMCPEndpoint(), bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	w := httptest.NewRecorder()
	tr.handleMCPRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	// Should have session ID in response
	sessionID := w.Header().Get("MCP-Session-ID")
	if sessionID == "" {
		t.Error("Expected session ID in response")
	}

	// Verify response body
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", response["jsonrpc"])
	}
}

func TestSessionPersistence(t *testing.T) {
	tr := NewTransport("127.0.0.1:0")
	tr.isClient = false

	tr.SetMessageHandler(func(message []byte) ([]byte, error) {
		return []byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`), nil
	})

	// First request - creates session
	reqBytes := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)
	req1 := httptest.NewRequest("POST", tr.GetFullMCPEndpoint(), bytes.NewReader(reqBytes))
	req1.Header.Set("Content-Type", "application/json")

	w1 := httptest.NewRecorder()
	tr.handleMCPRequest(w1, req1)

	sessionID := w1.Header().Get("MCP-Session-ID")
	if sessionID == "" {
		t.Fatal("Expected session ID in first response")
	}

	// Second request - uses existing session
	req2 := httptest.NewRequest("POST", tr.GetFullMCPEndpoint(), bytes.NewReader(reqBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("MCP-Session-ID", sessionID)

	w2 := httptest.NewRecorder()
	tr.handleMCPRequest(w2, req2)

	// Should return same session ID
	returnedSessionID := w2.Header().Get("MCP-Session-ID")
	if returnedSessionID != sessionID {
		t.Errorf("Expected same session ID %s, got %s", sessionID, returnedSessionID)
	}

	// Verify session exists in transport
	tr.sessionsMu.Lock()
	session, exists := tr.sessions[sessionID]
	tr.sessionsMu.Unlock()

	if !exists {
		t.Error("Session should exist in transport")
	}

	if session.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, session.ID)
	}
}

func TestSessionTermination(t *testing.T) {
	tr := NewTransport("127.0.0.1:0")
	tr.isClient = false

	// Create a session first
	sessionID := tr.generateSessionID()
	tr.sessionsMu.Lock()
	tr.sessions[sessionID] = &SessionInfo{
		ID:        sessionID,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
		ClientID:  "test",
	}
	tr.sessionsMu.Unlock()

	// Test DELETE request
	req := httptest.NewRequest("DELETE", tr.GetFullMCPEndpoint(), nil)
	req.Header.Set("MCP-Session-ID", sessionID)

	w := httptest.NewRecorder()
	tr.handleMCPRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify session was deleted
	tr.sessionsMu.Lock()
	_, exists := tr.sessions[sessionID]
	tr.sessionsMu.Unlock()

	if exists {
		t.Error("Session should have been deleted")
	}
}

func TestClientMode(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		// Echo the request
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("MCP-Session-ID", "test-session-123")
		w.Write(body)
	}))
	defer server.Close()

	// Create client transport
	tr := NewTransport(server.URL)
	tr.SetClientMode(true)

	if err := tr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}

	if err := tr.Start(); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}
	defer tr.Stop()

	// Test sending a message
	message := []byte(`{"jsonrpc":"2.0","method":"test","id":1}`)
	if err := tr.Send(message); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Check that session ID was stored
	sessionID := tr.sessionID.Load()
	if sessionID == nil || *sessionID != "test-session-123" {
		t.Errorf("Expected session ID 'test-session-123', got %v", sessionID)
	}
}

func TestInvalidContentType(t *testing.T) {
	tr := NewTransport("127.0.0.1:0")
	tr.isClient = false

	req := httptest.NewRequest("POST", tr.GetFullMCPEndpoint(), strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	tr.handleMCPRequest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Content-Type must be application/json") {
		t.Error("Expected Content-Type error message")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	tr := NewTransport("127.0.0.1:0")

	// Test unsupported method (PUT)
	req := httptest.NewRequest("PUT", tr.GetFullMCPEndpoint(), nil)
	w := httptest.NewRecorder()
	tr.handleMCPRequest(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestReceiveNotSupported(t *testing.T) {
	tr := NewTransport("localhost:8080")
	tr.SetClientMode(true)

	_, err := tr.Receive()
	if err == nil {
		t.Error("Expected error for Receive in HTTP transport")
	}

	if !strings.Contains(err.Error(), "receive not supported") {
		t.Errorf("Expected 'receive not supported' error, got %v", err)
	}
}

func TestSendServerMode(t *testing.T) {
	tr := NewTransport("localhost:8080")
	tr.isClient = false // Server mode

	// Initialize transport first
	if err := tr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}

	// Server mode should support Send (via SSE streams)
	err := tr.Send([]byte("test"))
	if err != nil {
		t.Errorf("Send should work in server mode, got error: %v", err)
	}
}

func TestCompatibilityMethods(t *testing.T) {
	tr := NewTransport("localhost:8080")

	// Test compatibility aliases
	if tr.GetFullAPIPath() != tr.GetFullMCPEndpoint() {
		t.Error("GetFullAPIPath should be alias for GetFullMCPEndpoint")
	}

	customPath := "/custom-api"
	tr.SetAPIPath(customPath)
	if tr.mcpEndpoint != customPath {
		t.Errorf("SetAPIPath should set MCP endpoint, expected %s, got %s", customPath, tr.mcpEndpoint)
	}
}

// TestTransportInterface verifies the transport implements the expected interface
func TestTransportInterface(t *testing.T) {
	tr := NewTransport("localhost:8080")

	// Verify it implements the transport.Transport interface
	var _ transport.Transport = tr

	// Test basic interface methods
	if err := tr.Initialize(); err != nil {
		t.Errorf("Initialize failed: %v", err)
	}

	// Test logger
	logger := tr.GetLogger()
	if logger == nil {
		t.Error("Expected logger to be set")
	}

	// Test protocol version
	version := "2025-03-26"
	tr.SetProtocolVersion(version)
	if tr.GetProtocolVersion() != version {
		t.Errorf("Expected protocol version %s, got %s", version, tr.GetProtocolVersion())
	}
}
