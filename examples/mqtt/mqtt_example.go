// Package main provides an example of using the MQTT transport for gomcp with an embedded MQTT broker
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/mqtt"

	mqttserver "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Use standard MQTT port
	const mqttPort = 1883
	brokerURL := fmt.Sprintf("tcp://localhost:%d", mqttPort)

	// Start embedded MQTT broker on standard port
	broker := startEmbeddedBrokerOnPort(mqttPort)

	fmt.Printf("🚀 Embedded MQTT broker started on %s\n", brokerURL)

	// Wait for the broker to be ready
	time.Sleep(1 * time.Second)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("📡 Starting MCP-over-MQTT Example")
	fmt.Println(strings.Repeat("=", 50))

	// Start the MCP server
	srv := startMCPServer(brokerURL)

	// Wait for the server to initialize and be ready
	time.Sleep(3 * time.Second)

	// Start the MCP client
	clientDone := make(chan struct{})
	go func() {
		runMCPClient(brokerURL)
		close(clientDone)
	}()

	// Wait for termination signal
	fmt.Println("\n📡 MQTT MCP example running... Press Ctrl+C to exit")
	<-signals
	fmt.Println("\n🛑 Shutdown signal received, exiting...")

	// Clean shutdown with timeout
	shutdownComplete := make(chan struct{})
	go func() {
		defer close(shutdownComplete)

		// Shutdown server with timeout
		if srv != nil {
			fmt.Println("🔄 Shutting down MCP server...")
			srv.Shutdown()
		}

		// Close broker
		if broker != nil {
			fmt.Println("🔄 Shutting down MQTT broker...")
			broker.Close()
		}
	}()

	// Wait for shutdown to complete or timeout
	select {
	case <-shutdownComplete:
		fmt.Println("✅ Clean shutdown completed")
	case <-time.After(2 * time.Second):
		fmt.Println("⏰ Shutdown timeout - forcing exit")
	}

	// Wait for client to finish (brief)
	select {
	case <-clientDone:
	case <-time.After(100 * time.Millisecond):
	}
}

// startEmbeddedBrokerOnPort starts a mochi-mqtt broker on a specific port
func startEmbeddedBrokerOnPort(port int) *mqttserver.Server {
	// Create the MQTT broker
	broker := mqttserver.New(&mqttserver.Options{
		InlineClient: true, // Enable inline client for direct publishing
	})

	// Allow all connections (no auth for example)
	_ = broker.AddHook(new(auth.AllowHook), nil)

	// Add TCP listener
	tcp := listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: fmt.Sprintf(":%d", port),
	})
	err := broker.AddListener(tcp)
	if err != nil {
		log.Fatalf("Failed to add TCP listener: %v", err)
	}

	// Start the broker in a goroutine
	go func() {
		err := broker.Serve()
		if err != nil {
			log.Printf("MQTT broker error: %v", err)
		}
	}()

	return broker
}

func startMCPServer(brokerURL string) server.Server {
	// Create a new MCP server
	srv := server.NewServer("mqtt-example-server")

	// Configure the server with MQTT transport (using default topic prefix)
	srv.AsMQTT(brokerURL,
		mqtt.WithClientID("mcp-example-server"),
		mqtt.WithQoS(1),
	)

	// Register a simple echo tool
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (map[string]interface{}, error) {
		fmt.Printf("🔧 Server received: %s\n", args.Message)
		return map[string]interface{}{
			"message": args.Message,
		}, nil
	})

	// Register a calculator tool
	srv.Tool("calculate", "Perform basic arithmetic", func(ctx *server.Context, args struct {
		Operation string  `json:"operation"` // "add", "subtract", "multiply", "divide"
		A         float64 `json:"a"`
		B         float64 `json:"b"`
	}) (map[string]interface{}, error) {
		var result float64
		switch args.Operation {
		case "add":
			result = args.A + args.B
		case "subtract":
			result = args.A - args.B
		case "multiply":
			result = args.A * args.B
		case "divide":
			if args.B == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			result = args.A / args.B
		default:
			return nil, fmt.Errorf("unknown operation: %s", args.Operation)
		}

		fmt.Printf("🧮 Calculator: %.2f %s %.2f = %.2f\n", args.A, args.Operation, args.B, result)
		return map[string]interface{}{
			"result": result,
		}, nil
	})

	// Start the server in a goroutine
	go func() {
		fmt.Printf("🔥 Starting MCP server on MQTT broker %s\n", brokerURL)
		if err := srv.Run(); err != nil {
			log.Fatalf("MCP Server error: %v", err)
		}
	}()

	return srv
}

func runMCPClient(brokerURL string) {
	fmt.Println("💬 Starting MCP client...")

	// Create a new MCP client with MQTT transport (using default topic prefix)
	c, err := client.NewClient("mqtt-example-client",
		client.WithMQTT(brokerURL,
			client.WithMQTTClientID("mcp-example-client"),
			client.WithMQTTQoS(1),
		),
		client.WithConnectionTimeout(15*time.Second),
		client.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create MCP client: %v", err)
	}
	defer c.Close()

	fmt.Printf("✅ MCP client connected to %s\n", brokerURL)

	// List available tools first
	fmt.Println("\n📋 Listing available tools...")
	tools, err := c.ListTools()
	if err != nil {
		log.Printf("❌ Failed to list tools: %v", err)
		return
	}
	fmt.Printf("✅ Available tools (%d):\n", len(tools))
	for _, tool := range tools {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}

	// Test the echo tool with a simple call
	fmt.Println("\n📞 Testing echo tool...")
	echoResult, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello MQTT MCP! 🎉",
	})
	if err != nil {
		log.Printf("❌ Echo call failed: %v", err)
		return
	}
	fmt.Printf("✅ Echo result: %v\n", echoResult)

	fmt.Println("\n🎯 MQTT MCP basic communication test completed!")
}
