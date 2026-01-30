// Package config handles sbox configuration parsing and management.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Constants
const (
	SboxDir       = ".sbox"
	ConfigFile    = "config.yaml"
	LockFile      = "sbox.lock"
	EnvDir        = "env"
	RootfsDir     = "rootfs"
	EnvScript     = "env.sh"
	GlobalCacheName = "cache"
)

// Config represents the sandbox configuration
type Config struct {
	Runtime string            `yaml:"runtime"`
	Workdir string            `yaml:"workdir"`
	Copy    []string          `yaml:"copy"`
	Mount   []string          `yaml:"mount"`
	Install []string          `yaml:"install"`
	Cmd     string            `yaml:"cmd"`
	Env     map[string]string `yaml:"env"`
}

// CopySpec represents a parsed copy specification
type CopySpec struct {
	Src string
	Dst string
}

// MountSpec represents a parsed mount specification
type MountSpec struct {
	Src      string // Host path
	Dst      string // Container path
	ReadOnly bool   // Whether mount is read-only
}

// RuntimeInfo contains parsed runtime information
type RuntimeInfo struct {
	Language string
	Version  string
}

// LockData represents the lock file content
type LockData struct {
	Version    string `json:"version"`
	ConfigHash string `json:"config_hash"`
	BuiltAt    string `json:"built_at"`
	Runtime    string `json:"runtime"`
}

// MicromambaURLs maps platform to download URL
var MicromambaURLs = map[string]string{
	"darwin-arm64":  "https://micro.mamba.pm/api/micromamba/osx-arm64/latest",
	"darwin-amd64":  "https://micro.mamba.pm/api/micromamba/osx-64/latest",
	"linux-amd64":   "https://micro.mamba.pm/api/micromamba/linux-64/latest",
	"linux-arm64":   "https://micro.mamba.pm/api/micromamba/linux-aarch64/latest",
}

// NewDefaultConfig creates a new default configuration
func NewDefaultConfig(runtimeStr string) *Config {
	if runtimeStr == "" {
		runtimeStr = "python:3.10"
	}
	return &Config{
		Runtime: runtimeStr,
		Workdir: "/app",
		Copy:    []string{"./app:/app"},
		Install: []string{"pip install -r app/requirements.txt"},
		Cmd:     "python main.py",
		Env:     make(map[string]string),
	}
}

// Load loads configuration from a project root
func Load(projectRoot string) (*Config, error) {
	configPath := filepath.Join(projectRoot, SboxDir, ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.Runtime == "" {
		cfg.Runtime = "python:3.10"
	}
	if cfg.Workdir == "" {
		cfg.Workdir = "/app"
	}
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
	}

	return &cfg, nil
}

// Save saves configuration to a project root
func (c *Config) Save(projectRoot string) error {
	configPath := filepath.Join(projectRoot, SboxDir, ConfigFile)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// ParseCopy parses copy specifications
func (c *Config) ParseCopy() []CopySpec {
	var specs []CopySpec
	for _, item := range c.Copy {
		parts := strings.SplitN(item, ":", 2)
		if len(parts) == 2 {
			specs = append(specs, CopySpec{Src: parts[0], Dst: parts[1]})
		} else {
			specs = append(specs, CopySpec{Src: item, Dst: item})
		}
	}
	return specs
}

// ParseMount parses mount specifications
// Format: /host/path:/container/path or /host/path:/container/path:ro
func (c *Config) ParseMount() []MountSpec {
	var specs []MountSpec
	for _, item := range c.Mount {
		parts := strings.Split(item, ":")
		if len(parts) < 2 {
			// Invalid format, skip
			continue
		}
		
		spec := MountSpec{
			Src:      parts[0],
			Dst:      parts[1],
			ReadOnly: false,
		}
		
		// Check for read-only flag
		if len(parts) >= 3 && (parts[2] == "ro" || parts[2] == "readonly") {
			spec.ReadOnly = true
		}
		
		specs = append(specs, spec)
	}
	return specs
}

// ParseRuntime parses the runtime string
func (c *Config) ParseRuntime() RuntimeInfo {
	parts := strings.SplitN(c.Runtime, ":", 2)
	info := RuntimeInfo{Language: "python", Version: "3.10"}
	if len(parts) >= 1 {
		info.Language = strings.ToLower(parts[0])
	}
	if len(parts) >= 2 {
		info.Version = parts[1]
	}
	return info
}

// Hash computes a hash of the configuration
func (c *Config) Hash() string {
	data, _ := json.Marshal(c)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])[:16]
}

// GetProjectRoot finds the project root by looking for .sbox directory
func GetProjectRoot(startPath string) (string, error) {
	if startPath == "" {
		var err error
		startPath, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	path, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	for {
		sboxPath := filepath.Join(path, SboxDir)
		if info, err := os.Stat(sboxPath); err == nil && info.IsDir() {
			return path, nil
		}

		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}

	return "", fmt.Errorf("not in an sbox project (no %s directory found)", SboxDir)
}

// GetSboxDir returns the .sbox directory path
func GetSboxDir(projectRoot string) string {
	return filepath.Join(projectRoot, SboxDir)
}

// GetEnvDir returns the environment directory path
func GetEnvDir(projectRoot string) string {
	return filepath.Join(projectRoot, SboxDir, EnvDir)
}

// GetRootfsDir returns the rootfs directory path
func GetRootfsDir(projectRoot string) string {
	return filepath.Join(projectRoot, SboxDir, RootfsDir)
}

// GetMicromambaPath returns the micromamba binary path
func GetMicromambaPath(projectRoot string) string {
	return filepath.Join(projectRoot, SboxDir, "bin", "micromamba")
}

// GetGlobalSboxDir returns the global sbox directory (~/.sbox)
func GetGlobalSboxDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, SboxDir), nil
}

// GetGlobalCacheDir returns the global cache directory (~/.sbox/cache)
func GetGlobalCacheDir() (string, error) {
	globalDir, err := GetGlobalSboxDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(globalDir, GlobalCacheName), nil
}

// GetGlobalMicromambaPath returns the path to the globally cached micromamba binary
func GetGlobalMicromambaPath() (string, error) {
	cacheDir, err := GetGlobalCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "bin", "micromamba"), nil
}

// GetGlobalPkgsCacheDir returns the path to the shared package cache
func GetGlobalPkgsCacheDir() (string, error) {
	cacheDir, err := GetGlobalCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "pkgs"), nil
}

// GetLockPath returns the lock file path
func GetLockPath(projectRoot string) string {
	return filepath.Join(projectRoot, LockFile)
}

// GetPlatformKey returns the platform key for micromamba download
func GetPlatformKey() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	return fmt.Sprintf("%s-%s", os, arch)
}

// GetMicromambaURL returns the download URL for current platform
func GetMicromambaURL() (string, error) {
	key := GetPlatformKey()
	url, ok := MicromambaURLs[key]
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s", key)
	}
	return url, nil
}

// LoadLock loads the lock file
func LoadLock(projectRoot string) (*LockData, error) {
	lockPath := GetLockPath(projectRoot)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}

	var lock LockData
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}

// SaveLock saves the lock file
func SaveLock(projectRoot string, cfg *Config) error {
	lock := LockData{
		Version:    "0.1.0",
		ConfigHash: cfg.Hash(),
		BuiltAt:    time.Now().Format(time.RFC3339),
		Runtime:    cfg.Runtime,
	}

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(GetLockPath(projectRoot), data, 0644)
}

// IsBuilt checks if the project has been built
func IsBuilt(projectRoot string) bool {
	lockPath := GetLockPath(projectRoot)
	envDir := GetEnvDir(projectRoot)
	pythonPath := filepath.Join(envDir, "bin", "python")
	nodePath := filepath.Join(envDir, "bin", "node")

	if _, err := os.Stat(lockPath); err != nil {
		return false
	}
	// Check if either Python or Node environment exists
	pythonExists := false
	nodeExists := false
	if _, err := os.Stat(pythonPath); err == nil {
		pythonExists = true
	}
	if _, err := os.Stat(nodePath); err == nil {
		nodeExists = true
	}
	return pythonExists || nodeExists
}

// IsUpToDate checks if the build is up to date
func IsUpToDate(projectRoot string, cfg *Config) bool {
	lock, err := LoadLock(projectRoot)
	if err != nil {
		return false
	}
	return lock.ConfigHash == cfg.Hash()
}
