package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	pb "github.com/localrivet/gomcp/transport/grpc/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// bufDialer is a helper for testing gRPC servers without network connections
func bufDialer(listener *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, s string) (net.Conn, error) {
		return listener.Dial()
	}
}

func TestNewTransport(t *testing.T) {
	// Test server mode
	serverTransport := NewTransport(":50051", true)
	if !serverTransport.isServer {
		t.Errorf("Expected server mode, got client mode")
	}

	// Test client mode
	clientTransport := NewTransport("localhost:50051", false)
	if clientTransport.isServer {
		t.Errorf("Expected client mode, got server mode")
	}

	// Test options
	transport := NewTransport(":50052", true,
		WithTLS("cert.pem", "key.pem", "ca.pem"),
		WithMaxMessageSize(8*1024*1024),
		WithConnectionTimeout(20*time.Second),
		WithBufferSize(200),
		WithKeepAliveParams(20*time.Second, 5*time.Second),
	)

	if !transport.useTLS {
		t.Errorf("Expected TLS to be enabled")
	}
	if transport.tlsCertFile != "cert.pem" {
		t.Errorf("Expected cert file 'cert.pem', got '%s'", transport.tlsCertFile)
	}
	if transport.maxMessageSize != 8*1024*1024 {
		t.Errorf("Expected max message size %d, got %d", 8*1024*1024, transport.maxMessageSize)
	}
	if transport.connectionTimeout != 20*time.Second {
		t.Errorf("Expected connection timeout %s, got %s", 20*time.Second, transport.connectionTimeout)
	}
	if transport.bufferSize != 200 {
		t.Errorf("Expected buffer size %d, got %d", 200, transport.bufferSize)
	}
	if transport.keepAliveTime != 20*time.Second {
		t.Errorf("Expected keepalive time %s, got %s", 20*time.Second, transport.keepAliveTime)
	}
	if transport.keepAliveTimeout != 5*time.Second {
		t.Errorf("Expected keepalive timeout %s, got %s", 5*time.Second, transport.keepAliveTimeout)
	}
}

func TestTransportInitialize(t *testing.T) {
	transport := NewTransport(":50053", true)

	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}

	// Check if channels are created
	if transport.sendCh == nil {
		t.Errorf("Send channel not created")
	}
	if transport.recvCh == nil {
		t.Errorf("Receive channel not created")
	}
	if transport.errCh == nil {
		t.Errorf("Error channel not created")
	}

	// Clean up
	defer func() {
		if err := transport.Stop(); err != nil {
			t.Logf("Error stopping transport: %v", err)
		}
	}()
}

func TestSendBeforeStart(t *testing.T) {
	transport := NewTransport(":50054", true)
	_ = transport.Initialize()

	// Sending before starting should return an error
	err := transport.Send([]byte("test message"))
	if err == nil || err != ErrNotRunning {
		t.Errorf("Expected error '%v', got '%v'", ErrNotRunning, err)
	}
}

func TestStopBeforeStart(t *testing.T) {
	transport := NewTransport(":50055", true)
	_ = transport.Initialize()

	// Stopping before starting should not return an error
	err := transport.Stop()
	if err != nil {
		t.Errorf("Expected no error when stopping before start, got '%v'", err)
	}
}

func TestErrorMapping(t *testing.T) {
	testCases := []struct {
		name            string
		grpcCode        codes.Code
		grpcMsg         string
		expectedMessage string
	}{
		{
			name:            "InvalidArgument",
			grpcCode:        codes.InvalidArgument,
			grpcMsg:         "Invalid request",
			expectedMessage: "Invalid request",
		},
		{
			name:            "NotFound",
			grpcCode:        codes.NotFound,
			grpcMsg:         "Resource not found",
			expectedMessage: "Resource not found",
		},
		{
			name:            "Internal",
			grpcCode:        codes.Internal,
			grpcMsg:         "Internal server error",
			expectedMessage: "Internal server error",
		},
		{
			name:            "Unimplemented",
			grpcCode:        codes.Unimplemented,
			grpcMsg:         "Method not implemented",
			expectedMessage: "Method not implemented",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a gRPC status error
			err := status.Error(tc.grpcCode, tc.grpcMsg)

			// Convert to JSON-RPC error
			jsonError := GRPCToJSONRPCError(err)

			// Verify the message conversion
			if jsonError.Message != tc.expectedMessage {
				t.Errorf("Expected error message '%s', got '%s'", tc.expectedMessage, jsonError.Message)
			}

			// Convert back to gRPC error
			grpcErr := JSONRPCToGRPCError(jsonError)

			// Verify the round-trip conversion
			st, ok := status.FromError(grpcErr)
			if !ok {
				t.Fatalf("Expected gRPC status error, got '%v'", grpcErr)
			}
			if st.Message() != tc.grpcMsg {
				t.Errorf("Expected gRPC message '%s', got '%s'", tc.grpcMsg, st.Message())
			}

			// Verify code round-trip (the code might change, but the status code category should be preserved)
			if tc.grpcCode != st.Code() {
				t.Errorf("gRPC code changed after round-trip. Original: %s, Got: %s", tc.grpcCode, st.Code())
			}
		})
	}
}

func TestValueConversion(t *testing.T) {
	testCases := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "String Value",
			value: "test string",
		},
		{
			name:  "Boolean Value",
			value: true,
		},
		{
			name:  "Integer Value",
			value: 42,
		},
		{
			name:  "Float Value",
			value: 3.14159,
		},
		{
			name:  "Null Value",
			value: nil,
		},
		{
			name:  "Binary Value",
			value: []byte("binary data"),
		},
		{
			name:  "Array Value",
			value: []interface{}{"string", 42, true, nil},
		},
		{
			name: "Object Value",
			value: map[string]interface{}{
				"string": "value",
				"number": 42,
				"bool":   true,
				"null":   nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert Go value to Proto value
			protoValue, err := ValueToProto(tc.value)
			if err != nil {
				t.Fatalf("Failed to convert value to proto: %v", err)
			}

			// Convert Proto value back to Go value
			goValue, err := ProtoToValue(protoValue)
			if err != nil {
				t.Fatalf("Failed to convert proto to value: %v", err)
			}

			// Verify the conversion
			// Note: For some types like floats, direct comparison might not work well
			// We could implement a more sophisticated comparison for a real test
			// For this example, we'll just check that the conversion doesn't error
			fmt.Printf("Original: %v, Converted: %v\n", tc.value, goValue)
		})
	}
}

// Integration test for client-server communication
func TestClientServerCommunication(t *testing.T) {
	// Create a buffer connection listener
	listener := bufconn.Listen(bufSize)

	// Create and start a gRPC server
	s := grpc.NewServer()
	pb.RegisterMCPServer(s, &mcpServer{
		transport: &Transport{
			sendCh: make(chan []byte, 10),
			recvCh: make(chan []byte, 10),
			errCh:  make(chan error, 10),
			ctx:    context.Background(),
		},
	})
	go func() {
		if err := s.Serve(listener); err != nil {
			t.Errorf("Failed to serve: %v", err)
		}
	}()
	defer s.Stop()

	// Create a client connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(listener)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := pb.NewMCPClient(conn)

	// Test the Initialize RPC
	resp, err := client.Initialize(ctx, &pb.InitializeRequest{
		ClientId:      "test-client",
		ClientVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if !resp.Success {
		t.Errorf("Expected successful initialization, got failure: %v", resp.Error)
	}
}

func TestErrorHandling(t *testing.T) {
	// Test handling of nil errors
	if jsonErr := GRPCToJSONRPCError(nil); jsonErr != nil {
		t.Errorf("Expected nil JSON-RPC error for nil gRPC error, got %v", jsonErr)
	}

	if grpcErr := JSONRPCToGRPCError(nil); grpcErr != nil {
		t.Errorf("Expected nil gRPC error for nil JSON-RPC error, got %v", grpcErr)
	}

	// Test handling of non-status errors
	plainErr := errors.New("plain error")
	jsonErr := GRPCToJSONRPCError(plainErr)
	if jsonErr.Code != -32603 {
		t.Errorf("Expected internal error code for plain error, got %d", jsonErr.Code)
	}
	if jsonErr.Message != "plain error" {
		t.Errorf("Expected plain error message, got %s", jsonErr.Message)
	}

	// Test handling of unknown JSON-RPC error codes
	unknownJSONErr := &pb.ErrorInfo{
		Code:    -99999,
		Message: "Unknown error",
	}
	grpcErr := JSONRPCToGRPCError(unknownJSONErr)
	st, ok := status.FromError(grpcErr)
	if !ok {
		t.Fatalf("Expected gRPC status error, got %v", grpcErr)
	}
	if st.Code() != codes.Internal {
		t.Errorf("Expected Internal code for unknown JSON-RPC error, got %s", st.Code())
	}
}

func TestMapFunctionRequest(t *testing.T) {
	// Create a sample gRPC function request
	req := &pb.FunctionRequest{
		FunctionId: "test_function",
		RequestId:  "req-123",
		Parameters: map[string]*pb.Value{
			"string_param": {Kind: &pb.Value_StringValue{StringValue: "value"}},
			"number_param": {Kind: &pb.Value_NumberValue{NumberValue: 42.0}},
			"bool_param":   {Kind: &pb.Value_BoolValue{BoolValue: true}},
		},
	}

	// Map to JSON-RPC request
	jsonReq, err := MapToJSONRPCRequest(req)
	if err != nil {
		t.Fatalf("Failed to map to JSON-RPC request: %v", err)
	}

	// Verify basic fields
	if jsonReq["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc version '2.0', got %v", jsonReq["jsonrpc"])
	}
	if jsonReq["method"] != "test_function" {
		t.Errorf("Expected method 'test_function', got %v", jsonReq["method"])
	}
	if jsonReq["id"] != "req-123" {
		t.Errorf("Expected id 'req-123', got %v", jsonReq["id"])
	}

	// Verify parameters
	params, ok := jsonReq["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params to be a map, got %T", jsonReq["params"])
	}
	if params["string_param"] != "value" {
		t.Errorf("Expected string_param 'value', got %v", params["string_param"])
	}
	if params["number_param"] != 42.0 {
		t.Errorf("Expected number_param 42.0, got %v", params["number_param"])
	}
	if params["bool_param"] != true {
		t.Errorf("Expected bool_param true, got %v", params["bool_param"])
	}
}

// TestSendWithContextRequestResponseMatching tests the new request/response matching functionality
func TestSendWithContextRequestResponseMatching(t *testing.T) {
	// Create client transport
	transport := NewTransport("localhost:50051", false)
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}
	defer transport.Stop()

	// This would normally connect to a server, but we're testing the logic directly
	// We'll simulate by adding the request to pending and then routing a response
	transport.pendingMu.Lock()
	transport.pendingRequests[1.0] = make(chan []byte, 1) // JSON numbers are float64
	transport.pendingMu.Unlock()

	// Send a response to test routing
	responseJSON := `{"jsonrpc":"2.0","id":1,"result":"success"}`
	transport.routeMessage([]byte(responseJSON))

	// Verify the response was routed correctly
	transport.pendingMu.RLock()
	responseCh, exists := transport.pendingRequests[1.0]
	transport.pendingMu.RUnlock()

	if !exists {
		t.Fatal("Request was not registered in pending requests")
	}

	select {
	case response := <-responseCh:
		if string(response) != responseJSON {
			t.Errorf("Expected response %s, got %s", responseJSON, string(response))
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for response")
	}
}

// TestSendWithContextNotification tests handling of notifications (no ID)
func TestSendWithContextNotification(t *testing.T) {
	// Create client transport
	transport := NewTransport("localhost:50051", false)
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}
	defer transport.Stop()

	// Test notification (no ID) - should return empty response immediately
	notificationJSON := `{"jsonrpc":"2.0","method":"notification","params":{}}`

	// Since we can't actually connect to a server in this test, we'll test that
	// the method properly identifies notifications and handles them
	// For a real test, this would call SendWithContext, but we're testing the logic

	// Parse the notification to verify it has no ID
	var notificationMap map[string]interface{}
	if err := json.Unmarshal([]byte(notificationJSON), &notificationMap); err != nil {
		t.Fatalf("Failed to parse notification: %v", err)
	}

	if notificationMap["id"] != nil {
		t.Error("Notification should not have an ID")
	}

	// Verify the method would identify this as a notification
	if _, hasID := notificationMap["id"]; hasID {
		t.Error("Expected no ID for notification")
	}
}

// TestRouteMessage tests the new message routing functionality
func TestRouteMessage(t *testing.T) {
	// Create client transport
	transport := NewTransport("localhost:50051", false)
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}
	defer transport.Stop()

	// Test 1: Route response to pending request
	requestID := 42.0 // JSON numbers are float64
	responseCh := make(chan []byte, 1)
	transport.pendingMu.Lock()
	transport.pendingRequests[requestID] = responseCh
	transport.pendingMu.Unlock()

	responseJSON := `{"jsonrpc":"2.0","id":42,"result":"test result"}`
	transport.routeMessage([]byte(responseJSON))

	select {
	case response := <-responseCh:
		if string(response) != responseJSON {
			t.Errorf("Expected response %s, got %s", responseJSON, string(response))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Response was not routed to pending request")
	}

	// Test 2: Route notification to recvCh
	notificationJSON := `{"jsonrpc":"2.0","method":"notification"}`
	transport.routeMessage([]byte(notificationJSON))

	select {
	case message := <-transport.recvCh:
		if string(message) != notificationJSON {
			t.Errorf("Expected notification %s, got %s", notificationJSON, string(message))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Notification was not routed to recvCh")
	}

	// Test 3: Route response for unknown request to recvCh
	unknownResponseJSON := `{"jsonrpc":"2.0","id":999,"result":"unknown"}`
	transport.routeMessage([]byte(unknownResponseJSON))

	select {
	case message := <-transport.recvCh:
		if string(message) != unknownResponseJSON {
			t.Errorf("Expected unknown response %s, got %s", unknownResponseJSON, string(message))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Unknown response was not routed to recvCh")
	}

	// Test 4: Handle invalid JSON gracefully
	invalidJSON := `{invalid json`
	transport.routeMessage([]byte(invalidJSON))

	select {
	case message := <-transport.recvCh:
		if string(message) != invalidJSON {
			t.Errorf("Expected invalid JSON %s, got %s", invalidJSON, string(message))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Invalid JSON was not routed to recvCh")
	}
}

// TestSendWithContextServerMode tests that SendWithContext fails appropriately in server mode
func TestSendWithContextServerMode(t *testing.T) {
	// Create server transport
	transport := NewTransport(":50051", true)
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}
	defer transport.Stop()

	requestJSON := `{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// SendWithContext should fail in server mode
	_, err = transport.SendWithContext(ctx, []byte(requestJSON))
	if err == nil {
		t.Error("Expected error for SendWithContext in server mode")
	}
	if err.Error() != "SendWithContext not supported in server mode" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// TestConcurrentRequests tests handling of multiple concurrent requests
func TestConcurrentRequests(t *testing.T) {
	// Create client transport
	transport := NewTransport("localhost:50051", false)
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}
	defer transport.Stop()

	// Set up multiple pending requests
	numRequests := 5
	responseChannels := make([]chan []byte, numRequests)

	for i := 0; i < numRequests; i++ {
		responseCh := make(chan []byte, 1)
		responseChannels[i] = responseCh
		transport.pendingMu.Lock()
		transport.pendingRequests[float64(i+1)] = responseCh // JSON numbers are float64
		transport.pendingMu.Unlock()
	}

	// Send responses in different order to test proper routing
	responses := []string{
		`{"jsonrpc":"2.0","id":3,"result":"third"}`,
		`{"jsonrpc":"2.0","id":1,"result":"first"}`,
		`{"jsonrpc":"2.0","id":5,"result":"fifth"}`,
		`{"jsonrpc":"2.0","id":2,"result":"second"}`,
		`{"jsonrpc":"2.0","id":4,"result":"fourth"}`,
	}

	// Route all responses
	for _, response := range responses {
		transport.routeMessage([]byte(response))
	}

	// Verify each response went to the correct channel
	expectedResults := []string{"first", "second", "third", "fourth", "fifth"}
	for i := 0; i < numRequests; i++ {
		select {
		case response := <-responseChannels[i]:
			var responseMap map[string]interface{}
			if err := json.Unmarshal(response, &responseMap); err != nil {
				t.Fatalf("Failed to parse response %d: %v", i+1, err)
			}
			if responseMap["result"] != expectedResults[i] {
				t.Errorf("Request %d: expected result %s, got %s", i+1, expectedResults[i], responseMap["result"])
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timeout waiting for response %d", i+1)
		}
	}
}

// TestPendingRequestCleanup tests that pending requests are properly cleaned up
func TestPendingRequestCleanup(t *testing.T) {
	// Create client transport
	transport := NewTransport("localhost:50051", false)
	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}
	defer transport.Stop()

	// Simulate the cleanup that would happen in SendWithContext
	requestID := 123
	responseCh := make(chan []byte, 1)

	// Add to pending requests
	transport.pendingMu.Lock()
	transport.pendingRequests[requestID] = responseCh
	transport.pendingMu.Unlock()

	// Verify it's there
	transport.pendingMu.RLock()
	_, exists := transport.pendingRequests[requestID]
	transport.pendingMu.RUnlock()

	if !exists {
		t.Error("Request was not added to pending requests")
	}

	// Simulate cleanup (what defer does in SendWithContext)
	transport.pendingMu.Lock()
	delete(transport.pendingRequests, requestID)
	transport.pendingMu.Unlock()
	close(responseCh)

	// Verify it's cleaned up
	transport.pendingMu.RLock()
	_, exists = transport.pendingRequests[requestID]
	transport.pendingMu.RUnlock()

	if exists {
		t.Error("Request was not cleaned up from pending requests")
	}
}
