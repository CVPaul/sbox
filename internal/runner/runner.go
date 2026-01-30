// Package runner handles sandbox command execution.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sbox-project/sbox/internal/config"
	"github.com/sbox-project/sbox/internal/console"
)

// Runner executes commands in the sandbox environment
type Runner struct {
	ProjectRoot string
	Config      *config.Config
	EnvDir      string
	Rootfs      string
	SboxDir     string
}

// New creates a new runner
func New(projectRoot string) (*Runner, error) {
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return nil, err
	}

	return &Runner{
		ProjectRoot: projectRoot,
		Config:      cfg,
		EnvDir:      config.GetEnvDir(projectRoot),
		Rootfs:      config.GetRootfsDir(projectRoot),
		SboxDir:     config.GetSboxDir(projectRoot),
	}, nil
}

// Run executes the command in the sandbox
func (r *Runner) Run(cmd string) (int, error) {
	if !config.IsBuilt(r.ProjectRoot) {
		return 1, fmt.Errorf("sandbox not built. Run 'sbox build' first")
	}

	command := cmd
	if command == "" {
		command = r.Config.Cmd
	}
	if command == "" {
		return 1, fmt.Errorf("no command specified and no default cmd in config")
	}

	workdir := r.ResolveWorkdir()
	env := r.BuildEnv()

	console.Step("Running: %s", command)
	console.Info("Workdir: %s", workdir)
	fmt.Println()

	execCmd := exec.Command("sh", "-c", command)
	execCmd.Dir = workdir
	execCmd.Env = env
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	err := execCmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}

	return 0, nil
}

// Shell starts an interactive shell in the sandbox
func (r *Runner) Shell() (int, error) {
	if !config.IsBuilt(r.ProjectRoot) {
		return 1, fmt.Errorf("sandbox not built. Run 'sbox build' first")
	}

	workdir := r.ResolveWorkdir()
	env := r.BuildEnv()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	console.Step("Starting shell in sandbox...")
	console.Info("Workdir: %s", workdir)
	console.Info("Type 'exit' to leave the sandbox")
	fmt.Println()

	execCmd := exec.Command(shell)
	execCmd.Dir = workdir
	execCmd.Env = env
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	err := execCmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}

	return 0, nil
}

// Exec executes a command with arguments in the sandbox
func (r *Runner) Exec(args []string) (int, error) {
	if !config.IsBuilt(r.ProjectRoot) {
		return 1, fmt.Errorf("sandbox not built. Run 'sbox build' first")
	}

	if len(args) == 0 {
		return 1, fmt.Errorf("no command provided")
	}

	workdir := r.ResolveWorkdir()
	env := r.BuildEnv()

	execCmd := exec.Command(args[0], args[1:]...)
	execCmd.Dir = workdir
	execCmd.Env = env
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	err := execCmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}

	return 0, nil
}

// ResolveWorkdir returns the resolved working directory path
func (r *Runner) ResolveWorkdir() string {
	workdirConfig := r.Config.Workdir

	var resolved string
	if strings.HasPrefix(workdirConfig, "/") {
		resolved = filepath.Join(r.Rootfs, strings.TrimPrefix(workdirConfig, "/"))
	} else {
		resolved = filepath.Join(r.ProjectRoot, workdirConfig)
	}

	if _, err := os.Stat(resolved); err != nil {
		// Fallback to app directory in rootfs
		appDir := filepath.Join(r.Rootfs, "app")
		if _, err := os.Stat(appDir); err == nil {
			return appDir
		}
		return r.ProjectRoot
	}

	return resolved
}

// BuildEnv returns the environment variables for the sandbox
func (r *Runner) BuildEnv() []string {
	var env []string

	// Essential system vars
	essentialVars := []string{"LANG", "TERM", "USER", "LOGNAME", "DISPLAY", "SSH_AUTH_SOCK"}
	for _, key := range essentialVars {
		if val := os.Getenv(key); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// Defaults
	if os.Getenv("LANG") == "" {
		env = append(env, "LANG=en_US.UTF-8")
	}
	if os.Getenv("TERM") == "" {
		env = append(env, "TERM=xterm-256color")
	}

	// Sandbox identification
	env = append(env, "SBOX_ACTIVE=1")
	env = append(env, fmt.Sprintf("SBOX_PROJECT=%s", r.ProjectRoot))

	// Python isolation
	env = append(env, "PYTHONNOUSERSITE=1")
	env = append(env, "PYTHONDONTWRITEBYTECODE=1")
	env = append(env, "PIP_DISABLE_PIP_VERSION_CHECK=1")

	// Paths - isolated
	env = append(env, fmt.Sprintf("PATH=%s/bin:/usr/bin:/bin:/usr/sbin:/sbin", r.EnvDir))
	env = append(env, fmt.Sprintf("HOME=%s/home", r.Rootfs))
	env = append(env, fmt.Sprintf("TMPDIR=%s/tmp", r.Rootfs))

	// Conda/mamba vars
	env = append(env, fmt.Sprintf("CONDA_PREFIX=%s", r.EnvDir))
	env = append(env, fmt.Sprintf("MAMBA_ROOT_PREFIX=%s/mamba", r.SboxDir))

	// Custom environment variables from config
	for key, value := range r.Config.Env {
		expanded := os.ExpandEnv(value)
		env = append(env, fmt.Sprintf("%s=%s", key, expanded))
	}

	return env
}
