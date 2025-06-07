package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	// Create a registry with process tracking enabled and logging
	logger := client.NewLogger(client.WithLogLevel(slog.LevelDebug))
	registry := client.NewServerRegistry(
		client.WithRegistryLogger(logger),
		client.WithProcessTracking(), // Enable process tracking
	)

	fmt.Println("🚀 Testing process tracking with GoMCP ServerRegistry")
	fmt.Println("📊 This will demonstrate comprehensive process cleanup")

	// Create a test configuration with processes that spawn children
	config := client.ServerConfig{
		MCPServers: map[string]client.ServerDefinition{
			"test-process": {
				Command: "sh",
				Args:    []string{"-c", "sleep 2 & echo 'Started process'; wait"}, // Spawns a background sleep
			},
		},
	}

	// Start the server
	fmt.Println("\n🔧 Starting server with process tracking...")
	err := registry.ApplyConfig(config)
	if err != nil {
		fmt.Printf("❌ Failed to start server: %v\n", err)
		return
	}

	// Let it run for a moment
	time.Sleep(1 * time.Second)

	// Close the registry - this should clean up all processes
	fmt.Println("\n🧹 Closing registry (should cleanup all tracked processes)...")
	err = registry.Close()
	if err != nil {
		fmt.Printf("❌ Error during cleanup: %v\n", err)
	} else {
		fmt.Println("✅ Registry closed successfully with process tracking cleanup")
	}

	fmt.Println("\n🎯 Process tracking test completed!")
	fmt.Println("📝 Check the debug logs above to see process tracking in action")
}
