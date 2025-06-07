// Package util provides utilities for the gomcp library.
package util

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// ProcessGroupManager provides enhanced process cleanup capabilities
// that work in conjunction with the MCP ServerRegistry to prevent orphan processes.
type ProcessGroupManager struct {
	processGroups map[int]bool // Track process groups we've created
}

// NewProcessGroupManager creates a new process group manager.
func NewProcessGroupManager() *ProcessGroupManager {
	return &ProcessGroupManager{
		processGroups: make(map[int]bool),
	}
}

// PrepareCommand configures a command to run in its own process group
// for enhanced cleanup capabilities. This should be called before cmd.Start().
func (pgm *ProcessGroupManager) PrepareCommand(cmd *exec.Cmd) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	// Set up process group based on OS
	if runtime.GOOS != "windows" {
		// On Unix-like systems, create a new process group
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true, // Create new process group
		}
	} else {
		// On Windows, create a new process group
		// Note: Windows process group handling is more complex and may require
		// platform-specific imports. For now, we skip process group setup on Windows.
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	return nil
}

// RegisterProcessGroup registers a process group for cleanup tracking.
// Call this after cmd.Start() succeeds.
func (pgm *ProcessGroupManager) RegisterProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("command or process cannot be nil")
	}

	pid := cmd.Process.Pid

	if runtime.GOOS != "windows" {
		// On Unix-like systems, the process group ID is the same as the process ID
		// when we used Setpgid: true
		pgm.processGroups[pid] = true
	} else {
		// On Windows, track the process ID
		pgm.processGroups[pid] = true
	}

	return nil
}

// TerminateProcessGroup terminates a process group more aggressively than
// the standard ServerRegistry termination. This kills the entire process tree.
func (pgm *ProcessGroupManager) TerminateProcessGroup(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid

	// Remove from tracking
	delete(pgm.processGroups, pid)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// First, try the standard approach (close stdin, wait)
	if stdinCloser, ok := cmd.Stdin.(interface{ Close() error }); ok && stdinCloser != nil {
		stdinCloser.Close()
	}

	// Wait briefly for graceful shutdown
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		return nil // Process exited gracefully
	case <-time.After(2 * time.Second):
		// Proceed to force termination
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for process %d to terminate", pid)
	}

	// Force kill the entire process group
	if runtime.GOOS != "windows" {
		// On Unix-like systems, send SIGKILL to the entire process group
		// Negative PID kills the process group
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			// Fallback to killing just the main process
			if killErr := cmd.Process.Kill(); killErr != nil {
				return fmt.Errorf("failed to kill process or process group: %v (original: %v)", killErr, err)
			}
		}
	} else {
		// On Windows, kill the process (Windows process groups are more complex)
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %v", err)
		}
	}

	// Wait for process death
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("process %d did not die after SIGKILL within timeout", pid)
	}
}

// CleanupAllProcessGroups forcefully terminates all tracked process groups.
// This is a nuclear option for application shutdown.
func (pgm *ProcessGroupManager) CleanupAllProcessGroups(timeout time.Duration) error {
	if len(pgm.processGroups) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var lastErr error

	// Copy the process groups to avoid map modification during iteration
	processGroupsCopy := make([]int, 0, len(pgm.processGroups))
	for pgid := range pgm.processGroups {
		processGroupsCopy = append(processGroupsCopy, pgid)
	}

	// Kill all process groups
	for _, pgid := range processGroupsCopy {
		if runtime.GOOS != "windows" {
			// Send SIGKILL to process group (negative PID kills the whole group)
			if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
				// Try killing the individual process as fallback
				if err := syscall.Kill(pgid, syscall.SIGKILL); err != nil {
					lastErr = fmt.Errorf("failed to kill process group %d: %v", pgid, err)
				}
			}
		} else {
			// On Windows, find and kill the process
			if proc, err := os.FindProcess(pgid); err == nil {
				if killErr := proc.Kill(); killErr != nil {
					lastErr = fmt.Errorf("failed to kill Windows process %d: %v", pgid, killErr)
				}
			}
		}
		delete(pgm.processGroups, pgid)
	}

	// Brief wait to let processes die
	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
	}

	return lastErr
}

// GetTrackedProcessGroups returns the number of currently tracked process groups.
func (pgm *ProcessGroupManager) GetTrackedProcessGroups() int {
	return len(pgm.processGroups)
}
