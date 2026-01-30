# sbox

A secure sandbox for running AI agents with enforced isolation - **no Docker or sudo required**.

> *"Don't trust the agent. Trust the sandbox."*
>
> sbox enforces isolation through environment, not promises. Your files are protected not because the agent behaves, but because they simply don't exist in its world.

## Why sbox?

AI agents are powerful but unpredictable. When you give an agent access to your system, you're trusting it not to:
- Read sensitive files outside its workspace
- Modify system configurations
- Access credentials or API keys it shouldn't have
- Interfere with other processes

**sbox removes the need to trust agents** by restricting what they can access. Each agent runs in an isolated environment with:
- Its own filesystem (no access to host files unless explicitly mounted)
- Its own runtime (Python/Node.js isolated from system installations)
- Controlled environment variables (only what you configure)
- No network namespace tricks or root privileges required

Think of it as a lightweight container specifically designed for AI agent workloads, with a Docker-like workflow that runs entirely in user space.

## Privacy & Isolation

Some AI agent tools (such as [OpenClaw](https://github.com/openclaw/openclaw)) run directly with user-level permissions by default. This means the agent may technically access your entire home directory, including private files, unless additional measures are taken.

**sbox solves this problem by design.**

Agents are executed inside an isolated sandbox with a virtual HOME directory. If a file is not explicitly placed inside the sandbox, it simply does not exist to the agent.

**If a file is not copied or mounted, it does not exist to the agent.** Your SSH keys, cloud credentials, browser data, and personal documents remain completely invisible — not restricted, not permission-denied, but literally nonexistent from the agent's perspective. The agent cannot access, list, or even detect files outside its sandbox.

| Without sbox | With sbox |
|--------------|-----------|
| Agent has access to `~/.ssh/`, `~/.aws/`, etc. | Agent only sees `.sbox/rootfs/home/` |
| Agent can read browser cookies, credentials | Private files are invisible to the agent |
| Must trust agent code completely | Zero trust - agent is contained |

To make this easy to adopt, sbox provides pre-packaged releases. You can download a release, unpack it, and run agents inside a sandboxed environment in minutes — without Docker, sudo, or complex system setup:

```bash
# Download and run a pre-packaged agent environment
curl -LO https://github.com/CVPaul/sbox/releases/latest/download/sbox-openclaw.tar.gz
tar -xzf sbox-openclaw.tar.gz
cd openclaw
sbox unpack
sbox run
```

## Features

- **Zero trust by default** - Agents can only access what you explicitly allow
- **No sudo required** - Runs entirely in user space, no root privileges needed
- **No Docker required** - Works on any Linux/macOS system without container runtimes
- **AI agent ready** - Designed for running code-generating agents safely
- **Multi-runtime support** - Python and Node.js environments
- **Portable environments** - Pack and distribute sandboxes across machines
- **Process management** - Background daemons, logs, and monitoring
- **Fast setup** - Uses micromamba for quick environment creation

## Example: Running OpenClaw in sbox

This example shows how sbox isolates an AI agent like [OpenClaw](https://github.com/openclaw/openclaw) from your host system.

### The Isolation Model

When OpenClaw runs inside sbox, it sees a completely different filesystem:

```
What the agent sees:          What actually exists on host:
─────────────────────         ────────────────────────────
/                             .sbox/rootfs/
├── app/                      .sbox/rootfs/app/
│   └── openclaw/             .sbox/rootfs/app/openclaw/
├── home/                     .sbox/rootfs/home/        ← virtual HOME
│   ├── .npm/                 .sbox/rootfs/home/.npm/   ← npm cache isolated
│   └── .config/              .sbox/rootfs/home/.config/
└── tmp/                      .sbox/rootfs/tmp/

Files invisible to agent:
  ~/.ssh/*           (your SSH keys)
  ~/.aws/*           (cloud credentials)
  ~/.config/*        (app configs, tokens)
  ~/Documents/*      (personal files)
```

### Virtual HOME Directory

The agent's HOME is redirected to `.sbox/rootfs/home/`:

| Environment Variable | Value inside sbox |
|---------------------|-------------------|
| `HOME` | `/path/to/project/.sbox/rootfs/home` |
| `TMPDIR` | `/path/to/project/.sbox/rootfs/tmp` |
| `npm_config_cache` | Points to sandbox-local npm cache |

This means:
- `npm install` writes to the sandbox, not `~/.npm`
- Config files created by the agent stay in the sandbox
- The agent cannot discover or access your real home directory

### Package Managers are Sandboxed

When you run `npm install -g`, `pip install --user`, or any package manager command inside sbox, packages are installed **into the sandbox, not your system**.

**"Global" installs are sandbox-global, not system-global.**

| Package Manager | Without sbox | With sbox |
|-----------------|--------------|-----------|
| `npm install -g` | `/usr/local/lib/node_modules/` | `.sbox/env/lib/node_modules/` |
| `pip install --user` | `~/.local/lib/python*/` | Blocked by `PYTHONNOUSERSITE=1` |
| `pip install` | System or venv | `.sbox/env/lib/python*/` |
| npm cache | `~/.npm/` | `.sbox/rootfs/home/.npm/` |
| pip cache | `~/.cache/pip/` | `.sbox/rootfs/home/.cache/pip/` |

This works through environment variable redirection:

```bash
# Inside the sandbox, these are set automatically:
HOME=/path/to/project/.sbox/rootfs/home
TMPDIR=/path/to/project/.sbox/rootfs/tmp
PYTHONNOUSERSITE=1        # Blocks ~/.local imports
PATH=.sbox/env/bin:...    # Sandbox binaries first
```

Because `HOME` points to the sandbox, any tool that respects `$HOME` for cache or config storage (npm, pip, cargo, go, yarn, pnpm, etc.) will write to the sandbox instead of your real home directory.

**What this means for AI agents:**
- An agent can run `npm install -g malicious-package` — it only affects the sandbox
- An agent can run `pip install sketchy-lib` — it cannot touch your system Python
- An agent cannot pollute your `~/.npmrc`, `~/.pip/`, or any other dotfiles

### Example: Complete Isolation Config

Here's a typical `.sbox/config.yaml` for running an AI coding agent:

```yaml
# .sbox/config.yaml
runtime: node:22

# Copy only what the agent needs
copy:
  - src: ../my-project
    dst: /app/workspace

# Working directory inside sandbox
workdir: /app/workspace

# Command to run the agent
cmd: node agent.js

# Environment variables passed to the agent
env:
  OPENAI_API_KEY: "${OPENAI_API_KEY}"   # Passed from host
  NODE_ENV: "production"

# Optional: Mount directories (read-only recommended)
mount:
  - "/path/to/datasets:/data:ro"        # Read-only dataset access

# Build-time commands (run once during 'sbox build')
build:
  - npm install
  - npm run build
```

**What happens when you run `sbox build` + `sbox run`:**

```
Your System                          Sandbox (.sbox/)
─────────────────────────────────────────────────────────────────
~/.ssh/                         →    (not visible)
~/.aws/                         →    (not visible)
~/.npmrc                        →    (not visible)
~/.gitconfig                    →    (not visible)

$HOME                           →    .sbox/rootfs/home/     (empty)
$TMPDIR                         →    .sbox/rootfs/tmp/
$PATH                           →    .sbox/env/bin:...

npm install -g <pkg>            →    .sbox/env/lib/node_modules/
pip install <pkg>               →    .sbox/env/lib/python*/
~/                              →    .sbox/rootfs/home/
```

The agent sees an isolated world where your credentials and system files simply don't exist.

### Directory Layout After Build

```
openclaw-sandbox/
├── .sbox/
│   ├── config.yaml        # What files to copy, what command to run
│   ├── env/               # Isolated Node.js runtime
│   │   ├── bin/node
│   │   └── bin/pnpm
│   ├── rootfs/            # The agent's entire world
│   │   ├── app/openclaw/  # Application code
│   │   ├── home/          # Agent's HOME (empty by default)
│   │   └── tmp/           # Agent's temp directory
│   └── env.sh             # Environment activation script
└── sbox.lock              # Build state
```

### What the Agent Can and Cannot Do

| Action | Allowed? | Reason |
|--------|----------|--------|
| Read/write files in `/app/` | Yes | Explicitly copied into sandbox |
| Install npm packages | Yes | Writes to sandbox-local directories |
| Read `~/.ssh/id_rsa` | No | Does not exist in sandbox |
| Access `~/.aws/credentials` | No | Does not exist in sandbox |
| Create files in `~/` | Yes* | Creates in virtual HOME, not real HOME |
| Run system commands | Limited | Only what's in the sandbox runtime |

*Files created in the agent's `~/` end up in `.sbox/rootfs/home/`, completely separate from your real home directory.

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

### Packaging & Distribution

| Command | Description |
|---------|-------------|
| `sbox pack` | Package sandbox into portable tar.gz archive |
| `sbox unpack` | Relocate paths in extracted archive for new location |
| `sbox cache list` | List cached runtimes |
| `sbox cache clean` | Remove cached runtimes |
| `sbox cache prune` | Remove old unused cache entries |

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

## Directory Mounts

Mount host directories into the sandbox without copying files. Mounts are implemented as symlinks, providing direct access to host files with zero copy overhead.

### Configuration

Add the `mount` field to `.sbox/config.yaml`:

```yaml
runtime: python:3.11
workdir: /app

copy:
  - ./app:/app

# Mount host directories into the sandbox
mount:
  - /path/to/data:/data           # Read-write mount
  - /path/to/models:/models:ro    # Read-only mount (ro flag for documentation)
  - ./local/dir:/container/path   # Relative path (resolved to project root)
```

### Mount vs Copy

| Aspect | `mount` | `copy` |
|--------|---------|--------|
| Mechanism | Symlink | File copy |
| Disk usage | Zero overhead | Duplicates files |
| File changes | Instantly reflected | Requires rebuild |
| Best for | Large datasets, shared models | Application code |

### Use Cases

```yaml
# Mount large datasets
mount:
  - /mnt/datasets:/data

# Mount pre-trained models (avoid copying GBs of weights)
mount:
  - ~/.cache/huggingface:/models

# Mount configuration from host
mount:
  - /etc/myapp:/etc/myapp:ro

# Mount output directory for results
mount:
  - ./output:/app/output
```

## Packaging & Distribution

sbox provides two commands for portable deployment:

| Command | Purpose | Executes Code? | Network Access? |
|---------|---------|----------------|-----------------|
| `sbox pack` | Create a portable tar.gz archive | No | No |
| `sbox unpack` | Relocate paths after extraction | No | No |

**Key concept:** `sbox pack` bundles everything needed to run the sandbox. `sbox unpack` only rewrites hardcoded paths for the new location — similar to `conda-unpack`. Neither command executes code or downloads anything.

### Creating a Portable Archive with `sbox pack`

`sbox pack` creates a self-contained archive that can be transferred to other machines:

```bash
# Pack the current project (creates <project>-sbox.tar.gz)
sbox pack

# Custom output path
sbox pack --output myapp-v1.0.tar.gz

# Exclude runtime environment (smaller archive, recipient must run sbox build)
sbox pack --exclude-env

# Include local cache (for offline deployment)
sbox pack --include-cache
```

### Archive Contents

The packed archive includes:

```
myproject-sbox.tar.gz
├── .sbox/
│   ├── config.yaml      # Project configuration
│   ├── rootfs/          # Application files
│   └── env/             # Runtime environment (unless --exclude-env)
├── metadata.json        # Pack metadata (version, timestamp, checksums)
└── README.txt           # Instructions for extraction and usage
```

### Distributing a Packed Sandbox

The complete workflow for distributing sbox environments:

```bash
# === On source machine ===
cd myproject
sbox build                    # Build the sandbox
sbox pack                     # Create portable archive

# Transfer to target
scp myproject-sbox.tar.gz user@remote:/path/

# === On target machine ===
cd /path
tar -xzf myproject-sbox.tar.gz   # Manual extraction (security checkpoint)
cd myproject
sbox unpack                      # Relocate paths for new location
sbox run                         # Run the application
```

### Path Relocation with `sbox unpack`

When you extract a packed archive to a different path than where it was built, hardcoded paths in the environment need to be updated. The `sbox unpack` command handles this automatically:

```bash
# After extracting to a new location
cd /new/path/myproject
sbox unpack

# With verbose output
sbox unpack --verbose

# Dry run to see what would change
sbox unpack --dry-run
```

**What `sbox unpack` does:**

1. **Regenerates `.sbox/env.sh`** with correct absolute paths
2. **Updates conda-meta/*.json** files with new prefix paths  
3. **Fixes shebang lines** in scripts that reference the old location
4. **Updates sbox.lock** to reflect the relocation

This is similar to `conda-unpack` - it only performs text replacement in configuration files.

### `sbox unpack` is a Relocator, Not an Installer

To be absolutely clear: **`sbox unpack` does not install anything.** It is a pure path relocator.

| What people might expect | What actually happens |
|--------------------------|----------------------|
| Runs `npm install` or `pip install` | ❌ No package manager is invoked |
| Executes post-install scripts | ❌ No scripts are executed |
| Downloads dependencies from the internet | ❌ Zero network access |
| Modifies files outside the project | ❌ Only touches `.sbox/` directory |
| Requires elevated permissions | ❌ Runs as normal user |

**The only thing `sbox unpack` does:**
1. Read the original build path from `metadata.json`
2. Find-and-replace that path with the current path in config files
3. Write the updated files back

No code is executed. No interpreters are invoked. No network connections are made. Files outside `.sbox/` are never read or written.

This makes `sbox unpack` safe to run on untrusted archives — it cannot execute malicious code because it doesn't execute any code at all.

### Security Boundary of `sbox unpack`

The `sbox unpack` command has a strict security boundary:

| What it does | What it does NOT do |
|--------------|---------------------|
| Text replacement in config files | Execute any code |
| Regenerate env.sh script | Download anything |
| Update JSON metadata | Run install commands |
| Fix hardcoded paths | Modify application code |

**Security guarantees:**

- **No code execution**: Only performs string replacement operations
- **No network access**: Does not download or upload anything
- **No side effects**: Only modifies sbox-specific configuration files
- **Auditable**: Use `--dry-run --verbose` to see exactly what will change

```bash
# Verify changes before applying
sbox unpack --dry-run --verbose

# Example output:
# [STEP] Relocating paths for: myproject
# [INFO] Original prefix: /home/alice/myproject
# [INFO] New prefix:      /home/bob/myproject
# [STEP] Regenerating environment script...
# [INFO]   Writing: /home/bob/myproject/.sbox/env.sh
# [STEP] Updating conda metadata...
# [INFO]   Updating: nodejs-22.21.1-h273caaf_1.json
# ...
```

### When is `sbox unpack` Required?

| Scenario | Unpack needed? |
|----------|----------------|
| Extract to same path as original build | No |
| Extract to different path | **Yes** |
| Extract on same machine, different user | **Yes** |
| Extract on different machine | **Yes** |

If you skip `sbox unpack` when paths differ, you'll see errors like:
- "No such file or directory" when running commands
- Wrong Python/Node interpreter being used
- Environment variables pointing to non-existent paths

### Safe Workflow for Receiving Archives

Always inspect archives from untrusted sources before running:

```bash
# 1. Extract manually (security checkpoint)
tar -xzf received-archive.tar.gz

# 2. Inspect contents before running
cat myproject/.sbox/config.yaml    # What command will run?
cat myproject/metadata.json        # Where was it built?
ls -la myproject/.sbox/rootfs/     # What files are included?

# 3. Relocate paths (safe - no code execution)
cd myproject
sbox unpack --dry-run              # Preview changes
sbox unpack                        # Apply changes

# 4. Run only after verification
sbox run
```

## Cache Management

sbox maintains a global cache at `~/.sbox/cache/` to speed up builds and reduce disk usage.

### Cache Structure

```
~/.sbox/cache/
├── bin/
│   └── micromamba           # Shared micromamba binary
├── runtimes/
│   ├── python-3.11/         # Cached Python 3.11 environment
│   ├── python-3.12/         # Cached Python 3.12 environment
│   └── node-22/             # Cached Node.js 22 environment
└── pkgs/                    # Shared conda package cache
```

### Cache Commands

```bash
# List cached runtimes
sbox cache list

# Show cache location and size
sbox cache info

# Show cache path (useful for scripts)
sbox cache path

# Remove all cached runtimes
sbox cache clean

# Remove old/unused cache entries
sbox cache prune
```

### How Caching Works

1. **First build**: Downloads micromamba, creates runtime, caches it
2. **Subsequent builds**: Copies cached runtime instead of downloading
3. **Shared packages**: All projects share the same conda package cache

```bash
# First project - downloads and caches Python 3.11
cd project1
sbox build    # ~2 minutes

# Second project - uses cached Python 3.11
cd project2
sbox build    # ~10 seconds
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
