// Package validate provides configuration validation for sbox.
package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sbox-project/sbox/internal/config"
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string
	Message string
	Hint    string
}

// ValidationResult contains all validation results
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationError
}

// Supported runtimes and versions
var (
	SupportedLanguages = []string{"python", "node", "nodejs"}

	SupportedPythonVersions = []string{"3.8", "3.9", "3.10", "3.11", "3.12", "3.13"}
	SupportedNodeVersions   = []string{"18", "20", "22", "23", "24"}

	// Regex patterns
	runtimePattern = regexp.MustCompile(`^(python|node|nodejs):(\d+\.?\d*)$`)
	copyPattern    = regexp.MustCompile(`^[^:]+:[^:]+$|^[^:]+$`)
	mountPattern   = regexp.MustCompile(`^[^:]+:[^:]+(:(ro|readonly))?$`)
	workdirPattern = regexp.MustCompile(`^/[a-zA-Z0-9_\-./]*$`)
	envKeyPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// ValidateConfig performs comprehensive validation on a config
func ValidateConfig(cfg *config.Config, projectRoot string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Validate runtime
	validateRuntime(cfg, result)

	// Validate workdir
	validateWorkdir(cfg, result)

	// Validate copy specs
	validateCopy(cfg, projectRoot, result)

	// Validate mount specs
	validateMount(cfg, projectRoot, result)

	// Validate install commands
	validateInstall(cfg, result)

	// Validate cmd
	validateCmd(cfg, result)

	// Validate environment variables
	validateEnv(cfg, result)

	// Set overall validity
	result.Valid = len(result.Errors) == 0

	return result
}

// ValidateConfigFile validates a config file at the given path
func ValidateConfigFile(configPath string) (*ValidationResult, *config.Config, error) {
	// Check file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "config",
				Message: fmt.Sprintf("Config file not found: %s", configPath),
				Hint:    "Run 'sbox init <name>' to create a new project, or create .sbox/config.yaml manually",
			}},
		}, nil, nil
	}

	// Try to load config
	projectRoot := filepath.Dir(filepath.Dir(configPath))
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "config",
				Message: fmt.Sprintf("Failed to parse config: %s", err),
				Hint:    "Check YAML syntax. Use 'sbox validate' for detailed diagnostics",
			}},
		}, nil, nil
	}

	result := ValidateConfig(cfg, projectRoot)
	return result, cfg, nil
}

func validateRuntime(cfg *config.Config, result *ValidationResult) {
	if cfg.Runtime == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "runtime",
			Message: "Runtime is required",
			Hint:    "Add 'runtime: python:3.11' or 'runtime: node:22' to your config.yaml",
		})
		return
	}

	// Check format
	if !runtimePattern.MatchString(cfg.Runtime) {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "runtime",
			Message: fmt.Sprintf("Invalid runtime format: '%s'", cfg.Runtime),
			Hint:    "Use format 'language:version', e.g., 'python:3.11' or 'node:22'",
		})
		return
	}

	// Parse and validate
	info := cfg.ParseRuntime()

	// Check language
	validLang := false
	for _, lang := range SupportedLanguages {
		if info.Language == lang {
			validLang = true
			break
		}
	}
	if !validLang {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "runtime",
			Message: fmt.Sprintf("Unsupported language: '%s'", info.Language),
			Hint:    fmt.Sprintf("Supported languages: %s", strings.Join(SupportedLanguages, ", ")),
		})
		return
	}

	// Check version (warning only for unknown versions)
	var supportedVersions []string
	if info.Language == "python" {
		supportedVersions = SupportedPythonVersions
	} else {
		supportedVersions = SupportedNodeVersions
	}

	versionValid := false
	for _, v := range supportedVersions {
		if info.Version == v || strings.HasPrefix(info.Version, v) {
			versionValid = true
			break
		}
	}

	if !versionValid {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "runtime",
			Message: fmt.Sprintf("Version '%s' may not be available for %s", info.Version, info.Language),
			Hint:    fmt.Sprintf("Recommended versions: %s", strings.Join(supportedVersions, ", ")),
		})
	}
}

func validateWorkdir(cfg *config.Config, result *ValidationResult) {
	if cfg.Workdir == "" {
		// Will use default, just warn
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "workdir",
			Message: "Workdir not specified, using default '/app'",
			Hint:    "Add 'workdir: /app' to explicitly set the working directory",
		})
		return
	}

	// Check format (must be absolute path)
	if !strings.HasPrefix(cfg.Workdir, "/") {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "workdir",
			Message: fmt.Sprintf("Workdir must be an absolute path: '%s'", cfg.Workdir),
			Hint:    "Use an absolute path like '/app' or '/home/user/app'",
		})
		return
	}

	// Check for valid characters
	if !workdirPattern.MatchString(cfg.Workdir) {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "workdir",
			Message: fmt.Sprintf("Workdir contains unusual characters: '%s'", cfg.Workdir),
			Hint:    "Stick to alphanumeric characters, dashes, underscores, and slashes",
		})
	}
}

func validateCopy(cfg *config.Config, projectRoot string, result *ValidationResult) {
	if len(cfg.Copy) == 0 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "copy",
			Message: "No files specified to copy into sandbox",
			Hint:    "Add 'copy: [\"./app:/app\"]' to copy your application files",
		})
		return
	}

	for i, spec := range cfg.Copy {
		// Check format
		if !copyPattern.MatchString(spec) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("copy[%d]", i),
				Message: fmt.Sprintf("Invalid copy specification: '%s'", spec),
				Hint:    "Use format 'source:destination' or just 'path' (e.g., './app:/app')",
			})
			continue
		}

		// Parse and check source exists
		parts := strings.SplitN(spec, ":", 2)
		src := parts[0]

		// Resolve relative to project root
		srcPath := src
		if !filepath.IsAbs(src) {
			srcPath = filepath.Join(projectRoot, src)
		}

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   fmt.Sprintf("copy[%d]", i),
				Message: fmt.Sprintf("Source path does not exist: '%s'", src),
				Hint:    fmt.Sprintf("Create the directory/file or update the path. Looked in: %s", srcPath),
			})
		}

		// Check destination is absolute
		if len(parts) == 2 {
			dst := parts[1]
			if !strings.HasPrefix(dst, "/") {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("copy[%d]", i),
					Message: fmt.Sprintf("Destination must be absolute path: '%s'", dst),
					Hint:    "Use an absolute path like '/app' for the destination",
				})
			}
		}
	}
}

func validateMount(cfg *config.Config, projectRoot string, result *ValidationResult) {
	if len(cfg.Mount) == 0 {
		// Mount is optional, no warning needed
		return
	}

	for i, spec := range cfg.Mount {
		// Check format: /host/path:/container/path or /host/path:/container/path:ro
		if !mountPattern.MatchString(spec) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mount[%d]", i),
				Message: fmt.Sprintf("Invalid mount specification: '%s'", spec),
				Hint:    "Use format '/host/path:/container/path' or '/host/path:/container/path:ro'",
			})
			continue
		}

		parts := strings.Split(spec, ":")
		if len(parts) < 2 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mount[%d]", i),
				Message: fmt.Sprintf("Invalid mount specification: '%s'", spec),
				Hint:    "Use format '/host/path:/container/path'",
			})
			continue
		}

		src := parts[0]
		dst := parts[1]

		// Resolve relative paths to project root
		srcPath := src
		if !filepath.IsAbs(src) {
			srcPath = filepath.Join(projectRoot, src)
		}

		// Check source exists
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   fmt.Sprintf("mount[%d]", i),
				Message: fmt.Sprintf("Mount source path does not exist: '%s'", src),
				Hint:    fmt.Sprintf("Create the directory or update the path. Looked in: %s", srcPath),
			})
		}

		// Check destination is absolute
		if !strings.HasPrefix(dst, "/") {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("mount[%d]", i),
				Message: fmt.Sprintf("Mount destination must be absolute path: '%s'", dst),
				Hint:    "Use an absolute path like '/data' for the mount destination",
			})
		}

		// Check for option validity
		if len(parts) >= 3 {
			option := parts[2]
			if option != "ro" && option != "readonly" {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   fmt.Sprintf("mount[%d]", i),
					Message: fmt.Sprintf("Unknown mount option: '%s'", option),
					Hint:    "Valid options: 'ro' or 'readonly' for read-only mounts",
				})
			}
		}

		// Check for conflicts with copy destinations
		for j, copySpec := range cfg.Copy {
			copyParts := strings.SplitN(copySpec, ":", 2)
			var copyDst string
			if len(copyParts) == 2 {
				copyDst = copyParts[1]
			} else {
				copyDst = copyParts[0]
			}

			if dst == copyDst || strings.HasPrefix(dst, copyDst+"/") || strings.HasPrefix(copyDst, dst+"/") {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   fmt.Sprintf("mount[%d]", i),
					Message: fmt.Sprintf("Mount destination '%s' overlaps with copy destination in copy[%d]", dst, j),
					Hint:    "Mount and copy destinations should not overlap to avoid conflicts",
				})
			}
		}
	}
}

func validateInstall(cfg *config.Config, result *ValidationResult) {
	if len(cfg.Install) == 0 {
		// This is fine, might not need install commands
		return
	}

	runtimeInfo := cfg.ParseRuntime()

	for i, cmd := range cfg.Install {
		// Check for empty commands
		if strings.TrimSpace(cmd) == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("install[%d]", i),
				Message: "Empty install command",
				Hint:    "Remove empty commands or add a valid command",
			})
			continue
		}

		// Check for common mistakes based on runtime
		if runtimeInfo.Language == "python" {
			if strings.Contains(cmd, "npm install") || strings.Contains(cmd, "pnpm install") {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   fmt.Sprintf("install[%d]", i),
					Message: "Using npm/pnpm with Python runtime",
					Hint:    "You're using a Python runtime but have Node.js install commands. Change runtime to 'node:22' if this is a Node.js project",
				})
			}
		} else if runtimeInfo.Language == "node" || runtimeInfo.Language == "nodejs" {
			if strings.Contains(cmd, "pip install") {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   fmt.Sprintf("install[%d]", i),
					Message: "Using pip with Node.js runtime",
					Hint:    "You're using a Node.js runtime but have Python install commands. Change runtime to 'python:3.11' if this is a Python project",
				})
			}
		}

		// Check for sudo usage (not needed in sbox)
		if strings.Contains(cmd, "sudo ") {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   fmt.Sprintf("install[%d]", i),
				Message: "Using sudo in install command",
				Hint:    "sbox runs in user space - sudo is not needed and may cause issues. Remove 'sudo' from the command",
			})
		}

		// Check for global installs that might fail
		if strings.Contains(cmd, "npm install -g") || strings.Contains(cmd, "pip install --user") {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   fmt.Sprintf("install[%d]", i),
				Message: "Global/user install may not work as expected",
				Hint:    "In sbox, packages are installed in an isolated environment. Global flags may not be necessary",
			})
		}
	}
}

func validateCmd(cfg *config.Config, result *ValidationResult) {
	if cfg.Cmd == "" {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "cmd",
			Message: "No default command specified",
			Hint:    "Add 'cmd: python main.py' or similar to set the default run command",
		})
		return
	}

	runtimeInfo := cfg.ParseRuntime()

	// Check command matches runtime
	if runtimeInfo.Language == "python" {
		if strings.HasPrefix(cfg.Cmd, "node ") || strings.HasPrefix(cfg.Cmd, "npm ") {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   "cmd",
				Message: "Node.js command with Python runtime",
				Hint:    "Your command uses Node.js but runtime is Python. Update runtime or command",
			})
		}
	} else if runtimeInfo.Language == "node" || runtimeInfo.Language == "nodejs" {
		if strings.HasPrefix(cfg.Cmd, "python ") || strings.HasPrefix(cfg.Cmd, "python3 ") {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:   "cmd",
				Message: "Python command with Node.js runtime",
				Hint:    "Your command uses Python but runtime is Node.js. Update runtime or command",
			})
		}
	}
}

func validateEnv(cfg *config.Config, result *ValidationResult) {
	if cfg.Env == nil {
		return
	}

	reservedVars := []string{
		"PATH", "HOME", "SBOX_ACTIVE", "SBOX_PROJECT",
		"CONDA_PREFIX", "MAMBA_ROOT_PREFIX",
	}

	for key, value := range cfg.Env {
		// Check key format
		if !envKeyPattern.MatchString(key) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("env.%s", key),
				Message: fmt.Sprintf("Invalid environment variable name: '%s'", key),
				Hint:    "Environment variable names must start with a letter or underscore, followed by letters, numbers, or underscores",
			})
			continue
		}

		// Check for reserved variables
		for _, reserved := range reservedVars {
			if strings.ToUpper(key) == reserved {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   fmt.Sprintf("env.%s", key),
					Message: fmt.Sprintf("'%s' is managed by sbox and may be overwritten", key),
					Hint:    "This variable is set automatically by sbox. Your value may not take effect",
				})
				break
			}
		}

		// Check for potentially sensitive values in plain text
		sensitivePatterns := []string{"password", "secret", "key", "token", "credential"}
		keyLower := strings.ToLower(key)
		for _, pattern := range sensitivePatterns {
			if strings.Contains(keyLower, pattern) && value != "" && !strings.HasPrefix(value, "${") {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   fmt.Sprintf("env.%s", key),
					Message: "Sensitive value may be stored in plain text",
					Hint:    "Consider using environment variable expansion like '${MY_SECRET}' or a secrets manager",
				})
				break
			}
		}
	}
}

// FormatValidationResult returns a formatted string of validation results
func FormatValidationResult(result *ValidationResult) string {
	var sb strings.Builder

	if result.Valid && len(result.Warnings) == 0 {
		sb.WriteString("✓ Configuration is valid\n")
		return sb.String()
	}

	if len(result.Errors) > 0 {
		sb.WriteString("✗ Configuration errors:\n\n")
		for _, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("  [ERROR] %s\n", err.Field))
			sb.WriteString(fmt.Sprintf("          %s\n", err.Message))
			if err.Hint != "" {
				sb.WriteString(fmt.Sprintf("          → %s\n", err.Hint))
			}
			sb.WriteString("\n")
		}
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("⚠ Configuration warnings:\n\n")
		for _, warn := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  [WARN] %s\n", warn.Field))
			sb.WriteString(fmt.Sprintf("         %s\n", warn.Message))
			if warn.Hint != "" {
				sb.WriteString(fmt.Sprintf("         → %s\n", warn.Hint))
			}
			sb.WriteString("\n")
		}
	}

	if result.Valid {
		sb.WriteString("✓ Configuration is valid (with warnings)\n")
	} else {
		sb.WriteString("✗ Configuration is invalid. Please fix the errors above.\n")
	}

	return sb.String()
}

// QuickValidate performs a quick validation and returns an error if invalid
func QuickValidate(cfg *config.Config, projectRoot string) error {
	result := ValidateConfig(cfg, projectRoot)
	if !result.Valid {
		// Return first error
		if len(result.Errors) > 0 {
			err := result.Errors[0]
			return fmt.Errorf("%s: %s\n  Hint: %s", err.Field, err.Message, err.Hint)
		}
	}
	return nil
}

// GetConfigExample returns an example config for the given runtime
func GetConfigExample(runtime string) string {
	if strings.HasPrefix(runtime, "node") {
		return `# sbox configuration for Node.js project
runtime: node:22
workdir: /app

# Files to copy into sandbox
copy:
  - .:/app

# Directories to mount (symlink, not copy)
# mount:
#   - /path/to/data:/data
#   - /path/to/models:/models:ro

# Commands to run during build
install:
  - cd /app && npm install

# Default command
cmd: node index.js

# Environment variables
env:
  NODE_ENV: production
`
	}

	return `# sbox configuration for Python project
runtime: python:3.11
workdir: /app

# Files to copy into sandbox
copy:
  - ./app:/app

# Directories to mount (symlink, not copy)
# mount:
#   - /path/to/data:/data
#   - /path/to/models:/models:ro

# Commands to run during build
install:
  - pip install -r /app/requirements.txt

# Default command
cmd: python main.py

# Environment variables
env:
  PYTHONPATH: /app
`
}
