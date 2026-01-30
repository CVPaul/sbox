# sbox-openclaw

Pre-configured sbox template for running [OpenClaw](https://github.com/openclaw/openclaw) AI agent in isolation.

## Quick Start

```bash
# 1. Extract the archive
tar -xzf sbox-openclaw.tar.gz
cd sbox-openclaw

# 2. Copy your openclaw installation
cp -r /path/to/openclaw .sbox/rootfs/app/

# 3. Edit config if needed
vi .sbox/config.yaml

# 4. Build the sandbox
sbox build

# 5. Run the agent
sbox run
```

## What's Included

```
sbox-openclaw/
├── .sbox/
│   ├── config.yaml      # Pre-configured for Node.js 22
│   └── rootfs/
│       ├── app/         # Place openclaw code here
│       ├── home/        # Agent's virtual HOME (isolated)
│       └── tmp/         # Agent's temp directory
└── README.md
```

## Security

The agent runs in complete isolation:
- **No access to your real HOME** - `~/.ssh`, `~/.aws`, etc. are invisible
- **No access to system files** - only what's in `.sbox/rootfs/`
- **Package installs are sandboxed** - `npm install -g` writes to sandbox only

**If a file is not mounted, it does not exist to the agent.**

## Configuration

Edit `.sbox/config.yaml` to:
- Add environment variables (API keys)
- Mount additional directories
- Customize build commands

See [sbox documentation](https://github.com/CVPaul/sbox) for full configuration options.
