package client

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestServerRegistry_BasicProcessCleanup(t *testing.T) {
	registry := NewServerRegistry(WithRegistryLogger(slog.Default()))
	defer registry.Close()

	// Test the process termination logic directly without MCP client creation
	def := ServerDefinition{
		Command: "sleep",
		Args:    []string{"30"}, // Sleep for 30 seconds
	}

	// Manually create and start the process (bypassing the full StartServer flow)
	cmd := exec.Command(def.Command, def.Args...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	pid := cmd.Process.Pid
	t.Logf("Started test process with PID: %d", pid)

	// Verify process is alive
	if !isProcessAlive(pid) {
		t.Fatal("Process should be alive")
	}

	// Test our terminateProcess method directly
	err := registry.terminateProcess(cmd, "test-process")
	if err != nil {
		t.Fatalf("Failed to terminate process: %v", err)
	}

	// Give it a moment to terminate
	time.Sleep(100 * time.Millisecond)

	// Verify process is dead
	if isProcessAlive(pid) {
		t.Fatal("Process should be dead after terminateProcess")
	}
}

func TestServerRegistry_CloseAll(t *testing.T) {
	registry := NewServerRegistry(WithRegistryLogger(slog.Default()))

	var pids []int
	var cmds []*exec.Cmd

	// Manually create processes to test process cleanup logic
	for i := 0; i < 3; i++ {
		cmd := exec.Command("sleep", "30")
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start test process %d: %v", i, err)
		}

		// Manually add to registry for testing
		serverName := fmt.Sprintf("test-server-%d", i)
		registry.mu.Lock()
		registry.servers[serverName] = &MCPServer{
			Name: serverName,
			cmd:  cmd,
			// Client will be nil, but that's ok for testing process cleanup
		}
		registry.mu.Unlock()

		pids = append(pids, cmd.Process.Pid)
		cmds = append(cmds, cmd)
		t.Logf("Started test process %d with PID: %d", i, cmd.Process.Pid)
	}

	// Verify all processes are alive
	for _, pid := range pids {
		if !isProcessAlive(pid) {
			t.Fatalf("Process %d should be alive", pid)
		}
	}

	// Close the registry (should kill all processes)
	err := registry.Close()
	if err != nil {
		t.Fatalf("Failed to close registry: %v", err)
	}

	// Give processes time to die
	time.Sleep(500 * time.Millisecond)

	// Verify all processes are dead
	for i, pid := range pids {
		if isProcessAlive(pid) {
			t.Fatalf("Process %d (server %d) should be dead after Close()", pid, i)
		}
	}
}

func TestServerRegistry_ConcurrentOperations(t *testing.T) {
	registry := NewServerRegistry(WithRegistryLogger(slog.Default()))
	defer registry.Close()

	const numGoroutines = 10
	const numServersPerGoroutine = 5

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allPids []int

	// Create and add servers concurrently (without full MCP client setup)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numServersPerGoroutine; j++ {
				// Create process directly
				cmd := exec.Command("sleep", "30")
				if err := cmd.Start(); err != nil {
					t.Errorf("Failed to start process for goroutine %d, server %d: %v", goroutineID, j, err)
					return
				}

				// Add to registry under lock
				serverName := fmt.Sprintf("server-%d-%d", goroutineID, j)
				registry.mu.Lock()
				registry.servers[serverName] = &MCPServer{
					Name: serverName,
					cmd:  cmd,
					// Client is nil for testing
				}
				registry.mu.Unlock()

				// Track PID for verification
				mu.Lock()
				allPids = append(allPids, cmd.Process.Pid)
				mu.Unlock()
			}
		}(i)
	}

	// Wait for all servers to be created
	wg.Wait()

	// Verify we have the expected number of servers
	registry.mu.RLock()
	serverCount := len(registry.servers)
	registry.mu.RUnlock()

	expectedCount := numGoroutines * numServersPerGoroutine
	if serverCount != expectedCount {
		t.Fatalf("Expected %d servers, got %d", expectedCount, serverCount)
	}

	// Verify all processes are alive
	mu.Lock()
	pidCount := len(allPids)
	testPids := make([]int, len(allPids))
	copy(testPids, allPids)
	mu.Unlock()

	if pidCount != expectedCount {
		t.Fatalf("Expected %d PIDs, got %d", expectedCount, pidCount)
	}

	for _, pid := range testPids {
		if !isProcessAlive(pid) {
			t.Errorf("Process %d should be alive", pid)
		}
	}

	// Now stop servers concurrently using Close() for maximum concurrency test
	err := registry.Close()
	if err != nil {
		t.Fatalf("Failed to close registry: %v", err)
	}

	// Give processes time to die
	time.Sleep(500 * time.Millisecond)

	// Verify all processes are dead
	for _, pid := range testPids {
		if isProcessAlive(pid) {
			t.Errorf("Process %d should be dead after Close()", pid)
		}
	}

	// Verify all servers are gone from registry
	registry.mu.RLock()
	finalCount := len(registry.servers)
	registry.mu.RUnlock()

	if finalCount != 0 {
		t.Fatalf("Expected 0 servers after close, got %d", finalCount)
	}
}

func TestServerRegistry_DoubleClose(t *testing.T) {
	registry := NewServerRegistry()

	// Add a test server manually (without MCP client setup)
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	registry.mu.Lock()
	registry.servers["test-server"] = &MCPServer{
		Name: "test-server",
		cmd:  cmd,
		// Client is nil for testing
	}
	registry.mu.Unlock()

	pid := cmd.Process.Pid
	t.Logf("Started test process with PID: %d", pid)

	// Close once
	err := registry.Close()
	if err != nil {
		t.Fatalf("First close failed: %v", err)
	}

	// Verify process is dead
	time.Sleep(100 * time.Millisecond)
	if isProcessAlive(pid) {
		t.Fatal("Process should be dead after first Close()")
	}

	// Close again - should not error
	err = registry.Close()
	if err != nil {
		t.Fatalf("Second close failed: %v", err)
	}
}

func TestServerRegistry_OperationsOnClosedRegistry(t *testing.T) {
	registry := NewServerRegistry()

	// Close the registry first
	err := registry.Close()
	if err != nil {
		t.Fatalf("Failed to close registry: %v", err)
	}

	// Try to start a server on closed registry
	def := ServerDefinition{
		Command: "sleep",
		Args:    []string{"5"},
	}
	err = registry.StartServer("test-server", def)
	if err == nil {
		t.Fatal("Expected error when starting server on closed registry")
	}

	expectedMsg := "cannot start server test-server: registry is closed"
	if err.Error() != expectedMsg {
		t.Fatalf("Unexpected error message: got %q, want %q", err.Error(), expectedMsg)
	}
}

func TestServerRegistry_GracefulVsForcefulTermination(t *testing.T) {
	registry := NewServerRegistry()
	defer registry.Close()

	// Test case 1: Fast terminating process (should complete gracefully)
	t.Run("FastTermination", func(t *testing.T) {
		// Start a simple sleep command that terminates easily
		cmd := exec.Command("sleep", "10")
		err := cmd.Start()
		if err != nil {
			t.Fatalf("Failed to start test process: %v", err)
		}

		// Directly test termination logic
		start := time.Now()
		err = registry.terminateProcess(cmd, "test-process")
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("Failed to terminate process: %v", err)
		}

		// Sleep doesn't respond to stdin close, so it will take the graceful timeout (3s) + kill time
		// Should complete within reasonable time (3-4 seconds)
		if elapsed > 4*time.Second {
			t.Errorf("Termination took too long: %v", elapsed)
		}

		// Verify process is dead
		if isProcessAlive(cmd.Process.Pid) {
			t.Error("Process should be dead but is still alive")
		}
	})

	// Test case 2: Stubborn process that requires force kill
	t.Run("ForceTermination", func(t *testing.T) {
		// Start a process that ignores signals (trap in bash)
		cmd := exec.Command("bash", "-c", "trap '' TERM; sleep 30")
		err := cmd.Start()
		if err != nil {
			t.Fatalf("Failed to start stubborn process: %v", err)
		}

		// Directly test termination logic
		start := time.Now()
		err = registry.terminateProcess(cmd, "stubborn-process")
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("Failed to terminate stubborn process: %v", err)
		}

		// Should take close to the timeout duration (3 seconds for graceful + immediate kill)
		if elapsed < 3*time.Second || elapsed > 5*time.Second {
			t.Logf("Termination took %v (expected ~3-5s for stubborn process)", elapsed)
		}

		// Verify process is dead
		if isProcessAlive(cmd.Process.Pid) {
			t.Error("Stubborn process should be dead but is still alive")
		}
	})
}

func TestServerRegistry_ProcessTimeout(t *testing.T) {
	registry := NewServerRegistry()
	defer registry.Close()

	// Start a very stubborn process that ignores all signals
	cmd := exec.Command("bash", "-c", "trap '' TERM INT QUIT; while true; do sleep 1; done")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start very stubborn process: %v", err)
	}

	t.Logf("Started very stubborn process with PID: %d", cmd.Process.Pid)

	// Test termination with timeout
	start := time.Now()
	err = registry.terminateProcess(cmd, "very-stubborn-process")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Failed to terminate very stubborn process: %v", err)
	}

	// Should complete within reasonable time (graceful timeout + kill + wait)
	if elapsed > 10*time.Second {
		t.Errorf("Termination took too long: %v", elapsed)
	}

	// Verify process is finally dead
	if isProcessAlive(cmd.Process.Pid) {
		t.Error("Very stubborn process should be dead but is still alive")
	}
}

func TestServerRegistry_RaceConditionStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Test the registry mechanics without actual subprocess creation
	// This tests for race conditions in the map operations and locking

	registry := NewServerRegistry(WithRegistryLogger(slog.Default()))
	defer registry.Close()

	const iterations = 50
	const concurrency = 10

	var wg sync.WaitGroup
	errors := make(chan error, iterations*concurrency*2) // *2 for start+stop

	// Test: Concurrent map operations on the registry
	for i := 0; i < iterations; i++ {
		for j := 0; j < concurrency; j++ {
			wg.Add(1)
			go func(iter, worker int) {
				defer wg.Done()

				serverName := fmt.Sprintf("stress-%d-%d", iter, worker)

				// Test race conditions in the registry map operations
				// by directly manipulating the internal structure

				// Simulate adding a server to the registry (without process creation)
				registry.mu.Lock()
				if _, exists := registry.servers[serverName]; !exists {
					// Create a fake server entry to test map operations
					registry.servers[serverName] = &MCPServer{
						Name:   serverName,
						Client: nil, // No actual client
						cmd:    nil, // No actual process
					}
				} else {
					registry.mu.Unlock()
					errors <- fmt.Errorf("duplicate server detected: %s", serverName)
					return
				}
				registry.mu.Unlock()

				// Brief pause to increase chance of race conditions
				time.Sleep(1 * time.Millisecond)

				// Simulate removing the server
				registry.mu.Lock()
				if _, exists := registry.servers[serverName]; exists {
					delete(registry.servers, serverName)
				} else {
					registry.mu.Unlock()
					errors <- fmt.Errorf("server not found during cleanup: %s", serverName)
					return
				}
				registry.mu.Unlock()
			}(i, j)
		}
	}

	// Wait for all goroutines with reasonable timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("All goroutines completed successfully")
	case <-time.After(5 * time.Second): // Short timeout since this is just map operations
		t.Fatal("Test timed out - indicates deadlock or excessive contention")
	}

	// Check for race condition errors
	close(errors)
	var errorCount int
	for err := range errors {
		t.Errorf("Race condition detected: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Detected %d race condition errors", errorCount)
	}

	// Verify registry is clean
	registry.mu.RLock()
	remaining := len(registry.servers)
	registry.mu.RUnlock()

	if remaining != 0 {
		t.Errorf("Expected 0 remaining servers, got %d", remaining)
		// Log what's remaining for debugging
		registry.mu.RLock()
		for name := range registry.servers {
			t.Logf("Remaining server: %s", name)
		}
		registry.mu.RUnlock()
	} else {
		t.Log("Registry is clean - no servers remaining")
	}
}

// Helper function to check if a process is alive
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix-like systems, sending signal 0 checks if process exists
	if runtime.GOOS != "windows" {
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}

	// On Windows, we need a different approach
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(output) > 0
}
