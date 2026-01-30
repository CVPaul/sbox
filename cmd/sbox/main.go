// sbox - A rootless, user-space sandbox runtime.
// Docker-like workflow without sudo.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sbox-project/sbox/internal/builder"
	"github.com/sbox-project/sbox/internal/config"
	"github.com/sbox-project/sbox/internal/console"
	"github.com/sbox-project/sbox/internal/runner"
)

const version = "0.1.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "sbox",
		Short: "A rootless, user-space sandbox runtime",
		Long:  "sbox - Docker-like workflow without sudo.\nA rootless, user-space sandbox runtime for Python applications.",
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
	var initRuntime string
	var initForce bool
	initCmd.Flags().StringVarP(&initRuntime, "runtime", "r", "python:3.10", "Runtime to use")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing project")
	rootCmd.AddCommand(initCmd)

	// Build command
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build the sandbox environment",
		Run:   runBuild,
	}
	var buildForce bool
	buildCmd.Flags().BoolVarP(&buildForce, "force", "f", false, "Force rebuild even if up to date")
	rootCmd.AddCommand(buildCmd)

	// Run command
	runCmd := &cobra.Command{
		Use:   "run [command]",
		Short: "Run the application in the sandbox",
		Run:   runRun,
	}
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

	// Status command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show project status",
		Run:   runStatus,
	})

	// Clean command
	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean the sandbox environment",
		Run:   runClean,
	}
	var cleanAll bool
	cleanCmd.Flags().BoolVarP(&cleanAll, "all", "a", false, "Remove everything including config")
	rootCmd.AddCommand(cleanCmd)

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

	// Create project structure
	sboxDir := filepath.Join(projectPath, config.SboxDir)
	dirs := []string{
		sboxDir,
		filepath.Join(sboxDir, "env"),
		filepath.Join(sboxDir, "rootfs"),
		filepath.Join(sboxDir, "bin"),
		filepath.Join(projectPath, "app"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			console.Fatal("Failed to create directory: %s", err)
		}
	}

	// Create default main.py
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

	// Create config
	cfg := config.NewDefaultConfig(runtimeStr)
	if err := cfg.Save(projectPath); err != nil {
		console.Fatal("Failed to create config: %s", err)
	}

	// Create .gitignore
	gitignore := `.sbox/env/
.sbox/rootfs/
.sbox/bin/
.sbox/mamba/
sbox.lock
__pycache__/
*.pyc
.env
`
	if err := os.WriteFile(filepath.Join(projectPath, ".gitignore"), []byte(gitignore), 0644); err != nil {
		console.Fatal("Failed to create .gitignore: %s", err)
	}

	console.Success("Created project structure:")
	console.Print("  %s/", projectName)
	console.Print("  ├── .sbox/")
	console.Print("  │   └── config.yaml")
	console.Print("  ├── app/")
	console.Print("  │   ├── main.py")
	console.Print("  │   └── requirements.txt")
	console.Print("  └── .gitignore")
	console.Print("")
	console.Print("Next steps:")
	console.Print("  cd %s", projectName)
	console.Print("  sbox build")
	console.Print("  sbox run")
}

func runBuild(cmd *cobra.Command, args []string) {
	force, _ := cmd.Flags().GetBool("force")

	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project. Run 'sbox init <name>' first.")
	}

	console.Step("Building sandbox: %s", filepath.Base(projectRoot))

	b, err := builder.New(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	if err := b.Build(force); err != nil {
		console.Fatal("Build failed: %s", err)
	}
}

func runRun(cmd *cobra.Command, args []string) {
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	r, err := runner.New(projectRoot)
	if err != nil {
		console.Fatal("Failed to load config: %s", err)
	}

	var command string
	if len(args) > 0 {
		command = args[0]
	}

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
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Warning("Not in an sbox project")
		return
	}

	console.Step("sbox project: %s", filepath.Base(projectRoot))

	cfg, err := config.Load(projectRoot)
	if err != nil {
		console.Error("Config error: %s", err)
		return
	}

	configPath := filepath.Join(config.GetSboxDir(projectRoot), config.ConfigFile)
	console.Success("Config: %s", configPath)
	console.Print("  Runtime: %s", cfg.Runtime)
	console.Print("  Workdir: %s", cfg.Workdir)
	console.Print("  Command: %s", cfg.Cmd)

	if config.IsBuilt(projectRoot) {
		console.Success("Build status: Built")
		if lock, err := config.LoadLock(projectRoot); err == nil {
			console.Print("  Built at: %s", lock.BuiltAt)
		}
	} else {
		console.Warning("Build status: Not built")
	}
}

func runClean(cmd *cobra.Command, args []string) {
	projectRoot, err := config.GetProjectRoot("")
	if err != nil {
		console.Fatal("Not in an sbox project.")
	}

	cleanAll, _ := cmd.Flags().GetBool("all")
	sboxDir := config.GetSboxDir(projectRoot)

	if cleanAll {
		os.RemoveAll(sboxDir)
		os.Remove(config.GetLockPath(projectRoot))
		console.Success("Cleaned all sbox files")
	} else {
		dirsToClean := []string{"env", "rootfs", "mamba", "bin"}
		for _, d := range dirsToClean {
			os.RemoveAll(filepath.Join(sboxDir, d))
		}
		os.Remove(config.GetLockPath(projectRoot))
		os.Remove(filepath.Join(sboxDir, config.EnvScript))
		console.Success("Cleaned build artifacts")
	}
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
