// Test script to demonstrate process group functionality
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		runChildProcess()
		return
	}

	testProcessGroup()
}

func testProcessGroup() {
	fmt.Println("=== Process Group Test ===")
	fmt.Printf("Operating System: %s\n", runtime.GOOS)

	if runtime.GOOS == "windows" {
		fmt.Println("Process groups are not demonstrated on Windows")
		return
	}

	// Create a command that will spawn children
	cmd := exec.Command("go", "run", "main.go", "child")

	// Set up process group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Start the process
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start process: %v\n", err)
		return
	}

	fmt.Printf("Started main process with PID: %d\n", cmd.Process.Pid)

	// Get the process group ID
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		fmt.Printf("Failed to get process group ID: %v\n", err)
		return
	}

	fmt.Printf("Process group ID: %d\n", pgid)

	// Let the process run for a bit to create children
	fmt.Println("Waiting for child processes to spawn...")
	time.Sleep(3 * time.Second)

	// Check what processes exist before killing
	fmt.Println("\n--- Processes before cleanup ---")
	beforePIDs := findProcessesInGroup(pgid)
	for _, pid := range beforePIDs {
		processInfo := getProcessInfo(pid)
		fmt.Printf("PID %d: %s\n", pid, processInfo)
	}

	// Now kill the entire process group
	fmt.Println("\n--- Killing entire process group ---")
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		fmt.Printf("Failed to kill process group: %v\n", err)
		return
	}

	// Wait for the process to die
	if err := cmd.Wait(); err != nil {
		// This is expected (killed by signal)
		fmt.Printf("Process ended as expected: %v\n", err)
	}

	// Wait a moment for cleanup
	time.Sleep(1 * time.Second)

	// Check what processes exist after killing
	fmt.Println("\n--- Processes after cleanup ---")
	afterPIDs := findProcessesInGroup(pgid)
	if len(afterPIDs) == 0 {
		fmt.Println("✓ All processes in group successfully cleaned up!")
	} else {
		fmt.Println("⚠️  Some processes may still be running:")
		for _, pid := range afterPIDs {
			processInfo := getProcessInfo(pid)
			fmt.Printf("PID %d: %s\n", pid, processInfo)
		}
	}

	// Also check for any orphaned sleep processes
	orphanedSleeps := findOrphanedSleepProcesses()
	if len(orphanedSleeps) == 0 {
		fmt.Println("✓ No orphaned sleep processes found!")
	} else {
		fmt.Println("⚠️  Found orphaned sleep processes:")
		for _, pid := range orphanedSleeps {
			processInfo := getProcessInfo(pid)
			fmt.Printf("PID %d: %s\n", pid, processInfo)
		}
	}

	fmt.Println("\n✓ Process group cleanup test completed!")
}

func runChildProcess() {
	fmt.Printf("Child process started with PID: %d, PPID: %d\n", os.Getpid(), os.Getppid())

	// Create multiple grandchild processes
	for i := 0; i < 2; i++ {
		grandchild := exec.Command("sleep", "30")
		if err := grandchild.Start(); err != nil {
			fmt.Printf("Failed to start grandchild %d: %v\n", i, err)
			continue
		}
		fmt.Printf("Grandchild %d started with PID: %d\n", i+1, grandchild.Process.Pid)
	}

	// Create a great-grandchild through shell
	shell := exec.Command("sh", "-c", "sleep 30 & sleep 30 & wait")
	if err := shell.Start(); err != nil {
		fmt.Printf("Failed to start shell with great-grandchildren: %v\n", err)
	} else {
		fmt.Printf("Shell with great-grandchildren started with PID: %d\n", shell.Process.Pid)
	}

	// Wait indefinitely (until killed by parent)
	select {}
}

// findProcessesInGroup finds all processes with the given process group ID
func findProcessesInGroup(pgid int) []int {
	var pids []int

	cmd := exec.Command("ps", "-eo", "pid,pgid")
	output, err := cmd.Output()
	if err != nil {
		return pids
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pid, err1 := strconv.Atoi(fields[0])
			groupID, err2 := strconv.Atoi(fields[1])

			if err1 == nil && err2 == nil && groupID == pgid {
				pids = append(pids, pid)
			}
		}
	}

	return pids
}

// getProcessInfo gets information about a process
func getProcessInfo(pid int) string {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "pid,ppid,pgid,comm")
	output, err := cmd.Output()
	if err != nil {
		return "process not found"
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		return strings.TrimSpace(lines[1])
	}

	return "unknown"
}

// findOrphanedSleepProcesses finds any sleep processes that might be orphaned
func findOrphanedSleepProcesses() []int {
	var pids []int

	cmd := exec.Command("pgrep", "sleep")
	output, err := cmd.Output()
	if err != nil {
		return pids
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line != "" {
			if pid, err := strconv.Atoi(line); err == nil {
				pids = append(pids, pid)
			}
		}
	}

	return pids
}
