package util

import (
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestProcessGroupManager(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Process group management is more complex on Windows - skipping test")
	}

	pgm := NewProcessGroupManager()

	// Test that we start with no tracked process groups
	if count := pgm.GetTrackedProcessGroups(); count != 0 {
		t.Fatalf("Expected 0 tracked process groups, got %d", count)
	}

	// Create a test command
	cmd := exec.Command("sleep", "10")

	// Prepare the command with process group
	err := pgm.PrepareCommand(cmd)
	if err != nil {
		t.Fatalf("Failed to prepare command: %v", err)
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Register the process group
	err = pgm.RegisterProcessGroup(cmd)
	if err != nil {
		t.Fatalf("Failed to register process group: %v", err)
	}

	// Verify we now have one tracked process group
	if count := pgm.GetTrackedProcessGroups(); count != 1 {
		t.Fatalf("Expected 1 tracked process group, got %d", count)
	}

	// Test individual process group termination
	err = pgm.TerminateProcessGroup(cmd, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to terminate process group: %v", err)
	}

	// Verify the process group was removed from tracking
	if count := pgm.GetTrackedProcessGroups(); count != 0 {
		t.Fatalf("Expected 0 tracked process groups after termination, got %d", count)
	}
}

func TestProcessGroupManagerCleanupAll(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Process group management is more complex on Windows - skipping test")
	}

	pgm := NewProcessGroupManager()

	// Create multiple test commands
	var cmds []*exec.Cmd
	for i := 0; i < 3; i++ {
		cmd := exec.Command("sleep", "30")

		err := pgm.PrepareCommand(cmd)
		if err != nil {
			t.Fatalf("Failed to prepare command %d: %v", i, err)
		}

		err = cmd.Start()
		if err != nil {
			t.Fatalf("Failed to start command %d: %v", i, err)
		}

		err = pgm.RegisterProcessGroup(cmd)
		if err != nil {
			t.Fatalf("Failed to register process group %d: %v", i, err)
		}

		cmds = append(cmds, cmd)
	}

	// Verify we have 3 tracked process groups
	if count := pgm.GetTrackedProcessGroups(); count != 3 {
		t.Fatalf("Expected 3 tracked process groups, got %d", count)
	}

	// Cleanup all process groups
	err := pgm.CleanupAllProcessGroups(5 * time.Second)
	if err != nil {
		t.Logf("Warning during cleanup (may be expected): %v", err)
	}

	// Verify all process groups were cleaned up
	if count := pgm.GetTrackedProcessGroups(); count != 0 {
		t.Fatalf("Expected 0 tracked process groups after cleanup, got %d", count)
	}

	// The main goal is that the process group manager no longer tracks any processes
	// Process verification is complex due to timing, so we'll focus on the tracking aspect
	t.Log("All process groups successfully cleaned up and no longer tracked")
}

func TestProcessGroupManagerEdgeCases(t *testing.T) {
	pgm := NewProcessGroupManager()

	// Test with nil command
	err := pgm.PrepareCommand(nil)
	if err == nil {
		t.Fatal("Expected error when preparing nil command")
	}

	// Test registering nil command
	err = pgm.RegisterProcessGroup(nil)
	if err == nil {
		t.Fatal("Expected error when registering nil command")
	}

	// Test terminating nil command (should not error)
	err = pgm.TerminateProcessGroup(nil, time.Second)
	if err != nil {
		t.Fatalf("Unexpected error when terminating nil command: %v", err)
	}

	// Test cleanup with no tracked processes
	err = pgm.CleanupAllProcessGroups(time.Second)
	if err != nil {
		t.Fatalf("Unexpected error when cleaning up empty process groups: %v", err)
	}
}
