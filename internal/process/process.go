// Package process handles process tracking and management for sbox.
package process

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// ProcessFile stores running process information
	ProcessFile = "processes.json"
	// LogDir stores process logs
	LogDir = "logs"
)

// ProcessInfo represents a running sandbox process
type ProcessInfo struct {
	PID       int       `json:"pid"`
	Name      string    `json:"name"`
	Command   string    `json:"command"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"` // running, stopped, crashed
	LogFile   string    `json:"log_file"`
	Project   string    `json:"project"`
}

// ProcessManager handles process lifecycle
type ProcessManager struct {
	SboxDir     string
	ProjectRoot string
	ProjectName string
}

// NewProcessManager creates a new process manager
func NewProcessManager(projectRoot string) *ProcessManager {
	return &ProcessManager{
		SboxDir:     filepath.Join(projectRoot, ".sbox"),
		ProjectRoot: projectRoot,
		ProjectName: filepath.Base(projectRoot),
	}
}

// GetProcessFile returns the path to the process tracking file
func (pm *ProcessManager) GetProcessFile() string {
	return filepath.Join(pm.SboxDir, ProcessFile)
}

// GetLogDir returns the path to the logs directory
func (pm *ProcessManager) GetLogDir() string {
	return filepath.Join(pm.SboxDir, LogDir)
}

// GetLogFile returns the path to a specific log file
func (pm *ProcessManager) GetLogFile(name string) string {
	return filepath.Join(pm.GetLogDir(), fmt.Sprintf("%s.log", name))
}

// EnsureLogDir creates the log directory if it doesn't exist
func (pm *ProcessManager) EnsureLogDir() error {
	return os.MkdirAll(pm.GetLogDir(), 0755)
}

// LoadProcesses loads all tracked processes
func (pm *ProcessManager) LoadProcesses() ([]ProcessInfo, error) {
	data, err := os.ReadFile(pm.GetProcessFile())
	if err != nil {
		if os.IsNotExist(err) {
			return []ProcessInfo{}, nil
		}
		return nil, err
	}

	var processes []ProcessInfo
	if err := json.Unmarshal(data, &processes); err != nil {
		return nil, err
	}

	return processes, nil
}

// SaveProcesses saves the process list
func (pm *ProcessManager) SaveProcesses(processes []ProcessInfo) error {
	data, err := json.MarshalIndent(processes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pm.GetProcessFile(), data, 0644)
}

// AddProcess adds a new process to tracking
func (pm *ProcessManager) AddProcess(info ProcessInfo) error {
	processes, err := pm.LoadProcesses()
	if err != nil {
		processes = []ProcessInfo{}
	}

	// Remove any existing entry with same name
	var filtered []ProcessInfo
	for _, p := range processes {
		if p.Name != info.Name {
			filtered = append(filtered, p)
		}
	}

	filtered = append(filtered, info)
	return pm.SaveProcesses(filtered)
}

// RemoveProcess removes a process from tracking
func (pm *ProcessManager) RemoveProcess(name string) error {
	processes, err := pm.LoadProcesses()
	if err != nil {
		return err
	}

	var filtered []ProcessInfo
	for _, p := range processes {
		if p.Name != name {
			filtered = append(filtered, p)
		}
	}

	return pm.SaveProcesses(filtered)
}

// GetProcess gets a specific process by name
func (pm *ProcessManager) GetProcess(name string) (*ProcessInfo, error) {
	processes, err := pm.LoadProcesses()
	if err != nil {
		return nil, err
	}

	for _, p := range processes {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("process '%s' not found", name)
}

// IsProcessRunning checks if a process is still running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// UpdateProcessStatus updates the status of all tracked processes
func (pm *ProcessManager) UpdateProcessStatus() ([]ProcessInfo, error) {
	processes, err := pm.LoadProcesses()
	if err != nil {
		return nil, err
	}

	updated := false
	for i := range processes {
		if processes[i].Status == "running" {
			if !IsProcessRunning(processes[i].PID) {
				processes[i].Status = "stopped"
				updated = true
			}
		}
	}

	if updated {
		pm.SaveProcesses(processes)
	}

	return processes, nil
}

// GetRunningProcesses returns only running processes
func (pm *ProcessManager) GetRunningProcesses() ([]ProcessInfo, error) {
	processes, err := pm.UpdateProcessStatus()
	if err != nil {
		return nil, err
	}

	var running []ProcessInfo
	for _, p := range processes {
		if p.Status == "running" && IsProcessRunning(p.PID) {
			running = append(running, p)
		}
	}

	return running, nil
}

// StopProcess stops a running process
func (pm *ProcessManager) StopProcess(name string) error {
	info, err := pm.GetProcess(name)
	if err != nil {
		return err
	}

	if info.Status != "running" {
		return fmt.Errorf("process '%s' is not running (status: %s)", name, info.Status)
	}

	process, err := os.FindProcess(info.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Try graceful shutdown first (SIGTERM)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		process.Signal(syscall.SIGKILL)
	}

	// Wait a bit for process to terminate
	time.Sleep(100 * time.Millisecond)

	// Update status
	info.Status = "stopped"
	processes, _ := pm.LoadProcesses()
	for i := range processes {
		if processes[i].Name == name {
			processes[i].Status = "stopped"
			break
		}
	}
	pm.SaveProcesses(processes)

	return nil
}

// StartDaemon starts a command as a background daemon with logging
func (pm *ProcessManager) StartDaemon(name, command string, env []string, workdir string) (*ProcessInfo, error) {
	if err := pm.EnsureLogDir(); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := pm.GetLogFile(name)

	// Open log file for writing
	logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Write startup header
	fmt.Fprintf(logFd, "\n=== sbox daemon started at %s ===\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(logFd, "Command: %s\n", command)
	fmt.Fprintf(logFd, "Workdir: %s\n", workdir)
	fmt.Fprintf(logFd, "=========================================\n\n")

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workdir
	cmd.Env = env
	cmd.Stdout = logFd
	cmd.Stderr = logFd

	// Start the process
	if err := cmd.Start(); err != nil {
		logFd.Close()
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	info := ProcessInfo{
		PID:       cmd.Process.Pid,
		Name:      name,
		Command:   command,
		StartTime: time.Now(),
		Status:    "running",
		LogFile:   logFile,
		Project:   pm.ProjectName,
	}

	// Track the process
	if err := pm.AddProcess(info); err != nil {
		return nil, fmt.Errorf("failed to track process: %w", err)
	}

	// Start a goroutine to wait for process and update status
	go func() {
		cmd.Wait()
		logFd.Close()
		// Update process status when it exits
		processes, _ := pm.LoadProcesses()
		for i := range processes {
			if processes[i].PID == info.PID {
				processes[i].Status = "stopped"
				break
			}
		}
		pm.SaveProcesses(processes)
	}()

	return &info, nil
}

// ReadLogs reads the last n lines from a log file
func (pm *ProcessManager) ReadLogs(name string, lines int, follow bool) error {
	logFile := pm.GetLogFile(name)

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("no logs found for '%s'", name)
	}

	if follow {
		return pm.tailFollow(logFile)
	}

	return pm.tailLines(logFile, lines)
}

// tailLines reads the last n lines from a file
func (pm *ProcessManager) tailLines(filename string, n int) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Print last n lines
	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for i := start; i < len(lines); i++ {
		fmt.Println(lines[i])
	}

	return scanner.Err()
}

// tailFollow follows a log file like tail -f
func (pm *ProcessManager) tailFollow(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Seek to end
	file.Seek(0, io.SeekEnd)

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return err
		}
		fmt.Print(line)
	}
}

// ListLogs lists all available log files
func (pm *ProcessManager) ListLogs() ([]string, error) {
	logDir := pm.GetLogDir()
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	var logs []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			name := strings.TrimSuffix(entry.Name(), ".log")
			logs = append(logs, name)
		}
	}

	return logs, nil
}

// CleanOldLogs removes log files older than the specified duration
func (pm *ProcessManager) CleanOldLogs(maxAge time.Duration) (int, error) {
	logDir := pm.GetLogDir()
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return 0, nil
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(logDir, entry.Name())
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// GetLogSize returns the size of a log file
func (pm *ProcessManager) GetLogSize(name string) (int64, error) {
	logFile := pm.GetLogFile(name)
	info, err := os.Stat(logFile)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// FormatDuration formats a duration in human-readable form
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}

// FormatBytes formats bytes in human-readable form
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetSystemProcesses finds all sbox-related processes on the system
func GetSystemProcesses() ([]ProcessInfo, error) {
	// Use ps to find processes with SBOX_ACTIVE env var
	cmd := exec.Command("ps", "-eo", "pid,etime,args")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var processes []ProcessInfo
	lines := strings.Split(string(output), "\n")

	for _, line := range lines[1:] { // Skip header
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is an sbox process
		if !strings.Contains(line, "SBOX_ACTIVE") && !strings.Contains(line, "sbox") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		processes = append(processes, ProcessInfo{
			PID:     pid,
			Command: strings.Join(fields[2:], " "),
			Status:  "running",
		})
	}

	return processes, nil
}
