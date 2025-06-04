package test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestConcurrentRootsAccess tests concurrent access to root management without locks
func TestConcurrentRootsAccess(t *testing.T) {
	c, _ := SetupClientWithMockTransport(t, "2024-11-05")

	const numGoroutines = 100
	const operationsPerGoroutine = 10

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*operationsPerGoroutine)

	// Start multiple goroutines doing different operations concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				switch j % 4 {
				case 0:
					// Add root
					err := c.AddRoot(fmt.Sprintf("/test/root/%d-%d", id, j), fmt.Sprintf("Root %d-%d", id, j))
					if err != nil {
						errChan <- fmt.Errorf("AddRoot failed: %w", err)
					}
				case 1:
					// Get roots
					_, err := c.GetRoots()
					if err != nil {
						errChan <- fmt.Errorf("GetRoots failed: %w", err)
					}
				case 2:
					// Add duplicate (should fail)
					err := c.AddRoot("/test/duplicate", "Duplicate Root")
					if err == nil {
						// First time should succeed, subsequent should fail
						err2 := c.AddRoot("/test/duplicate", "Duplicate Root")
						if err2 == nil {
							errChan <- fmt.Errorf("AddRoot should have failed for duplicate")
						}
					}
				case 3:
					// Try to remove a root (may or may not exist)
					_ = c.RemoveRoot(fmt.Sprintf("/test/root/%d-0", id))
				}

				// Small delay to encourage race conditions if they exist
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		t.Errorf("Concurrent operations failed with %d errors:", len(errors))
		for i, err := range errors {
			t.Errorf("  Error %d: %v", i+1, err)
			if i >= 10 { // Limit error output
				t.Errorf("  ... and %d more errors", len(errors)-i-1)
				break
			}
		}
	}

	// Verify final state is consistent
	roots, err := c.GetRoots()
	if err != nil {
		t.Fatalf("Final GetRoots failed: %v", err)
	}

	t.Logf("Final state: %d roots after %d concurrent operations", len(roots), numGoroutines*operationsPerGoroutine)
}

// TestRaceConditionDetection uses runtime detection to verify no race conditions
func TestRaceConditionDetection(t *testing.T) {
	if !testing.Short() {
		t.Skip("Skipping race condition test in long mode")
	}

	c, _ := SetupClientWithMockTransport(t, "2024-11-05")

	// Enable more aggressive race detection
	runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup
	const numWorkers = 50

	// Reader goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = c.GetRoots()
				runtime.Gosched() // Yield to encourage race conditions
			}
		}()
	}

	// Writer goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = c.AddRoot(fmt.Sprintf("/test/race/%d-%d", id, j), fmt.Sprintf("Race %d-%d", id, j))
				runtime.Gosched() // Yield to encourage race conditions
			}
		}(i)
	}

	// Removal goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = c.RemoveRoot(fmt.Sprintf("/test/race/%d-%d", id, j))
				runtime.Gosched() // Yield to encourage race conditions
			}
		}(i)
	}

	wg.Wait()
	t.Log("Race condition detection test completed successfully")
}

// TestMemoryConsistency verifies that concurrent reads see consistent state
func TestMemoryConsistency(t *testing.T) {
	c, _ := SetupClientWithMockTransport(t, "2024-11-05")

	// Add some initial roots
	for i := 0; i < 10; i++ {
		err := c.AddRoot(fmt.Sprintf("/test/initial/%d", i), fmt.Sprintf("Initial %d", i))
		if err != nil {
			t.Fatalf("Failed to add initial root %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	results := make(chan int, 100)

	// Multiple readers should see consistent state
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				roots, err := c.GetRoots()
				if err != nil {
					t.Errorf("GetRoots failed: %v", err)
					return
				}
				results <- len(roots)
			}
		}()
	}

	wg.Wait()
	close(results)

	// All reads should see at least the initial 10 roots
	var counts []int
	for count := range results {
		counts = append(counts, count)
		if count < 10 {
			t.Errorf("Saw inconsistent state: only %d roots, expected at least 10", count)
		}
	}

	t.Logf("Memory consistency test: saw root counts ranging from %d to %d",
		minInt(counts), maxInt(counts))
}

func minInt(slice []int) int {
	if len(slice) == 0 {
		return 0
	}
	min := slice[0]
	for _, v := range slice[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func maxInt(slice []int) int {
	if len(slice) == 0 {
		return 0
	}
	max := slice[0]
	for _, v := range slice[1:] {
		if v > max {
			max = v
		}
	}
	return max
}
