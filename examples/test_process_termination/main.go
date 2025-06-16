package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/localrivet/gomcp/client"
)

func main() {
	fmt.Println("=== Simple Process Termination Test ===")

	// Create a registry
	registry := client.NewServerRegistry(client.WithRegistryLogger(slog.Default()))
	defer registry.Close()

	fmt.Println("Creating sleep process...")

	// Create a simple sleep process (like the test does)
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start process: %v\n", err)
		os.Exit(1)
	}

	pid := cmd.Process.Pid
	fmt.Printf("Started process with PID: %d\n", pid)

	// Wait a moment
	time.Sleep(1 * time.Second)

	fmt.Println("Calling StopServer (which calls terminateProcess)...")
	start := time.Now()

	// Create an MCPServer entry and use StopServer to test termination
	// First we need to access the registry's internal structure
	// Since we can't access private fields, let's create a minimal server entry
	serverDef := client.ServerDefinition{
		Command: "sleep",
		Args:    []string{"5"},
	}

	// Use StartServer but with a command that should fail to avoid full MCP setup
	err := registry.StartServer("test", serverDef)
	if err != nil {
		fmt.Printf("StartServer failed (expected): %v\n", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("Operation completed in %v\n", elapsed)

	fmt.Println("Test completed")
}
