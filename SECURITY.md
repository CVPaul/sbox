# Security Model

> **sbox provides filesystem and environment isolation in user space. It is not a kernel-level security sandbox.**

This document explains what sbox protects against, what it doesn't, and why.

## What sbox IS

sbox is an **environment isolation tool** that:

- Redirects `$HOME`, `$TMPDIR`, `$PATH` to sandbox-local directories
- Prevents agents from discovering or accessing files outside the sandbox
- Isolates package manager state (npm, pip, etc.) from your system
- Provides a clean, reproducible execution environment

**If a file is not mounted, it does not exist to the agent.**

## What sbox is NOT

sbox is **not** a security sandbox in the kernel sense. It does not:

- Use namespaces, seccomp, or cgroups
- Prevent ptrace or other syscall-based attacks
- Isolate network access
- Restrict CPU, memory, or disk usage
- Protect against malicious native code exploits

## Threat Model

### Protected Against

| Threat | Protection |
|--------|------------|
| Agent reads `~/.ssh/id_rsa` | File doesn't exist in sandbox |
| Agent reads `~/.aws/credentials` | File doesn't exist in sandbox |
| Agent modifies `~/.bashrc` | Writes go to virtual HOME |
| Agent runs `npm install -g malware` | Installs to sandbox only |
| Agent pollutes pip/npm cache | Cache is sandbox-local |
| Agent discovers your file structure | Only sees sandbox contents |
| Accidental file deletion | Can't delete what doesn't exist |

### NOT Protected Against

| Threat | Why |
|--------|-----|
| Agent makes network requests | No network isolation |
| Agent consumes all CPU/RAM | No resource limits |
| Malicious native code (buffer overflow, etc.) | No syscall filtering |
| Agent exploits kernel vulnerabilities | No kernel-level isolation |
| Agent reads explicitly mounted paths | Mounts are trust boundaries |

## Design Philosophy

### "Isolation through absence, not permission"

Traditional sandboxes block access to files. sbox takes a different approach: **files outside the sandbox simply don't exist**.

```
Traditional sandbox:
  Agent: open("/home/user/.ssh/id_rsa")
  Kernel: EACCES (Permission denied)
  Agent: "Hmm, file exists but I can't read it..."

sbox:
  Agent: open("/home/user/.ssh/id_rsa")
  System: ENOENT (No such file or directory)
  Agent: "File doesn't exist."
```

This is a weaker security boundary, but sufficient for the primary use case: **preventing AI agents from accidentally accessing sensitive files**.

### "Trust the sandbox, not the agent"

sbox assumes:
- The agent code may be buggy or unpredictable
- The agent may try to read files it shouldn't
- The agent may install packages globally
- The agent may create files in unexpected locations

sbox does NOT assume:
- The agent is actively malicious
- The agent will attempt syscall exploits
- The agent will try to escape the sandbox

## When to Use sbox

**Good use cases:**
- Running AI coding agents (Claude, GPT, etc.)
- Isolating untrusted npm/pip packages
- Creating reproducible development environments
- Preventing accidental access to sensitive files

**Not recommended for:**
- Running untrusted binaries from the internet
- Containing actively malicious code
- High-security isolation requirements
- Multi-tenant hosting environments

## Comparison with Other Tools

| Feature | sbox | Docker | Firejail | gVisor |
|---------|------|--------|----------|--------|
| Kernel isolation | ❌ | ✅ | ✅ | ✅ |
| Filesystem isolation | ✅ | ✅ | ✅ | ✅ |
| Network isolation | ❌ | ✅ | ✅ | ✅ |
| Resource limits | ❌ | ✅ | ✅ | ✅ |
| No root required | ✅ | ❌* | ❌ | ❌ |
| No daemon required | ✅ | ❌ | ✅ | ❌ |
| Works in containers | ✅ | ❌ | ❌ | ❌ |
| Simple setup | ✅ | ⚠️ | ⚠️ | ❌ |

*Docker rootless mode exists but has limitations.

## Hardening Recommendations

For higher security requirements, combine sbox with:

1. **Run in a VM** - Full kernel isolation
2. **Use Docker** - `docker run --rm -it $(pwd):/workspace` 
3. **Network firewall** - Restrict outbound connections
4. **Resource limits** - Use `ulimit` or cgroups
5. **Read-only mounts** - Use `:ro` flag for all mounts

Example hardened setup:
```bash
# Run sbox inside Docker for defense in depth
docker run --rm -it \
  --network=none \
  --memory=2g \
  --cpus=1 \
  -v $(pwd):/workspace \
  ubuntu:22.04 \
  bash -c "cd /workspace && ./sbox run"
```

## Reporting Security Issues

If you discover a security vulnerability in sbox, please report it privately:

1. **Do NOT** open a public GitHub issue
2. Email the maintainers directly (see repository for contact info)
3. Include steps to reproduce the issue
4. Allow time for a fix before public disclosure

## Summary

sbox provides **practical isolation for AI agents** — not bulletproof security, but meaningful protection against the most common risks: accidental file access, environment pollution, and credential exposure.

For most AI agent use cases, this is exactly the right tradeoff: **simple, portable, effective isolation without the complexity of kernel sandboxing.**

> *"Don't trust the agent. Trust the sandbox."*
