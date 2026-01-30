# sbox

A rootless, user-space sandbox runtime with Docker-like workflow - **no sudo required**.

sbox creates isolated environments for Python and Node.js applications using [micromamba](https://mamba.readthedocs.io/en/latest/user_guide/micromamba.html), providing container-like isolation without requiring root privileges or Docker.

## Features

- **No sudo required** - Runs entirely in user space
- **Docker-like workflow** - Familiar `init`, `build`, `run` commands
- **Multi-runtime support** - Python and Node.js
- **Process management** - Background daemons, logs, and monitoring
- **Portable environments** - Self-contained, reproducible builds
- **Fast setup** - Uses micromamba for quick environment creation
- **Cross-platform** - Linux (amd64/arm64) and macOS (Intel/Apple Silicon)

## Installation

### Build from source

```bash
# Requires Go 1.21+
git clone https://github.com/user/sbox.git
cd sbox
go build -o sbox ./cmd/sbox

# Optional: Install to PATH
sudo mv sbox /usr/local/bin/
```

### Pre-built binary

```bash
# Download the latest release for your platform
curl -LO https://github.com/user/sbox/releases/latest/download/sbox-linux-amd64
chmod +x sbox-linux-amd64
mv sbox-linux-amd64 /usr/local/bin/sbox
```

## Quick Start

### Python Project

```bash
# Initialize a new Python project
sbox init myapp --runtime python:3.11

# Navigate to project
cd myapp

# Build the sandbox environment
sbox build

# Run the application
sbox run

# Or start an interactive shell
sbox shell
```

### Node.js Project

```bash
# Initialize a new Node.js project
sbox init myapp --runtime node:22

cd myapp
sbox build
sbox run
```

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `sbox init <name>` | Initialize a new sbox project |
| `sbox build` | Build the sandbox environment |
| `sbox run [cmd]` | Run the application (or custom command) |
| `sbox shell` | Start an interactive shell in the sandbox |
| `sbox exec <cmd>` | Execute a command in the sandbox |
| `sbox clean` | Clean build artifacts |
| `sbox version` | Print version information |

### Process Management

| Command | Description |
|---------|-------------|
| `sbox run -d` | Run as a background daemon |
| `sbox ps` | List running sandbox processes |
| `sbox stop [name]` | Stop a running daemon |
| `sbox restart [name]` | Restart a daemon process |
| `sbox logs [name]` | View process logs |

### Status & Info

| Command | Description |
|---------|-------------|
| `sbox status` | Show detailed project status |
| `sbox info` | Show environment information |
| `sbox validate` | Validate configuration file |

### Command Options

```bash
# Initialize with specific runtime
sbox init myapp --runtime python:3.12
sbox init myapp --runtime node:20

# Force rebuild
sbox build --force
sbox build --verbose

# Run as background daemon
sbox run -d                    # Run default command as daemon
sbox run -d --name myservice   # Run with custom name
sbox run -d "node server.js"   # Run specific command

# Process management
sbox ps                        # List running processes
sbox ps --all                  # Include stopped processes
sbox stop myservice            # Stop specific process
sbox stop --all                # Stop all processes
sbox restart myservice         # Restart a process

# View logs
sbox logs                      # View default process logs
sbox logs myservice            # View specific process logs
sbox logs -f                   # Follow logs in real-time
sbox logs -n 100               # Show last 100 lines
sbox logs --list               # List available log files

# Status and info
sbox status                    # Detailed project status
sbox status --json             # Output as JSON
sbox info                      # Environment details
sbox validate                  # Validate configuration
sbox validate --quiet          # Only show errors

# Clean up
sbox clean                     # Clean build artifacts
sbox clean --all               # Remove everything including config
sbox clean --logs              # Only clean log files
```

## Configuration

sbox uses a YAML configuration file at `.sbox/config.yaml`:

```yaml
# Runtime: python:<version> or node:<version>
runtime: python:3.11

# Working directory inside the sandbox
workdir: /app

# Files/directories to copy into the sandbox
copy:
  - ./app:/app
  - ./config:/config

# Commands to run during build (after environment setup)
install:
  - pip install -r app/requirements.txt

# Default command to run
cmd: python main.py

# Environment variables
env:
  PYTHONPATH: /app
  DEBUG: "true"
```

### Configuration Validation

sbox validates your configuration before build/run and provides helpful error messages:

```bash
# Validate configuration
sbox validate

# Example output for invalid config:
# [STEP] Validating configuration: .sbox/config.yaml
#
# [ERROR] Configuration errors (2):
#
#   1. [runtime] Invalid runtime format: 'ruby:3.0'
#      → Use format 'language:version', e.g., 'python:3.11' or 'node:22'
#
#   2. [workdir] Workdir must be an absolute path: 'relative/path'
#      → Use an absolute path like '/app' or '/home/user/app'
#
# [WARN] Configuration warnings (1):
#
#   1. [install[0]] Using sudo in install command
#      → sbox runs in user space - sudo is not needed
```

Validation checks:
- **Runtime format**: Must be `language:version` (e.g., `python:3.11`, `node:22`)
- **Supported languages**: `python`, `node` (with recommended versions)
- **Workdir**: Must be an absolute path
- **Copy specs**: Valid format and source existence
- **Install commands**: Runtime compatibility, sudo usage warnings
- **Environment variables**: Valid naming, reserved variable warnings
- **Security**: Warnings for plain-text secrets

## Project Structure

After running `sbox init myproject`, you'll get:

```
myproject/
├── .sbox/
│   ├── config.yaml    # Project configuration
│   ├── logs/          # Process log files
│   ├── env/           # Runtime environment (after build)
│   ├── rootfs/        # Application files (after build)
│   ├── mamba/         # Micromamba installation (after build)
│   └── bin/           # Binaries (after build)
├── app/
│   ├── main.py        # Entry point
│   └── requirements.txt
├── .gitignore
└── sbox.lock          # Build lock file (after build)
```

## Real-World Example: Deploying OpenClaw

This example demonstrates deploying [OpenClaw](https://github.com/openclaw/openclaw), a Node.js-based personal AI assistant, using sbox.

### Step 1: Clone the Project

```bash
git clone https://github.com/openclaw/openclaw.git
cd openclaw
```

### Step 2: Create sbox Configuration

Create `.sbox/config.yaml`:

```yaml
# OpenClaw sbox configuration
runtime: node:22
workdir: /app

copy:
  - .:/app

install:
  - cd /app && pnpm install --registry https://registry.npmmirror.com
  - cd /app && pnpm build

cmd: node openclaw.mjs --help

env:
  NODE_ENV: production
  OPENCLAW_PROFILE: sbox
```

### Step 3: Build and Run

```bash
# Build the sandbox (downloads Node.js, installs dependencies)
sbox build

# Run OpenClaw
sbox run

# Or run as a background daemon
sbox run -d --name openclaw-gateway "node openclaw.mjs gateway"

# Check running processes
sbox ps

# View logs
sbox logs openclaw-gateway -f

# Interactive shell for debugging
sbox shell
```

### Step 4: Verify

```bash
# Check detailed status
sbox status

# Expected output:
# [STEP] sbox project: openclaw
#
#   ┌─ Configuration
#   │  Runtime:  node:22
#   │  Workdir:  /app
#   │  Command:  node openclaw.mjs --help
#   │  Env vars: 2 defined
#
#   ┌─ Build Status
#   │  Status:  ✓ Built
#   │  State:   Up to date
#   │  Hash:    a1b2c3d4
#   │  Built:   2026-01-30 12:00:00 (5m ago)
#
#   ┌─ Processes
#   │  Running: 1
#   │    • openclaw-gateway (PID 12345) - up 5m30s
#
#   ┌─ Logs
#   │  Available: 1 log file(s)
#   │    • openclaw-gateway (1.2 MB)
```

### Manual Deployment (Without sbox build)

If you prefer to use system-installed Node.js and pnpm:

```bash
# Configure npm registry (for environments with limited network)
pnpm config set registry https://registry.npmmirror.com

# Install dependencies
pnpm install

# Build the project
pnpm build

# Verify installation
node openclaw.mjs --version
# Output: 2026.1.29

node openclaw.mjs --help
# Shows full CLI help
```

## How It Works

1. **Environment Setup**: sbox downloads [micromamba](https://mamba.readthedocs.io/) and uses it to create isolated Python/Node.js environments
2. **Build Phase**: Copies project files to a rootfs, installs dependencies, runs install commands
3. **Run Phase**: Executes commands with proper PATH and environment variables set

### Directory Layout After Build

```
.sbox/
├── bin/
│   └── micromamba       # Micromamba binary
├── mamba/
│   └── pkgs/            # Package cache
├── env/
│   ├── bin/             # Python/Node binaries
│   ├── lib/             # Libraries
│   └── ...
├── rootfs/
│   └── app/             # Your application files
├── logs/
│   └── *.log            # Process log files
├── processes.json       # Process tracking
└── env.sh               # Environment activation script
```

## Supported Runtimes

| Runtime | Versions | Package Manager |
|---------|----------|-----------------|
| Python | 3.8, 3.9, 3.10, 3.11, 3.12 | pip |
| Node.js | 18, 20, 22, 23 | npm, pnpm |

## Tips & Tricks

### Using Chinese npm Mirror

For faster downloads in China:

```yaml
install:
  - cd /app && npm config set registry https://registry.npmmirror.com
  - cd /app && npm install
```

Or with pnpm:

```yaml
install:
  - cd /app && pnpm install --registry https://registry.npmmirror.com
```

### Custom Environment Variables

```yaml
env:
  DATABASE_URL: postgresql://localhost/mydb
  SECRET_KEY: your-secret-key
  NODE_OPTIONS: "--max-old-space-size=4096"
```

### Multiple Install Commands

```yaml
install:
  - pip install --upgrade pip
  - pip install -r requirements.txt
  - pip install -r requirements-dev.txt
  - python setup.py develop
```

### Debugging Build Issues

```bash
# Start shell without running cmd
sbox shell

# Inside the sandbox, check environment
which python
python --version
pip list

# Or for Node.js
which node
node --version
npm list
```

## Comparison with Alternatives

| Feature | sbox | Docker | venv | nvm |
|---------|------|--------|------|-----|
| No root required | Yes | No* | Yes | Yes |
| Multi-language | Yes | Yes | Python only | Node only |
| Portable | Yes | Yes | No | No |
| Fast startup | Yes | No | Yes | Yes |
| Process management | Yes | Yes | No | No |
| Log management | Yes | Yes | No | No |
| Network isolation | No | Yes | No | No |

*Docker can run rootless but requires additional setup

## Troubleshooting

### Build fails with network timeout

Try using a mirror:

```yaml
install:
  - pip install -i https://pypi.tuna.tsinghua.edu.cn/simple -r requirements.txt
```

### micromamba download fails

Manually download and place in `.sbox/bin/`:

```bash
# Linux x64
curl -L https://micro.mamba.pm/api/micromamba/linux-64/latest | tar -xvj bin/micromamba
mv bin/micromamba .sbox/bin/
```

### Permission denied

Ensure the sbox binary is executable:

```bash
chmod +x sbox
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [micromamba](https://mamba.readthedocs.io/) - Fast, minimal conda package manager
- [Cobra](https://github.com/spf13/cobra) - CLI framework for Go
- [OpenClaw](https://github.com/openclaw/openclaw) - Example deployment target
