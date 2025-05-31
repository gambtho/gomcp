package test

import (
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
)

// TestMQTTServer tests MQTT transport integration for the server
// Note: this test is skipped by default as it requires a running MQTT broker
func TestMQTTServer(t *testing.T) {
	t.Skip("MQTT integration test requires a running MQTT broker - enable manually")

	// Create a test server
	srv := server.NewServer("test-server")

	// Note: In a real test with MQTT, we would configure it like:
	// srvImpl := srv.(*server.serverImpl)
	// srvImpl.AsMQTT("tcp://localhost:1883")

	// Add a simple echo tool for testing
	srv.Tool("echo", "Echo the input", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Extract the message parameter from the request
		if ctx.Request == nil || ctx.Request.ToolArgs == nil {
			return map[string]interface{}{"error": "no arguments provided"}, nil
		}

		// Get the message parameter
		message, ok := ctx.Request.ToolArgs["message"].(string)
		if !ok {
			return map[string]interface{}{"echo": "no message provided"}, nil
		}

		return map[string]interface{}{"message": message}, nil
	})

	// Create a done channel to signal shutdown
	done := make(chan struct{})

	// Start the server in a goroutine
	go func() {
		err := srv.Run()
		if err != nil {
			t.Logf("Server error: %v", err)
		}
		close(done)
	}()

	// Allow time for server to start
	time.Sleep(1 * time.Second)

	// Stop the server after the test
	t.Cleanup(func() {
		// Access and stop the underlying server implementation
		srvImpl := srv.GetServer()
		if srvImpl != nil && srvImpl.GetTransport() != nil {
			_ = srvImpl.GetTransport().Stop()
		}

		// Wait for the server to shut down
		select {
		case <-done:
			// Server shut down successfully
		case <-time.After(5 * time.Second):
			t.Log("Server shutdown timed out")
		}
	})

	// The actual client calls would be in TestMQTTClient in the client test package
}

// TestMQTTServerWithTLS tests MQTT transport with TLS
func TestMQTTServerWithTLS(t *testing.T) {
	t.Skip("MQTT TLS integration test requires a running secure MQTT broker - enable manually")

	// Create a test server
	srv := server.NewServer("test-server-tls")

	// Note: In a real test with MQTT transport, we would use:
	// srvImpl := srv.(*server.serverImpl)
	// srvImpl.AsMQTTWithTLS("ssl://localhost:8883", mqtt.TLSConfig{
	//    CertFile:   "../testdata/cert.pem",
	//    KeyFile:    "../testdata/key.pem",
	//    CAFile:     "../testdata/ca.pem",
	//    SkipVerify: false,
	// })

	// Add a simple echo tool for testing
	srv.Tool("echo", "Echo the input", func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Extract the message parameter from the request
		if ctx.Request == nil || ctx.Request.ToolArgs == nil {
			return map[string]interface{}{"error": "no arguments provided"}, nil
		}

		// Get the message parameter
		message, ok := ctx.Request.ToolArgs["message"].(string)
		if !ok {
			return map[string]interface{}{"echo": "no message provided"}, nil
		}

		return map[string]interface{}{"message": message}, nil
	})

	// Create a done channel to signal shutdown
	done := make(chan struct{})

	// Start the server in a goroutine
	go func() {
		err := srv.Run()
		if err != nil {
			t.Logf("Server error: %v", err)
		}
		close(done)
	}()

	// Allow time for server to start
	time.Sleep(1 * time.Second)

	// Stop the server after the test
	t.Cleanup(func() {
		// Access and stop the underlying server implementation
		srvImpl := srv.GetServer()
		if srvImpl != nil && srvImpl.GetTransport() != nil {
			_ = srvImpl.GetTransport().Stop()
		}

		// Wait for the server to shut down
		select {
		case <-done:
			// Server shut down successfully
		case <-time.After(5 * time.Second):
			t.Log("Server shutdown timed out")
		}
	})
}
