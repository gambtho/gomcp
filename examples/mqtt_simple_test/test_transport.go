package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/transport/mqtt"
	mqtt2 "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	fmt.Println("🚀 Starting simple MQTT transport test...")

	// Start embedded MQTT broker
	mqttServer := mqtt2.New(&mqtt2.Options{
		InlineClient: true,
	})

	_ = mqttServer.AddHook(new(auth.AllowHook), nil)

	tcp := listeners.NewTCP(listeners.Config{ID: "tcp", Address: ":1883"})
	err := mqttServer.AddListener(tcp)
	if err != nil {
		log.Fatal("Failed to add MQTT listener:", err)
	}

	go func() {
		err := mqttServer.Serve()
		if err != nil {
			log.Println("MQTT server error:", err)
		}
	}()

	// Give broker time to start
	time.Sleep(500 * time.Millisecond)
	fmt.Println("📡 MQTT broker started")

	// Test the transport directly
	testTransportDirectly()

	// Cleanup
	mqttServer.Close()
	fmt.Println("✅ Test completed")
}

func testTransportDirectly() {
	fmt.Println("🔧 Testing MQTT transport directly...")

	// Create server transport
	serverTransport := mqtt.NewTransport("tcp://localhost:1883", true)
	serverTransport.SetMessageHandler(func(message []byte) ([]byte, error) {
		fmt.Printf("📨 Server received: %s\n", string(message))
		response := `{"jsonrpc":"2.0","id":1,"result":{"message":"Hello from server"}}`
		fmt.Printf("📤 Server responding: %s\n", response)
		return []byte(response), nil
	})

	// Initialize and start server
	if err := serverTransport.Initialize(); err != nil {
		log.Fatalf("❌ Server initialize error: %v", err)
	}
	if err := serverTransport.Start(); err != nil {
		log.Fatalf("❌ Server start error: %v", err)
	}
	fmt.Println("✅ Server transport started")

	// Ensure server is cleaned up when function exits
	defer func() {
		serverTransport.Stop()
		fmt.Println("🛑 Server transport stopped")
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Create and connect client
	clientTransport := client.NewMQTTTransport("tcp://localhost:1883")
	if err := clientTransport.Connect(); err != nil {
		log.Fatalf("❌ Client connect error: %v", err)
	}
	fmt.Println("✅ Client transport connected")

	// Ensure client is cleaned up
	defer func() {
		clientTransport.Disconnect()
		fmt.Println("🛑 Client transport disconnected")
	}()

	// Send a test message
	request := `{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`
	fmt.Printf("📤 Client sending: %s\n", request)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	response, err := clientTransport.SendWithContext(ctx, []byte(request))
	if err != nil {
		log.Fatalf("❌ Send error: %v", err)
	}

	fmt.Printf("📨 Client received: %s\n", string(response))
	fmt.Println("✅ Transport test successful!")
}
