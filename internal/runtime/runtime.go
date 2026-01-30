// Package runtime handles runtime environment setup.
package runtime

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sbox-project/sbox/internal/config"
	"github.com/sbox-project/sbox/internal/console"
)

// Manager handles runtime environment setup
type Manager struct {
	ProjectRoot string
	SboxDir     string
	EnvDir      string
	MambaRoot   string
}

// NewManager creates a new runtime manager
func NewManager(projectRoot string) *Manager {
	sboxDir := config.GetSboxDir(projectRoot)
	return &Manager{
		ProjectRoot: projectRoot,
		SboxDir:     sboxDir,
		EnvDir:      config.GetEnvDir(projectRoot),
		MambaRoot:   filepath.Join(sboxDir, "mamba"),
	}
}

// Setup sets up the runtime environment
func (m *Manager) Setup(info config.RuntimeInfo) error {
	switch info.Language {
	case "python":
		return m.setupPython(info.Version)
	case "node", "nodejs":
		return m.setupNode(info.Version)
	default:
		return fmt.Errorf("unsupported runtime: %s (supported: python, node)", info.Language)
	}
}

func (m *Manager) setupPython(version string) error {
	console.Step("Setting up Python %s environment...", version)

	// Ensure micromamba is available
	mambaPath, err := m.ensureMicromamba()
	if err != nil {
		return fmt.Errorf("failed to setup micromamba: %w", err)
	}

	// Create mamba root directory
	if err := os.MkdirAll(m.MambaRoot, 0755); err != nil {
		return err
	}

	// Check if environment already exists
	if m.pythonEnvExists() {
		currentVersion := m.getPythonVersion()
		if strings.HasPrefix(currentVersion, version) {
			console.Success("Python %s already installed", currentVersion)
			return nil
		}
		console.Warning("Version mismatch, recreating environment...")
		if err := m.removeEnv(); err != nil {
			return err
		}
	}

	console.Step("Creating Python %s environment with micromamba...", version)

	// Create environment with Python
	cmd := exec.Command(mambaPath,
		"create",
		"-p", m.EnvDir,
		"-c", "conda-forge",
		fmt.Sprintf("python=%s", version),
		"pip",
		"--yes",
		"--quiet",
	)

	cmd.Env = append(os.Environ(), fmt.Sprintf("MAMBA_ROOT_PREFIX=%s", m.MambaRoot))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	console.Success("Python %s environment created", version)
	return nil
}

func (m *Manager) setupNode(version string) error {
	console.Step("Setting up Node.js %s environment...", version)

	// Ensure micromamba is available
	mambaPath, err := m.ensureMicromamba()
	if err != nil {
		return fmt.Errorf("failed to setup micromamba: %w", err)
	}

	// Create mamba root directory
	if err := os.MkdirAll(m.MambaRoot, 0755); err != nil {
		return err
	}

	// Check if environment already exists
	if m.nodeEnvExists() {
		currentVersion := m.getNodeVersion()
		if strings.HasPrefix(currentVersion, version) || strings.HasPrefix(currentVersion, "v"+version) {
			console.Success("Node.js %s already installed", currentVersion)
			return nil
		}
		console.Warning("Version mismatch (have %s, want %s), recreating environment...", currentVersion, version)
		if err := m.removeEnv(); err != nil {
			return err
		}
	}

	console.Step("Creating Node.js %s environment with micromamba...", version)

	// Create environment with Node.js and pnpm
	cmd := exec.Command(mambaPath,
		"create",
		"-p", m.EnvDir,
		"-c", "conda-forge",
		fmt.Sprintf("nodejs=%s", version),
		"pnpm",
		"--yes",
		"--quiet",
	)

	cmd.Env = append(os.Environ(), fmt.Sprintf("MAMBA_ROOT_PREFIX=%s", m.MambaRoot))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	console.Success("Node.js %s environment created", version)
	return nil
}

func (m *Manager) ensureMicromamba() (string, error) {
	mambaPath := config.GetMicromambaPath(m.ProjectRoot)

	if _, err := os.Stat(mambaPath); err == nil {
		return mambaPath, nil
	}

	console.Step("Downloading micromamba...")

	url, err := config.GetMicromambaURL()
	if err != nil {
		return "", err
	}

	// Create bin directory
	binDir := filepath.Dir(mambaPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", err
	}

	// Download archive
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download micromamba: %w", err)
	}
	defer resp.Body.Close()

	// Create temp file for archive
	tmpFile, err := os.CreateTemp("", "micromamba-*.tar.bz2")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// Create temp directory for extraction
	tmpDir, err := os.MkdirTemp("", "micromamba-extract-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// Extract archive
	cmd := exec.Command("tar", "-xjf", tmpFile.Name(), "-C", tmpDir)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to extract micromamba: %w", err)
	}

	// Find the extracted binary
	extractedPath := filepath.Join(tmpDir, "bin", "micromamba")
	if _, err := os.Stat(extractedPath); err != nil {
		// Try to find it
		err := filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Name() == "micromamba" && !info.IsDir() {
				extractedPath = path
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil && err != filepath.SkipAll {
			return "", fmt.Errorf("failed to find micromamba in archive")
		}
	}

	// Copy to destination
	srcFile, err := os.Open(extractedPath)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(mambaPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", err
	}

	console.Success("micromamba downloaded")
	return mambaPath, nil
}

func (m *Manager) pythonEnvExists() bool {
	pythonPath := filepath.Join(m.EnvDir, "bin", "python")
	_, err := os.Stat(pythonPath)
	return err == nil
}

func (m *Manager) nodeEnvExists() bool {
	nodePath := filepath.Join(m.EnvDir, "bin", "node")
	_, err := os.Stat(nodePath)
	return err == nil
}

func (m *Manager) getPythonVersion() string {
	pythonPath := filepath.Join(m.EnvDir, "bin", "python")
	cmd := exec.Command(pythonPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	// Output is like "Python 3.10.12"
	parts := strings.Fields(string(output))
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func (m *Manager) getNodeVersion() string {
	nodePath := filepath.Join(m.EnvDir, "bin", "node")
	cmd := exec.Command(nodePath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	// Output is like "v22.12.0"
	return strings.TrimSpace(string(output))
}

func (m *Manager) removeEnv() error {
	return os.RemoveAll(m.EnvDir)
}

// GetPythonPath returns the path to Python interpreter
func (m *Manager) GetPythonPath() string {
	return filepath.Join(m.EnvDir, "bin", "python")
}

// GetPipPath returns the path to pip
func (m *Manager) GetPipPath() string {
	return filepath.Join(m.EnvDir, "bin", "pip")
}

// GetNodePath returns the path to Node.js
func (m *Manager) GetNodePath() string {
	return filepath.Join(m.EnvDir, "bin", "node")
}

// GetNpmPath returns the path to npm
func (m *Manager) GetNpmPath() string {
	return filepath.Join(m.EnvDir, "bin", "npm")
}

// GetPnpmPath returns the path to pnpm
func (m *Manager) GetPnpmPath() string {
	return filepath.Join(m.EnvDir, "bin", "pnpm")
}

// InstallPackages runs install commands in the environment
func (m *Manager) InstallPackages(commands []string) error {
	if len(commands) == 0 {
		return nil
	}

	console.Step("Installing packages...")

	env := m.buildEnv()

	for _, cmdStr := range commands {
		console.Info("Running: %s", cmdStr)

		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Dir = m.ProjectRoot
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install command failed: %s: %w", cmdStr, err)
		}
	}

	console.Success("Package installation complete")
	return nil
}

func (m *Manager) buildEnv() []string {
	path := fmt.Sprintf("PATH=%s/bin:%s", m.EnvDir, os.Getenv("PATH"))

	env := []string{
		path,
		"PYTHONNOUSERSITE=1",
		"PYTHONDONTWRITEBYTECODE=1",
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
		fmt.Sprintf("CONDA_PREFIX=%s", m.EnvDir),
		fmt.Sprintf("MAMBA_ROOT_PREFIX=%s", m.MambaRoot),
		// Node.js specific
		fmt.Sprintf("npm_config_prefix=%s", m.EnvDir),
	}

	// Add essential system vars
	for _, key := range []string{"LANG", "TERM", "USER", "HOME", "TMPDIR"} {
		if val := os.Getenv(key); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	return env
}
