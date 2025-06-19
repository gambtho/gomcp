// Package util provides utilities for the gomcp library.
package util

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
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

// ProcessMonitor monitors the parent process and stdin for unexpected termination.
// This prevents orphaned MCP server processes when the parent (e.g., Cursor) exits unexpectedly.
type ProcessMonitor struct {
	parentPID     int
	logger        *slog.Logger
	shutdownFunc  func()
	exitFunc      func(int) // Configurable exit function for testing
	stopChan      chan struct{}
	monitorActive bool
}

// NewProcessMonitor creates a new process monitor.
// shutdownFunc will be called when the parent process exits or stdin closes.
func NewProcessMonitor(logger *slog.Logger, shutdownFunc func()) *ProcessMonitor {
	return &ProcessMonitor{
		parentPID:    os.Getppid(),
		logger:       logger,
		shutdownFunc: shutdownFunc,
		exitFunc:     os.Exit, // Default to os.Exit
		stopChan:     make(chan struct{}),
	}
}

// SetExitFunc sets a custom exit function (useful for testing).
// If set to nil, no exit will be performed.
func (pm *ProcessMonitor) SetExitFunc(exitFunc func(int)) {
	pm.exitFunc = exitFunc
}

// Start begins monitoring the parent process and stdin.
// This should be called once when the server starts.
func (pm *ProcessMonitor) Start() {
	if pm.monitorActive {
		return // Already monitoring
	}

	pm.monitorActive = true

	// Start parent PID monitoring
	go pm.monitorParentProcess()

	// Start stdin monitoring
	go pm.monitorStdin()

	// Set up signal handlers
	pm.setupSignalHandlers()

	if pm.logger != nil {
		pm.logger.Debug("process monitor started",
			"parent_pid", pm.parentPID)
	}
}

// Stop stops the process monitor.
func (pm *ProcessMonitor) Stop() {
	if !pm.monitorActive {
		return
	}

	pm.monitorActive = false
	close(pm.stopChan)

	if pm.logger != nil {
		pm.logger.Debug("process monitor stopped")
	}
}

// monitorParentProcess watches for parent process termination.
// If the parent PID changes to 1 (init), it means our parent died.
func (pm *ProcessMonitor) monitorParentProcess() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopChan:
			return
		case <-ticker.C:
			currentParentPID := os.Getppid()
			if currentParentPID != pm.parentPID {
				if pm.logger != nil {
					pm.logger.Info("parent process died, shutting down",
						"original_parent_pid", pm.parentPID,
						"current_parent_pid", currentParentPID)
				}
				pm.gracefulShutdown("parent process died")
				return
			}
		}
	}
}

// monitorStdin watches for stdin closure (POLLHUP equivalent).
// When the parent process exits, stdin gets closed/disconnected.
func (pm *ProcessMonitor) monitorStdin() {
	// Create a buffer to attempt reading from stdin
	buffer := make([]byte, 1)

	for {
		select {
		case <-pm.stopChan:
			return
		default:
			// Set a read deadline to avoid blocking forever
			os.Stdin.SetReadDeadline(time.Now().Add(1 * time.Second))

			// Try to read from stdin
			n, err := os.Stdin.Read(buffer)

			// Reset deadline
			os.Stdin.SetReadDeadline(time.Time{})

			if err != nil {
				// Check if it's a timeout (expected)
				if os.IsTimeout(err) {
					continue
				}

				// EOF or other error indicates stdin closed
				if pm.logger != nil {
					pm.logger.Info("stdin closed or error, shutting down",
						"error", err.Error())
				}
				pm.gracefulShutdown("stdin closed")
				return
			}

			// If we actually read data, we need to handle it properly
			// For MCP servers, this shouldn't happen during monitoring
			// as the main readLoop should be handling stdin
			if n > 0 {
				if pm.logger != nil {
					pm.logger.Debug("unexpected data read during stdin monitoring",
						"bytes", n)
				}
			}

			// Brief sleep to avoid CPU spinning
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// setupSignalHandlers configures signal handlers for graceful shutdown.
func (pm *ProcessMonitor) setupSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-pm.stopChan:
			return
		case sig := <-sigChan:
			if pm.logger != nil {
				pm.logger.Info("received signal, shutting down",
					"signal", sig.String())
			}
			pm.gracefulShutdown("signal received: " + sig.String())
		}
	}()
}

// gracefulShutdown performs a graceful shutdown.
func (pm *ProcessMonitor) gracefulShutdown(reason string) {
	if !pm.monitorActive {
		return // Already shutting down
	}

	pm.monitorActive = false

	if pm.logger != nil {
		pm.logger.Info("initiating graceful shutdown", "reason", reason)
	}

	// Call the shutdown function if provided
	if pm.shutdownFunc != nil {
		pm.shutdownFunc()
	}

	// Give a brief moment for cleanup
	time.Sleep(100 * time.Millisecond)

	// Exit the process (if exit function is set)
	if pm.exitFunc != nil {
		pm.exitFunc(0)
	}
}
