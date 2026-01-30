// sbox - A rootless, user-space sandbox runtime.
// Docker-like workflow without sudo.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sbox-project/sbox/internal/builder"
	"github.com/sbox-project/sbox/internal/config"
	"github.com/sbox-project/sbox/internal/console"
	"github.com/sbox-project/sbox/internal/process"
	"github.com/sbox-project/sbox/internal/runner"
	"github.com/sbox-project/sbox/internal/validate"
)

const version = "0.3.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "sbox",
		Short: "A rootless, user-space sandbox runtime",
		Long:  "sbox - Docker-like workflow without sudo.\nA rootless, user-space sandbox runtime for Python and Node.js applications.",
	}

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sbox version %s\n", version)
		},
	})

	// Init command
	initCmd := &cobra.Command{
		Use:   "init <project_name>",
		Short: "Initialize a new sbox project",
		Args:  cobra.ExactArgs(1),
		Run:   runInit,
	}
	initCmd.Flags().StringP("runtime", "r", "python:3.10", "Runtime to use (python:X.Y or node:X)")
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing project")
	rootCmd.AddCommand(initCmd)

	// Build command
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build the sandbox environment",
		Run:   runBuild,
	}
	buildCmd.Flags().BoolP("force", "f", false, "Force rebuild even if up to date")
	buildCmd.Flags().BoolP("verbose", "v", false, "Show detailed build output")
	rootCmd.AddCommand(buildCmd)

	// Run command
	runCmd := &cobra.Command{
		Use:   "run [command]",
		Short: "Run the application in the sandbox",
		Long: `Run the application in the sandbox environment.

If no command is provided, uses the default command from config.yaml.
Use --detach to run as a background daemon with logging.`,
		Run: runRun,
	}
	runCmd.Flags().BoolP("detach", "d", false, "Run in background as daemon")
	runCmd.Flags().StringP("name", "n", "", "Name for the daemon process (default: project name)")
	rootCmd.AddCommand(runCmd)

	// Shell command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "shell",
		Short: "Start an interactive shell in the sandbox",
		Run:   runShell,
	})

	// Exec command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "exec <command> [args...]",
		Short: "Execute a command in the sandbox",
		Args:  cobra.MinimumNArgs(1),
		Run:   runExec,
	})

	// Status command (enhanced)
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show detailed project status",
		Long: `Show detailed status information about the sbox project.

Displays:
- Project configuration
- Build status and metadata
- Environment details
- Running processes
- Available logs`,
		Run: runStatus,
	}
	statusCmd.Flags().BoolP("json", "j", false, "Output status as JSON")
	rootCmd.AddCommand(statusCmd)

	// PS command - show running processes
	psCmd := &cobra.Command{
		Use:   "ps",
		Short: "List running sandbox processes",
		Long: `List all running sandbox processes for this project.

Shows process ID, name, command, uptime, and status.
Use --all to show stopped processes as well.`,
		Run: runPs,
	}
	psCmd.Flags().BoolP("all", "a", false, "Show all processes (including stopped)")
	psCmd.Flags().BoolP("quiet", "q", false, "Only show process IDs")
	rootCmd.AddCommand(psCmd)

	// Logs command
	logsCmd := &cobra.Command{
		Use:   "logs [name]",
		Short: "View process logs",
		Long: `View logs for a sandbox process.

If no name is provided, shows logs for the default process.
Use --follow to stream new log entries in real-time.`,
		Run: runLogs,
	}
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output (like tail -f)")
	logsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")
	logsCmd.Flags().Bool("list", false, "List available log files")
	rootCmd.AddCommand(logsCmd)

	// Stop command
	stopCmd := &cobra.Command{
		Use:   "stop [name]",
		Short: "Stop a running daemon process",
		Long: `Stop a running daemon process.

If no name is provided, stops the default process.
Use --all to stop all running processes.`,
		Run: runStop,
	}
	stopCmd.Flags().BoolP("all", "a", false, "Stop all running processes")
	rootCmd.AddCommand(stopCmd)

	// Restart command
	restartCmd := &cobra.Command{
		Use:   "restart [name]",
		Short: "Restart a daemon process",
		Run:   runRestart,
	}
	rootCmd.AddCommand(restartCmd)

	// Clean command
	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean the sandbox environment",
		Long: `Clean the sandbox build artifacts.

By default, keeps configuration files.
Use --all to remove everything including config.
Use --logs to only clean log files.`,
		Run: runClean,
	}
	cleanCmd.Flags().BoolP("all", "a", false, "Remove everything including config")
	cleanCmd.Flags().Bool("logs", false, "Only clean log files")
	cleanCmd.Flags().Duration("logs-older-than", 7*24*time.Hour, "Remove logs older than duration (e.g., 24h, 7d)")
	rootCmd.AddCommand(cleanCmd)

	// Info command - detailed environment info
	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Show detailed environment information",
		Run:   runInfo,
	}
	rootCmd.AddCommand(infoCmd)

	// Validate command - check config validity
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the configuration file",
		Long: `Validate the sbox configuration file for errors and warnings.

Checks for:
- Required fields (runtime, workdir, etc.)
- Valid runtime format and supported versions
- Copy specification syntax and source existence
- Install command compatibility with runtime
- Environment variable naming and reserved names
- Common configuration mistakes`,
		Run: runValidate,
	}
	validateCmd.Flags().BoolP("quiet", "q", false, "Only show errors, not warnings")
	validateCmd.Flags().Bool("fix", false, "Attempt to fix common issues")
	rootCmd.AddCommand(validateCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runInit(cmd *cobra.Command, args []string) {
	projectName := args[0]
	runtimeStr, _ := cmd.Flags().GetString("runtime")
	force, _ := cmd.Flags().GetBool("force")

	projectPath := filepath.Join(".", projectName)

	// Check if project exists
	if info, err := os.Stat(projectPath); err == nil && info.IsDir() {
		if !force {
			console.Fatal("Directory '%s' already exists. Use --force to overwrite.", projectName)
		}
		os.RemoveAll(projectPath)
	}

	console.Step("Initializing sbox project: %s", projectName)
	console.Info("Runtime: %s", runtimeStr)

	// Create project structure
	sboxDir := filepath.Join(projectPath, config.SboxDir)
	dirs := []string{
		sboxDir,
		filepath.Join(sboxDir, "env"),
		filepath.Join(sboxDir, "rootfs"),
		filepath.Join(sboxDir, "bin"),
		filepath.Join(sboxDir, "logs"),
		filepath.Join(projectPath, "app"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			console.Fatal("Failed to create directory: %s", err)
		}
	}
	console.Success("Created directory structure")

	// Create runtime-specific files
	runtime := config.RuntimeInfo{}
	parts := strings.SplitN(runtimeStr, ":", 2)
	if len(parts) >= 1 {
		runtime.Language = strings.ToLower(parts[0])
	}
	if len(parts) >= 2 {
		runtime.Version = parts[1]
	}

	if runtime.Language == "node" || runtime.Language == "nodejs" {
		// Create package.json for Node.js
		packageJSON := `{
  "name": "` + projectName + `",
  "version": "1.0.0",
  "description": "A sbox project",
  "main": "main.js",
  "scripts": {
    "start": "node main.js"
  }
}
`
		if err := os.WriteFile(filepath.Join(projectPath, "app", "package.json"), []byte(packageJSON), 0644); err != nil {
			console.Fatal("Failed to create package.json: %s", err)
		}

		// Create main.js
		mainJS := `// Main entry point for the application

function main() {
    console.log("Hello from sbox!");
    console.log("Edit app/main.js to get started.");
}

main();
`
		if err := os.WriteFile(filepath.Join(projectPath, "app", "main.js"), []byte(mainJS), 0644); err != nil {
			console.Fatal("Failed to create main.js: %s", err)
		}
		console.Success("Created Node.js project files")
	} else {
		// Create Python files (default)
		mainPy := `#!/usr/bin/env python3
"""
Main entry point for the application.
"""

def main():
    print("Hello from sbox!")
    print("Edit app/main.py to get started.")

if __name__ == "__main__":
    main()
`
		if err := os.WriteFile(filepath.Join(projectPath, "app", "main.py"), []byte(mainPy), 0644); err != nil {
			console.Fatal("Failed to create main.py: %s", err)
		}

		// Create requirements.txt
		reqTxt := "# Add your dependencies here\n"
		if err := os.WriteFile(filepath.Join(projectPath, "app", "requirements.txt"), []byte(reqTxt), 0644); err != nil {
			console.Fatal("Failed to create requirements.txt: %s", err)
		}
		console.Success("Created Python project files")
	}

	// Create config
	cfg := config.NewDefaultConfig(runtimeStr)
	if err := cfg.Save(projectPath); err != nil {
		console.Fatal("Failed to create config: %s", err)
	}
	console.Success("Created config.yaml")

	// Create .gitignore
	gitignore := `.sbox/env/
.sbox/rootfs/
.sbox/bin/
.sbox/mamba/
.sbox/logs/
sbox.lock
__pycache__/
*.pyc
node_modules/
.env
`
	if err := os.WriteFile(filepath.Join(projectPath, ".gitignore"), []byte(gitignore), 0644); err != nil {
		console.Fatal("Failed to create .gitignore: %s", err)
	}
	console.Success("Created .gitignore")

	fmt.Println()
	console.Success("Project initialized successfully!")
	fmt.Println()
	console.Print("  Project structure:")
	console.Print("  %s/", projectName)
	console.Print("  ├── .sbox/")
	console.Print("  │   ├── config.yaml")
	console.Print("  │   └── logs/")
	console.Print("  ├── app/")
	if runtime.Language == "node" || runtime.Language == "nodejs" {
		console.Print("  │   ├── main.js")
		console.Print("  │   └── package.json")
	} else {
		console.Print("  │   ├── main.py")
		console.Print("  │   └── requirements.txt")
	}
	console.Print("  └── .gitignore")
	fmt.Println()
	console.Print("  Next steps:")
	console.Print("    cd %s", projectName)
	console.Print("    sbox build      # Build the sandbox environment")
	console.Print("    sbox run        # Run the application")
	console.Print("    sbox status     # Check project status")
}

func runBuild(cmd *cobra.Command, args []string) {
	force, _ := cmd.Flags().GetBool("force")
	verbose, _ := cmd.Flags().GetBool("verbose")

	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project. Run 'sbox init <name>' first.")
	}

	projectName := filepath.Base(projectRoot)
	console.Step("Building sandbox: %s", projectName)

	cfg, err := config.Load(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	// Validate configuration before building
	console.Info("Validating configuration...")
	validationResult := validate.ValidateConfig(cfg, projectRoot)

	if !validationResult.Valid {
		console.Error("Configuration validation failed:")
		fmt.Println()
		for _, verr := range validationResult.Errors {
			console.Print("  ✗ %s: %s", verr.Field, verr.Message)
			if verr.Hint != "" {
				console.Print("    → %s", verr.Hint)
			}
		}
		fmt.Println()
		console.Fatal("Fix the configuration errors above and try again. Run 'sbox validate' for more details.")
	}

	// Show warnings but continue
	if len(validationResult.Warnings) > 0 {
		console.Warning("Configuration has %d warning(s):", len(validationResult.Warnings))
		for _, warn := range validationResult.Warnings {
			console.Print("  ⚠ %s: %s", warn.Field, warn.Message)
		}
		fmt.Println()
	}

	console.Success("Configuration validated")
	console.Info("Runtime: %s", cfg.Runtime)
	console.Info("Workdir: %s", cfg.Workdir)

	if !force && config.IsUpToDate(projectRoot, cfg) {
		console.Success("Build is up to date (use --force to rebuild)")
		return
	}

	startTime := time.Now()

	b, err := builder.New(projectRoot)
	if err != nil {
		console.Fatal("Failed to initialize builder: %s", err)
	}

	if verbose {
		console.Info("Starting build process...")
	}

	if err := b.Build(force); err != nil {
		console.Fatal("Build failed: %s", err)
	}

	elapsed := time.Since(startTime)
	fmt.Println()
	console.Success("Build completed in %s", formatDuration(elapsed))

	// Show build summary
	if lock, err := config.LoadLock(projectRoot); err == nil {
		console.Print("  Config hash: %s", lock.ConfigHash[:8])
		console.Print("  Built at: %s", lock.BuiltAt)
	}
}

func runRun(cmd *cobra.Command, args []string) {
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	detach, _ := cmd.Flags().GetBool("detach")
	name, _ := cmd.Flags().GetString("name")

	if name == "" {
		name = filepath.Base(projectRoot)
	}

	// Quick validation before running
	cfg, err := config.Load(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	if err := validate.QuickValidate(cfg, projectRoot); err != nil {
		console.Fatal("Configuration error: %s\n\nRun 'sbox validate' for detailed diagnostics.", err)
	}

	r, err := runner.New(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	var command string
	if len(args) > 0 {
		command = strings.Join(args, " ")
	}

	if detach {
		// Run as daemon
		pm := process.NewProcessManager(projectRoot)

		// Check if already running
		existing, _ := pm.GetProcess(name)
		if existing != nil && existing.Status == "running" && process.IsProcessRunning(existing.PID) {
			console.Fatal("Process '%s' is already running (PID: %d). Use 'sbox stop %s' first.", name, existing.PID, name)
		}

		console.Step("Starting daemon: %s", name)

		cmdToRun := command
		if cmdToRun == "" {
			cmdToRun = r.Config.Cmd
		}
		if cmdToRun == "" {
			console.Fatal("No command specified and no default cmd in config")
		}

		env := r.BuildEnv()
		workdir := r.ResolveWorkdir()

		info, err := pm.StartDaemon(name, cmdToRun, env, workdir)
		if err != nil {
			console.Fatal("Failed to start daemon: %s", err)
		}

		console.Success("Daemon started successfully")
		console.Print("  PID:     %d", info.PID)
		console.Print("  Name:    %s", info.Name)
		console.Print("  Command: %s", info.Command)
		console.Print("  Log:     %s", info.LogFile)
		fmt.Println()
		console.Print("  Use 'sbox logs %s' to view output", name)
		console.Print("  Use 'sbox stop %s' to stop the daemon", name)
		return
	}

	// Run in foreground
	exitCode, err := r.Run(command)
	if err != nil {
		console.Fatal("%s", err)
	}

	os.Exit(exitCode)
}

func runShell(cmd *cobra.Command, args []string) {
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	r, err := runner.New(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	exitCode, err := r.Shell()
	if err != nil {
		console.Fatal("%s", err)
	}

	os.Exit(exitCode)
}

func runExec(cmd *cobra.Command, args []string) {
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	r, err := runner.New(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	exitCode, err := r.Exec(args)
	if err != nil {
		console.Fatal("%s", err)
	}

	os.Exit(exitCode)
}

func runStatus(cmd *cobra.Command, args []string) {
	asJSON, _ := cmd.Flags().GetBool("json")

	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		if asJSON {
			fmt.Println(`{"error": "not in sbox project"}`)
		} else {
			console.Warning("Not in an sbox project")
		}
		return
	}

	projectName := filepath.Base(projectRoot)

	cfg, err := config.Load(projectRoot)
	if err != nil {
		console.Error("Config error: %s", err)
		return
	}

	pm := process.NewProcessManager(projectRoot)
	runningProcesses, _ := pm.GetRunningProcesses()
	allProcesses, _ := pm.LoadProcesses()
	logs, _ := pm.ListLogs()

	// Build status info
	statusInfo := map[string]interface{}{
		"project":  projectName,
		"root":     projectRoot,
		"runtime":  cfg.Runtime,
		"workdir":  cfg.Workdir,
		"command":  cfg.Cmd,
		"built":    config.IsBuilt(projectRoot),
		"upToDate": config.IsUpToDate(projectRoot, cfg),
	}

	if lock, err := config.LoadLock(projectRoot); err == nil {
		statusInfo["buildInfo"] = map[string]string{
			"configHash": lock.ConfigHash,
			"builtAt":    lock.BuiltAt,
			"runtime":    lock.Runtime,
		}
	}

	statusInfo["processes"] = map[string]interface{}{
		"running": len(runningProcesses),
		"total":   len(allProcesses),
	}
	statusInfo["logs"] = logs

	if asJSON {
		data, _ := json.MarshalIndent(statusInfo, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Pretty print status
	fmt.Println()
	console.Step("sbox project: %s", projectName)
	fmt.Println()

	// Configuration section
	console.Print("  ┌─ Configuration")
	console.Print("  │  Runtime:  %s", cfg.Runtime)
	console.Print("  │  Workdir:  %s", cfg.Workdir)
	console.Print("  │  Command:  %s", cfg.Cmd)
	if len(cfg.Env) > 0 {
		console.Print("  │  Env vars: %d defined", len(cfg.Env))
	}
	fmt.Println()

	// Build section
	console.Print("  ┌─ Build Status")
	if config.IsBuilt(projectRoot) {
		console.Print("  │  Status:  ✓ Built")
		if config.IsUpToDate(projectRoot, cfg) {
			console.Print("  │  State:   Up to date")
		} else {
			console.Print("  │  State:   ⚠ Config changed, rebuild recommended")
		}
		if lock, err := config.LoadLock(projectRoot); err == nil {
			console.Print("  │  Hash:    %s", lock.ConfigHash[:8])
			if t, err := time.Parse(time.RFC3339, lock.BuiltAt); err == nil {
				console.Print("  │  Built:   %s (%s ago)", t.Format("2006-01-02 15:04:05"), formatDuration(time.Since(t)))
			}
		}
	} else {
		console.Print("  │  Status:  ✗ Not built")
		console.Print("  │  Run 'sbox build' to build the sandbox")
	}
	fmt.Println()

	// Processes section
	console.Print("  ┌─ Processes")
	if len(runningProcesses) > 0 {
		console.Print("  │  Running: %d", len(runningProcesses))
		for _, p := range runningProcesses {
			uptime := time.Since(p.StartTime)
			console.Print("  │    • %s (PID %d) - up %s", p.Name, p.PID, formatDuration(uptime))
		}
	} else {
		console.Print("  │  Running: 0")
	}
	stoppedCount := len(allProcesses) - len(runningProcesses)
	if stoppedCount > 0 {
		console.Print("  │  Stopped: %d", stoppedCount)
	}
	fmt.Println()

	// Logs section
	console.Print("  ┌─ Logs")
	if len(logs) > 0 {
		console.Print("  │  Available: %d log file(s)", len(logs))
		for _, log := range logs {
			size, _ := pm.GetLogSize(log)
			console.Print("  │    • %s (%s)", log, process.FormatBytes(size))
		}
	} else {
		console.Print("  │  No logs available")
	}
	fmt.Println()

	// Quick actions
	console.Print("  ┌─ Quick Actions")
	if !config.IsBuilt(projectRoot) {
		console.Print("  │  sbox build     Build the sandbox")
	} else {
		console.Print("  │  sbox run       Run the application")
		console.Print("  │  sbox run -d    Run as background daemon")
	}
	console.Print("  │  sbox shell     Interactive shell")
	console.Print("  │  sbox ps        List processes")
	console.Print("  │  sbox logs      View logs")
	fmt.Println()
}

func runPs(cmd *cobra.Command, args []string) {
	showAll, _ := cmd.Flags().GetBool("all")
	quiet, _ := cmd.Flags().GetBool("quiet")

	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	pm := process.NewProcessManager(projectRoot)

	var processes []process.ProcessInfo
	if showAll {
		processes, err = pm.UpdateProcessStatus()
	} else {
		processes, err = pm.GetRunningProcesses()
	}

	if err != nil {
		console.Fatal("Failed to get process list: %s", err)
	}

	if len(processes) == 0 {
		if !quiet {
			console.Info("No %s processes", func() string {
				if showAll {
					return ""
				}
				return "running "
			}())
		}
		return
	}

	if quiet {
		for _, p := range processes {
			fmt.Println(p.PID)
		}
		return
	}

	// Print table header
	fmt.Println()
	fmt.Printf("  %-8s %-15s %-10s %-12s %s\n", "PID", "NAME", "STATUS", "UPTIME", "COMMAND")
	fmt.Printf("  %-8s %-15s %-10s %-12s %s\n", "---", "----", "------", "------", "-------")

	for _, p := range processes {
		status := p.Status
		statusColor := ""
		switch status {
		case "running":
			statusColor = "\033[32m" // Green
		case "stopped":
			statusColor = "\033[33m" // Yellow
		case "crashed":
			statusColor = "\033[31m" // Red
		}

		uptime := "-"
		if p.Status == "running" {
			uptime = formatDuration(time.Since(p.StartTime))
		}

		// Truncate command if too long
		command := p.Command
		if len(command) > 40 {
			command = command[:37] + "..."
		}

		fmt.Printf("  %-8d %-15s %s%-10s\033[0m %-12s %s\n",
			p.PID, p.Name, statusColor, status, uptime, command)
	}
	fmt.Println()
}

func runLogs(cmd *cobra.Command, args []string) {
	follow, _ := cmd.Flags().GetBool("follow")
	lines, _ := cmd.Flags().GetInt("lines")
	listLogs, _ := cmd.Flags().GetBool("list")

	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	pm := process.NewProcessManager(projectRoot)

	if listLogs {
		logs, err := pm.ListLogs()
		if err != nil {
			console.Fatal("Failed to list logs: %s", err)
		}

		if len(logs) == 0 {
			console.Info("No log files found")
			return
		}

		console.Step("Available logs:")
		for _, log := range logs {
			size, _ := pm.GetLogSize(log)
			console.Print("  • %s (%s)", log, process.FormatBytes(size))
		}
		return
	}

	// Determine which log to show
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		// Default to project name
		name = filepath.Base(projectRoot)
	}

	if follow {
		console.Info("Following logs for '%s' (Ctrl+C to exit)...", name)
		fmt.Println()
	}

	if err := pm.ReadLogs(name, lines, follow); err != nil {
		console.Fatal("%s", err)
	}
}

func runStop(cmd *cobra.Command, args []string) {
	stopAll, _ := cmd.Flags().GetBool("all")

	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	pm := process.NewProcessManager(projectRoot)

	if stopAll {
		processes, err := pm.GetRunningProcesses()
		if err != nil {
			console.Fatal("Failed to get process list: %s", err)
		}

		if len(processes) == 0 {
			console.Info("No running processes to stop")
			return
		}

		console.Step("Stopping all processes...")
		for _, p := range processes {
			if err := pm.StopProcess(p.Name); err != nil {
				console.Error("Failed to stop %s: %s", p.Name, err)
			} else {
				console.Success("Stopped %s (PID %d)", p.Name, p.PID)
			}
		}
		return
	}

	// Stop specific process
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		name = filepath.Base(projectRoot)
	}

	console.Step("Stopping process: %s", name)

	if err := pm.StopProcess(name); err != nil {
		console.Fatal("%s", err)
	}

	console.Success("Process stopped")
}

func runRestart(cmd *cobra.Command, args []string) {
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	pm := process.NewProcessManager(projectRoot)

	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		name = filepath.Base(projectRoot)
	}

	// Get existing process info
	existing, err := pm.GetProcess(name)
	if err != nil {
		console.Fatal("Process '%s' not found", name)
	}

	command := existing.Command

	// Stop if running
	if existing.Status == "running" && process.IsProcessRunning(existing.PID) {
		console.Step("Stopping process: %s", name)
		if err := pm.StopProcess(name); err != nil {
			console.Warning("Failed to stop gracefully: %s", err)
		}
		// Wait a bit for process to fully stop
		time.Sleep(500 * time.Millisecond)
	}

	// Start again
	console.Step("Starting process: %s", name)

	r, err := runner.New(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	env := r.BuildEnv()
	workdir := r.ResolveWorkdir()

	info, err := pm.StartDaemon(name, command, env, workdir)
	if err != nil {
		console.Fatal("Failed to start: %s", err)
	}

	console.Success("Process restarted (PID %d)", info.PID)
}

func runClean(cmd *cobra.Command, args []string) {
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	cleanAll, _ := cmd.Flags().GetBool("all")
	cleanLogs, _ := cmd.Flags().GetBool("logs")
	logsAge, _ := cmd.Flags().GetDuration("logs-older-than")

	sboxDir := config.GetSboxDir(projectRoot)
	pm := process.NewProcessManager(projectRoot)

	// Stop running processes first
	runningProcesses, _ := pm.GetRunningProcesses()
	if len(runningProcesses) > 0 {
		console.Step("Stopping %d running process(es)...", len(runningProcesses))
		for _, p := range runningProcesses {
			pm.StopProcess(p.Name)
			console.Print("  Stopped: %s", p.Name)
		}
	}

	if cleanLogs {
		console.Step("Cleaning logs older than %s...", logsAge)
		removed, err := pm.CleanOldLogs(logsAge)
		if err != nil {
			console.Error("Failed to clean logs: %s", err)
		} else {
			console.Success("Removed %d log file(s)", removed)
		}
		return
	}

	if cleanAll {
		console.Step("Removing all sbox files...")
		os.RemoveAll(sboxDir)
		os.Remove(config.GetLockPath(projectRoot))
		console.Success("Cleaned all sbox files")
		console.Info("Run 'sbox init' to reinitialize the project")
	} else {
		console.Step("Cleaning build artifacts...")
		dirsToClean := []string{"env", "rootfs", "mamba", "bin"}
		for _, d := range dirsToClean {
			path := filepath.Join(sboxDir, d)
			if _, err := os.Stat(path); err == nil {
				os.RemoveAll(path)
				console.Print("  Removed: %s/", d)
			}
		}
		os.Remove(config.GetLockPath(projectRoot))
		os.Remove(filepath.Join(sboxDir, config.EnvScript))
		console.Success("Cleaned build artifacts")
		console.Info("Run 'sbox build' to rebuild")
	}
}

func runInfo(cmd *cobra.Command, args []string) {
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	cfg, err := config.Load(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	sboxDir := config.GetSboxDir(projectRoot)
	envDir := config.GetEnvDir(projectRoot)
	rootfsDir := config.GetRootfsDir(projectRoot)

	fmt.Println()
	console.Step("Environment Information")
	fmt.Println()

	console.Print("  ┌─ Paths")
	console.Print("  │  Project root:  %s", projectRoot)
	console.Print("  │  Sbox dir:      %s", sboxDir)
	console.Print("  │  Environment:   %s", envDir)
	console.Print("  │  Rootfs:        %s", rootfsDir)
	fmt.Println()

	console.Print("  ┌─ Runtime")
	runtimeInfo := cfg.ParseRuntime()
	console.Print("  │  Language:  %s", runtimeInfo.Language)
	console.Print("  │  Version:   %s", runtimeInfo.Version)

	// Check for runtime binary
	var binaryPath string
	if runtimeInfo.Language == "python" {
		binaryPath = filepath.Join(envDir, "bin", "python")
	} else if runtimeInfo.Language == "node" {
		binaryPath = filepath.Join(envDir, "bin", "node")
	}

	if binaryPath != "" {
		if info, err := os.Stat(binaryPath); err == nil {
			console.Print("  │  Binary:    %s (%s)", binaryPath, process.FormatBytes(info.Size()))
		} else {
			console.Print("  │  Binary:    Not installed")
		}
	}
	fmt.Println()

	// Directory sizes
	console.Print("  ┌─ Disk Usage")
	dirs := map[string]string{
		"Environment": envDir,
		"Rootfs":      rootfsDir,
		"Logs":        filepath.Join(sboxDir, "logs"),
	}
	for name, path := range dirs {
		size := getDirSize(path)
		console.Print("  │  %-12s %s", name+":", process.FormatBytes(size))
	}
	fmt.Println()

	// Environment variables
	if len(cfg.Env) > 0 {
		console.Print("  ┌─ Environment Variables")
		for key, value := range cfg.Env {
			// Mask sensitive values
			displayValue := value
			if strings.Contains(strings.ToLower(key), "secret") ||
				strings.Contains(strings.ToLower(key), "password") ||
				strings.Contains(strings.ToLower(key), "key") {
				displayValue = "********"
			}
			console.Print("  │  %s=%s", key, displayValue)
		}
		fmt.Println()
	}

	// Copy specs
	if len(cfg.Copy) > 0 {
		console.Print("  ┌─ File Mappings")
		for _, spec := range cfg.ParseCopy() {
			console.Print("  │  %s → %s", spec.Src, spec.Dst)
		}
		fmt.Println()
	}

	// Install commands
	if len(cfg.Install) > 0 {
		console.Print("  ┌─ Install Commands")
		for i, cmd := range cfg.Install {
			console.Print("  │  %d. %s", i+1, cmd)
		}
		fmt.Println()
	}
}

// Helper functions

func formatDuration(d time.Duration) string {
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

func getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// For status command - load lock file
func loadLockJSON(path string) map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}

func runValidate(cmd *cobra.Command, args []string) {
	quiet, _ := cmd.Flags().GetBool("quiet")
	fix, _ := cmd.Flags().GetBool("fix")

	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project. Run 'sbox init <name>' first.")
	}

	configPath := filepath.Join(config.GetSboxDir(projectRoot), config.ConfigFile)

	fmt.Println()
	console.Step("Validating configuration: %s", configPath)
	fmt.Println()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		console.Fatal("Config file not found: %s\n  → Run 'sbox init <name>' to create a new project", configPath)
	}

	// Load config
	cfg, err := config.Load(projectRoot)
	if err != nil {
		console.Error("Failed to parse config file:")
		console.Print("  %s", err)
		fmt.Println()
		console.Print("Common causes:")
		console.Print("  • Invalid YAML syntax (check indentation)")
		console.Print("  • Missing colons after field names")
		console.Print("  • Unquoted special characters")
		fmt.Println()
		console.Print("Example valid config:")
		fmt.Println()
		console.Print(validate.GetConfigExample("python"))
		os.Exit(1)
	}

	// Validate
	result := validate.ValidateConfig(cfg, projectRoot)

	// Display results
	if len(result.Errors) > 0 {
		console.Error("Configuration errors (%d):", len(result.Errors))
		fmt.Println()
		for i, verr := range result.Errors {
			console.Print("  %d. [%s] %s", i+1, verr.Field, verr.Message)
			if verr.Hint != "" {
				console.Print("     → %s", verr.Hint)
			}
			fmt.Println()
		}
	}

	if !quiet && len(result.Warnings) > 0 {
		console.Warning("Configuration warnings (%d):", len(result.Warnings))
		fmt.Println()
		for i, warn := range result.Warnings {
			console.Print("  %d. [%s] %s", i+1, warn.Field, warn.Message)
			if warn.Hint != "" {
				console.Print("     → %s", warn.Hint)
			}
			fmt.Println()
		}
	}

	// Summary
	fmt.Println()
	if result.Valid {
		if len(result.Warnings) > 0 {
			console.Success("Configuration is valid with %d warning(s)", len(result.Warnings))
		} else {
			console.Success("Configuration is valid")
		}
		fmt.Println()
		console.Print("  You can now run:")
		console.Print("    sbox build     Build the sandbox")
		console.Print("    sbox run       Run the application")
	} else {
		console.Error("Configuration is invalid with %d error(s)", len(result.Errors))
		fmt.Println()
		console.Print("  Please fix the errors above and run 'sbox validate' again.")

		if fix {
			fmt.Println()
			console.Info("Auto-fix is not yet implemented.")
			console.Print("  Please manually edit: %s", configPath)
		}

		os.Exit(1)
	}
	fmt.Println()

	// Show config summary
	console.Print("  ┌─ Config Summary")
	console.Print("  │  Runtime:  %s", cfg.Runtime)
	console.Print("  │  Workdir:  %s", cfg.Workdir)
	console.Print("  │  Command:  %s", cfg.Cmd)
	console.Print("  │  Copy:     %d mapping(s)", len(cfg.Copy))
	console.Print("  │  Install:  %d command(s)", len(cfg.Install))
	console.Print("  │  Env vars: %d defined", len(cfg.Env))
	fmt.Println()
}
