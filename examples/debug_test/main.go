package main

import (
	"fmt"
	"os/exec"
	"time"
)

func main() {
	fmt.Println("=== Debug Test ===")

	// Create a simple sleep process (exactly like the test does)
	cmd := exec.Command("sleep", "2")
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start process: %v\n", err)
		return
	}

	pid := cmd.Process.Pid
	fmt.Printf("Started process with PID: %d\n", pid)

	// Test our Wait logic
	done := make(chan error, 1)
	go func() {
		fmt.Println("Starting cmd.Wait()...")
		err := cmd.Wait()
		fmt.Printf("cmd.Wait() returned: %v\n", err)
		done <- err
	}()

	// Wait a moment, then kill
	time.Sleep(500 * time.Millisecond)
	fmt.Println("Calling Process.Kill()...")
	if err := cmd.Process.Kill(); err != nil {
		fmt.Printf("Kill failed: %v\n", err)
	}

	// Wait for the Wait() to complete
	fmt.Println("Waiting for cmd.Wait() to complete...")
	select {
	case err := <-done:
		fmt.Printf("Wait completed with: %v\n", err)
	case <-time.After(5 * time.Second):
		fmt.Println("cmd.Wait() timed out!")
	}
}
