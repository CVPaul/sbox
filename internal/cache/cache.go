// Package cache provides global cache management for sbox runtimes.
package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Constants for cache structure
const (
	CacheDir    = ".sbox"
	CacheName   = "cache"
	RuntimesDir = "runtimes"
	PkgsDir     = "pkgs"
	BinDir      = "bin"
)

// CachedRuntime represents metadata for a cached runtime environment
type CachedRuntime struct {
	Language    string    `json:"language"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	Size        int64     `json:"size"`
	Path        string    `json:"path"`
}

// CacheInfo contains information about the cache
type CacheInfo struct {
	Path         string           `json:"path"`
	TotalSize    int64            `json:"total_size"`
	RuntimeCount int              `json:"runtime_count"`
	Runtimes     []CachedRuntime  `json:"runtimes"`
}

// Manager handles global cache operations
type Manager struct {
	CacheRoot string
}

// NewManager creates a new cache manager
func NewManager() (*Manager, error) {
	cacheRoot, err := GetGlobalCacheDir()
	if err != nil {
		return nil, err
	}
	return &Manager{CacheRoot: cacheRoot}, nil
}

// GetGlobalCacheDir returns the global cache directory path (~/.sbox/cache)
func GetGlobalCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, CacheDir, CacheName), nil
}

// GetGlobalSboxDir returns the global sbox directory path (~/.sbox)
func GetGlobalSboxDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, CacheDir), nil
}

// GetRuntimesDir returns the path to cached runtimes
func (m *Manager) GetRuntimesDir() string {
	return filepath.Join(m.CacheRoot, RuntimesDir)
}

// GetPkgsDir returns the path to shared package cache
func (m *Manager) GetPkgsDir() string {
	return filepath.Join(m.CacheRoot, PkgsDir)
}

// GetBinDir returns the path to shared binaries (micromamba)
func (m *Manager) GetBinDir() string {
	return filepath.Join(m.CacheRoot, BinDir)
}

// GetMicromambaPath returns the path to the cached micromamba binary
func (m *Manager) GetMicromambaPath() string {
	return filepath.Join(m.GetBinDir(), "micromamba")
}

// GetRuntimeKey generates a unique key for a runtime
func GetRuntimeKey(language, version string) string {
	return fmt.Sprintf("%s-%s", language, version)
}

// GetCachedRuntimePath returns the path to a cached runtime
func (m *Manager) GetCachedRuntimePath(language, version string) string {
	key := GetRuntimeKey(language, version)
	return filepath.Join(m.GetRuntimesDir(), key)
}

// GetCachedRuntime checks if a runtime is cached and returns its info
func (m *Manager) GetCachedRuntime(language, version string) (*CachedRuntime, error) {
	runtimePath := m.GetCachedRuntimePath(language, version)
	
	// Check if the runtime directory exists
	info, err := os.Stat(runtimePath)
	if os.IsNotExist(err) {
		return nil, nil // Not cached
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("runtime path is not a directory")
	}

	// Check for marker file or actual binaries
	var hasRuntime bool
	if language == "python" {
		pythonPath := filepath.Join(runtimePath, "bin", "python")
		if _, err := os.Stat(pythonPath); err == nil {
			hasRuntime = true
		}
	} else if language == "node" || language == "nodejs" {
		nodePath := filepath.Join(runtimePath, "bin", "node")
		if _, err := os.Stat(nodePath); err == nil {
			hasRuntime = true
		}
	}

	if !hasRuntime {
		return nil, nil // Directory exists but runtime not properly installed
	}

	// Load metadata if exists
	metaPath := filepath.Join(runtimePath, ".sbox-cache.json")
	runtime := &CachedRuntime{
		Language:  language,
		Version:   version,
		Path:      runtimePath,
		CreatedAt: info.ModTime(),
		LastUsed:  info.ModTime(),
	}

	if metaData, err := os.ReadFile(metaPath); err == nil {
		json.Unmarshal(metaData, runtime)
	}

	// Calculate size
	runtime.Size = getDirSize(runtimePath)

	return runtime, nil
}

// IsMicromambaCached checks if micromamba binary is cached
func (m *Manager) IsMicromambaCached() bool {
	mambaPath := m.GetMicromambaPath()
	info, err := os.Stat(mambaPath)
	if err != nil {
		return false
	}
	// Check if it's executable
	return info.Mode()&0111 != 0
}

// EnsureCacheDirs creates the cache directory structure
func (m *Manager) EnsureCacheDirs() error {
	dirs := []string{
		m.CacheRoot,
		m.GetRuntimesDir(),
		m.GetPkgsDir(),
		m.GetBinDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create cache directory %s: %w", dir, err)
		}
	}
	return nil
}

// SaveRuntimeMetadata saves metadata for a cached runtime
func (m *Manager) SaveRuntimeMetadata(language, version string) error {
	runtimePath := m.GetCachedRuntimePath(language, version)
	metaPath := filepath.Join(runtimePath, ".sbox-cache.json")

	meta := CachedRuntime{
		Language:  language,
		Version:   version,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Path:      runtimePath,
		Size:      getDirSize(runtimePath),
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}

// UpdateLastUsed updates the last used timestamp for a runtime
func (m *Manager) UpdateLastUsed(language, version string) error {
	runtimePath := m.GetCachedRuntimePath(language, version)
	metaPath := filepath.Join(runtimePath, ".sbox-cache.json")

	meta := &CachedRuntime{}
	if data, err := os.ReadFile(metaPath); err == nil {
		json.Unmarshal(data, meta)
	}

	meta.Language = language
	meta.Version = version
	meta.Path = runtimePath
	meta.LastUsed = time.Now()
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}
	meta.Size = getDirSize(runtimePath)

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}

// CopyFromCache copies a cached runtime to a project directory
func (m *Manager) CopyFromCache(language, version, targetDir string) error {
	sourcePath := m.GetCachedRuntimePath(language, version)

	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("runtime %s-%s not found in cache", language, version)
	}

	// Remove target if exists
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("failed to remove existing target: %w", err)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return err
	}

	// Copy directory recursively
	if err := copyDir(sourcePath, targetDir); err != nil {
		return fmt.Errorf("failed to copy from cache: %w", err)
	}

	// Update last used timestamp
	m.UpdateLastUsed(language, version)

	return nil
}

// CopyToCache copies a project runtime to the global cache
func (m *Manager) CopyToCache(language, version, sourceDir string) error {
	if err := m.EnsureCacheDirs(); err != nil {
		return err
	}

	targetPath := m.GetCachedRuntimePath(language, version)

	// Remove existing cache if present
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("failed to remove existing cache: %w", err)
	}

	// Copy directory recursively
	if err := copyDir(sourceDir, targetPath); err != nil {
		return fmt.Errorf("failed to copy to cache: %w", err)
	}

	// Save metadata
	return m.SaveRuntimeMetadata(language, version)
}

// ListCachedRuntimes returns all cached runtimes
func (m *Manager) ListCachedRuntimes() ([]CachedRuntime, error) {
	runtimesDir := m.GetRuntimesDir()

	entries, err := os.ReadDir(runtimesDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var runtimes []CachedRuntime
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse runtime key (language-version)
		name := entry.Name()
		language := ""
		version := ""
		
		// Handle cases like "python-3.10" or "node-22"
		if len(name) > 0 {
			for _, prefix := range []string{"python-", "node-", "nodejs-"} {
				if len(name) > len(prefix) && name[:len(prefix)] == prefix {
					language = name[:len(prefix)-1]
					version = name[len(prefix):]
					break
				}
			}
		}

		if language == "" {
			continue
		}

		runtime, err := m.GetCachedRuntime(language, version)
		if err != nil || runtime == nil {
			continue
		}

		runtimes = append(runtimes, *runtime)
	}

	// Sort by last used (most recent first)
	sort.Slice(runtimes, func(i, j int) bool {
		return runtimes[i].LastUsed.After(runtimes[j].LastUsed)
	})

	return runtimes, nil
}

// GetCacheInfo returns information about the cache
func (m *Manager) GetCacheInfo() (*CacheInfo, error) {
	runtimes, err := m.ListCachedRuntimes()
	if err != nil {
		return nil, err
	}

	info := &CacheInfo{
		Path:         m.CacheRoot,
		Runtimes:     runtimes,
		RuntimeCount: len(runtimes),
	}

	// Calculate total size
	info.TotalSize = getDirSize(m.CacheRoot)

	return info, nil
}

// CleanCache removes all cached data
func (m *Manager) CleanCache() error {
	return os.RemoveAll(m.CacheRoot)
}

// CleanRuntime removes a specific cached runtime
func (m *Manager) CleanRuntime(language, version string) error {
	runtimePath := m.GetCachedRuntimePath(language, version)
	return os.RemoveAll(runtimePath)
}

// PruneCache removes runtimes not used within the specified duration
func (m *Manager) PruneCache(olderThan time.Duration) (int, error) {
	runtimes, err := m.ListCachedRuntimes()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	pruned := 0

	for _, runtime := range runtimes {
		if runtime.LastUsed.Before(cutoff) {
			if err := m.CleanRuntime(runtime.Language, runtime.Version); err == nil {
				pruned++
			}
		}
	}

	return pruned, nil
}

// Helper functions

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

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, targetPath)
		}

		// Copy regular file
		return copyFile(path, targetPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// FormatBytes formats bytes as human-readable string
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
