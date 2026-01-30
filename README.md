# sbox

A rootless, user-space sandbox runtime with Docker-like workflow - **no sudo required**.

sbox creates isolated environments for Python and Node.js applications using [micromamba](https://mamba.readthedocs.io/en/latest/user_guide/micromamba.html), providing container-like isolation without requiring root privileges or Docker.

## Features

- **No sudo required** - Runs entirely in user space
- **Docker-like workflow** - Familiar `init`, `build`, `run` commands
- **Multi-runtime support** - Python and Node.js
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

| Command | Description |
|---------|-------------|
| `sbox init <name>` | Initialize a new sbox project |
| `sbox build` | Build the sandbox environment |
| `sbox run [cmd]` | Run the application (or custom command) |
| `sbox shell` | Start an interactive shell in the sandbox |
| `sbox exec <cmd>` | Execute a command in the sandbox |
| `sbox status` | Show project status |
| `sbox clean` | Clean build artifacts |
| `sbox version` | Print version information |

### Command Options

```bash
# Initialize with specific runtime
sbox init myapp --runtime python:3.12
sbox init myapp --runtime node:20

# Force rebuild
sbox build --force

# Clean everything including config
sbox clean --all
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

## Project Structure

After running `sbox init myproject`, you'll get:

```
myproject/
├── .sbox/
│   ├── config.yaml    # Project configuration
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

# Or run with custom command
sbox run "node openclaw.mjs gateway"

# Interactive shell for debugging
sbox shell
```

### Step 4: Verify

```bash
# Check status
sbox status

# Expected output:
# [STEP] sbox project: openclaw
# [OK] Config: /path/to/openclaw/.sbox/config.yaml
#   Runtime: node:22
#   Workdir: /app
#   Command: node openclaw.mjs --help
# [OK] Build status: Built
#   Built at: 2026-01-30T12:00:00Z
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
