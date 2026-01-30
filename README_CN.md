# sbox

[English](README.md) | [中文](README_CN.md)

一个用于运行 AI 代理的安全沙箱，强制隔离环境 - **无需 Docker 或 sudo**。

**如果文件没有被挂载，它对代理来说就不存在。**

> *"不要信任代理，信任沙箱。"*
>
> sbox 通过环境隔离而非承诺来强制执行隔离。你的文件受到保护，不是因为代理表现良好，而是因为它们根本不存在于代理的世界中。

## 为什么选择 sbox？

AI 代理功能强大但不可预测。当你给代理访问系统的权限时，你是在信任它不会：
- 读取工作区之外的敏感文件
- 修改系统配置
- 访问不应该拥有的凭据或 API 密钥
- 干扰其他进程

**sbox 通过限制代理可以访问的内容来消除信任的需要**。每个代理都在隔离环境中运行：
- 独立的文件系统（除非明确挂载，否则无法访问主机文件）
- 独立的运行时（Python/Node.js 与系统安装隔离）
- 受控的环境变量（只有你配置的内容）
- 无需网络命名空间技巧或 root 权限

可以将其视为专门为 AI 代理工作负载设计的轻量级容器，具有完全在用户空间运行的类 Docker 工作流程。

## 隐私与隔离

> **注意：** sbox 在用户空间提供文件系统和环境隔离。它不是内核级安全沙箱。

一些 AI 代理工具（如 [OpenClaw](https://github.com/openclaw/openclaw)）默认以用户级权限直接运行。这意味着除非采取额外措施，否则代理可能会访问你的整个主目录，包括私人文件。

**sbox 通过设计解决了这个问题。**

代理在具有虚拟 HOME 目录的隔离沙箱内执行。如果文件没有明确放入沙箱，它对代理来说根本不存在。

**如果文件没有被复制或挂载，它对代理就不存在。** 你的 SSH 密钥、云凭据、浏览器数据和个人文档保持完全不可见——不是被限制、不是权限被拒绝，而是从代理的角度看字面上不存在。代理无法访问、列出甚至检测沙箱外的文件。

| 没有 sbox | 使用 sbox |
|-----------|-----------|
| 代理可以访问 `~/.ssh/`、`~/.aws/` 等 | 代理只能看到 `.sbox/rootfs/home/` |
| 代理可以读取浏览器 cookie、凭据 | 私人文件对代理不可见 |
| 必须完全信任代理代码 | 零信任 - 代理被限制 |

## 功能特性

- **默认零信任** - 代理只能访问你明确允许的内容
- **无需 sudo** - 完全在用户空间运行，不需要 root 权限
- **无需 Docker** - 在任何 Linux/macOS 系统上工作，无需容器运行时
- **AI 代理就绪** - 专为安全运行代码生成代理而设计
- **多运行时支持** - Python 和 Node.js 环境
- **可移植环境** - 打包和分发跨机器的沙箱
- **进程管理** - 后台守护进程、日志和监控
- **快速设置** - 使用 micromamba 快速创建环境

## 快速开始

> **新手？** 从这里开始：
> 1. [快速开始](#快速开始)
> 2. [隐私与隔离](#隐私与隔离)
> 3. [打包与分发](#打包与分发)

### Python 项目

```bash
# 初始化新的 Python 项目
sbox init myapp --runtime python:3.11

# 进入项目目录
cd myapp

# 构建沙箱环境
sbox build

# 运行应用
sbox run

# 或启动交互式 shell
sbox shell
```

### Node.js 项目

```bash
# 初始化新的 Node.js 项目
sbox init myapp --runtime node:22

cd myapp
sbox build
sbox run
```

## 安装

### 从源码构建

```bash
# 需要 Go 1.21+
git clone https://github.com/CVPaul/sbox.git
cd sbox
go build -o sbox ./cmd/sbox

# 可选：安装到 PATH
sudo mv sbox /usr/local/bin/
```

### 预编译二进制

```bash
# 下载适合你平台的最新版本
curl -LO https://github.com/CVPaul/sbox/releases/latest/download/sbox-linux-amd64
chmod +x sbox-linux-amd64
mv sbox-linux-amd64 /usr/local/bin/sbox
```

## 包管理器被沙箱化

当你在 sbox 内运行 `npm install -g`、`pip install --user` 或任何包管理器命令时，包被安装**到沙箱中，而不是你的系统**。

**"全局"安装是沙箱全局的，不是系统全局的。**

| 包管理器 | 没有 sbox | 使用 sbox |
|----------|-----------|-----------|
| `npm install -g` | `/usr/local/lib/node_modules/` | `.sbox/env/lib/node_modules/` |
| `pip install --user` | `~/.local/lib/python*/` | 被 `PYTHONNOUSERSITE=1` 阻止 |
| `pip install` | 系统或 venv | `.sbox/env/lib/python*/` |
| npm 缓存 | `~/.npm/` | `.sbox/rootfs/home/.npm/` |
| pip 缓存 | `~/.cache/pip/` | `.sbox/rootfs/home/.cache/pip/` |

这通过环境变量重定向实现：

```bash
# 在沙箱内，这些会自动设置：
HOME=/path/to/project/.sbox/rootfs/home
TMPDIR=/path/to/project/.sbox/rootfs/tmp
PYTHONNOUSERSITE=1        # 阻止 ~/.local 导入
PATH=.sbox/env/bin:...    # 沙箱二进制文件优先
```

**这对 AI 代理意味着什么：**
- 代理可以运行 `npm install -g malicious-package` — 它只影响沙箱
- 代理可以运行 `pip install sketchy-lib` — 它无法触及你的系统 Python
- 代理无法污染你的 `~/.npmrc`、`~/.pip/` 或任何其他点文件

## 配置示例

以下是运行 AI 编码代理的典型 `.sbox/config.yaml`：

```yaml
# .sbox/config.yaml
runtime: node:22

# 只复制代理需要的内容
copy:
  - src: ../my-project
    dst: /app/workspace

# 沙箱内的工作目录
workdir: /app/workspace

# 运行代理的命令
cmd: node agent.js

# 传递给代理的环境变量
env:
  OPENAI_API_KEY: "${OPENAI_API_KEY}"   # 从主机传递
  NODE_ENV: "production"

# 可选：挂载目录（建议只读）
mount:
  - "/path/to/datasets:/data:ro"        # 只读数据集访问

# 构建时命令（在 'sbox build' 期间运行一次）
build:
  - npm install
  - npm run build
```

**运行 `sbox build` + `sbox run` 时发生什么：**

```
你的系统                               沙箱 (.sbox/)
─────────────────────────────────────────────────────────────────
~/.ssh/                         →    (不可见)
~/.aws/                         →    (不可见)
~/.npmrc                        →    (不可见)
~/.gitconfig                    →    (不可见)

$HOME                           →    .sbox/rootfs/home/     (空)
$TMPDIR                         →    .sbox/rootfs/tmp/
$PATH                           →    .sbox/env/bin:...

npm install -g <pkg>            →    .sbox/env/lib/node_modules/
pip install <pkg>               →    .sbox/env/lib/python*/
~/                              →    .sbox/rootfs/home/
```

代理看到一个隔离的世界，你的凭据和系统文件根本不存在。

## 命令

### 核心命令

| 命令 | 描述 |
|------|------|
| `sbox init <name>` | 初始化新的 sbox 项目 |
| `sbox build` | 构建沙箱环境 |
| `sbox run [cmd]` | 运行应用（或自定义命令） |
| `sbox shell` | 在沙箱中启动交互式 shell |
| `sbox exec <cmd>` | 在沙箱中执行命令 |
| `sbox clean` | 清理构建产物 |
| `sbox version` | 打印版本信息 |

### 进程管理

| 命令 | 描述 |
|------|------|
| `sbox run -d` | 作为后台守护进程运行 |
| `sbox ps` | 列出运行中的沙箱进程 |
| `sbox stop [name]` | 停止运行中的守护进程 |
| `sbox restart [name]` | 重启守护进程 |
| `sbox logs [name]` | 查看进程日志 |

### 打包与分发

| 命令 | 描述 |
|------|------|
| `sbox pack` | 将沙箱打包为可移植的 tar.gz 归档 |
| `sbox unpack` | 为新位置重定位提取归档中的路径 |
| `sbox cache list` | 列出缓存的运行时 |
| `sbox cache clean` | 移除缓存的运行时 |

## 目录挂载

将主机目录挂载到沙箱中而不复制文件。挂载通过符号链接实现，提供零复制开销的直接访问。

> **警告：** 挂载有意将主机路径暴露给沙箱，应被视为明确的信任边界。

```yaml
# 挂载大型数据集
mount:
  - /mnt/datasets:/data

# 挂载预训练模型（避免复制数 GB 的权重）
mount:
  - ~/.cache/huggingface:/models

# 只读挂载配置
mount:
  - /etc/myapp:/etc/myapp:ro
```

## `sbox unpack` 是重定位器，不是安装器

明确说明：**`sbox unpack` 不安装任何东西。** 它是一个纯粹的路径重定位器。

| 人们可能期望的 | 实际发生的 |
|----------------|------------|
| 运行 `npm install` 或 `pip install` | ❌ 没有调用包管理器 |
| 执行安装后脚本 | ❌ 没有脚本被执行 |
| 从互联网下载依赖 | ❌ 零网络访问 |
| 修改项目外的文件 | ❌ 只触及 `.sbox/` 目录 |
| 需要提升权限 | ❌ 以普通用户运行 |

**`sbox unpack` 唯一做的事：**
1. 从 `metadata.json` 读取原始构建路径
2. 在配置文件中查找并替换该路径为当前路径
3. 写回更新的文件

没有执行代码。没有调用解释器。没有网络连接。`.sbox/` 之外的文件从不被读取或写入。

## 与其他工具的比较

| 功能 | sbox | Docker | venv | nvm |
|------|------|--------|------|-----|
| 无需 root | 是 | 否* | 是 | 是 |
| 多语言 | 是 | 是 | 仅 Python | 仅 Node |
| 可移植 | 是 | 是 | 否 | 否 |
| 快速启动 | 是 | 否 | 是 | 是 |
| 进程管理 | 是 | 是 | 否 | 否 |
| 日志管理 | 是 | 是 | 否 | 否 |
| 网络隔离 | 否 | 是 | 否 | 否 |

*Docker 可以无 root 运行但需要额外设置

## 支持的运行时

| 运行时 | 版本 | 包管理器 |
|--------|------|----------|
| Python | 3.8, 3.9, 3.10, 3.11, 3.12 | pip |
| Node.js | 18, 20, 22, 23 | npm, pnpm |

## 故障排除

### 构建因网络超时失败

尝试使用镜像：

```yaml
install:
  - pip install -i https://pypi.tuna.tsinghua.edu.cn/simple -r requirements.txt
```

### 使用中国 npm 镜像

```yaml
install:
  - cd /app && npm config set registry https://registry.npmmirror.com
  - cd /app && npm install
```

或使用 pnpm：

```yaml
install:
  - cd /app && pnpm install --registry https://registry.npmmirror.com
```

## 安全模型

有关详细的安全信息，请参阅 [SECURITY.md](SECURITY.md)。

**简要说明：**
- sbox 提供用户空间的文件系统和环境隔离
- 它不是内核级安全沙箱
- 对于大多数 AI 代理用例，这是正确的权衡：简单、可移植、有效的隔离

## 贡献

欢迎贡献！请随时提交 Pull Request。

## 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE)。

## 致谢

- [micromamba](https://mamba.readthedocs.io/) - 快速、最小化的 conda 包管理器
- [Cobra](https://github.com/spf13/cobra) - Go 的 CLI 框架
- [OpenClaw](https://github.com/openclaw/openclaw) - 示例部署目标
