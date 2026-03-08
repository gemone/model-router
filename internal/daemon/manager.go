package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DaemonStatus represents the status of a daemon process
type DaemonStatus struct {
	Running bool
	PID     int
	Uptime  time.Duration
}

// Manager handles daemon process management
type Manager struct {
	PIDFile string
	LogFile string
}

// NewDaemonManager creates a new daemon manager
func NewDaemonManager(pidFile, logFile string) *Manager {
	return &Manager{
		PIDFile: pidFile,
		LogFile: logFile,
	}
}

// Start starts the daemon process
func (m *Manager) Start(configPath string) error {
	// Check if already running
	if pid, err := m.readPID(); err == nil {
		if m.isProcessRunning(pid) {
			return fmt.Errorf("server already running with PID %d", pid)
		}
		// Stale PID file, remove it
		os.Remove(m.PIDFile)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Dir(m.PIDFile), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build the command
	args := []string{"serve", "--config", configPath}
	cmd := exec.Command(execPath, args...)

	// Set up log file
	logFile, err := os.OpenFile(m.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Detach from terminal
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Write PID file
	if err := m.writePID(cmd.Process.Pid); err != nil {
		// Kill the process if we can't write PID file
		cmd.Process.Kill()
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Wait a moment to ensure process started successfully
	time.Sleep(100 * time.Millisecond)

	// Check if process is still running
	if !m.isProcessRunning(cmd.Process.Pid) {
		return fmt.Errorf("daemon process exited immediately, check log file: %s", m.LogFile)
	}

	return nil
}

// Stop stops the daemon process
func (m *Manager) Stop(force bool) error {
	pid, err := m.readPID()
	if err != nil {
		return fmt.Errorf("server not running (no PID file)")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send SIGTERM directly - the signal will fail if process doesn't exist
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Check if it's "no such process" error (process already exited)
		if strings.Contains(err.Error(), "no such process") || strings.Contains(err.Error(), "process already finished") {
			os.Remove(m.PIDFile)
			return fmt.Errorf("server not running (stale PID file)")
		}
		if force {
			// Try SIGKILL
			if err := process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
		} else {
			return fmt.Errorf("failed to send SIGTERM: %w", err)
		}
	}

	// Wait for process to exit with exponential backoff
	timeout := time.After(10 * time.Second)
	backoff := 10 * time.Millisecond

	for {
		select {
		case <-timeout:
			if force {
				// Force kill
				if err := process.Kill(); err != nil {
					return fmt.Errorf("failed to force kill process: %w", err)
				}
			} else {
				return fmt.Errorf("timeout waiting for process to exit, use --force")
			}
		case <-time.After(backoff):
			if !m.isProcessRunning(pid) {
				os.Remove(m.PIDFile)
				return nil
			}
			// Exponential backoff: double interval, max 500ms
			if backoff < 500*time.Millisecond {
				backoff *= 2
			}
		}
	}
}

// Restart restarts the daemon process
func (m *Manager) Restart(configPath string) error {
	// Try to stop first (ignore errors if not running)
	_ = m.Stop(false)

	// Wait a moment
	time.Sleep(500 * time.Millisecond)

	// Start new instance
	return m.Start(configPath)
}

// Status returns the current status of the daemon
func (m *Manager) Status() (*DaemonStatus, error) {
	status := &DaemonStatus{}

	pid, err := m.readPID()
	if err != nil {
		return status, nil
	}

	status.PID = pid
	status.Running = m.isProcessRunning(pid)

	if status.Running {
		// Try to get uptime from PID file modification time
		if info, err := os.Stat(m.PIDFile); err == nil {
			status.Uptime = time.Since(info.ModTime())
		}
	}

	return status, nil
}

// readPID reads the PID from the PID file
func (m *Manager) readPID() (int, error) {
	data, err := os.ReadFile(m.PIDFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}

// writePID writes the PID to the PID file
func (m *Manager) writePID(pid int) error {
	return os.WriteFile(m.PIDFile, []byte(strconv.Itoa(pid)), 0644)
}

// isProcessRunning checks if a process with the given PID is running
func (m *Manager) isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
