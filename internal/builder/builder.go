// Package builder handles the sandbox build process.
package builder

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sbox-project/sbox/internal/config"
	"github.com/sbox-project/sbox/internal/console"
	"github.com/sbox-project/sbox/internal/runtime"
)

// Builder builds the sandbox environment
type Builder struct {
	ProjectRoot string
	Config      *config.Config
}

// New creates a new builder
func New(projectRoot string) (*Builder, error) {
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return nil, err
	}

	return &Builder{
		ProjectRoot: projectRoot,
		Config:      cfg,
	}, nil
}

// Build executes the full build process
func (b *Builder) Build(force bool) error {
	console.Step("Building sandbox in %s", b.ProjectRoot)

	// Check if rebuild is needed
	if !force && config.IsUpToDate(b.ProjectRoot, b.Config) {
		console.Info("Build is up to date, use --force to rebuild")
		return nil
	}

	// 1. Setup runtime
	rtInfo := b.Config.ParseRuntime()
	rtManager := runtime.NewManager(b.ProjectRoot)
	if err := rtManager.Setup(rtInfo); err != nil {
		return fmt.Errorf("runtime setup failed: %w", err)
	}

	// 2. Setup rootfs structure
	if err := b.setupRootfs(); err != nil {
		return fmt.Errorf("rootfs setup failed: %w", err)
	}

	// 3. Copy files
	if err := b.copyFiles(); err != nil {
		return fmt.Errorf("file copy failed: %w", err)
	}

	// 4. Install packages
	if err := rtManager.InstallPackages(b.Config.Install); err != nil {
		return fmt.Errorf("package installation failed: %w", err)
	}

	// 5. Generate env.sh
	if err := b.generateEnvScript(); err != nil {
		return fmt.Errorf("env script generation failed: %w", err)
	}

	// 6. Update lock file
	if err := config.SaveLock(b.ProjectRoot, b.Config); err != nil {
		return fmt.Errorf("lock file update failed: %w", err)
	}
	console.Info("Updated %s", config.GetLockPath(b.ProjectRoot))

	console.Success("Build complete!")
	return nil
}

func (b *Builder) setupRootfs() error {
	console.Step("Setting up rootfs...")

	rootfs := config.GetRootfsDir(b.ProjectRoot)
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return err
	}

	// Create standard directories
	dirs := []string{"home", "tmp", "app"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(rootfs, d), 0755); err != nil {
			return err
		}
	}

	// Make tmp writable
	os.Chmod(filepath.Join(rootfs, "tmp"), 0777)

	console.Success("Rootfs ready")
	return nil
}

func (b *Builder) copyFiles() error {
	copySpecs := b.Config.ParseCopy()
	if len(copySpecs) == 0 {
		return nil
	}

	console.Step("Copying files...")
	rootfs := config.GetRootfsDir(b.ProjectRoot)

	for _, spec := range copySpecs {
		// Resolve source (relative to project root)
		src := filepath.Join(b.ProjectRoot, strings.TrimPrefix(spec.Src, "./"))

		// Resolve destination (in rootfs)
		var dst string
		if strings.HasPrefix(spec.Dst, "/") {
			dst = filepath.Join(rootfs, strings.TrimPrefix(spec.Dst, "/"))
		} else {
			dst = filepath.Join(rootfs, spec.Dst)
		}

		if _, err := os.Stat(src); err != nil {
			console.Warning("Source not found: %s", src)
			continue
		}

		// Create destination parent
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		// Copy
		if err := copyPath(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", spec.Src, err)
		}

		console.Info("Copied: %s -> %s", spec.Src, spec.Dst)
	}

	console.Success("Files copied")
	return nil
}

func copyPath(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyDir(src, dst string) error {
	// Remove existing destination
	os.RemoveAll(dst)

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (b *Builder) generateEnvScript() error {
	envDir := config.GetEnvDir(b.ProjectRoot)
	rootfs := config.GetRootfsDir(b.ProjectRoot)
	sboxDir := config.GetSboxDir(b.ProjectRoot)
	scriptPath := filepath.Join(sboxDir, config.EnvScript)

	content := fmt.Sprintf(`#!/bin/bash
# sbox environment activation script
# Source this file to activate the sandbox environment:
#   source .sbox/env.sh

export SBOX_ACTIVE=1
export SBOX_PROJECT="%s"

# Python isolation
export PYTHONNOUSERSITE=1
export PYTHONDONTWRITEBYTECODE=1
export PIP_DISABLE_PIP_VERSION_CHECK=1

# Paths
export PATH="%s/bin:$PATH"
export HOME="%s/home"
export TMPDIR="%s/tmp"

# Conda/mamba
export CONDA_PREFIX="%s"
export MAMBA_ROOT_PREFIX="%s/mamba"

`, b.ProjectRoot, envDir, rootfs, rootfs, envDir, sboxDir)

	// Add custom env vars
	for key, value := range b.Config.Env {
		content += fmt.Sprintf("export %s=\"%s\"\n", key, value)
	}

	content += `
echo "sbox environment activated"
echo "Python: $(which python)"
echo "Working directory: $SBOX_PROJECT"
`

	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		return err
	}

	console.Info("Generated %s", scriptPath)
	return nil
}
